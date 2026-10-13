package reporting

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCSVPreviewKeepsUnknownsAndRejectsBadRows(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "tests", "fixtures", "reporting", "metrics.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect([]byte("source,shop\n")); err != ErrHeader {
		t.Fatal(err)
	}
	preview, err := Inspect(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Rows) != 5 {
		t.Fatalf("valid rows %d", len(preview.Rows))
	}
	unknown := preview.Rows[1]
	if unknown.Views != nil || unknown.GMVMinor != nil || unknown.Orders == nil || *unknown.Orders != 0 {
		t.Fatalf("blank became a number: %+v", unknown)
	}
	zero := preview.Rows[2]
	if zero.Views == nil || *zero.Views != 0 || zero.GMVMinor == nil || *zero.GMVMinor != 0 {
		t.Fatalf("written zero was dropped: %+v", zero)
	}
	if len(preview.Sources) != 2 || preview.Sources[0] != "shop_export" || preview.Sources[1] != "ads_export" {
		t.Fatalf("sources %+v", preview.Sources)
	}
	if len(preview.Errors) != 3 {
		t.Fatalf("errors %+v", preview.Errors)
	}
	bound := map[string]bool{"7123456789012345678": true, "7999888777666555444": true}
	keep, extra, unbound := Classify(preview.Rows, bound, nil)
	if len(unbound) != 0 || len(keep) != 4 || len(extra) != 1 {
		t.Fatalf("keep %d extra %+v unbound %+v", len(keep), extra, unbound)
	}
	var shopGMV, adsGMV int
	for _, row := range keep {
		if row.Source == "shop_export" && row.GMVMinor != nil {
			shopGMV += *row.GMVMinor
		}
		if row.Source == "ads_export" && row.GMVMinor != nil {
			adsGMV += *row.GMVMinor
		}
	}
	if shopGMV != 50000 || adsGMV != 1000 {
		t.Fatalf("sources were combined: shop %d ads %d", shopGMV, adsGMV)
	}
	again, extra, _ := Classify(keep, bound, map[string]bool{keep[0].Key(): true})
	if len(again) != 3 || len(extra) != 1 {
		t.Fatalf("duplicate counted again: %d %+v", len(again), extra)
	}
}

func TestBindRequiresAnOutputAndAllowsAHandEnteredVideoID(t *testing.T) {
	if _, err := Bind("", "", "7123", "shop-a", "2026-10-13T00:00:00Z", true); err != ErrOutput {
		t.Fatal(err)
	}
	if _, err := Bind("out-1", "", "7123", "shop-a", "2026-10-13T00:00:00Z", false); err != ErrOutput {
		t.Fatal(err)
	}
	if _, err := Bind("out-1", "https://www.tiktok.com/t/short/", "", "shop-a", "2026-10-13T00:00:00Z", true); err != ErrVideo {
		t.Fatal(err)
	}
	got, err := Bind("out-1", "", "7123456789012345678", "shop-a", "2026-10-13T00:00:00Z", true)
	if err != nil || got.URL != "" || got.VideoID != "7123456789012345678" || got.OutputID != "out-1" {
		t.Fatalf("%+v %v", got, err)
	}
}
