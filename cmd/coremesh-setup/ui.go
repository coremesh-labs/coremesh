package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed ui.html
var uiFS embed.FS

// Bekannte Installationen dieses Benutzers (für die Übersicht der Oberfläche).
func registryPath() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "coremesh-setup", "installations.json")
}

func loadRegistry() []string {
	var dirs []string
	if b, err := os.ReadFile(registryPath()); err == nil {
		json.Unmarshal(b, &dirs)
	}
	return dirs
}

func remember(dir string) {
	dirs := loadRegistry()
	for _, d := range dirs {
		if d == dir {
			return
		}
	}
	dirs = append(dirs, dir)
	sort.Strings(dirs)
	b, _ := json.MarshalIndent(dirs, "", "  ")
	os.MkdirAll(filepath.Dir(registryPath()), 0o755)
	os.WriteFile(registryPath(), b, 0o644)
}

func forget(dir string) {
	var keep []string
	for _, d := range loadRegistry() {
		if d != dir {
			keep = append(keep, d)
		}
	}
	b, _ := json.MarshalIndent(keep, "", "  ")
	os.WriteFile(registryPath(), b, 0o644)
}

func known(dir string) bool {
	for _, d := range loadRegistry() {
		if d == dir {
			return true
		}
	}
	return false
}

// job: laufende Einrichtung, Aktualisierung oder Rückfall mit Fortschritt.
type job struct {
	mu    sync.Mutex
	title string
	dir   string
	lines []string
	done  bool
	err   error
}

func (j *job) Title() string { return j.title }

func (j *job) add(s string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, time.Now().Format("15:04:05")+"  "+s)
}

type uiInstall struct {
	Dir, Release, Line, Address, User string
	Layout                            string // setup | eigenständig
	Format                            int
	Package, PackageTitle             string
	Running                           bool
	UpdateTo                          string // neuerer Stand auf der Linie der Installation
	Previous                          string // Rückfall möglich auf
	BackupTime                        string
}

type uiLine struct{ Name, Release string }

// uiStep: Schritt des Assistenten in der Kopfzeile.
type uiStep struct {
	N             int
	Title         string
	Current, Done bool
}

// uiPage: Daten aller Seiten; Page wählt die Seite.
type uiPage struct {
	Page     string
	Index    string
	Platform string
	Error    string
	Steps    []uiStep
	Step     int

	Lines    []uiLine
	Releases []ReleaseRef
	Packages []PackageRef
	Installs []uiInstall

	// Assistent: gewählte Werte (wandern als versteckte Felder von Seite zu Seite)
	Dir, Release, Package string
	Port                  int
	Data                  bool
	DataTitle             string
	Plan                  *plan
	Inst                  *uiInstall
	ReleaseRef            *ReleaseRef
	PackageRef            *PackageRef

	Detected   *detected
	PluginList []string
	Removal    *removal
	BackupDir  string
	Confirm    string

	Job      *job
	JobLines []string
	JobDone  bool
	JobErr   string
	Address  string
}

var newSteps = []string{"Verzeichnis", "Stand", "Ausprägung", "Übersicht"}
var updateSteps = []string{"Stand", "Übersicht"}
var adoptSteps = []string{"Verzeichnis", "Prüfen"}

func steps(titles []string, cur int) []uiStep {
	var out []uiStep
	for i, t := range titles {
		out = append(out, uiStep{N: i + 1, Title: t, Current: i+1 == cur, Done: i+1 < cur})
	}
	return out
}

