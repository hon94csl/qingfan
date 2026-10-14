package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hon94csl/qingfan/internal/ai"
	"github.com/hon94csl/qingfan/internal/auth"
	"github.com/hon94csl/qingfan/internal/credentials"
	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/media"
	"github.com/hon94csl/qingfan/internal/store"
	"github.com/hon94csl/qingfan/internal/timeline"
)

func TestUsageReadsAnEmptyLedger(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })

	if _, err := app.Usage(); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	report, err := app.Usage()
	if err != nil {
		t.Fatal(err)
	}
	if report != (ai.CostReport{}) {
		t.Fatalf("empty ledger: %+v", report)
	}
}

func TestProduceVideoNeedsFontAndLogin(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })

	if _, err := app.ProduceVideo("project", "sku", "font", "bgm", nil, ""); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	if _, err := app.ProduceVideo("", "sku", "font", "bgm", nil, ""); err == nil || err.Error() != "先填写商品" {
		t.Fatalf("product: %v", err)
	}
	if _, err := app.ProduceVideo("project", "sku", "", "bgm", nil, ""); err == nil || err.Error() != "先选择字体和 BGM" {
		t.Fatalf("font: %v", err)
	}
}

func TestStartupRewindsInterruptedSteps(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QINGFAN_DATA", dir)
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	ready := filepath.Join(dir, "voice.wav")
	if err := os.WriteFile(ready, []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone.wav")
	if err := os.WriteFile(gone, []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := app.db.PutStep("job-1", "run", jobs.Running, ready, ""); err != nil {
		t.Fatal(err)
	}
	if err := app.db.PutStep("job-1", "gone", jobs.Succeeded, gone, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if err := app.db.PutStep("job-1", "kept", jobs.Succeeded, ready, ""); err != nil {
		t.Fatal(err)
	}
	if err := app.db.PutStep("job-1", "stop", jobs.Cancelled, "", ""); err != nil {
		t.Fatal(err)
	}
	app.shutdown(context.Background())

	app = &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.storeErr != nil {
		t.Fatal(app.storeErr)
	}
	steps, err := app.db.Steps("job-1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, step := range steps {
		got[step.Key] = step.Status
	}
	if got["run"] != jobs.Pending || got["gone"] != jobs.Pending || got["kept"] != jobs.Succeeded || got["stop"] != jobs.Cancelled {
		t.Fatalf("%v", got)
	}
}

func TestProduceStopKeepsEarlierExports(t *testing.T) {
	cause := errors.New("缺镜头")
	if got := produceStop(0, 1, cause); got != cause {
		t.Fatalf("first failure: %v", got)
	}
	got := produceStop(1, 2, cause)
	if !errors.Is(got, cause) || !strings.Contains(got.Error(), "已导出 1 条。第 2 条没有完成") {
		t.Fatalf("later failure: %v", got)
	}
}

func TestProduceVideoExportsEachCandidate(t *testing.T) {
	requireProduceMedia(t)
	dir := t.TempDir()
	t.Setenv("QINGFAN_DATA", dir)
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	app := &App{creds: &credentials.Memory{}}
	app.startup(ctx)
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if app.storeErr != nil {
		t.Fatal(app.storeErr)
	}
	if app.aiErr != nil {
		t.Fatal(app.aiErr)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}

	project, err := app.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := app.CreateSKU(project.ID, "演示商品", "瓶盖可以单手打开")
	if err != nil {
		t.Fatal(err)
	}
	importOne := func(kind, name string) store.ImportResult {
		t.Helper()
		got, err := app.ImportFile(sku.ID, kind, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", name))
		if err != nil {
			t.Fatal(err)
		}
		if got.AssetID == "" {
			t.Fatal("missing asset id")
		}
		return got
	}
	importOne(timeline.KindClip, filepath.Join("media", "clip_red.mp4"))
	importOne(timeline.KindClip, filepath.Join("media", "clip_blue.mp4"))
	bgm := importOne(timeline.KindBGM, filepath.Join("media", "bgm_demo.wav"))
	font := importOne(timeline.KindFont, filepath.Join("fonts", "Lato-Regular.ttf"))

	result, err := app.ProduceVideo(project.ID, sku.ID, font.AssetID, bgm.AssetID, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidates != 2 || len(result.Outputs) != 2 {
		t.Fatalf("candidates %d outputs %d", result.Candidates, len(result.Outputs))
	}
	if result.KnownMicros <= 0 || result.UnknownCalls != 0 {
		t.Fatalf("usage %+v", result)
	}
	if !strings.Contains(result.Note, "印尼语还没有人工审核") {
		t.Fatal(result.Note)
	}
	var firstSize int64
	for _, item := range result.Outputs {
		rel, err := filepath.Rel(dir, item.Path)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			t.Fatalf("export left the data dir: %s", item.Path)
		}
		info, err := os.Stat(item.Path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("export %s: %v", item.Path, err)
		}
		if firstSize == 0 {
			firstSize = info.Size()
		}
	}
	probed, err := media.Probe(ctx, result.Outputs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if probed.Width != 1080 || probed.Height != 1920 || probed.AvgFPS < 29 || probed.AvgFPS > 31 || !probed.HasAudio || probed.DurationMS <= 0 {
		t.Fatalf("%+v", probed)
	}

	spent := usageOf(t, app)
	again, err := app.ProduceVideo(project.ID, sku.ID, font.AssetID, bgm.AssetID, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Outputs) != 2 || again.Outputs[0].Path != result.Outputs[0].Path {
		t.Fatalf("repeat should keep the files: %+v", again.Outputs)
	}
	if !strings.Contains(again.Note, "沿用已导出的成片") || !strings.Contains(again.Note, "印尼语还没有人工审核") {
		t.Fatal(again.Note)
	}
	if againSpent := usageOf(t, app); againSpent != spent {
		t.Fatalf("repeat called the provider: before %+v after %+v", spent, againSpent)
	}
	kept, err := os.Stat(result.Outputs[0].Path)
	if err != nil || kept.Size() != firstSize {
		t.Fatalf("first export changed: %v size %d", err, kept.Size())
	}

	wav, err := ai.WavSilence(2000)
	if err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(dir, "bgm-2.wav")
	if err := os.WriteFile(otherPath, wav, 0o644); err != nil {
		t.Fatal(err)
	}
	other, err := app.ImportFile(sku.ID, timeline.KindBGM, timeline.SourceOwned, store.RightsUsable, otherPath)
	if err != nil {
		t.Fatal(err)
	}
	if other.AssetID == bgm.AssetID {
		t.Fatal("replacement BGM reused the first asset")
	}
	remixed, err := app.ProduceVideo(project.ID, sku.ID, font.AssetID, other.AssetID, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(remixed.Outputs) != 2 || remixed.Outputs[0].Path == result.Outputs[0].Path {
		t.Fatalf("bgm remix %+v", remixed.Outputs)
	}
	if !strings.Contains(remixed.Note, "配音没有重做") {
		t.Fatal(remixed.Note)
	}
	if remixedSpent := usageOf(t, app); remixedSpent != spent {
		t.Fatalf("bgm remix called the provider: before %+v after %+v", spent, remixedSpent)
	}
	kept, err = os.Stat(result.Outputs[0].Path)
	if err != nil || kept.Size() != firstSize {
		t.Fatalf("first export changed after remix: %v size %d", err, kept.Size())
	}
	probed, err = media.Probe(ctx, remixed.Outputs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if probed.Width != 1080 || probed.Height != 1920 || !probed.HasAudio {
		t.Fatalf("remix %+v", probed)
	}
}

func TestCheckedClipsLeaveTheOtherShotOut(t *testing.T) {
	kept, err := keepClipAssets([]store.Clip{
		{ID: "c1", AssetID: "a1"},
		{ID: "c2", AssetID: "a2"},
	}, []string{" a1 "})
	if err != nil || len(kept) != 1 || kept[0].ID != "c1" {
		t.Fatalf("kept %+v %v", kept, err)
	}
	if _, err := keepClipAssets(nil, []string{" "}); err == nil || err.Error() != "先选择镜头" {
		t.Fatalf("blank selection: %v", err)
	}
	all, err := keepClipAssets([]store.Clip{{ID: "c1", AssetID: "a1"}}, nil)
	if err != nil || len(all) != 1 {
		t.Fatalf("empty selection uses every clip: %+v %v", all, err)
	}

	requireProduceMedia(t)
	dir := t.TempDir()
	t.Setenv("QINGFAN_DATA", dir)
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	app := &App{creds: &credentials.Memory{}}
	app.startup(ctx)
	t.Cleanup(func() { app.shutdown(context.Background()) })
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	project, err := app.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := app.CreateSKU(project.ID, "演示商品", "瓶盖可以单手打开")
	if err != nil {
		t.Fatal(err)
	}
	red, err := app.ImportFile(sku.ID, timeline.KindClip, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", "media", "clip_red.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if red.DurationMS <= 0 {
		t.Fatalf("probe duration: %d", red.DurationMS)
	}
	if _, err := app.ImportFile(sku.ID, timeline.KindClip, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", "media", "clip_blue.mp4")); err != nil {
		t.Fatal(err)
	}
	bgm, err := app.ImportFile(sku.ID, timeline.KindBGM, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", "media", "bgm_demo.wav"))
	if err != nil {
		t.Fatal(err)
	}
	font, err := app.ImportFile(sku.ID, timeline.KindFont, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", "fonts", "Lato-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.ProduceVideo(project.ID, sku.ID, font.AssetID, bgm.AssetID, []string{red.AssetID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outputs) == 0 {
		t.Fatal("no export")
	}
	base := strings.TrimSuffix(filepath.Base(result.Outputs[0].Path), filepath.Ext(result.Outputs[0].Path))
	draftID := strings.TrimSuffix(base, "-01")
	draft, err := app.GetDraft(draftID)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, id := range red.ClipIDs {
		allowed[id] = true
	}
	shots := 0
	for _, seg := range draft.Segments {
		for _, shot := range seg.Shots {
			shots++
			if !allowed[shot.ClipID] {
				t.Fatalf("shot %s is not the checked clip", shot.ClipID)
			}
		}
	}
	if shots == 0 {
		t.Fatal("draft has no shots")
	}
}

func TestPickMaterialNeedsTheDesktopWindow(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	if _, err := app.PickMaterial(); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	if _, err := app.PickMaterial(); err == nil || err.Error() != "选择文件需要在桌面应用里打开" {
		t.Fatalf("dialog: %v", err)
	}
}

func TestMixFromLinksRejectsTheRequest(t *testing.T) {
	t.Setenv("QINGFAN_DATA", t.TempDir())
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	app := &App{creds: &credentials.Memory{}}
	app.startup(context.Background())
	t.Cleanup(func() { app.shutdown(context.Background()) })
	if _, err := app.MixFromLinks("project", "sku", "", "", "提示", []string{"https://cdn.example.com/a.mp4"}); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatalf("before login: %v", err)
	}
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	if _, err := app.MixFromLinks("", "sku", "", "", "提示", []string{"https://cdn.example.com/a.mp4"}); err == nil || err.Error() != "先填写商品" {
		t.Fatalf("product: %v", err)
	}
	if _, err := app.MixFromLinks("project", "sku", "", "", "  ", []string{"https://cdn.example.com/a.mp4"}); err == nil || err.Error() != "先填写提示词" {
		t.Fatalf("prompt: %v", err)
	}
	if _, err := app.MixFromLinks("project", "sku", "", "", "提示", []string{"  "}); err == nil || err.Error() != "先填写视频链接" {
		t.Fatalf("links: %v", err)
	}
	if _, err := app.MixFromLinks("project", "sku", "", "", "提示", []string{"file:///tmp/a.mp4"}); !errors.Is(err, media.ErrVideoURL) {
		t.Fatalf("file url: %v", err)
	}
	if _, err := app.MixFromLinks("project", "sku", "", "", "提示", []string{"https://user:secret@cdn.example.com/a.mp4"}); !errors.Is(err, media.ErrVideoUser) {
		t.Fatalf("userinfo: %v", err)
	}
	project, err := app.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := app.CreateSKU(project.ID, "演示商品", "旧事实")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.MixFromLinks(project.ID, sku.ID, "", "", "提示", []string{"http://169.254.169.254/a.mp4"}); !errors.Is(err, media.ErrVideoBlocked) {
		t.Fatalf("link local: %v", err)
	}
	saved, err := app.db.GetSKU(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Facts != "旧事实" {
		t.Fatalf("facts changed before a download: %q", saved.Facts)
	}
}

func TestMixFromLinksUsesTheDownloadedFile(t *testing.T) {
	requireProduceMedia(t)
	dir := t.TempDir()
	t.Setenv("QINGFAN_DATA", dir)
	t.Setenv("QINGFAN_PROVIDER_URL", "")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	app := &App{creds: &credentials.Memory{}}
	app.startup(ctx)
	t.Cleanup(func() { app.shutdown(context.Background()) })
	app.who = auth.Account{Email: "desk@example.com", Kind: auth.KindDesktop}
	project, err := app.CreateProject("演示", "ID", "id-ID")
	if err != nil {
		t.Fatal(err)
	}
	sku, err := app.CreateSKU(project.ID, "演示商品", "旧事实")
	if err != nil {
		t.Fatal(err)
	}
	blue, err := app.ImportFile(sku.ID, timeline.KindClip, timeline.SourceOwned, store.RightsUsable, filepath.Join("tests", "fixtures", "media", "clip_blue.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	redBody, err := os.ReadFile(filepath.Join("tests", "fixtures", "media", "clip_red.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	videoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write(redBody)
	}))
	t.Cleanup(videoSrv.Close)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<html><body>不是视频</body></html>"))
	}))
	t.Cleanup(pageSrv.Close)
	const prompt = "先讲痛点，再演示开盖"
	if _, err := app.MixFromLinks(project.ID, sku.ID, "", "", prompt, []string{pageSrv.URL + "/watch"}); err == nil || err.Error() != "这个链接不是可下载的视频文件" {
		t.Fatalf("page: %v", err)
	}
	unchanged, err := app.db.GetSKU(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Facts != "旧事实" {
		t.Fatalf("facts changed after a page link: %q", unchanged.Facts)
	}
	result, err := app.MixFromLinks(project.ID, sku.ID, "", "", prompt, []string{videoSrv.URL + "/clip_red.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fetched) != 1 || result.Fetched[0].DurationMS <= 0 || len(result.Outputs) == 0 {
		t.Fatalf("result %+v", result)
	}
	saved, err := app.db.GetSKU(sku.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Facts != prompt {
		t.Fatalf("facts %q", saved.Facts)
	}
	base := strings.TrimSuffix(filepath.Base(result.Outputs[0].Path), filepath.Ext(result.Outputs[0].Path))
	draft, err := app.GetDraft(strings.TrimSuffix(base, "-01"))
	if err != nil {
		t.Fatal(err)
	}
	blocked := map[string]bool{}
	for _, id := range blue.ClipIDs {
		blocked[id] = true
	}
	shots := 0
	for _, seg := range draft.Segments {
		for _, shot := range seg.Shots {
			shots++
			if blocked[shot.ClipID] {
				t.Fatalf("shot %s came from the other file", shot.ClipID)
			}
		}
	}
	if shots == 0 {
		t.Fatal("draft has no shots")
	}
}

func usageOf(t *testing.T, app *App) ai.CostReport {
	t.Helper()
	secret, err := app.creds.Get("local")
	if err != nil {
		t.Fatal(err)
	}
	report, err := app.ai.Costs(secret)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestReuseExportFollowsTheDiff(t *testing.T) {
	base := jobs.Fingerprint{
		AnalysisKey: "analysis", RenderKey: "render", BGMKey: "bgm", CaptionKey: "caption",
		Segments: []jobs.SegmentPrint{{ID: "seg_1", VoiceKey: "voice", ShotKey: "shot"}},
	}
	ready := filepath.Join(t.TempDir(), "cut.mp4")
	if err := os.WriteFile(ready, []byte("mp4"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !reuseExport(jobs.Diff(base, base), ready) {
		t.Fatal("unchanged cut should be reused")
	}
	if reuseExport(jobs.Diff(base, base), filepath.Join(t.TempDir(), "missing.mp4")) {
		t.Fatal("missing file was reused")
	}
	bgm := base
	bgm.BGMKey = "bgm-2"
	if reuseExport(jobs.Diff(base, bgm), ready) {
		t.Fatal("bgm change was reused")
	}
	voice := base
	voice.Segments = append([]jobs.SegmentPrint(nil), base.Segments...)
	voice.Segments[0].VoiceKey = "voice-2"
	if reuseExport(jobs.Diff(base, voice), ready) {
		t.Fatal("voice change was reused")
	}
	analysis := base
	analysis.AnalysisKey = "analysis-2"
	if !reuseExport(jobs.Diff(base, analysis), ready) {
		t.Fatal("analysis-only change should keep the file")
	}
}

func requireProduceMedia(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s is not installed", name)
		}
		out, err := exec.Command(path, "-version").CombinedOutput()
		if err != nil {
			t.Fatalf("%s at %s failed to start: %v\n%s", name, path, err, out)
		}
	}
}
