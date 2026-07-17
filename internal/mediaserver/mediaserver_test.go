package mediaserver

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServesCurrentFile(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	body := "fake-video-bytes"
	s.SetPath(writeTemp(t, body))

	resp, err := http.Get(s.MediaURL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
	// Cross-origin: the webview loads the page from another origin and draws the
	// frame onto a <canvas> (calibration). Without CORS the canvas taints.
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing permissive CORS header")
	}
}

func TestNoVideoOpen(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp, err := http.Get(s.MediaURL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no path set", resp.StatusCode)
	}
}

func TestRangeRequestSupported(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.SetPath(writeTemp(t, "0123456789"))

	req, _ := http.NewRequest(http.MethodGet, s.MediaURL(), nil)
	req.Header.Set("Range", "bytes=2-5")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206 for a Range request", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "2345" {
		t.Fatalf("range body = %q, want %q", got, "2345")
	}
}

func TestMediaURLShape(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u := s.MediaURL()
	if !strings.HasPrefix(u, "http://127.0.0.1:") || !strings.HasSuffix(u, "/media/current") {
		t.Fatalf("MediaURL = %q, want http://127.0.0.1:<port>/media/current", u)
	}
}
