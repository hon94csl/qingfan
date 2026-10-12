package timeline

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInterval    = errors.New("invalid interval")
	ErrBounds      = errors.New("shot exceeds asset bounds")
	ErrDuration    = errors.New("shot durations do not match segment")
	ErrVoice       = errors.New("voice exceeds segment")
	ErrCaption     = errors.New("caption exceeds segment")
	ErrSKU         = errors.New("cross-sku asset")
	ErrReference   = errors.New("reference source")
	ErrMissing     = errors.New("missing asset")
	ErrHash        = errors.New("content hash mismatch")
	ErrUnsupported = errors.New("unsupported")
)

func fail(sentinel error, format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{sentinel}, args...)...)
}

// Validate checks a timeline against its catalog before any render starts.
// It only reads asset files. Asset.DurationMS is the available media length
// and is required unless check.SkipBounds is set.
func Validate(tl Timeline, assets []Asset, check Check) error {
	if err := validateShape(tl); err != nil {
		return err
	}
	index, err := indexAssets(assets)
	if err != nil {
		return err
	}
	if err := validateCatalog(tl, index); err != nil {
		return err
	}
	if check.SkipBounds {
		return nil
	}
	return validateBounds(tl, index)
}

func validateShape(tl Timeline) error {
	if tl.SchemaVersion != SchemaVersion {
		return fail(ErrUnsupported, "schema_version %d", tl.SchemaVersion)
	}
	if tl.OutputID == "" || tl.SKUID == "" || tl.Market == "" || tl.Locale == "" {
		return fail(ErrMissing, "output id, sku, market, or locale")
	}
	if tl.Revision < 1 {
		return fail(ErrInterval, "revision")
	}
	if tl.Canvas.Width != 1080 || tl.Canvas.Height != 1920 || tl.Canvas.FPS != 30 {
		return fail(ErrUnsupported, "canvas must be 1080x1920 at 30fps")
	}
	if tl.FontAssetID == "" {
		return fail(ErrMissing, "font")
	}
	if len(tl.Segments) == 0 {
		return fail(ErrMissing, "segment")
	}
	if math.IsNaN(tl.BGM.GainDB) || math.IsInf(tl.BGM.GainDB, 0) {
		return fail(ErrInterval, "bgm gain")
	}
	if tl.BGM.AssetID == "" {
		return fail(ErrMissing, "bgm")
	}
	seen := map[string]bool{}
	for _, seg := range tl.Segments {
		if seg.ID == "" {
			return fail(ErrMissing, "segment id")
		}
		if seen[seg.ID] {
			return fail(ErrInterval, "duplicate segment %s", seg.ID)
		}
		seen[seg.ID] = true
		if seg.DurationMS <= 0 {
			return fail(ErrInterval, "segment %s duration", seg.ID)
		}
		if seg.OriginalAudio != OriginalMute {
			return fail(ErrUnsupported, "segment %s original_audio %q", seg.ID, seg.OriginalAudio)
		}
		if seg.Voice.AudioID == "" || seg.Voice.DurationMS <= 0 {
			return fail(ErrMissing, "segment %s voice", seg.ID)
		}
		if seg.Voice.DurationMS > seg.DurationMS {
			return fail(ErrVoice, "segment %s voice %dms past %dms", seg.ID, seg.Voice.DurationMS, seg.DurationMS)
		}
		if len(seg.Captions) == 0 {
			return fail(ErrMissing, "segment %s caption", seg.ID)
		}
		if len(seg.Shots) == 0 {
			return fail(ErrDuration, "segment %s has no shots", seg.ID)
		}
		var sum int64
		for _, shot := range seg.Shots {
			if shot.ClipID == "" {
				return fail(ErrMissing, "segment %s clip", seg.ID)
			}
			if shot.SourceInMS < 0 || shot.SourceOutMS <= shot.SourceInMS {
				return fail(ErrInterval, "segment %s clip %s [%d,%d)", seg.ID, shot.ClipID, shot.SourceInMS, shot.SourceOutMS)
			}
			sum += int64(shot.SourceOutMS - shot.SourceInMS)
		}
		if sum != int64(seg.DurationMS) {
			return fail(ErrDuration, "segment %s shots %dms canvas %dms", seg.ID, sum, seg.DurationMS)
		}
		for _, cap := range seg.Captions {
			if strings.TrimSpace(cap.Text) == "" || !utf8.ValidString(cap.Text) {
				return fail(ErrCaption, "segment %s empty caption", seg.ID)
			}
			if strings.ContainsAny(cap.Text, "\r\n") {
				return fail(ErrUnsupported, "segment %s caption line break", seg.ID)
			}
			if cap.StartMS < 0 || cap.EndMS <= cap.StartMS {
				return fail(ErrInterval, "segment %s caption [%d,%d)", seg.ID, cap.StartMS, cap.EndMS)
			}
			if cap.EndMS > seg.DurationMS {
				return fail(ErrCaption, "segment %s caption ends %dms past %dms", seg.ID, cap.EndMS, seg.DurationMS)
			}
		}
	}
	if _, err := SegmentStarts(tl); err != nil {
		return err
	}
	return nil
}

func indexAssets(assets []Asset) (map[string]Asset, error) {
	index := make(map[string]Asset, len(assets))
	for _, asset := range assets {
		if asset.ID == "" {
			return nil, fail(ErrMissing, "asset id")
		}
		if _, ok := index[asset.ID]; ok {
			return nil, fail(ErrInterval, "duplicate asset %s", asset.ID)
		}
		index[asset.ID] = asset
	}
	return index, nil
}

