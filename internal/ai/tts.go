package ai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
)

// Take is one synthesized segment. It stays valid only for the same text and voice.
type Take struct {
	SegmentID  string `json:"segment_id"`
	Voice      string `json:"voice"`
	TextHash   string `json:"text_hash"`
	DurationMS int    `json:"duration_ms"`
	WAV        []byte `json:"-"`
	Valid      bool   `json:"valid"`
}

// Matches reports whether this audio can still be used for the spoken text and voice.
func (t Take) Matches(text, voice string) bool {
	return t.Valid && t.Voice == voice && t.TextHash == hashText(text) && t.DurationMS > 0 && len(t.WAV) > 0
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Synthesize asks for one segment's audio. Invalid payloads stay invalid.
func Synthesize(ctx context.Context, g *Gateway, token, task, voice string, seg ScriptSegment) (Take, error) {
	if voice == "" || seg.VoiceText == "" {
		return Take{}, ErrInvalidAudio
	}
	payload, err := json.Marshal(struct {
		PromptVersion string `json:"prompt_version"`
		Text          string `json:"text"`
		Voice         string `json:"voice"`
	}{PromptVersion: PromptVersion, Text: seg.VoiceText, Voice: voice})
	if err != nil {
		return Take{}, err
	}
	result, err := g.Do(ctx, token, Call{
		TaskID: task, SegmentID: seg.ID, Kind: "tts", Model: g.provider.Name(), PromptVersion: PromptVersion,
		Voice: voice, Input: payload, Attempt: 1,
	})
	if err != nil {
		return Take{}, err
	}
	var out struct {
		DurationMS int    `json:"duration_ms"`
		WAVB64     string `json:"wav_b64"`
	}
	if err := decode(result.Body, &out); err != nil {
		return Take{}, err
	}
	wav, err := base64.StdEncoding.DecodeString(out.WAVB64)
	if err != nil || out.DurationMS <= 0 || len(wav) == 0 {
		return Take{}, ErrInvalidAudio
	}
	got, err := WavDurationMS(wav)
	if err != nil || got != out.DurationMS {
		return Take{}, ErrInvalidAudio
	}
	return Take{SegmentID: seg.ID, Voice: voice, TextHash: hashText(seg.VoiceText), DurationMS: got, WAV: wav, Valid: true}, nil
}

// WavSilence builds a PCM WAV whose duration is exactly durationMS.
func WavSilence(durationMS int) ([]byte, error) {
	if durationMS <= 0 {
		return nil, ErrInvalidAudio
	}
	const rate = 8000
	samples := rate * durationMS / 1000
	return wavPCM(rate, samples), nil
}

// WavDurationMS reads a 16-bit mono PCM WAV produced by WavSilence.
func WavDurationMS(raw []byte) (int, error) {
	if len(raw) < 44 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return 0, ErrInvalidAudio
	}
	rate := int(binary.LittleEndian.Uint32(raw[24:28]))
	dataBytes := int(binary.LittleEndian.Uint32(raw[40:44]))
	if rate <= 0 || dataBytes < 2 || dataBytes%2 != 0 {
		return 0, ErrInvalidAudio
	}
	samples := dataBytes / 2
	if samples*1000%rate != 0 {
		return 0, ErrInvalidAudio
	}
	return samples * 1000 / rate, nil
}

func wavPCM(rate, samples int) []byte {
	dataBytes := samples * 2
	buf := make([]byte, 44+dataBytes)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataBytes))
	copy(buf[8:], "WAVE")
	copy(buf[12:], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)
	binary.LittleEndian.PutUint16(buf[22:], 1)
	binary.LittleEndian.PutUint32(buf[24:], uint32(rate))
	binary.LittleEndian.PutUint32(buf[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(buf[32:], 2)
	binary.LittleEndian.PutUint16(buf[34:], 16)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataBytes))
	return buf
}

func encodeWAV(durationMS int) (string, int, error) {
	wav, err := WavSilence(durationMS)
	if err != nil {
		return "", 0, err
	}
	ms, err := WavDurationMS(wav)
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(wav), ms, nil
}

// SilenceWAV is the JSON body a speech supplier would return for a known duration.
func SilenceWAV(durationMS int) ([]byte, error) {
	b64, ms, err := encodeWAV(durationMS)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		DurationMS int    `json:"duration_ms"`
		WAVB64     string `json:"wav_b64"`
	}{DurationMS: ms, WAVB64: b64})
}
