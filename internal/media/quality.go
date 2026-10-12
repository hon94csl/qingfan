package media

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/hon94csl/qingfan/internal/timeline"
)

// QualityReport is the machine check stored beside a successful export.
// Black frames and silence are recorded. They do not by themselves fail the export.
type QualityReport struct {
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	FPS          float64  `json:"fps"`
	DurationMS   int      `json:"duration_ms"`
	AudioStreams int      `json:"audio_streams"`
	Subtitles    []string `json:"subtitles"`
	BlackFrames  []string `json:"black_frames"`
	Silence      []string `json:"silence"`
}

func inspect(ctx context.Context, path string, tl timeline.Timeline) (QualityReport, error) {
	info, err := Probe(ctx, path)
	if err != nil {
		return QualityReport{}, err
	}
	srtOut, _, err := runIn(ctx, "", "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", path,
		"-map", "0:s:0",
		"-f", "srt", "pipe:1",
	)
	if err != nil {
		return QualityReport{}, fmt.Errorf("read subtitles: %w", err)
	}
	got, err := ParseSRT(string(srtOut))
	if err != nil {
		return QualityReport{}, err
	}
	want, err := timeline.AbsoluteCues(tl)
	if err != nil {
		return QualityReport{}, err
	}
	if err := matchCues(got, want, tl); err != nil {
		return QualityReport{}, err
	}
	_, blackErr, err := runIn(ctx, "", "ffmpeg",
		"-hide_banner", "-nostdin",
		"-i", path,
		"-vf", "blackframe=amount=98:threshold=32",
		"-an", "-f", "null", "-",
	)
	if err != nil {
		return QualityReport{}, fmt.Errorf("blackframe: %w", err)
	}
	_, silenceErr, err := runIn(ctx, "", "ffmpeg",
		"-hide_banner", "-nostdin",
		"-i", path,
		"-af", "silencedetect=n=-50dB:d=0.2",
		"-vn", "-f", "null", "-",
	)
	if err != nil {
		return QualityReport{}, fmt.Errorf("silencedetect: %w", err)
	}
	texts := make([]string, len(got))
	for i, cue := range got {
		texts[i] = cue.Text
	}
	report := QualityReport{
		Width:        info.Width,
		Height:       info.Height,
		FPS:          info.AvgFPS,
		DurationMS:   info.DurationMS,
		AudioStreams: info.AudioStreams,
		Subtitles:    texts,
		BlackFrames:  matchingLines(blackErr, "pblack:"),
		Silence:      matchingLines(silenceErr, "silence_"),
	}
	if err := checkPicture(report, tl); err != nil {
		return report, err
	}
	return report, nil
}

func matchCues(got, want []timeline.Cue, tl timeline.Timeline) error {
	if len(got) != len(want) {
		return fmt.Errorf("subtitle count %d, want %d", len(got), len(want))
	}
	starts, err := timeline.SegmentStarts(tl)
	if err != nil {
		return err
	}
	for i := range want {
		if got[i].Text != want[i].Text || got[i].StartMS != want[i].StartMS || got[i].EndMS != want[i].EndMS {
			return fmt.Errorf("subtitle %d got %q %d-%d, want %q %d-%d", i, got[i].Text, got[i].StartMS, got[i].EndMS, want[i].Text, want[i].StartMS, want[i].EndMS)
		}
		seg := 0
		for seg+1 < len(starts) && got[i].StartMS >= starts[seg+1] {
			seg++
		}
		segmentEnd := starts[seg] + tl.Segments[seg].DurationMS
		if got[i].EndMS > segmentEnd {
			return fmt.Errorf("subtitle %d ends %dms past segment %s", i, got[i].EndMS, tl.Segments[seg].ID)
		}
	}
	return nil
}

func checkPicture(report QualityReport, tl timeline.Timeline) error {
	if report.Width != tl.Canvas.Width || report.Height != tl.Canvas.Height {
		return fmt.Errorf("picture %dx%d, want %dx%d", report.Width, report.Height, tl.Canvas.Width, tl.Canvas.Height)
	}
	if math.Abs(report.FPS-float64(tl.Canvas.FPS)) > 0.01 {
		return fmt.Errorf("fps %v, want %d", report.FPS, tl.Canvas.FPS)
	}
	if report.AudioStreams < 1 {
		return fmt.Errorf("missing audio stream")
	}
	if len(report.Subtitles) == 0 {
		return fmt.Errorf("missing subtitles")
	}
	total, err := timeline.TotalDurationMS(tl)
	if err != nil {
		return err
	}
	if !withinOneFrame(report.DurationMS, total, tl.Canvas.FPS) {
		return fmt.Errorf("duration %dms, want %dms within one frame", report.DurationMS, total)
	}
	return nil
}

func withinOneFrame(actual, expected, fps int) bool {
	if fps <= 0 {
		return false
	}
	limit := (1000 + fps - 1) / fps
	delta := actual - expected
	if delta < 0 {
		delta = -delta
	}
	return delta <= limit
}

func matchingLines(raw, marker string) []string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, marker) {
			lines = append(lines, line)
		}
	}
	if lines == nil {
		return []string{}
	}
	return lines
}
