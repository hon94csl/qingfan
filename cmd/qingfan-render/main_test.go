package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsage(t *testing.T) {
	if run([]string{"qingfan-render"}) != 2 || run([]string{"qingfan-render", "nope"}) != 2 {
		t.Fatal("expected usage status")
	}
	if run([]string{"qingfan-render", "render"}) != 2 {
		t.Fatal("expected missing flags")
	}
}

func TestDoctorMissingTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if run([]string{"qingfan-render", "doctor"}) != 1 {
		t.Fatal("expected doctor failure")
	}
}

func TestRenderMissingTimeline(t *testing.T) {
	dir := t.TempDir()
	code := run([]string{
		"qingfan-render", "render",
		"-timeline", filepath.Join(dir, "missing.json"),
		"-assets", filepath.Join(dir, "missing-assets.json"),
		"-output", filepath.Join(dir, "out.mp4"),
	})
	if code != 1 {
		t.Fatalf("status %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.mp4")); !os.IsNotExist(err) {
		t.Fatal("render created an output for a missing timeline")
	}
}
