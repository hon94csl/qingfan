package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hon94csl/qingfan/internal/timeline"
)

const (
	RendererName    = "qingfan-render"
	RendererVersion = "m1-libx264-1"
	EncoderName     = "libx264"
	EncoderConfig   = "libx264 preset=veryfast crf=23 pix_fmt=yuv420p fps=30 size=1080x1920 threads=1"
)

// Manifest is the immutable record written next to a successful export.
type Manifest struct {
	SchemaVersion int               `json:"schema_version"`
	OutputID      string            `json:"output_id"`
	Revision      int               `json:"revision"`
	SKUID         string            `json:"sku_id"`
	Timeline      timeline.Timeline `json:"timeline"`
	Resources     []Resource        `json:"resources"`
	Renderer      BuildInfo         `json:"renderer"`
	FFmpeg        BuildInfo         `json:"ffmpeg"`
	Encoder       EncoderInfo       `json:"encoder"`
	DurationMS    int               `json:"duration_ms"`
	Quality       QualityReport     `json:"quality"`
	Notes         []string          `json:"notes,omitempty"`
}

type Resource struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type BuildInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path,omitempty"`
}

type EncoderInfo struct {
	Name   string `json:"name"`
	Config string `json:"config"`
}

// ManifestPath is the JSON record published beside an mp4.
func ManifestPath(outputPath string) string {
	return strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".manifest.json"
}

