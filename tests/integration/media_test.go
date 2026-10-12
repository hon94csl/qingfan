package integration

import (
	"context"
	"github.com/hon94csl/qingfan/internal/media"
	"os/exec"
	"testing"
)

func TestInstalledMediaTools(t *testing.T) {
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s is not installed", name)
		}
	}
	for _, status := range media.CheckTools(context.Background()) {
		if status.Error != "" || status.Version == "" {
			t.Fatalf("tool failed: %+v", status)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, status := range media.CheckTools(ctx) {
		if status.Error == "" {
			t.Fatalf("canceled check succeeded: %+v", status)
		}
	}
}
