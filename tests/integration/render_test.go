package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestRenderFixedTimeline(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	root := repoRoot(t)
	work := filepath.Join(t.TempDir(), "镜头 素材")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	red := filepath.Join(work, "红 镜头.mp4")
	blue := filepath.Join(work, "蓝 镜头.mp4")
	tail := filepath.Join(work, "尾 镜头.mp4")
	voice1 := filepath.Join(work, "口播 1.wav")
	voice2 := filepath.Join(work, "口播 2.wav")
	bgm := filepath.Join(work, "背景音乐.wav")
	makeVideo(t, red, "0xCC3322", "320x180", 60)
	makeVideo(t, blue, "0x2266CC", "320x180", 60)
	makeVideo(t, tail, "0xE6C229", "180x320", 30)
	makeTone(t, voice1, 440, "1.5")
	makeTone(t, voice2, 554, "1.0")
	makeTone(t, bgm, 196, "0.8")
	font := filepath.Join(root, "tests", "fixtures", "fonts", "Lato-Regular.ttf")

	before := map[string]string{}
	for _, path := range []string{red, blue, tail, voice1, voice2, bgm, font} {
		before[path] = fileSHA(t, path)
	}
	tl, assets := twoSegmentProject(t, red, blue, tail, voice1, voice2, bgm, font)
	first := filepath.Join(t.TempDir(), "first.mp4")
	second := filepath.Join(t.TempDir(), "second.mp4")
	manifest, err := media.Render(ctx, tl, assets, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := media.Render(ctx, tl, assets, second); err != nil {
		t.Fatal(err)
	}
	for path, sum := range before {
		if got := fileSHA(t, path); got != sum {
			t.Fatalf("source rewritten: %s", path)
		}
	}
	assertPicture(t, first, 3000)
	assertPicture(t, second, 3000)
	text := extractSRT(t, first)
	for _, want := range []string{`Coba "cara" ini: 100% & ok`, "Lanjut."} {
		if !strings.Contains(text, want) {
			t.Fatalf("subtitle readback missing %q\n%s", want, text)
		}
	}
	if !frameVaried(t, first) {
		t.Fatal("extracted frame is empty or a single color")
	}
	video1, audio1 := decodedDigest(t, first)
	video2, audio2 := decodedDigest(t, second)
	if video1 != video2 || audio1 != audio2 || video1 == "" || audio1 == "" {
		t.Fatalf("content mismatch video %s/%s audio %s/%s", video1, video2, audio1, audio2)
	}
	if manifest.Encoder.Name != "libx264" || manifest.FFmpeg.Version == "" || manifest.Renderer.Version == "" {
		t.Fatalf("manifest versions: %+v %+v", manifest.Encoder, manifest.FFmpeg)
	}
	seen := map[string]bool{}
	for _, resource := range manifest.Resources {
		seen[resource.Kind] = true
		if resource.SHA256 == "" {
			t.Fatalf("missing hash: %+v", resource)
		}
	}
	for _, kind := range []string{timeline.KindClip, timeline.KindVoice, timeline.KindBGM, timeline.KindFont} {
		if !seen[kind] {
			t.Fatalf("manifest missing %s: %+v", kind, manifest.Resources)
		}
	}
	if manifest.Quality.BlackFrames == nil || manifest.Quality.Silence == nil {
		t.Fatal("quality report omitted blackframe or silence results")
	}
	joined := strings.Join(manifest.Notes, "\n")
	if !strings.Contains(joined, "no audio") || !strings.Contains(joined, "landscape") {
		t.Fatalf("notes = %s", joined)
	}

	kept, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	broken := tl
	broken.Segments[0].Captions = nil
	if _, err := media.Render(ctx, broken, assets, first); err == nil {
		t.Fatal("missing captions succeeded")
	}
	after, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kept, after) {
		t.Fatal("failed render changed an existing export")
	}
}

