package main

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Arten von Installationen
const (
	layoutSetup  = "setup"        // von coremesh-setup eingerichtet (current, releases/, local/)
	layoutSingle = "eigenständig" // Programme und configs/ direkt im Verzeichnis (z. B. make_release)
)

// detected: was sich über ein Verzeichnis herausfinden lässt.
type detected struct {
	Layout  string
	Release string            // Stand (leer bei eigenständigen Builds)
	Format  int               // 2 = Bausteine, 1 = ganzes Archiv (nur Layout setup)
	Plugins map[string]string // Plugin → Version aus der Konfiguration
	Port    int               // Port der Weboberfläche
	Package string            // erkannte Ausprägung (leer = unbekannt)
}

// detect untersucht ein Verzeichnis; Fehler, wenn es keine CoreMesh-Installation ist.
func detect(dir string) (*detected, error) {
	in := &installation{dir: dir}
	d := &detected{}
	var configDirs []string
	switch {
	case in.current() != "":
		d.Layout, d.Release = layoutSetup, in.current()
		d.Format = 1
		if m := in.installed(d.Release); m != nil && m.Format == 2 {
			d.Format = 2
		}
		configDirs = []string{in.path("releases", d.Release, "configs"), in.path("local")}
	case exists(in.path(exe("host"))) && exists(in.path("configs")):
		d.Layout = layoutSingle
		configDirs = []string{in.path("configs")}
	default:
		return nil, fmt.Errorf("%s ist keine CoreMesh-Installation (weder current noch host und configs/)", dir)
	}
	d.Plugins, d.Port = readConfigs(configDirs)
	if len(d.Plugins) == 0 {
		return nil, fmt.Errorf("%s: keine Plugins in der Konfiguration", dir)
	}
	return d, nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readConfigs liest die Plugin-Versionen und den Port des Webservers aus den
// Konfigurationsdateien (spätere Dateien überschreiben frühere, wie beim Host).
func readConfigs(dirs []string) (map[string]string, int) {
	versions := map[string]string{}
	port := 0
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
		sort.Strings(files)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			var doc struct {
				Plugins map[string]struct {
					Version  string `yaml:"version"`
					Settings struct {
						Listen string `yaml:"listen"`
					} `yaml:"settings"`
				} `yaml:"plugins"`
			}
			if yaml.Unmarshal(b, &doc) != nil {
				continue
			}
			for name, p := range doc.Plugins {
				if p.Version != "" {
					versions[name] = p.Version
				}
				if name == "webserver" && p.Settings.Listen != "" {
					if i := strings.LastIndex(p.Settings.Listen, ":"); i >= 0 {
						if n, err := strconv.Atoi(p.Settings.Listen[i+1:]); err == nil {
							port = n
						}
					}
				}
			}
		}
	}
	return versions, port
}

// matchPackage: Ausprägung, deren Erkennungs-Plugins (Package.Detect) alle vorhanden sind;
// bei mehreren die mit den meisten. Ohne Treffer leer.
func matchPackage(m *Manifest, plugins map[string]string) string {
	best, n := "", 0
	ids := make([]string, 0, len(m.Packages))
	for id := range m.Packages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		det := m.Packages[id].Detect
		if len(det) == 0 || len(det) <= n {
			continue
		}
		all := true
		for _, p := range det {
			if _, ok := plugins[p]; !ok {
				all = false
				break
			}
		}
		if all {
			best, n = id, len(det)
		}
	}
	return best
}

// adopt nimmt eine vorhandene Installation in die Liste auf. Bei Installationen von
// coremesh-setup ergänzt es setup.json (Ausprägung, Port), damit Updates möglich sind.
func adopt(in *installation, pkg string) (*detected, error) {
	d, err := detect(in.dir)
	if err != nil {
		return nil, err
	}
	if d.Layout == layoutSetup {
		st := in.state()
		if pkg != "" {
			st.Package = pkg
		}
		if d.Port != 0 {
			st.Port = d.Port
		}
		if err := in.saveState(st); err != nil {
			return nil, err
		}
	}
	remember(in.dir)
	return d, nil
}

// Einträge, die coremesh-setup in einer Installation anlegt. Löschen entfernt nur diese.
var ownEntries = []string{"releases", "plugins", "local", "data", "downloads", "current", "setup.json", "ZUGANG.txt",
	"coremesh-setup", "coremesh-setup.exe", "coremesh-setup.new", "coremesh-setup.exe.new",
	"start.sh", "stop.sh", "status.sh", "start.cmd", "stop.cmd", "status.cmd"}

// removal: was „Löschen“ entfernen würde.
type removal struct {
	Size    int64    // Bytes der eigenen Einträge
	DBSize  int64    // Bytes der Datenbank
	Foreign []string // fremde Einträge (bleiben liegen)
}

func (in *installation) removal() (*removal, error) {
	if in.current() == "" {
		return nil, fmt.Errorf("%s wurde nicht von coremesh-setup eingerichtet – nur aus der Liste entfernen", in.dir)
	}
	entries, err := os.ReadDir(in.dir)
	if err != nil {
		return nil, err
	}
	r := &removal{}
	for _, e := range entries {
		if !contains(ownEntries, e.Name()) {
			r.Foreign = append(r.Foreign, e.Name())
			continue
		}
		r.Size += dirSize(in.path(e.Name()))
	}
	for _, f := range dbFiles {
		r.DBSize += dirSize(in.path("data", f))
	}
	return r, nil
}

func dirSize(p string) (n int64) {
	filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return
}

// remove beendet die Installation, sichert auf Wunsch die Datenbank nach backupDir und
// löscht die Einträge von coremesh-setup. Fremde Dateien bleiben samt Verzeichnis liegen.
// Liefert den Pfad der Sicherung.
func remove(in *installation, backupDir string, progress func(string)) (string, error) {
	r, err := in.removal()
	if err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	if clean := filepath.Clean(in.dir); clean == "/" || clean == filepath.Clean(home) {
		return "", fmt.Errorf("%s wird nicht gelöscht", in.dir)
	}
	if in.running() {
		progress("beende")
		if err := in.stop(); err != nil {
			return "", err
		}
	}
	saved := ""
	if backupDir != "" && r.DBSize > 0 {
		saved = filepath.Join(backupDir, fmt.Sprintf("%s-%s-%s.db", filepath.Base(in.dir), in.current(), nowStamp()))
		progress("sichere die Datenbank nach " + saved)
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return "", err
		}
		if err := copyFile(in.path("data", "coremesh.db"), saved); err != nil {
			return "", err
		}
		if exists(in.path("data", "coremesh.db-wal")) {
			if err := copyFile(in.path("data", "coremesh.db-wal"), saved+"-wal"); err != nil {
				return "", err
			}
		}
	}
	progress(fmt.Sprintf("lösche %s (%s)", in.dir, sizeText(r.Size)))
	for _, e := range ownEntries {
		if err := os.RemoveAll(in.path(e)); err != nil {
			return saved, err
		}
	}
	if len(r.Foreign) == 0 {
		os.Remove(in.dir)
	} else {
		progress("fremde Dateien bleiben: " + strings.Join(r.Foreign, ", "))
	}
	forget(in.dir)
	return saved, nil
}

// listening: antwortet die Konsole der Installation (auch ohne PID-Datei, z. B. eigenständig)?
func listening(sock string) bool {
	c, err := net.Dial("unix", sock)
	if err != nil {
		return false
	}
	c.Close()
	return true
}
