package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/timeline"
)

// Project is one editing workspace.
type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Market string `json:"market"`
	Locale string `json:"locale"`
}

// SKU is the product a draft is cut for.
type SKU struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Facts     string `json:"facts"`
}

// DraftShot selects a stored clip interval.
// ClipID is the clips row, not the asset id.
type DraftShot struct {
	ClipID      string `json:"clip_id"`
	SourceInMS  int    `json:"source_in_ms"`
	SourceOutMS int    `json:"source_out_ms"`
}

// DraftSegment is one card.
type DraftSegment struct {
	ID           string             `json:"id"`
	Role         string             `json:"role"`
	VoiceText    string             `json:"voice_text"`
	VoiceAssetID string             `json:"voice_asset_id"`
	Shots        []DraftShot        `json:"shots"`
	Captions     []timeline.Caption `json:"captions"`
}

// Draft is editable. Exported manifests are not.
type Draft struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	SKUID       string         `json:"sku_id"`
	Revision    int            `json:"revision"`
	Market      string         `json:"market"`
	Locale      string         `json:"locale"`
	FontAssetID string         `json:"font_asset_id"`
	BGMAssetID  string         `json:"bgm_asset_id"`
	BGMGainDB   float64        `json:"bgm_gain_db"`
	Segments    []DraftSegment `json:"segments"`
}

// Output is one immutable export.
type Output struct {
	ID            string `json:"id"`
	DraftID       string `json:"draft_id"`
	DraftRevision int    `json:"draft_revision"`
	Path          string `json:"path"`
	ManifestJSON  string `json:"manifest_json"`
	CreatedAt     string `json:"created_at"`
}

// CreateProject inserts a project.
func (s *Store) CreateProject(name, market, locale string) (Project, error) {
	if name == "" || market == "" || locale == "" {
		return Project{}, fmt.Errorf("项目名称、市场和语言都不能空")
	}
	id, err := newID()
	if err != nil {
		return Project{}, err
	}
	_, err = s.db.Exec(`INSERT INTO projects(id, tenant_id, name, market, locale) VALUES (?, ?, ?, ?, ?)`, id, tenantID, name, market, locale)
	return Project{ID: id, Name: name, Market: market, Locale: locale}, err
}

