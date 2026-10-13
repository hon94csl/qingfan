package integration

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestGenerationUsesFixtureAndRealDuration(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gateway, err := ai.Open(filepath.Join(dir, "usage.sqlite"), ai.Fixture{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { gateway.Close() })

	var logs bytes.Buffer
	routes := ai.Routes(gateway, "boot-secret", log.New(&logs, "", 0))
	anonymous := httptest.NewRecorder()
	routes.ServeHTTP(anonymous, httptest.NewRequest(http.MethodPost, "/v1/scripts", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", anonymous.Code)
	}
	revoked, err := gateway.Issue("local", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.Revoke(revoked); err != nil {
		t.Fatal(err)
	}
	revokedRes := httptest.NewRecorder()
	revokedReq := httptest.NewRequest(http.MethodPost, "/v1/scripts", strings.NewReader(`{"task_id":"t","facts":[{"id":"fact-1","text":"瓶盖"}],"clips":[{"id":"clip-a","sku_id":"sku","source":"owned","selectable":true,"source_in_ms":0,"source_out_ms":1000}]}`))
	revokedReq.Header.Set("Authorization", "Bearer "+revoked)
	routes.ServeHTTP(revokedRes, revokedReq)
	if revokedRes.Code != http.StatusUnauthorized {
		t.Fatalf("revoked: %d %s", revokedRes.Code, revokedRes.Body.String())
	}
	if strings.Contains(logs.String(), "boot-secret") || strings.Contains(logs.String(), revoked) {
		t.Fatal("log contains a credential")
	}

	root := repoRoot(t)
	project, err := db.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := db.CreateSKU(project.ID, "演示商品", "瓶盖可以单手打开")
	if err != nil {
		t.Fatal(err)
	}
	importOne := func(kind, source, name string) store.ImportResult {
		t.Helper()
		got, err := db.Import(ctx, store.ImportRequest{
			SKUID: sku.ID, Kind: kind, Source: source, Rights: store.RightsUsable,
			Path: filepath.Join(root, "tests", "fixtures", name),
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	importOne(timeline.KindClip, timeline.SourceOwned, filepath.Join("media", "clip_red.mp4"))
	importOne(timeline.KindClip, timeline.SourceOwned, filepath.Join("media", "clip_blue.mp4"))
	bgm := importOne(timeline.KindBGM, timeline.SourceLibrary, filepath.Join("media", "bgm_demo.wav"))
	font := importOne(timeline.KindFont, timeline.SourceLibrary, filepath.Join("fonts", "Lato-Regular.ttf"))
	rows, err := db.ListClips(sku.ID)
	if err != nil || len(rows) == 0 {
		t.Fatal(err, len(rows))
	}
	token, err := gateway.Issue("local", 8)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := ai.Generate(ctx, gateway, token, sku.ID, ai.FactsFromText(sku.Facts), testClips(rows))
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.Candidates) < 1 || len(gen.Candidates) > 3 || len(gen.Notes) == 0 {
		t.Fatalf("candidates %d notes %d", len(gen.Candidates), len(gen.Notes))
	}
	edited := ai.EditVoiceText(gen.Candidates[0], gen.Candidates[0].Segments[0].ID, "Lihat barang ini sekarang.")
	if !edited.TranslationStale() {
		t.Fatal("edited copy kept the translation")
	}
	fresh, err := ai.Retranslate(ctx, gateway, token, sku.ID, edited)
	if err != nil || fresh.TranslationStale() || fresh.BackTranslation == "" {
		t.Fatal(err, fresh.TranslationStale())
	}

	footage := make([]timeline.Footage, 0, len(rows))
	known := map[string]bool{}
	for _, clip := range rows {
		known[clip.ID] = true
		footage = append(footage, timeline.Footage{ClipID: clip.ID, SKUID: clip.SKUID, Source: clip.Source, InMS: clip.InMS, OutMS: clip.OutMS})
	}
	if _, err := timeline.Assemble(800, footage, []string{"missing-clip"}, sku.ID); !errors.Is(err, timeline.ErrMissing) {
		t.Fatal(err)
	}
	if shots, err := timeline.Assemble(1000000, footage, fresh.Segments[0].ClipIDs, sku.ID); !errors.Is(err, timeline.ErrGap) || shots != nil {
		t.Fatal(err, shots)
	}

	plans := make([]timeline.SegmentPlan, 0, len(fresh.Segments))
	for _, seg := range fresh.Segments {
		take, err := ai.Synthesize(ctx, gateway, token, sku.ID, "fixture-id", seg)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "cache", "tts", take.TextHash+".wav")
		if err := writeFile(path, take.WAV); err != nil {
			t.Fatal(err)
		}
		imported, err := db.Import(ctx, store.ImportRequest{
			SKUID: sku.ID, Kind: timeline.KindVoice, Source: timeline.SourceOwned, Rights: store.RightsUsable, Path: path,
		})
		if err != nil {
			t.Fatal(err)
		}
		duration := voiceDuration(t, db, sku.ID, imported.AssetID)
		plans = append(plans, timeline.SegmentPlan{
			ID: seg.ID, Role: seg.Role, VoiceText: seg.VoiceText,
			VoiceAssetID: imported.AssetID, VoiceDurationMS: duration, Order: seg.ClipIDs,
		})
	}
	built, err := timeline.Build(timeline.BuildInput{
		SKUID: sku.ID, Market: "ID", Locale: "id-ID",
		FontAssetID: font.AssetID, BGMAssetID: bgm.AssetID, BGMGainDB: -18,
		HookType: fresh.HookType, PromptVersion: ai.PromptVersion,
		Footage: footage, Segments: plans,
	})
	if err != nil {
		t.Fatal(err)
	}
	draft := store.Draft{
		ProjectID: project.ID, SKUID: sku.ID, Market: "ID", Locale: "id-ID",
		FontAssetID: font.AssetID, BGMAssetID: bgm.AssetID, BGMGainDB: -18,
		HookType: built.Recipe.HookType, PromptVersion: built.Recipe.PromptVersion,
	}
	for i, seg := range built.Segments {
		item := store.DraftSegment{
			ID: seg.ID, Role: seg.Role, VoiceText: seg.Voice.Text, VoiceAssetID: plans[i].VoiceAssetID, Captions: seg.Captions,
		}
		sum := 0
		for _, shot := range seg.Shots {
			if !known[shot.ClipID] {
				t.Fatalf("shot uses an unknown clip %s", shot.ClipID)
			}
			item.Shots = append(item.Shots, store.DraftShot{ClipID: shot.ClipID, SourceInMS: shot.SourceInMS, SourceOutMS: shot.SourceOutMS})
			sum += shot.SourceOutMS - shot.SourceInMS
		}
		if sum != plans[i].VoiceDurationMS {
			t.Fatalf("shots %d voice %d", sum, plans[i].VoiceDurationMS)
		}
		draft.Segments = append(draft.Segments, item)
	}
	saved, err := db.SaveDraft(draft)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := db.Export(ctx, saved.ID, filepath.Join(dir, "candidate.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	info, err := media.Probe(ctx, exported.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 1080 || info.Height != 1920 || info.AvgFPS < 29 || info.AvgFPS > 31 || !info.HasAudio {
		t.Fatalf("%+v", info)
	}
	if delta(info.DurationMS, plans[0].VoiceDurationMS) > 34 {
		t.Fatalf("duration %d voice %d", info.DurationMS, plans[0].VoiceDurationMS)
	}
}

func testClips(rows []store.Clip) []ai.Clip {
	out := make([]ai.Clip, 0, len(rows))
	for _, row := range rows {
		out = append(out, ai.Clip{
			ID: row.ID, SKUID: row.SKUID, Source: row.Source, Selectable: row.Selectable,
			InMS: row.InMS, OutMS: row.OutMS, Path: row.Path,
		})
	}
	return out
}

func voiceDuration(t *testing.T, db *store.Store, skuID, assetID string) int {
	t.Helper()
	voices, err := db.ListAssets(skuID, timeline.KindVoice)
	if err != nil {
		t.Fatal(err)
	}
	for _, voice := range voices {
		if voice.ID == assetID && voice.DurationMS > 0 {
			return voice.DurationMS
		}
	}
	t.Fatal("missing voice duration")
	return 0
}

func writeFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func delta(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
