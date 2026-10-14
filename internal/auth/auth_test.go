package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "auth.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	store.cost = bcrypt.MinCost
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password-value"), store.cost)
	if err != nil {
		t.Fatal(err)
	}
	store.dummy = dummy
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRegisterLoginAndSession(t *testing.T) {
	store := newStore(t)
	mux := http.NewServeMux()
	Mount(mux, store, []string{"http://127.0.0.1:5173"})

	res := call(mux, "POST", "/v1/register", `{"email":" Ops@Example.com ","password":"long-test-password"}`, true)
	if res.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", res.Code, res.Body)
	}
	var created struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil || created.Email != "ops@example.com" {
		t.Fatalf("created: %s", res.Body)
	}
	if strings.Contains(res.Body.String(), "long-test-password") {
		t.Fatal("password leaked in the response")
	}
	cookie := sessionCookie(t, res)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge <= 0 {
		t.Fatalf("cookie: %+v", cookie)
	}

	req := httptest.NewRequest("GET", "/v1/session", nil)
	req.AddCookie(cookie)
	got := httptest.NewRecorder()
	mux.ServeHTTP(got, req)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "ops@example.com") {
		t.Fatalf("session: %d %s", got.Code, got.Body)
	}

	open := call(mux, "POST", "/v1/register", `{"email":"second@example.com","password":"another-long-password"}`, true)
	if open.Code != http.StatusUnauthorized {
		t.Fatalf("register without session: %d %s", open.Code, open.Body)
	}
	again := postSession(mux, cookie, "/v1/register", `{"email":"ops@example.com","password":"another-long-password"}`)
	if again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), "已经注册") {
		t.Fatalf("duplicate: %d %s", again.Code, again.Body)
	}
	short := postSession(mux, cookie, "/v1/register", `{"email":"new@example.com","password":"short"}`)
	if short.Code != http.StatusBadRequest {
		t.Fatalf("short password: %d %s", short.Code, short.Body)
	}
	added := postSession(mux, cookie, "/v1/register", `{"email":"second@example.com","password":"another-long-password"}`)
	if added.Code != http.StatusCreated || len(added.Result().Cookies()) != 0 {
		t.Fatalf("adding an admin replaced the session: %d cookies=%d %s", added.Code, len(added.Result().Cookies()), added.Body)
	}
	still := httptest.NewRequest("GET", "/v1/session", nil)
	still.AddCookie(cookie)
	same := httptest.NewRecorder()
	mux.ServeHTTP(same, still)
	if same.Code != http.StatusOK || !strings.Contains(same.Body.String(), "ops@example.com") {
		t.Fatalf("session changed: %d %s", same.Code, same.Body)
	}
	hidden := httptest.NewRecorder()
	mux.ServeHTTP(hidden, httptest.NewRequest("GET", "/v1/accounts", nil))
	if hidden.Code != http.StatusUnauthorized {
		t.Fatalf("accounts: %d", hidden.Code)
	}
	listed := httptest.NewRequest("GET", "/v1/accounts", nil)
	listed.AddCookie(cookie)
	list := httptest.NewRecorder()
	mux.ServeHTTP(list, listed)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "second@example.com") || strings.Contains(list.Body.String(), "password") {
		t.Fatalf("list: %d %s", list.Code, list.Body)
	}

	bad := call(mux, "POST", "/v1/login", `{"email":"ops@example.com","password":"wrong-password-value"}`, true)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d %s", bad.Code, bad.Body)
	}
	missing := call(mux, "POST", "/v1/login", `{"email":"missing@example.com","password":"long-test-password"}`, true)
	if missing.Code != http.StatusUnauthorized || missing.Body.String() != bad.Body.String() {
		t.Fatalf("unknown email should match a wrong password: %s vs %s", missing.Body, bad.Body)
	}

	ok := call(mux, "POST", "/v1/login", `{"email":"ops@example.com","password":"long-test-password"}`, true)
	if ok.Code != http.StatusOK {
		t.Fatalf("login: %d %s", ok.Code, ok.Body)
	}
	next := sessionCookie(t, ok)
	logout := httptest.NewRequest("POST", "/v1/logout", nil)
	logout.Header.Set("Origin", "http://127.0.0.1:5173")
	logout.AddCookie(next)
	loggedOut := httptest.NewRecorder()
	mux.ServeHTTP(loggedOut, logout)
	if loggedOut.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", loggedOut.Code, loggedOut.Body)
	}
	after := httptest.NewRequest("GET", "/v1/session", nil)
	after.AddCookie(next)
	closed := httptest.NewRecorder()
	mux.ServeHTTP(closed, after)
	if closed.Code != http.StatusUnauthorized {
		t.Fatalf("session after logout: %d %s", closed.Code, closed.Body)
	}
}

