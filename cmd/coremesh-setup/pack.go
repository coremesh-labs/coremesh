package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// packConfig: dist/pack.yaml – welche Repositories, Programme, Inhalte und Ausprägungen.
type packConfig struct {
	Repositories []string            `yaml:"repositories"`
	Build        map[string][]string `yaml:"build"`
	Binaries     map[string][]string `yaml:"binaries"`
	Copy         map[string]string   `yaml:"copy"`
	Configs      struct {
		Exclude   []string `yaml:"exclude"`
		DropLines []string `yaml:"drop_lines"`
	} `yaml:"configs"`
	Packages map[string]struct {
		Title       string   `yaml:"title"`
		Description string   `yaml:"description"`
		Plugins     []string `yaml:"plugins"`
		Data        *struct {
			Title    string `yaml:"title"`
			Seed     string `yaml:"seed"`
			User     string `yaml:"user"`
			Password string `yaml:"password"`
		} `yaml:"data"`
	} `yaml:"packages"`
}

type packOptions struct {
	labs, config, out, release, previous, date string
	build, data                                bool
	port                                       int
	src                                        *source // Server mit dem vorherigen Stand (Wiederverwendung)
}

func platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// pack stellt einen Stand für die eigene Plattform zusammen: je Baustein eine Datei,
// manifest.json und (mit data) je Ausprägung den Datenbestand, eingespielt über die
// Fachobjekte. Bausteine, die der vorherige Stand schon enthält (Plugin in derselben
// Version, Programm oder Dateien mit derselben Prüfsumme), verweisen auf dessen Datei und
// werden nicht noch einmal abgelegt. Ein vorhandenes manifest.json im Zielverzeichnis wird
// um die Plattform ergänzt.
func pack(o packOptions) error {
	var pc packConfig
	b, err := os.ReadFile(o.config)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, &pc); err != nil {
		return fmt.Errorf("%s: %w", o.config, err)
	}
	if o.build {
		for _, r := range pc.Repositories {
			if cmd := pc.Build[r]; len(cmd) > 0 {
				logf("baue %s: %s", r, strings.Join(cmd, " "))
				c := exec.Command(cmd[0], cmd[1:]...)
				c.Dir, c.Stdout, c.Stderr = filepath.Join(o.labs, r), os.Stderr, os.Stderr
				if err := c.Run(); err != nil {
					return fmt.Errorf("bauen %s: %w", r, err)
				}
			}
		}
	}
	// Bühne: programs/ und release/ (zusammen releases/<stand>), plugins/ wie in einer Installation
	stage, err := os.MkdirTemp("", "coremesh-pack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	relDir := filepath.Join(stage, "release")
	var programs []string
	for r, names := range pc.Binaries {
		for _, n := range names {
			if err := copyFile(filepath.Join(o.labs, r, "bin", exe(n)), filepath.Join(stage, "programs", exe(n))); err != nil {
				return err
			}
			programs = append(programs, n)
		}
	}
	sort.Strings(programs)
	for dst, src := range pc.Copy {
		if err := copyTree(filepath.Join(o.labs, src), filepath.Join(relDir, dst)); err != nil {
			return err
		}
	}
	plugins, err := packConfigs(o.labs, relDir, pc)
	if err != nil {
		return err
	}
	pluginFiles, err := packPlugins(o.labs, stage, pc.Repositories, plugins)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}

	m := &Manifest{Format: 2, Release: o.release, Date: o.date, Previous: o.previous, Sources: map[string]string{}, Packages: map[string]Package{}}
	mf := filepath.Join(o.out, "manifest.json")
	if b, err := os.ReadFile(mf); err == nil {
		if err := json.Unmarshal(b, m); err != nil {
			return fmt.Errorf("%s: %w", mf, err)
		}
		if m.Release != o.release || m.Format != 2 {
			return fmt.Errorf("%s gehört zu Stand %s (Format %d)", mf, m.Release, m.Format)
		}
	}
	prev := previousManifest(o, m)
	for _, r := range pc.Repositories {
		if out, err := exec.Command("git", "-C", filepath.Join(o.labs, r), "rev-parse", "HEAD").Output(); err == nil {
			m.Sources[r] = strings.TrimSpace(string(out))
		}
	}
	pk := &packer{out: o.out, prev: prev, m: m}

	// Programme und Plugins: je Datei ein Baustein
	for _, n := range programs {
		if err := pk.file(kindProgram, n, "", platform(), filepath.Join(stage, "programs", exe(n)), exe(n)); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(plugins))
	for n := range plugins {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		rel := pluginFiles[n]
		if err := pk.file(kindPlugin, n, plugins[n], platform(), filepath.Join(stage, rel), filepath.ToSlash(rel)); err != nil {
			return err
		}
	}
	// Dateien (configs, migrations, Inhalte, Lizenzen): ein Archiv für alle Plattformen
	if err := pk.files(relDir); err != nil {
		return err
	}
	// das Werkzeug selbst, damit es mit dem Stand geladen werden kann
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := pk.setup(self); err != nil {
		return err
	}
	logf("%d Bausteine, %d neu abgelegt, %d aus früheren Ständen", len(m.Components), pk.written, pk.reused)

	for id, p := range pc.Packages {
		pkg := Package{Title: p.Title, Description: p.Description, Plugins: p.Plugins}
		if old, ok := m.Packages[id]; ok {
			pkg.Data = old.Data
		}
		if p.Data != nil && o.data {
			db := filepath.Join(o.out, "data-"+id+"-"+o.release+".db.gz")
			if err := buildData(o, stage, filepath.Join(o.labs, p.Data.Seed), p.Data.User, p.Data.Password, db); err != nil {
				return fmt.Errorf("Datenbestand %s: %w", id, err)
			}
			a, err := artifact(db)
			if err != nil {
				return err
			}
			pkg.Data = &DataSet{Artifact: a, Title: p.Data.Title, User: p.Data.User, Password: p.Data.Password}
		}
		m.Packages[id] = pkg
	}
	b, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mf, append(b, '\n'), 0o644)
}

