package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// moduleEntry ist der Metamodell-Stand eines Moduls.
type moduleEntry struct {
	Module       string                       `json:"module"`
	Version      string                       `json:"version"`
	Checksum     string                       `json:"checksum"`
	RegisteredAt string                       `json:"registered_at"`
	Objects      []metamodel.ObjectDefinition `json:"objects"`
	Modules      []metamodel.ModuleDefinition `json:"modules,omitempty"`
}

// cacheFile ist das Format der Cache-Datei.
type cacheFile struct {
	Modules map[string]*moduleEntry `json:"modules"`
}

const cacheName = "catalog-cache.json"

// checkOwnership prüft, dass ein Modul nur Objects beschreibt, die es selbst
// bedient, und nur Actions anbietet, die als Route dieses Moduls existieren.
func checkOwnership(module string, defs []metamodel.ObjectDefinition, mods []metamodel.ModuleDefinition, routes []dispatcher.Entry) error {
	own := map[string][]string{} // Object -> Actions dieses Moduls
	for _, e := range routes {
		if e.Plugin == module {
			own[e.Object] = append(own[e.Object], e.Action)
		}
	}
	var errs []error
	seen := map[string]bool{}
	for _, d := range defs {
		if err := d.Validate(); err != nil {
			errs = append(errs, err)
			continue
		}
		if seen[d.Name] {
			errs = append(errs, fmt.Errorf("Object %s: doppelt definiert", d.Name))
		}
		seen[d.Name] = true
		actions, ok := own[d.Name]
		if !ok {
			errs = append(errs, fmt.Errorf("Object %s: wird nicht von Modul %s bedient", d.Name, module))
			continue
		}
		for _, a := range d.Actions {
			if !slices.Contains(actions, a.Name) {
				errs = append(errs, fmt.Errorf("Object %s: Action %q ist keine Route von Modul %s", d.Name, a.Name, module))
			}
		}
	}
	// Module bündeln nur eigene, beschriebene Objects; jedes Object höchstens einmal.
	inModule := map[string]string{}
	names := map[string]bool{}
	for _, md := range mods {
		if err := md.Validate(seen); err != nil {
			errs = append(errs, err)
		}
		if names[md.Name] {
			errs = append(errs, fmt.Errorf("Modul %s: doppelt", md.Name))
		}
		names[md.Name] = true
		for _, o := range md.Objects {
			if other, ok := inModule[o.Object]; ok && other != md.Name {
				errs = append(errs, fmt.Errorf("Object %s: gehört zu Modul %s und %s", o.Object, other, md.Name))
			}
			inModule[o.Object] = md.Name
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: Metamodell von Modul %s abgelehnt:\n%w", sdk.ErrPermissionDenied, module, errors.Join(errs...))
	}
	return nil
}

func newEntry(module, version string, defs []metamodel.ObjectDefinition, mods []metamodel.ModuleDefinition) (*moduleEntry, error) {
	b, err := json.Marshal(struct {
		Objects []metamodel.ObjectDefinition
		Modules []metamodel.ModuleDefinition
	}{defs, mods})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	return &moduleEntry{
		Module:       module,
		Version:      version,
		Checksum:     hex.EncodeToString(sum[:]),
		RegisteredAt: time.Now().UTC().Format(time.RFC3339),
		Objects:      defs,
		Modules:      mods,
	}, nil
}

// loadCache liest die Cache-Datei; fehlt sie, ist das kein Fehler.
func loadCache(dir string) (map[string]*moduleEntry, error) {
	b, err := os.ReadFile(filepath.Join(dir, cacheName))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]*moduleEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f cacheFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", cacheName, err)
	}
	if f.Modules == nil {
		f.Modules = map[string]*moduleEntry{}
	}
	return f.Modules, nil
}

// writeCache schreibt die Cache-Datei atomar (temporäre Datei + Rename).
func writeCache(dir string, modules map[string]*moduleEntry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cacheFile{Modules: modules}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, cacheName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op nach Rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, cacheName))
}
