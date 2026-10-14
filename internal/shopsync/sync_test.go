package shopsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func TestSyncWritesTheBusinessDatabase(t *testing.T) {
	clock := time.Unix(1_700_000_000, 0)
	var mu sync.Mutex
	var videoTokens []string
	var shopCalls, refreshCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "access_token=") {
			t.Errorf("access token leaked into the query")
		}
		switch r.URL.Path {
		case "/api/v2/token/get":
			if r.URL.Query().Get("grant_type") != "authorized_code" {
				t.Errorf("grant %s", r.URL.Query().Get("grant_type"))
			}
			writeToken(w, "access-old", "refresh-old", 0, clock.Unix()+100)
		case "/api/v2/token/refresh":
			mu.Lock()
			refreshCalls++
			mu.Unlock()
			writeToken(w, "access-new", "refresh-new", 0, clock.Unix()+86400)
		case tiktok.ShopsPath:
			mu.Lock()
			shopCalls++
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"shops": []map[string]string{{
				"id": "shop-1", "name": "店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-secret", "code": "C1",
			}}}})
		case tiktok.VideosPath:
			mu.Lock()
			videoTokens = append(videoTokens, r.Header.Get("x-tts-access-token"))
			mu.Unlock()
			if r.URL.Query().Get("account_type") != "ALL" || r.URL.Query().Get("shop_cipher") != "cipher-secret" {
				t.Errorf("video query %s", r.URL.RawQuery)
			}
			page := r.URL.Query().Get("page_token")
			if page == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
					"latest_available_date": "2026-10-13", "next_page_token": "page-2",
					"videos": []any{map[string]any{"id": "v1", "title": "一", "views": 4, "gmv": map[string]string{"amount": "38.24", "currency": "USD"}}},
				}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
				"latest_available_date": "2026-10-13",
				"videos":                []any{map[string]any{"video_id": "v2", "views": 0, "orders": 2}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openDB(t)
	client := &tiktok.Client{
		AppKey: "key", AppSecret: "app-secret-value", ServiceID: "service",
		HTTP: server.Client(), AuthBase: server.URL, APIBase: server.URL,
		Now: func() time.Time { return clock },
	}
	svc := &Service{DB: db, Client: client, Callback: "http://127.0.0.1:8090/v1/tiktok/callback"}
	if _, err := (&Service{DB: db, Client: &tiktok.Client{}}).Connect(context.Background(), "acct-a"); !errors.Is(err, tiktok.ErrConfig) {
		t.Fatalf("config: %v", err)
	}
	if _, err := svc.Connect(context.Background(), " "); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("account: %v", err)
	}
	started, err := svc.Connect(context.Background(), "acct-a")
	if err != nil || !strings.Contains(started.URL, "state=") {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), " ", started.State); !errors.Is(err, tiktok.ErrCode) || shopCalls != 0 {
		t.Fatalf("code %v calls %d", err, shopCalls)
	}
	if _, err := svc.Complete(context.Background(), "once", "wrong"); !errors.Is(err, tiktok.ErrState) || shopCalls != 0 {
		t.Fatalf("state %v calls %d", err, shopCalls)
	}
	shops, err := svc.Complete(context.Background(), "once", started.State)
	if err != nil || len(shops) != 1 || shops[0].Name != "店" {
		t.Fatalf("shops %+v %v", shops, err)
	}
	raw, _ := json.Marshal(shops)
	if strings.Contains(string(raw), "cipher-secret") || strings.Contains(string(raw), "access-old") || strings.Contains(string(raw), "app-secret-value") {
		t.Fatalf("complete leaked a secret: %s", raw)
	}
	if _, err := svc.Sync(context.Background(), "acct-a", "shop-1", "2026-10-08", "2026-10-01"); !errors.Is(err, tiktok.ErrRange) {
		t.Fatalf("range: %v", err)
	}
	if _, err := svc.Sync(context.Background(), "acct-b", "shop-1", "2026-10-01", "2026-10-08"); !errors.Is(err, tiktok.ErrShop) {
		t.Fatalf("other account: %v", err)
	}
	got, err := svc.Sync(context.Background(), "acct-a", "shop-1", "2026-10-01", "2026-10-08")
	if err != nil || got.Written != 2 || got.Latest != "2026-10-13" || refreshCalls != 1 {
		t.Fatalf("sync %+v refresh %d err %v", got, refreshCalls, err)
	}
	if len(videoTokens) != 2 || videoTokens[0] != "access-new" || videoTokens[1] != "access-new" {
		t.Fatalf("video tokens %v", videoTokens)
	}
	rows, err := db.ListTikTokVideos("acct-a", "shop-1")
	if err != nil || len(rows) != 2 || rows[0].GMVMinor == nil || *rows[0].GMVMinor != 3824 || rows[1].Views == nil || *rows[1].Views != 0 {
		t.Fatalf("stored %+v %v", rows, err)
	}

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/token/refresh" {
			writeToken(w, "access-new", "refresh-new", 0, clock.Unix()+86400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"videos": []any{}, "latest_available_date": "2026-10-13"}})
	}))
	defer empty.Close()
	client.HTTP = empty.Client()
	client.APIBase = empty.URL
	client.AuthBase = empty.URL
	replaced, err := svc.Sync(context.Background(), "acct-a", "shop-1", "2026-10-01", "2026-10-08")
	if err != nil || replaced.Written != 0 || replaced.Note != "这个区间没有视频" {
		t.Fatalf("replace %+v %v", replaced, err)
	}
	rows, err = db.ListTikTokVideos("acct-a", "")
	if err != nil || len(rows) != 0 {
		t.Fatalf("after replace %+v %v", rows, err)
	}
}

func TestPartialSyncDoesNotWrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/token/get" {
			writeToken(w, "access", "refresh", 0, time.Now().Unix()+86400)
			return
		}
		if r.URL.Path == tiktok.ShopsPath {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"shops": []map[string]string{{
				"id": "shop-1", "name": "店", "cipher": "cipher", "region": "ID", "seller_type": "LOCAL",
			}}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
			"next_page_token": "again", "videos": []any{map[string]any{"id": "v1"}},
		}})
	}))
	defer server.Close()
	db := openDB(t)
	client := &tiktok.Client{AppKey: "key", AppSecret: "secret", ServiceID: "service", HTTP: server.Client(), AuthBase: server.URL, APIBase: server.URL}
	svc := &Service{DB: db, Client: client, PageLimit: 1}
	started, err := svc.Connect(context.Background(), "acct-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), "once", started.State); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sync(context.Background(), "acct-a", "shop-1", "2026-10-01", "2026-10-08"); !errors.Is(err, tiktok.ErrPartial) {
		t.Fatalf("partial: %v", err)
	}
	rows, err := db.ListTikTokVideos("acct-a", "shop-1")
	if err != nil || len(rows) != 0 {
		t.Fatalf("wrote a partial page: %+v %v", rows, err)
	}
}

func TestNonSellerIsNotStored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeToken(w, "access", "refresh", 1, time.Now().Unix()+86400)
	}))
	defer server.Close()
	db := openDB(t)
	client := &tiktok.Client{AppKey: "key", AppSecret: "secret", ServiceID: "service", HTTP: server.Client(), AuthBase: server.URL}
	svc := &Service{DB: db, Client: client}
	started, err := svc.Connect(context.Background(), "acct-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), "once", started.State); !errors.Is(err, tiktok.ErrSeller) {
		t.Fatalf("seller: %v", err)
	}
	shops, err := db.ListTikTokShops("acct-a")
	if err != nil || len(shops) != 0 {
		t.Fatalf("shops %+v %v", shops, err)
	}
}

func TestLaterAuthorizationAddsShops(t *testing.T) {
	var pass int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/token/get" {
			writeToken(w, "access", "refresh", 0, time.Now().Unix()+86400)
			return
		}
		shops := []map[string]string{{"id": "shop-1", "name": "一店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-1", "code": "C1"}}
		if pass > 0 {
			shops = []map[string]string{{"id": "shop-2", "name": "二店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-2", "code": "C2"}}
		}
		pass++
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"shops": shops}})
	}))
	defer server.Close()
	db := openDB(t)
	client := &tiktok.Client{AppKey: "key", AppSecret: "secret", ServiceID: "service", HTTP: server.Client(), AuthBase: server.URL, APIBase: server.URL}
	svc := &Service{DB: db, Client: client}
	first, err := svc.Connect(context.Background(), "acct-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), "one", first.State); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Connect(context.Background(), "acct-a")
	if err != nil {
		t.Fatal(err)
	}
	shops, err := svc.Complete(context.Background(), "two", second.State)
	if err != nil || len(shops) != 2 {
		t.Fatalf("shops %+v %v", shops, err)
	}
	if others, err := db.ListTikTokShops("acct-b"); err != nil || len(others) != 0 {
		t.Fatalf("other %+v %v", others, err)
	}
}

func openDB(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func writeToken(w http.ResponseWriter, access, refresh string, userType int, expire int64) {
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{
		"access_token": access, "refresh_token": refresh, "open_id": "open-1", "seller_name": "卖家",
		"user_type": userType, "access_token_expire_in": expire, "refresh_token_expire_in": expire,
		"granted_scopes": []string{"seller.authorization.info"},
	}})
}
