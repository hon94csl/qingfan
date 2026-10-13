package timeline

import (
	"errors"
	"testing"
)

func footage() []Footage {
	return []Footage{
		{ClipID: "a", SKUID: "sku", Source: SourceOwned, InMS: 0, OutMS: 1000},
		{ClipID: "b", SKUID: "sku", Source: SourceOwned, InMS: 200, OutMS: 1200},
		{ClipID: "ref", SKUID: "sku", Source: SourceReference, InMS: 0, OutMS: 4000},
		{ClipID: "other", SKUID: "other", Source: SourceOwned, InMS: 0, OutMS: 4000},
	}
}

func TestAssembleCombinesShotsAndStopsAtGap(t *testing.T) {
	shots, err := Assemble(1500, footage(), []string{"a", "b"}, "sku")
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 2 || shots[0] != (Shot{ClipID: "a", SourceInMS: 0, SourceOutMS: 1000}) || shots[1] != (Shot{ClipID: "b", SourceInMS: 200, SourceOutMS: 700}) {
		t.Fatalf("%+v", shots)
	}
	shots, err = Assemble(2500, footage(), []string{"a", "b"}, "sku")
	if !errors.Is(err, ErrGap) || shots != nil {
		t.Fatalf("err=%v shots=%+v", err, shots)
	}
	if _, err := Assemble(500, footage(), []string{"missing"}, "sku"); !errors.Is(err, ErrMissing) {
		t.Fatal(err)
	}
	if _, err := Assemble(500, footage(), []string{"ref"}, "sku"); !errors.Is(err, ErrReference) {
		t.Fatal(err)
	}
	if _, err := Assemble(500, footage(), []string{"other"}, "sku"); !errors.Is(err, ErrSKU) {
		t.Fatal(err)
	}
}

func TestBuildUsesVoiceDuration(t *testing.T) {
	tl, err := Build(BuildInput{
		OutputID: "out", Revision: 1, SKUID: "sku", Market: "ID", Locale: "id-ID",
		FontAssetID: "font", BGMAssetID: "bgm", BGMGainDB: -18,
		HookType: "pain_question", PromptVersion: "m3-2026-10-13",
		Footage: footage(),
		Segments: []SegmentPlan{{
			ID: "seg_1", Role: "hook", VoiceText: "Mudah dibuka.", VoiceAssetID: "voice",
			VoiceDurationMS: 1500, Order: []string{"a", "b"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	seg := tl.Segments[0]
	if seg.DurationMS != 1500 || seg.Voice.DurationMS != 1500 || seg.Captions[0].EndMS != 1500 {
		t.Fatalf("%+v", seg)
	}
	if seg.Shots[1].SourceOutMS > 1200 {
		t.Fatal(seg.Shots)
	}
}