// previousManifest: Manifest des vorherigen Stands (--previous, sonst die Linie preview des
// Servers), dessen Dateien wiederverwendet werden. Ohne Server oder bei altem Format: keines.
func previousManifest(o packOptions, m *Manifest) *Manifest {
	if o.src == nil {
		return nil
	}
	ix, err := o.src.Index()
	if err != nil {
		logf("vorheriger Stand nicht lesbar (%v) – alle Bausteine werden abgelegt", err)
		return nil
	}
	id := o.previous
	if id == "" {
		id = ix.Lines["preview"]
	}
	if id == "" || id == o.release {
		return nil
	}
	prev, err := o.src.Manifest(ix, id)
	if err != nil {
		logf("vorheriger Stand %s: %v – alle Bausteine werden abgelegt", id, err)
		return nil
	}
	if m.Previous == "" {
		m.Previous = id
	}
	logf("vorheriger Stand %s: gleiche Bausteine verweisen dorthin", id)
	return prev
}

// packer legt die Bausteine eines Stands ab.
type packer struct {
	out             string
	prev, m         *Manifest
	written, reused int
}

// add trägt die Datei eines Bausteins für eine Plattform ein.
func (pk *packer) add(kind, name, version, plat string, a Artifact) {
	c := pk.m.component(kind, name)
	if c == nil {
		pk.m.Components = append(pk.m.Components, Component{Name: name, Kind: kind, Version: version, Platforms: map[string]Artifact{}})
		c = &pk.m.Components[len(pk.m.Components)-1]
	}
	if kind == kindPlugin || c.Version == "" {
		c.Version = version
	}
	c.Platforms[plat] = a
}

