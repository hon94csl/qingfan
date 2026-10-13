package ai

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const MaxAttempts = 3

// Usage is a supplier cost. Known is false when the supplier omitted it.
// A missing cost stays NULL and is never stored as zero.
type Usage struct {
	Known  bool
	Micros int64
}

// CostReport separates known spend from calls that still need reconciliation.
type CostReport struct {
	KnownMicros   int64 `json:"known_micros"`
	UnknownCalls  int   `json:"unknown_calls"`
	BillableCalls int   `json:"billable_calls"`
	CacheHits     int   `json:"cache_hits"`
}

type tokenRow struct {
	Hash    string
	Tenant  string
	Quota   int
	Revoked bool
}

func openUsage(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(usageSchema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

const usageSchema = `
CREATE TABLE IF NOT EXISTS tokens (
  hash TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  quota INTEGER NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS usage_events (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  segment_id TEXT NOT NULL DEFAULT '',
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  kind TEXT NOT NULL,
  voice TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL,
  attempt INTEGER NOT NULL,
  status TEXT NOT NULL,
  billable INTEGER NOT NULL,
  cost_micros INTEGER,
  cost_state TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS response_cache (
  tenant_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  voice TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY (tenant_id, kind, voice, input_hash)
);`

func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func hashInput(kind, voice, model, version string, input []byte) string {
	h := sha256.New()
	writePart := func(part string) {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	writePart(kind)
	writePart(voice)
	writePart(model)
	writePart(version)
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (g *Gateway) Issue(tenant string, quota int) (string, error) {
	if tenant == "" || quota < 1 {
		return "", fmt.Errorf("令牌缺少工作空间或额度")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	plain := hex.EncodeToString(buf)
	_, err := g.db.Exec(`INSERT INTO tokens(hash, tenant_id, quota, revoked, created_at) VALUES (?, ?, ?, 0, ?)`,
		hashToken(plain), tenant, quota, nowText())
	if err != nil {
		return "", err
	}
	return plain, nil
}

func (g *Gateway) Revoke(plain string) error {
	res, err := g.db.Exec(`UPDATE tokens SET revoked=1 WHERE hash=?`, hashToken(plain))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnauthorized
	}
	return nil
}

func (g *Gateway) lookup(plain string) (tokenRow, error) {
	var row tokenRow
	var revoked int
	err := g.db.QueryRow(`SELECT hash, tenant_id, quota, revoked FROM tokens WHERE hash=?`, hashToken(plain)).Scan(&row.Hash, &row.Tenant, &row.Quota, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return tokenRow{}, ErrUnauthorized
	}
	if err != nil {
		return tokenRow{}, err
	}
	row.Revoked = revoked == 1
	return row, nil
}

func (g *Gateway) cached(tenant, kind, voice, sum string) ([]byte, bool, error) {
	var body string
	err := g.db.QueryRow(`SELECT body FROM response_cache WHERE tenant_id=? AND kind=? AND voice=? AND input_hash=?`, tenant, kind, voice, sum).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(body), true, nil
}

func (g *Gateway) remember(tenant, kind, voice, sum, body string) error {
	_, err := g.db.Exec(`INSERT INTO response_cache(tenant_id, kind, voice, input_hash, body) VALUES (?, ?, ?, ?, ?)`, tenant, kind, voice, sum, body)
	return err
}

func (g *Gateway) reserve(row tokenRow, call Call, sum, provider string) (string, error) {
	tx, err := g.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var used int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM usage_events WHERE tenant_id=? AND billable=1`, row.Tenant).Scan(&used); err != nil {
		return "", err
	}
	if used >= row.Quota {
		return "", ErrQuota
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(`INSERT INTO usage_events(id, tenant_id, task_id, segment_id, provider, model, prompt_version, kind, voice, input_hash, attempt, status, billable, cost_micros, cost_state, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'running', 1, NULL, 'unknown', ?)`,
		id, row.Tenant, call.TaskID, call.SegmentID, provider, call.Model, call.PromptVersion, call.Kind, call.Voice, sum, call.Attempt, nowText())
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (g *Gateway) finish(id, status string, usage Usage, ok bool) error {
	state := "unknown"
	var micros any
	if ok && usage.Known {
		state = "known"
		micros = usage.Micros
	}
	if !ok && usage.Known {
		state = "known"
		micros = usage.Micros
	}
	_, err := g.db.Exec(`UPDATE usage_events SET status=?, cost_micros=?, cost_state=? WHERE id=?`, status, micros, state, id)
	return err
}

func (g *Gateway) recordCache(row tokenRow, call Call, sum, provider string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	_, err = g.db.Exec(`INSERT INTO usage_events(id, tenant_id, task_id, segment_id, provider, model, prompt_version, kind, voice, input_hash, attempt, status, billable, cost_micros, cost_state, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'cache', 0, NULL, 'none', ?)`,
		id, row.Tenant, call.TaskID, call.SegmentID, provider, call.Model, call.PromptVersion, call.Kind, call.Voice, sum, call.Attempt, nowText())
	return err
}

// Costs reports one tenant. The tenant comes from the token, not from the caller.
func (g *Gateway) Costs(plain string) (CostReport, error) {
	row, err := g.lookup(plain)
	if err != nil {
		return CostReport{}, err
	}
	if row.Revoked {
		return CostReport{}, ErrRevoked
	}
	var report CostReport
	var known sql.NullInt64
	err = g.db.QueryRow(`
		SELECT COALESCE(SUM(cost_micros), 0),
		       COALESCE(SUM(CASE WHEN cost_state='unknown' AND billable=1 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(billable), 0),
		       COALESCE(SUM(CASE WHEN status='cache' THEN 1 ELSE 0 END), 0)
		FROM usage_events WHERE tenant_id=?`, row.Tenant).Scan(&known, &report.UnknownCalls, &report.BillableCalls, &report.CacheHits)
	if err != nil {
		return CostReport{}, err
	}
	report.KnownMicros = known.Int64
	return report, nil
}

// EventCost reads one row's stored cost. ok is false when the cost is NULL.
func (g *Gateway) EventCost(id string) (micros int64, ok bool, state string, err error) {
	var value sql.NullInt64
	err = g.db.QueryRow(`SELECT cost_micros, cost_state FROM usage_events WHERE id=?`, id).Scan(&value, &state)
	if err != nil {
		return 0, false, "", err
	}
	return value.Int64, value.Valid, state, nil
}

func (g *Gateway) latestEvent(tenant, status string) (string, error) {
	var id string
	err := g.db.QueryRow(`SELECT id FROM usage_events WHERE tenant_id=? AND status=? ORDER BY created_at DESC LIMIT 1`, tenant, status).Scan(&id)
	return id, err
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339Nano) }
