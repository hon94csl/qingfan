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

const (
	RightsUsable        = "usable"
	RightsReferenceOnly = "reference_only"
)

// ImportRequest describes one local file entering a SKU.
type ImportRequest struct {
	SKUID  string
	Kind   string
	Source string
	Rights string
	Path   string
}

// ImportResult is the asset and clips after import.
type ImportResult struct {
	AssetID     string
	LocationID  string
	SHA256      string
	Source      string
	Rights      string
	ReusedProbe bool
	ClipIDs     []string
}

// Clip is one usable or rejected shot interval.
type Clip struct {
	ID         string `json:"id"`
	AssetID    string `json:"asset_id"`
	SKUID      string `json:"sku_id"`
	Label      string `json:"label"`
	Source     string `json:"source"`
	Rights     string `json:"rights"`
	Selectable bool   `json:"selectable"`
	Reason     string `json:"reason,omitempty"`
	InMS       int    `json:"source_in_ms"`
	OutMS      int    `json:"source_out_ms"`
	DurationMS int    `json:"duration_ms"`
	Valid      bool   `json:"valid"`
	Path       string `json:"path,omitempty"`
}

// AssetView is a voice, BGM, or font row for the workbench.
type AssetView struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	SKUID      string `json:"sku_id"`
	Source     string `json:"source"`
	Rights     string `json:"rights"`
	Path       string `json:"path"`
	DurationMS int    `json:"duration_ms"`
	Selectable bool   `json:"selectable"`
	Reason     string `json:"reason,omitempty"`
	SHA256     string `json:"sha256"`
}

type probeRecord struct {
	SHA256    string
	Kind      string
	Info      media.Info
	Thumbnail string
	Cuts      []media.Cut
}

// Import stores a file. Identical bytes reuse the probe.
// Source and rights only become stricter when the same content is seen again.
func (s *Store) Import(ctx context.Context, req ImportRequest) (ImportResult, error) {
	var result ImportResult
	err := jobs.Run(ctx, s, "import", func(ctx context.Context) error {
		var err error
		result, err = s.importFile(ctx, req)
		return err
	})
	return result, err
}

func (s *Store) importFile(ctx context.Context, req ImportRequest) (ImportResult, error) {
	if req.Path == "" || req.SKUID == "" {
		return ImportResult{}, fmt.Errorf("缺少商品或文件路径")
	}
	sum, err := media.Hash(req.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ImportResult{}, ErrMissing
		}
		return ImportResult{}, err
	}
	abs, err := filepath.Abs(req.Path)
	if err != nil {
		return ImportResult{}, err
	}
	source, rights, err := normalizeDeclared(req.Source, req.Rights)
	if err != nil {
		return ImportResult{}, err
	}
	cached, haveProbe, err := s.loadProbe(sum)
	if err != nil {
		return ImportResult{}, err
	}
	var prior *media.CachedProbe
	if haveProbe {
		prior = &media.CachedProbe{SHA256: sum, Info: cached.Info, Cuts: cached.Cuts, ThumbnailPath: cached.Thumbnail}
	}
	ingested, err := s.ingest(ctx, abs, req.Kind, s.thumbDir(), prior)
	if err != nil {
		return ImportResult{}, err
	}
	if ingested.SHA256 != sum {
		return ImportResult{}, fmt.Errorf("探测哈希和文件不一致")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback()
	if !haveProbe {
		if err := insertProbe(tx, req.Kind, ingested); err != nil {
			return ImportResult{}, err
		}
	}
	locID, locAsset, locSum, found, err := locationAt(tx, abs)
	if err != nil {
		return ImportResult{}, err
	}
	if found && locSum != sum {
		if err := detachReplaced(tx, locID, locAsset); err != nil {
			return ImportResult{}, err
		}
	} else if found {
		var haveSKU, haveKind string
		if err := tx.QueryRow(`SELECT sku_id, kind FROM assets WHERE id=?`, locAsset).Scan(&haveSKU, &haveKind); err != nil {
			return ImportResult{}, err
		}
		if haveSKU != req.SKUID || haveKind != req.Kind {
			return ImportResult{}, fmt.Errorf("这个路径已经导入到其他商品")
		}
	}
	assetID, source, rights, err := s.upsertAsset(tx, req.SKUID, req.Kind, sum, source, rights)
	if err != nil {
		return ImportResult{}, err
	}
	if found {
		if _, err := tx.Exec(`UPDATE asset_locations SET asset_id=?, sha256=? WHERE id=?`, assetID, sum, locID); err != nil {
			return ImportResult{}, err
		}
	} else {
		locID, err = newID()
		if err != nil {
			return ImportResult{}, err
		}
		if _, err := tx.Exec(`INSERT INTO asset_locations(id, asset_id, path, sha256) VALUES (?, ?, ?, ?)`, locID, assetID, abs, sum); err != nil {
			return ImportResult{}, err
		}
	}
	clipIDs, err := ensureClips(tx, assetID, req.SKUID, req.Kind, ingested.Cuts)
	if err != nil {
		return ImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		AssetID: assetID, LocationID: locID, SHA256: sum,
		Source: source, Rights: rights, ReusedProbe: ingested.Reused, ClipIDs: clipIDs,
	}, nil
}

