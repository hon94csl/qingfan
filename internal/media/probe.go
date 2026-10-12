package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// Info is a probed media file. DurationMS is the available length.
type Info struct {
	DurationMS        int
	Width             int
	Height            int
	Rotation          int
	HasVideo          bool
	HasAudio          bool
	AudioStreams      int
	SubtitleStreams   int
	AvgFPS            float64
	VariableFrameRate bool
}

type cmdError struct {
	Path   string
	Args   []string
	Stderr string
	Err    error
}

func (e *cmdError) Error() string {
	msg := e.Path
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *cmdError) Unwrap() error { return e.Err }

func runIn(ctx context.Context, dir, name string, args ...string) ([]byte, string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, "", err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	errText := stderr.String()
	if err != nil {
		shown := errText
		if len(shown) > 8000 {
			shown = shown[len(shown)-8000:]
		}
		return stdout.Bytes(), errText, &cmdError{Path: path, Args: args, Stderr: strings.TrimSpace(shown), Err: err}
	}
	return stdout.Bytes(), errText, nil
}

// Probe reads duration, picture, rotation, frame rate, and stream kinds.
// The file is not modified.
func Probe(ctx context.Context, path string) (Info, error) {
	out, _, err := runIn(ctx, "", "ffprobe",
		"-hide_banner", "-v", "error",
		"-print_format", "json",
		"-show_format", "-show_streams",
		path,
	)
	if err != nil {
		return Info{}, err
	}
	return parseProbe(out)
}

func parseProbe(raw []byte) (Info, error) {
	var doc probeDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Info{}, fmt.Errorf("ffprobe json: %w", err)
	}
	var info Info
	formatDur, _ := strconv.ParseFloat(string(doc.Format.Duration), 64)
	for _, stream := range doc.Streams {
		switch stream.CodecType {
		case "video":
			if info.HasVideo {
				continue
			}
			info.HasVideo = true
			info.Width = stream.Width
			info.Height = stream.Height
			info.AvgFPS = parseRate(stream.AvgFrameRate)
			real := parseRate(stream.RFrameRate)
			info.VariableFrameRate = variableRate(info.AvgFPS, real)
			info.Rotation = stream.rotation()
			if formatDur <= 0 {
				formatDur, _ = strconv.ParseFloat(string(stream.Duration), 64)
			}
		case "audio":
			info.HasAudio = true
			info.AudioStreams++
			if formatDur <= 0 {
				if sec, err := strconv.ParseFloat(string(stream.Duration), 64); err == nil && sec > formatDur {
					formatDur = sec
				}
			}
		case "subtitle":
			info.SubtitleStreams++
		}
	}
	if formatDur > 0 {
		info.DurationMS = int(math.Round(formatDur * 1000))
	}
	return info, nil
}

func variableRate(avg, real float64) bool {
	if avg <= 0 || real <= 0 {
		return false
	}
	diff := math.Abs(avg - real)
	base := math.Max(avg, real)
	return diff/base > 0.01
}

func parseRate(raw string) float64 {
	if raw == "" || raw == "0/0" {
		return 0
	}
	num, den, ok := strings.Cut(raw, "/")
	if !ok {
		v, _ := strconv.ParseFloat(raw, 64)
		return v
	}
	n, errN := strconv.ParseFloat(num, 64)
	d, errD := strconv.ParseFloat(den, 64)
	if errN != nil || errD != nil || d == 0 {
		return 0
	}
	return n / d
}

type probeDoc struct {
	Streams []probeStream `json:"streams"`
	Format  struct {
		Duration flexString `json:"duration"`
	} `json:"format"`
}

type probeStream struct {
	CodecType    string     `json:"codec_type"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	AvgFrameRate string     `json:"avg_frame_rate"`
	RFrameRate   string     `json:"r_frame_rate"`
	Duration     flexString `json:"duration"`
	Tags         probeTags  `json:"tags"`
	SideData     []sideData `json:"side_data_list"`
}

func (s probeStream) rotation() int {
	for _, side := range s.SideData {
		if strings.Contains(strings.ToLower(side.Type), "display matrix") && side.Rotation != 0 {
			return normRotation(side.Rotation)
		}
	}
	if text := strings.TrimSpace(s.Tags.Rotate); text != "" {
		v, err := strconv.ParseFloat(text, 64)
		if err == nil {
			return normRotation(v)
		}
	}
	return 0
}

type sideData struct {
	Type     string  `json:"side_data_type"`
	Rotation float64 `json:"rotation"`
}

type probeTags struct {
	Rotate string
}

func (t *probeTags) UnmarshalJSON(raw []byte) error {
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key, value := range fields {
		if strings.EqualFold(key, "rotate") {
			t.Rotate = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return nil
}

func normRotation(v float64) int {
	n := int(math.Round(v)) % 360
	if n < 0 {
		n += 360
	}
	return n
}

type flexString string

func (f *flexString) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*f = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		*f = flexString(text)
		return nil
	}
	var number json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&number); err == nil {
		*f = flexString(number.String())
		return nil
	}
	return errors.New("expected string or number")
}
