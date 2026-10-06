package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadDir liest alle *.yaml/*.yml-Dateien aus dir in alphanumerischer
// Reihenfolge (z. B. 01-system.yaml, 02-db-plugin.yaml, 03-domain-plugins.yaml)
// und führt sie zusammen:
//
//   - Maps werden rekursiv verschmolzen, sodass spätere Dateien einzelne
//     Schlüssel ergänzen oder überschreiben können.
//   - Alle anderen Werte, auch Listen, ersetzt die spätere Datei vollständig.
//   - ${env:NAME} in Strings wird durch die Umgebungsvariable NAME des Hosts
//     ersetzt (für Geheimnisse); fehlt sie, ist das ein Fehler.
//
// Unbekannte Schlüssel sind ein Fehler, damit Tippfehler auffallen.
func LoadDir(dir string) (*Config, []string, error) {
	return LoadDirs(dir)
}

// LoadDirs liest mehrere Verzeichnisse nacheinander (je Verzeichnis
// alphanumerisch) und führt alles wie LoadDir zusammen. Spätere Verzeichnisse
// ergänzen frühere, z. B. configs des Kerns und configs von coremesh-erp.
func LoadDirs(dirs ...string) (*Config, []string, error) {
	var files []string
	for _, dir := range dirs {
		if dir = strings.TrimSpace(dir); dir == "" {
			continue
		}
		f, err := configFiles(dir)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, f...)
	}
	if len(files) == 0 {
		return nil, nil, errors.New("kein Konfigurationsverzeichnis angegeben")
	}

	merged := map[string]any{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		var doc map[string]any
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f, err)
		}
		merge(merged, doc)
	}

	if _, err := expandEnv(merged); err != nil {
		return nil, nil, err
	}

	// Über YAML in die typisierte Struktur dekodieren (inkl. Dauer-Angaben wie "30s").
	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, nil, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(out))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, nil, fmt.Errorf("Konfiguration: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, nil, fmt.Errorf("Konfiguration ungültig:\n%w", err)
	}
	return &cfg, files, nil
}

func configFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || strings.HasPrefix(name, ".") || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("keine *.yaml-Dateien in %s", dir)
	}
	sort.Strings(files)
	for i, name := range files {
		files[i] = filepath.Join(dir, name)
	}
	return files, nil
}

// merge führt src in dst zusammen: Maps rekursiv, alles andere wird ersetzt.
func merge(dst, src map[string]any) {
	for k, sv := range src {
		if sm, ok := sv.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
}

var envRef = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

// expandEnv ersetzt ${env:NAME} rekursiv in allen Strings.
func expandEnv(v any) (any, error) {
	switch v := v.(type) {
	case string:
		var missing []string
		out := envRef.ReplaceAllStringFunc(v, func(m string) string {
			name := envRef.FindStringSubmatch(m)[1]
			val, ok := os.LookupEnv(name)
			if !ok {
				missing = append(missing, name)
			}
			return val
		})
		if len(missing) > 0 {
			return nil, fmt.Errorf("Umgebungsvariable nicht gesetzt: %s", strings.Join(missing, ", "))
		}
		return out, nil
	case map[string]any:
		var errs []error
		for k, x := range v {
			nx, err := expandEnv(x)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			v[k] = nx
		}
		return v, errors.Join(errs...)
	case []any:
		var errs []error
		for i, x := range v {
			nx, err := expandEnv(x)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			v[i] = nx
		}
		return v, errors.Join(errs...)
	}
	return v, nil
}
