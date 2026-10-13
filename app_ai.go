package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func (a *App) aiReady() error {
	if err := a.ready(); err != nil {
		return err
	}
	if a.aiErr != nil {
		return a.aiErr
	}
	if a.ai == nil {
		return fmt.Errorf("调用记录库还没有打开")
	}
	return nil
}

func (a *App) token() (string, error) {
	if err := a.aiReady(); err != nil {
		return "", err
	}
	const account = "local"
	if secret, err := a.creds.Get(account); err == nil && secret != "" {
		return secret, nil
	}
	plain, err := a.ai.Issue("local", 40)
	if err != nil {
		return "", err
	}
	if err := a.creds.Put(account, plain); err != nil {
		return "", fmt.Errorf("无法写入系统凭据库")
	}
	return plain, nil
}

func aiClips(rows []store.Clip) []ai.Clip {
	out := make([]ai.Clip, 0, len(rows))
	for _, row := range rows {
		out = append(out, ai.Clip{
			ID: row.ID, SKUID: row.SKUID, Source: row.Source, Selectable: row.Selectable,
			InMS: row.InMS, OutMS: row.OutMS, Path: row.Path,
		})
	}
	return out
}

// GenerateScripts validates a fixture or supplier response into reviewable candidates.
func (a *App) GenerateScripts(skuID, facts string) (ai.Generation, error) {
	plain, err := a.token()
	if err != nil {
		return ai.Generation{}, err
	}
	if facts != "" {
		if err := a.db.UpdateFacts(skuID, facts); err != nil {
			return ai.Generation{}, err
		}
	} else {
		sku, err := a.db.GetSKU(skuID)
		if err != nil {
			return ai.Generation{}, err
		}
		facts = sku.Facts
	}
	clips, err := a.db.ListClips(skuID)
	if err != nil {
		return ai.Generation{}, err
	}
	gen, err := ai.Generate(a.ctx, a.ai, plain, skuID, ai.FactsFromText(facts), aiClips(clips))
	if err != nil {
		return ai.Generation{}, err
	}
	for _, note := range gen.Notes {
		if err := a.db.SaveNote(skuID, note); err != nil {
			return ai.Generation{}, err
		}
	}
	return gen, nil
}

// ListNotes returns saved shot descriptions.
func (a *App) ListNotes(skuID string) ([]ai.ShotNote, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListNotes(skuID)
}

// SaveCorrection checks an operator edit, then stores it.
func (a *App) SaveCorrection(skuID string, edit ai.ShotNote) (ai.ShotNote, error) {
	if err := a.ready(); err != nil {
		return ai.ShotNote{}, err
	}
	clips, err := a.db.ListClips(skuID)
	if err != nil {
		return ai.ShotNote{}, err
	}
	current := ai.ShotNote{ClipID: edit.ClipID}
	notes, err := a.db.ListNotes(skuID)
	if err != nil {
		return ai.ShotNote{}, err
	}
	for _, note := range notes {
		if note.ClipID == edit.ClipID {
			current = note
		}
	}
	fixed, err := ai.ApplyCorrection(aiClips(clips), current, edit)
	if err != nil {
		return ai.ShotNote{}, err
	}
	if err := a.db.SaveNote(skuID, fixed); err != nil {
		return ai.ShotNote{}, err
	}
	return fixed, nil
}

// Retranslate replaces the Chinese text for the current voice copy.
func (a *App) Retranslate(skuID string, candidate ai.Candidate) (ai.Candidate, error) {
	plain, err := a.token()
	if err != nil {
		return ai.Candidate{}, err
	}
	return ai.Retranslate(a.ctx, a.ai, plain, skuID, candidate)
}

func ttsVoice(voice string) string {
	if voice != "" {
		return voice
	}
	if env := os.Getenv("QINGFAN_TTS_VOICE"); env != "" {
		return env
	}
	return "fixture-id"
}

