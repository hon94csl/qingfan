package timeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const SchemaVersion = 1

const (
	KindClip  = "clip"
	KindVoice = "voice"
	KindBGM   = "bgm"
	KindFont  = "font"
)

const (
	SourceOwned     = "owned"
	SourceLibrary   = "library"
	SourceReference = "reference"
)

const OriginalMute = "mute"

// Timeline is the frozen render timeline v1.
// Times are integer milliseconds and intervals are half-open.
type Timeline struct {
	SchemaVersion int       `json:"schema_version"`
	OutputID      string    `json:"output_id"`
	Revision      int       `json:"revision"`
	SKUID         string    `json:"sku_id"`
	Market        string    `json:"market"`
	Locale        string    `json:"locale"`
	Canvas        Canvas    `json:"canvas"`
	FontAssetID   string    `json:"font_asset_id"`
	Recipe        Recipe    `json:"recipe"`
	Segments      []Segment `json:"segments"`
	BGM           BGM       `json:"bgm"`
}

type Canvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

type Recipe struct {
	ReferenceID   string `json:"reference_id,omitempty"`
	HookType      string `json:"hook_type,omitempty"`
	PromptVersion string `json:"prompt_version,omitempty"`
}

type Segment struct {
	ID            string    `json:"id"`
	Role          string    `json:"role"`
	DurationMS    int       `json:"duration_ms"`
	Voice         Voice     `json:"voice"`
	Shots         []Shot    `json:"shots"`
	OriginalAudio string    `json:"original_audio"`
	Captions      []Caption `json:"captions"`
	CaptionStyle  string    `json:"caption_style,omitempty"`
}

type Voice struct {
	Text       string `json:"text,omitempty"`
	AudioID    string `json:"audio_id"`
	DurationMS int    `json:"duration_ms"`
}

type Shot struct {
	ClipID      string `json:"clip_id"`
	SourceInMS  int    `json:"source_in_ms"`
	SourceOutMS int    `json:"source_out_ms"`
}

type Caption struct {
	StartMS int    `json:"start_ms"`
	EndMS   int    `json:"end_ms"`
	Text    string `json:"text"`
}

type BGM struct {
	AssetID string  `json:"asset_id"`
	GainDB  float64 `json:"gain_db"`
}

// Asset is one catalog entry. DurationMS is measured by the caller (probe)
// and is not part of the catalog file. SHA256 is the expected content hash.
type Asset struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	SKUID      string `json:"sku_id,omitempty"`
	Source     string `json:"source"`
	DurationMS int    `json:"-"`
}

type catalogFile struct {
	Assets []Asset `json:"assets"`
}

// Check selects optional validation stages.
// The zero value checks shot and voice bounds. SkipBounds is for a catalog
// pass that runs before media duration is known.
type Check struct {
	SkipBounds bool
}

func LoadTimeline(path string) (Timeline, error) {
	var tl Timeline
	if err := decodeFile(path, &tl); err != nil {
		return Timeline{}, err
	}
	return tl, nil
}

func LoadAssets(path string) ([]Asset, error) {
	var file catalogFile
	if err := decodeFile(path, &file); err != nil {
		return nil, err
	}
	return file.Assets, nil
}

func decodeFile(path string, dest any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// ResolvePaths joins relative asset paths to base. Absolute paths are unchanged.
func ResolvePaths(assets []Asset, base string) []Asset {
	out := make([]Asset, len(assets))
	for i, asset := range assets {
		out[i] = asset
		if asset.Path != "" && !filepath.IsAbs(asset.Path) {
			out[i].Path = filepath.Join(base, asset.Path)
		}
	}
	return out
}
