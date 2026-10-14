package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVideoURLRejectsNonHTTP(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/a.mp4", "ftp://example.com/a.mp4", "http://", "/tmp/a.mp4"} {
		if _, err := ParseVideoURL(raw); err != ErrVideoURL {
			t.Fatalf("%q: %v", raw, err)
		}
	}
	if _, err := ParseVideoURL("https://user:secret@example.com/a.mp4"); err != ErrVideoUser {
		t.Fatal(err)
	}
	parsed, err := ParseVideoURL(" https://cdn.example.com/a.mp4 ")
	if err != nil || parsed.Host != "cdn.example.com" {
		t.Fatalf("%v %+v", err, parsed)
	}
}

func TestDownloadRejectsLinkLocalAndKeepsTheFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "meta.mp4")
	err := Download(context.Background(), "http://169.254.169.254/latest/meta-data", dest)
	if err != ErrVideoBlocked {
		t.Fatalf("metadata: %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatal("blocked host left a file")
	}

	body := []byte("video-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/go" {
			http.Redirect(w, r, "/clip.mp4", http.StatusFound)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	got := filepath.Join(dir, "clip.mp4")
	if err := Download(context.Background(), srv.URL+"/go", got); err != nil {
		t.Fatal(err)
	}
	read, err := os.ReadFile(got)
	if err != nil || string(read) != string(body) {
		t.Fatalf("body %q %v", read, err)
	}
	if _, err := os.Stat(got + ".partial"); !os.IsNotExist(err) {
		t.Fatal("partial file remained")
	}
}

func TestDownloadStopsAHugeBody(t *testing.T) {
	previous := maxVideoBytes
	maxVideoBytes = 4
	t.Cleanup(func() { maxVideoBytes = previous })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0123456789"))
	}))
	t.Cleanup(srv.Close)
	dest := filepath.Join(t.TempDir(), "big.mp4")
	if err := Download(context.Background(), srv.URL+"/a.mp4", dest); err != ErrVideoLarge {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("oversized file was kept")
	}
}

func TestDownloadHidesTransportFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("secret-body"))
	}))
	t.Cleanup(srv.Close)
	err := Download(context.Background(), srv.URL+"/missing", filepath.Join(t.TempDir(), "x.mp4"))
	if err != ErrVideoDownload {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), srv.URL) {
		t.Fatal(err)
	}
}