func TestRenderSourceVariants(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	font := filepath.Join(repoRoot(t), "tests", "fixtures", "fonts", "Lato-Regular.ttf")

	t.Run("portrait", func(t *testing.T) {
		clip := filepath.Join(t.TempDir(), "portrait.mp4")
		makeVideo(t, clip, "0x33AA66", "180x320", 30)
		out := renderOne(t, ctx, clip, font, nil)
		assertPicture(t, out, 1000)
	})
	t.Run("rotation", func(t *testing.T) {
		dir := t.TempDir()
		plain := filepath.Join(dir, "plain.mp4")
		rotated := filepath.Join(dir, "rotated.mp4")
		makeVideo(t, plain, "0x228833", "320x180", 30)
		runFF(t, "-hide_banner", "-nostdin", "-y", "-display_rotation", "90", "-i", plain, "-c", "copy", rotated)
		info, err := media.Probe(ctx, rotated)
		if err != nil {
			t.Fatal(err)
		}
		if info.Rotation == 0 {
			t.Fatal("fixture has no display rotation")
		}
		manifest := renderOneManifest(t, ctx, rotated, font)
		if !strings.Contains(strings.Join(manifest.Notes, "\n"), "display rotation") {
			t.Fatalf("notes = %s", manifest.Notes)
		}
		assertPicture(t, manifestPathMedia(manifest), 1000)
	})
	t.Run("variable frame rate", func(t *testing.T) {
		clip := filepath.Join(t.TempDir(), "vfr.mp4")
		runFF(t, "-hide_banner", "-nostdin", "-y",
			"-f", "lavfi", "-i", "testsrc=size=160x120:rate=30:duration=1",
			"-vf", `setpts=PTS*if(eq(mod(N\,2)\,0)\,1\,1.5),format=yuv420p`,
			"-fps_mode", "vfr", "-c:v", "libx264", "-preset", "veryfast", "-an", clip)
		info, err := media.Probe(ctx, clip)
		if err != nil {
			t.Fatal(err)
		}
		if !info.VariableFrameRate {
			t.Fatalf("fixture is not variable frame rate: %+v", info)
		}
		manifest := renderOneManifest(t, ctx, clip, font)
		if !strings.Contains(strings.Join(manifest.Notes, "\n"), "variable frame rate") {
			t.Fatalf("notes = %s", manifest.Notes)
		}
		assertPicture(t, manifestPathMedia(manifest), 1000)
	})
	t.Run("black frame is recorded and still exports", func(t *testing.T) {
		clip := filepath.Join(t.TempDir(), "black.mp4")
		makeVideo(t, clip, "black", "320x180", 30)
		manifest := renderOneManifest(t, ctx, clip, font)
		if len(manifest.Quality.BlackFrames) == 0 {
			t.Fatal("black frames were not recorded")
		}
		assertPicture(t, manifestPathMedia(manifest), 1000)
	})
}

func TestCommittedSample(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := repoRoot(t)
	timelinePath := filepath.Join(root, "tests", "fixtures", "timelines", "sample.json")
	assetsPath := filepath.Join(root, "tests", "fixtures", "timelines", "sample.assets.json")
	tl, err := timeline.LoadTimeline(timelinePath)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := timeline.LoadAssets(assetsPath)
	if err != nil {
		t.Fatal(err)
	}
	assets = timeline.ResolvePaths(assets, filepath.Dir(assetsPath))
	out := filepath.Join(t.TempDir(), "sample.mp4")
	manifest, err := media.Render(ctx, tl, assets, out)
	if err != nil {
		t.Fatal(err)
	}
	assertPicture(t, out, 4000)
	text := extractSRT(t, out)
	if !strings.Contains(text, `Coba "cara" ini: 100% & ok`) || !strings.Contains(text, "Susah membuka tutup botol?") {
		t.Fatalf("sample subtitles:\n%s", text)
	}
	if manifest.DurationMS != 4000 || manifest.Encoder.Name != "libx264" {
		t.Fatalf("%+v", manifest.Encoder)
	}
}

func TestRenderRelativePaths(t *testing.T) {
	requireTools(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := repoRoot(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	timelinePath := filepath.Join(root, "tests", "fixtures", "timelines", "sample.json")
	assetsPath := filepath.Join(root, "tests", "fixtures", "timelines", "sample.assets.json")
	tl, err := timeline.LoadTimeline(timelinePath)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := timeline.LoadAssets(assetsPath)
	if err != nil {
		t.Fatal(err)
	}
	assets = timeline.ResolvePaths(assets, filepath.Dir(assetsPath))
	for i, asset := range assets {
		rel, err := filepath.Rel(wd, asset.Path)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.IsAbs(rel) {
			t.Fatalf("path stayed absolute: %s", rel)
		}
		assets[i].Path = rel
	}
	outAbs := filepath.Join(t.TempDir(), "relative.mp4")
	outRel, err := filepath.Rel(wd, outAbs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := media.Render(ctx, tl, assets, outRel); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outAbs)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("relative-path export is empty")
	}
}

func TestMissingVoiceDoesNotCreateExport(t *testing.T) {
	requireTools(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "out.mp4")
	tl := timeline.Timeline{SchemaVersion: timeline.SchemaVersion, OutputID: "x", Revision: 1, SKUID: "sku", Market: "ID", Locale: "id-ID"}
	if _, err := media.Render(context.Background(), tl, nil, out); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed render created an export")
	}
}

