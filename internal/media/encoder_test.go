package media

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestUnavailableHardwareUsesSoftware(t *testing.T) {
	got := Select(func(string) string { return "auto" }, func(string) error {
		return errors.New("simulated unavailable")
	})
	if got.Name != EncoderName || got.Config != EncoderConfig {
		t.Fatalf("%s %s", got.Name, got.Config)
	}
	joined := strings.Join(got.Args(30), " ")
	if !strings.Contains(joined, "libx264") || !strings.Contains(joined, "-crf 23") || strings.Contains(joined, hardwareEncoder) {
		t.Fatal(joined)
	}
}

func TestHardwareArgsOmitX264Options(t *testing.T) {
	got := Select(func(string) string { return "auto" }, func(name string) error {
		if name == hardwareEncoder {
			return nil
		}
		return errors.New("missing")
	})
	if got.Name != hardwareEncoder {
		t.Fatal(got.Name)
	}
	joined := strings.Join(got.Args(30), " ")
	for _, unexpected := range []string{"-crf", "-preset", "x264-params", "libx264"} {
		if strings.Contains(joined, unexpected) {
			t.Fatal(joined)
		}
	}
	if !strings.Contains(joined, "-c:v "+hardwareEncoder) || !strings.Contains(joined, "-b:v 8M") {
		t.Fatal(joined)
	}
}

func TestDefaultEncoderIgnoresASuccessfulProbe(t *testing.T) {
	got := Select(func(string) string { return "" }, func(string) error { return nil })
	if got.Name != EncoderName {
		t.Fatal(got.Name)
	}
}

func TestInstalledFFmpegProbeFallsBackOrUsesVideoToolbox(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "ffmpeg", "-version").Run(); err != nil {
		t.Fatal(err)
	}
	err := ProbeEncoder(ctx, hardwareEncoder)
	got := Select(func(string) string { return "auto" }, func(string) error { return err })
	joined := strings.Join(got.Args(30), " ")
	if err != nil {
		if got.Name != EncoderName || strings.Contains(joined, hardwareEncoder) {
			t.Fatalf("probe %v encoder %s args %s", err, got.Name, joined)
		}
		t.Logf("videotoolbox unavailable, fallback libx264: %v", err)
		return
	}
	if got.Name != hardwareEncoder || strings.Contains(joined, "-crf") || strings.Contains(joined, "x264-params") {
		t.Fatalf("probe succeeded but args are %s", joined)
	}
	t.Logf("videotoolbox probe encoded one frame")
}
