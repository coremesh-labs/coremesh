package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCleanConfig(t *testing.T) {
	in := "host:\n  extra_plugin_dirs: [../x]\nplugins:\n  a:\n    version: 1.0.0\n"
	got := cleanConfig(in, []*regexp.Regexp{regexp.MustCompile(`^\s*extra_plugin_dirs:`)})
	if strings.Contains(got, "host:") || !strings.Contains(got, "plugins:\n  a:") {
		t.Fatalf("leerer Abschnitt bleibt oder Inhalt fehlt:\n%s", got)
	}
}

func TestExpand(t *testing.T) {
	vars := map[string]any{"faust": map[string]any{"id": "100000", "rec": map[string]any{"no": 7.0}}}
	v, err := expand(map[string]any{"bp": "${faust.id}", "x": []any{"BP ${faust.rec.no}"}}, vars)
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if m["bp"] != "100000" || m["x"].([]any)[0] != "BP 7" {
		t.Fatalf("Ersetzung: %v", m)
	}
	if _, err := expand("${gibtsnicht.id}", vars); err == nil {
		t.Fatal("unbekannte Variable ohne Fehler")
	}
}

func TestSeedArgv(t *testing.T) {
	s := seedStep{Handle: "Contract.activate", ID: "1000|MV-1", Args: map[string]any{"file": "./a.yaml", "n": 3}}
	args, _, err := s.argv("/basis")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"handle", "Contract.activate", "--id=1000|MV-1", "--file=" + filepath.Join("/basis", "a.yaml"), "--n=3"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("argv: %v", args)
	}
	if _, _, err := (seedStep{Handle: "a.b", Command: "x:y"}).argv(""); err == nil {
		t.Fatal("command und handle zugleich ohne Fehler")
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(src, "configs"), 0o755)
	os.WriteFile(filepath.Join(src, "configs", "01.yaml"), []byte("a: 1\n"), 0o644)
	os.WriteFile(filepath.Join(src, "host"), []byte("bin"), 0o755)
	for _, name := range []string{"x.tar.gz", "x.zip"} {
		archive := filepath.Join(t.TempDir(), name)
		if err := writeArchive(src, archive); err != nil {
			t.Fatal(err)
		}
		in := &installation{dir: dst}
		if err := in.unpack(archive, "r1"); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dst, "releases", "r1", "configs", "01.yaml"))
		if err != nil || string(b) != "a: 1\n" {
			t.Fatalf("%s: %q %v", name, b, err)
		}
		if fi, _ := os.Stat(filepath.Join(dst, "releases", "r1", "host")); fi.Mode().Perm()&0o100 == 0 {
			t.Fatalf("%s: host nicht ausführbar", name)
		}
	}
	if _, err := safeJoin(dst, "../böse"); err == nil {
		t.Fatal("Pfad außerhalb ohne Fehler")
	}
}

func TestIndexResolve(t *testing.T) {
	ix := &Index{Format: 1, Repositories: map[string]string{"gh": "https://h/{release}/{file}"},
		Lines: map[string]string{"release": "2026.10.23"}, Releases: []ReleaseRef{{ID: "2026.10.23", Repository: "gh"}}}
	if r, err := ix.resolve(""); err != nil || r != "2026.10.23" {
		t.Fatalf("resolve: %s %v", r, err)
	}
	if u, _ := ix.fileURL("2026.10.23", "manifest.json"); u != "https://h/2026.10.23/manifest.json" {
		t.Fatalf("URL: %s", u)
	}
	if _, err := ix.resolve("1999.01.01"); err == nil {
		t.Fatal("unbekannter Stand ohne Fehler")
	}
}

func TestAccess(t *testing.T) {
	in := &installation{dir: t.TempDir()}
	if err := in.writeAccess("http://localhost:1", "admin", "geheim"); err != nil {
		t.Fatal(err)
	}
	if a, u, p := in.access(); a != "http://localhost:1" || u != "admin" || p != "geheim" {
		t.Fatalf("Zugang: %s %s %s", a, u, p)
	}
	if fi, _ := os.Stat(in.path("ZUGANG.txt")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("ZUGANG.txt: Rechte %v", fi.Mode().Perm())
	}
}
