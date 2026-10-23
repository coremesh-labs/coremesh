package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// setupState: setup.json einer Installation – was das Werkzeug für Updates braucht.
type setupState struct {
	Line     string `json:"line"`               // release, preview oder ein fester Stand
	Package  string `json:"package"`            // Ausprägung
	Port     int    `json:"port"`               // Port der Weboberfläche
	Previous string `json:"previous,omitempty"` // Stand vor dem letzten Update (Rückfall)
	PrevLine string `json:"previous_line,omitempty"`
	Backup   string `json:"backup,omitempty"`   // Sicherung der Datenbank vor dem letzten Update
}

func (in *installation) state() setupState {
	var st setupState
	if b, err := os.ReadFile(in.path("setup.json")); err == nil {
		json.Unmarshal(b, &st)
	}
	if st.Port == 0 { // ältere Installationen: Port aus ZUGANG.txt
		addr, _, _ := in.access()
		if i := strings.LastIndex(addr, ":"); i >= 0 {
			st.Port, _ = strconv.Atoi(addr[i+1:])
		}
	}
	if st.Line == "" {
		st.Line = "release"
	}
	return st
}

func (in *installation) saveState(st setupState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(in.path("setup.json"), append(b, '\n'), 0o644)
}

// installed: Manifest des Stands in der Installation (nil bei Ständen vor den Bausteinen).
func (in *installation) installed(rel string) *Manifest {
	var m Manifest
	b, err := os.ReadFile(in.path("releases", rel, "manifest.json"))
	if err != nil || json.Unmarshal(b, &m) != nil {
		return nil
	}
	return &m
}

// Was mit einem Baustein geschieht
const (
	actPresent = "vorhanden"  // liegt schon da
	actCopy    = "übernehmen" // gleiche Datei aus einem anderen Stand der Installation
	actLoad    = "laden"
)

// planItem: ein Baustein im Plan einer Einrichtung oder eines Updates.
type planItem struct {
	Name, Kind   string
	Old, New     string // Versionen (Old leer = neu)
	Action       string
	Size         int64 // zu ladende Bytes
	art          Artifact
	target, from string
}

// plan: Bausteine des Stands und was zu tun ist.
type plan struct {
	Release, Current string
	Items            []planItem
	Data             *DataSet
}

func (p *plan) LoadSize() (n int64) {
	for _, it := range p.Items {
		n += it.Size
	}
	return
}

func (p *plan) Count(action string) (n int) {
	for _, it := range p.Items {
		if it.Action == action {
			n++
		}
	}
	return
}

// Changed: Bausteine mit anderer Version als im laufenden Stand.
func (p *plan) Changed() (out []planItem) {
	for _, it := range p.Items {
		if it.Old != it.New {
			out = append(out, it)
		}
	}
	return
}

// makePlan vergleicht die Bausteine des Stands rel mit der Installation.
func makePlan(m *Manifest, in *installation, pkg string, withData bool) (*plan, error) {
	comps, err := m.components(pkg)
	if err != nil {
		return nil, err
	}
	p := &plan{Release: m.Release, Current: in.current()}
	var old *Manifest
	var oldPlugins map[string]string // Stände ohne Manifest: Versionen aus der Konfiguration
	if p.Current != "" {
		if old = in.installed(p.Current); old == nil {
			oldPlugins, _ = readConfigs([]string{in.path("releases", p.Current, "configs"), in.path("local")})
		}
	}
	self, _ := os.Executable()
	for _, c := range comps {
		a, ok := c.artifactFor(platform())
		if !ok {
			return nil, fmt.Errorf("Stand %s: %s gibt es nicht für %s", m.Release, c.Name, platform())
		}
		it := planItem{Name: c.Name, Kind: c.Kind, New: c.Version, art: a, Action: actLoad}
		if old != nil {
			if oc := old.component(c.Kind, c.Name); oc != nil {
				it.Old = oc.Version
			}
		} else if c.Kind == kindPlugin {
			it.Old = oldPlugins[c.Name]
		}
		switch c.Kind {
		case kindPlugin:
			it.target = in.path(filepath.FromSlash(a.Path))
		case kindProgram:
			it.target = in.path("releases", m.Release, a.Path)
		case kindSetup:
			it.target = in.path(a.Path)
		case kindFiles:
			it.target = in.path("releases", m.Release)
		default:
			continue // unbekannte Art eines neueren Werkzeugs
		}
		switch {
		case c.Kind == kindFiles:
			if b, _ := os.ReadFile(filepath.Join(it.target, ".files")); strings.TrimSpace(string(b)) == a.SHA256 {
				it.Action = actPresent
			}
		case sameContent(it.target, a.Content):
			it.Action = actPresent
		case c.Kind == kindProgram:
			// gleiches Programm in einem anderen Stand dieser Installation
			others, _ := filepath.Glob(in.path("releases", "*", a.Path))
			for _, o := range others {
				if sameContent(o, a.Content) {
					it.Action, it.from = actCopy, o
					break
				}
			}
		case c.Kind == kindSetup && sameContent(self, a.Content):
			it.Action, it.from = actCopy, self
		}
		if it.Action == actLoad {
			it.Size = a.Size
		}
		p.Items = append(p.Items, it)
	}
	// Geänderte zuerst, dann nach Art
	sort.SliceStable(p.Items, func(i, j int) bool {
		a, b := p.Items[i], p.Items[j]
		if (a.Old != a.New) != (b.Old != b.New) {
			return a.Old != a.New
		}
		return kindOrder(a.Kind) < kindOrder(b.Kind)
	})
	if withData {
		if d := m.Packages[pkg].Data; d != nil {
			p.Data = d
		}
	}
	return p, nil
}

