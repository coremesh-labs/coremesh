package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
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
	os.WriteFile(filepath.Join(src, "run"), []byte("bin"), 0o755)
	a1, a2 := filepath.Join(t.TempDir(), "x.tar.gz"), filepath.Join(t.TempDir(), "y.tar.gz")
	if err := writeArchive(src, a1); err != nil {
		t.Fatal(err)
	}
	if err := untar(a1, dst); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "configs", "01.yaml"))
	if err != nil || string(b) != "a: 1\n" {
		t.Fatalf("%q %v", b, err)
	}
	if fi, _ := os.Stat(filepath.Join(dst, "run")); fi.Mode().Perm()&0o100 == 0 {
		t.Fatal("run nicht ausführbar")
	}
	// gleicher Inhalt → gleiche Prüfsumme (Wiederverwendung in späteren Ständen)
	os.Chtimes(filepath.Join(src, "run"), time.Now(), time.Now().Add(-time.Hour))
	writeArchive(src, a2)
	if s1, _ := fileSHA(a1); s1 != func() string { s2, _ := fileSHA(a2); return s2 }() {
		t.Fatal("Archiv hängt von Zeiten ab")
	}
	if _, err := safeJoin(dst, "../böse"); err == nil {
		t.Fatal("Pfad außerhalb ohne Fehler")
	}
}

// server legt Bausteine wie ein Release ab und liefert die Quelle dazu.
type testServer struct {
	t   *testing.T
	dir string
	ix  Index
}

func newTestServer(t *testing.T) *testServer {
	d := t.TempDir()
	return &testServer{t: t, dir: d, ix: Index{Format: 1, Repositories: map[string]string{"local": "file://" + filepath.ToSlash(d) + "/{release}/{file}"},
		Lines: map[string]string{}}}
}

// put legt eine Datei als Baustein ab (gzip bzw. tar.gz bei files).
func (ts *testServer) put(rel, kind, name, version, path string, content []byte) Artifact {
	raw := filepath.Join(ts.t.TempDir(), "raw")
	os.WriteFile(raw, content, 0o755)
	dir := filepath.Join(ts.dir, rel)
	os.MkdirAll(dir, 0o755)
	file := filepath.Join(dir, name+"-"+version+".gz")
	if kind == kindFiles {
		src := ts.t.TempDir()
		os.MkdirAll(filepath.Join(src, "configs"), 0o755)
		os.WriteFile(filepath.Join(src, "configs", "01.yaml"), content, 0o644)
		file = filepath.Join(dir, "files-"+version+".tar.gz")
		if err := writeArchive(src, file); err != nil {
			ts.t.Fatal(err)
		}
	} else if err := gzipFile(raw, file); err != nil {
		ts.t.Fatal(err)
	}
	a, _ := artifact(file)
	a.Path = path
	if kind != kindFiles {
		a.Content, _ = fileSHA(raw)
	}
	return a
}

func (ts *testServer) publish(m *Manifest) *source {
	m.Format = 2
	os.MkdirAll(filepath.Join(ts.dir, m.Release), 0o755)
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(ts.dir, m.Release, "manifest.json"), b, 0o644)
	ts.ix.Releases = append(ts.ix.Releases, ReleaseRef{ID: m.Release, Repository: "local"})
	ts.ix.Lines["release"] = m.Release
	b, _ = json.Marshal(ts.ix)
	os.WriteFile(filepath.Join(ts.dir, "index.json"), b, 0o644)
	return newSource(filepath.Join(ts.dir, "index.json"))
}

func comp(kind, name, version string, a Artifact) Component {
	plat := platform()
	if kind == kindFiles {
		plat = "any"
	}
	return Component{Name: name, Kind: kind, Version: version, Platforms: map[string]Artifact{plat: a}}
}

