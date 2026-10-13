package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const hardwareEncoder = "h264_videotoolbox"

// Encoder is one video encode configuration.
type Encoder struct {
	Name   string
	Config string
	args   func(fps int) []string
}

// Args returns the ffmpeg video arguments for this encoder.
func (e Encoder) Args(fps int) []string {
	if e.args == nil {
		return Software().Args(fps)
	}
	return e.args(fps)
}

// Software is the libx264 configuration used by the portrait baseline.
func Software() Encoder {
	return Encoder{
		Name:   EncoderName,
		Config: EncoderConfig,
		args: func(fps int) []string {
			return []string{
				"-c:v", EncoderName,
				"-preset", "veryfast",
				"-crf", "23",
				"-pix_fmt", "yuv420p",
				"-fps_mode", "cfr",
				"-r", strconv.Itoa(fps),
				"-threads", "1",
				"-x264-params", "threads=1:sliced-threads=0",
			}
		},
	}
}

// Hardware is the VideoToolbox configuration.
// It does not reuse libx264 preset, crf, or x264-params.
func Hardware() Encoder {
	return Encoder{
		Name:   hardwareEncoder,
		Config: "h264_videotoolbox bitrate=8M pix_fmt=yuv420p fps=30 size=1080x1920",
		args: func(fps int) []string {
			return []string{
				"-c:v", hardwareEncoder,
				"-b:v", "8M",
				"-pix_fmt", "yuv420p",
				"-fps_mode", "cfr",
				"-r", strconv.Itoa(fps),
			}
		},
	}
}

// Select uses hardware only when the caller asked for auto and the probe succeeded.
// Any other setting, including an unavailable encoder, stays on libx264.
func Select(getenv func(string) string, probe func(name string) error) Encoder {
	if getenv == nil || getenv("QINGFAN_ENCODER") != "auto" {
		return Software()
	}
	if probe == nil || probe(hardwareEncoder) != nil {
		return Software()
	}
	return Hardware()
}

var (
	hardwareOnce sync.Once
	hardwareOK   bool
)

// Resolved is the encoder for this process.
// The default is libx264. QINGFAN_ENCODER=auto probes VideoToolbox once and falls back.
func Resolved() Encoder {
	if os.Getenv("QINGFAN_ENCODER") != "auto" {
		return Software()
	}
	hardwareOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		hardwareOK = ProbeEncoder(ctx, hardwareEncoder) == nil
	})
	if !hardwareOK {
		return Software()
	}
	return Hardware()
}

func encoderInfo(enc Encoder) EncoderInfo {
	return EncoderInfo{Name: enc.Name, Config: enc.Config}
}

// ProbeEncoder asks ffmpeg to encode one frame with name.
func ProbeEncoder(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=64x64:r=30:d=0.1",
		"-frames:v", "1", "-c:v", name, "-pix_fmt", "yuv420p",
		"-f", "null", "-",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if len(text) > 240 {
			text = text[len(text)-240:]
		}
		return fmt.Errorf("%s: %w: %s", name, err, text)
	}
	return nil
}
