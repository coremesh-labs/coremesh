package module

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Translator ist optional: Ein Modul liefert seine Übersetzungen
// (Locale → Schlüssel → Text). Alle Schlüssel beginnen mit "<Modulname>.".
// Üblich sind eingebettete JSON-Dateien, siehe LoadTranslations.
type Translator interface {
	Translations() metamodel.Translations
}

// LoadTranslations liest <dir>/<locale>.json aus einem eingebetteten
// Dateisystem, z. B.
//
//	//go:embed i18n/*.json
//	var i18nFiles embed.FS
//	func (m *Module) Translations() metamodel.Translations {
//		return module.MustLoadTranslations(i18nFiles, "i18n")
//	}
//
// Jede Datei ist ein flaches JSON-Objekt {"<schlüssel>": "<text>"}.
func LoadTranslations(fsys embed.FS, dir string) (metamodel.Translations, error) {
	out := metamodel.Translations{}
	for _, loc := range metamodel.Locales {
		b, err := fsys.ReadFile(path.Join(dir, loc+".json"))
		if err != nil {
			continue // Sprache nicht übersetzt – Rückfall auf de bzw. Originaltext
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s/%s.json: %w", dir, loc, err)
		}
		out[loc] = m
	}
	return out, nil
}

// MustLoadTranslations ist LoadTranslations, bricht bei Fehlern ab (beim Start).
func MustLoadTranslations(fsys embed.FS, dir string) metamodel.Translations {
	t, err := LoadTranslations(fsys, dir)
	if err != nil {
		panic(err)
	}
	return t
}

// checkTranslations: nur bekannte Sprachen, Schlüssel nur im eigenen Namensraum.
func checkTranslations(modules []string, t metamodel.Translations) []error {
	var errs []error
	for loc, dict := range t {
		if !slices.Contains(metamodel.Locales, loc) {
			errs = append(errs, fmt.Errorf("Übersetzungen: Sprache %q nicht unterstützt (%v)", loc, metamodel.Locales))
		}
		for key := range dict {
			if !slices.ContainsFunc(modules, func(m string) bool { return strings.HasPrefix(key, m+".") }) {
				errs = append(errs, fmt.Errorf("Übersetzungen %s: Schlüssel %q liegt nicht im Namensraum eines eigenen Moduls %v", loc, key, modules))
			}
		}
	}
	return errs
}

// checkTranslations prüft die Übersetzungen aller Module des Plugins.
func (p *Plugin) checkTranslations() []error {
	var names []string
	for _, m := range p.mounted {
		names = append(names, m.desc.Name)
	}
	var errs []error
	for _, m := range p.mounted {
		if tr, ok := m.mod.(Translator); ok {
			errs = append(errs, checkTranslations(names, tr.Translations())...)
		}
	}
	return errs
}