func TestDesktopAccountStaysOutOfTheAdminConsole(t *testing.T) {
	store := newStore(t)
	mux := http.NewServeMux()
	Mount(mux, store, []string{"http://127.0.0.1:5173"})
	if _, err := store.CreateDesktop("Desk@Example.com", "long-test-password"); err != nil {
		t.Fatal(err)
	}
	if n, err := store.Count(); err != nil || n != 0 {
		t.Fatalf("desktop accounts must not count as administrators: %d %v", n, err)
	}
	setup := call(mux, "GET", "/v1/setup", "", false)
	if setup.Code != http.StatusOK || !strings.Contains(setup.Body.String(), `"needs_admin":true`) {
		t.Fatalf("setup: %d %s", setup.Code, setup.Body)
	}

	res := call(mux, "POST", "/v1/register", `{"email":"ops@example.com","password":"long-test-password"}`, true)
	if res.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", res.Code, res.Body)
	}
	cookie := sessionCookie(t, res)
	blocked := call(mux, "POST", "/v1/desktop-accounts", `{"email":"shop@example.com","password":"another-long-password"}`, true)
	if blocked.Code != http.StatusUnauthorized {
		t.Fatalf("desktop account without admin: %d %s", blocked.Code, blocked.Body)
	}
	added := postSession(mux, cookie, "/v1/desktop-accounts", `{"email":"shop@example.com","password":"another-long-password"}`)
	if added.Code != http.StatusCreated || len(added.Result().Cookies()) != 0 || strings.Contains(added.Body.String(), "another-long-password") {
		t.Fatalf("create desktop account: %d cookies=%d %s", added.Code, len(added.Result().Cookies()), added.Body)
	}
	if n, err := store.Count(); err != nil || n != 1 {
		t.Fatalf("administrator count: %d %v", n, err)
	}
	admins := httptest.NewRequest("GET", "/v1/accounts", nil)
	admins.AddCookie(cookie)
	adminList := httptest.NewRecorder()
	mux.ServeHTTP(adminList, admins)
	if adminList.Code != http.StatusOK || strings.Contains(adminList.Body.String(), "shop@example.com") || strings.Contains(adminList.Body.String(), "desk@example.com") {
		t.Fatalf("admin list: %d %s", adminList.Code, adminList.Body)
	}
	hidden := call(mux, "GET", "/v1/desktop-accounts", "", false)
	if hidden.Code != http.StatusUnauthorized {
		t.Fatalf("desktop list: %d %s", hidden.Code, hidden.Body)
	}
	listed := httptest.NewRequest("GET", "/v1/desktop-accounts", nil)
	listed.AddCookie(cookie)
	deskList := httptest.NewRecorder()
	mux.ServeHTTP(deskList, listed)
	body := deskList.Body.String()
	if deskList.Code != http.StatusOK || !strings.Contains(body, "shop@example.com") || !strings.Contains(body, "desk@example.com") || strings.Contains(body, "password") {
		t.Fatalf("desktop accounts: %d %s", deskList.Code, deskList.Body)
	}

	refused := call(mux, "POST", "/v1/login", `{"email":"shop@example.com","password":"another-long-password"}`, true)
	if refused.Code != http.StatusForbidden || len(refused.Result().Cookies()) != 0 || !strings.Contains(refused.Body.String(), "只能登录桌面应用") {
		t.Fatalf("desktop login on the admin console: %d cookies=%d %s", refused.Code, len(refused.Result().Cookies()), refused.Body)
	}
	account, token, err := store.Login("shop@example.com", "another-long-password")
	if err != nil || account.Kind != KindDesktop || account.Email != "shop@example.com" {
		t.Fatalf("desktop store login: %+v %v", account, err)
	}
	if looked, err := store.Lookup(token); err != nil || looked.Email != account.Email {
		t.Fatalf("desktop session: %+v %v", looked, err)
	}
	still := httptest.NewRequest("GET", "/v1/session", nil)
	still.AddCookie(cookie)
	same := httptest.NewRecorder()
	mux.ServeHTTP(same, still)
	if same.Code != http.StatusOK || !strings.Contains(same.Body.String(), "ops@example.com") {
		t.Fatalf("admin session: %d %s", same.Code, same.Body)
	}
}

