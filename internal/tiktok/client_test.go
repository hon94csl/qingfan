package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignMatchesOfficialShopsExample(t *testing.T) {
	query := url.Values{}
	query.Set("app_key", "29a39d")
	query.Set("timestamp", "1623812664")
	query.Set("access_token", "ignored")
	query.Set("sign", "ignored")
	got := Sign("e59af819cc", "/authorization/202309/shops", query, "")
	const want = "b596b73e0cc6de07ac26f036364178ab16b0a907af13d43f0a0cd2345f582dc8"
	if got != want {
		t.Fatalf("sign %s", got)
	}
}

func TestMinorUnits(t *testing.T) {
	got, err := MinorUnits("38.24", "usd")
	if err != nil || got == nil || *got != 3824 {
		t.Fatalf("usd: %v %v", got, err)
	}
	got, err = MinorUnits("", "IDR")
	if err != nil || got != nil {
		t.Fatalf("blank amount: %v %v", got, err)
	}
	got, err = MinorUnits("10", "")
	if err != nil || got != nil {
		t.Fatalf("blank currency invented a unit: %v %v", got, err)
	}
	if _, err := MinorUnits("1.2", "JPY"); err == nil {
		t.Fatal("extra fraction accepted")
	}
	got, err = MinorUnits("0", "IDR")
	if err != nil || got == nil || *got != 0 {
		t.Fatalf("zero: %v %v", got, err)
	}
}

func TestParseVideosKeepsMissingDistinctFromZero(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"message": "Success",
		"data": {
			"latest_available_date": "2026-10-13",
			"videos": [
				{"id": "v1", "title": "面霜", "username": "shop", "gmv": {"amount": "38.24", "currency": "USD"}},
				{"video_id": "v2", "views": 0, "sku_orders": 3, "gmv": 12.5},
				{"title": "没有编号", "views": 9},
				{"id": "v3", "views": null, "orders": 0, "gmv": {"amount": "1", "currency": ""}}
			]
		}
	}`)
	page, err := parseVideos(body)
	if err != nil {
		t.Fatal(err)
	}
	if page.Latest != "2026-10-13" || len(page.Videos) != 3 {
		t.Fatalf("page: %+v", page)
	}
	if page.Videos[0].Views != nil || page.Videos[0].Orders != nil || page.Videos[0].GMVMinor == nil || *page.Videos[0].GMVMinor != 3824 || page.Videos[0].Currency != "USD" {
		t.Fatalf("money: %+v", page.Videos[0])
	}
	if page.Videos[1].ID != "v2" || page.Videos[1].Views == nil || *page.Videos[1].Views != 0 || page.Videos[1].Orders == nil || *page.Videos[1].Orders != 3 || page.Videos[1].GMVMinor != nil || page.Videos[1].Currency != "" {
		t.Fatalf("zero and bare gmv: %+v", page.Videos[1])
	}
	if page.Videos[2].Views != nil || page.Videos[2].Orders == nil || *page.Videos[2].Orders != 0 || page.Videos[2].GMVMinor != nil || page.Videos[2].Currency != "" {
		t.Fatalf("null and blank currency: %+v", page.Videos[2])
	}
}

func TestParseRejectsFailedCode(t *testing.T) {
	_, err := parseToken([]byte(`{"code":36009004,"message":"invalid auth code","data":null}`))
	var api *APIError
	if !errors.As(err, &api) || err.Error() != "TikTok Shop：invalid auth code" {
		t.Fatalf("code: %v", err)
	}
}

func TestExchangeRejectsNonSeller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("grant_type") != "authorized_code" || r.URL.Query().Get("sign") != "" {
			t.Errorf("token query grant %s sign %s", r.URL.Query().Get("grant_type"), r.URL.Query().Get("sign"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "success",
			"data": map[string]any{"access_token": "a", "refresh_token": "r", "open_id": "o", "user_type": 1},
		})
	}))
	defer server.Close()
	client := &Client{AppKey: "key", AppSecret: "secret", ServiceID: "service", HTTP: server.Client(), AuthBase: server.URL}
	_, err := client.Exchange(context.Background(), "once")
	if !errors.Is(err, ErrSeller) {
		t.Fatalf("seller: %v", err)
	}
}

func TestShopsRequestKeepsTokenInHeader(t *testing.T) {
	const access = "access-1"
	var gotHeader, gotQueryToken, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("x-tts-access-token")
		gotQueryToken = r.URL.Query().Get("access_token")
		gotPath = r.URL.Path
		query := r.URL.Query()
		sign := query.Get("sign")
		query.Del("sign")
		want := Sign("secret", ShopsPath, query, "")
		if sign != want {
			t.Errorf("request sign %s", sign)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"shops": []map[string]string{{
				"id": "shop-1", "name": "店", "region": "ID", "seller_type": "LOCAL", "cipher": "cipher-1", "code": "C1",
			}}},
		})
	}))
	defer server.Close()
	client := &Client{
		AppKey: "29a39d", AppSecret: "secret", ServiceID: "service",
		HTTP: server.Client(), APIBase: server.URL,
		Now: func() time.Time { return time.Unix(1623812664, 0) },
	}
	shops, err := client.Shops(context.Background(), access)
	if err != nil {
		t.Fatal(err)
	}
	if gotHeader != access || gotQueryToken != "" || gotPath != ShopsPath || len(shops) != 1 || shops[0].Cipher != "cipher-1" {
		t.Fatalf("header %q query %q path %s shops %+v", gotHeader, gotQueryToken, gotPath, shops)
	}
}

func TestTransportErrorHidesSecret(t *testing.T) {
	client := &Client{
		AppKey: "key", AppSecret: "app-secret-value", ServiceID: "service",
		HTTP: &http.Client{Timeout: time.Millisecond}, AuthBase: "http://127.0.0.1:1",
	}
	_, err := client.Exchange(context.Background(), "auth-code-value")
	if err == nil || strings.Contains(err.Error(), "app-secret-value") || strings.Contains(err.Error(), "auth-code-value") {
		t.Fatalf("transport: %v", err)
	}
}

func TestAuthorizeURLUsesMarketHost(t *testing.T) {
	client := &Client{AppKey: "key", AppSecret: "secret", ServiceID: "service id"}
	link, err := client.AuthorizeURL("state 1")
	if err != nil || !strings.HasPrefix(link, "https://services.tiktokshop.com/open/authorize?") || !strings.Contains(link, "service_id=service+id") || !strings.Contains(link, "state=state+1") {
		t.Fatalf("row: %s %v", link, err)
	}
	client.US = true
	link, err = client.AuthorizeURL("")
	if err != nil || !strings.HasPrefix(link, "https://services.us.tiktokshop.com/open/authorize?") || strings.Contains(link, "state=") {
		t.Fatalf("us: %s %v", link, err)
	}
	if _, err := (&Client{}).AuthorizeURL(""); !errors.Is(err, ErrConfig) {
		t.Fatalf("config: %v", err)
	}
}
