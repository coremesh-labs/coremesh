package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Index: Server-Datei mit den Ständen (dist/index.json im Kern-Repository).
// Nur hier wird gepflegt, welcher Stand Release bzw. Preview ist.
type Index struct {
	Format       int               `json:"format"`
	Repositories map[string]string `json:"repositories"` // Name → Vorlage mit {release} und {file}
	Lines        map[string]string `json:"lines"`        // release/preview → Stand
	Releases     []ReleaseRef      `json:"releases"`
	Packages     []PackageRef      `json:"packages"`
}

// ReleaseRef: ein Stand; unveränderlich, sobald veröffentlicht.
type ReleaseRef struct {
	ID         string `json:"id"`
	Date       string `json:"date"`
	Previous   string `json:"previous,omitempty"` // Update-Pfad
	Via        bool   `json:"via,omitempty"`      // Pflicht-Zwischenstand bei Updates
	Repository string `json:"repository"`         // Name aus Index.Repositories
	Notes      string `json:"notes,omitempty"`
}

// PackageRef: Ausprägung zur Auswahl beim Einrichten (z. B. ERP Vermietung und Verwaltung).
type PackageRef struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Description    string   `json:"description,omitempty"`
	CompatibleWith []string `json:"compatible_with,omitempty"` // in einem System kombinierbar
}

// Manifest: Inhalt eines Stands (manifest.json im Release). Ein Stand ist die Liste der
// zusammengehörenden Bausteine; jeder Baustein ist eine eigene Datei. Das Werkzeug lädt nur,
// was in der Installation fehlt – bei einem Update meist wenige Plugins.
type Manifest struct {
	Format     int                `json:"format"` // 2: Bausteine
	Release    string             `json:"release"`
	Date       string             `json:"date"`
	Previous   string             `json:"previous,omitempty"`
	Sources    map[string]string  `json:"sources"` // Repository → Commit
	Components []Component        `json:"components"`
	Packages   map[string]Package `json:"packages"`
}

// Baustein-Arten
const (
	kindProgram = "program" // host, console → releases/<stand>/
	kindPlugin  = "plugin"  // → plugins/<xx>/ (gemeinsam für alle Stände)
	kindFiles   = "files"   // configs, migrations, Inhalte, Lizenzen (tar.gz) → releases/<stand>/
	kindSetup   = "setup"   // coremesh-setup selbst → Installationsverzeichnis
)

// Component: ein Baustein. Plugins heißen nach ihrem Namen und tragen die Version aus der
// Konfiguration; die übrigen tragen als Version den Anfang ihrer Prüfsumme.
type Component struct {
	Name      string              `json:"name"`
	Kind      string              `json:"kind"`
	Version   string              `json:"version"`
	Platforms map[string]Artifact `json:"platforms"` // Plattform (linux-amd64, …) bzw. "any"
}

// Artifact: herunterladbare Datei. Gzip-komprimiert, außer bei coremesh-setup (zum
// direkten Herunterladen). Release nennt den Stand, in dem die Datei liegt, wenn ein
// früherer Stand sie schon veröffentlicht hat – sie wird dann nicht noch einmal hochgeladen.
type Artifact struct {
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Path    string `json:"path,omitempty"`    // Ziel (Plugin: plugins/we/webserver-0.45.0-linux-amd64)
	Content string `json:"content,omitempty"` // SHA-256 der entpackten Datei (nicht bei files)
	Release string `json:"release,omitempty"` // Stand mit der Datei; leer = dieser
}

// artifactFor: Datei des Bausteins für eine Plattform ("any" gilt für alle).
func (c Component) artifactFor(p string) (Artifact, bool) {
	if a, ok := c.Platforms[p]; ok {
		return a, true
	}
	a, ok := c.Platforms["any"]
	return a, ok
}

// Package: Ausprägung in einem Stand – Plugins (leer = alle) und optional ein Datenbestand.
type Package struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Plugins     []string `json:"plugins,omitempty"`
	Detect      []string `json:"detect,omitempty"` // an diesen Plugins erkennt das Werkzeug die Ausprägung
	Data        *DataSet `json:"data,omitempty"`
}