func normalizeDeclared(source, rights string) (string, string, error) {
	if _, err := sourceRank(source); err != nil {
		return "", "", err
	}
	if _, err := rightsRank(rights); err != nil {
		return "", "", err
	}
	if source == timeline.SourceReference {
		rights = RightsReferenceOnly
	}
	return source, rights, nil
}

func (s *Store) upsertAsset(tx *sql.Tx, sku, kind, sum, source, rights string) (string, string, string, error) {
	var id, haveSource, haveRights string
	err := tx.QueryRow(`SELECT id, source, rights FROM assets WHERE tenant_id=? AND sku_id=? AND kind=? AND sha256=? AND analysis_valid=1`, tenantID, sku, kind, sum).Scan(&id, &haveSource, &haveRights)
	if errors.Is(err, sql.ErrNoRows) {
		id, err = newID()
		if err != nil {
			return "", "", "", err
		}
		_, err = tx.Exec(`INSERT INTO assets(id, tenant_id, sku_id, kind, source, rights, sha256, analysis_valid) VALUES (?, ?, ?, ?, ?, ?, ?, 1)`, id, tenantID, sku, kind, source, rights, sum)
		return id, source, rights, err
	}
	if err != nil {
		return "", "", "", err
	}
	source, err = stricterSource(haveSource, source)
	if err != nil {
		return "", "", "", err
	}
	rights, err = stricterRights(source, haveRights, rights)
	if err != nil {
		return "", "", "", err
	}
	_, err = tx.Exec(`UPDATE assets SET source=?, rights=? WHERE id=?`, source, rights, id)
	return id, source, rights, err
}

func stricterSource(a, b string) (string, error) {
	ra, err := sourceRank(a)
	if err != nil {
		return "", err
	}
	rb, err := sourceRank(b)
	if err != nil {
		return "", err
	}
	if ra <= rb {
		return a, nil
	}
	return b, nil
}

func stricterRights(source, a, b string) (string, error) {
	if source == timeline.SourceReference {
		return RightsReferenceOnly, nil
	}
	ra, err := rightsRank(a)
	if err != nil {
		return "", err
	}
	rb, err := rightsRank(b)
	if err != nil {
		return "", err
	}
	if ra <= rb {
		return a, nil
	}
	return b, nil
}

func sourceRank(source string) (int, error) {
	switch source {
	case timeline.SourceReference:
		return 0, nil
	case timeline.SourceLibrary:
		return 1, nil
	case timeline.SourceOwned:
		return 2, nil
	default:
		return 0, fmt.Errorf("不认识的来源 %q", source)
	}
}

func rightsRank(rights string) (int, error) {
	switch rights {
	case RightsReferenceOnly:
		return 0, nil
	case RightsUsable:
		return 1, nil
	default:
		return 0, fmt.Errorf("不认识的使用权 %q", rights)
	}
}

