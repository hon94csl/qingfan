package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hon94csl/qingfan/internal/jobs"
	"github.com/hon94csl/qingfan/internal/reporting"
)

func TestPublicationAndCSVDoNotChangeTheExport(t *testing.T) {
	dir := t.TempDir()
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), filepath.Join(dir, "cache"), (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, _, draft := mustDraft(t, s)
	saved, err := s.SaveDraft(draft)
	if err != nil {
		t.Fatal(err)
	}
	exported := filepath.Join(dir, "export.mp4")
	payload := []byte("frozen-export")
	if err := os.WriteFile(exported, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	const manifest = `{"output_id":"out-1"}`
	if _, err := s.db.Exec(`INSERT INTO outputs(id, draft_id, draft_revision, path, manifest_json, created_at) VALUES ('out-1', ?, 1, ?, ?, ?)`, saved.ID, exported, manifest, nowText()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindPublication("missing", "", "7123", "shop-a", "2026-10-13T00:00:00Z"); err != reporting.ErrOutput {
		t.Fatal(err)
	}
	first, err := s.BindPublication("out-1", "", "7123456789012345678", "shop-a", "2026-10-13T00:00:00Z")
	if err != nil || first.URL != "" {
		t.Fatalf("%+v %v", first, err)
	}
	if _, err := s.BindPublication("out-1", "https://www.tiktok.com/@shop/video/7999888777666555444", "7999888777666555444", "shop-b", "2026-10-13T01:00:00Z"); err != nil {
		t.Fatal(err)
	}
	pubs, err := s.ListPublications("out-1")
	if err != nil || len(pubs) != 2 {
		t.Fatalf("%+v %v", pubs, err)
	}
	body, err := os.ReadFile(exported)
	if err != nil || string(body) != string(payload) {
		t.Fatalf("export changed: %q %v", body, err)
	}
	path, gotManifest, err := s.outputSnapshot("out-1")
	if err != nil || path != exported || gotManifest != manifest {
		t.Fatalf("record changed: %s %s %v", path, gotManifest, err)
	}

	csvPath := filepath.Join(callerRoot(t), "tests", "fixtures", "reporting", "metrics.csv")
	csvBody, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewMetrics(csvBody)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Accepted != 4 || len(preview.Unbound) != 0 || len(preview.Sources) != 2 {
		t.Fatalf("preview %+v", preview)
	}
	if n, err := s.countMetrics(); err != nil || n != 0 {
		t.Fatalf("preview wrote %d %v", n, err)
	}
	imported, err := s.ImportMetrics(csvBody)
	if err != nil || imported.Accepted != 4 || imported.Repeated {
		t.Fatalf("%+v %v", imported, err)
	}
	unknown, err := s.metricByDay("shop_export", "7123456789012345678", "2026-10-13")
	if err != nil || unknown.Views.Valid || unknown.GMV.Valid || !unknown.Orders.Valid || unknown.Orders.Int64 != 0 {
		t.Fatalf("unknown stored as zero: %+v %v", unknown, err)
	}
	zeros, err := s.metricByDay("shop_export", "7123456789012345678", "2026-10-14")
	if err != nil || !zeros.Views.Valid || zeros.Views.Int64 != 0 || !zeros.GMV.Valid || zeros.GMV.Int64 != 0 {
		t.Fatalf("real zero dropped: %+v %v", zeros, err)
	}
	ads, err := s.metricByDay("ads_export", "7999888777666555444", "2026-10-12")
	if err != nil || !ads.GMV.Valid || ads.GMV.Int64 != 1000 {
		t.Fatalf("ads row %+v %v", ads, err)
	}
	again, err := s.ImportMetrics(csvBody)
	if err != nil || !again.Repeated || again.Accepted != 0 {
		t.Fatalf("repeat %+v %v", again, err)
	}
	if n, err := s.countMetrics(); err != nil || n != 4 {
		t.Fatalf("count %d %v", n, err)
	}
}

func TestStepSuccessRequiresAnArtifactAndRestartClearsRunning(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	s, err := open(filepath.Join(dir, "qingfan.sqlite"), cache, (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(cache, "voice.wav")
	if err := os.WriteFile(ready, []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.PutStep("job-1", "missing", jobs.Succeeded, filepath.Join(cache, "absent.wav"), ""); err == nil {
		t.Fatal("missing artifact was marked successful")
	}
	if err := s.PutStep("job-1", "run", jobs.Running, ready, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.PutStep("job-1", "done", jobs.Succeeded, ready, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.PutStep("job-1", "stop", jobs.Cancelled, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := open(filepath.Join(dir, "qingfan.sqlite"), cache, (&fakeIngest{}).Ingest)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	steps, err := reopened.RecoverJob("job-1")
	if err != nil || len(steps) != 3 {
		t.Fatalf("%+v %v", steps, err)
	}
	if steps[0].Status != jobs.Pending || steps[0].Artifact != "" || steps[1].Status != jobs.Succeeded || steps[1].Artifact != ready || steps[2].Status != jobs.Cancelled {
		t.Fatalf("%+v", steps)
	}
}
