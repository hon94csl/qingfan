package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

const cookieName = "qf_session"

// DefaultOrigins are the admin console and the desktop dev page.
func DefaultOrigins() []string {
	return []string{
		"http://127.0.0.1:5173",
		"http://localhost:5173",
		"http://127.0.0.1:5188",
		"http://localhost:5188",
		"http://wails.localhost",
	}
}

// Mount adds register, login, logout, and session routes.
func Mount(mux *http.ServeMux, store *Store, origins []string) {
	h := &handler{store: store, origins: origins, limit: newLimiter(20, time.Minute)}
	mux.HandleFunc("GET /v1/setup", h.setup)
	mux.HandleFunc("POST /v1/register", h.register)
	mux.HandleFunc("POST /v1/login", h.login)
	mux.HandleFunc("POST /v1/logout", h.logout)
	mux.HandleFunc("GET /v1/session", h.session)
	mux.HandleFunc("GET /v1/accounts", h.accounts)
	mux.HandleFunc("GET /v1/desktop-accounts", h.desktopAccounts)
	mux.HandleFunc("POST /v1/desktop-accounts", h.createDesktopAccount)
	mux.HandleFunc("GET /v1/registration", h.registration)
	mux.HandleFunc("POST /v1/registration", h.setRegistration)
}

type handler struct {
	store   *Store
	origins []string
	limit   *limiter
}

func (h *handler) setup(w http.ResponseWriter, r *http.Request) {
	count, err := h.store.Count()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_admin": count == 0})
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	if !originOK(r, h.origins) {
		writeErr(w, http.StatusForbidden, errors.New("不能从这里注册或登录"))
		return
	}
	if !h.limit.allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, errors.New("尝试太多次，请稍后再试"))
		return
	}
	count, err := h.store.Count()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if count > 0 {
		if _, err := h.admin(r); err != nil {
			writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
	}
	email, password, err := readAccount(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, ErrInvalid)
		return
	}
	if count > 0 {
		account, err := h.store.Create(email, password)
		if err != nil {
			writeErr(w, statusOf(err), err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"email": account.Email})
		return
	}
	saved, token, err := h.store.Register(email, password)
	if err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	setSession(w, r, token)
	writeJSON(w, http.StatusCreated, map[string]string{"email": saved})
}

func (h *handler) accounts(w http.ResponseWriter, r *http.Request) {
	if _, err := h.admin(r); err != nil {
		writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	rows, err := h.store.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": rows})
}

func (h *handler) desktopAccounts(w http.ResponseWriter, r *http.Request) {
	if _, err := h.admin(r); err != nil {
		writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	rows, err := h.store.ListKind(KindDesktop)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": rows})
}

func (h *handler) registration(w http.ResponseWriter, r *http.Request) {
	open, err := h.store.RegistrationOpen()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"open": open})
}

func (h *handler) setRegistration(w http.ResponseWriter, r *http.Request) {
	if !originOK(r, h.origins) {
		writeErr(w, http.StatusForbidden, errors.New("不能从这里注册或登录"))
		return
	}
	if _, err := h.admin(r); err != nil {
		writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var body struct {
		Open bool `json:"open"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, ErrInvalid)
		return
	}
	if err := h.store.SetRegistrationOpen(body.Open); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"open": body.Open})
}

func (h *handler) createDesktopAccount(w http.ResponseWriter, r *http.Request) {
	if !originOK(r, h.origins) {
		writeErr(w, http.StatusForbidden, errors.New("不能从这里注册或登录"))
		return
	}
	if _, err := h.admin(r); err != nil {
		writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	email, password, err := readAccount(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, ErrInvalid)
		return
	}
	account, err := h.store.CreateDesktop(email, password)
	if err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"email": account.Email})
}

// Admin reads the session cookie and returns the signed-in administrator.
// A desktop session is not an administrator.
func Admin(r *http.Request, store *Store) (Account, error) {
	if store == nil {
		return Account{}, ErrUnauthorized
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return Account{}, ErrUnauthorized
	}
	account, err := store.Lookup(cookie.Value)
	if err != nil {
		return Account{}, err
	}
	if account.Kind != KindAdmin {
		return Account{}, ErrUnauthorized
	}
	return account, nil
}

func (h *handler) admin(r *http.Request) (Account, error) {
	account, err := h.current(r)
	if err != nil {
		return Account{}, err
	}
	if account.Kind != KindAdmin {
		return Account{}, ErrUnauthorized
	}
	return account, nil
}

func (h *handler) current(r *http.Request) (Account, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return Account{}, ErrUnauthorized
	}
	return h.store.Lookup(cookie.Value)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	if !originOK(r, h.origins) {
		writeErr(w, http.StatusForbidden, errors.New("不能从这里注册或登录"))
		return
	}
	if !h.limit.allow(clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, errors.New("尝试太多次，请稍后再试"))
		return
	}
	email, password, err := readAccount(w, r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, ErrCredentials)
		return
	}
	account, token, err := h.store.Login(email, password)
	if err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	if account.Kind != KindAdmin {
		_ = h.store.Logout(token)
		writeErr(w, http.StatusForbidden, errors.New("这个账号只能登录桌面应用"))
		return
	}
	setSession(w, r, token)
	writeJSON(w, http.StatusOK, map[string]string{"email": account.Email, "kind": account.Kind})
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if !originOK(r, h.origins) {
		writeErr(w, http.StatusForbidden, errors.New("不能从这里注册或登录"))
		return
	}
	cookie, _ := r.Cookie(cookieName)
	token := ""
	if cookie != nil {
		token = cookie.Value
	}
	if err := h.store.Logout(token); err != nil {
		writeErr(w, http.StatusInternalServerError, errors.New("服务暂时不可用"))
		return
	}
	clearSession(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) session(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, ErrUnauthorized)
		return
	}
	account, err := h.store.Lookup(cookie.Value)
	if err != nil {
		writeErr(w, statusOf(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"email": account.Email, "kind": account.Kind})
}

func readAccount(w http.ResponseWriter, r *http.Request) (string, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		return "", "", err
	}
	return body.Email, body.Password, nil
}

func setSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})
}

func originOK(r *http.Request, origins []string) bool {
	got := r.Header.Get("Origin")
	if got == "" {
		return false
	}
	for _, origin := range origins {
		if equalToken(got, origin) {
			return true
		}
	}
	return false
}

func statusOf(err error) int {
	switch {
	case errors.Is(err, ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ErrExists):
		return http.StatusConflict
	case errors.Is(err, ErrCredentials), errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrRegistrationClosed):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func writeErr(w http.ResponseWriter, code int, err error) {
	message := "服务暂时不可用"
	if code != http.StatusInternalServerError {
		message = err.Error()
	}
	writeJSON(w, code, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *limiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	var kept []time.Time
	for _, hit := range l.hits[key] {
		if hit.After(cutoff) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// Protect adds credentialed CORS for the local pages and answers their preflight.
func Protect(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if originOK(r, origins) {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