func requireTools(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		cmd := exec.Command(path, "-version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s at %s failed to start: %v\n%s", name, path, err, out)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func makeVideo(t *testing.T, path, color, size string, frames int) {
	t.Helper()
	seconds := float64(frames) / 30
	runFF(t, "-hide_banner", "-nostdin", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%s:r=30:d=%g", color, size, seconds),
		"-frames:v", strconv.Itoa(frames),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "veryfast", "-an", path)
}

func makeTone(t *testing.T, path string, freq int, seconds string) {
	t.Helper()
	runFF(t, "-hide_banner", "-nostdin", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:sample_rate=48000:duration=%s", freq, seconds),
		"-t", seconds, "-c:a", "pcm_s16le", path)
}

func runFF(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func asset(id, kind, path, sku, source string) timeline.Asset {
	return timeline.Asset{ID: id, Kind: kind, Path: path, SHA256: mustSHA(path), SKUID: sku, Source: source}
}

func mustSHA(path string) string {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func twoSegmentProject(t *testing.T, red, blue, tail, voice1, voice2, bgm, font string) (timeline.Timeline, []timeline.Asset) {
	t.Helper()
	tl := timeline.Timeline{
		SchemaVersion: timeline.SchemaVersion,
		OutputID:      "it-fixed",
		Revision:      1,
		SKUID:         "sku_demo",
		Market:        "ID",
		Locale:        "id-ID",
		Canvas:        timeline.Canvas{Width: 1080, Height: 1920, FPS: 30},
		FontAssetID:   "font",
		Segments: []timeline.Segment{
			{
				ID: "seg_1", Role: "hook", DurationMS: 2000,
				Voice:         timeline.Voice{Text: "hook", AudioID: "voice1", DurationMS: 1500},
				Shots:         []timeline.Shot{{ClipID: "red", SourceInMS: 0, SourceOutMS: 1000}, {ClipID: "blue", SourceInMS: 200, SourceOutMS: 1200}},
				OriginalAudio: timeline.OriginalMute,
				Captions:      []timeline.Caption{{StartMS: 0, EndMS: 1500, Text: `Coba "cara" ini: 100% & ok`}},
				CaptionStyle:  "id-default",
			},
			{
				ID: "seg_2", Role: "demo", DurationMS: 1000,
				Voice:         timeline.Voice{Text: "next", AudioID: "voice2", DurationMS: 1000},
				Shots:         []timeline.Shot{{ClipID: "tail", SourceInMS: 0, SourceOutMS: 1000}},
				OriginalAudio: timeline.OriginalMute,
				Captions:      []timeline.Caption{{StartMS: 0, EndMS: 1000, Text: "Lanjut."}},
			},
		},
		BGM: timeline.BGM{AssetID: "bgm", GainDB: -18},
	}
	assets := []timeline.Asset{
		asset("red", timeline.KindClip, red, "sku_demo", timeline.SourceOwned),
		asset("blue", timeline.KindClip, blue, "sku_demo", timeline.SourceOwned),
		asset("tail", timeline.KindClip, tail, "sku_demo", timeline.SourceOwned),
		asset("voice1", timeline.KindVoice, voice1, "sku_demo", timeline.SourceOwned),
		asset("voice2", timeline.KindVoice, voice2, "sku_demo", timeline.SourceOwned),
		asset("bgm", timeline.KindBGM, bgm, "", timeline.SourceLibrary),
		asset("font", timeline.KindFont, font, "", timeline.SourceLibrary),
	}
	return tl, assets
}

func renderOne(t *testing.T, ctx context.Context, clip, font string, _ any) string {
	t.Helper()
	manifest := renderOneManifest(t, ctx, clip, font)
	return manifestPathMedia(manifest)
}

func renderOneManifest(t *testing.T, ctx context.Context, clip, font string) media.Manifest {
	t.Helper()
	dir := t.TempDir()
	voice := filepath.Join(dir, "voice.wav")
	bgm := filepath.Join(dir, "bgm.wav")
	makeTone(t, voice, 440, "1.0")
	makeTone(t, bgm, 196, "0.6")
	out := filepath.Join(dir, "out.mp4")
	tl := timeline.Timeline{
		SchemaVersion: timeline.SchemaVersion,
		OutputID:      "variant",
		Revision:      1,
		SKUID:         "sku_demo",
		Market:        "ID",
		Locale:        "id-ID",
		Canvas:        timeline.Canvas{Width: 1080, Height: 1920, FPS: 30},
		FontAssetID:   "font",
		Segments: []timeline.Segment{{
			ID: "seg", Role: "demo", DurationMS: 1000,
			Voice:         timeline.Voice{AudioID: "voice", DurationMS: 1000},
			Shots:         []timeline.Shot{{ClipID: "clip", SourceInMS: 0, SourceOutMS: 1000}},
			OriginalAudio: timeline.OriginalMute,
			Captions:      []timeline.Caption{{StartMS: 0, EndMS: 1000, Text: "ok"}},
		}},
		BGM: timeline.BGM{AssetID: "bgm", GainDB: -18},
	}
	assets := []timeline.Asset{
		asset("clip", timeline.KindClip, clip, "sku_demo", timeline.SourceOwned),
		asset("voice", timeline.KindVoice, voice, "sku_demo", timeline.SourceOwned),
		asset("bgm", timeline.KindBGM, bgm, "", timeline.SourceLibrary),
		asset("font", timeline.KindFont, font, "", timeline.SourceLibrary),
	}
	manifest, err := media.Render(ctx, tl, assets, out)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Timeline.OutputID = out
	return manifest
}

func manifestPathMedia(manifest media.Manifest) string {
	return manifest.Timeline.OutputID
}

func assertPicture(t *testing.T, path string, wantMS int) {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", "-show_format", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Rate      string `json:"avg_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	var video, audio int
	var width, height int
	var rate string
	for _, stream := range doc.Streams {
		switch stream.CodecType {
		case "video":
			video++
			width, height, rate = stream.Width, stream.Height, stream.Rate
		case "audio":
			audio++
		}
	}
	if video != 1 || audio < 1 || width != 1080 || height != 1920 || rate != "30/1" {
		t.Fatalf("streams video=%d audio=%d size=%dx%d rate=%s", video, audio, width, height, rate)
	}
	seconds, err := strconv.ParseFloat(doc.Format.Duration, 64)
	if err != nil {
		t.Fatal(err)
	}
	got := int(seconds*1000 + 0.5)
	delta := got - wantMS
	if delta < 0 {
		delta = -delta
	}
	if delta > 34 {
		t.Fatalf("duration %dms, want %dms", got, wantMS)
	}
}

func extractSRT(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", path, "-map", "0:s:0", "-f", "srt", "pipe:1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func frameVaried(t *testing.T, path string) bool {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-hide_banner", "-v", "error", "-i", path, "-vf", `select=eq(n\,10)`, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("frame: %v", err)
	}
	const want = 1080 * 1920 * 3
	if len(out) != want {
		t.Fatalf("frame bytes %d, want %d", len(out), want)
	}
	var sum int
	varied := false
	r0, g0, b0 := out[0], out[1], out[2]
	for i := 0; i+2 < len(out); i += 3000 {
		sum += int(out[i])
		if out[i] != r0 || out[i+1] != g0 || out[i+2] != b0 {
			varied = true
		}
	}
	return varied && sum > 0
}

func decodedDigest(t *testing.T, path string) (string, string) {
	t.Helper()
	video := ffmpegStdout(t, "-hide_banner", "-v", "error", "-i", path, "-map", "0:v:0", "-f", "framemd5", "pipe:1")
	audio := ffmpegStdout(t, "-hide_banner", "-v", "error", "-i", path, "-map", "0:a:0", "-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1")
	return shaBytes(video), shaBytes(audio)
}

func ffmpegStdout(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes()
}

func shaBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
