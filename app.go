package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/credentials"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/store"
)

// App is the thin Wails binding around the local store and media core.
type App struct {
	ctx      context.Context
	db       *store.Store
	storeErr error
	ai       *ai.Gateway
	aiErr    error
	creds    credentials.Store
	authMu   sync.Mutex
	auth     *auth.Store
	authErr  error
	session  string
	who      auth.Account
}

func dataDir() string {
	if base := os.Getenv("QINGFAN_DATA"); base != "" {
		return base
	}
	return "data"
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.creds == nil {
		a.creds = credentials.Default()
	}
	base := dataDir()
	accounts, err := auth.Open(filepath.Join(base, "auth.sqlite"))
	a.auth = accounts
	a.authErr = err
	db, err := store.Open(filepath.Join(base, "qingfan.sqlite"), filepath.Join(base, "cache"))
	a.db = db
	a.storeErr = err
	if err == nil && db != nil {
		if err := db.RecoverInterrupted(); err != nil {
			a.storeErr = err
		}
	}
	provider, err := ai.ProviderFromEnv()
	if err != nil {
		a.aiErr = err
		return
	}
	gateway, err := ai.Open(filepath.Join(base, "usage.sqlite"), provider, 2)
	a.ai = gateway
	a.aiErr = err
}

func (a *App) shutdown(context.Context) {
	if a.ai != nil {
		a.ai.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
	if a.auth != nil {
		a.auth.Close()
	}
}

func (a *App) ready() error {
	if _, err := a.requireAccount(); err != nil {
		return err
	}
	if a.storeErr != nil {
		return a.storeErr
	}
	if a.db == nil {
		return fmt.Errorf("素材库还没有打开")
	}
	return nil
}

// CheckMediaTools reports whether ffmpeg and ffprobe can start.
func (a *App) CheckMediaTools() []media.ToolStatus {
	if _, err := a.requireAccount(); err != nil {
		return nil
	}
	return media.CheckTools(a.ctx)
}

// CreateProject adds a workspace.
func (a *App) CreateProject(name, market, locale string) (store.Project, error) {
	if err := a.ready(); err != nil {
		return store.Project{}, err
	}
	return a.db.CreateProject(name, market, locale)
}

// CreateSKU adds a product.
func (a *App) CreateSKU(projectID, name, facts string) (store.SKU, error) {
	if err := a.ready(); err != nil {
		return store.SKU{}, err
	}
	return a.db.CreateSKU(projectID, name, facts)
}

// ImportFile probes a local file into a SKU.
func (a *App) ImportFile(skuID, kind, source, rights, path string) (store.ImportResult, error) {
	if err := a.ready(); err != nil {
		return store.ImportResult{}, err
	}
	return a.db.Import(a.ctx, store.ImportRequest{SKUID: skuID, Kind: kind, Source: source, Rights: rights, Path: path})
}

// Relocate points an imported file at its new path.
func (a *App) Relocate(locationID, newPath string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.db.Relocate(locationID, newPath)
}

// ListClips lists shot intervals, including ones that cannot be selected.
func (a *App) ListClips(skuID string) ([]store.Clip, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListClips(skuID)
}

// ListAssets lists voice, BGM, or font assets.
func (a *App) ListAssets(skuID, kind string) ([]store.AssetView, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListAssets(skuID, kind)
}

// AddManualClip adds an operator-defined interval.
func (a *App) AddManualClip(assetID string, inMS, outMS int) (store.Clip, error) {
	if err := a.ready(); err != nil {
		return store.Clip{}, err
	}
	return a.db.AddManualClip(assetID, inMS, outMS)
}

// SaveDraft stores the cards. Existing exports stay unchanged.
func (a *App) SaveDraft(draft store.Draft) (store.Draft, error) {
	if err := a.ready(); err != nil {
		return store.Draft{}, err
	}
	return a.db.SaveDraft(draft)
}

// GetDraft reads the current draft.
func (a *App) GetDraft(id string) (store.Draft, error) {
	if err := a.ready(); err != nil {
		return store.Draft{}, err
	}
	return a.db.GetDraft(id)
}

// PreviewDraft renders the current cards without recording an export.
func (a *App) PreviewDraft(draftID, outputPath string) (media.Manifest, error) {
	if err := a.ready(); err != nil {
		return media.Manifest{}, err
	}
	return a.db.Preview(a.ctx, draftID, outputPath)
}

// ExportDraft renders and records an immutable output.
func (a *App) ExportDraft(draftID, outputPath string) (store.Output, error) {
	if err := a.ready(); err != nil {
		return store.Output{}, err
	}
	return a.db.Export(a.ctx, draftID, outputPath)
}

// ListOutputs lists recorded exports for a draft.
func (a *App) ListOutputs(draftID string) ([]store.Output, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListOutputs(draftID)
}
