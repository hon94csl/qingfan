package ai

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/timeline"
)

func catalog() ([]Fact, []Clip) {
	facts := []Fact{{ID: "fact-1", Text: "瓶盖可以单手打开"}}
	clips := []Clip{
		{ID: "clip-a", SKUID: "sku", Source: timeline.SourceOwned, Selectable: true, InMS: 0, OutMS: 2000},
		{ID: "clip-b", SKUID: "sku", Source: timeline.SourceOwned, Selectable: true, InMS: 0, OutMS: 2000},
		{ID: "clip-ref", SKUID: "sku", Source: timeline.SourceReference, Selectable: false, InMS: 0, OutMS: 2000},
	}
	return facts, clips
}

func TestParseScriptsRejectsBadModelOutput(t *testing.T) {
	facts, clips := catalog()
	cases := []struct {
		name string
		body string
		want error
	}{
		{"json", `{`, ErrModelJSON},
		{"unknown clip", `{"candidates":[{"id":"c1","hook_type":"pain_question","back_translation":"回译","segments":[{"id":"seg_1","role":"hook","purpose":"问","fact_ids":["fact-1"],"voice_text":"Lihat.","clip_ids":["missing"],"estimated_ms":1000}]}]}`, ErrUnknownAsset},
		{"bounds", `{"clips":[{"clip_id":"clip-a","subject":"瓶","action":"开","purpose":"展示","source_in_ms":0,"source_out_ms":9000}]}`, ErrBounds},
		{"fact", `{"candidates":[{"id":"c1","hook_type":"pain_question","back_translation":"回译","segments":[{"id":"seg_1","role":"hook","purpose":"问","fact_ids":["fact-missing"],"voice_text":"Lihat.","clip_ids":["clip-a"],"estimated_ms":1000}]}]}`, ErrUnknownFact},
		{"reference", `{"candidates":[{"id":"c1","hook_type":"pain_question","back_translation":"回译","segments":[{"id":"seg_1","role":"hook","purpose":"问","fact_ids":["fact-1"],"voice_text":"Lihat.","clip_ids":["clip-ref"],"estimated_ms":1000}]}]}`, timeline.ErrReference},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.name == "bounds" {
				_, err = ParseAnalysis([]byte(tc.body), facts, clips)
			} else {
				_, err = ParseScripts([]byte(tc.body), facts, clips)
			}
			if !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
			if tc.name == "unknown clip" && errors.Is(err, ErrUnknownAsset) {
				cands, parseErr := ParseScripts([]byte(tc.body), facts, clips)
				if parseErr == nil || len(cands) > 0 {
					t.Fatalf("substituted catalog clip: %+v %v", cands, parseErr)
				}
			}
		})
	}
}

func TestTranslationAndVoiceInvalidation(t *testing.T) {
	facts, clips := catalog()
	body := []byte(`{"candidates":[{"id":"c1","hook_type":"pain_question","back_translation":"瓶盖好开吗","segments":[{"id":"seg_1","role":"hook","purpose":"问","fact_ids":["fact-1"],"voice_text":"Mudah dibuka.","clip_ids":["clip-a"],"estimated_ms":1000}]}]}`)
	cands, err := ParseScripts(body, facts, clips)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0].TranslationStale() {
		t.Fatal("fresh translation was stale")
	}
	edited := EditVoiceText(cands[0], "seg_1", "Mudah dibuka dengan satu tangan.")
	if !edited.TranslationStale() || edited.BackTranslation != cands[0].BackTranslation {
		t.Fatal("edit kept the translation current")
	}
	wav, err := SilenceWAV(900)
	if err != nil {
		t.Fatal(err)
	}
	g := testGateway(t, &staticProvider{body: wav, usage: Usage{Known: true, Micros: 4}}, 2)
	token, err := g.Issue("shop-a", 4)
	if err != nil {
		t.Fatal(err)
	}
	take, err := Synthesize(context.Background(), g, token, "task", "voice-a", edited.Segments[0])
	if err != nil {
		t.Fatal(err)
	}
	if !take.Matches(edited.Segments[0].VoiceText, "voice-a") || take.Matches(edited.Segments[0].VoiceText, "voice-b") {
		t.Fatal("voice change did not invalidate the take")
	}
	if _, err := g.Do(context.Background(), token, Call{TaskID: "task", Kind: "tts", Model: "test", PromptVersion: PromptVersion, Voice: "voice-a", Attempt: 1, Input: mustJSON(t, edited.Segments[0].VoiceText)}); err != nil {
		t.Fatal(err)
	}
}

func TestFrameTimesAndExtract(t *testing.T) {
	times, err := FrameTimes(0, 1000)
	if err != nil || len(times) != 3 || times[0] != 0 || times[1] != 500 || times[2] != 958 {
		t.Fatal(times, err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := t.TempDir()
	video := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-nostdin", "-y", "-f", "lavfi", "-i", "color=c=0x224466:s=160x120:r=30:d=1", "-frames:v", "30", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-an", video)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	frames, err := ExtractFrames(context.Background(), video, []int{0, 400}, dir)
	if err != nil || len(frames) != 2 {
		t.Fatal(err, frames)
	}
}

func TestReferenceStructureRejectsShots(t *testing.T) {
	if _, err := ParseStructure([]byte(`{"hook_type":"pain_question","beats":["hook"],"clip_ids":["clip-ref"]}`)); !errors.Is(err, timeline.ErrReference) {
		t.Fatal(err)
	}
	got, err := ParseStructure([]byte(`{"hook_type":"demo","beats":["demo","offer"]}`))
	if err != nil || got.HookType != "demo" {
		t.Fatal(got, err)
	}
}

func mustJSON(t *testing.T, text string) []byte {
	t.Helper()
	return []byte(`{"prompt_version":"m3-2026-10-13","text":"` + text + `","voice":"voice-a"}`)
}
