package store

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestDraftEditDoesNotChangeExportRecord(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeIngest{}
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), fake.Ingest)
	if err != nil {
		t.Fatal(err)
	}
	project, sku, draft := mustDraft(t, s)
	saved, err := s.SaveDraft(draft)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 {
		t.Fatal(saved.Revision)
	}
	const manifest = `{"output_id":"frozen"}`
	if _, err := s.db.Exec(`INSERT INTO outputs(id, draft_id, draft_revision, path, manifest_json, created_at) VALUES ('out1', ?, 1, ?, ?, ?)`, saved.ID, filepath.Join(t.TempDir(), "a.mp4"), manifest, nowText()); err != nil {
		t.Fatal(err)
	}
	saved.Segments[0].VoiceText = "改过的口播"
	saved.Segments[0].Shots[0].SourceOutMS = 3800
	updated, err := s.SaveDraft(saved)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 {
		t.Fatal(updated.Revision)
	}
	outputs, err := s.ListOutputs(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 || outputs[0].ManifestJSON != manifest || outputs[0].DraftRevision != 1 {
		t.Fatalf("%+v", outputs)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetDraft(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Segments[0].VoiceText != "改过的口播" || got.Revision != 2 {
		t.Fatalf("%+v", got.Segments)
	}
	again, err := reopened.ListOutputs(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].ManifestJSON != manifest {
		t.Fatalf("%+v", again)
	}
	clips, err := reopened.ListClips(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if clips[0].OutMS != 3800 {
		t.Fatalf("interval not corrected: %+v", clips)
	}
	_ = project
}

func TestExportReflectsShotChange(t *testing.T) {
	requireStoreMedia(t)
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root := filepath.Join(callerRoot(t), "tests", "fixtures")
	project, err := s.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := s.CreateSKU(project.ID, "瓶子", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	importAt := func(kind, source, name string) ImportResult {
		t.Helper()
		got, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: kind, Source: source, Rights: RightsUsable, Path: filepath.Join(root, name)})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	red := importAt(timeline.KindClip, timeline.SourceOwned, filepath.Join("media", "clip_red.mp4"))
	blue := importAt(timeline.KindClip, timeline.SourceOwned, filepath.Join("media", "clip_blue.mp4"))
	voice := importAt(timeline.KindVoice, timeline.SourceOwned, filepath.Join("media", "voice_demo.wav"))
	bgm := importAt(timeline.KindBGM, timeline.SourceLibrary, filepath.Join("media", "bgm_demo.wav"))
	font := importAt(timeline.KindFont, timeline.SourceLibrary, filepath.Join("fonts", "Lato-Regular.ttf"))
	draft := Draft{
		ProjectID: project.ID, SKUID: sku.ID, Market: "ID", Locale: "id-ID",
		FontAssetID: font.AssetID, BGMAssetID: bgm.AssetID, BGMGainDB: -18,
		Segments: []DraftSegment{{
			ID: "seg_1", Role: "hook", VoiceText: "Susah membuka tutup botol?", VoiceAssetID: voice.AssetID,
			Shots: []DraftShot{
				{ClipID: red.ClipIDs[0], SourceInMS: 0, SourceOutMS: 2000},
				{ClipID: blue.ClipIDs[0], SourceInMS: 0, SourceOutMS: 2000},
			},
			Captions: []timeline.Caption{{StartMS: 0, EndMS: 3500, Text: `Coba "cara" ini: 100% & ok`}},
		}},
	}
	saved, err := s.SaveDraft(draft)
	if err != nil {
		t.Fatal(err)
	}
	previewPath := filepath.Join(dir, "preview.mp4")
	preview, err := s.Preview(ctx, saved.ID, previewPath)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Timeline.Segments[0].Shots[1].ClipID != blue.AssetID {
		t.Fatalf("preview shot %+v", preview.Timeline.Segments[0].Shots)
	}
	if outputs, err := s.ListOutputs(saved.ID); err != nil || len(outputs) != 0 {
		t.Fatalf("preview created an output: %v %+v", err, outputs)
	}
	outPath := filepath.Join(dir, "export-a.mp4")
	exported, err := s.Export(ctx, saved.ID, outPath)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	saved.Segments[0].Shots[1] = DraftShot{ClipID: red.ClipIDs[0], SourceInMS: 2000, SourceOutMS: 4000}
	if _, err := s.SaveDraft(saved); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Export(ctx, saved.ID, outPath); err != ErrOutputExists {
		t.Fatalf("overwrite: %v", err)
	}
	after, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("existing export bytes changed")
	}
	kept, err := s.ListOutputs(saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].ManifestJSON != exported.ManifestJSON {
		t.Fatal("export record changed")
	}
	var frozen media.Manifest
	if err := json.Unmarshal([]byte(kept[0].ManifestJSON), &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Timeline.Segments[0].Shots[1].ClipID != blue.AssetID {
		t.Fatalf("frozen manifest %+v", frozen.Timeline.Segments[0].Shots)
	}
	second, err := s.Export(ctx, saved.ID, filepath.Join(dir, "export-b.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	var changed media.Manifest
	if err := json.Unmarshal([]byte(second.ManifestJSON), &changed); err != nil {
		t.Fatal(err)
	}
	shot := changed.Timeline.Segments[0].Shots[1]
	if shot.ClipID != red.AssetID || shot.SourceInMS != 2000 || shot.SourceOutMS != 4000 {
		t.Fatalf("new export %+v", changed.Timeline.Segments[0].Shots)
	}
	if changed.Timeline.Segments[0].Captions[0].Text != `Coba "cara" ini: 100% & ok` {
		t.Fatal(changed.Timeline.Segments[0].Captions)
	}
}

func mustDraft(t *testing.T, s *Store) (Project, SKU, Draft) {
	t.Helper()
	project, err := s.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := s.CreateSKU(project.ID, "瓶子", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx := context.Background()
	clip, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: writeBytes(t, dir, "clip.mp4", "clip")})
	if err != nil {
		t.Fatal(err)
	}
	voice, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindVoice, Source: timeline.SourceOwned, Rights: RightsUsable, Path: writeBytes(t, dir, "voice.wav", "voice")})
	if err != nil {
		t.Fatal(err)
	}
	font, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindFont, Source: timeline.SourceLibrary, Rights: RightsUsable, Path: writeBytes(t, dir, "font.ttf", "font")})
	if err != nil {
		t.Fatal(err)
	}
	bgm, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindBGM, Source: timeline.SourceLibrary, Rights: RightsUsable, Path: writeBytes(t, dir, "bgm.wav", "bgm")})
	if err != nil {
		t.Fatal(err)
	}
	return project, sku, sampleDraft(project.ID, sku.ID, clip.ClipIDs[0], voice.AssetID, font.AssetID, bgm.AssetID)
}

func callerRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func requireStoreMedia(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		out, err := exec.Command(path, "-version").CombinedOutput()
		if err != nil {
			t.Fatalf("%s at %s failed to start: %v\n%s", name, path, err, out)
		}
	}
}
