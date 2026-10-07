// Package resolver findet Plugin-Binaries im zentralen Plugin-Verzeichnis und
// lädt fehlende bei Bedarf herunter (z. B. GitLab Generic Package Registry).
//
// Dateinamen: <name>-<version>-<GOOS>-<GOARCH>[.exe]
// Suchreihenfolge:
//  1. <plugin_dir>/<binary>
//  2. <plugin_dir>/<xx>/<binary>   (xx = erste 2 Buchstaben des Namens, klein)
//  3. dasselbe in jedem Verzeichnis aus extra_plugin_dirs
//
// Heruntergeladene Binaries landen immer in <plugin_dir>/<xx>/.
package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coremesh-labs/coremesh/internal/config"
)

// ErrNotFound: Binary weder lokal vorhanden noch herunterladbar.
var ErrNotFound = errors.New("plugin binary not found")

type Resolver struct {
	dir    string
	extra  []string // weitere Suchverzeichnisse (nur lokal)
	dl     config.Download
	client *http.Client
	log    *slog.Logger
	goos   string
	goarch string
}

func New(dir string, dl config.Download, log *slog.Logger, extra ...string) *Resolver {
	return &Resolver{
		dir:    dir,
		extra:  extra,
		dl:     dl,
		client: &http.Client{Timeout: dl.Timeout},
		log:    log,
		goos:   runtime.GOOS,
		goarch: runtime.GOARCH,
	}
}

// BinaryName bildet den Dateinamen nach der Konvention
// <name>-<version>-<goos>-<goarch>, unter Windows mit .exe.
func BinaryName(name, version, goos, goarch string) string {
	n := fmt.Sprintf("%s-%s-%s-%s", name, version, goos, goarch)
	if goos == "windows" {
		n += ".exe"
	}
	return n
}

// Shard liefert den Git-Style-Unterordner: die ersten 2 Zeichen, klein.
func Shard(name string) string {
	return strings.ToLower(name[:2])
}

// Candidates liefert die Suchpfade in Prüfreihenfolge.
func (r *Resolver) Candidates(name, version string) []string {
	bin := BinaryName(name, version, r.goos, r.goarch)
	var out []string
	for _, d := range append([]string{r.dir}, r.extra...) {
		out = append(out, filepath.Join(d, bin), filepath.Join(d, Shard(name), bin))
	}
	return out
}

// Find sucht die Binary lokal.
func (r *Resolver) Find(name, version string) (string, error) {
	if err := validate(name, version); err != nil {
		return "", err
	}
	for _, p := range r.Candidates(name, version) {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: %s (gesucht: %s)", ErrNotFound, name,
		strings.Join(r.Candidates(name, version), ", "))
}

// Resolve sucht die Binary lokal und lädt sie andernfalls herunter.
// wantSHA256 (hex, optional) wird bei lokalen und geladenen Dateien geprüft.
func (r *Resolver) Resolve(ctx context.Context, name, version, wantSHA256 string) (string, error) {
	path, err := r.Find(name, version)
	if err == nil {
		if wantSHA256 != "" {
			if err := verifyFile(path, wantSHA256); err != nil {
				return "", err
			}
		}
		return path, nil
	}
	if !errors.Is(err, ErrNotFound) || r.dl.URLTemplate == "" {
		return "", err
	}
	return r.download(ctx, name, version, wantSHA256)
}

