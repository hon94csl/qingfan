package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// SegmentPrint is the cache identity of one spoken segment.
// VoiceKey covers the spoken text and voice. ShotKey covers the clip and crop.
type SegmentPrint struct {
	ID       string
	VoiceKey string
	ShotKey  string
}

// Fingerprint is the cache identity of one cut.
// Keys include input hashes and the prompt or render version supplied by the caller.
type Fingerprint struct {
	AnalysisKey string
	RenderKey   string
	BGMKey      string
	CaptionKey  string
	Segments    []SegmentPrint
}

// Redo says which finished work can no longer be reused.
type Redo struct {
	Analyze  bool
	TTS      []string
	Assemble []string
	Mix      bool
}

// Key hashes the parts so a cache filename does not contain the spoken text.
func Key(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		sum := sha256.Sum256([]byte(part))
		h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Diff compares two fingerprints.
// A zero before value means every part of after still has to be produced.
// Changing one segment does not redo the other segments.
func Diff(before, after Fingerprint) Redo {
	if blank(before) {
		redo := Redo{Analyze: true, Mix: true}
		for _, seg := range after.Segments {
			redo.TTS = append(redo.TTS, seg.ID)
			redo.Assemble = append(redo.Assemble, seg.ID)
		}
		return redo
	}
	var redo Redo
	if before.AnalysisKey != after.AnalysisKey {
		redo.Analyze = true
	}
	if before.RenderKey != after.RenderKey || before.BGMKey != after.BGMKey || before.CaptionKey != after.CaptionKey {
		redo.Mix = true
	}
	old := make(map[string]SegmentPrint, len(before.Segments))
	for _, seg := range before.Segments {
		old[seg.ID] = seg
	}
	seen := make(map[string]bool, len(after.Segments))
	for _, seg := range after.Segments {
		seen[seg.ID] = true
		prev, ok := old[seg.ID]
		if !ok || prev.VoiceKey != seg.VoiceKey {
			redo.TTS = append(redo.TTS, seg.ID)
			redo.Assemble = append(redo.Assemble, seg.ID)
			redo.Mix = true
			continue
		}
		if prev.ShotKey != seg.ShotKey {
			redo.Assemble = append(redo.Assemble, seg.ID)
			redo.Mix = true
		}
	}
	if len(before.Segments) != len(after.Segments) {
		redo.Mix = true
	}
	for _, seg := range before.Segments {
		if !seen[seg.ID] {
			redo.Mix = true
		}
	}
	return redo
}

func blank(fp Fingerprint) bool {
	return fp.AnalysisKey == "" && fp.RenderKey == "" && fp.BGMKey == "" && fp.CaptionKey == "" && len(fp.Segments) == 0
}

// Publish writes a temporary sibling, then renames it only after write succeeds.
// A failed write removes the temporary file and leaves an existing destination unchanged.
func Publish(dst string, write func(path string) error) error {
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	partial := dst + ".partial"
	if err := write(partial); err != nil {
		os.Remove(partial)
		return err
	}
	return os.Rename(partial, dst)
}