func kindOrder(k string) int {
	return strings.Index("program files plugin setup", k)
}

func sameContent(path, sum string) bool {
	if sum == "" {
		return false
	}
	ok, _ := checkSum(path, sum)
	return ok
}

// apply lädt bzw. übernimmt die fehlenden Bausteine.
func (p *plan) apply(src *source, ix *Index, in *installation, progress func(string)) error {
	for _, it := range p.Items {
		switch it.Action {
		case actPresent:
			continue
		case actCopy:
			progress(fmt.Sprintf("übernehme %s %s", it.Name, it.New))
			if err := replaceFile(it.from, it.target); err != nil {
				return err
			}
			continue
		}
		progress(fmt.Sprintf("lade %s %s (%s)", it.Name, it.New, sizeText(it.Size)))
		f, err := download(src, ix, p.Release, it.art, in.path("downloads"))
		if err != nil {
			return err
		}
		switch it.Kind {
		case kindFiles:
			if err := untar(f, it.target); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(it.target, ".files"), []byte(it.art.SHA256+"\n"), 0o644); err != nil {
				return err
			}
		case kindSetup:
			if err := replaceFile(f, it.target); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(it.target), 0o755); err != nil {
				return err
			}
			if err := gunzipFile(f, it.target+".part"); err != nil {
				return err
			}
			if ok, _ := checkSum(it.target+".part", it.art.Content); !ok {
				os.Remove(it.target + ".part")
				return fmt.Errorf("%s: Inhalt stimmt nicht mit dem Manifest überein", it.Name)
			}
			if err := os.Rename(it.target+".part", it.target); err != nil {
				return err
			}
		}
		if it.Kind == kindPlugin || it.Kind == kindProgram {
			if err := os.Chmod(it.target, 0o755); err != nil {
				return err
			}
		}
		os.Remove(f) // die entpackte Datei genügt
	}
	return nil
}

// replaceFile kopiert ausführbar über eine Zwischendatei – das Ziel darf gerade laufen
// (coremesh-setup aus dem Installationsverzeichnis). Unter Windows lässt sich ein laufendes
// Programm nicht ersetzen; dann bleibt das alte bis zum nächsten Update.
func replaceFile(src, dst string) error {
	if err := copyFile(src, dst+".new"); err != nil {
		return err
	}
	if err := os.Chmod(dst+".new", 0o755); err != nil {
		return err
	}
	if err := os.Rename(dst+".new", dst); err != nil {
		os.Remove(dst + ".new")
		if runtime.GOOS == "windows" {
			logf("%s läuft gerade und bleibt bis zum nächsten Update: %v", dst, err)
			return nil
		}
		return err
	}
	return nil
}