func insertProbe(tx *sql.Tx, kind string, in media.Ingested) error {
	cuts, err := json.Marshal(in.Cuts)
	if err != nil {
		return err
	}
	info := in.Info
	_, err = tx.Exec(`INSERT INTO content_probes(sha256, kind, duration_ms, width, height, has_video, has_audio, rotation, variable_frame_rate, thumbnail_path, cuts_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.SHA256, kind, info.DurationMS, info.Width, info.Height, boolInt(info.HasVideo), boolInt(info.HasAudio), info.Rotation, boolInt(info.VariableFrameRate), in.ThumbnailPath, string(cuts))
	return err
}

func (s *Store) loadProbe(sum string) (probeRecord, bool, error) {
	var rec probeRecord
	var cuts string
	var video, audio, vfr int
	err := s.db.QueryRow(`SELECT sha256, kind, duration_ms, width, height, has_video, has_audio, rotation, variable_frame_rate, thumbnail_path, cuts_json FROM content_probes WHERE sha256=?`, sum).Scan(
		&rec.SHA256, &rec.Kind, &rec.Info.DurationMS, &rec.Info.Width, &rec.Info.Height, &video, &audio, &rec.Info.Rotation, &vfr, &rec.Thumbnail, &cuts)
	if errors.Is(err, sql.ErrNoRows) {
		return probeRecord{}, false, nil
	}
	if err != nil {
		return probeRecord{}, false, err
	}
	rec.Info.HasVideo = video == 1
	rec.Info.HasAudio = audio == 1
	rec.Info.VariableFrameRate = vfr == 1
	if cuts != "" && cuts != "null" {
		if err := json.Unmarshal([]byte(cuts), &rec.Cuts); err != nil {
			return probeRecord{}, false, err
		}
	}
	return rec, true, nil
}

func locationAt(tx *sql.Tx, path string) (id, assetID, sum string, found bool, err error) {
	err = tx.QueryRow(`SELECT id, asset_id, sha256 FROM asset_locations WHERE path=?`, path).Scan(&id, &assetID, &sum)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, err
	}
	return id, assetID, sum, true, nil
}

func detachReplaced(tx *sql.Tx, locationID, assetID string) error {
	var others int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM asset_locations WHERE asset_id=? AND id<>?`, assetID, locationID).Scan(&others); err != nil {
		return err
	}
	if others == 0 {
		if _, err := tx.Exec(`UPDATE assets SET analysis_valid=0 WHERE id=?`, assetID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE clips SET valid=0 WHERE asset_id=?`, assetID); err != nil {
			return err
		}
	}
	return nil
}

func ensureClips(tx *sql.Tx, assetID, sku, kind string, cuts []media.Cut) ([]string, error) {
	rows, err := tx.Query(`SELECT id FROM clips WHERE asset_id=? AND valid=1 ORDER BY source_in_ms, id`, assetID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 || kind != timeline.KindClip {
		return ids, nil
	}
	for _, cut := range cuts {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO clips(id, asset_id, sku_id, source_in_ms, source_out_ms, origin, valid) VALUES (?, ?, ?, ?, ?, 'detected', 1)`, id, assetID, sku, cut.InMS, cut.OutMS); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Relocate points a location at a moved file after the bytes match.
func (s *Store) Relocate(locationID, newPath string) error {
	sum, err := media.Hash(newPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrMissing
		}
		return err
	}
	abs, err := filepath.Abs(newPath)
	if err != nil {
		return err
	}
	var have string
	if err := s.db.QueryRow(`SELECT sha256 FROM asset_locations WHERE id=?`, locationID).Scan(&have); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMissing
		}
		return err
	}
	if sum != have {
		return ErrHash
	}
	var other string
	err = s.db.QueryRow(`SELECT id FROM asset_locations WHERE path=? AND id<>?`, abs, locationID).Scan(&other)
	if err == nil {
		return fmt.Errorf("这个路径已经登记过")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.db.Exec(`UPDATE asset_locations SET path=? WHERE id=?`, abs, locationID)
	return err
}

// AddManualClip divides a clip when scene detection found no cut.
func (s *Store) AddManualClip(assetID string, inMS, outMS int) (Clip, error) {
	if err := s.rangeInside(assetID, inMS, outMS); err != nil {
		return Clip{}, err
	}
	var sku string
	if err := s.db.QueryRow(`SELECT sku_id FROM assets WHERE id=? AND kind=? AND analysis_valid=1`, assetID, timeline.KindClip).Scan(&sku); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Clip{}, ErrStale
		}
		return Clip{}, err
	}
	id, err := newID()
	if err != nil {
		return Clip{}, err
	}
	_, err = s.db.Exec(`INSERT INTO clips(id, asset_id, sku_id, source_in_ms, source_out_ms, origin, valid) VALUES (?, ?, ?, ?, ?, 'manual', 1)`, id, assetID, sku, inMS, outMS)
	if err != nil {
		return Clip{}, err
	}
	clips, err := s.ListClips(sku)
	if err != nil {
		return Clip{}, err
	}
	for _, clip := range clips {
		if clip.ID == id {
			return clip, nil
		}
	}
	return Clip{}, fmt.Errorf("镜头没有写入")
}

