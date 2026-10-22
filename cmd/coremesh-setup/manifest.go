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

// Manifest: Inhalt eines Stands (manifest.json neben den Archiven).
type Manifest struct {
	Release   string              `json:"release"`
	Date      string              `json:"date"`
	Previous  string              `json:"previous,omitempty"`
	Sources   map[string]string   `json:"sources"` // Repository → Commit
	Plugins   map[string]string   `json:"plugins"` // Plugin → Version
	Platforms map[string]Artifact `json:"platforms"`
	Setup     map[string]Artifact `json:"setup,omitempty"` // coremesh-setup je Plattform
	Packages  map[string]Package  `json:"packages"`
}

// Artifact: herunterladbare Datei mit Prüfsumme.
type Artifact struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Package: Ausprägung in einem Stand – Plugins (leer = alle) und optional ein Datenbestand.
type Package struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Plugins     []string `json:"plugins,omitempty"`
	Data        *DataSet `json:"data,omitempty"`
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
	return &m, nil
}