// CreateSKU inserts a product under a project.
func (s *Store) CreateSKU(projectID, name, facts string) (SKU, error) {
	if name == "" {
		return SKU{}, fmt.Errorf("商品名称不能空")
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id=?`, projectID).Scan(&n); err != nil {
		return SKU{}, err
	}
	if n == 0 {
		return SKU{}, fmt.Errorf("项目不存在")
	}
	id, err := newID()
	if err != nil {
		return SKU{}, err
	}
	_, err = s.db.Exec(`INSERT INTO skus(id, project_id, name, facts) VALUES (?, ?, ?, ?)`, id, projectID, name, facts)
	return SKU{ID: id, ProjectID: projectID, Name: name, Facts: facts}, err
}

// SaveDraft inserts or updates a draft and stores manual interval corrections.
// It does not change output rows.
func (s *Store) SaveDraft(draft Draft) (Draft, error) {
	if err := s.validateDraft(draft); err != nil {
		return Draft{}, err
	}
	body, err := json.Marshal(struct {
		Segments []DraftSegment `json:"segments"`
	}{Segments: draft.Segments})
	if err != nil {
		return Draft{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback()
	if draft.ID == "" {
		draft.ID, err = newID()
		if err != nil {
			return Draft{}, err
		}
		draft.Revision = 1
		_, err = tx.Exec(`INSERT INTO drafts(id, project_id, sku_id, revision, market, locale, font_asset_id, bgm_asset_id, bgm_gain_db, body_json, updated_at) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?)`,
			draft.ID, draft.ProjectID, draft.SKUID, draft.Market, draft.Locale, draft.FontAssetID, draft.BGMAssetID, draft.BGMGainDB, string(body), nowText())
	} else {
		var project, sku string
		var revision int
		err = tx.QueryRow(`SELECT project_id, sku_id, revision FROM drafts WHERE id=?`, draft.ID).Scan(&project, &sku, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			return Draft{}, fmt.Errorf("草稿不存在")
		}
		if err != nil {
			return Draft{}, err
		}
		if project != draft.ProjectID || sku != draft.SKUID {
			return Draft{}, fmt.Errorf("不能把草稿改到别的商品")
		}
		draft.Revision = revision + 1
		_, err = tx.Exec(`UPDATE drafts SET revision=?, market=?, locale=?, font_asset_id=?, bgm_asset_id=?, bgm_gain_db=?, body_json=?, updated_at=? WHERE id=?`,
			draft.Revision, draft.Market, draft.Locale, draft.FontAssetID, draft.BGMAssetID, draft.BGMGainDB, string(body), nowText(), draft.ID)
	}
	if err != nil {
		return Draft{}, err
	}
	for _, seg := range draft.Segments {
		for _, shot := range seg.Shots {
			_, err = tx.Exec(`UPDATE clips SET source_in_ms=?, source_out_ms=?, origin='manual' WHERE id=? AND sku_id=? AND valid=1`, shot.SourceInMS, shot.SourceOutMS, shot.ClipID, draft.SKUID)
			if err != nil {
				return Draft{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return Draft{}, err
	}
	return draft, nil
}

// GetDraft reads the current draft.
func (s *Store) GetDraft(id string) (Draft, error) {
	var draft Draft
	var body string
	err := s.db.QueryRow(`SELECT id, project_id, sku_id, revision, market, locale, font_asset_id, bgm_asset_id, bgm_gain_db, body_json FROM drafts WHERE id=?`, id).Scan(
		&draft.ID, &draft.ProjectID, &draft.SKUID, &draft.Revision, &draft.Market, &draft.Locale, &draft.FontAssetID, &draft.BGMAssetID, &draft.BGMGainDB, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, fmt.Errorf("草稿不存在")
	}
	if err != nil {
		return Draft{}, err
	}
	var wrapped struct {
		Segments []DraftSegment `json:"segments"`
	}
	if err := json.Unmarshal([]byte(body), &wrapped); err != nil {
		return Draft{}, err
	}
	draft.Segments = wrapped.Segments
	return draft, nil
}

func (s *Store) validateDraft(draft Draft) error {
	if draft.ProjectID == "" || draft.SKUID == "" || draft.Market == "" || draft.Locale == "" {
		return fmt.Errorf("草稿缺少项目、商品或市场")
	}
	if len(draft.Segments) == 0 {
		return fmt.Errorf("至少要有一段")
	}
	if _, err := s.resolveCatalogAsset(draft.FontAssetID, timeline.KindFont, draft.SKUID, false); err != nil {
		return err
	}
	if _, err := s.resolveCatalogAsset(draft.BGMAssetID, timeline.KindBGM, draft.SKUID, false); err != nil {
		return err
	}
	for _, seg := range draft.Segments {
		if seg.ID == "" {
			return fmt.Errorf("分段缺少编号")
		}
		if len(seg.Shots) == 0 {
			return fmt.Errorf("分段 %s 没有镜头", seg.ID)
		}
		if len(seg.Captions) == 0 {
			return fmt.Errorf("分段 %s 缺少字幕", seg.ID)
		}
		duration := 0
		for _, shot := range seg.Shots {
			if _, err := s.resolveClip(draft.SKUID, shot.ClipID, shot.SourceInMS, shot.SourceOutMS); err != nil {
				return err
			}
			duration += shot.SourceOutMS - shot.SourceInMS
		}
		voice, err := s.resolveCatalogAsset(seg.VoiceAssetID, timeline.KindVoice, draft.SKUID, true)
		if err != nil {
			return err
		}
		if voice.DurationMS > duration {
			return ErrVoice
		}
		for _, caption := range seg.Captions {
			if caption.Text == "" || caption.StartMS < 0 || caption.EndMS <= caption.StartMS || caption.EndMS > duration {
				return ErrCaption
			}
		}
	}
	return nil
}

func (s *Store) resolveClip(sku, clipID string, inMS, outMS int) (string, error) {
	var assetID, clipSKU, source, rights string
	var valid, analysis, duration int
	err := s.db.QueryRow(`
		SELECT c.asset_id, c.sku_id, c.valid, a.source, a.rights, a.analysis_valid, p.duration_ms
		FROM clips c
		JOIN assets a ON a.id=c.asset_id
		JOIN content_probes p ON p.sha256=a.sha256
		WHERE c.id=?`, clipID).Scan(&assetID, &clipSKU, &valid, &source, &rights, &analysis, &duration)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrStale
	}
	if err != nil {
		return "", err
	}
	if clipSKU != sku {
		return "", ErrSKU
	}
	if valid != 1 || analysis != 1 {
		return "", ErrStale
	}
	if source == timeline.SourceReference || rights == RightsReferenceOnly {
		return "", ErrReference
	}
	if inMS < 0 || outMS <= inMS || outMS > duration {
		return "", ErrRange
	}
	return assetID, nil
}

func (s *Store) resolveCatalogAsset(id, kind, sku string, skuMustMatch bool) (timeline.Asset, error) {
	var asset timeline.Asset
	var analysis int
	var rights string
	err := s.db.QueryRow(`
		SELECT a.id, a.kind, a.sku_id, a.source, a.sha256, a.analysis_valid, a.rights, p.duration_ms
		FROM assets a JOIN content_probes p ON p.sha256=a.sha256 WHERE a.id=?`, id).Scan(
		&asset.ID, &asset.Kind, &asset.SKUID, &asset.Source, &asset.SHA256, &analysis, &rights, &asset.DurationMS)
	if errors.Is(err, sql.ErrNoRows) {
		return timeline.Asset{}, ErrMissing
	}
	if err != nil {
		return timeline.Asset{}, err
	}
	if asset.Kind != kind {
		return timeline.Asset{}, fmt.Errorf("素材 %s 不是%s", id, kind)
	}
	if analysis != 1 {
		return timeline.Asset{}, ErrStale
	}
	if skuMustMatch && asset.SKUID != sku {
		return timeline.Asset{}, ErrSKU
	}
	if !skuMustMatch && asset.SKUID != "" && asset.SKUID != sku {
		return timeline.Asset{}, ErrSKU
	}
	if asset.Source == timeline.SourceReference || rights == RightsReferenceOnly {
		return timeline.Asset{}, ErrReference
	}
	path, err := s.onePath(id)
	if err != nil {
		return timeline.Asset{}, err
	}
	if path == "" {
		return timeline.Asset{}, ErrMissing
	}
	sum, err := media.Hash(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return timeline.Asset{}, ErrMissing
		}
		return timeline.Asset{}, err
	}
	if sum != asset.SHA256 {
		return timeline.Asset{}, ErrStale
	}
	asset.Path = path
	return asset, nil
}

func (s *Store) build(draftID string) (timeline.Timeline, []timeline.Asset, Draft, error) {
	draft, err := s.GetDraft(draftID)
	if err != nil {
		return timeline.Timeline{}, nil, Draft{}, err
	}
	if err := s.validateDraft(draft); err != nil {
		return timeline.Timeline{}, nil, Draft{}, err
	}
	byID := map[string]timeline.Asset{}
	add := func(asset timeline.Asset) {
		byID[asset.ID] = asset
	}
	font, err := s.resolveCatalogAsset(draft.FontAssetID, timeline.KindFont, draft.SKUID, false)
	if err != nil {
		return timeline.Timeline{}, nil, Draft{}, err
	}
	bgm, err := s.resolveCatalogAsset(draft.BGMAssetID, timeline.KindBGM, draft.SKUID, false)
	if err != nil {
		return timeline.Timeline{}, nil, Draft{}, err
	}
	add(font)
	add(bgm)
	tl := timeline.Timeline{
		SchemaVersion: timeline.SchemaVersion,
		OutputID:      draft.ID,
		Revision:      draft.Revision,
		SKUID:         draft.SKUID,
		Market:        draft.Market,
		Locale:        draft.Locale,
		Canvas:        timeline.Canvas{Width: 1080, Height: 1920, FPS: 30},
		FontAssetID:   font.ID,
		Recipe:        timeline.Recipe{HookType: "manual", PromptVersion: "workbench-m2"},
		BGM:           timeline.BGM{AssetID: bgm.ID, GainDB: draft.BGMGainDB},
	}
	for _, seg := range draft.Segments {
		voice, err := s.resolveCatalogAsset(seg.VoiceAssetID, timeline.KindVoice, draft.SKUID, true)
		if err != nil {
			return timeline.Timeline{}, nil, Draft{}, err
		}
		add(voice)
		out := timeline.Segment{
			ID: seg.ID, Role: seg.Role, Voice: timeline.Voice{Text: seg.VoiceText, AudioID: voice.ID, DurationMS: voice.DurationMS},
			OriginalAudio: timeline.OriginalMute, Captions: seg.Captions, CaptionStyle: "id-default",
		}
		if out.Role == "" {
			out.Role = "body"
		}
		for _, shot := range seg.Shots {
			assetID, err := s.resolveClip(draft.SKUID, shot.ClipID, shot.SourceInMS, shot.SourceOutMS)
			if err != nil {
				return timeline.Timeline{}, nil, Draft{}, err
			}
			clipAsset, err := s.resolveCatalogAsset(assetID, timeline.KindClip, draft.SKUID, true)
			if err != nil {
				return timeline.Timeline{}, nil, Draft{}, err
			}
			add(clipAsset)
			out.Shots = append(out.Shots, timeline.Shot{ClipID: assetID, SourceInMS: shot.SourceInMS, SourceOutMS: shot.SourceOutMS})
			out.DurationMS += shot.SourceOutMS - shot.SourceInMS
		}
		tl.Segments = append(tl.Segments, out)
	}
	assets := make([]timeline.Asset, 0, len(byID))
	for _, asset := range byID {
		assets = append(assets, asset)
	}
	if err := timeline.Validate(tl, assets, timeline.Check{}); err != nil {
		return timeline.Timeline{}, nil, Draft{}, err
	}
	return tl, assets, draft, nil
}

// Preview renders the current draft without recording an output.
func (s *Store) Preview(ctx context.Context, draftID, outputPath string) (media.Manifest, error) {
	var manifest media.Manifest
	err := jobs.Run(ctx, s, "preview", func(ctx context.Context) error {
		abs, err := filepath.Abs(outputPath)
		if err != nil {
			return err
		}
		if err := s.rejectRecordedPath(abs); err != nil {
			return err
		}
		tl, assets, _, err := s.build(draftID)
		if err != nil {
			return err
		}
		manifest, err = media.Render(ctx, tl, assets, abs)
		return err
	})
	return manifest, err
}

// Export renders and records an immutable output.
// An existing file or a previously exported path is left untouched.
func (s *Store) Export(ctx context.Context, draftID, outputPath string) (Output, error) {
	var out Output
	err := jobs.Run(ctx, s, "export", func(ctx context.Context) error {
		var err error
		out, err = s.export(ctx, draftID, outputPath)
		return err
	})
	return out, err
}

func (s *Store) export(ctx context.Context, draftID, outputPath string) (Output, error) {
	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return Output{}, err
	}
	if err := s.rejectRecordedPath(abs); err != nil {
		return Output{}, err
	}
	if _, err := os.Stat(abs); err == nil {
		return Output{}, ErrOutputExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return Output{}, err
	}
	tl, assets, draft, err := s.build(draftID)
	if err != nil {
		return Output{}, err
	}
	id, err := newID()
	if err != nil {
		return Output{}, err
	}
	tl.OutputID = id
	manifest, err := media.Render(ctx, tl, assets, abs)
	if err != nil {
		return Output{}, err
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return Output{}, err
	}
	created := nowText()
	_, err = s.db.Exec(`INSERT INTO outputs(id, draft_id, draft_revision, path, manifest_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, draft.ID, draft.Revision, abs, string(body), created)
	if err != nil {
		return Output{}, err
	}
	return Output{ID: id, DraftID: draft.ID, DraftRevision: draft.Revision, Path: abs, ManifestJSON: string(body), CreatedAt: created}, nil
}

func (s *Store) rejectRecordedPath(path string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM outputs WHERE path=?`, path).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrOutputExists
	}
	return nil
}

// ListOutputs returns exports for a draft, oldest first.
func (s *Store) ListOutputs(draftID string) ([]Output, error) {
	rows, err := s.db.Query(`SELECT id, draft_id, draft_revision, path, manifest_json, created_at FROM outputs WHERE draft_id=? ORDER BY created_at, id`, draftID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var outputs []Output
	for rows.Next() {
		var out Output
		if err := rows.Scan(&out.ID, &out.DraftID, &out.DraftRevision, &out.Path, &out.ManifestJSON, &out.CreatedAt); err != nil {
			return nil, err
		}
		outputs = append(outputs, out)
	}
	return outputs, rows.Err()
}
