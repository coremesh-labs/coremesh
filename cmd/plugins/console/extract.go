package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
)

// Sicheres Entpacken von ZIP-Archiven, die ein Fachmodul als zip_content
// liefert.
//
// Zielverzeichnis:
//   - muss absolut sein,
//   - darf kein Systemverzeichnis des Betriebssystems sein und nicht darunter
//     liegen (systemDirectories + settings.blocked_directories),
//   - darf kein Laufwerks- bzw. Dateisystem-Wurzelverzeichnis sein,
//   - wird vor der Prüfung über Symlinks aufgelöst (ein Link /tmp/x → /etc
//     hilft also nicht).
//
// Ob der Prozess dort schreiben darf, entscheidet das Betriebssystem.
//
// Einträge (Zip-Slip-Schutz):
//   - Namen müssen lokal sein (filepath.IsLocal): keine absoluten Pfade,
//     kein "..", keine Laufwerke, keine reservierten Windows-Namen.
//   - Der aufgelöste Zielpfad muss im Zielverzeichnis liegen – auch wenn dort
//     schon Symlinks existieren.
//   - Nur Dateien und Verzeichnisse; Symlinks und Sonderdateien werden
//     abgelehnt.
//   - Grenzen für Anzahl und entpackte Größe (Schutz vor ZIP-Bomben).
//
// Alle Einträge werden geprüft, bevor die erste Datei geschrieben wird.

type extractLimits struct {
	MaxBytes int64
	MaxFiles int
}

type extractResult struct {
	Directory   string
	Files       int
	Directories int
	Bytes       int64
}

// systemDirectories sind die gesperrten Verzeichnisse des Betriebssystems
// (jeweils mit allen Unterverzeichnissen).
func systemDirectories() []string {
	switch runtime.GOOS {
	case "windows":
		env := func(name, def string) string {
			if v := os.Getenv(name); v != "" {
				return v
			}
			return def
		}
		drive := env("SystemDrive", `C:`)
		return []string{
			env("SystemRoot", drive+`\Windows`),
			env("ProgramFiles", drive+`\Program Files`),
			env("ProgramFiles(x86)", drive+`\Program Files (x86)`),
			env("ProgramW6432", drive+`\Program Files`),
			env("ProgramData", drive+`\ProgramData`),
			drive + `\Boot`,
			drive + `\Recovery`,
			drive + `\System Volume Information`,
			drive + `\$Recycle.Bin`,
		}
	case "darwin":
		return []string{"/System", "/Library", "/bin", "/sbin", "/usr", "/etc", "/private/etc",
			"/private/var/db", "/dev", "/Applications", "/cores", "/Volumes"}
	default: // Linux und andere Unix-Systeme
		return []string{"/bin", "/sbin", "/usr", "/lib", "/lib32", "/lib64", "/libx32", "/boot",
			"/dev", "/proc", "/sys", "/etc", "/run", "/snap", "/var/lib", "/var/run"}
	}
}

// checkTarget prüft das Zielverzeichnis und liefert es bereinigt und über
// Symlinks aufgelöst.
func checkTarget(dir string, blocked []string) (string, error) {
	if dir == "" || !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%w: target_directory muss ein absoluter Pfad sein: %q", sdk.ErrInvalidArgument, dir)
	}
	resolved, err := resolveExisting(filepath.Clean(dir))
	if err != nil {
		return "", err
	}
	if isRoot(resolved) {
		return "", fmt.Errorf("%w: Wurzelverzeichnis %s ist als Ziel nicht erlaubt", sdk.ErrPermissionDenied, resolved)
	}
	for _, b := range blocked {
		if b == "" {
			continue
		}
		for _, cand := range []string{filepath.Clean(b), mustResolve(b)} {
			if isUnder(resolved, cand) {
				return "", fmt.Errorf("%w: %s liegt im Systemverzeichnis %s", sdk.ErrPermissionDenied, dir, cand)
			}
		}
	}
	return resolved, nil
}

// resolveExisting löst Symlinks für den längsten existierenden Teil des Pfads
// auf und hängt den (noch) nicht existierenden Rest wieder an.
func resolveExisting(p string) (string, error) {
	rest := ""
	cur := p
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, rest), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p, nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

