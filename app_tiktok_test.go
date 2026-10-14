package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/credentials"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func TestListPlatformVideosReadsTheBusinessDatabase(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })

	if _, err := app.ListPlatformVideos(); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	if _, err := app.ListPlatformVideos(); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("missing account id: %v", err)
	}
	app.who.ID = "acct-desk"
	t.Setenv("QINGFAN_TIKTOK_APP_KEY", "")
	t.Setenv("QINGFAN_TIKTOK_APP_SECRET", "")
	t.Setenv("QINGFAN_TIKTOK_SERVICE_ID", "")
	t.Setenv("QINGFAN_TIKTOK_CALLBACK", "")
	status, err := app.PlatformStatus()
	if err != nil || status.Configured || status.Callback != "http://127.0.0.1:8090/v1/tiktok/callback" {
		t.Fatalf("status %+v %v", status, err)
	}
	if _, err := app.ConnectPlatformShop(); !errors.Is(err, tiktok.ErrConfig) {
		t.Fatalf("connect: %v", err)
	}
	gmv := 3824
	if err := app.db.SaveTikTokAuth("acct-desk", tiktok.Token{
		AccessToken: "plain-access-token", RefreshToken: "plain-refresh-token", OpenID: "open-1", SellerName: "卖家",
	}, []tiktok.Shop{
		{ID: "shop-1", Name: "店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-secret", Code: "C1"},
		{ID: "shop-2", Name: "二店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-2", Code: "C2"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.db.SaveTikTokAuth("acct-other", tiktok.Token{
		AccessToken: "other-access-token", RefreshToken: "other-refresh-token", OpenID: "open-2", SellerName: "别人",
	}, []tiktok.Shop{{ID: "shop-9", Name: "别人的店", Region: "ID", SellerType: "LOCAL", Cipher: "cipher-other", Code: "C9"}}); err != nil {
		t.Fatal(err)
	}
	if err := app.db.ReplaceShopVideos("acct-desk", "shop-1", "2026-10-01", "2026-10-08", "2026-10-13", []tiktok.Video{{
		ID: "v1", Title: "面霜", GMVMinor: &gmv, Currency: "USD",
	}}); err != nil {
		t.Fatal(err)
	}
	shops, err := app.ListPlatformShops()
	if err != nil || len(shops) != 2 {
		t.Fatalf("shops %+v %v", shops, err)
	}
	rows, err := app.ListPlatformVideos()
	if err != nil || len(rows) != 1 || rows[0].ShopName != "店" || rows[0].Title != "面霜" || rows[0].Views != nil || rows[0].GMVMinor == nil || *rows[0].GMVMinor != 3824 {
		t.Fatalf("rows %+v %v", rows, err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "plain-access-token") || strings.Contains(string(raw), "cipher-secret") || strings.Contains(string(raw), "别人的店") || strings.Contains(string(raw), "other-access-token") {
		t.Fatalf("desktop list leaked a secret or another account: %s", raw)
	}
}

func TestCreateShopBelongsToTheSignedInAccount(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })

	if _, err := app.CreateShop("店"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	if _, err := app.ListShops(); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("missing account id: %v", err)
	}
	app.who.ID = "acct-desk"
	shop, err := app.CreateShop(" 一店 ")
	if err != nil || shop.Name != "一店" || shop.ID == "" {
		t.Fatalf("create %+v %v", shop, err)
	}
	if _, err := app.CreateShop("一店"); !errors.Is(err, store.ErrShopExists) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := app.CreateShop("二店"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.CreateShop("acct-other", "别人的店"); err != nil {
		t.Fatal(err)
	}
	rows, err := app.ListShops()
	if err != nil || len(rows) != 2 || rows[0].Name != "一店" || rows[1].Name != "二店" {
		t.Fatalf("rows %+v %v", rows, err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "别人的店") || strings.Contains(string(raw), "acct-other") {
		t.Fatalf("list included another account: %s", raw)
	}
}
