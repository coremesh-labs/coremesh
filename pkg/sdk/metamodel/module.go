package metamodel

import (
	"errors"
	"fmt"
	"regexp"
)

// ModuleDefinition beschreibt ein fachliches Modul: eine Gruppe
// zusammengehöriger Business-Objects mit eigenem Namensraum.
//
// Ein Plugin (Prozess) bedient ein oder mehrere Module. Der WebServer
// registriert ausschließlich Module: Er bindet jedes unter
// /m/<Name>/… (Oberfläche) und /api/v1/<Name>/… (JSON) ein. Objects, die
// keinem Modul angehören, sind über den WebServer nicht erreichbar.
type ModuleDefinition struct {
	Name           string         `json:"name"`                      // Namensraum (URL-Segment), z. B. "businesspartner"
	Title          string         `json:"title"`                     // z. B. "Geschäftspartner"
	Icon           string         `json:"icon,omitempty"`            // Icon-CSS-Klasse
	Description    string         `json:"description,omitempty"`     // Kurzbeschreibung (Startseite)
	TitleKey       string         `json:"title_key,omitempty"`       // Übersetzungsschlüssel (i18n)
	DescriptionKey string         `json:"description_key,omitempty"` // Übersetzungsschlüssel (i18n)
	Objects        []ModuleObject `json:"objects"`                   // Reihenfolge = Navigation; das erste ist die Einstiegsseite
	// Services sind Objects des Moduls ohne Metamodell (z. B. Tags): nur über die
	// JSON-API /api/v1/<Name>/<Object>/<Action> erreichbar, nicht in der Navigation.
	Services []string `json:"services,omitempty"`
}

// ModuleObject ordnet ein Business-Object einem Modul zu.
type ModuleObject struct {
	Object     string `json:"object"`
	Section    string `json:"section,omitempty"`     // Gruppe in der Navigation, z. B. "Kataloge"
	SectionKey string `json:"section_key,omitempty"` // Übersetzungsschlüssel der Gruppe (i18n)
}

var moduleRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidModuleName: Kleinbuchstaben, Ziffern und "-", beginnt mit einem Buchstaben.
func ValidModuleName(name string) bool {
	return len(name) >= 2 && len(name) <= 40 && moduleRe.MatchString(name) && !reservedModules[name]
}

// Validate prüft ein Modul gegen die Objects, die dasselbe Plugin
// definiert (defined): Ein Modul bündelt nur eigene Objects.
func (m ModuleDefinition) Validate(defined map[string]bool) error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if !ValidModuleName(m.Name) {
		add("Modul %q: name muss %s entsprechen (2–40 Zeichen)", m.Name, moduleRe)
	}
	if m.Title == "" {
		add("Modul %s: title fehlt", m.Name)
	}
	if len(m.Objects) == 0 {
		add("Modul %s: enthält keine Objects", m.Name)
	}
	seen := map[string]bool{}
	for _, o := range m.Objects {
		switch {
		case seen[o.Object]:
			add("Modul %s: Object %s doppelt", m.Name, o.Object)
		case !defined[o.Object]:
			add("Modul %s: Object %s ist kein eigenes Object mit Metamodell", m.Name, o.Object)
		}
		seen[o.Object] = true
	}
	for _, s := range m.Services {
		if !objectRe.MatchString(s) || seen[s] {
			add("Modul %s: Service %q ungültig oder auch als Object eingetragen", m.Name, s)
		}
		seen[s] = true
	}
	return errors.Join(errs...)
}
