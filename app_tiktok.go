package main

import (
	"os"

	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/shopsync"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/tiktok"
)

func (a *App) platform() (auth.Account, *shopsync.Service, error) {
	if err := a.ready(); err != nil {
		return auth.Account{}, nil, err
	}
	account, err := a.requireAccount()
	if err != nil {
		return auth.Account{}, nil, err
	}
	if account.ID == "" {
		return auth.Account{}, nil, auth.ErrUnauthorized
	}
	callback := os.Getenv("QINGFAN_TIKTOK_CALLBACK")
	if callback == "" {
		callback = "http://127.0.0.1:8090/v1/tiktok/callback"
	}
	return account, &shopsync.Service{
		DB:       a.db,
		Client:   tiktok.ClientFromEnv(),
		Callback: callback,
	}, nil
}

// PlatformStatus reports whether the TikTok Shop app is configured. It includes no secrets.
func (a *App) PlatformStatus() (shopsync.Status, error) {
	_, svc, err := a.platform()
	if err != nil {
		return shopsync.Status{}, err
	}
	return svc.Status(), nil
}

// ConnectPlatformShop starts a seller authorization for the signed-in account.
func (a *App) ConnectPlatformShop() (shopsync.ConnectResult, error) {
	account, svc, err := a.platform()
	if err != nil {
		return shopsync.ConnectResult{}, err
	}
	return svc.Connect(a.ctx, account.ID)
}

// CompletePlatformShop writes the shops from one authorization onto the account that started it.
// The page only receives the signed-in account's shops.
func (a *App) CompletePlatformShop(code, state string) ([]store.TikTokShop, error) {
	account, svc, err := a.platform()
	if err != nil {
		return nil, err
	}
	saved, err := svc.Complete(a.ctx, code, state)
	if err != nil {
		return nil, err
	}
	mine, err := a.db.ListTikTokShops(account.ID)
	if err != nil {
		return nil, err
	}
	if len(saved) > 0 && !shopVisible(mine, saved[0].ID) {
		return nil, tiktok.ErrState
	}
	return mine, nil
}

func shopVisible(shops []store.TikTokShop, id string) bool {
	for _, shop := range shops {
		if shop.ID == id {
			return true
		}
	}
	return false
}

// CreateShop stores a shop the signed-in account typed in. It does not call TikTok.
func (a *App) CreateShop(name string) (store.Shop, error) {
	account, _, err := a.platform()
	if err != nil {
		return store.Shop{}, err
	}
	return a.db.CreateShop(account.ID, name)
}

// ListShops returns the shops created by the signed-in account.
func (a *App) ListShops() ([]store.Shop, error) {
	account, _, err := a.platform()
	if err != nil {
		return nil, err
	}
	return a.db.ListShops(account.ID)
}

// ListPlatformShops returns the shops added by the signed-in account.
func (a *App) ListPlatformShops() ([]store.TikTokShop, error) {
	account, _, err := a.platform()
	if err != nil {
		return nil, err
	}
	return a.db.ListTikTokShops(account.ID)
}

// SyncPlatformShop writes one of this account's shop video ranges into the business database.
func (a *App) SyncPlatformShop(shopID, start, end string) (shopsync.SyncResult, error) {
	account, svc, err := a.platform()
	if err != nil {
		return shopsync.SyncResult{}, err
	}
	return svc.Sync(a.ctx, account.ID, shopID, start, end)
}

// ListPlatformVideos reads this account's shop video snapshots from the business database.
func (a *App) ListPlatformVideos() ([]store.TikTokVideo, error) {
	account, _, err := a.platform()
	if err != nil {
		return nil, err
	}
	return a.db.ListTikTokVideos(account.ID, "")
}
