package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hon94csl/qingfan/internal/timeline"
)

const (
	ErrModelJSON    = gatewayError("模型结果无法解析")
	ErrUnknownAsset = gatewayError("模型引用了不存在的素材")
	ErrUnknownFact  = gatewayError("模型引用了未确认的商品事实")
	ErrBounds       = gatewayError("模型给出的时间超出镜头")
	ErrNeedsReview  = gatewayError("镜头理解不完整，需要人工确认")
)

// Fact is one confirmed product statement. Models may cite its ID and nothing else.
type Fact struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Clip is one interval the model is allowed to see.
type Clip struct {
	ID         string `json:"id"`
	SKUID      string `json:"sku_id"`
	Source     string `json:"source"`
	Selectable bool   `json:"selectable"`
	InMS       int    `json:"source_in_ms"`
	OutMS      int    `json:"source_out_ms"`
	Path       string `json:"path,omitempty"`
}

// ShotNote is a reviewed description of one clip.
type ShotNote struct {
	ClipID        string   `json:"clip_id"`
	Subject       string   `json:"subject"`
	Action        string   `json:"action"`
	Purpose       string   `json:"purpose"`
	SellingPoints []string `json:"selling_points"`
	SourceInMS    int      `json:"source_in_ms"`
	SourceOutMS   int      `json:"source_out_ms"`
	Manual        bool     `json:"manual"`
	FactIDs       []string `json:"fact_ids,omitempty"`
}

// FactsFromText splits operator-confirmed lines. Blank lines are ignored.
func FactsFromText(text string) []Fact {
	var facts []Fact
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		facts = append(facts, Fact{ID: fmt.Sprintf("fact-%d", len(facts)+1), Text: line})
	}
	return facts
}

// FrameTimes returns the start, middle, and a decodable point near the end of a half-open interval.
// The end sample stays 42ms inside the interval so a 24fps frame still has a picture there.
func FrameTimes(in, out int) ([]int, error) {
	if out <= in {
		return nil, fmt.Errorf("%w: 镜头区间为空", ErrBounds)
	}
	mid := in + (out-in)/2
	end := in
	if out-in > 42 {
		end = out - 42
	}
	times := []int{in}
	if mid != in {
		times = append(times, mid)
	}
	if end != in && end != mid {
		times = append(times, end)
	}
	return times, nil
}

// ExtractFrames writes one JPEG per timestamp. Missing ffmpeg is an error, not an empty analysis.
func ExtractFrames(ctx context.Context, video string, times []int, dir string) ([]string, error) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("%w: 没有 ffmpeg，无法抽帧", ErrNeedsReview)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(times))
	for _, at := range times {
		out := filepath.Join(dir, fmt.Sprintf("frame-%d.jpg", at))
		cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-nostdin", "-y", "-i", video, "-ss", fmt.Sprintf("%dms", at), "-frames:v", "1", "-update", "1", out)
		if raw, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrNeedsReview, trimOutput(raw))
		}
		paths = append(paths, out)
	}
	return paths, nil
}

// reviewFrames extracts a few frames for every usable clip that has a file.
// A missing tool or a failed extract stops the analysis instead of pretending the picture was seen.
func reviewFrames(ctx context.Context, clips []Clip) error {
	var dirs []string
	defer func() {
		for _, dir := range dirs {
			os.RemoveAll(dir)
		}
	}()
	for _, clip := range clips {
		if clip.Path == "" || !clip.Selectable || clip.Source == timeline.SourceReference {
			continue
		}
		times, err := FrameTimes(clip.InMS, clip.OutMS)
		if err != nil {
			return err
		}
		dir, err := os.MkdirTemp("", "qingfan-frames-")
		if err != nil {
			return err
		}
		dirs = append(dirs, dir)
		if _, err := ExtractFrames(ctx, clip.Path, times, dir); err != nil {
			return err
		}
	}
	return nil
}

func trimOutput(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 240 {
		return text[len(text)-240:]
	}
	return text
}

type analyzeInput struct {
	PromptVersion string `json:"prompt_version"`
	Prompt        string `json:"prompt"`
	Facts         []Fact `json:"facts"`
	Clips         []Clip `json:"clips"`
}