func (s *Store) rangeInside(assetID string, inMS, outMS int) error {
	if inMS < 0 || outMS <= inMS {
		return ErrRange
	}
	var duration int
	err := s.db.QueryRow(`SELECT p.duration_ms FROM assets a JOIN content_probes p ON p.sha256=a.sha256 WHERE a.id=?`, assetID).Scan(&duration)
	if err != nil {
		return err
	}
	if outMS > duration {
		return ErrRange
	}
	return nil
}

// ListClips returns shot intervals for one SKU, including those that cannot be used.
func (s *Store) ListClips(skuID string) ([]Clip, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.asset_id, c.sku_id, c.source_in_ms, c.source_out_ms, c.valid,
		       a.source, a.rights, a.analysis_valid, p.duration_ms
		FROM clips c
		JOIN assets a ON a.id = c.asset_id
		JOIN content_probes p ON p.sha256 = a.sha256
		WHERE c.sku_id=?
		ORDER BY c.source_in_ms, c.id`, skuID)
	if err != nil {
		return nil, err
	}
	var clips []Clip
	for rows.Next() {
		var clip Clip
		var valid, analysis int
		if err := rows.Scan(&clip.ID, &clip.AssetID, &clip.SKUID, &clip.InMS, &clip.OutMS, &valid, &clip.Source, &clip.Rights, &analysis, &clip.DurationMS); err != nil {
			rows.Close()
			return nil, err
		}
		clip.Valid = valid == 1 && analysis == 1
		clips = append(clips, clip)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range clips {
		path, err := s.onePath(clips[i].AssetID)
		if err != nil {
			return nil, err
		}
		clips[i].Path = path
		clips[i].Label = filepath.Base(path)
		clips[i].Selectable, clips[i].Reason = selectable(clips[i].Valid, clips[i].Source, clips[i].Rights, path != "")
	}
	return clips, nil
}

// ListAssets returns non-clip assets for a SKU.
func (s *Store) ListAssets(skuID, kind string) ([]AssetView, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.kind, a.sku_id, a.source, a.rights, a.sha256, a.analysis_valid, p.duration_ms
		FROM assets a
		JOIN content_probes p ON p.sha256=a.sha256
		WHERE a.sku_id=? AND a.kind=?
		ORDER BY a.id`, skuID, kind)
	if err != nil {
		return nil, err
	}
	var assets []AssetView
	var analysisFlags []bool
	for rows.Next() {
		var asset AssetView
		var analysis int
		if err := rows.Scan(&asset.ID, &asset.Kind, &asset.SKUID, &asset.Source, &asset.Rights, &asset.SHA256, &analysis, &asset.DurationMS); err != nil {
			rows.Close()
			return nil, err
		}
		assets = append(assets, asset)
		analysisFlags = append(analysisFlags, analysis == 1)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range assets {
		path, err := s.onePath(assets[i].ID)
		if err != nil {
			return nil, err
		}
		assets[i].Path = path
		assets[i].Selectable, assets[i].Reason = selectable(analysisFlags[i], assets[i].Source, assets[i].Rights, path != "")
	}
	return assets, nil
}

func (s *Store) onePath(assetID string) (string, error) {
	var path string
	err := s.db.QueryRow(`SELECT path FROM asset_locations WHERE asset_id=? ORDER BY path LIMIT 1`, assetID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return path, err
}

func selectable(valid bool, source, rights string, hasPath bool) (bool, string) {
	if !valid {
		return false, "原分析已失效"
	}
	if source == timeline.SourceReference || rights == RightsReferenceOnly {
		return false, "参考素材不能放进成片"
	}
	if !hasPath {
		return false, "找不到文件，请重新定位"
	}
	return true, ""
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
