package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ProducedVideo is one exported file. The manifest stays in the database.
type ProducedVideo struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// ProduceResult is one automatic run from the stored facts to exported files.
type ProduceResult struct {
	Candidates   int             `json:"candidates"`
	Outputs      []ProducedVideo `json:"outputs"`
	Fetched      []FetchedClip   `json:"fetched,omitempty"`
	KnownMicros  int64           `json:"known_micros"`
	UnknownCalls int             `json:"unknown_calls"`
	Note         string          `json:"note"`
}

type savedCut struct {
	CandidateID string           `json:"candidate_id"`
	Print       jobs.Fingerprint `json:"print"`
	Output      ProducedVideo    `json:"output"`
}

type cutRecord struct {
	AnalysisKey string         `json:"analysis_key"`
	Candidates  []ai.Candidate `json:"candidates"`
	Cuts        []savedCut     `json:"cuts"`
}

func (r cutRecord) cut(id string) (savedCut, bool) {
	for _, cut := range r.Cuts {
		if cut.CandidateID == id {
			return cut, true
		}
	}
	return savedCut{}, false
}

// PickMaterial opens the desktop file dialog. Cancelling returns an empty path.
func (a *App) PickMaterial() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	// Wails calls log.Fatal when this context has no desktop frontend.
	if a.ctx == nil || a.ctx.Value("frontend") == nil {
		return "", fmt.Errorf("选择文件需要在桌面应用里打开")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择素材",
		Filters: []runtime.FileFilter{{
			DisplayName: "视频、音频和字体",
			Pattern:     "*.mp4;*.mov;*.m4v;*.mkv;*.webm;*.wav;*.mp3;*.m4a;*.aac;*.ttf;*.otf",
		}},
	})
	if err != nil {
		return "", fmt.Errorf("选择文件需要在桌面应用里打开")
	}
	return path, nil
}

// ProduceVideo generates the candidates, synthesizes each one, and exports a new file per candidate.
// clipAssetIDs limits the footage to the checked clip files. An empty list uses every clip of the product.
// A non-empty prompt replaces the stored product facts before generation.
// A repeat with the same facts, shots, voice, font, and BGM reuses the files.
// Changing BGM or the font remixes without calling analysis or TTS again.
// It does not replace an existing export. A failed candidate stops the run after the files already written.
func (a *App) ProduceVideo(projectID, skuID, fontID, bgmID string, clipAssetIDs []string, prompt string) (ProduceResult, error) {
	if err := a.aiReady(); err != nil {
		return ProduceResult{}, err
	}
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(skuID) == "" {
		return ProduceResult{}, fmt.Errorf("先填写商品")
	}
	if strings.TrimSpace(fontID) == "" || strings.TrimSpace(bgmID) == "" {
		return ProduceResult{}, fmt.Errorf("先选择字体和 BGM")
	}
	if strings.TrimSpace(prompt) != "" {
		if err := a.db.UpdateFacts(skuID, strings.TrimSpace(prompt)); err != nil {
			return ProduceResult{}, err
		}
	}
	sku, err := a.db.GetSKU(skuID)
	if err != nil {
		return ProduceResult{}, err
	}
	clips, err := a.db.ListClips(skuID)
	if err != nil {
		return ProduceResult{}, err
	}
	clips, err = keepClipAssets(clips, clipAssetIDs)
	if err != nil {
		return ProduceResult{}, err
	}
	prev, err := loadCut(skuID)
	if err != nil {
		return ProduceResult{}, err
	}
	analysis := analysisKey(sku.Facts, clips)
	candidates := prev.Candidates
	if prev.AnalysisKey != analysis || len(candidates) == 0 {
		gen, err := a.generateScripts(skuID, "", clips)
		if err != nil {
			return ProduceResult{}, err
		}
		candidates = gen.Candidates
	}
	if len(candidates) == 0 {
		return ProduceResult{}, fmt.Errorf("这次没有生成口播")
	}
	voice := ttsVoice("")
	encoder := media.Resolved()
	renderKey := jobs.Key(media.RendererVersion, encoder.Name, encoder.Config)
	result := ProduceResult{Candidates: len(candidates), Outputs: []ProducedVideo{}}
	next := cutRecord{AnalysisKey: analysis, Candidates: candidates}
	reusedAll := true
	redidVoice := false
	redidAnalysis := prev.AnalysisKey != analysis || len(prev.Candidates) == 0
	dir := filepath.Join(dataDir(), "exports", skuID)
	for i, candidate := range candidates {
		key := candidateKey(candidates, i)
		var before jobs.Fingerprint
		var old ProducedVideo
		if cut, ok := prev.cut(key); ok {
			before = cut.Print
			old = cut.Output
		}
		redo := jobs.Diff(before, cutFingerprint(analysis, renderKey, fontID, bgmID, voice, clips, candidate))
		if reuseExport(redo, old.Path) {
			result.Outputs = append(result.Outputs, old)
			next.Cuts = append(next.Cuts, savedCut{CandidateID: key, Print: before, Output: old})
			continue
		}
		reusedAll = false
		if len(redo.TTS) > 0 {
			redidVoice = true
		}
		draft, err := a.assembleCandidate(projectID, skuID, fontID, bgmID, "", candidate, clips)
		if err != nil {
			_ = saveCut(skuID, next)
			return result, produceStop(len(result.Outputs), i+1, err)
		}
		path := filepath.Join(dir, fmt.Sprintf("%s-%02d.mp4", draft.ID, i+1))
		exported, err := a.ExportDraft(draft.ID, path)
		if err != nil {
			_ = saveCut(skuID, next)
			return result, produceStop(len(result.Outputs), i+1, err)
		}
		item := ProducedVideo{ID: exported.ID, Path: exported.Path}
		result.Outputs = append(result.Outputs, item)
		afterClips, err := a.db.ListClips(skuID)
		if err != nil {
			return result, err
		}
		afterClips, err = keepClipAssets(afterClips, clipAssetIDs)
		if err != nil {
			return result, err
		}
		clips = afterClips
		next.Cuts = append(next.Cuts, savedCut{
			CandidateID: key,
			Print:       cutFingerprint(analysis, renderKey, fontID, bgmID, voice, clips, candidate),
			Output:      item,
		})
	}
	if err := saveCut(skuID, next); err != nil {
		result.Note = "成片已导出。出片记录没有写上，下次会重做。"
		return result, nil
	}
	return a.finishProduce(result, reusedAll, redidVoice, redidAnalysis)
}

