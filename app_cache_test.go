package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/jobs"
)

func TestCachedVoiceSkipsTheProvider(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QINGFAN_DATA", dir)
	const spoken = "Lihat barang ini."
	path := voiceCachePath("id-female-1", spoken)
	if strings.Contains(path, spoken) || !strings.Contains(path, filepath.Join("cache", "steps")) {
		t.Fatal(path)
	}
	if err := jobs.Publish(path, func(partial string) error {
		return os.WriteFile(partial, []byte("RIFF"), 0o644)
	}); err != nil {
		t.Fatal(err)
	}
	wav, err := loadVoice(context.Background(), nil, "", "", "id-female-1", ai.ScriptSegment{ID: "seg_1", VoiceText: spoken}, path)
	if err != nil || string(wav) != "RIFF" {
		t.Fatalf("%q %v", wav, err)
	}
	if err := jobs.Publish(path, func(string) error { return errors.New("cancelled") }); err == nil {
		t.Fatal("expected cancel")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "RIFF" {
		t.Fatalf("cache lost: %q %v", body, err)
	}
}
