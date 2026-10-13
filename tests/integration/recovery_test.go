package integration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/reporting"
	"github.com/hon94csl/qingfan/internal/store"
)

func TestRecoveryKeepsExportsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(dir, "original.mp4")
	exported := filepath.Join(dir, "export.mp4")
	if err := os.WriteFile(original, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exported, []byte("export-ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := jobs.Publish(exported, func(path string) error {
		if writeErr := os.WriteFile(path, []byte("torn"), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
		return errors.New("disk full")
	})
	if err == nil {
		t.Fatal("expected a failed publish")
	}
	ready := filepath.Join(cache, "ready-step")
	if err := jobs.Publish(ready, func(path string) error {
		return os.WriteFile(path, []byte("voice"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "stale-step"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, filepath.Join(cache, "linked-original")); err != nil {
		t.Fatal(err)
	}

	db := filepath.Join(dir, "qingfan.sqlite")
	s, err := store.Open(db, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindPublication("missing", "", "7123", "shop-a", "2026-10-13T00:00:00Z"); err != reporting.ErrOutput {
		s.Close()
		t.Fatal(err)
	}
	if err := s.PutStep("job-1", "running", jobs.Running, ready, ""); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.PutStep("job-1", "done", jobs.Succeeded, ready, ""); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(db, cache)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	steps, err := reopened.RecoverJob("job-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].Status != jobs.Pending || steps[0].Artifact != "" || steps[1].Status != jobs.Succeeded || steps[1].Artifact != ready {
		t.Fatalf("%+v", steps)
	}
	if _, err := jobs.Cleanup(cache, map[string]bool{"ready-step": true}, map[string]bool{original: true, exported: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "stale-step")); !os.IsNotExist(err) {
		t.Fatal("stale step remained")
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{original: "original", exported: "export-ok"} {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != want {
			t.Fatalf("%s is %q (%v)", path, body, err)
		}
	}
}
