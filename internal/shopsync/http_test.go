package shopsync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/tiktok"
)

func TestShopRoutesStayOffTheAdminAPI(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux, &Service{})
	shops := httptest.NewRecorder()
	mux.ServeHTTP(shops, httptest.NewRequest(http.MethodGet, "/v1/tiktok/shops", nil))
	if shops.Code != http.StatusNotFound {
		t.Fatalf("shops %d", shops.Code)
	}
	callback := httptest.NewRecorder()
	mux.ServeHTTP(callback, httptest.NewRequest(http.MethodGet, "/v1/tiktok/callback", nil))
	if callback.Code != http.StatusBadRequest || !strings.Contains(callback.Body.String(), "还没有配置 TikTok Shop 应用") {
		t.Fatalf("callback %d %s", callback.Code, callback.Body)
	}
}

func TestCallbackWritesShopsWithoutReturningTokens(t *testing.T) {
	const access = "plain-access-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/token/get" {
			writeToken(w, access, "plain-refresh-token", 0, time.Now().Unix()+86400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"shops": []map[string]string{
			{"id": "shop-1", "name": "一店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-secret", "code": "C1"},
			{"id": "shop-2", "name": "二店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-2", "code": "C2"},
		}}})
	}))
	defer server.Close()
	db := openDB(t)
	client := &tiktok.Client{AppKey: "key", AppSecret: "secret", ServiceID: "service", HTTP: server.Client(), AuthBase: server.URL, APIBase: server.URL}
	svc := &Service{DB: db, Client: client}
	started, err := svc.Connect(context.Background(), "acct-a")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Mount(mux, svc)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/tiktok/callback?code=once&state="+started.State, nil))
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, "店铺已写入后台数据库") || !strings.Contains(body, "可以回到桌面应用") || strings.Contains(body, "管理后台") || strings.Contains(body, access) || strings.Contains(body, "once") {
		t.Fatalf("callback %d %s", res.Code, body)
	}
	shops, err := db.ListTikTokShops("acct-a")
	if err != nil || len(shops) != 2 {
		t.Fatalf("db %+v %v", shops, err)
	}
	if others, err := db.ListTikTokShops("acct-b"); err != nil || len(others) != 0 {
		t.Fatalf("other %+v %v", others, err)
	}
}
