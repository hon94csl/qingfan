package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
	"unicode/utf8"

	"github.com/hon94csl/qingfan/internal/timeline"
)

// Fixture builds valid review data from the supplied catalog. It does not call a model.
type Fixture struct{}

func (Fixture) Name() string { return "fixture" }

// ProviderFromEnv uses the fixture catalog unless a supplier URL is configured.
// The key stays in the provider and is not returned.
func ProviderFromEnv() (Provider, error) {
	url := os.Getenv("QINGFAN_PROVIDER_URL")
	if url == "" {
		return Fixture{}, nil
	}
	key := os.Getenv("QINGFAN_PROVIDER_KEY")
	if key == "" {
		return nil, fmt.Errorf("已配置供应商地址但没有密钥")
	}
	return &HTTPProvider{URL: url, Key: secret(key), Client: &http.Client{Timeout: 60 * time.Second}}, nil
}

func (Fixture) Complete(_ context.Context, kind string, input []byte) ([]byte, Usage, error) {
	switch kind {
	case "analyze":
		var in analyzeInput
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, Usage{}, err
		}
		notes := make([]ShotNote, 0, len(in.Clips))
		for _, clip := range in.Clips {
			if !clip.Selectable || clip.Source == timeline.SourceReference {
				continue
			}
			notes = append(notes, ShotNote{
				ClipID: clip.ID, Subject: "待确认主体", Action: "待确认动作", Purpose: "展示商品",
				SourceInMS: clip.InMS, SourceOutMS: clip.OutMS,
			})
		}
		body, err := json.Marshal(analyzeOutput{Clips: notes})
		return body, Usage{Known: true, Micros: 1}, err
	case "script":
		var in analyzeInput
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, Usage{}, err
		}
		var ids []string
		var facts []string
		for _, clip := range in.Clips {
			if clip.Selectable && clip.Source != timeline.SourceReference {
				ids = append(ids, clip.ID)
			}
		}
		for _, fact := range in.Facts {
			facts = append(facts, fact.ID)
		}
		cands := []Candidate{
			fixtureCandidate("c1", "pain_question", ids, facts),
			fixtureCandidate("c2", "demo", ids, facts),
		}
		body, err := json.Marshal(scriptOutput{Candidates: cands})
		return body, Usage{Known: true, Micros: 2}, err
	case "translate":
		body, err := json.Marshal(struct {
			BackTranslation string `json:"back_translation"`
		}{BackTranslation: "这是固定回译，需要人工核对。"})
		return body, Usage{Known: true, Micros: 1}, err
	case "tts":
		var in struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return nil, Usage{}, err
		}
		if in.Text == "" {
			return nil, Usage{}, ErrInvalidAudio
		}
		ms := 800
		if n := utf8.RuneCountInString(in.Text); n > 20 {
			ms = 40 * n
		}
		body, err := SilenceWAV(ms)
		return body, Usage{Known: true, Micros: int64(ms)}, err
	default:
		return nil, Usage{}, fmt.Errorf("不支持的调用类型 %s", kind)
	}
}

func fixtureCandidate(id, hook string, clips, facts []string) Candidate {
	return Candidate{
		ID: id, HookType: hook, BackTranslation: "这是固定回译，需要人工核对。",
		Segments: []ScriptSegment{{
			ID: "seg_1", Role: "hook", Purpose: "提出问题", FactIDs: facts,
			VoiceText: "Lihat barang ini.", ClipIDs: clips, EstimatedMS: 1000,
		}},
	}
}
