package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hon94csl/qingfan/internal/media"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const tenantID = "local"

var (
	ErrReference    = errors.New("参考素材不能放进成片")
	ErrSKU          = errors.New("这个镜头属于其他商品，不能放进当前成片")
	ErrStale        = errors.New("原分析已失效，请重新选择镜头")
	ErrMissing      = errors.New("找不到文件，请重新定位")
	ErrHash         = errors.New("新位置的内容和原素材不一致")
	ErrOutputExists = errors.New("这个路径已经有成片，草稿修改不会覆盖它")
	ErrRange        = errors.New("镜头区间超出了素材时长")
	ErrVoice        = errors.New("配音超过了这一段的结尾")
	ErrCaption      = errors.New("字幕超出了这一段")
)

// IngestFunc analyses one file. A matching cache reuses the previous probe.
type IngestFunc func(ctx context.Context, path, kind, thumbDir string, cached *media.CachedProbe) (media.Ingested, error)

// Store is the local project database.
type Store struct {
	db       *sql.DB
	cacheDir string
	ingest   IngestFunc
}

// Open creates the database and applies migrations.
func Open(path, cacheDir string) (*Store, error) {
	return open(path, cacheDir, media.Ingest)
}

func open(path, cacheDir string, ingest IngestFunc) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, err
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(filepath.Dir(path), "cache")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, cacheDir: cacheDir, ingest: ingest}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		var seen int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=?`, name).Scan(&seen); err != nil {
			return err
		}
		if seen > 0 {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(body), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) thumbDir() string { return filepath.Join(s.cacheDir, "thumbs") }

// StartJob implements jobs.Book.
func (s *Store) StartJob(kind string) (string, error) {
	id, err := newID()
	if err != nil {
		return "", err
	}
	now := nowText()
	_, err = s.db.Exec(`INSERT INTO jobs(id, kind, status, error, created_at, updated_at) VALUES (?, ?, 'running', '', ?, ?)`, id, kind, now, now)
	return id, err
}

// FinishJob implements jobs.Book.
func (s *Store) FinishJob(id, status, message string) error {
	_, err := s.db.Exec(`UPDATE jobs SET status=?, error=?, updated_at=? WHERE id=?`, status, message, nowText(), id)
	return err
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
