package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func basePrint() Fingerprint {
	return Fingerprint{
		AnalysisKey: "analysis-1",
		RenderKey:   "render-1",
		BGMKey:      "bgm-1",
		CaptionKey:  "caption-1",
		Segments: []SegmentPrint{
			{ID: "seg_1", VoiceKey: "voice-a", ShotKey: "shot-a"},
			{ID: "seg_2", VoiceKey: "voice-b", ShotKey: "shot-b"},
		},
	}
}

func TestDiffRedoesOnlyTheChangedSegment(t *testing.T) {
	fresh := Diff(Fingerprint{}, basePrint())
	if !fresh.Analyze || !fresh.Mix || len(fresh.TTS) != 2 || len(fresh.Assemble) != 2 {
		t.Fatalf("zero before: %+v", fresh)
	}

	same := Diff(basePrint(), basePrint())
	if same.Analyze || same.Mix || len(same.TTS) != 0 || len(same.Assemble) != 0 {
		t.Fatalf("unchanged: %+v", same)
	}

	bgm := basePrint()
	bgm.BGMKey = "bgm-2"
	got := Diff(basePrint(), bgm)
	if got.Analyze || len(got.TTS) != 0 || len(got.Assemble) != 0 || !got.Mix {
		t.Fatalf("bgm: %+v", got)
	}

	caption := basePrint()
	caption.CaptionKey = "caption-2"
	got = Diff(basePrint(), caption)
	if got.Analyze || len(got.TTS) != 0 || len(got.Assemble) != 0 || !got.Mix {
		t.Fatalf("caption: %+v", got)
	}

	voice := basePrint()
	voice.Segments[1].VoiceKey = "voice-c"
	got = Diff(basePrint(), voice)
	if got.Analyze || !got.Mix || len(got.TTS) != 1 || got.TTS[0] != "seg_2" || len(got.Assemble) != 1 || got.Assemble[0] != "seg_2" {
		t.Fatalf("voice: %+v", got)
	}

	shot := basePrint()
	shot.Segments[0].ShotKey = "shot-c"
	got = Diff(basePrint(), shot)
	if got.Analyze || len(got.TTS) != 0 || !got.Mix || len(got.Assemble) != 1 || got.Assemble[0] != "seg_1" {
		t.Fatalf("shot: %+v", got)
	}

	analysis := basePrint()
	analysis.AnalysisKey = "analysis-2"
	got = Diff(basePrint(), analysis)
	if !got.Analyze || got.Mix || len(got.TTS) != 0 || len(got.Assemble) != 0 {
		t.Fatalf("analysis: %+v", got)
	}
}

func TestPublishLeavesTheExistingFileWhenWriteFails(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "export.mp4")
	if err := os.WriteFile(dst, []byte("exported"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Publish(dst, func(path string) error {
		if err := os.WriteFile(path, []byte("partial-bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		return errors.New("disk full")
	})
	if err == nil {
		t.Fatal("expected write failure")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "exported" {
		t.Fatalf("export changed to %q", got)
	}
	if _, err := os.Stat(dst + ".partial"); !os.IsNotExist(err) {
		t.Fatal("partial file remained")
	}
}

func TestRecoverIgnoresProgressAndKeepsFinishedArtifacts(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "voice.wav")
	if err := os.WriteFile(ready, []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []Step{
		{ID: "run", Status: Running, Artifact: ready, Progress: 90},
		{ID: "bad", Status: Succeeded, Artifact: filepath.Join(dir, "missing.wav"), Progress: 100},
		{ID: "ok", Status: Succeeded, Artifact: ready, Progress: 100},
		{ID: "fail", Status: Failed, Progress: 40},
		{ID: "stop", Status: Cancelled, Progress: 100},
		{ID: "review", Status: NeedsReview, Progress: 10},
	}
	got := Recover(steps, FileReady)
	if steps[0].Status != Running {
		t.Fatal("input was modified")
	}
	want := []string{Pending, Pending, Succeeded, Failed, Cancelled, NeedsReview}
	for i, status := range want {
		if got[i].Status != status {
			t.Fatalf("step %s: got %s", got[i].ID, got[i].Status)
		}
	}
	if got[0].Artifact != "" || got[0].Progress != 90 {
		t.Fatalf("running step: %+v", got[0])
	}
	if got[1].Artifact != "" || got[2].Artifact != ready {
		t.Fatalf("artifacts: %+v %+v", got[1], got[2])
	}
}

func TestCleanupKeepsReferencedFilesAndOutsideTargets(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(dir, "original.mp4")
	exported := filepath.Join(dir, "export.mp4")
	if err := os.WriteFile(original, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exported, []byte("export"), 0o644); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(cache, "kept-voice")
	if err := Publish(kept, func(path string) error {
		return os.WriteFile(path, []byte("voice"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "stale"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, filepath.Join(cache, "linked-original")); err != nil {
		t.Fatal(err)
	}
	removed, err := Cleanup(cache, map[string]bool{"kept-voice": true}, map[string]bool{
		original: true,
		exported: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed %+v", removed)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "stale")); !os.IsNotExist(err) {
		t.Fatal("stale cache remained")
	}
	body, err := os.ReadFile(original)
	if err != nil || string(body) != "original" {
		t.Fatalf("original: %q %v", body, err)
	}
	body, err = os.ReadFile(exported)
	if err != nil || string(body) != "export" {
		t.Fatalf("export: %q %v", body, err)
	}
}
