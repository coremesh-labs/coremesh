// Command coremesh-setup richtet CoreMesh-Systeme ein und stellt Stände zusammen.
//
//	coremesh-setup [ui]        Weboberfläche auf 127.0.0.1 (Stand wählen, einrichten, starten)
//	coremesh-setup list        Linien, Stände und Ausprägungen des Servers
//	coremesh-setup install --dir=D [--release=release|preview|<stand>] --package=erp-vuv [--data] [--port=8080]
//	coremesh-setup start|stop|status --dir=D
//	coremesh-setup seed --dir=D --file=einrichtung.yaml
//	coremesh-setup pack --labs=.. --release=2026.10.23 [--build] [--data] [--out=dist/out]
//
// Server-Dateien: --index (Adresse oder Pfad von index.json), Standard COREMESH_SETUP_INDEX
// bzw. dist/index.json im Kern-Repository auf GitHub.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const defaultIndex = "https://raw.githubusercontent.com/coremesh-labs/coremesh/master/dist/index.json"

func logf(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }

func main() {
	cmd := "ui"
	args := os.Args[1:]
	if len(args) > 0 && args[0][0] != '-' {
		cmd, args = args[0], args[1:]
	}
	if err := run(cmd, args); err != nil {
		logf("Fehler: %v", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	index := fs.String("index", envOr("COREMESH_SETUP_INDEX", defaultIndex), "index.json (Adresse oder Pfad)")
	dir := fs.String("dir", "", "Installationsverzeichnis")
	release := fs.String("release", "release", "Linie (release, preview) oder Stand")
	pkg := fs.String("package", "erp-vuv", "Ausprägung")
	data := fs.Bool("data", false, "install: Testbestand der Ausprägung einrichten; pack: Datenbestände erzeugen")
	port := fs.Int("port", 8080, "Port der Weboberfläche des Systems")
	file := fs.String("file", "", "Einrichtungsdatei")
	labs := fs.String("labs", "..", "pack: Verzeichnis mit den Repositories")
	config := fs.String("config", "", "pack: Zusammenstellung (Standard <labs>/coremesh/dist/pack.yaml)")
	out := fs.String("out", "", "pack: Zielverzeichnis (Standard <labs>/coremesh/dist/out/<stand>)")
	previous := fs.String("previous", "", "pack: vorheriger Stand (Update-Pfad)")
	build := fs.Bool("build", false, "pack: vorher bauen")
	listen := fs.String("listen", "127.0.0.1:9870", "ui: Adresse der Oberfläche")
	fs.Parse(args)

	inst := func() (*installation, error) {
		if *dir == "" {
			return nil, fmt.Errorf("--dir fehlt")
		}
		d, err := filepath.Abs(*dir)
		return &installation{dir: d}, err
	}
	src := newSource(*index)
	switch cmd {
	case "ui":
		return serveUI(*listen, src)
	case "list":
		ix, err := src.Index()
		if err != nil {
			return err
		}
		lines := make([]string, 0, len(ix.Lines))
		for l := range ix.Lines {
			lines = append(lines, l)
		}
		sort.Strings(lines)
		for _, l := range lines {
			fmt.Printf("Linie %-8s %s\n", l, ix.Lines[l])
		}
		for _, r := range ix.Releases {
			fmt.Printf("Stand %-12s %s %s\n", r.ID, r.Date, r.Notes)
		}
		for _, p := range ix.Packages {
			fmt.Printf("Ausprägung %-10s %s\n", p.ID, p.Title)
		}
		return nil
	case "install":
		in, err := inst()
		if err != nil {
			return err
		}
		err = install(src, in, installOptions{release: *release, pkg: *pkg, port: *port, withData: *data}, func(s string) { logf("%s", s) })
		if err == nil {
			addr, user, _ := in.access()
			logf("bereit: %s (Benutzer %s, Passwort in %s)", addr, user, in.path("ZUGANG.txt"))
		}
		return err
	case "start", "stop", "status":
		in, err := inst()
		if err != nil {
			return err
		}
		switch cmd {
		case "start":
			remember(in.dir)
			return in.start()
		case "stop":
			return in.stop()
		}
		addr, _, _ := in.access()
		state := "beendet"
		if in.running() {
			state = "läuft"
		}
		fmt.Printf("Stand %s, %s, %s\n", in.current(), state, addr)
		return nil
	case "seed":
		in, err := inst()
		if err != nil {
			return err
		}
		return seed(in, *file)
	case "pack":
		if *release == "release" {
			*release = time.Now().Format("2006.01.02")
		}
		l, err := filepath.Abs(*labs)
		if err != nil {
			return err
		}
		if *config == "" {
			*config = filepath.Join(l, "coremesh", "dist", "pack.yaml")
		}
		if *out == "" {
			*out = filepath.Join(l, "coremesh", "dist", "out", *release)
		}
		return pack(packOptions{labs: l, config: *config, out: *out, release: *release, previous: *previous,
			date: time.Now().Format("2006-01-02"), build: *build, data: *data, port: 19870})
	}
	return fmt.Errorf("unbekannter Befehl %q (ui, list, install, start, stop, status, seed, pack)", cmd)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
