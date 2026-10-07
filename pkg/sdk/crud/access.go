package crud

import (
	"context"
	"fmt"
	"slices"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Access: Zugriff je Datensatz und je Feldgruppe – crud setzt ihn bei jedem
// Weg durch (list, get, create, update, Beenden, eigene Datensatz-Actions) und
// damit auch für Lookup, Kopfdaten-Vorschau und API. Die Rechte vergibt iam
// (Rolle → Berechtigung je Object.Action → Feldwerte); die Objects und
// Felder bietet die Rollenpflege aus dem Metamodell an.
//
//	Records:     Datensätze nur, soweit <Object>.read sie abdeckt – mit den
//	             Werten von CompanyCode (Attr company_code) und Fields.
//	FieldGroups: Felder einer Gruppe nur mit <Object>.readFields sichtbar und
//	             nur mit <Object>.changeFields änderbar (Berechtigungsfeld
//	             field_group, dazu dieselben Werte wie bei Records).
//
// Es gibt nur Erlaubnisse: Ohne passende Berechtigung ist ein Datensatz
// unsichtbar bzw. eine Feldgruppe ausgeblendet. Anfragen ohne Benutzer
// (System) dürfen alles.
type Access struct {
	// Object, dessen Rechte gelten (Standard: das eigene). So folgen Positionen
	// und Entwürfe den Rechten des Belegs, ohne eigene Rollenpflege.
	Object      string
	Records     bool
	CompanyCode string   // Spalte des Buchungskreises; leer = keine Dimension
	Fields      []string // weitere Berechtigungsfelder (Spalten der Entity)
	FieldGroups []metamodel.FieldGroup
}

// Marker in Datensätzen für die Oberfläche.
const (
	hiddenFieldsKey   = "_hidden_fields"   // ohne Leserecht entfernt
	readonlyFieldsKey = "_readonly_fields" // sichtbar, aber nicht änderbar
)

// authorization ergänzt die Deklaration der Entity um Access (Rollenpflege).
func (e *Entity) authorization() *metamodel.Authorization {
	a := e.Access
	if a == nil || (a.Object != "" && a.Object != e.Object) {
		return e.Authorization
	}
	out := metamodel.Authorization{}
	if e.Authorization != nil {
		out = *e.Authorization
		out.Fields = slices.Clone(out.Fields)
		out.Actions = slices.Clone(out.Actions)
	}
	addField := func(k string) {
		if !slices.Contains(out.Fields, k) {
			out.Fields = append(out.Fields, k)
		}
	}
	// Standardtexte aus dem Modul admin (iam), damit nicht jedes Modul sie übersetzt.
	addAction := func(name, label string) {
		if !slices.ContainsFunc(out.Actions, func(x metamodel.AuthAction) bool { return x.Name == name }) {
			out.Actions = append(out.Actions, metamodel.AuthAction{Name: name, Label: label, LabelKey: "admin.auth." + name})
		}
	}
	for _, k := range a.Fields {
		addField(k)
	}
	if a.Records {
		addAction(metamodel.ActionRead, "Datensätze sehen")
	}
	if len(a.FieldGroups) > 0 {
		addField(metamodel.FieldGroupAttr)
		addAction(metamodel.ActionReadFields, "Feldgruppe sehen")
		addAction(metamodel.ActionChangeFields, "Feldgruppe ändern")
		out.FieldGroups = append(slices.Clone(out.FieldGroups), a.FieldGroups...)
	}
	return &out
}

// accessCheck sind die Erlaubnisse des Benutzers für eine Anfrage.
type accessCheck struct {
	e                    *Entity
	read, rFields, chFld *sdk.GrantSet // nil = nicht eingeschränkt
}

func (e *Entity) accessCheck(ctx context.Context) (*accessCheck, error) {
	c := &accessCheck{e: e}
	a := e.Access
	if a == nil || sdk.CallFromContext(ctx).UserID == "" {
		return c, nil
	}
	load := func(action string) (*sdk.GrantSet, error) {
		g, err := sdk.Grants(ctx, a.object(e.Object), action)
		if err != nil {
			return nil, err
		}
		return &g, nil
	}
	var err error
	if a.Records {
		if c.read, err = load(metamodel.ActionRead); err != nil {
			return nil, err
		}
	}
	if len(a.FieldGroups) > 0 {
		if c.rFields, err = load(metamodel.ActionReadFields); err != nil {
			return nil, err
		}
		if c.chFld, err = load(metamodel.ActionChangeFields); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// attrs: Werte des Datensatzes für die Berechtigungsprüfung.
func (e *Entity) attrs(rec Record) sdk.Attrs {
	out := sdk.Attrs{}
	if a := e.Access; a != nil {
		if a.CompanyCode != "" && rec[a.CompanyCode] != nil {
			out[sdk.AttrCompanyCode] = Str(rec[a.CompanyCode])
		}
		for _, k := range a.Fields {
			if rec[k] != nil {
				out[k] = Str(rec[k])
			}
		}
	}
	return out
}

// canRead: Deckt <Object>.read den Datensatz ab?
func (c *accessCheck) canRead(rec Record) bool {
	return c.read == nil || c.read.Allows(c.e.attrs(rec))
}

// scope: WHERE-Bedingung für list.
func (c *accessCheck) scope() (string, []any) {
	if c.read == nil {
		return "", nil
	}
	cols := map[string]string{}
	if a := c.e.Access; a.CompanyCode != "" {
		cols[sdk.AttrCompanyCode] = a.CompanyCode
	}
	for _, k := range c.e.Access.Fields {
		cols[k] = k
	}
	return c.read.SQL(cols)
}

func allowsGroup(g *sdk.GrantSet, attrs sdk.Attrs, group string) bool {
	if g == nil {
		return true
	}
	a := sdk.Attrs{metamodel.FieldGroupAttr: group}
	for k, v := range attrs {
		a[k] = v
	}
	return g.Allows(a)
}

// groups: Felder ohne Leserecht (hidden) und ohne Änderungsrecht (locked).
func (c *accessCheck) groups(rec Record) (hidden, locked []string) {
	if c.e.Access == nil {
		return nil, nil
	}
	attrs := c.e.attrs(rec)
	for _, g := range c.e.Access.FieldGroups {
		switch {
		case !allowsGroup(c.rFields, attrs, g.Key):
			hidden = append(hidden, g.Fields...)
			locked = append(locked, g.Fields...)
		case !allowsGroup(c.chFld, attrs, g.Key):
			locked = append(locked, g.Fields...)
		}
	}
	return hidden, locked
}

// filter entfernt Felder ohne Leserecht (auch ihre Labels) und markiert
// ausgeblendete und schreibgeschützte Felder für die Oberfläche.
func (c *accessCheck) filter(rec Record) {
	hidden, locked := c.groups(rec)
	if len(locked) == 0 {
		return
	}
	labels, _ := rec["_labels"].(map[string]any)
	var h, ro []any
	for _, k := range hidden {
		delete(rec, k)
		delete(labels, k)
		h = append(h, k)
	}
	for _, k := range locked {
		if !slices.Contains(hidden, k) {
			ro = append(ro, k)
		}
	}
	if len(h) > 0 {
		rec[hiddenFieldsKey] = h
	}
	if len(ro) > 0 {
		rec[readonlyFieldsKey] = ro
	}
}

// checkChange: Felder gesperrter Gruppen dürfen sich nicht ändern (old nil =
// Neuanlage: sie dürfen nicht gesetzt sein).
func (c *accessCheck) checkChange(in, rec, old Record) error {
	if c.chFld == nil {
		return nil
	}
	attrs := c.e.attrs(rec)
	for _, g := range c.e.Access.FieldGroups {
		if allowsGroup(c.chFld, attrs, g.Key) && (old == nil || allowsGroup(c.chFld, c.e.attrs(old), g.Key)) {
			continue
		}
		for _, k := range g.Fields {
			v, ok := in[k]
			if !ok {
				continue
			}
			if (old == nil && v != nil && Str(v) != "") || (old != nil && Str(v) != Str(old[k])) {
				return fmt.Errorf("%w: keine Berechtigung, %s (%s) zu ändern", sdk.ErrPermissionDenied, g.Label, c.e.Field(k).Label)
			}
		}
	}
	return nil
}

// notVisible: Datensatz ohne Leserecht – wie nicht vorhanden.
func (e *Entity) notVisible(rec Record) error {
	return fmt.Errorf("%w: %s %s", sdk.ErrNotFound, e.Title, e.RecordID(rec))
}

// checkReadable lädt die Erlaubnisse und prüft rec (für Actions auf einem Datensatz).
func (e *Entity) checkReadable(ctx context.Context, rec Record) (*accessCheck, error) {
	c, err := e.accessCheck(ctx)
	if err != nil {
		return nil, err
	}
	if !c.canRead(rec) {
		return nil, e.notVisible(rec)
	}
	return c, nil
}

// checkAccess: Access nennt nur Spalten der Entity.
func (e *Entity) checkAccess() error {
	a := e.Access
	if a == nil {
		return nil
	}
	cols := append([]string{a.CompanyCode}, a.Fields...)
	for _, g := range a.FieldGroups {
		cols = append(cols, g.Fields...)
	}
	for _, k := range cols {
		if k == "" {
			continue
		}
		if f := e.Field(k); f == nil || f.Virtual {
			return fmt.Errorf("crud %s: Access nennt %q – keine Spalte", e.Object, k)
		}
	}
	return nil
}

// object: Object, dessen Rechte gelten.
func (a *Access) object(own string) string {
	if a.Object != "" {
		return a.Object
	}
	return own
}