func (a *App) finishProduce(result ProduceResult, reusedAll, redidVoice, redidAnalysis bool) (ProduceResult, error) {
	plain, err := a.token()
	if err != nil {
		result.Note = produceNote(reusedAll, redidVoice, redidAnalysis, false)
		return result, nil
	}
	report, err := a.ai.Costs(plain)
	if err != nil {
		result.Note = produceNote(reusedAll, redidVoice, redidAnalysis, false)
		return result, nil
	}
	result.KnownMicros = report.KnownMicros
	result.UnknownCalls = report.UnknownCalls
	result.Note = produceNote(reusedAll, redidVoice, redidAnalysis, true)
	return result, nil
}

func produceNote(reusedAll, redidVoice, redidAnalysis, haveCost bool) string {
	lead := ""
	switch {
	case !haveCost:
		lead = "成片已导出。费用没有读出来。"
	case reusedAll && !redidAnalysis:
		lead = "没有需要重做的步骤，沿用已导出的成片。"
	case reusedAll:
		lead = "口播已重新生成。成片不需要重做，沿用已导出的成片。"
	case !redidVoice:
		lead = "已按改动重新导出。配音没有重做。"
	}
	const honest = "配音来自当前的固定响应或已配置的供应商。印尼语还没有人工审核，不能当作可发布成片。"
	if !haveCost {
		return lead
	}
	if lead == "" {
		return honest
	}
	return lead + honest
}

func reuseExport(redo jobs.Redo, path string) bool {
	if redo.Mix || len(redo.TTS) > 0 || len(redo.Assemble) > 0 {
		return false
	}
	return jobs.FileReady(path)
}

func keepClipAssets(clips []store.Clip, assetIDs []string) ([]store.Clip, error) {
	if len(assetIDs) == 0 {
		return clips, nil
	}
	want := map[string]bool{}
	for _, id := range assetIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = true
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("先选择镜头")
	}
	kept := make([]store.Clip, 0, len(clips))
	for _, clip := range clips {
		if want[clip.AssetID] {
			kept = append(kept, clip)
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("先选择镜头")
	}
	return kept, nil
}

func analysisKey(facts string, clips []store.Clip) string {
	parts := []string{ai.PromptVersion, strings.TrimSpace(facts)}
	for _, clip := range clips {
		parts = append(parts, clip.ID, clip.AssetID, clip.Source, strconv.Itoa(clip.DurationMS), strconv.FormatBool(clip.Selectable))
	}
	return jobs.Key(parts...)
}

func cutFingerprint(analysis, renderKey, fontID, bgmID, voice string, clips []store.Clip, candidate ai.Candidate) jobs.Fingerprint {
	print := jobs.Fingerprint{
		AnalysisKey: analysis,
		RenderKey:   renderKey,
		BGMKey:      jobs.Key(bgmID, "-18"),
		CaptionKey:  jobs.Key(fontID),
	}
	for _, seg := range candidate.Segments {
		print.Segments = append(print.Segments, jobs.SegmentPrint{
			ID:       seg.ID,
			VoiceKey: jobs.Key(voice, seg.VoiceText),
			ShotKey:  shotKey(clips, seg.ClipIDs),
		})
	}
	return print
}

func shotKey(clips []store.Clip, ids []string) string {
	byID := make(map[string]store.Clip, len(clips))
	for _, clip := range clips {
		byID[clip.ID] = clip
	}
	parts := make([]string, 0, len(ids)*3)
	for _, id := range ids {
		clip := byID[id]
		parts = append(parts, id, strconv.Itoa(clip.InMS), strconv.Itoa(clip.OutMS))
	}
	return jobs.Key(parts...)
}

func candidateKey(candidates []ai.Candidate, index int) string {
	id := candidates[index].ID
	for i := 0; i < index; i++ {
		if candidates[i].ID == id {
			return fmt.Sprintf("%s#%d", id, index+1)
		}
	}
	return id
}

func cutPath(skuID string) (string, error) {
	name := filepath.Base(skuID)
	if name != skuID || name == "." || name == ".." {
		return "", fmt.Errorf("商品标识无效")
	}
	return filepath.Join(dataDir(), "cache", "cuts", name+".json"), nil
}

func loadCut(skuID string) (cutRecord, error) {
	path, err := cutPath(skuID)
	if err != nil {
		return cutRecord{}, err
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cutRecord{}, nil
	}
	if err != nil {
		return cutRecord{}, err
	}
	var record cutRecord
	if err := json.Unmarshal(body, &record); err != nil {
		return cutRecord{}, nil
	}
	return record, nil
}

func saveCut(skuID string, record cutRecord) error {
	path, err := cutPath(skuID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return jobs.Publish(path, func(partial string) error {
		return os.WriteFile(partial, body, 0o644)
	})
}

func produceStop(done, index int, err error) error {
	if done == 0 {
		return err
	}
	return fmt.Errorf("已导出 %d 条。第 %d 条没有完成：%w", done, index, err)
}
