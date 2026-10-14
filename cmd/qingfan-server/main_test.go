package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/auth"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"GET", "/healthz", 200}, {"POST", "/healthz", 405}, {"GET", "/api/me", 404},
	} {
		res := httptest.NewRecorder()
		handler().ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
		if res.Code != tc.code {
			t.Fatalf("%s %s: %d", tc.method, tc.path, res.Code)
		}
		if tc.code == 200 {
			var body struct {
				Status  string
				Service string
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body.Status != "ok" || body.Service != "qingfan-server" {
				t.Fatalf("invalid health: %s", res.Body)
			}
		}
	}
}

func TestV1StaysClosedWithoutGateway(t *testing.T) {
	res := httptest.NewRecorder()
	handler().ServeHTTP(res, httptest.NewRequest("POST", "/v1/scripts", nil))
	if res.Code != 404 {
		t.Fatalf("closed gateway: %d", res.Code)
	}
}

func TestV1RejectsAnonymousScripts(t *testing.T) {
	gateway, err := ai.Open(filepath.Join(t.TempDir(), "usage.sqlite"), ai.Fixture{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gateway.Close() })
	res := httptest.NewRecorder()
	withGateway(gateway, "boot", log.New(io.Discard, "", 0)).ServeHTTP(res, httptest.NewRequest("POST", "/v1/scripts", nil))
	if res.Code != 401 {
		t.Fatalf("anonymous: %d %s", res.Code, res.Body.String())
	}
}

func TestAccountAndWorkspaceRoutesStaySeparate(t *testing.T) {
	dir := t.TempDir()
	gateway, err := ai.Open(filepath.Join(dir, "usage.sqlite"), ai.Fixture{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gateway.Close() })
	accounts, err := auth.Open(filepath.Join(dir, "auth.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { accounts.Close() })
	app := routes(gateway, accounts, "boot", log.New(io.Discard, "", 0))

	scripts := httptest.NewRecorder()
	app.ServeHTTP(scripts, httptest.NewRequest(http.MethodPost, "/v1/scripts", nil))
	if scripts.Code != http.StatusUnauthorized {
		t.Fatalf("scripts: %d %s", scripts.Code, scripts.Body)
	}

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/login", strings.NewReader(`{"email":"ops@example.com","password":"long-test-password"}`))
	app.ServeHTTP(login, req)
	if login.Code != http.StatusForbidden {
		t.Fatalf("login without origin: %d %s", login.Code, login.Body)
	}
}
