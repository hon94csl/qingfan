package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/shopsync"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func TestTikTokStaysClosedWithoutService(t *testing.T) {
	res := httptest.NewRecorder()
	handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/tiktok/shops", nil))
	if res.Code != http.StatusNotFound {
		t.Fatalf("closed: %d", res.Code)
	}
}

func TestTikTokRouteBeatsTheGateway(t *testing.T) {
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
	_, _, err = accounts.Register("ops@example.com", "long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	business, err := store.Open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { business.Close() })
	app := serve(gateway, accounts, "boot", log.New(io.Discard, "", 0), &shopsync.Service{
		DB: business, Client: &tiktok.Client{}, Callback: "http://127.0.0.1:8090/v1/tiktok/callback",
	})

	callback := httptest.NewRecorder()
	app.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "/v1/tiktok/callback", nil))
	if callback.Code != http.StatusBadRequest || !strings.Contains(callback.Body.String(), "还没有配置 TikTok Shop 应用") {
		t.Fatalf("callback %d %s", callback.Code, callback.Body)
	}
	scripts := httptest.NewRecorder()
	app.ServeHTTP(scripts, httptest.NewRequest(http.MethodPost, "/v1/scripts", nil))
	if scripts.Code != http.StatusUnauthorized {
		t.Fatalf("scripts %d %s", scripts.Code, scripts.Body)
	}
}
