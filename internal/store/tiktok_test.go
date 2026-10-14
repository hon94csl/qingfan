package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func TestTikTokFactsReplaceOneRangeAndHideSecrets(t *testing.T) {
	dir := t.TempDir()
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	token := tiktok.Token{
		AccessToken: "plain-access-token", RefreshToken: "plain-refresh-token",
		OpenID: "open-1", SellerName: "卖家", AccessExpire: 100, RefreshExpire: 200,
		Scopes: []string{"seller.authorization.info"},
	}
	shop := tiktok.Shop{ID: "shop-1", Name: "店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-secret", Code: "C1"}
	if err := s.SaveTikTokAuth("acct-a", token, []tiktok.Shop{shop}); err != nil {
		t.Fatal(err)
	}
	shops, err := s.ListTikTokShops("acct-a")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(shops)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "plain-access-token") || strings.Contains(string(raw), "plain-refresh-token") || strings.Contains(string(raw), "cipher-secret") {
		t.Fatalf("public shop leaked a secret: %s", raw)
	}
	secret, err := s.TikTokSecret("acct-a", "shop-1")
	if err != nil || secret.AccessToken != "plain-access-token" || secret.Cipher != "cipher-secret" {
		t.Fatalf("secret: %+v %v", secret, err)
	}

	views := 8
	zero := 0
	gmv := 3824
	first := []tiktok.Video{
		{ID: "v1", Title: "一", Username: "a", Views: &views, GMVMinor: &gmv, Currency: "USD"},
		{ID: "v2", Title: "二", Views: &zero},
	}
	if err := s.ReplaceShopVideos("acct-a", "shop-1", "2026-10-01", "2026-10-08", "2026-10-13", first); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceShopVideos("acct-a", "shop-1", "2026-09-01", "2026-09-08", "", []tiktok.Video{{ID: "old", Title: "旧"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceShopVideos("acct-a", "shop-1", "2026-10-01", "2026-10-08", "2026-10-13", []tiktok.Video{{ID: "v1", Title: "改"}}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTikTokVideos("acct-a", "shop-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].VideoID != "old" || rows[1].VideoID != "v1" || rows[1].Title != "改" || rows[1].Views != nil || rows[1].Orders != nil || rows[1].GMVMinor != nil {
		t.Fatalf("rows: %+v", rows)
	}
	var metrics int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM metrics_daily`).Scan(&metrics); err != nil || metrics != 0 {
		t.Fatalf("metrics_daily %d %v", metrics, err)
	}
	if _, err := s.TikTokSecret("acct-a", "missing"); !errors.Is(err, tiktok.ErrShop) {
		t.Fatalf("missing shop: %v", err)
	}
	if _, err := s.TikTokSecret("acct-b", "shop-1"); !errors.Is(err, tiktok.ErrShop) {
		t.Fatalf("other account secret: %v", err)
	}
}

func TestOneAccountKeepsItsShops(t *testing.T) {
	dir := t.TempDir()
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	first := tiktok.Token{AccessToken: "token-a", RefreshToken: "refresh-a", OpenID: "open-a", SellerName: "甲"}
	if err := s.SaveTikTokAuth("acct-a", first, []tiktok.Shop{
		{ID: "shop-1", Name: "一店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-1", Code: "C1"},
		{ID: "shop-2", Name: "二店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-2", Code: "C2"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTikTokAuth("acct-a", first, []tiktok.Shop{
		{ID: "shop-3", Name: "三店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-3", Code: "C3"},
	}); err != nil {
		t.Fatal(err)
	}
	shops, err := s.ListTikTokShops("acct-a")
	if err != nil || len(shops) != 3 || shops[0].Name != "一店" || shops[1].Name != "三店" || shops[2].Name != "二店" {
		t.Fatalf("shops %+v %v", shops, err)
	}
	if others, err := s.ListTikTokShops("acct-b"); err != nil || len(others) != 0 {
		t.Fatalf("other list %+v %v", others, err)
	}
	if empty, err := s.ListTikTokShops(""); err != nil || len(empty) != 0 {
		t.Fatalf("empty account %+v %v", empty, err)
	}
	if err := s.SaveTikTokAuth("acct-b", tiktok.Token{AccessToken: "token-b", RefreshToken: "refresh-b", OpenID: "open-b"}, []tiktok.Shop{
		{ID: "shop-1", Name: "抢走", Cipher: "cipher-x", Region: "ID", SellerType: "LOCAL"},
	}); !errors.Is(err, tiktok.ErrOwned) {
		t.Fatalf("owned: %v", err)
	}
	secret, err := s.TikTokSecret("acct-a", "shop-1")
	if err != nil || secret.AccessToken != "token-a" {
		t.Fatalf("token changed: %+v %v", secret, err)
	}
	if err := s.ReplaceShopVideos("acct-b", "shop-1", "2026-10-01", "2026-10-08", "", nil); !errors.Is(err, tiktok.ErrShop) {
		t.Fatalf("other replace: %v", err)
	}
	if videos, err := s.ListTikTokVideos("acct-b", ""); err != nil || len(videos) != 0 {
		t.Fatalf("other videos %+v %v", videos, err)
	}
	if err := s.SaveTikTokAuth("", first, []tiktok.Shop{{ID: "shop-9", Name: "无主", Cipher: "c", Region: "ID", SellerType: "LOCAL"}}); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("ownerless: %v", err)
	}
}

func TestTikTokStateIsSingleUse(t *testing.T) {
	dir := t.TempDir()
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.SaveTikTokState("once", "", 200); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("ownerless state: %v", err)
	}
	if err := s.SaveTikTokState("once", "acct-a", 200); err != nil {
		t.Fatal(err)
	}
	owner, err := s.ConsumeTikTokState("once", 150)
	if err != nil || owner != "acct-a" {
		t.Fatalf("owner %q %v", owner, err)
	}
	if _, err := s.ConsumeTikTokState("once", 150); !errors.Is(err, tiktok.ErrState) {
		t.Fatalf("reuse: %v", err)
	}
	if err := s.SaveTikTokState("old", "acct-a", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConsumeTikTokState("old", 100); !errors.Is(err, tiktok.ErrState) {
		t.Fatalf("expired: %v", err)
	}
}
