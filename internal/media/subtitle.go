package media

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hon94csl/qingfan/internal/timeline"
)

func sec(ms int) string {
	if ms < 0 {
		return "-" + sec(-ms)
	}
	return fmt.Sprintf("%d.%03d", ms/1000, ms%1000)
}

func srtStamp(ms int) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / int(time.Hour/time.Millisecond)
	ms -= h * int(time.Hour/time.Millisecond)
	m := ms / int(time.Minute/time.Millisecond)
	ms -= m * int(time.Minute/time.Millisecond)
	s := ms / int(time.Second/time.Millisecond)
	milli := ms - s*int(time.Second/time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, milli)
}

func parseSRTStamp(raw string) (int, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, ".", ","))
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return 0, fmt.Errorf("srt time %q", raw)
	}
	clock := strings.Split(parts[0], ":")
	if len(clock) != 3 {
		return 0, fmt.Errorf("srt time %q", raw)
	}
	h, errH := strconv.Atoi(clock[0])
	m, errM := strconv.Atoi(clock[1])
	s, errS := strconv.Atoi(clock[2])
	milli, errMS := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || errS != nil || errMS != nil {
		return 0, fmt.Errorf("srt time %q", raw)
	}
	return ((h*60+m)*60+s)*1000 + milli, nil
}

// WriteSRT writes cues. Caption text is file content, not a filter expression.
func WriteSRT(w io.Writer, cues []timeline.Cue) error {
	for i, cue := range cues {
		if strings.ContainsAny(cue.Text, "\r\n") {
			return fmt.Errorf("caption %d contains a line break", i)
		}
		if _, err := fmt.Fprintf(w, "%d\n%s --> %s\n%s\n\n", i+1, srtStamp(cue.StartMS), srtStamp(cue.EndMS), cue.Text); err != nil {
			return err
		}
	}
	return nil
}

// ParseSRT reads cues produced by WriteSRT or ffmpeg's srt muxer.
func ParseSRT(raw string) ([]timeline.Cue, error) {
	raw = strings.TrimPrefix(raw, "\uFEFF")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var cues []timeline.Cue
	for _, block := range strings.Split(raw, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) < 3 {
			return nil, fmt.Errorf("srt block %q", block)
		}
		times := strings.Split(lines[1], " --> ")
		if len(times) != 2 {
			return nil, fmt.Errorf("srt time %q", lines[1])
		}
		start, err := parseSRTStamp(times[0])
		if err != nil {
			return nil, err
		}
		end, err := parseSRTStamp(times[1])
		if err != nil {
			return nil, err
		}
		cues = append(cues, timeline.Cue{StartMS: start, EndMS: end, Text: strings.Join(lines[2:], "\n")})
	}
	return cues, nil
}

type drawCaption struct {
	File    string
	StartMS int
	EndMS   int
}

// DrawtextFilter burns captions from text files. The caption text itself is
// not inserted into the filter graph.
func DrawtextFilter(font string, captions []drawCaption) (string, error) {
	if !safeToken(font) {
		return "", fmt.Errorf("font file name %q is not used as a filter token", font)
	}
	parts := make([]string, 0, len(captions))
	for _, cap := range captions {
		if !safeToken(cap.File) {
			return "", fmt.Errorf("caption file name %q is not used as a filter token", cap.File)
		}
		parts = append(parts, fmt.Sprintf(
			"drawtext=fontfile=%s:textfile=%s:expansion=none:fontsize=54:fontcolor=white:borderw=3:bordercolor=black:x=(w-text_w)/2:y=h*0.78:enable='gte(t\\,%s)*lt(t\\,%s)'",
			font, cap.File, sec(cap.StartMS), sec(cap.EndMS),
		))
	}
	return strings.Join(parts, ","), nil
}

func safeToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// mixFilter trims voice and looped BGM to the timeline, then applies gainDB.
func mixFilter(gainDB float64, durationMS int) string {
	dur := sec(durationMS)
	gain := strconv.FormatFloat(gainDB, 'f', -1, 64)
	return fmt.Sprintf(
		"[0:a]aformat=sample_fmts=fltp:channel_layouts=stereo,aresample=48000,apad,atrim=0:%s,asetpts=PTS-STARTPTS[vo];"+
			"[1:a]aformat=sample_fmts=fltp:channel_layouts=stereo,aresample=48000,atrim=0:%s,asetpts=PTS-STARTPTS,volume=%sdB[bg];"+
			"[vo][bg]amix=inputs=2:duration=first:dropout_transition=0:normalize=0[a]",
		dur, dur, gain,
	)
}

func canvasFilter(canvas timeline.Canvas) string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,fps=%d,setsar=1,format=yuv420p",
		canvas.Width, canvas.Height, canvas.Width, canvas.Height, canvas.FPS,
	)
}
