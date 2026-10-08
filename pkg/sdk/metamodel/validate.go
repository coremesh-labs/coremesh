package metamodel

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

var (
	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	actionRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	keyRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

var (
	fieldTypes  = map[FieldType]bool{TypeText: true, TypeTextarea: true, TypeNumber: true, TypeDate: true, TypeSelect: true, TypeEmail: true, TypeBoolean: true, TypePassword: true, TypeFile: true}
	actionKinds = map[ActionKind]bool{KindList: true, KindItem: true, KindCreate: true, KindUpdate: true, KindExpire: true, KindDeactivate: true, KindCustom: true}
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
		if f.Type == TypeFile && !f.ActionOnly {
			add("%s: type file nur bei action_only (Formulare eigener Aktionen)", where)
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
		if f.Type == TypeSelect && len(f.Options) == 0 && d.FormState == "" {
			add("%s: select braucht options (oder FormState, der sie liefert)", where)
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
		if lk := f.Lookup; lk != nil {
			if !objectRe.MatchString(lk.Object) {
				add("%s: lookup.object %q: PascalCase erwartet", where, lk.Object)
			}
			if !keyRe.MatchString(lk.ValueField) {
				add("%s: lookup.value_field fehlt oder ungültig", where)
			}
			if len(lk.LabelFields) == 0 {
				add("%s: lookup.label_fields fehlt", where)
			}
			for _, k := range append(slices.Clone(lk.LabelFields), lk.Columns...) {
				if !keyRe.MatchString(k) {
					add("%s: lookup: ungültiges Feld %q", where, k)
				}
			}
			if f.Type == TypeSelect || f.Type == TypeBoolean || f.Type == TypePassword || f.Type == TypeFile {
				add("%s: lookup nicht bei type %s", where, f.Type)
			}
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
		switch {
		case a.Kind == "delete":
			add("%s: kind delete gibt es nicht – physisches Löschen ist nicht vorgesehen (Lifecycle: expire bzw. deactivate)", where)
		case !actionKinds[a.Kind]:
			add("%s: unbekannter kind %q", where, a.Kind)
		case (a.Kind == KindExpire || a.Kind == KindDeactivate) && a.Kind != d.Lifecycle.EndAction():
			add("%s: kind %s passt nicht zum Lifecycle %s", where, a.Kind, d.Lifecycle.Kind())
		}
		if a.Label == "" {
			add("%s: label fehlt", where)
		}
	}

	checkLifecycle(d, add)
	if d.TitleField != "" && !keys[d.TitleField] {
		add("title_field %q ist kein Feld", d.TitleField)
	}
	sections, placed := map[string]bool{}, map[string]string{}
	for i, s := range d.Sections {
		where := fmt.Sprintf("sections[%d] %q", i, s.Key)
		if !keyRe.MatchString(s.Key) || sections[s.Key] {
			add("%s: key ungültig oder doppelt", where)
		}
		sections[s.Key] = true
		if s.Title == "" {
			add("%s: title fehlt", where)
		}
		kinds := 0
		for _, set := range []bool{len(s.Fields) > 0, s.Relation != nil, s.Tags, s.Documents} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			add("%s: genau eines von fields, relation, tags und documents angeben", where)
		}
		for _, k := range s.Fields {
			switch {
			case !keys[k]:
				add("%s: Feld %q gibt es nicht", where, k)
			case placed[k] != "":
				add("%s: Feld %q steht schon in Abschnitt %s", where, k, placed[k])
			}
			placed[k] = s.Key
		}
		if r := s.Relation; r != nil {
			if !objectRe.MatchString(r.Object) || !keyRe.MatchString(r.ForeignKey) {
				add("%s: relation braucht object (PascalCase) und foreign_key", where)
			}
			if _, ok := r.Match[r.ForeignKey]; len(r.Match) > 0 && !ok {
				add("%s: relation.match muss foreign_key %q enthalten", where, r.ForeignKey)
			}
			for child, master := range r.Match {
				if !keyRe.MatchString(child) || !keyRe.MatchString(master) {
					add("%s: relation.match: ungültiges Feld %q → %q", where, child, master)
				}
			}
			for _, k := range r.Columns {
				if !keyRe.MatchString(k) {
					add("%s: relation: ungültige Spalte %q", where, k)
				}
			}
		}
	}

	if az := d.Authorization; az != nil {
		for _, k := range az.Fields {
			switch {
			case k == "company_code":
				add("authorization: company_code ist immer Dimension, kein Berechtigungsfeld")
			case k == FieldGroupAttr:
				if len(az.FieldGroups) == 0 {
					add("authorization: field_group nur mit field_groups")
				}
			case !keys[k]:
				add("authorization: Feld %q gibt es nicht", k)
			}
		}
		for _, a := range az.Actions {
			switch {
			case !actionRe.MatchString(a.Name) || names[a.Name]:
				add("authorization: Action %q ungültig oder schon in actions", a.Name)
			case a.Label == "":
				add("authorization: Action %q: label fehlt", a.Name)
			}
			names[a.Name] = true
		}
		groups, grouped := map[string]bool{}, map[string]string{}
		for _, g := range az.FieldGroups {
			if !keyRe.MatchString(g.Key) || groups[g.Key] || g.Label == "" || len(g.Fields) == 0 {
				add("authorization: Feldgruppe %q braucht eindeutigen key, label und fields", g.Key)
			}
			groups[g.Key] = true
			for _, k := range g.Fields {
				switch {
				case !keys[k]:
					add("authorization: Feldgruppe %s: Feld %q gibt es nicht", g.Key, k)
				case grouped[k] != "":
					add("authorization: Feld %q steht schon in Feldgruppe %s", k, grouped[k])
				}
				grouped[k] = g.Key
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Object %s: %w", d.Name, errors.Join(errs...))
	}
	return nil
}

// checkLifecycle prüft Typ und Felder des Lebenszyklus.
func checkLifecycle(d ObjectDefinition, add func(string, ...any)) {
	l := d.Lifecycle
	field := func(key string, want FieldType, what string) {
		i := slices.IndexFunc(d.Fields, func(f FieldDefinition) bool { return f.Key == key })
		switch {
		case key == "":
			add("lifecycle %s: %s fehlt", l.Type, what)
		case i < 0:
			add("lifecycle %s: %s %q ist kein Feld", l.Type, what, key)
		case d.Fields[i].Type != want:
			add("lifecycle %s: %s %q muss type %s haben", l.Type, what, key, want)
		}
	}
	switch l.Kind() {
	case LifecycleTimeSlice:
		field(l.ValidFrom, TypeDate, "valid_from")
		field(l.ValidTo, TypeDate, "valid_to")
	case LifecycleStatus:
		i := slices.IndexFunc(d.Fields, func(f FieldDefinition) bool { return f.Key == l.StatusField })
		switch {
		case l.InactiveValue == "":
			field(l.StatusField, TypeBoolean, "status_field")
		case i < 0 || d.Fields[i].Type != TypeSelect:
			add("lifecycle status: status_field %q mit inactive_value muss type select haben", l.StatusField)
		case !slices.ContainsFunc(d.Fields[i].Options, func(o Option) bool { return o.Value == l.InactiveValue }):
			add("lifecycle status: inactive_value %q ist kein Wert von %s", l.InactiveValue, l.StatusField)
		}
	case LifecycleImmutable:
		if l.ValidFrom != "" || l.ValidTo != "" || l.StatusField != "" {
			add("lifecycle immutable: keine Felder angeben")
		}
	default:
		add("lifecycle: unbekannter type %q (timeslice, status, immutable)", l.Type)
	}
}
