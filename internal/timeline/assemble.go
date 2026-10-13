package timeline

import (
	"errors"
	"fmt"
)

// ErrGap means the approved voice is longer than the usable footage.
// Callers must not pad that gap by looping, freezing, or crossing a shot boundary.
var ErrGap = errors.New("可用画面不够这一段配音")

// Footage is one selectable interval. Times are milliseconds and half-open.
type Footage struct {
	ClipID string
	SKUID  string
	Source string
	InMS   int
	OutMS  int
}

// SegmentPlan is one approved voice and the clip order the operator kept.
type SegmentPlan struct {
	ID              string
	Role            string
	VoiceText       string
	VoiceAssetID    string
	VoiceDurationMS int
	Order           []string
}

// BuildInput is everything the renderer needs besides the shot math.
type BuildInput struct {
	OutputID      string
	Revision      int
	SKUID         string
	Market        string
	Locale        string
	FontAssetID   string
	BGMAssetID    string
	BGMGainDB     float64
	HookType      string
	PromptVersion string
	Footage       []Footage
	Segments      []SegmentPlan
}

// Assemble fills voiceDurationMS from the clips named in order.
// Each clip is used once, inside its own interval. A shortfall returns ErrGap
// and no shots, so the caller cannot export a padded timeline.
func Assemble(voiceDurationMS int, footage []Footage, order []string, sku string) ([]Shot, error) {
	if voiceDurationMS <= 0 {
		return nil, fmt.Errorf("%w: 配音时长", ErrInterval)
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("%w: 这一段没有镜头", ErrMissing)
	}
	index := make(map[string]Footage, len(footage))
	for _, clip := range footage {
		if clip.ClipID == "" {
			return nil, fmt.Errorf("%w: 镜头编号", ErrMissing)
		}
		if _, ok := index[clip.ClipID]; ok {
			return nil, fmt.Errorf("%w: 重复镜头 %s", ErrInterval, clip.ClipID)
		}
		index[clip.ClipID] = clip
	}
	remain := voiceDurationMS
	shots := make([]Shot, 0, len(order))
	seen := map[string]bool{}
	for _, id := range order {
		clip, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissing, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: 镜头 %s 被重复使用", ErrInterval, id)
		}
		seen[id] = true
		if clip.SKUID != sku {
			return nil, fmt.Errorf("这个镜头属于其他商品，不能放进当前成片: %w", ErrSKU)
		}
		if clip.Source == SourceReference {
			return nil, fmt.Errorf("参考镜头不能放进成片: %w", ErrReference)
		}
		if clip.OutMS <= clip.InMS {
			return nil, fmt.Errorf("%w: 镜头 %s 没有可用区间", ErrInterval, id)
		}
		if remain == 0 {
			break
		}
		span := clip.OutMS - clip.InMS
		use := span
		if use > remain {
			use = remain
		}
		shots = append(shots, Shot{ClipID: id, SourceInMS: clip.InMS, SourceOutMS: clip.InMS + use})
		remain -= use
	}
	if remain > 0 {
		return nil, fmt.Errorf("%w：还差 %d 毫秒", ErrGap, remain)
	}
	return shots, nil
}

// Build turns approved voices into a render timeline.
// Segment length follows the audio. Captions cover the whole segment and do not
// invent word timings.
func Build(in BuildInput) (Timeline, error) {
	if len(in.Segments) == 0 {
		return Timeline{}, fmt.Errorf("%w: 没有分段", ErrMissing)
	}
	hook := in.HookType
	if hook == "" {
		hook = "manual"
	}
	version := in.PromptVersion
	if version == "" {
		version = "fixed-m1"
	}
	tl := Timeline{
		SchemaVersion: SchemaVersion,
		OutputID:      in.OutputID,
		Revision:      in.Revision,
		SKUID:         in.SKUID,
		Market:        in.Market,
		Locale:        in.Locale,
		Canvas:        Canvas{Width: 1080, Height: 1920, FPS: 30},
		FontAssetID:   in.FontAssetID,
		Recipe:        Recipe{HookType: hook, PromptVersion: version},
		BGM:           BGM{AssetID: in.BGMAssetID, GainDB: in.BGMGainDB},
	}
	for _, seg := range in.Segments {
		shots, err := Assemble(seg.VoiceDurationMS, in.Footage, seg.Order, in.SKUID)
		if err != nil {
			return Timeline{}, err
		}
		role := seg.Role
		if role == "" {
			role = "body"
		}
		tl.Segments = append(tl.Segments, Segment{
			ID:            seg.ID,
			Role:          role,
			DurationMS:    seg.VoiceDurationMS,
			Voice:         Voice{Text: seg.VoiceText, AudioID: seg.VoiceAssetID, DurationMS: seg.VoiceDurationMS},
			Shots:         shots,
			OriginalAudio: OriginalMute,
			Captions: []Caption{{
				StartMS: 0,
				EndMS:   seg.VoiceDurationMS,
				Text:    seg.VoiceText,
			}},
			CaptionStyle: "id-default",
		})
	}
	return tl, nil
}