// AssembleCandidate synthesizes the approved copy and builds a draft from the real audio length.
func (a *App) AssembleCandidate(projectID, skuID, fontID, bgmID, voice string, candidate ai.Candidate) (store.Draft, error) {
	if candidate.TranslationStale() {
		return store.Draft{}, fmt.Errorf("中文回译已失效，请先重新生成回译")
	}
	if fontID == "" || bgmID == "" {
		return store.Draft{}, fmt.Errorf("先选择字体和 BGM")
	}
	plain, err := a.token()
	if err != nil {
		return store.Draft{}, err
	}
	voice = ttsVoice(voice)
	clips, err := a.db.ListClips(skuID)
	if err != nil {
		return store.Draft{}, err
	}
	footage := make([]timeline.Footage, 0, len(clips))
	for _, clip := range clips {
		footage = append(footage, timeline.Footage{
			ClipID: clip.ID, SKUID: clip.SKUID, Source: clip.Source, InMS: clip.InMS, OutMS: clip.OutMS,
		})
	}
	plans := make([]timeline.SegmentPlan, 0, len(candidate.Segments))
	for _, seg := range candidate.Segments {
		path := voiceCachePath(voice, seg.VoiceText)
		stepKey := jobs.Key("tts", ai.PromptVersion, voice, seg.VoiceText)
		if _, err := loadVoice(a.ctx, a.ai, plain, skuID, voice, seg, path); err != nil {
			_ = a.db.PutStep(skuID, stepKey, jobs.Failed, "", "配音失败")
			return store.Draft{}, err
		}
		if err := a.db.PutStep(skuID, stepKey, jobs.Succeeded, path, ""); err != nil {
			return store.Draft{}, err
		}
		imported, err := a.db.Import(a.ctx, store.ImportRequest{
			SKUID: skuID, Kind: timeline.KindVoice, Source: timeline.SourceOwned, Rights: store.RightsUsable, Path: path,
		})
		if err != nil {
			return store.Draft{}, err
		}
		duration, err := a.voiceDuration(skuID, imported.AssetID)
		if err != nil {
			return store.Draft{}, err
		}
		plans = append(plans, timeline.SegmentPlan{
			ID: seg.ID, Role: seg.Role, VoiceText: seg.VoiceText,
			VoiceAssetID: imported.AssetID, VoiceDurationMS: duration, Order: seg.ClipIDs,
		})
	}
	hook := candidate.HookType
	if hook == "" {
		hook = ai.DefaultStructure().HookType
	}
	built, err := timeline.Build(timeline.BuildInput{
		SKUID: skuID, Market: "ID", Locale: "id-ID",
		FontAssetID: fontID, BGMAssetID: bgmID, BGMGainDB: -18,
		HookType: hook, PromptVersion: ai.PromptVersion,
		Footage: footage, Segments: plans,
	})
	if err != nil {
		return store.Draft{}, err
	}
	draft := store.Draft{
		ProjectID: projectID, SKUID: skuID, Market: "ID", Locale: "id-ID",
		FontAssetID: fontID, BGMAssetID: bgmID, BGMGainDB: -18,
		HookType: built.Recipe.HookType, PromptVersion: built.Recipe.PromptVersion,
		Segments: make([]store.DraftSegment, 0, len(built.Segments)),
	}
	for i, seg := range built.Segments {
		shots := make([]store.DraftShot, 0, len(seg.Shots))
		for _, shot := range seg.Shots {
			shots = append(shots, store.DraftShot{ClipID: shot.ClipID, SourceInMS: shot.SourceInMS, SourceOutMS: shot.SourceOutMS})
		}
		draft.Segments = append(draft.Segments, store.DraftSegment{
			ID: seg.ID, Role: seg.Role, VoiceText: seg.Voice.Text, VoiceAssetID: plans[i].VoiceAssetID,
			Shots: shots, Captions: seg.Captions,
		})
	}
	return a.db.SaveDraft(draft)
}

func (a *App) voiceDuration(skuID, assetID string) (int, error) {
	voices, err := a.db.ListAssets(skuID, timeline.KindVoice)
	if err != nil {
		return 0, err
	}
	for _, voice := range voices {
		if voice.ID == assetID {
			if voice.DurationMS <= 0 {
				return 0, ai.ErrInvalidAudio
			}
			return voice.DurationMS, nil
		}
	}
	return 0, ai.ErrInvalidAudio
}

func voiceCachePath(voice, text string) string {
	return filepath.Join(dataDir(), "cache", "steps", jobs.Key("tts", ai.PromptVersion, voice, text)+".wav")
}

func readCachedVoice(path string) ([]byte, bool) {
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 {
		return nil, false
	}
	return body, true
}

// loadVoice reuses a validated wav. A cache hit does not call the provider.
func loadVoice(ctx context.Context, g *ai.Gateway, token, task, voice string, seg ai.ScriptSegment, path string) ([]byte, error) {
	if wav, ok := readCachedVoice(path); ok {
		return wav, nil
	}
	take, err := ai.Synthesize(ctx, g, token, task, voice, seg)
	if err != nil {
		return nil, err
	}
	if !take.Matches(seg.VoiceText, voice) {
		return nil, ai.ErrInvalidAudio
	}
	if err := jobs.Publish(path, func(partial string) error {
		return os.WriteFile(partial, take.WAV, 0o644)
	}); err != nil {
		return nil, err
	}
	return take.WAV, nil
}
