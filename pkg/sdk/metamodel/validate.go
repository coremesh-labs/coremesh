package metamodel

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	actionRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	keyRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

var (
	fieldTypes  = map[FieldType]bool{TypeText: true, TypeTextarea: true, TypeNumber: true, TypeDate: true, TypeSelect: true, TypeEmail: true, TypeBoolean: true, TypePassword: true}
	actionKinds = map[ActionKind]bool{KindList: true, KindItem: true, KindCreate: true, KindUpdate: true, KindDelete: true, KindCustom: true}
)

// Validate prüft eine Definition auf Vollständigkeit und Eindeutigkeit.
func (d ObjectDefinition) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if !objectRe.MatchString(d.Name) {
		add("name %q: PascalCase erwartet", d.Name)
	}
	if d.Title == "" {
		add("title fehlt")
	}

	keys := map[string]bool{}
	for i, f := range d.Fields {
		where := fmt.Sprintf("fields[%d] %q", i, f.Key)
		if !keyRe.MatchString(f.Key) {
			add("%s: key muss [a-z][a-z0-9_]* sein", where)
		}
		if keys[f.Key] {
			add("%s: key doppelt", where)
		}
		keys[f.Key] = true
		if f.Label == "" {
			add("%s: label fehlt", where)
		}
		if !fieldTypes[f.Type] {
			add("%s: unbekannter type %q", where, f.Type)
		}
		if f.Type == TypeSelect && len(f.Options) == 0 {
			add("%s: select braucht options", where)
		}
		if f.Type != TypeSelect && len(f.Options) > 0 {
			add("%s: options nur bei type select", where)
		}
		values := map[string]bool{}
		for _, o := range f.Options {
			if o.Value == "" || values[o.Value] {
				add("%s: option value leer oder doppelt (%q)", where, o.Value)
			}
			values[o.Value] = true
		}
	}

	names := map[string]bool{}
	for i, a := range d.Actions {
		where := fmt.Sprintf("actions[%d] %q", i, a.Name)
		if !actionRe.MatchString(a.Name) {
			add("%s: ungültiger name", where)
		}
		if names[a.Name] {
			add("%s: name doppelt", where)
		}
		names[a.Name] = true
		if !actionKinds[a.Kind] {
			add("%s: unbekannter kind %q", where, a.Kind)
		}
		if a.Label == "" {
			add("%s: label fehlt", where)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Object %s: %w", d.Name, errors.Join(errs...))
	}
	return nil
}
