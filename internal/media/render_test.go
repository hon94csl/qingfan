package media

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestSubtitleRoundTripKeepsSpecialText(t *testing.T) {
	cues := []timeline.Cue{{
		StartMS: 0,
		EndMS:   2000,
		Text:    `He said "wow" & 100%`,
	}}
	var buf bytes.Buffer
	if err := WriteSRT(&buf, cues); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "drawtext") {
		t.Fatal(buf.String())
	}
	got, err := ParseSRT(buf.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != cues[0] {
		t.Fatalf("got %+v", got)
	}
	filter, err := DrawtextFilter("font.ttf", []drawCaption{{File: "cap-0.txt", StartMS: 0, EndMS: 2000}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filter, cues[0].Text) || !strings.Contains(filter, "textfile=cap-0.txt") || !strings.Contains(filter, "expansion=none") {
		t.Fatal(filter)
	}
}

func TestMixFilterUsesTimelineGain(t *testing.T) {
	filter := mixFilter(-18, 4000)
	if !strings.Contains(filter, "volume=-18dB") || !strings.Contains(filter, "atrim=0:4.000") {
		t.Fatal(filter)
	}
	if strings.Contains(filter, `say "hi"`) {
		t.Fatal(filter)
	}
	video := canvasFilter(timeline.Canvas{Width: 1080, Height: 1920, FPS: 30})
	if !strings.Contains(video, "scale=1080:1920") || !strings.Contains(video, "pad=1080:1920") {
		t.Fatal(video)
	}
	args := appendVideoEncode(nil, 30)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "libx264") || !strings.Contains(joined, "-threads 1") {
		t.Fatal(joined)
	}
}

func TestRenderLeavesExistingFileWhenTimelineIsInvalid(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.mp4")
	payload := []byte("already-exported-success")
	if err := os.WriteFile(dest, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Render(context.Background(), timeline.Timeline{}, nil, dest)
	if err == nil {
		t.Fatal("expected rejection")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("output changed to %q", got)
	}
	if _, err := os.Stat(strings.TrimSuffix(dest, filepath.Ext(dest)) + ".partial.mp4"); !os.IsNotExist(err) {
		t.Fatal("partial file was created")
	}
}

func TestParseProbeRotationAndVariableRate(t *testing.T) {
	raw := []byte(`{
	  "streams": [{
	    "codec_type": "video",
	    "width": 320,
	    "height": 180,
	    "avg_frame_rate": "27/2",
	    "r_frame_rate": "30/1",
	    "duration": "1.333333",
	    "side_data_list": [{"side_data_type": "Display Matrix", "rotation": 90}]
	  }, {
	    "codec_type": "audio",
	    "duration": "1.000"
	  }],
	  "format": {"duration": "1.333333"}
	}`)
	info, err := parseProbe(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.Rotation != 90 || !info.VariableFrameRate || !info.HasVideo || !info.HasAudio {
		t.Fatalf("%+v", info)
	}
	if info.DurationMS != 1333 || info.Width != 320 || info.AudioStreams != 1 {
		t.Fatalf("%+v", info)
	}
}
