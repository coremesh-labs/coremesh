package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/camel/coremesh/internal/config"
)

func newTestResolver(dir string, dl config.Download) *Resolver {
	if dl.MaxBytes == 0 {
		dl.MaxBytes = 1 << 20
	}
	if dl.TokenHeader == "" {
		dl.TokenHeader = "PRIVATE-TOKEN"
	}
	dl.Timeout = 5 * time.Second
	r := New(dir, dl, slog.New(slog.DiscardHandler))
	r.goos, r.goarch = "linux", "amd64"
	return r
}

func TestBinaryName(t *testing.T) {
	if got := BinaryName("partner-service", "1.2.0", "linux", "amd64"); got != "partner-service-1.2.0-linux-amd64" {
		t.Error(got)
	}
	if got := BinaryName("partner-service", "1.2.0", "windows", "amd64"); got != "partner-service-1.2.0-windows-amd64.exe" {
		t.Error(got)
	}
}

func TestFindRootThenShard(t *testing.T) {
	dir := t.TempDir()
	r := newTestResolver(dir, config.Download{})
	bin := "partner-service-1.2.0-linux-amd64"

	if _, err := r.Find("partner-service", "1.2.0"); err == nil {
		t.Fatal("ErrNotFound erwartet")
	}
	shard := filepath.Join(dir, "pa", bin)
	os.MkdirAll(filepath.Dir(shard), 0o755)
	os.WriteFile(shard, []byte("x"), 0o755)
	if p, err := r.Find("partner-service", "1.2.0"); err != nil || p != shard {
		t.Fatalf("Shard: %s %v", p, err)
	}
	root := filepath.Join(dir, bin)
	os.WriteFile(root, []byte("x"), 0o755)
	if p, _ := r.Find("partner-service", "1.2.0"); p != root {
		t.Fatalf("Root hat Vorrang: %s", p)
	}
}

func TestDownload(t *testing.T) {
	content := []byte("#!/bin/plugin-binary")
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])

	var gotToken string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotToken = req.Header.Get("PRIVATE-TOKEN")
		switch {
		case strings.HasSuffix(req.URL.Path, ".sha256"):
			w.Write([]byte(want + "  partner-service-1.2.0-linux-amd64\n"))
		case strings.HasSuffix(req.URL.Path, "/partner-service/1.2.0/partner-service-1.2.0-linux-amd64"):
			w.Write(content)
		default:
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()
	t.Setenv("TEST_GITLAB_TOKEN", "tok")

	dl := config.Download{
		URLTemplate: srv.URL + "/packages/generic/{name}/{version}/{binary}",
		TokenEnv:    "TEST_GITLAB_TOKEN",
	}
	ctx := context.Background()

	t.Run("Prüfsumme aus Konfiguration", func(t *testing.T) {
		dir := t.TempDir()
		r := newTestResolver(dir, dl)
		r.client = srv.Client()
		p, err := r.Resolve(ctx, "partner-service", "1.2.0", want)
		if err != nil {
			t.Fatal(err)
		}
		if p != filepath.Join(dir, "pa", "partner-service-1.2.0-linux-amd64") {
			t.Fatalf("Ziel: %s", p)
		}
		if gotToken != "tok" {
			t.Errorf("Token-Header: %q", gotToken)
		}
	})

	t.Run("Prüfsumme aus .sha256-Datei", func(t *testing.T) {
		d := dl
		d.ChecksumURLTemplate = d.URLTemplate + ".sha256"
		r := newTestResolver(t.TempDir(), d)
		r.client = srv.Client()
		if _, err := r.Resolve(ctx, "partner-service", "1.2.0", ""); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("falsche Prüfsumme", func(t *testing.T) {
		dir := t.TempDir()
		r := newTestResolver(dir, dl)
		r.client = srv.Client()
		_, err := r.Resolve(ctx, "partner-service", "1.2.0", strings.Repeat("0", 64))
		if err == nil || !strings.Contains(err.Error(), "SHA-256") {
			t.Fatalf("Fehler erwartet: %v", err)
		}
		if entries, _ := os.ReadDir(filepath.Join(dir, "pa")); len(entries) != 0 {
			t.Fatalf("keine Datei darf zurückbleiben: %v", entries)
		}
	})

	t.Run("ohne Prüfsumme abgelehnt", func(t *testing.T) {
		r := newTestResolver(t.TempDir(), dl)
		r.client = srv.Client()
		if _, err := r.Resolve(ctx, "partner-service", "1.2.0", ""); err == nil {
			t.Fatal("Fehler erwartet")
		}
	})

	t.Run("HTTP abgelehnt", func(t *testing.T) {
		d := dl
		d.URLTemplate = "http://example.invalid/{binary}"
		r := newTestResolver(t.TempDir(), d)
		if _, err := r.Resolve(ctx, "partner-service", "1.2.0", want); err == nil || !strings.Contains(err.Error(), "HTTPS") {
			t.Fatalf("Fehler erwartet: %v", err)
		}
	})
}
