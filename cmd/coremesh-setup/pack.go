package main

import (
	"archive/tar"
	"archive/zip"
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
}

func platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// pack stellt einen Stand für die eigene Plattform zusammen: Archiv, Manifest und
// (mit data) je Ausprägung den Datenbestand, eingespielt über die Fachobjekte.
// Ein vorhandenes manifest.json im Zielverzeichnis wird um die Plattform ergänzt.
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
	stage, err := os.MkdirTemp("", "coremesh-pack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	for r, names := range pc.Binaries {
		for _, n := range names {
			if err := copyFile(filepath.Join(o.labs, r, "bin", exe(n)), filepath.Join(stage, exe(n))); err != nil {
				return err
			}
		}
	}
	for dst, src := range pc.Copy {
		if err := copyTree(filepath.Join(o.labs, src), filepath.Join(stage, dst)); err != nil {
			return err
		}
	}
	plugins, err := packConfigs(o.labs, stage, pc)
	if err != nil {
		return err
	}
	if err := packPlugins(o.labs, stage, pc.Repositories, plugins); err != nil {
		return err
	}

	m := &Manifest{Release: o.release, Date: o.date, Previous: o.previous, Sources: map[string]string{},
		Plugins: plugins, Platforms: map[string]Artifact{}, Setup: map[string]Artifact{}, Packages: map[string]Package{}}
	mf := filepath.Join(o.out, "manifest.json")
	if b, err := os.ReadFile(mf); err == nil {
		if err := json.Unmarshal(b, m); err != nil {
			return fmt.Errorf("%s: %w", mf, err)
		}
		if m.Release != o.release {
			return fmt.Errorf("%s gehört zu Stand %s", mf, m.Release)
		}
	}
	for _, r := range pc.Repositories {
		if out, err := exec.Command("git", "-C", filepath.Join(o.labs, r), "rev-parse", "HEAD").Output(); err == nil {
			m.Sources[r] = strings.TrimSpace(string(out))
		}
	}
	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return err
	}
	name := "coremesh-" + o.release + "-" + platform()
	archive := filepath.Join(o.out, name+".tar.gz")
	if runtime.GOOS == "windows" {
		archive = filepath.Join(o.out, name+".zip")
	}
	if err := writeArchive(stage, archive); err != nil {
		return err
	}
	if m.Platforms[platform()], err = artifact(archive); err != nil {
		return err
	}
	// das Werkzeug selbst, damit es mit dem Stand geladen werden kann
	self, err := os.Executable()
	if err != nil {
		return err
	}
	setupFile := filepath.Join(o.out, exe("coremesh-setup-"+platform()))
	if err := copyFile(self, setupFile); err != nil {
		return err
	}
	if m.Setup[platform()], err = artifact(setupFile); err != nil {
		return err
	}
	logf("Archiv %s (%d Plugins)", filepath.Base(archive), len(plugins))

	for id, p := range pc.Packages {
		pkg := Package{Title: p.Title, Description: p.Description, Plugins: p.Plugins}
		if old, ok := m.Packages[id]; ok {
			pkg.Data = old.Data
		}
		if p.Data != nil && o.data {
			db := filepath.Join(o.out, "data-"+id+"-"+o.release+".db.gz")
			if err := buildData(o, archive, filepath.Join(o.labs, p.Data.Seed), p.Data.User, p.Data.Password, db); err != nil {
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

// packPlugins kopiert genau die konfigurierten Versionen nach plugins/<xx>/.
func packPlugins(labs, stage string, repos []string, want map[string]string) error {
	found := map[string]bool{}
	for _, r := range repos {
		paths, _ := filepath.Glob(filepath.Join(labs, r, "bin", "plugins", "*", "*"))
		for _, p := range paths {
			for name, v := range want {
				if filepath.Base(p) == exe(name+"-"+v+"-"+platform()) {
					if err := copyFile(p, filepath.Join(stage, "plugins", filepath.Base(filepath.Dir(p)), filepath.Base(p))); err != nil {
						return err
					}
					found[name] = true
				}
			}
		}
	}
	var missing []string
	for name, v := range want {
		if !found[name] {
			missing = append(missing, name+" "+v)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Plugins fehlen (bauen mit --build): %s", strings.Join(missing, ", "))
	}
	return nil
}

// buildData richtet aus dem Archiv eine Installation ein, spielt die Einrichtungsdatei
// über die Konsole ein und legt die Datenbank komprimiert ab.
func buildData(o packOptions, archive, seedFile, user, password, out string) error {
	dir, err := os.MkdirTemp("", "coremesh-data-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	inst := &installation{dir: dir}
	if err := inst.unpack(archive, o.release); err != nil {
		return err
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

// writeArchive: tar.gz bzw. zip (Windows) mit dem Inhalt von dir.
func writeArchive(dir, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var add func(rel string, fi fs.FileInfo, r io.Reader) error
	var finish func() error
	if strings.HasSuffix(path, ".zip") {
		zw := zip.NewWriter(f)
		add = func(rel string, fi fs.FileInfo, r io.Reader) error {
			h, err := zip.FileInfoHeader(fi)
			if err != nil {
				return err
			}
			h.Name, h.Method = rel, zip.Deflate
			w, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, r)
			return err
		}
		finish = zw.Close
	} else {
		gz := gzip.NewWriter(f)
		tw := tar.NewWriter(gz)
		add = func(rel string, fi fs.FileInfo, r io.Reader) error {
			h, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			h.Name, h.Uname, h.Gname, h.Uid, h.Gid, h.ModTime = rel, "", "", 0, 0, time.Unix(0, 0)
			if err := tw.WriteHeader(h); err != nil {
				return err
			}
			_, err = io.Copy(tw, r)
			return err
		}
		finish = func() error {
			if err := tw.Close(); err != nil {
				return err
			}
			return gz.Close()
		}
	}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		fi, err := d.Info()
		if err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		return add(filepath.ToSlash(rel), fi, r)
	})
	if err != nil {
		return err
	}
	return finish()
}