func validateCatalog(tl Timeline, index map[string]Asset) error {
	refs := referenced(tl)
	for _, ref := range refs {
		asset, ok := index[ref.id]
		if !ok || asset.Path == "" {
			return fail(ErrMissing, "%s %s", ref.kind, ref.id)
		}
		if asset.Kind != ref.kind {
			return fail(ErrMissing, "%s is %s, want %s", ref.id, asset.Kind, ref.kind)
		}
		switch asset.Source {
		case SourceReference:
			return fail(ErrReference, "%s", ref.id)
		case SourceOwned, SourceLibrary:
		default:
			return fail(ErrUnsupported, "%s source %q", ref.id, asset.Source)
		}
		if ref.kind == KindClip || ref.kind == KindVoice {
			if asset.SKUID != tl.SKUID {
				return fail(ErrSKU, "%s sku %q", ref.id, asset.SKUID)
			}
		} else if asset.SKUID != "" && asset.SKUID != tl.SKUID {
			return fail(ErrSKU, "%s sku %q", ref.id, asset.SKUID)
		}
		sum, err := hashFile(asset.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fail(ErrMissing, "%s %s", ref.id, asset.Path)
			}
			return err
		}
		if !strings.EqualFold(sum, strings.TrimSpace(asset.SHA256)) {
			return fail(ErrHash, "%s", ref.id)
		}
	}
	return nil
}

type assetRef struct {
	id   string
	kind string
}

func referenced(tl Timeline) []assetRef {
	var refs []assetRef
	seen := map[string]bool{}
	add := func(id, kind string) {
		if seen[id+"\x00"+kind] {
			return
		}
		seen[id+"\x00"+kind] = true
		refs = append(refs, assetRef{id: id, kind: kind})
	}
	add(tl.FontAssetID, KindFont)
	add(tl.BGM.AssetID, KindBGM)
	for _, seg := range tl.Segments {
		add(seg.Voice.AudioID, KindVoice)
		for _, shot := range seg.Shots {
			add(shot.ClipID, KindClip)
		}
	}
	return refs
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fail(ErrMissing, "%s is a directory", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validateBounds(tl Timeline, index map[string]Asset) error {
	slack := frameMS(tl.Canvas.FPS)
	for _, seg := range tl.Segments {
		voice := index[seg.Voice.AudioID]
		if voice.DurationMS+slack < seg.Voice.DurationMS {
			return fail(ErrBounds, "voice %s available %dms declared %dms", voice.ID, voice.DurationMS, seg.Voice.DurationMS)
		}
		for _, shot := range seg.Shots {
			clip := index[shot.ClipID]
			if shot.SourceOutMS > clip.DurationMS {
				return fail(ErrBounds, "clip %s [%d,%d) available %dms", shot.ClipID, shot.SourceInMS, shot.SourceOutMS, clip.DurationMS)
			}
		}
	}
	return nil
}

func frameMS(fps int) int {
	if fps <= 0 {
		return 0
	}
	return (1000 + fps - 1) / fps
}

// SegmentStarts returns each segment's start on the output timeline.
// The start of a later segment is the sum of the durations before it.
func SegmentStarts(tl Timeline) ([]int, error) {
	starts := make([]int, len(tl.Segments))
	var acc int64
	for i, seg := range tl.Segments {
		if seg.DurationMS <= 0 {
			return nil, fail(ErrInterval, "segment %s duration", seg.ID)
		}
		if acc > int64(math.MaxInt)-int64(seg.DurationMS) {
			return nil, fail(ErrInterval, "timeline duration overflow")
		}
		starts[i] = int(acc)
		acc += int64(seg.DurationMS)
	}
	return starts, nil
}

// TotalDurationMS is the sum of segment durations.
func TotalDurationMS(tl Timeline) (int, error) {
	starts, err := SegmentStarts(tl)
	if err != nil {
		return 0, err
	}
	if len(starts) == 0 {
		return 0, fail(ErrMissing, "segment")
	}
	last := len(tl.Segments) - 1
	return starts[last] + tl.Segments[last].DurationMS, nil
}

// FrameDelta is the absolute error of rounding ms to the nearest frame at fps.
// Nearest-frame rounding stays within half a frame, which is within one frame.
func FrameDelta(ms, fps int) (time.Duration, error) {
	if fps <= 0 || ms < 0 {
		return 0, fail(ErrInterval, "frame grid ms=%d fps=%d", ms, fps)
	}
	num := int64(ms) * int64(fps)
	frame := (num + 500) / 1000
	diff := num*1_000_000 - frame*1_000_000_000
	if diff < 0 {
		diff = -diff
	}
	return time.Duration(diff / int64(fps)), nil
}

// Cue is one subtitle interval on the output timeline.
type Cue struct {
	StartMS int
	EndMS   int
	Text    string
}

// AbsoluteCues places segment-relative captions on the output timeline.
func AbsoluteCues(tl Timeline) ([]Cue, error) {
	starts, err := SegmentStarts(tl)
	if err != nil {
		return nil, err
	}
	var cues []Cue
	for i, seg := range tl.Segments {
		for _, cap := range seg.Captions {
			cues = append(cues, Cue{
				StartMS: starts[i] + cap.StartMS,
				EndMS:   starts[i] + cap.EndMS,
				Text:    cap.Text,
			})
		}
	}
	return cues, nil
}
