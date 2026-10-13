package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestCutsFromSceneLog(t *testing.T) {
	points := scenePoints("n:1 pts_time:1.5\nn:2 pts_time:0.01\n")
	if len(points) != 2 || points[0] != 1500 || points[1] != 10 {
		t.Fatalf("%v", points)
	}
	cuts := cutsFromPoints([]int{1500, 10, 1500}, 3000)
	if len(cuts) != 2 || cuts[0] != (Cut{InMS: 0, OutMS: 1500}) || cuts[1] != (Cut{InMS: 1500, OutMS: 3000}) {
		t.Fatalf("%+v", cuts)
	}
	full := cutsFromPoints(nil, 4000)
	if len(full) != 1 || full[0].OutMS != 4000 {
		t.Fatalf("%+v", full)
	}
}

func TestIngestReusesCachedBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.bin")
	if err := os.WriteFile(path, []byte("same-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := Hash(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Ingest(context.Background(), path, timeline.KindClip, dir, &CachedProbe{
		SHA256: sum, Info: Info{DurationMS: 10, HasVideo: true}, Cuts: []Cut{{OutMS: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Reused || got.SHA256 != sum || got.Info.DurationMS != 10 {
		t.Fatalf("%+v", got)
	}
}

func TestIngestFontDoesNotProbe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(path, []byte("font-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Ingest(context.Background(), path, timeline.KindFont, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reused || got.SHA256 == "" || got.Info.DurationMS != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestIngestRealClip(t *testing.T) {
	requireMedia(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-nostdin", "-y", "-f", "lavfi", "-i", "color=c=0xCC3322:s=160x120:r=30:d=1", "-frames:v", "30", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-an", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	ctx := context.Background()
	first, err := Ingest(ctx, path, timeline.KindClip, filepath.Join(dir, "thumbs"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.Info.DurationMS < 900 || len(first.Cuts) == 0 || first.ThumbnailPath == "" {
		t.Fatalf("%+v", first)
	}
	if _, err := os.Stat(first.ThumbnailPath); err != nil {
		t.Fatal(err)
	}
	if first.Cuts[0].InMS != 0 || first.Cuts[len(first.Cuts)-1].OutMS != first.Info.DurationMS {
		t.Fatalf("cuts do not cover the file: %+v duration %d", first.Cuts, first.Info.DurationMS)
	}
	second, err := Ingest(ctx, path, timeline.KindClip, filepath.Join(dir, "thumbs"), &CachedProbe{
		SHA256: first.SHA256, Info: first.Info, Cuts: first.Cuts, ThumbnailPath: first.ThumbnailPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.SHA256 != first.SHA256 {
		t.Fatalf("%+v", second)
	}
}

func requireMedia(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		out, err := exec.Command(path, "-version").CombinedOutput()
		if err != nil {
			t.Fatalf("%s at %s failed to start: %v\n%s", name, path, err, out)
		}
	}
}