func mustResolve(p string) string {
	r, err := resolveExisting(filepath.Clean(p))
	if err != nil {
		return filepath.Clean(p)
	}
	return r
}

func isRoot(p string) bool {
	return filepath.Dir(p) == p
}

// isUnder meldet, ob p gleich base ist oder darunter liegt (unter Windows
// ohne Beachtung der Groß-/Kleinschreibung – filepath.Rel berücksichtigt das).
func isUnder(p, base string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// extractZip entpackt data nach dir.
func extractZip(data []byte, dir string, blocked []string, lim extractLimits) (extractResult, error) {
	target, err := checkTarget(dir, blocked)
	if err != nil {
		return extractResult{}, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return extractResult{}, fmt.Errorf("%w: kein gültiges ZIP: %v", sdk.ErrInvalidArgument, err)
	}
	if len(zr.File) > lim.MaxFiles {
		return extractResult{}, fmt.Errorf("%w: ZIP enthält %d Einträge (max. %d)", sdk.ErrInvalidArgument, len(zr.File), lim.MaxFiles)
	}

	// 1. Alle Einträge prüfen, bevor etwas geschrieben wird.
	type entry struct {
		f    *zip.File
		dest string
		dir  bool
	}
	var entries []entry
	var declared uint64
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(name) {
			return extractResult{}, fmt.Errorf("%w: unsicherer Pfad im ZIP: %q", sdk.ErrInvalidArgument, f.Name)
		}
		dest := filepath.Join(target, name)
		if !isUnder(dest, target) {
			return extractResult{}, fmt.Errorf("%w: Pfad verlässt das Zielverzeichnis: %q", sdk.ErrInvalidArgument, f.Name)
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			entries = append(entries, entry{f, dest, true})
		case mode.IsRegular():
			declared += f.UncompressedSize64
			entries = append(entries, entry{f, dest, false})
		default:
			return extractResult{}, fmt.Errorf("%w: nur Dateien und Verzeichnisse erlaubt, %q ist %v", sdk.ErrInvalidArgument, f.Name, mode.Type())
		}
	}
	if declared > uint64(lim.MaxBytes) {
		return extractResult{}, fmt.Errorf("%w: entpackt %d Bytes (max. %d)", sdk.ErrInvalidArgument, declared, lim.MaxBytes)
	}

	// 2. Entpacken.
	if err := os.MkdirAll(target, 0o755); err != nil {
		return extractResult{}, fmt.Errorf("%w: %v", sdk.ErrPermissionDenied, err)
	}
	res := extractResult{Directory: target}
	remaining := lim.MaxBytes
	for _, e := range entries {
		if e.dir {
			if err := mkdirInside(e.dest, target); err != nil {
				return res, err
			}
			res.Directories++
			continue
		}
		if err := mkdirInside(filepath.Dir(e.dest), target); err != nil {
			return res, err
		}
		if fi, err := os.Lstat(e.dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			return res, fmt.Errorf("%w: %s ist ein Symlink und wird nicht überschrieben", sdk.ErrPermissionDenied, e.dest)
		}
		n, err := writeEntry(e.f, e.dest, remaining)
		res.Bytes += n
		remaining -= n
		if err != nil {
			return res, err
		}
		res.Files++
	}
	return res, nil
}

// mkdirInside legt ein Verzeichnis an und prüft danach, dass es – nach
// Auflösen bestehender Symlinks – im Zielverzeichnis liegt.
func mkdirInside(dir, target string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", sdk.ErrPermissionDenied, err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if !isUnder(resolved, target) {
		return fmt.Errorf("%w: %s führt über einen Symlink aus dem Zielverzeichnis", sdk.ErrPermissionDenied, dir)
	}
	return nil
}

func writeEntry(f *zip.File, dest string, remaining int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %v", sdk.ErrInvalidArgument, f.Name, err)
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", sdk.ErrPermissionDenied, err)
	}
	// Tatsächliche Größe begrenzen – die Angabe im ZIP kann lügen.
	n, err := io.Copy(out, io.LimitReader(rc, remaining+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if n > remaining {
		return n, errors.Join(fmt.Errorf("%w: ZIP überschreitet max_extract_bytes", sdk.ErrInvalidArgument), os.Remove(dest))
	}
	return n, nil
}
