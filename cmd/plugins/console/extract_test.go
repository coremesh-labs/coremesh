package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

type zentry struct {
	name    string
	body    string
	symlink bool
}

func makeZip(t *testing.T, entries ...zentry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.symlink {
			h.SetMode(os.ModeSymlink | 0o777)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(e.body))
	}
	w.Close()
	return buf.Bytes()
}

var lim = extractLimits{MaxBytes: 1 << 20, MaxFiles: 100}

func TestExtractOK(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "neu", "ziel") // existiert noch nicht
	data := makeZip(t,
		zentry{name: "index.html", body: "<h1>Hallo</h1>"},
		zentry{name: "css/"},
		zentry{name: "css/app.css", body: "body{}"},
		zentry{name: "js/deep/x.js", body: "1"},
	)
	res, err := extractZip(data, dir, systemDirectories(), lim)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 3 || res.Directories != 1 || res.Bytes != int64(len("<h1>Hallo</h1>")+len("body{}")+1) {
		t.Fatalf("Ergebnis: %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "css", "app.css")); string(b) != "body{}" {
		t.Fatalf("Inhalt: %q", b)
	}
}

func TestZipSlipRejected(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "boese.txt")
	cases := []string{
		"../boese.txt",
		"a/../../boese.txt",
		"/etc/passwd",
		"..\\boese.txt",
	}
	if runtime.GOOS == "windows" {
		// "CON.txt" ist seit Windows 11 kein Gerätename mehr; "CON" schon.
		cases = append(cases, `C:\Windows\boese.txt`, "C:boese.txt", "NUL", "a/CON")
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			data := makeZip(t, zentry{name: "ok.txt", body: "ok"}, zentry{name: name, body: "x"})
			_, err := extractZip(data, dir, nil, lim)
			if runtime.GOOS != "windows" && name == "..\\boese.txt" {
				// Unter Unix ist "\" ein normales Zeichen: harmloser Dateiname im Ziel.
				if err != nil {
					t.Fatalf("unerwartet: %v", err)
				}
				return
			}
			if !errors.Is(err, sdk.ErrInvalidArgument) {
				t.Fatalf("muss abgelehnt werden: %v", err)
			}
			// Geprüft wird vor dem Schreiben: auch ok.txt wurde nicht angelegt.
			if _, err := os.Stat(filepath.Join(dir, "ok.txt")); err == nil {
				t.Fatal("Teil-Entpackung trotz unsicherem Eintrag")
			}
		})
	}
	if _, err := os.Stat(outside); err == nil {
		t.Fatal("Datei außerhalb des Ziels geschrieben")
	}
}

func TestSymlinkEntryRejected(t *testing.T) {
	data := makeZip(t, zentry{name: "link", body: "/etc/passwd", symlink: true})
	if _, err := extractZip(data, t.TempDir(), nil, lim); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Symlink im ZIP: %v", err)
	}
}

func TestBlockedTargets(t *testing.T) {
	data := makeZip(t, zentry{name: "a.txt", body: "x"})
	var targets []string
	if runtime.GOOS == "windows" {
		sys := os.Getenv("SystemRoot")
		if sys == "" {
			sys = `C:\Windows`
		}
		targets = []string{sys, filepath.Join(sys, "System32"), strings.ToLower(filepath.Join(sys, "Temp", "x")), `C:\Program Files\CoreMesh`, `C:\`}
	} else {
		targets = []string{"/bin", "/bin/sub", "/usr/local/bin", "/etc/coremesh", "/", "/proc/1"}
	}
	for _, dir := range targets {
		if _, err := extractZip(data, dir, systemDirectories(), lim); !errors.Is(err, sdk.ErrPermissionDenied) {
			t.Errorf("%s muss gesperrt sein: %v", dir, err)
		}
	}
	if _, err := extractZip(data, "relativ/pfad", nil, lim); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Errorf("relativer Pfad: %v", err)
	}
	// Zusätzlich konfigurierte Sperre (blocked_directories).
	extra := t.TempDir()
	if _, err := extractZip(data, filepath.Join(extra, "sub"), []string{extra}, lim); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Errorf("blocked_directories: %v", err)
	}
}

func TestSymlinkTargetResolved(t *testing.T) {
	base := t.TempDir()
	blocked := filepath.Join(base, "gesperrt")
	os.Mkdir(blocked, 0o755)
	link := filepath.Join(base, "link")
	if err := os.Symlink(blocked, link); err != nil {
		t.Skipf("Symlinks nicht verfügbar: %v", err)
	}
	data := makeZip(t, zentry{name: "a.txt", body: "x"})
	if _, err := extractZip(data, filepath.Join(link, "sub"), []string{blocked}, lim); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Link auf gesperrtes Verzeichnis: %v", err)
	}
	// Bestehender Symlink im Ziel, der hinausführt.
	target := filepath.Join(base, "ziel")
	os.Mkdir(target, 0o755)
	os.Symlink(base, filepath.Join(target, "raus"))
	data = makeZip(t, zentry{name: "raus/boese.txt", body: "x"})
	if _, err := extractZip(data, target, nil, lim); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Symlink im Ziel: %v", err)
	}
}

func TestExtractLimits(t *testing.T) {
	big := strings.Repeat("A", 2000)
	if _, err := extractZip(makeZip(t, zentry{name: "big.txt", body: big}), t.TempDir(), nil,
		extractLimits{MaxBytes: 1000, MaxFiles: 10}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Größe: %v", err)
	}
	var many []zentry
	for i := 0; i < 11; i++ {
		many = append(many, zentry{name: strings.Repeat("f", i+1), body: "x"})
	}
	if _, err := extractZip(makeZip(t, many...), t.TempDir(), nil, extractLimits{MaxBytes: 1 << 20, MaxFiles: 10}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Anzahl: %v", err)
	}
	if _, err := extractZip([]byte("kein zip"), t.TempDir(), nil, lim); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("kein ZIP: %v", err)
	}
}