// reuse: Datei des vorherigen Stands, wenn der Baustein dort gleich ist – Plugins nach
// Version (die Version ist ihre Kennung), alles andere nach Inhalt.
func (pk *packer) reuse(kind, name, version, plat, content string) (Artifact, bool) {
	if pk.prev == nil {
		return Artifact{}, false
	}
	c := pk.prev.component(kind, name)
	if c == nil {
		return Artifact{}, false
	}
	a, ok := c.Platforms[plat]
	if !ok {
		return Artifact{}, false
	}
	if kind == kindPlugin {
		if c.Version != version {
			return Artifact{}, false
		}
		if a.Content != content {
			logf("%s %s: Inhalt anders als im Stand %s, Version gleich – die veröffentlichte Datei bleibt (Version erhöhen!)", name, version, pk.prev.Release)
		}
	} else if a.Content != content {
		return Artifact{}, false
	}
	if a.Release == "" {
		a.Release = pk.prev.Release
	}
	pk.reused++
	return a, true
}

// file: Programm oder Plugin, gzip-komprimiert abgelegt.
func (pk *packer) file(kind, name, version, plat, src, path string) error {
	content, err := fileSHA(src)
	if err != nil {
		return err
	}
	if version == "" {
		version = content[:12]
	}
	if a, ok := pk.reuse(kind, name, version, plat, content); ok {
		a.Path = path
		pk.add(kind, name, version, plat, a)
		return nil
	}
	dst := filepath.Join(pk.out, name+"-"+version+"-"+plat+".gz")
	if err := gzipFile(src, dst); err != nil {
		return err
	}
	a, err := artifact(dst)
	if err != nil {
		return err
	}
	a.Path, a.Content = path, content
	pk.written++
	pk.add(kind, name, version, plat, a)
	return nil
}

// files: alles außer den Programmen als ein Archiv (tar.gz, für alle Plattformen gleich).
func (pk *packer) files(dir string) error {
	tmp := filepath.Join(pk.out, "files.tar.gz.part")
	if err := writeArchive(dir, tmp); err != nil {
		return err
	}
	content, err := fileSHA(tmp)
	if err != nil {
		return err
	}
	version := content[:12]
	if a, ok := pk.reuse(kindFiles, "files", version, "any", content); ok {
		pk.add(kindFiles, "files", version, "any", a)
		return os.Remove(tmp)
	}
	dst := filepath.Join(pk.out, "files-"+version+".tar.gz")
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	a, err := artifact(dst)
	if err != nil {
		return err
	}
	a.Content = content
	pk.written++
	pk.add(kindFiles, "files", version, "any", a)
	return nil
}

// setup: das Werkzeug unkomprimiert (zum direkten Herunterladen von der Release-Seite).
func (pk *packer) setup(self string) error {
	dst := filepath.Join(pk.out, exe("coremesh-setup-"+platform()))
	if err := copyFile(self, dst); err != nil {
		return err
	}
	a, err := artifact(dst)
	if err != nil {
		return err
	}
	a.Path, a.Content = exe("coremesh-setup"), a.SHA256
	pk.written++
	pk.add(kindSetup, "coremesh-setup", a.SHA256[:12], platform(), a)
	return nil
}

