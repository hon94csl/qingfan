package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/ai"
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
