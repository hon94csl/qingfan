package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hon94csl/qingfan/internal/timeline"
)

// Cut is a half-open source interval in milliseconds.
type Cut struct {
	InMS  int `json:"in_ms"`
	OutMS int `json:"out_ms"`
}

// CachedProbe is a previous analysis of the same bytes.
type CachedProbe struct {
	SHA256        string
	Info          Info
	Cuts          []Cut
	ThumbnailPath string
}

// Ingested is the analysis for one file.
type Ingested struct {
	SHA256        string
	Info          Info
	Cuts          []Cut
	ThumbnailPath string
	Reused        bool
}

// Hash is the SHA-256 of the file bytes.
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// Ingest probes a file, extracts a thumbnail and candidate cuts.
// The same hash reuses cached and does not call FFmpeg.
func Ingest(ctx context.Context, path, kind, thumbDir string, cached *CachedProbe) (Ingested, error) {
	if err := ctx.Err(); err != nil {
		return Ingested{}, err
	}
	sum, err := Hash(path)
	if err != nil {
		return Ingested{}, err
	}
	if cached != nil && strings.EqualFold(cached.SHA256, sum) {
		return Ingested{
			SHA256: sum, Info: cached.Info, Cuts: cached.Cuts,
			ThumbnailPath: cached.ThumbnailPath, Reused: true,
		}, nil
	}
	if kind == timeline.KindFont {
		return Ingested{SHA256: sum}, nil
	}
	info, err := Probe(ctx, path)
	if err != nil {
		return Ingested{}, err
	}
	out := Ingested{SHA256: sum, Info: info}
	switch kind {
	case timeline.KindClip:
		if !info.HasVideo {
			return Ingested{}, fmt.Errorf("素材没有视频轨")
		}
		cuts, err := detectCuts(ctx, path, info.DurationMS)
		if err != nil {
			return Ingested{}, err
		}
		thumb, err := writeThumbnail(ctx, path, thumbDir, sum)
		if err != nil {
			return Ingested{}, err
		}
		out.Cuts = cuts
		out.ThumbnailPath = thumb
	case timeline.KindVoice, timeline.KindBGM:
		if !info.HasAudio {
			return Ingested{}, fmt.Errorf("素材没有音轨")
		}
	default:
		return Ingested{}, fmt.Errorf("不认识的素材类型 %s", kind)
	}
	return out, nil
}

func detectCuts(ctx context.Context, path string, durationMS int) ([]Cut, error) {
	if durationMS <= 0 {
		return nil, fmt.Errorf("视频时长无效")
	}
	_, stderr, err := runIn(ctx, "", "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "info",
		"-i", path,
		"-filter:v", "select='gt(scene,0.4)',showinfo",
		"-an", "-f", "null", "-",
	)
	if err != nil {
		return nil, err
	}
	return cutsFromPoints(scenePoints(stderr), durationMS), nil
}

var scenePoint = regexp.MustCompile(`pts_time:([0-9]+(?:\.[0-9]+)?)`)

func scenePoints(stderr string) []int {
	var points []int
	for _, match := range scenePoint.FindAllStringSubmatch(stderr, -1) {
		sec, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		points = append(points, int(sec*1000+0.5))
	}
	return points
}

func cutsFromPoints(points []int, durationMS int) []Cut {
	const frameMS = 34
	bounds := []int{0}
	last := 0
	for _, point := range points {
		if point < frameMS || point > durationMS-frameMS {
			continue
		}
		if point-last < frameMS {
			continue
		}
		bounds = append(bounds, point)
		last = point
	}
	bounds = append(bounds, durationMS)
	cuts := make([]Cut, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		if bounds[i+1] <= bounds[i] {
			continue
		}
		cuts = append(cuts, Cut{InMS: bounds[i], OutMS: bounds[i+1]})
	}
	if len(cuts) == 0 {
		return []Cut{{InMS: 0, OutMS: durationMS}}
	}
	return cuts
}

func writeThumbnail(ctx context.Context, path, thumbDir, sum string) (string, error) {
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(thumbDir, sum+".jpg")
	_, _, err := runIn(ctx, "", "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error", "-y",
		"-i", path,
		"-frames:v", "1",
		"-vf", "scale=320:-2",
		dest,
	)
	if err != nil {
		return "", err
	}
	return dest, nil
}