func sizeText(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

type installOptions struct {
	release, pkg string
	port         int
	withData     bool
}

// target: Stand, Manifest und Plan zu den Optionen.
func target(src *source, in *installation, o installOptions) (*Index, *Manifest, *plan, error) {
	ix, err := src.Index()
	if err != nil {
		return nil, nil, nil, err
	}
	rel, err := ix.resolve(o.release)
	if err != nil {
		return nil, nil, nil, err
	}
	m, err := src.Manifest(ix, rel)
	if err != nil {
		return nil, nil, nil, err
	}
	p, err := makePlan(m, in, o.pkg, o.withData)
	if err != nil {
		return nil, nil, nil, err
	}
	if o.withData && p.Data == nil {
		return nil, nil, nil, fmt.Errorf("Ausprägung %s hat keinen Testbestand", o.pkg)
	}
	return ix, m, p, nil
}

// install richtet einen Stand ein. In einer bestehenden Installation ist das ein Update.
// Mit Datenbestand nur in eine neue Installation; das Passwort des Bestands wird sofort ersetzt.
func install(src *source, in *installation, o installOptions, progress func(string)) error {
	if in.current() != "" {
		return update(src, in, o.release, progress)
	}
	if o.withData {
		if _, err := os.Stat(in.path("data", "coremesh.db")); err == nil {
			return errors.New("es gibt schon eine Datenbank – den Testbestand nur in ein neues Verzeichnis einrichten")
		}
	}
	ix, m, p, err := target(src, in, o)
	if err != nil {
		return err
	}
	remember(in.dir)
	if err := in.prepare(src, ix, m, p, progress); err != nil {
		return err
	}
	user, password := "admin", newPassword()
	if p.Data != nil {
		progress("lade " + p.Data.Title)
		f, err := download(src, ix, m.Release, p.Data.Artifact, in.path("downloads"))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(in.path("data"), 0o755); err != nil {
			return err
		}
		if err := gunzipFile(f, in.path("data", "coremesh.db")); err != nil {
			return err
		}
		user = p.Data.User
	}
	if err := in.configure(m.Release, o.port, user, password); err != nil {
		return err
	}
	if err := in.saveState(setupState{Line: o.release, Package: o.pkg, Port: o.port}); err != nil {
		return err
	}
	if p.Data != nil {
		// Zugang zuerst mit dem Passwort des Bestands, dann das eigene setzen
		addr, _, own := in.access()
		if err := in.writeAccess(addr, user, p.Data.Password); err != nil {
			return err
		}
		progress("starte")
		if err := in.start(); err != nil {
			return err
		}
		progress("setze eigenes Passwort")
		if out, err := in.console("handle", "Account.ChangePassword", "--current="+p.Data.Password, "--new="+own); err != nil {
			// ohne die Aufrufzeile: sie enthält das neue Passwort
			return fmt.Errorf("eigenes Passwort setzen: %s", strings.TrimSpace(string(out)))
		}
		return in.writeAccess(addr, user, own)
	}
	progress("starte")
	return in.start()
}

// prepare legt die Bausteine des Stands ab (der laufende Stand bleibt unberührt).
func (in *installation) prepare(src *source, ix *Index, m *Manifest, p *plan, progress func(string)) error {
	if err := os.MkdirAll(in.path("releases", m.Release), 0o755); err != nil {
		return err
	}
	progress(fmt.Sprintf("Stand %s: %d Bausteine, %d vorhanden, %d übernommen, %d zu laden (%s)",
		m.Release, len(p.Items), p.Count(actPresent), p.Count(actCopy), p.Count(actLoad), sizeText(p.LoadSize())))
	if err := p.apply(src, ix, in, progress); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(in.path("releases", m.Release, "manifest.json"), b, 0o644); err != nil {
		return err
	}
	return in.writeScripts()
}

// update wechselt eine Installation auf einen anderen Stand: Bausteine laden, solange der
// alte Stand noch läuft; dann beenden, Datenbank sichern, umschalten, starten. Startet der
// neue Stand nicht, geht es automatisch zurück (Stand und Datenbank).
func update(src *source, in *installation, want string, progress func(string)) error {
	st := in.state()
	if want == "" {
		want = st.Line
	}
	cur := in.current()
	if st.Package == "" {
		return fmt.Errorf("Ausprägung von %s unbekannt – zuerst über „Vorhandene hinzufügen“ zuordnen (coremesh-setup adopt --dir=… --package=…)", in.dir)
	}
	ix, m, p, err := target(src, in, installOptions{release: want, pkg: st.Package})
	if err != nil {
		return err
	}
	if m.Release == cur {
		return fmt.Errorf("die Installation ist schon auf Stand %s", cur)
	}
	if err := in.prepare(src, ix, m, p, progress); err != nil {
		return err
	}
	if in.running() {
		progress("beende Stand " + cur)
		if err := in.stop(); err != nil {
			return err
		}
	}
	progress("sichere die Datenbank")
	backup, err := in.backupDB(cur)
	if err != nil {
		return err
	}
	_, user, _ := in.access()
	if err := in.configure(m.Release, st.Port, user, ""); err != nil {
		return err
	}
	st.Previous, st.Backup, st.PrevLine = cur, backup, st.Line
	if isLine(ix, want) {
		st.Line = want
	} else {
		st.Line = m.Release
	}
	if err := in.saveState(st); err != nil {
		return err
	}
	progress("starte Stand " + m.Release + " (Datenbank wird angepasst)")
	if err := in.start(); err != nil {
		progress("Start fehlgeschlagen – zurück auf " + cur)
		if rbErr := rollback(in, progress); rbErr != nil {
			return fmt.Errorf("Update fehlgeschlagen (%v), Rückfall ebenfalls (%v)", err, rbErr)
		}
		return fmt.Errorf("Update auf %s fehlgeschlagen, wieder auf %s: %w", m.Release, cur, err)
	}
	return nil
}

func isLine(ix *Index, s string) bool {
	_, ok := ix.Lines[s]
	return ok
}

// rollback: zurück auf den Stand vor dem letzten Update, mit der Datenbank von damals.
// Änderungen seit dem Update gehen verloren.
func rollback(in *installation, progress func(string)) error {
	st := in.state()
	if st.Previous == "" || st.Backup == "" {
		return errors.New("kein vorheriger Stand gespeichert")
	}
	if _, err := os.Stat(in.path("releases", st.Previous)); err != nil {
		return fmt.Errorf("Stand %s ist nicht mehr da", st.Previous)
	}
	if err := in.stop(); err != nil {
		return err
	}
	progress("stelle die Datenbank vom " + backupTime(st.Backup) + " wieder her")
	if err := in.restoreDB(st.Backup); err != nil {
		return err
	}
	_, user, _ := in.access()
	if err := in.configure(st.Previous, st.Port, user, ""); err != nil {
		return err
	}
	if st.PrevLine != "" {
		st.Line = st.PrevLine
	}
	st.Previous, st.Backup, st.PrevLine = "", "", ""
	if err := in.saveState(st); err != nil {
		return err
	}
	progress("starte Stand " + in.current())
	return in.start()
}

var dbFiles = []string{"coremesh.db", "coremesh.db-wal"}

// backupDB kopiert die Datenbank (beendeter Host) nach data/setup-backup/.
func (in *installation) backupDB(rel string) (string, error) {
	name := fmt.Sprintf("coremesh-%s-%s", rel, nowStamp())
	dir := in.path("data", "setup-backup", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, f := range dbFiles {
		if _, err := os.Stat(in.path("data", f)); err == nil {
			if err := copyFile(in.path("data", f), filepath.Join(dir, f)); err != nil {
				return "", err
			}
		}
	}
	return filepath.Join("data", "setup-backup", name), nil
}

func (in *installation) restoreDB(backup string) error {
	dir := in.path(backup)
	if _, err := os.Stat(filepath.Join(dir, "coremesh.db")); err != nil {
		return fmt.Errorf("Sicherung %s fehlt", backup)
	}
	os.Remove(in.path("data", "coremesh.db-shm"))
	for _, f := range dbFiles {
		os.Remove(in.path("data", f))
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			if err := copyFile(filepath.Join(dir, f), in.path("data", f)); err != nil {
				return err
			}
		}
	}
	return nil
}

func nowStamp() string { return time.Now().Format("20060102-150405") }

var backupStamp = regexp.MustCompile(`(\d{8})-(\d{6})$`)

func backupTime(backup string) string {
	if m := backupStamp.FindStringSubmatch(backup); m != nil {
		if t, err := time.Parse("20060102150405", m[1]+m[2]); err == nil {
			return t.Format("02.01.2006 15:04")
		}
	}
	return backup
}

// writeScripts legt Skripte zum Starten, Beenden und für den Zustand neben die Installation;
// sie rufen das mitgelieferte coremesh-setup auf. Ohne Argumente startet coremesh-setup die
// Oberfläche (Update, Rückfall).
func (in *installation) writeScripts() error {
	if _, err := os.Stat(in.path(exe("coremesh-setup"))); err != nil {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		if err := replaceFile(self, in.path(exe("coremesh-setup"))); err != nil {
			return err
		}
	}
	scripts := map[string]string{}
	for _, c := range []struct{ name, what string }{{"start", "Startet"}, {"stop", "Beendet"}, {"status", "Zeigt den Zustand von"}} {
		if runtime.GOOS == "windows" {
			scripts[c.name+".cmd"] = fmt.Sprintf("@echo off\r\nrem %s CoreMesh in diesem Verzeichnis (coremesh-setup).\r\ncd /d \"%%~dp0\"\r\ncoremesh-setup.exe %s --dir=.\r\n", c.what, c.name)
		} else {
			scripts[c.name+".sh"] = fmt.Sprintf("#!/bin/sh\n# %s CoreMesh in diesem Verzeichnis (coremesh-setup).\ncd \"$(dirname \"$0\")\" && exec ./coremesh-setup %s --dir=.\n", c.what, c.name)
		}
	}
	for name, body := range scripts {
		if err := os.WriteFile(in.path(name), []byte(body), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// download lädt eine Datei (falls nicht schon vorhanden) und prüft die Prüfsumme.
func download(src *source, ix *Index, rel string, a Artifact, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, a.File)
	if ok, _ := checkSum(dst, a.SHA256); ok {
		return dst, nil
	}
	u, err := ix.artifactURL(rel, a)
	if err != nil {
		return "", err
	}
	r, err := src.open(u)
	if err != nil {
		return "", err
	}
	defer r.Close()
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if ok, err := checkSum(tmp, a.SHA256); !ok {
		os.Remove(tmp)
		if err == nil {
			err = fmt.Errorf("%s: Prüfsumme stimmt nicht", a.File)
		}
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}