func TestRegistrationSwitch(t *testing.T) {
	store := newStore(t)
	mux := http.NewServeMux()
	Mount(mux, store, []string{"http://127.0.0.1:5173"})
	closed, err := store.RegistrationOpen()
	if err != nil || closed {
		t.Fatalf("default: %v %v", closed, err)
	}
	got := call(mux, "GET", "/v1/registration", "", false)
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"open":false`) {
		t.Fatalf("read: %d %s", got.Code, got.Body)
	}
	denied := call(mux, "POST", "/v1/registration", `{"open":true}`, true)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d %s", denied.Code, denied.Body)
	}
	created := call(mux, "POST", "/v1/register", `{"email":"ops@example.com","password":"long-test-password"}`, true)
	if created.Code != http.StatusCreated {
		t.Fatalf("admin: %d %s", created.Code, created.Body)
	}
	cookie := sessionCookie(t, created)
	missing := httptest.NewRequest("POST", "/v1/registration", strings.NewReader(`{"open":true}`))
	missing.AddCookie(cookie)
	noOrigin := httptest.NewRecorder()
	mux.ServeHTTP(noOrigin, missing)
	if noOrigin.Code != http.StatusForbidden {
		t.Fatalf("origin: %d %s", noOrigin.Code, noOrigin.Body)
	}
	opened := postSession(mux, cookie, "/v1/registration", `{"open":true}`)
	if opened.Code != http.StatusOK || !strings.Contains(opened.Body.String(), `"open":true`) {
		t.Fatalf("open: %d %s", opened.Code, opened.Body)
	}
	if open, err := store.RegistrationOpen(); err != nil || !open {
		t.Fatalf("stored: %v %v", open, err)
	}
	shut := postSession(mux, cookie, "/v1/registration", `{"open":false}`)
	if shut.Code != http.StatusOK || !strings.Contains(shut.Body.String(), `"open":false`) {
		t.Fatalf("close: %d %s", shut.Code, shut.Body)
	}
	if _, err := store.CreateDesktop("desk@example.com", "long-test-password"); err != nil {
		t.Fatal(err)
	}
	if n, err := store.Count(); err != nil || n != 1 {
		t.Fatalf("closing registration must keep the administrator: %d %v", n, err)
	}
}

func TestOriginIsRequired(t *testing.T) {
	store := newStore(t)
	mux := http.NewServeMux()
	Mount(mux, store, []string{"http://127.0.0.1:5173"})
	for _, origin := range []string{"", "https://evil.example"} {
		req := httptest.NewRequest("POST", "/v1/login", strings.NewReader(`{"email":"ops@example.com","password":"long-test-password"}`))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusForbidden {
			t.Fatalf("origin %q: %d %s", origin, res.Code, res.Body)
		}
	}
}

func TestExpiredSession(t *testing.T) {
	store := newStore(t)
	email, token, err := store.Register("ops@example.com", "long-test-password")
	if err != nil || email != "ops@example.com" {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE sessions SET expires_at = 1 WHERE token_hash = ?`, hashToken(token)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Lookup(token); !errorsIsUnauthorized(err) {
		t.Fatalf("expired: %v", err)
	}
}

func errorsIsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

func postSession(mux *http.ServeMux, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	return res
}

func call(mux *http.ServeMux, method, path, body string, origin bool) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if origin {
		req.Header.Set("Origin", "http://127.0.0.1:5173")
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	return res
}

func sessionCookie(t *testing.T, res *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookies := res.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || cookies[0].Value == "" {
		t.Fatalf("cookies: %+v", cookies)
	}
	return cookies[0]
}