// packConfigs übernimmt configs/*.yaml aller Repositories und liefert die Plugin-Versionen.
func packConfigs(labs, stage string, pc packConfig) (map[string]string, error) {
	var drop []*regexp.Regexp
	for _, s := range pc.Configs.DropLines {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, fmt.Errorf("drop_lines %q: %w", s, err)
		}
		drop = append(drop, re)
	}
	dir := filepath.Join(stage, "configs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	versions := map[string]string{}
	for _, r := range pc.Repositories {
		files, _ := filepath.Glob(filepath.Join(labs, r, "configs", "*.yaml"))
		for _, f := range files {
			if contains(pc.Configs.Exclude, filepath.Base(f)) {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			s := cleanConfig(string(b), drop)
			var doc struct {
				Plugins map[string]struct {
					Version string `yaml:"version"`
				} `yaml:"plugins"`
			}
			if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			for p, d := range doc.Plugins {
				if d.Version != "" {
					versions[p] = d.Version
				}
			}
			if err := os.WriteFile(filepath.Join(dir, filepath.Base(f)), []byte(s), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return versions, nil
}

// cleanConfig entfernt die Zeilen nach drop und danach leer gewordene Abschnitte der
// obersten Ebene (sonst überschriebe „host:“ ohne Inhalt die früheren Dateien).
func cleanConfig(s string, drop []*regexp.Regexp) string {
	var lines []string
next:
	for _, l := range strings.Split(s, "\n") {
		for _, re := range drop {
			if re.MatchString(l) {
				continue next
			}
		}
		lines = append(lines, l)
	}
	top := regexp.MustCompile(`^[A-Za-z_][\w-]*:\s*(#.*)?$`)
	var out []string
	for i, l := range lines {
		if top.MatchString(l) {
			empty := true
			for _, n := range lines[i+1:] {
				t := strings.TrimSpace(n)
				if t == "" || strings.HasPrefix(t, "#") {
					continue
				}
				empty = !strings.HasPrefix(n, " ") && !strings.HasPrefix(n, "\t")
				break
			}
			if empty {
				continue
			}
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// packPlugins kopiert genau die konfigurierten Versionen nach plugins/<xx>/ und liefert je
// Plugin den Pfad (plugins/<xx>/<datei>).
func packPlugins(labs, stage string, repos []string, want map[string]string) (map[string]string, error) {
	found := map[string]string{}
	for _, r := range repos {
		paths, _ := filepath.Glob(filepath.Join(labs, r, "bin", "plugins", "*", "*"))
		for _, p := range paths {
			for name, v := range want {
				if filepath.Base(p) == exe(name+"-"+v+"-"+platform()) {
					rel := filepath.Join("plugins", filepath.Base(filepath.Dir(p)), filepath.Base(p))
					if err := copyFile(p, filepath.Join(stage, rel)); err != nil {
						return nil, err
					}
					found[name] = rel
				}
			}
		}
	}
	var missing []string
	for name, v := range want {
		if found[name] == "" {
			missing = append(missing, name+" "+v)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("Plugins fehlen (bauen mit --build): %s", strings.Join(missing, ", "))
	}
	return found, nil
}

// buildData richtet aus der Bühne eine Installation ein, spielt die Einrichtungsdatei
// über die Konsole ein und legt die Datenbank komprimiert ab.
func buildData(o packOptions, stage, seedFile, user, password, out string) error {
	dir, err := os.MkdirTemp("", "coremesh-data-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	inst := &installation{dir: dir}
	for src, dst := range map[string]string{"release": inst.path("releases", o.release), "programs": inst.path("releases", o.release), "plugins": inst.path("plugins")} {
		if err := copyTree(filepath.Join(stage, src), dst); err != nil {
			return err
		}
	}
	if err := inst.configure(o.release, o.port, user, password); err != nil {
		return err
	}
	if err := inst.start(); err != nil {
		return err
	}
	seedErr := seed(inst, seedFile)
	if err := inst.stop(); err != nil {
		return err
	}
	if seedErr != nil {
		return seedErr
	}
	return gzipFile(filepath.Join(dir, "data", "coremesh.db"), out)
}

func fileSHA(path string) (string, error) {
	a, err := artifact(path)
	return a.SHA256, err
}

func artifact(path string) (Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return Artifact{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{File: filepath.Base(path), SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		out.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeArchive: tar.gz mit dem Inhalt von dir – ohne Zeiten und Besitzer, damit gleicher
// Inhalt dieselbe Prüfsumme ergibt (Wiederverwendung in späteren Ständen).
func writeArchive(dir, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		fi, err := d.Info()
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		h.Name, h.Uname, h.Gname, h.Uid, h.Gid, h.ModTime = filepath.ToSlash(rel), "", "", 0, 0, time.Unix(0, 0)
		h.AccessTime, h.ChangeTime = time.Time{}, time.Time{}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		_, err = io.Copy(tw, r)
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return f.Close()
}
