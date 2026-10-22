package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"net"
	"net/http"
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

// job: laufende Einrichtung mit Fortschritt.
type job struct {
	mu    sync.Mutex
	dir   string
	lines []string
	done  bool
	err   error
}

func (j *job) add(s string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, time.Now().Format("15:04:05")+"  "+s)
}

type uiInstall struct {
	Dir, Release, Address, User, State string
	Running                            bool
}

type uiLine struct{ Name, Release string }

type uiPage struct {
	Index       string
	Platform    string
	Error       string
	Lines       []uiLine
	Releases    []ReleaseRef
	Packages    []PackageRef
	Installs    []uiInstall
	Job         *job
	JobDir      string
	JobLines    []string
	JobDone     bool
	JobErr      string
	DefaultDir  string
	DefaultPort int
}

func serveUI(listen string, src *source) error {
	tpl := template.Must(template.ParseFS(uiFS, "ui.html"))
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
	render := func(w http.ResponseWriter, msg string) {
		p := uiPage{Index: src.index, Platform: platform(), Error: msg, DefaultPort: 8080}
		home, _ := os.UserHomeDir()
		p.DefaultDir = filepath.Join(home, "coremesh")
		if ix, err := src.Index(); err != nil {
			p.Error = strings.TrimSpace(p.Error + " Server: " + err.Error())
		} else {
			p.Releases, p.Packages = ix.Releases, ix.Packages
			for l, id := range ix.Lines {
				p.Lines = append(p.Lines, uiLine{l, id})
			}
			// „release“ zuerst, dann alphabetisch
			sort.Slice(p.Lines, func(i, j int) bool {
				a, b := p.Lines[i].Name, p.Lines[j].Name
				return a == "release" || (b != "release" && a < b)
			})
		}
		for _, d := range loadRegistry() {
			in := &installation{dir: d}
			addr, user, _ := in.access()
			u := uiInstall{Dir: d, Release: in.current(), Address: addr, User: user, Running: in.running()}
			if u.Release == "" {
				continue
			}
			p.Installs = append(p.Installs, u)
		}
		mu.Lock()
		if cur != nil {
			cur.mu.Lock()
			p.Job, p.JobDir, p.JobLines, p.JobDone = cur, cur.dir, append([]string(nil), cur.lines...), cur.done
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			http.Error(w, "nicht erlaubt", http.StatusForbidden)
			return
		}
		render(w, "")
	})
	mux.HandleFunc("POST /install", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			http.Error(w, "nicht erlaubt", http.StatusForbidden)
			return
		}
		dir, err := filepath.Abs(strings.TrimSpace(r.FormValue("dir")))
		port, _ := strconv.Atoi(r.FormValue("port"))
		if err != nil || r.FormValue("dir") == "" || port <= 0 {
			render(w, "Verzeichnis und Port angeben")
			return
		}
		mu.Lock()
		if cur != nil && !cur.done {
			mu.Unlock()
			render(w, "Es läuft schon eine Einrichtung")
			return
		}
		j := &job{dir: dir}
		cur = j
		mu.Unlock()
		o := installOptions{release: r.FormValue("release"), pkg: r.FormValue("package"), port: port, withData: r.FormValue("data") == "on"}
		go func() {
			err := install(src, &installation{dir: dir}, o, j.add)
			j.mu.Lock()
			j.done, j.err = true, err
			j.mu.Unlock()
			if err == nil {
				j.add("fertig")
			}
		}()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /control", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r) {
			http.Error(w, "nicht erlaubt", http.StatusForbidden)
			return
		}
		dir := r.FormValue("dir")
		known := false
		for _, d := range loadRegistry() {
			known = known || d == dir
		}
		if !known {
			render(w, "unbekannte Installation")
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
			render(w, err.Error())
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	logf("coremesh-setup: http://%s", listen)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}