// components: die Bausteine einer Ausprägung (Programme, Dateien und Werkzeug immer).
func (m *Manifest) components(pkg string) ([]Component, error) {
	p, ok := m.Packages[pkg]
	if !ok {
		return nil, fmt.Errorf("Stand %s hat keine Ausprägung %q", m.Release, pkg)
	}
	var out []Component
	for _, c := range m.Components {
		if c.Kind != kindPlugin || len(p.Plugins) == 0 || contains(p.Plugins, c.Name) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *Manifest) component(kind, name string) *Component {
	for i := range m.Components {
		if m.Components[i].Kind == kind && m.Components[i].Name == name {
			return &m.Components[i]
		}
	}
	return nil
}

// DataSet: mitgelieferte Datenbank (SQLite, gzip). Der Zugang des Bestands wird beim
// Einrichten durch ein neues Passwort ersetzt.
type DataSet struct {
	Artifact
	Title    string `json:"title"`
	User     string `json:"user"`
	Password string `json:"password"` // Passwort im Bestand (öffentlich, wird ersetzt)
}

// source: Quelle der Server-Dateien – https://… oder Verzeichnis (lokal gebaute Stände).
type source struct {
	index string // Adresse oder Pfad von index.json
	cli   *http.Client
}

func newSource(index string) *source {
	return &source{index: index, cli: &http.Client{Timeout: 10 * time.Minute}}
}

func isURL(s string) bool { return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") }

// open liefert den Inhalt einer Adresse oder Datei (file:// oder Pfad).
func (s *source) open(loc string) (io.ReadCloser, error) {
	if isURL(loc) {
		resp, err := s.cli.Get(loc)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("%s: %s", loc, resp.Status)
		}
		return resp.Body, nil
	}
	return os.Open(strings.TrimPrefix(loc, "file://"))
}

func (s *source) readJSON(loc string, v any) error {
	r, err := s.open(loc)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", loc, err)
	}
	return nil
}

func (s *source) Index() (*Index, error) {
	var ix Index
	if err := s.readJSON(s.index, &ix); err != nil {
		return nil, err
	}
	if ix.Format != 1 {
		return nil, fmt.Errorf("index.json: Format %d wird nicht unterstützt", ix.Format)
	}
	return &ix, nil
}

// artifactURL: Adresse einer Datei, auch wenn sie in einem früheren Stand liegt.
func (ix *Index) artifactURL(rel string, a Artifact) (string, error) {
	if a.Release != "" {
		rel = a.Release
	}
	return ix.fileURL(rel, a.File)
}

// fileURL: Adresse einer Datei eines Stands nach der Vorlage seines Repositories.
func (ix *Index) fileURL(rel, file string) (string, error) {
	r := ix.Release(rel)
	if r == nil {
		return "", fmt.Errorf("Stand %s ist nicht im Index", rel)
	}
	tpl, ok := ix.Repositories[r.Repository]
	if !ok {
		return "", fmt.Errorf("Stand %s: Repository %q fehlt", rel, r.Repository)
	}
	return strings.NewReplacer("{release}", rel, "{file}", file).Replace(tpl), nil
}

func (ix *Index) Release(id string) *ReleaseRef {
	for i := range ix.Releases {
		if ix.Releases[i].ID == id {
			return &ix.Releases[i]
		}
	}
	return nil
}

// resolve: Stand zu einer Linie (release/preview) oder einer Stand-Kennung.
func (ix *Index) resolve(want string) (string, error) {
	if want == "" {
		want = "release"
	}
	if id, ok := ix.Lines[want]; ok {
		return id, nil
	}
	if ix.Release(want) != nil {
		return want, nil
	}
	return "", fmt.Errorf("unbekannter Stand oder Linie %q", want)
}

func (s *source) Manifest(ix *Index, rel string) (*Manifest, error) {
	u, err := ix.fileURL(rel, "manifest.json")
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := s.readJSON(u, &m); err != nil {
		return nil, err
	}
	if m.Format != 2 {
		return nil, fmt.Errorf("Stand %s: Manifest-Format %d – dieses Werkzeug versteht nur Bausteine (Format 2); Stand neu packen oder älteres Werkzeug nehmen", rel, m.Format)
	}
	return &m, nil
}
