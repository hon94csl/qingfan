package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hon94csl/qingfan/internal/timeline"
)

// ScriptSegment is one spoken beat before real audio exists.
type ScriptSegment struct {
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Purpose     string   `json:"purpose"`
	FactIDs     []string `json:"fact_ids"`
	VoiceText   string   `json:"voice_text"`
	ClipIDs     []string `json:"clip_ids"`
	EstimatedMS int      `json:"estimated_ms"`
}

// Candidate is one reviewable Indonesian script plus a Chinese back-translation.
type Candidate struct {
	ID              string          `json:"id"`
	HookType        string          `json:"hook_type"`
	BackTranslation string          `json:"back_translation"`
	TranslationOf   string          `json:"translation_of"`
	Segments        []ScriptSegment `json:"segments"`
}

// Generation is the validated analysis and the candidate scripts.
type Generation struct {
	Notes      []ShotNote  `json:"notes"`
	Candidates []Candidate `json:"candidates"`
}

// TranslationStale reports that the Chinese text no longer matches the voice copy.
func (c Candidate) TranslationStale() bool {
	return c.TranslationOf == "" || c.TranslationOf != TextHash(c)
}

// TextHash covers every segment's spoken text.
func TextHash(c Candidate) string {
	h := sha256.New()
	for _, seg := range c.Segments {
		h.Write([]byte(seg.ID))
		h.Write([]byte{0})
		h.Write([]byte(seg.VoiceText))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// EditVoiceText changes one segment and leaves the previous translation marked stale.
func EditVoiceText(c Candidate, segmentID, text string) Candidate {
	for i := range c.Segments {
		if c.Segments[i].ID == segmentID {
			c.Segments[i].VoiceText = text
		}
	}
	return c
}

type scriptOutput struct {
	Candidates []Candidate `json:"candidates"`
}

// ParseScripts checks count, facts, and clip ids.
// An unknown clip id fails the candidate. The first catalog clip is not used instead.
func ParseScripts(body []byte, facts []Fact, clips []Clip) ([]Candidate, error) {
	var out scriptOutput
	if err := decode(body, &out); err != nil {
		return nil, err
	}
	if len(out.Candidates) < 1 || len(out.Candidates) > 3 {
		return nil, fmt.Errorf("%w: 候选数量必须是 1 到 3 条", ErrModelJSON)
	}
	catalog := indexClips(clips)
	known := indexFacts(facts)
	seen := map[string]bool{}
	for i, cand := range out.Candidates {
		if cand.ID == "" || seen[cand.ID] || cand.HookType == "" || strings.TrimSpace(cand.BackTranslation) == "" {
			return nil, fmt.Errorf("%w: 候选缺少编号、钩子或回译", ErrModelJSON)
		}
		seen[cand.ID] = true
		if len(cand.Segments) == 0 {
			return nil, fmt.Errorf("%w: 候选 %s 没有分段", ErrModelJSON, cand.ID)
		}
		for _, seg := range cand.Segments {
			if seg.ID == "" || strings.TrimSpace(seg.VoiceText) == "" || seg.EstimatedMS <= 0 || len(seg.FactIDs) == 0 || len(seg.ClipIDs) == 0 {
				return nil, fmt.Errorf("%w: 分段 %s 不完整", ErrModelJSON, seg.ID)
			}
			if strings.ContainsAny(seg.VoiceText, "\r\n") {
				return nil, fmt.Errorf("%w: 口播不能换行", ErrModelJSON)
			}
			for _, id := range seg.FactIDs {
				if _, ok := known[id]; !ok {
					return nil, fmt.Errorf("%w: %s", ErrUnknownFact, id)
				}
			}
			for _, id := range seg.ClipIDs {
				clip, ok := catalog[id]
				if !ok {
					return nil, fmt.Errorf("%w: %s", ErrUnknownAsset, id)
				}
				if clip.Source == timeline.SourceReference || !clip.Selectable {
					return nil, fmt.Errorf("参考镜头不能放进成片: %w", timeline.ErrReference)
				}
			}
		}
		out.Candidates[i].TranslationOf = TextHash(cand)
	}
	return out.Candidates, nil
}

// Generate analyzes the clips, then asks for 1–3 candidates. Both responses are validated.
func Generate(ctx context.Context, g *Gateway, token, task string, facts []Fact, clips []Clip) (Generation, error) {
	if len(facts) == 0 {
		return Generation{}, fmt.Errorf("%w: 先填写已确认的商品事实", ErrUnknownFact)
	}
	selectable := 0
	for _, clip := range clips {
		if clip.Selectable && clip.Source != timeline.SourceReference {
			selectable++
		}
	}
	if selectable == 0 {
		return Generation{}, fmt.Errorf("%w: 没有可用镜头", ErrUnknownAsset)
	}
	if err := reviewFrames(ctx, clips); err != nil {
		return Generation{}, err
	}
	notes, err := Analyze(ctx, g, token, task, facts, clips)
	if err != nil {
		return Generation{}, err
	}
	payload, err := json.Marshal(analyzeInput{PromptVersion: PromptVersion, Prompt: scriptPrompt, Facts: facts, Clips: clips})
	if err != nil {
		return Generation{}, err
	}
	result, err := g.Do(ctx, token, Call{
		TaskID: task, Kind: "script", Model: g.provider.Name(), PromptVersion: PromptVersion, Input: payload, Attempt: 1,
	})
	if err != nil {
		return Generation{}, err
	}
	candidates, err := ParseScripts(result.Body, facts, clips)
	if err != nil {
		return Generation{}, err
	}
	return Generation{Notes: notes, Candidates: candidates}, nil
}

// Retranslate replaces the Chinese text and binds it to the current voice copy.
func Retranslate(ctx context.Context, g *Gateway, token, task string, cand Candidate) (Candidate, error) {
	payload, err := json.Marshal(struct {
		PromptVersion string    `json:"prompt_version"`
		Prompt        string    `json:"prompt"`
		Candidate     Candidate `json:"candidate"`
	}{PromptVersion: PromptVersion, Prompt: scriptPrompt, Candidate: cand})
	if err != nil {
		return Candidate{}, err
	}
	result, err := g.Do(ctx, token, Call{
		TaskID: task, Kind: "translate", Model: g.provider.Name(), PromptVersion: PromptVersion, Input: payload, Attempt: 1,
	})
	if err != nil {
		return Candidate{}, err
	}
	var out struct {
		BackTranslation string `json:"back_translation"`
	}
	if err := decode(result.Body, &out); err != nil {
		return Candidate{}, err
	}
	if strings.TrimSpace(out.BackTranslation) == "" {
		return Candidate{}, fmt.Errorf("%w: 缺少中文回译", ErrModelJSON)
	}
	cand.BackTranslation = out.BackTranslation
	cand.TranslationOf = TextHash(cand)
	return cand, nil
}

// ParseStructure reads a reference breakdown. Clip ids in that payload are rejected
// so a reference shot cannot enter the cut.
func ParseStructure(body []byte) (timeline.Recipe, error) {
	var out struct {
		HookType string   `json:"hook_type"`
		Beats    []string `json:"beats"`
		ClipIDs  []string `json:"clip_ids"`
	}
	if err := decode(body, &out); err != nil {
		return timeline.Recipe{}, err
	}
	if len(out.ClipIDs) > 0 {
		return timeline.Recipe{}, fmt.Errorf("%w: %s", timeline.ErrReference, out.ClipIDs[0])
	}
	if out.HookType == "" || len(out.Beats) == 0 {
		return timeline.Recipe{}, fmt.Errorf("%w: 参考结构不完整", ErrModelJSON)
	}
	return timeline.Recipe{HookType: out.HookType, PromptVersion: PromptVersion}, nil
}