func serveUI(listen string, src *source) error {
	tpl := template.Must(template.New("ui.html").Funcs(template.FuncMap{
		"size": sizeText,
		"query": func(p uiPage, step int) template.URL {
			v := url.Values{"step": {strconv.Itoa(step)}, "dir": {p.Dir}, "port": {strconv.Itoa(p.Port)}, "release": {p.Release}, "package": {p.Package}}
			if p.Data {
				v.Set("data", "on")
			}
			return template.URL(v.Encode())
		},
	}).ParseFS(uiFS, "ui.html"))
	var cur *job
	var mu sync.Mutex

	allowed := func(r *http.Request) bool {
		// nur Anfragen an genau diese Adresse (keine fremden Seiten, kein DNS-Rebinding)
		if r.Host != listen && r.Host != strings.Replace(listen, "127.0.0.1", "localhost", 1) {
			return false
		}
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			return false
		}
		return true
	}
	base := func() (uiPage, *Index) {
		p := uiPage{Index: src.index, Platform: platform()}
		ix, err := src.Index()
		if err != nil {
			p.Error = "Server nicht erreichbar: " + err.Error()
			return p, nil
		}
		p.Releases, p.Packages = ix.Releases, ix.Packages
		for l, id := range ix.Lines {
			p.Lines = append(p.Lines, uiLine{l, id})
		}
		// „release“ zuerst, dann alphabetisch; Stände neueste zuerst
		sort.Slice(p.Lines, func(i, j int) bool {
			a, b := p.Lines[i].Name, p.Lines[j].Name
			return a == "release" || (b != "release" && a < b)
		})
		sort.Slice(p.Releases, func(i, j int) bool { return p.Releases[i].ID > p.Releases[j].ID })
		return p, ix
	}
	// latest: Manifest des Release-Stands (Erkennung der Ausprägung); nil ohne Server
	latest := func(ix *Index) *Manifest {
		if ix == nil {
			return nil
		}
		m, err := src.Manifest(ix, ix.Lines["release"])
		if err != nil {
			return nil
		}
		return m
	}
	packageTitle := func(ix *Index, id string) string {
		if ix != nil {
			for _, p := range ix.Packages {
				if p.ID == id {
					return p.Title
				}
			}
		}
		return id
	}
	installInfo := func(ix *Index, lm *Manifest, d string) *uiInstall {
		in := &installation{dir: d}
		det, err := detect(d)
		if err != nil {
			return &uiInstall{Dir: d, Layout: "fehlt", Release: err.Error()}
		}
		rel := in.current()
		st := in.state()
		addr, user, _ := in.access()
		if addr == "" && det.Port != 0 {
			addr = fmt.Sprintf("http://localhost:%d", det.Port)
		}
		u := &uiInstall{Dir: d, Release: rel, Line: st.Line, Address: addr, User: user, Running: in.running(), Previous: st.Previous,
			Layout: det.Layout, Format: det.Format, Package: st.Package}
		if det.Layout == layoutSingle {
			u.Line, u.Release = "", "eigener Build"
		}
		if u.Package == "" && lm != nil {
			u.Package = matchPackage(lm, det.Plugins)
		}
		if u.Package != "" {
			u.PackageTitle = packageTitle(ix, u.Package)
		}
		if st.Backup != "" {
			u.BackupTime = backupTime(st.Backup)
		}
		if ix != nil && det.Layout == layoutSetup {
			if id, ok := ix.Lines[st.Line]; ok && id != rel {
				u.UpdateTo = id
			}
		}
		return u
	}
	show := func(w http.ResponseWriter, p uiPage) {
		mu.Lock()
		if cur != nil {
			cur.mu.Lock()
			p.Job, p.JobLines, p.JobDone = cur, append([]string(nil), cur.lines...), cur.done
			if cur.err != nil {
				p.JobErr = cur.err.Error()
			}
			cur.mu.Unlock()
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tpl.Execute(w, p); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !allowed(r) {
				http.Error(w, "nicht erlaubt", http.StatusForbidden)
				return
			}
			h(w, r)
		}
	}
	busy := func() bool {
		mu.Lock()
		defer mu.Unlock()
		if cur == nil {
			return false
		}
		cur.mu.Lock()
		defer cur.mu.Unlock()
		return !cur.done
	}
	run := func(title, dir string, f func(progress func(string)) error) {
		j := &job{title: title, dir: dir}
		mu.Lock()
		cur = j
		mu.Unlock()
		go func() {
			err := f(j.add)
			if err == nil {
				j.add("fertig")
			}
			j.mu.Lock()
			j.done, j.err = true, err
			j.mu.Unlock()
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page = "home"
		lm := latest(ix)
		for _, d := range loadRegistry() {
			p.Installs = append(p.Installs, *installInfo(ix, lm, d))
		}
		p.Error = strings.TrimSpace(p.Error + " " + r.URL.Query().Get("error"))
		show(w, p)
	}))

	// Neue Installation: 1 Verzeichnis und Port, 2 Stand, 3 Ausprägung, 4 Übersicht
	mux.HandleFunc("GET /new", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page = "new"
		q := r.URL.Query()
		home, _ := os.UserHomeDir()
		p.Dir, p.Release, p.Package, p.Data = q.Get("dir"), q.Get("release"), q.Get("package"), q.Get("data") == "on"
		p.Port, _ = strconv.Atoi(q.Get("port"))
		if p.Dir == "" {
			p.Dir = filepath.Join(home, "coremesh")
		}
		if p.Port == 0 {
			p.Port = freePort(8080)
		}
		if p.Release == "" {
			p.Release = "release"
		}
		if p.Package == "" && len(p.Packages) > 0 {
			p.Package = p.Packages[0].ID
		}
		p.Step, _ = strconv.Atoi(q.Get("step"))
		if p.Step < 1 || p.Step > len(newSteps) || ix == nil {
			p.Step = 1
		}
		// jeder Schritt prüft die Angaben der vorigen
		if p.Step > 1 {
			if msg := checkNewDir(p.Dir, p.Port); msg != "" {
				p.Step, p.Error = 1, msg
			}
		}
		if p.Step > 2 {
			if _, err := ix.resolve(p.Release); err != nil {
				p.Step, p.Error = 2, err.Error()
			}
		}
		if p.Step == 4 {
			abs, _ := filepath.Abs(p.Dir)
			_, m, pl, err := target(src, &installation{dir: abs}, installOptions{release: p.Release, pkg: p.Package, withData: p.Data})
			if err != nil {
				p.Step, p.Error = 3, err.Error()
			} else {
				p.Plan, p.Dir = pl, abs
				if d := m.Packages[p.Package].Data; d != nil {
					p.DataTitle = d.Title
				}
			}
		}
		if p.Step == 3 && ix != nil {
			// Titel des Testbestands aus dem Manifest des gewählten Stands
			if rel, err := ix.resolve(p.Release); err == nil {
				if m, err := src.Manifest(ix, rel); err == nil {
					if pk, ok := m.Packages[p.Package]; ok && pk.Data != nil {
						p.DataTitle = pk.Data.Title
					}
				} else {
					p.Error = err.Error()
				}
			}
		}
		for i := range p.Packages {
			if p.Packages[i].ID == p.Package {
				p.PackageRef = &p.Packages[i]
			}
		}
		p.Steps = steps(newSteps, p.Step)
		show(w, p)
	}))
	mux.HandleFunc("POST /new", guard(func(w http.ResponseWriter, r *http.Request) {
		dir, _ := filepath.Abs(strings.TrimSpace(r.FormValue("dir")))
		port, _ := strconv.Atoi(r.FormValue("port"))
		if msg := checkNewDir(dir, port); msg != "" {
			http.Redirect(w, r, "/?error="+url.QueryEscape(msg), http.StatusSeeOther)
			return
		}
		if busy() {
			http.Redirect(w, r, "/job", http.StatusSeeOther)
			return
		}
		o := installOptions{release: r.FormValue("release"), pkg: r.FormValue("package"), port: port, withData: r.FormValue("data") == "on"}
		in := &installation{dir: dir}
		run("Einrichtung "+dir, dir, func(progress func(string)) error { return install(src, in, o, progress) })
		http.Redirect(w, r, "/job", http.StatusSeeOther)
	}))

	// Update: 1 Stand, 2 Übersicht (was sich ändert, was zu laden ist)
	mux.HandleFunc("GET /update", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page = "update"
		q := r.URL.Query()
		p.Dir = q.Get("dir")
		if !known(p.Dir) || ix == nil {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		in := &installation{dir: p.Dir}
		p.Inst = installInfo(ix, nil, p.Dir)
		st := in.state()
		if st.Package == "" {
			http.Redirect(w, r, "/adopt?step=2&dir="+url.QueryEscape(p.Dir), http.StatusSeeOther)
			return
		}
		p.Release, p.Package, p.Port = q.Get("release"), st.Package, st.Port
		if p.Release == "" {
			p.Release = st.Line
		}
		p.Step, _ = strconv.Atoi(q.Get("step"))
		if p.Step != 2 {
			p.Step = 1
		}
		if p.Step == 2 {
			_, m, pl, err := target(src, in, installOptions{release: p.Release, pkg: st.Package})
			switch {
			case err != nil:
				p.Step, p.Error = 1, err.Error()
			case m.Release == in.current():
				p.Step, p.Error = 1, "Die Installation ist schon auf Stand "+m.Release+"."
			default:
				p.Plan, p.ReleaseRef = pl, ix.Release(m.Release)
			}
		}
		p.Steps = steps(updateSteps, p.Step)
		show(w, p)
	}))
	mux.HandleFunc("POST /update", guard(func(w http.ResponseWriter, r *http.Request) {
		dir := r.FormValue("dir")
		if !known(dir) {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		if busy() {
			http.Redirect(w, r, "/job", http.StatusSeeOther)
			return
		}
		rel := r.FormValue("release")
		in := &installation{dir: dir}
		run("Update "+dir, dir, func(progress func(string)) error { return update(src, in, rel, progress) })
		http.Redirect(w, r, "/job", http.StatusSeeOther)
	}))

	mux.HandleFunc("GET /rollback", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page, p.Dir = "rollback", r.URL.Query().Get("dir")
		if !known(p.Dir) {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		p.Inst = installInfo(ix, nil, p.Dir)
		if p.Inst.Previous == "" {
			http.Redirect(w, r, "/?error="+url.QueryEscape("kein vorheriger Stand gespeichert"), http.StatusSeeOther)
			return
		}
		show(w, p)
	}))
	mux.HandleFunc("POST /rollback", guard(func(w http.ResponseWriter, r *http.Request) {
		dir := r.FormValue("dir")
		if !known(dir) || busy() {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		in := &installation{dir: dir}
		run("Rückfall "+dir, dir, func(progress func(string)) error { return rollback(in, progress) })
		http.Redirect(w, r, "/job", http.StatusSeeOther)
	}))

	// Vorhandene Installation hinzufügen: 1 Verzeichnis, 2 Erkanntes prüfen, Ausprägung wählen
	mux.HandleFunc("GET /adopt", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page = "adopt"
		q := r.URL.Query()
		p.Dir, p.Package = strings.TrimSpace(q.Get("dir")), q.Get("package")
		p.Step, _ = strconv.Atoi(q.Get("step"))
		if p.Step != 2 || p.Dir == "" {
			p.Step = 1
		}
		if p.Step == 2 {
			abs, _ := filepath.Abs(p.Dir)
			det, err := detect(abs)
			if err != nil {
				p.Step, p.Error = 1, err.Error()
			} else {
				p.Dir, p.Detected = abs, det
				if p.Package == "" {
					p.Package = (&installation{dir: abs}).state().Package
				}
				if lm := latest(ix); p.Package == "" && lm != nil {
					p.Package = matchPackage(lm, det.Plugins)
				}
				for n, v := range det.Plugins {
					p.PluginList = append(p.PluginList, n+" "+v)
				}
				sort.Strings(p.PluginList)
			}
		}
		p.Steps = steps(adoptSteps, p.Step)
		show(w, p)
	}))
	mux.HandleFunc("POST /adopt", guard(func(w http.ResponseWriter, r *http.Request) {
		dir, _ := filepath.Abs(strings.TrimSpace(r.FormValue("dir")))
		if _, err := adopt(&installation{dir: dir}, r.FormValue("package")); err != nil {
			http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}))

	// Entfernen: aus der Liste (Dateien bleiben) oder löschen (mit Sicherung der Datenbank)
	mux.HandleFunc("GET /remove", guard(func(w http.ResponseWriter, r *http.Request) {
		p, ix := base()
		p.Page, p.Dir = "remove", r.URL.Query().Get("dir")
		p.Error = strings.TrimSpace(p.Error + " " + r.URL.Query().Get("error"))
		if !known(p.Dir) {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		p.Inst = installInfo(ix, nil, p.Dir)
		p.Removal, _ = (&installation{dir: p.Dir}).removal()
		home, _ := os.UserHomeDir()
		p.BackupDir, p.Confirm = filepath.Join(home, "coremesh-sicherungen"), filepath.Base(p.Dir)
		show(w, p)
	}))
	mux.HandleFunc("POST /remove", guard(func(w http.ResponseWriter, r *http.Request) {
		dir := r.FormValue("dir")
		if !known(dir) {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		if r.FormValue("how") != "delete" {
			forget(dir)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if strings.TrimSpace(r.FormValue("confirm")) != filepath.Base(dir) {
			http.Redirect(w, r, "/remove?dir="+url.QueryEscape(dir)+"&error="+url.QueryEscape("Zum Löschen den Verzeichnisnamen eintippen"), http.StatusSeeOther)
			return
		}
		if busy() {
			http.Redirect(w, r, "/job", http.StatusSeeOther)
			return
		}
		backupDir := ""
		if r.FormValue("backup") == "on" {
			backupDir = strings.TrimSpace(r.FormValue("backup_dir"))
		}
		in := &installation{dir: dir}
		run("Löschen "+dir, "", func(progress func(string)) error {
			_, err := remove(in, backupDir, progress)
			return err
		})
		http.Redirect(w, r, "/job", http.StatusSeeOther)
	}))

	mux.HandleFunc("GET /job", guard(func(w http.ResponseWriter, r *http.Request) {
		p, _ := base()
		p.Page = "job"
		mu.Lock()
		j := cur
		mu.Unlock()
		if j == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if j.dir != "" {
			p.Address, _, _ = (&installation{dir: j.dir}).access()
		}
		show(w, p)
	}))

	mux.HandleFunc("POST /control", guard(func(w http.ResponseWriter, r *http.Request) {
		dir := r.FormValue("dir")
		if !known(dir) {
			http.Redirect(w, r, "/?error="+url.QueryEscape("unbekannte Installation"), http.StatusSeeOther)
			return
		}
		in := &installation{dir: dir}
		var err error
		switch r.FormValue("do") {
		case "start":
			err = in.start()
		case "stop":
			err = in.stop()
		}
		if err != nil {
			http.Redirect(w, r, "/?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}))

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	logf("coremesh-setup: http://%s", listen)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

// checkNewDir: Verzeichnis für eine neue Installation und freier Port.
func checkNewDir(dir string, port int) string {
	if strings.TrimSpace(dir) == "" {
		return "Bitte ein Verzeichnis angeben."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err.Error()
	}
	if (&installation{dir: abs}).current() != "" {
		return "In " + abs + " ist schon eine Installation – dort über „Aktualisieren“."
	}
	if port < 1 || port > 65535 {
		return "Bitte einen Port zwischen 1 und 65535 angeben."
	}
	if !portFree(port) {
		return fmt.Sprintf("Port %d ist belegt – bitte einen anderen wählen.", port)
	}
	return ""
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	// auch nicht von einer beendeten Installation belegt
	for _, d := range loadRegistry() {
		if (&installation{dir: d}).state().Port == port {
			return false
		}
	}
	return true
}

func freePort(from int) int {
	for p := from; p < from+100; p++ {
		if portFree(p) {
			return p
		}
	}
	return from
}
