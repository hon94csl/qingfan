package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/timeline"
)

type fakeIngest struct {
	probed int
	reused int
}

func (f *fakeIngest) Ingest(_ context.Context, path, kind, _ string, cached *media.CachedProbe) (media.Ingested, error) {
	sum, err := media.Hash(path)
	if err != nil {
		return media.Ingested{}, err
	}
	if cached != nil && cached.SHA256 == sum {
		f.reused++
		return media.Ingested{SHA256: sum, Info: cached.Info, Cuts: cached.Cuts, ThumbnailPath: cached.ThumbnailPath, Reused: true}, nil
	}
	f.probed++
	info := media.Info{DurationMS: 4000, HasAudio: true}
	var cuts []media.Cut
	switch kind {
	case timeline.KindClip:
		info.HasVideo = true
		info.Width, info.Height = 320, 180
		cuts = []media.Cut{{InMS: 0, OutMS: 4000}}
	case timeline.KindVoice:
		info.DurationMS = 3500
		info.HasVideo = false
	case timeline.KindFont:
		return media.Ingested{SHA256: sum}, nil
	}
	return media.Ingested{SHA256: sum, Info: info, Cuts: cuts}, nil
}

func newTestStore(t *testing.T) (*Store, *fakeIngest) {
	t.Helper()
	dir := t.TempDir()
	fake := &fakeIngest{}
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), fake.Ingest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, fake
}

func writeBytes(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportReusesProbeAndDoesNotPromoteRights(t *testing.T) {
	s, fake := newTestStore(t)
	project, err := s.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := s.CreateSKU(project.ID, "瓶子", "瓶盖难开")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := writeBytes(t, dir, "clip.mp4", "same-video-bytes")
	ctx := context.Background()
	first, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceReference, Rights: RightsUsable, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != timeline.SourceReference || first.Rights != RightsReferenceOnly {
		t.Fatalf("reference was stored as %s %s", first.Source, first.Rights)
	}
	second, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if second.AssetID != first.AssetID || second.Source != timeline.SourceReference || second.Rights != RightsReferenceOnly {
		t.Fatalf("promoted: %+v", second)
	}
	if !second.ReusedProbe || fake.probed != 1 || fake.reused != 1 {
		t.Fatalf("probe calls probed=%d reused=%d result=%+v", fake.probed, fake.reused, second)
	}
	if len(second.ClipIDs) != 1 || second.ClipIDs[0] != first.ClipIDs[0] {
		t.Fatalf("clips changed: %v %v", first.ClipIDs, second.ClipIDs)
	}
	clips, err := s.ListClips(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 1 || clips[0].Selectable {
		t.Fatalf("reference clip selectable: %+v", clips)
	}
}

func TestRelocateAndReplacement(t *testing.T) {
	s, _ := newTestStore(t)
	project, err := s.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := s.CreateSKU(project.ID, "瓶子", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateSKU(project.ID, "另一个", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := writeBytes(t, dir, "clip.mp4", "version-one")
	ctx := context.Background()
	first, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(dir, "moved.mp4")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := s.Relocate(first.LocationID, moved); err != nil {
		t.Fatal(err)
	}
	clips, err := s.ListClips(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 1 || clips[0].Path != moved || !clips[0].Selectable {
		t.Fatalf("%+v", clips)
	}
	if err := s.Relocate(first.LocationID, writeBytes(t, dir, "other.mp4", "different")); err != ErrHash {
		t.Fatalf("relocate mismatch: %v", err)
	}
	if err := os.WriteFile(moved, []byte("version-two"), 0o644); err != nil {
		t.Fatal(err)
	}
	replaced, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: moved})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.AssetID == first.AssetID {
		t.Fatal("replacement kept the old asset")
	}
	clips, err = s.ListClips(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	var oldOK, newOK bool
	for _, clip := range clips {
		if clip.ID == first.ClipIDs[0] {
			oldOK = !clip.Selectable && !clip.Valid
		}
		if clip.ID == replaced.ClipIDs[0] {
			newOK = clip.Selectable
		}
	}
	if !oldOK || !newOK {
		t.Fatalf("%+v", clips)
	}
	copyPath := writeBytes(t, dir, "copy.mp4", "version-two")
	copied, err := s.Import(ctx, ImportRequest{SKUID: other.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: copyPath})
	if err != nil {
		t.Fatal(err)
	}
	if copied.AssetID == replaced.AssetID || !copied.ReusedProbe {
		t.Fatalf("cross-sku dedup: %+v vs %s", copied, replaced.AssetID)
	}
}

func TestManualClipAndForeignSelection(t *testing.T) {
	s, _ := newTestStore(t)
	project, err := s.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := s.CreateSKU(project.ID, "瓶子", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateSKU(project.ID, "另一个", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx := context.Background()
	owned, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: writeBytes(t, dir, "owned.mp4", "owned-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.Import(ctx, ImportRequest{SKUID: other.ID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: RightsUsable, Path: writeBytes(t, dir, "foreign.mp4", "foreign-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddManualClip(owned.AssetID, 0, 9000); err != ErrRange {
		t.Fatalf("overlong clip: %v", err)
	}
	manual, err := s.AddManualClip(owned.AssetID, 1000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if !manual.Selectable || manual.InMS != 1000 {
		t.Fatalf("%+v", manual)
	}
	voice, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindVoice, Source: timeline.SourceOwned, Rights: RightsUsable, Path: writeBytes(t, dir, "voice.wav", "voice-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	font, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindFont, Source: timeline.SourceLibrary, Rights: RightsUsable, Path: writeBytes(t, dir, "font.ttf", "font-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	bgm, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindBGM, Source: timeline.SourceLibrary, Rights: RightsUsable, Path: writeBytes(t, dir, "bgm.wav", "bgm-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	draft := sampleDraft(project.ID, sku.ID, owned.ClipIDs[0], voice.AssetID, font.AssetID, bgm.AssetID)
	if _, err := s.SaveDraft(draft); err != nil {
		t.Fatal(err)
	}
	draft.Segments[0].Shots[0].ClipID = foreign.ClipIDs[0]
	if _, err := s.SaveDraft(draft); err != ErrSKU {
		t.Fatalf("foreign clip: %v", err)
	}
	ref, err := s.Import(ctx, ImportRequest{SKUID: sku.ID, Kind: timeline.KindClip, Source: timeline.SourceReference, Rights: RightsReferenceOnly, Path: writeBytes(t, dir, "ref.mp4", "ref-bytes")})
	if err != nil {
		t.Fatal(err)
	}
	draft.Segments[0].Shots[0].ClipID = ref.ClipIDs[0]
	if _, err := s.SaveDraft(draft); err != ErrReference {
		t.Fatalf("reference clip: %v", err)
	}
}

func sampleDraft(project, sku, clip, voice, font, bgm string) Draft {
	return Draft{
		ProjectID: project, SKUID: sku, Market: "ID", Locale: "id-ID",
		FontAssetID: font, BGMAssetID: bgm, BGMGainDB: -18,
		Segments: []DraftSegment{{
			ID: "seg_1", Role: "hook", VoiceText: "Susah membuka tutup botol?", VoiceAssetID: voice,
			Shots:    []DraftShot{{ClipID: clip, SourceInMS: 0, SourceOutMS: 4000}},
			Captions: []timeline.Caption{{StartMS: 0, EndMS: 3500, Text: `Coba "cara" ini: 100% & ok`}},
		}},
	}
}
