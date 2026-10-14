// Package auth stores administrator and desktop accounts for the local server.
// Passwords and session tokens are never stored in the clear.
// Desktop accounts identify who can use the app. Payment is not collected here.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	sessionTTL  = 12 * time.Hour
	minPassword = 12
	maxPassword = 72
	KindAdmin   = "admin"
	KindDesktop = "desktop"
)

var (
	ErrInvalid            = errors.New("请填写有效的邮箱，密码至少 12 个字符")
	ErrExists             = errors.New("这个邮箱已经注册")
	ErrCredentials        = errors.New("邮箱或密码不正确")
	ErrUnauthorized       = errors.New("请先登录")
	ErrRegistrationClosed = errors.New("现在没有开放注册")
)

// Store is the account database. One connection matches the local SQLite writer.
type Store struct {
	db    *sql.DB
	cost  int
	dummy []byte
}

// Open creates the account tables in path.
func Open(path string) (*Store, error) {
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
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`ALTER TABLE accounts ADD COLUMN kind TEXT NOT NULL DEFAULT 'admin'`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		db.Close()
		return nil, err
	}
	store := &Store{db: db, cost: bcrypt.DefaultCost}
	dummy, err := bcrypt.GenerateFromPassword([]byte(randomFallback()), store.cost)
	if err != nil {
		db.Close()
		return nil, err
	}
	store.dummy = dummy
	return store, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS accounts (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  kind TEXT NOT NULL DEFAULT 'admin'
);
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES accounts(id),
  expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);`

// Account is a person who can sign in. The password hash stays in the database.
// KindAdmin manages the admin console. KindDesktop is a desktop user the operator can bill later.
type Account struct {
	ID        string `json:"-"`
	Email     string `json:"email"`
	Kind      string `json:"kind"`
	CreatedAt int64  `json:"created_at"`
}

// RegistrationOpen reports whether the desktop may create its own accounts.
// A missing setting stays closed.
func (s *Store) RegistrationOpen() (bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, "registration").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return value == "open", nil
}

// SetRegistrationOpen stores the administrator's choice. Existing accounts can still sign in when it is closed.
func (s *Store) SetRegistrationOpen(open bool) error {
	value := "closed"
	if open {
		value = "open"
	}
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES ('registration', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, value)
	return err
}

// Count returns how many administrators exist.
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE kind = ?`, KindAdmin).Scan(&n)
	return n, err
}

// List returns administrators without password hashes.
func (s *Store) List() ([]Account, error) {
	return s.ListKind(KindAdmin)
}

// ListKind returns accounts of one kind without password hashes.
func (s *Store) ListKind(kind string) ([]Account, error) {
	rows, err := s.db.Query(`SELECT email, kind, created_at FROM accounts WHERE kind = ? ORDER BY created_at, email`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Account{}
	for rows.Next() {
		var row Account
		if err := rows.Scan(&row.Email, &row.Kind, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Create adds an administrator and does not start a session.
func (s *Store) Create(email, password string) (Account, error) {
	return s.create(email, password, KindAdmin)
}

// CreateDesktop adds a desktop user and does not start a session.
func (s *Store) CreateDesktop(email, password string) (Account, error) {
	return s.create(email, password, KindDesktop)
}

func (s *Store) create(email, password, kind string) (Account, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return Account{}, err
	}
	if err := checkPassword(password); err != nil {
		return Account{}, err
	}
	if kind != KindAdmin && kind != KindDesktop {
		return Account{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return Account{}, err
	}
	id, err := newID()
	if err != nil {
		return Account{}, err
	}
	created := time.Now().Unix()
	_, err = s.db.Exec(
		`INSERT INTO accounts (id, email, password_hash, created_at, kind) VALUES (?, ?, ?, ?, ?)`,
		id, normalized, string(hash), created, kind,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Account{}, ErrExists
		}
		return Account{}, err
	}
	return Account{ID: id, Email: normalized, Kind: kind, CreatedAt: created}, nil
}

// Register creates the first administrator and a session. The token is shown once.
func (s *Store) Register(email, password string) (string, string, error) {
	account, err := s.Create(email, password)
	if err != nil {
		return "", "", err
	}
	var id string
	if err := s.db.QueryRow(`SELECT id FROM accounts WHERE email = ?`, account.Email).Scan(&id); err != nil {
		return "", "", err
	}
	token, err := s.startSession(id)
	if err != nil {
		return "", "", err
	}
	return account.Email, token, nil
}

// Login checks the password and starts a session.
// An unknown email spends the same password check as a wrong password.
func (s *Store) Login(email, password string) (Account, string, error) {
	normalized, err := normalizeEmail(email)
	if err != nil || checkPassword(password) != nil {
		_ = bcrypt.CompareHashAndPassword(s.dummy, []byte(limitPassword(password)))
		return Account{}, "", ErrCredentials
	}
	var account Account
	var id, hash string
	err = s.db.QueryRow(`SELECT id, email, kind, password_hash, created_at FROM accounts WHERE email = ?`, normalized).Scan(&id, &account.Email, &account.Kind, &hash, &account.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummy, []byte(password))
		return Account{}, "", ErrCredentials
	}
	if err != nil {
		return Account{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return Account{}, "", ErrCredentials
	}
	token, err := s.startSession(id)
	if err != nil {
		return Account{}, "", err
	}
	account.ID = id
	return account, token, nil
}

// Logout removes one session. A missing token is not an error.
func (s *Store) Logout(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// Lookup returns the account for a live session.
func (s *Store) Lookup(token string) (Account, error) {
	if token == "" {
		return Account{}, ErrUnauthorized
	}
	var account Account
	err := s.db.QueryRow(`
		SELECT accounts.id, accounts.email, accounts.kind, accounts.created_at
		FROM sessions JOIN accounts ON accounts.id = sessions.account_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		hashToken(token), time.Now().Unix(),
	).Scan(&account.ID, &account.Email, &account.Kind, &account.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Store) startSession(accountID string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (token_hash, account_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), accountID, time.Now().Add(sessionTTL).Unix(),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) < 3 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return "", ErrInvalid
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at != strings.IndexByte(email, '@') || at >= len(email)-1 {
		return "", ErrInvalid
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", ErrInvalid
	}
	return email, nil
}

func checkPassword(password string) error {
	if len(password) < minPassword || len(password) > maxPassword {
		return ErrInvalid
	}
	return nil
}

func limitPassword(password string) string {
	if len(password) > maxPassword {
		return password[:maxPassword]
	}
	return password
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomFallback() string {
	token, err := newToken()
	if err != nil {
		return "qingfan-account-fallback"
	}
	return token
}

// equalToken compares two already-normalized secrets without early length leaks
// when the lengths match. Callers use it for fixed allow-lists, not passwords.
func equalToken(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
