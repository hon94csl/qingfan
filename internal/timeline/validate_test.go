package timeline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTwoShotTimelineFitsFrameGrid(t *testing.T) {
	tl, assets := validTwoSegment(t)
	if err := Validate(tl, assets, Check{}); err != nil {
		t.Fatal(err)
	}
	starts, err := SegmentStarts(tl)
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 2 || starts[0] != 0 || starts[1] != tl.Segments[0].DurationMS {
		t.Fatalf("starts = %v", starts)
	}
	total, err := TotalDurationMS(tl)
	if err != nil {
		t.Fatal(err)
	}
	if total != tl.Segments[0].DurationMS+tl.Segments[1].DurationMS {
		t.Fatalf("total = %d", total)
	}
	cues, err := AbsoluteCues(tl)
	if err != nil {
		t.Fatal(err)
	}
	if cues[len(cues)-1].StartMS != starts[1] || cues[len(cues)-1].EndMS > total {
		t.Fatalf("cues = %+v", cues)
	}
	bounds := []int{0, 2100, 5100, 1000, 3000, 5000, 2000, total}
	limit := time.Second / 30
	for _, ms := range bounds {
		delta, err := FrameDelta(ms, 30)
		if err != nil {
			t.Fatal(err)
		}
		if delta > limit {
			t.Fatalf("boundary %dms rounds by %s, limit %s", ms, delta, limit)
		}
	}
	odd, err := FrameDelta(1, 30)
	if err != nil {
		t.Fatal(err)
	}
	if odd > limit {
		t.Fatalf("1ms rounds by %s", odd)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		err  error
		edit func(*Timeline, []Asset)
	}{
		{"inverted shot", ErrInterval, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Shots[0].SourceOutMS = tl.Segments[0].Shots[0].SourceInMS
		}},
		{"negative shot", ErrInterval, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Shots[0].SourceInMS = -1
		}},
		{"shot sum", ErrDuration, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Shots[0].SourceOutMS--
		}},
		{"voice past segment", ErrVoice, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Voice.DurationMS = tl.Segments[0].DurationMS + 1
		}},
		{"caption past segment", ErrCaption, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Captions[0].EndMS = tl.Segments[0].DurationMS + 1
		}},
		{"cross sku", ErrSKU, func(_ *Timeline, assets []Asset) {
			assets[0].SKUID = "sku_other"
		}},
		{"reference source", ErrReference, func(_ *Timeline, assets []Asset) {
			assets[0].Source = SourceReference
		}},
		{"missing clip", ErrMissing, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Shots[0].ClipID = "missing_clip"
		}},
		{"missing file", ErrMissing, func(_ *Timeline, assets []Asset) {
			assets[0].Path = assets[0].Path + ".missing"
		}},
		{"hash mismatch", ErrHash, func(_ *Timeline, assets []Asset) {
			assets[0].SHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		}},
		{"shot past media", ErrBounds, func(tl *Timeline, assets []Asset) {
			assets[0].DurationMS = 1000
			tl.Segments[0].Shots[0].SourceInMS = 0
			tl.Segments[0].Shots[0].SourceOutMS = 2000
			tl.Segments[0].Shots[1].SourceInMS = 0
			tl.Segments[0].Shots[1].SourceOutMS = 3000
		}},
		{"missing caption", ErrMissing, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Captions = nil
		}},
		{"missing voice", ErrMissing, func(tl *Timeline, _ []Asset) {
			tl.Segments[0].Voice.AudioID = ""
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tl, assets := validTwoSegment(t)
			tc.edit(&tl, assets)
			err := Validate(tl, assets, Check{})
			if !errors.Is(err, tc.err) {
				t.Fatalf("got %v, want %v", err, tc.err)
			}
		})
	}
}

func TestValidateReadsFilesOnly(t *testing.T) {
	tl, assets := validTwoSegment(t)
	before, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(tl, assets, Check{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("validation rewrote a source file")
	}
}

func validTwoSegment(t *testing.T) (Timeline, []Asset) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{}
	for _, name := range []string{"clip12", "clip18", "clip2", "voice1", "voice2", "bgm", "font"} {
		path := filepath.Join(dir, name+".bin")
		body := []byte("qingfan fixture " + name)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		files[name] = path + "|" + hex.EncodeToString(sum[:])
	}
	asset := func(id, kind, key, sku, source string, duration int) Asset {
		parts := stringsSplit(files[key])
		return Asset{
			ID: id, Kind: kind, Path: parts[0], SHA256: parts[1],
			SKUID: sku, Source: source, DurationMS: duration,
		}
	}
	tl := Timeline{
		SchemaVersion: SchemaVersion,
		OutputID:      "out_0012",
		Revision:      1,
		SKUID:         "sku_88",
		Market:        "ID",
		Locale:        "id-ID",
		Canvas:        Canvas{Width: 1080, Height: 1920, FPS: 30},
		FontAssetID:   "font_main",
		Recipe:        Recipe{HookType: "pain_question", PromptVersion: "script-v1"},
		Segments: []Segment{
			{
				ID:         "seg_1",
				Role:       "hook",
				DurationMS: 5000,
				Voice:      Voice{Text: "Susah membuka tutup botol?", AudioID: "audio_001", DurationMS: 5000},
				Shots: []Shot{
					{ClipID: "clip_12", SourceInMS: 2100, SourceOutMS: 5100},
					{ClipID: "clip_18", SourceInMS: 1000, SourceOutMS: 3000},
				},
				OriginalAudio: OriginalMute,
				Captions: []Caption{
					{StartMS: 0, EndMS: 5000, Text: `Susah membuka tutup botol? Coba "cara" ini: 100% & ok`},
				},
				CaptionStyle: "id-default",
			},
			{
				ID:         "seg_2",
				Role:       "demo",
				DurationMS: 2000,
				Voice:      Voice{Text: "Lanjut.", AudioID: "audio_002", DurationMS: 2000},
				Shots: []Shot{
					{ClipID: "clip_20", SourceInMS: 0, SourceOutMS: 2000},
				},
				OriginalAudio: OriginalMute,
				Captions: []Caption{
					{StartMS: 0, EndMS: 2000, Text: "Lanjut."},
				},
			},
		},
		BGM: BGM{AssetID: "bgm_0042", GainDB: -18},
	}
	assets := []Asset{
		asset("clip_12", KindClip, "clip12", "sku_88", SourceOwned, 6000),
		asset("clip_18", KindClip, "clip18", "sku_88", SourceOwned, 4000),
		asset("clip_20", KindClip, "clip2", "sku_88", SourceOwned, 2000),
		asset("audio_001", KindVoice, "voice1", "sku_88", SourceOwned, 5000),
		asset("audio_002", KindVoice, "voice2", "sku_88", SourceOwned, 2000),
		asset("bgm_0042", KindBGM, "bgm", "", SourceLibrary, 8000),
		asset("font_main", KindFont, "font", "", SourceLibrary, 0),
	}
	return tl, assets
}

func stringsSplit(joined string) []string {
	for i := 0; i < len(joined); i++ {
		if joined[i] == '|' {
			return []string{joined[:i], joined[i+1:]}
		}
	}
	return []string{joined, ""}
}
