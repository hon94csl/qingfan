package media

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, status := range CheckTools(context.Background()) {
		if status.Error == "" || status.Version != "" {
			t.Fatalf("unexpected status: %+v", status)
		}
	}
}

func TestToolExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, tc := range []struct{ name, script, version, errorText string }{
		{"version", "printf 'tool v1\\nextra\\n'", "tool v1", ""},
		{"failure", "echo missing-library >&2; exit 1", "", "missing-library"},
		{"empty", "exit 0", "", "empty version output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test-tool")
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			status := checkTool(context.Background(), "test-tool")
			if status.Version != tc.version || !strings.Contains(status.Error, tc.errorText) || (tc.errorText == "" && status.Error != "") {
				t.Fatalf("unexpected status: %+v", status)
			}
		})
	}
}
