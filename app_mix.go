package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hon94csl/qingfan/internal/bundle"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/timeline"
)

// FetchedClip is one video saved from a link the operator supplied.
type FetchedClip struct {
	AssetID    string `json:"asset_id"`
	Path       string `json:"path"`
	DurationMS int    `json:"duration_ms"`
}

// MixFromLinks downloads each http(s) file, stores the prompt as the product facts,
// and mixes only those pictures. An empty font or BGM uses the bundled default.
// A page that is not a video file fails after probing. This does not open a shop page.
func (a *App) MixFromLinks(projectID, skuID, fontID, bgmID, prompt string, links []string) (ProduceResult, error) {
	if err := a.aiReady(); err != nil {
		return ProduceResult{}, err
	}
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(skuID) == "" {
		return ProduceResult{}, fmt.Errorf("先填写商品")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ProduceResult{}, fmt.Errorf("先填写提示词")
	}
	urls, err := cleanLinks(links)
	if err != nil {
		return ProduceResult{}, err
	}
	if _, err := a.db.GetSKU(skuID); err != nil {
		return ProduceResult{}, err
	}
	dir, err := importDir(skuID)
	if err != nil {
		return ProduceResult{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ProduceResult{}, err
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()
	fetched := make([]FetchedClip, 0, len(urls))
	assetIDs := make([]string, 0, len(urls))
	for i, raw := range urls {
		dest := filepath.Join(dir, saveName(raw, i))
		if err := media.Download(ctx, raw, dest); err != nil {
			return ProduceResult{}, err
		}
		imported, err := a.db.Import(ctx, store.ImportRequest{
			SKUID: skuID, Kind: timeline.KindClip, Source: timeline.SourceOwned, Rights: store.RightsUsable, Path: dest,
		})
		if err != nil {
			os.Remove(dest)
			return ProduceResult{}, videoLinkError(err)
		}
		if imported.DurationMS <= 0 {
			return ProduceResult{}, fmt.Errorf("这个链接不是可下载的视频文件")
		}
		assetIDs = append(assetIDs, imported.AssetID)
		fetched = append(fetched, FetchedClip{AssetID: imported.AssetID, Path: dest, DurationMS: imported.DurationMS})
	}
	if strings.TrimSpace(fontID) == "" {
		fontID, err = a.ensureBundled(skuID, timeline.KindFont)
		if err != nil {
			return ProduceResult{}, err
		}
	}
	if strings.TrimSpace(bgmID) == "" {
		bgmID, err = a.ensureBundled(skuID, timeline.KindBGM)
		if err != nil {
			return ProduceResult{}, err
		}
	}
	result, err := a.ProduceVideo(projectID, skuID, fontID, bgmID, assetIDs, prompt)
	result.Fetched = fetched
	return result, err
}

func cleanLinks(links []string) ([]string, error) {
	var urls []string
	for _, link := range links {
		for _, part := range strings.Fields(link) {
			if _, err := media.ParseVideoURL(part); err != nil {
				return nil, err
			}
			urls = append(urls, part)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("先填写视频链接")
	}
	return urls, nil
}

func importDir(skuID string) (string, error) {
	name := filepath.Base(skuID)
	if name != skuID || name == "." || name == ".." {
		return "", fmt.Errorf("商品标识无效")
	}
	return filepath.Join(dataDir(), "imports", name), nil
}

func saveName(raw string, index int) string {
	name := "video.mp4"
	if parsed, err := media.ParseVideoURL(raw); err == nil {
		name = filepath.Base(parsed.Path)
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "video.mp4"
	}
	if filepath.Ext(name) == "" {
		name += ".mp4"
	}
	return fmt.Sprintf("%02d-%s", index+1, name)
}

func videoLinkError(err error) error {
	if err == nil || errors.Is(err, exec.ErrNotFound) {
		return err
	}
	msg := err.Error()
	if strings.Contains(msg, "素材没有视频轨") || strings.Contains(msg, "视频时长无效") || strings.Contains(msg, "ffprobe") {
		return fmt.Errorf("这个链接不是可下载的视频文件")
	}
	return err
}

func (a *App) ensureBundled(skuID, kind string) (string, error) {
	var body []byte
	name := ""
	switch kind {
	case timeline.KindFont:
		body = bundle.Lato
		name = "Lato-Regular.ttf"
	case timeline.KindBGM:
		body = bundle.BGM
		name = "bgm_demo.wav"
	default:
		return "", fmt.Errorf("没有可自动使用的字体或 BGM")
	}
	dir := filepath.Join(dataDir(), "defaults")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := writeSame(path, body); err != nil {
		return "", err
	}
	if kind == timeline.KindFont {
		if err := writeSame(filepath.Join(dir, "OFL.txt"), bundle.LatoLicense); err != nil {
			return "", err
		}
	}
	imported, err := a.ImportFile(skuID, kind, timeline.SourceLibrary, store.RightsUsable, path)
	if err != nil {
		return "", err
	}
	return imported.AssetID, nil
}

func writeSame(path string, body []byte) error {
	have, err := os.ReadFile(path)
	if err == nil && bytes.Equal(have, body) {
		return nil
	}
	return os.WriteFile(path, body, 0o644)
}