func (r *Resolver) download(ctx context.Context, name, version, wantSum string) (string, error) {
	bin := BinaryName(name, version, r.goos, r.goarch)
	vars := map[string]string{
		"{name}": name, "{version}": version, "{os}": r.goos, "{arch}": r.goarch,
		"{binary}": bin, "{shard}": Shard(name),
	}
	src := expand(r.dl.URLTemplate, vars)
	if err := r.checkURL(src); err != nil {
		return "", err
	}

	if wantSum == "" && r.dl.ChecksumURLTemplate != "" {
		sumURL := expand(r.dl.ChecksumURLTemplate, vars)
		if err := r.checkURL(sumURL); err != nil {
			return "", err
		}
		s, err := r.fetchChecksum(ctx, sumURL)
		if err != nil {
			return "", fmt.Errorf("Prüfsumme laden: %w", err)
		}
		wantSum = s
	}
	if wantSum == "" && !r.dl.AllowUnverified {
		return "", fmt.Errorf("Download von %s abgelehnt: keine SHA-256-Prüfsumme (plugins.%s.sha256 oder resolver.download.checksum_url_template)", name, name)
	}

	targetDir := filepath.Join(r.dir, Shard(name))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(targetDir, bin)

	r.log.Info("Plugin wird heruntergeladen", "plugin", name, "version", version, "url", redact(src))
	tmp, err := os.CreateTemp(targetDir, bin+".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op nach erfolgreichem Rename

	gotSum, err := r.fetchTo(ctx, src, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("Download %s: %w", name, err)
	}
	if wantSum != "" && !strings.EqualFold(gotSum, wantSum) {
		return "", fmt.Errorf("Download %s: SHA-256 stimmt nicht (erwartet %s, erhalten %s)", name, wantSum, gotSum)
	}
	if wantSum == "" {
		r.log.Warn("Plugin ohne Prüfsumme übernommen (allow_unverified)", "plugin", name, "sha256", gotSum)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	r.log.Info("Plugin installiert", "plugin", name, "path", target, "sha256", gotSum)
	return target, nil
}

func (r *Resolver) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if r.dl.TokenEnv != "" {
		if tok := os.Getenv(r.dl.TokenEnv); tok != "" {
			req.Header.Set(r.dl.TokenHeader, tok)
		}
	}
	return req, nil
}

// fetchTo lädt rawURL nach w und liefert die SHA-256 der Daten.
func (r *Resolver) fetchTo(ctx context.Context, rawURL string, w io.Writer) (string, error) {
	req, err := r.newRequest(ctx, rawURL)
	if err != nil {
		return "", err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), io.LimitReader(resp.Body, r.dl.MaxBytes+1))
	if err != nil {
		return "", err
	}
	if n > r.dl.MaxBytes {
		return "", fmt.Errorf("Datei größer als max_bytes (%d)", r.dl.MaxBytes)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fetchChecksum liest eine Prüfsummendatei im sha256sum-Format ("<hex>  <datei>").
func (r *Resolver) fetchChecksum(ctx context.Context, rawURL string) (string, error) {
	var b strings.Builder
	if _, err := r.fetchTo(ctx, rawURL, &limitedWriter{w: &b, n: 4096}); err != nil {
		return "", err
	}
	fields := strings.Fields(b.String())
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", errors.New("ungültiges Format")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", errors.New("ungültiges Format")
	}
	return fields[0], nil
}

func (r *Resolver) checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme == "https" || (u.Scheme == "http" && r.dl.AllowInsecureHTTP) {
		return nil
	}
	return fmt.Errorf("Download-URL muss HTTPS verwenden: %s", redact(raw))
}

func verifyFile(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("%s: SHA-256 stimmt nicht (erwartet %s, erhalten %s)", path, want, got)
	}
	return nil
}

func validate(name, version string) error {
	if !config.ValidPluginName(name) {
		return fmt.Errorf("ungültiger Plugin-Name %q", name)
	}
	if !config.ValidVersion(version) {
		return fmt.Errorf("ungültige Version %q für Plugin %s", version, name)
	}
	return nil
}

func expand(tmpl string, vars map[string]string) string {
	for k, v := range vars {
		tmpl = strings.ReplaceAll(tmpl, k, url.PathEscape(v))
	}
	return tmpl
}

// redact entfernt Query-Parameter (könnten Tokens enthalten) für das Logging.
func redact(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		u.RawQuery = ""
		u.User = nil
		return u.String()
	}
	return raw
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > l.n {
		return 0, errors.New("Prüfsummendatei zu groß")
	}
	l.n -= len(p)
	return l.w.Write(p)
}