// Render validates the timeline, encodes a portrait mp4, and publishes it
// only after inspection. A failure leaves any existing output untouched.
func Render(ctx context.Context, tl timeline.Timeline, assets []timeline.Asset, outputPath string) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	if outputPath == "" {
		return Manifest{}, fmt.Errorf("output path is required")
	}
	// FFmpeg runs inside a temp directory, so every source and the export must be absolute.
	var err error
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return Manifest{}, err
	}
	assets, err = absoluteAssets(assets)
	if err != nil {
		return Manifest{}, err
	}
	if err := timeline.Validate(tl, assets, timeline.Check{SkipBounds: true}); err != nil {
		return Manifest{}, err
	}
	resolved, notes, err := probeAssets(ctx, tl, assets)
	if err != nil {
		return Manifest{}, err
	}
	if err := timeline.Validate(tl, resolved, timeline.Check{}); err != nil {
		return Manifest{}, err
	}
	ff, err := toolVersion(ctx, "ffmpeg")
	if err != nil {
		return Manifest{}, err
	}
	if _, err := toolVersion(ctx, "ffprobe"); err != nil {
		return Manifest{}, err
	}
	total, err := timeline.TotalDurationMS(tl)
	if err != nil {
		return Manifest{}, err
	}
	if dir := filepath.Dir(outputPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Manifest{}, err
		}
	}

	work, err := os.MkdirTemp("", "qingfan-render-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(work)

	partial := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".partial.mp4"
	published := false
	defer func() {
		if !published {
			os.Remove(partial)
		}
	}()
	if err := renderVideo(ctx, work, tl, resolved); err != nil {
		return Manifest{}, err
	}
	if err := renderAudio(ctx, work, tl, resolved, total); err != nil {
		return Manifest{}, err
	}
	if err := burnSubtitles(ctx, work, tl, resolved); err != nil {
		return Manifest{}, err
	}
	if err := mux(ctx, work, partial, total); err != nil {
		return Manifest{}, err
	}
	report, err := inspect(ctx, partial, tl)
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{
		SchemaVersion: timeline.SchemaVersion,
		OutputID:      tl.OutputID,
		Revision:      tl.Revision,
		SKUID:         tl.SKUID,
		Timeline:      tl,
		Resources:     resources(tl, resolved),
		Renderer:      BuildInfo{Name: RendererName, Version: RendererVersion},
		FFmpeg:        ff,
		Encoder:       encoderInfo(Resolved()),
		DurationMS:    total,
		Quality:       report,
		Notes:         notes,
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	manifestPartial := ManifestPath(outputPath) + ".partial"
	if err := os.WriteFile(manifestPartial, append(body, '\n'), 0o644); err != nil {
		return Manifest{}, err
	}
	// Replace the previous export only after the new file has passed inspection.
	if err := os.Rename(partial, outputPath); err != nil {
		os.Remove(manifestPartial)
		return Manifest{}, err
	}
	published = true
	if err := os.Rename(manifestPartial, ManifestPath(outputPath)); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func absoluteAssets(assets []timeline.Asset) ([]timeline.Asset, error) {
	out := make([]timeline.Asset, len(assets))
	for i, asset := range assets {
		out[i] = asset
		if asset.Path == "" || filepath.IsAbs(asset.Path) {
			continue
		}
		abs, err := filepath.Abs(asset.Path)
		if err != nil {
			return nil, err
		}
		out[i].Path = abs
	}
	return out, nil
}

func probeAssets(ctx context.Context, tl timeline.Timeline, assets []timeline.Asset) ([]timeline.Asset, []string, error) {
	needed := map[string]bool{}
	needed[tl.FontAssetID] = true
	needed[tl.BGM.AssetID] = true
	for _, seg := range tl.Segments {
		needed[seg.Voice.AudioID] = true
		for _, shot := range seg.Shots {
			needed[shot.ClipID] = true
		}
	}
	resolved := append([]timeline.Asset(nil), assets...)
	var notes []string
	for i, asset := range resolved {
		if !needed[asset.ID] {
			continue
		}
		if asset.Kind == timeline.KindFont {
			continue
		}
		info, err := Probe(ctx, asset.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("probe %s: %w", asset.ID, err)
		}
		switch asset.Kind {
		case timeline.KindClip:
			if !info.HasVideo {
				return nil, nil, fmt.Errorf("unsupported: clip %s has no video stream", asset.ID)
			}
			if !info.HasAudio {
				notes = append(notes, fmt.Sprintf("clip %s has no audio; original audio stays muted", asset.ID))
			}
			if info.VariableFrameRate {
				notes = append(notes, fmt.Sprintf("clip %s variable frame rate conformed to %d fps", asset.ID, tl.Canvas.FPS))
			}
			if info.Rotation != 0 {
				notes = append(notes, fmt.Sprintf("clip %s display rotation %d applied", asset.ID, info.Rotation))
			}
			width, height := info.Width, info.Height
			if info.Rotation == 90 || info.Rotation == 270 {
				width, height = height, width
			}
			if width > height {
				notes = append(notes, fmt.Sprintf("clip %s landscape %dx%d scaled into the canvas", asset.ID, info.Width, info.Height))
			}
		case timeline.KindVoice, timeline.KindBGM:
			if !info.HasAudio {
				return nil, nil, fmt.Errorf("unsupported: %s %s has no audio stream", asset.Kind, asset.ID)
			}
		}
		resolved[i].DurationMS = info.DurationMS
	}
	return resolved, notes, nil
}

func renderVideo(ctx context.Context, work string, tl timeline.Timeline, assets []timeline.Asset) error {
	index := indexAssets(assets)
	var list strings.Builder
	n := 0
	for _, seg := range tl.Segments {
		for _, shot := range seg.Shots {
			name := fmt.Sprintf("shot-%03d.mp4", n)
			n++
			clip := index[shot.ClipID]
			args := []string{
				"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
				"-i", clip.Path,
				"-ss", sec(shot.SourceInMS),
				"-to", sec(shot.SourceOutMS),
				"-vf", canvasFilter(tl.Canvas),
				"-an",
			}
			args = appendVideoEncode(args, tl.Canvas.FPS)
			args = append(args, name)
			if _, _, err := runIn(ctx, work, "ffmpeg", args...); err != nil {
				return err
			}
			fmt.Fprintf(&list, "file '%s'\n", name)
		}
	}
	if err := os.WriteFile(filepath.Join(work, "shots.txt"), []byte(list.String()), 0o644); err != nil {
		return err
	}
	_, _, err := runIn(ctx, work, "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", "shots.txt",
		"-c", "copy", "-an", "video.mp4",
	)
	return err
}

func renderAudio(ctx context.Context, work string, tl timeline.Timeline, assets []timeline.Asset, total int) error {
	index := indexAssets(assets)
	var list strings.Builder
	for i, seg := range tl.Segments {
		name := fmt.Sprintf("voice-%03d.wav", i)
		voice := index[seg.Voice.AudioID]
		filter := fmt.Sprintf(
			"atrim=start=0:end=%s,apad=whole_dur=%s,asetpts=PTS-STARTPTS,aformat=sample_fmts=s16:channel_layouts=stereo,aresample=48000",
			sec(seg.Voice.DurationMS), sec(seg.DurationMS),
		)
		if _, _, err := runIn(ctx, work, "ffmpeg",
			"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
			"-i", voice.Path,
			"-af", filter,
			"-t", sec(seg.DurationMS),
			"-c:a", "pcm_s16le",
			name,
		); err != nil {
			return err
		}
		fmt.Fprintf(&list, "file '%s'\n", name)
	}
	if err := os.WriteFile(filepath.Join(work, "voices.txt"), []byte(list.String()), 0o644); err != nil {
		return err
	}
	if _, _, err := runIn(ctx, work, "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", "voices.txt",
		"-c", "copy", "voice.wav",
	); err != nil {
		return err
	}
	bgm := index[tl.BGM.AssetID]
	_, _, err := runIn(ctx, work, "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", "voice.wav",
		"-stream_loop", "-1", "-i", bgm.Path,
		"-filter_complex", mixFilter(tl.BGM.GainDB, total),
		"-map", "[a]",
		"-c:a", "aac", "-b:a", "160k", "-ar", "48000", "-ac", "2",
		"-t", sec(total),
		"mix.m4a",
	)
	return err
}

func burnSubtitles(ctx context.Context, work string, tl timeline.Timeline, assets []timeline.Asset) error {
	cues, err := timeline.AbsoluteCues(tl)
	if err != nil {
		return err
	}
	srt, err := os.Create(filepath.Join(work, "captions.srt"))
	if err != nil {
		return err
	}
	writeErr := WriteSRT(srt, cues)
	closeErr := srt.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	font := indexAssets(assets)[tl.FontAssetID]
	if err := copyFile(font.Path, filepath.Join(work, "font.ttf")); err != nil {
		return err
	}
	draws := make([]drawCaption, len(cues))
	for i, cue := range cues {
		name := fmt.Sprintf("cap-%d.txt", i)
		if err := os.WriteFile(filepath.Join(work, name), []byte(cue.Text), 0o644); err != nil {
			return err
		}
		draws[i] = drawCaption{File: name, StartMS: cue.StartMS, EndMS: cue.EndMS}
	}
	filter, err := DrawtextFilter("font.ttf", draws)
	if err != nil {
		return err
	}
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", "video.mp4",
		"-vf", filter,
		"-an",
	}
	args = appendVideoEncode(args, tl.Canvas.FPS)
	args = append(args, "burned.mp4")
	_, _, err = runIn(ctx, work, "ffmpeg", args...)
	return err
}

func mux(ctx context.Context, work, partial string, total int) error {
	_, _, err := runIn(ctx, work, "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", "burned.mp4",
		"-i", "mix.m4a",
		"-i", "captions.srt",
		"-map", "0:v:0", "-map", "1:a:0", "-map", "2:0",
		"-c:v", "copy", "-c:a", "copy", "-c:s", "mov_text",
		"-metadata:s:s:0", "language=ind",
		"-movflags", "+faststart",
		"-t", sec(total),
		partial,
	)
	return err
}

func appendVideoEncode(args []string, fps int) []string {
	return append(args, Resolved().Args(fps)...)
}

func resources(tl timeline.Timeline, assets []timeline.Asset) []Resource {
	want := map[string]bool{tl.FontAssetID: true, tl.BGM.AssetID: true}
	for _, seg := range tl.Segments {
		want[seg.Voice.AudioID] = true
		for _, shot := range seg.Shots {
			want[shot.ClipID] = true
		}
	}
	var out []Resource
	for _, kind := range []string{timeline.KindClip, timeline.KindVoice, timeline.KindBGM, timeline.KindFont} {
		for _, asset := range assets {
			if asset.Kind == kind && want[asset.ID] {
				out = append(out, Resource{ID: asset.ID, Kind: asset.Kind, Path: asset.Path, SHA256: asset.SHA256})
			}
		}
	}
	return out
}

func indexAssets(assets []timeline.Asset) map[string]timeline.Asset {
	index := make(map[string]timeline.Asset, len(assets))
	for _, asset := range assets {
		index[asset.ID] = asset
	}
	return index
}

func toolVersion(ctx context.Context, name string) (BuildInfo, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return BuildInfo{}, err
	}
	out, _, err := runIn(ctx, "", name, "-version")
	if err != nil {
		return BuildInfo{}, err
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if line == "" {
		return BuildInfo{}, fmt.Errorf("%s: empty version output", name)
	}
	return BuildInfo{Name: name, Version: line, Path: path}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