// Update von R1 auf R2: nur das geänderte Plugin und die Dateien werden geladen, das
// gleiche Programm aus dem alten Stand übernommen, das gleiche Plugin bleibt liegen.
func TestPlanLoadsOnlyMissing(t *testing.T) {
	ts := newTestServer(t)
	host := ts.put("R1", kindProgram, "host", "h1", exe("host"), []byte("host v1"))
	pa1 := ts.put("R1", kindPlugin, "partner", "1.0.0", "plugins/pa/partner-1.0.0", []byte("partner 1"))
	we1 := ts.put("R1", kindPlugin, "webserver", "1.0.0", "plugins/we/webserver-1.0.0", []byte("web 1"))
	f1 := ts.put("R1", kindFiles, "files", "f1", "", []byte("a: 1\n"))
	pkgs := map[string]Package{"erp": {Title: "ERP"}}
	r1 := &Manifest{Release: "R1", Packages: pkgs, Components: []Component{
		comp(kindProgram, "host", "h1", host), comp(kindPlugin, "partner", "1.0.0", pa1),
		comp(kindPlugin, "webserver", "1.0.0", we1), comp(kindFiles, "files", "f1", f1)}}
	src := ts.publish(r1)

	in := &installation{dir: t.TempDir()}
	ix, _ := src.Index()
	m, err := src.Manifest(ix, "R1")
	if err != nil {
		t.Fatal(err)
	}
	p, err := makePlan(m, in, "erp", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Count(actLoad) != 4 {
		t.Fatalf("neu: %d zu laden", p.Count(actLoad))
	}
	if err := in.prepare(src, ix, m, p, func(string) {}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(in.path("current"), []byte("R1\n"), 0o644)
	if b, _ := os.ReadFile(in.path("plugins", "we", "webserver-1.0.0")); string(b) != "web 1" {
		t.Fatalf("Plugin: %q", b)
	}
	if b, _ := os.ReadFile(in.path("releases", "R1", "configs", "01.yaml")); string(b) != "a: 1\n" {
		t.Fatalf("Dateien: %q", b)
	}

	// R2: webserver neu, partner und host aus R1 (Verweis), Dateien geändert
	we2 := ts.put("R2", kindPlugin, "webserver", "1.1.0", "plugins/we/webserver-1.1.0", []byte("web 2"))
	f2 := ts.put("R2", kindFiles, "files", "f2", "", []byte("a: 2\n"))
	pa1.Release, host.Release = "R1", "R1"
	src = ts.publish(&Manifest{Release: "R2", Packages: pkgs, Components: []Component{
		comp(kindProgram, "host", "h1", host), comp(kindPlugin, "partner", "1.0.0", pa1),
		comp(kindPlugin, "webserver", "1.1.0", we2), comp(kindFiles, "files", "f2", f2)}})
	ix, _ = src.Index()
	m, _ = src.Manifest(ix, "R2")
	p, err = makePlan(m, in, "erp", false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range p.Items {
		got[it.Name] = it.Action + " " + it.Old + "→" + it.New
	}
	want := map[string]string{"host": "übernehmen h1→h1", "partner": "vorhanden 1.0.0→1.0.0",
		"webserver": "laden 1.0.0→1.1.0", "files": "laden f1→f2"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, erwartet %q", k, got[k], v)
		}
	}
	if len(p.Changed()) != 2 {
		t.Errorf("geändert: %d", len(p.Changed()))
	}
	if err := in.prepare(src, ix, m, p, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(in.path("releases", "R2", exe("host"))); string(b) != "host v1" {
		t.Fatalf("Programm übernommen: %q", b)
	}
	if _, err := os.Stat(in.path("plugins", "we", "webserver-1.0.0")); err != nil {
		t.Fatal("alte Plugin-Version fehlt (Rückfall)")
	}
	for _, s := range []string{"start", "stop", "status"} {
		name := s + ".sh"
		if runtime.GOOS == "windows" {
			name = s + ".cmd"
		}
		if _, err := os.Stat(in.path(name)); err != nil {
			t.Errorf("Skript %s fehlt", name)
		}
	}
}

// pack: gleiches Plugin (Version) und gleiches Programm (Inhalt) verweisen auf den
// vorherigen Stand und werden nicht noch einmal abgelegt.
func TestPackerReuse(t *testing.T) {
	out, src := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "p"), []byte("plugin"), 0o755)
	os.WriteFile(filepath.Join(src, "h"), []byte("host neu"), 0o755)
	psum, _ := fileSHA(filepath.Join(src, "p"))
	prev := &Manifest{Release: "R1", Components: []Component{
		{Name: "partner", Kind: kindPlugin, Version: "1.0.0", Platforms: map[string]Artifact{platform(): {File: "partner-1.0.0.gz", Content: psum}}},
		{Name: "host", Kind: kindProgram, Version: "alt", Platforms: map[string]Artifact{platform(): {File: "host-alt.gz", Content: "anders"}}},
	}}
	pk := &packer{out: out, prev: prev, m: &Manifest{Release: "R2"}}
	if err := pk.file(kindPlugin, "partner", "1.0.0", platform(), filepath.Join(src, "p"), "plugins/pa/partner-1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := pk.file(kindProgram, "host", "", platform(), filepath.Join(src, "h"), "host"); err != nil {
		t.Fatal(err)
	}
	if pk.reused != 1 || pk.written != 1 {
		t.Fatalf("wiederverwendet %d, abgelegt %d", pk.reused, pk.written)
	}
	if a := pk.m.component(kindPlugin, "partner").Platforms[platform()]; a.Release != "R1" || a.Path != "plugins/pa/partner-1.0.0" {
		t.Fatalf("Verweis: %+v", a)
	}
	files, _ := filepath.Glob(filepath.Join(out, "*"))
	if len(files) != 1 || !strings.HasPrefix(filepath.Base(files[0]), "host-") {
		t.Fatalf("abgelegt: %v", files)
	}
}

func TestBackupRestore(t *testing.T) {
	in := &installation{dir: t.TempDir()}
	os.MkdirAll(in.path("data"), 0o755)
	os.WriteFile(in.path("data", "coremesh.db"), []byte("vorher"), 0o644)
	b, err := in.backupDB("R1")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(in.path("data", "coremesh.db"), []byte("nachher"), 0o644)
	os.WriteFile(in.path("data", "coremesh.db-wal"), []byte("wal"), 0o644)
	if err := in.restoreDB(b); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(in.path("data", "coremesh.db")); string(got) != "vorher" {
		t.Fatalf("Datenbank: %q", got)
	}
	if _, err := os.Stat(in.path("data", "coremesh.db-wal")); err == nil {
		t.Fatal("WAL nach dem Zurückholen noch da")
	}
	if !strings.Contains(backupTime(b), ".") {
		t.Fatalf("Zeit: %s", backupTime(b))
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