type analyzeOutput struct {
	Clips []ShotNote `json:"clips"`
}

// ParseAnalysis rejects unknown clips, unknown facts, and intervals outside the catalog.
// It does not replace a bad clip id with the first catalog entry.
func ParseAnalysis(body []byte, facts []Fact, clips []Clip) ([]ShotNote, error) {
	var out analyzeOutput
	if err := decode(body, &out); err != nil {
		return nil, err
	}
	if len(out.Clips) == 0 {
		return nil, fmt.Errorf("%w: 没有镜头理解", ErrModelJSON)
	}
	catalog := indexClips(clips)
	knownFacts := indexFacts(facts)
	for i, note := range out.Clips {
		clip, ok := catalog[note.ClipID]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownAsset, note.ClipID)
		}
		if note.SourceOutMS <= note.SourceInMS || note.SourceInMS < clip.InMS || note.SourceOutMS > clip.OutMS {
			return nil, fmt.Errorf("%w: %s", ErrBounds, note.ClipID)
		}
		for _, id := range note.FactIDs {
			if _, ok := knownFacts[id]; !ok {
				return nil, fmt.Errorf("%w: %s", ErrUnknownFact, id)
			}
		}
		if strings.TrimSpace(note.Subject) == "" || strings.TrimSpace(note.Action) == "" {
			return nil, fmt.Errorf("%w: %s", ErrNeedsReview, note.ClipID)
		}
		out.Clips[i].Manual = false
	}
	return out.Clips, nil
}

// ApplyCorrection stores an operator edit. The clip id stays the same.
func ApplyCorrection(clips []Clip, current, edit ShotNote) (ShotNote, error) {
	if edit.ClipID == "" || edit.ClipID != current.ClipID {
		return ShotNote{}, fmt.Errorf("%w: %s", ErrUnknownAsset, edit.ClipID)
	}
	clip, ok := indexClips(clips)[edit.ClipID]
	if !ok {
		return ShotNote{}, fmt.Errorf("%w: %s", ErrUnknownAsset, edit.ClipID)
	}
	if edit.SourceOutMS <= edit.SourceInMS || edit.SourceInMS < clip.InMS || edit.SourceOutMS > clip.OutMS {
		return ShotNote{}, fmt.Errorf("%w: %s", ErrBounds, edit.ClipID)
	}
	if strings.TrimSpace(edit.Subject) == "" || strings.TrimSpace(edit.Action) == "" {
		return ShotNote{}, ErrNeedsReview
	}
	edit.Manual = true
	return edit, nil
}

// Analyze asks for one description per supplied clip and validates it before return.
func Analyze(ctx context.Context, g *Gateway, token, task string, facts []Fact, clips []Clip) ([]ShotNote, error) {
	if len(clips) == 0 {
		return nil, fmt.Errorf("%w: 没有镜头", ErrUnknownAsset)
	}
	payload, err := json.Marshal(analyzeInput{PromptVersion: PromptVersion, Prompt: analyzePrompt, Facts: facts, Clips: clips})
	if err != nil {
		return nil, err
	}
	result, err := g.Do(ctx, token, Call{
		TaskID: task, Kind: "analyze", Model: g.provider.Name(), PromptVersion: PromptVersion, Input: payload, Attempt: 1,
	})
	if err != nil {
		return nil, err
	}
	return ParseAnalysis(result.Body, facts, clips)
}

func decode(body []byte, dest any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("%w: %v", ErrModelJSON, err)
	}
	return nil
}

func indexClips(clips []Clip) map[string]Clip {
	out := make(map[string]Clip, len(clips))
	for _, clip := range clips {
		out[clip.ID] = clip
	}
	return out
}

func indexFacts(facts []Fact) map[string]Fact {
	out := make(map[string]Fact, len(facts))
	for _, fact := range facts {
		out[fact.ID] = fact
	}
	return out
}

// DefaultStructure is the fixed shape used when there is no reference video.
func DefaultStructure() timeline.Recipe {
	return timeline.Recipe{HookType: "pain_question", PromptVersion: PromptVersion}
}
