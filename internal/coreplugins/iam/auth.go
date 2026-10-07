package iam

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Rollenpflege auf Basis des Catalogs: RoleAuth (je Object.Action eine Zeile)
// und RoleAuthValue (erlaubte Feldwerte). Welche Objects, Actions und
// Berechtigungsfelder es gibt, liefert der Catalog – die Masken bieten sie
// über FormState als Auswahl an.

const formStateAction = "formState"

// hiddenField: ausgeblendet – der WebServer schickt den Wert nicht (nil).
var hiddenField = metamodel.FieldState{Visible: new(bool)}

// --- Catalog ---------------------------------------------------------------------

// catalogView fragt den Catalog innerhalb einer Anfrage ab (mit Zwischenspeicher)
// und übersetzt Titel in die Sprache der Anfrage.
type catalogView struct {
	p    *Plugin
	ctx  context.Context
	tr   metamodel.Translations
	loc  string
	defs map[string]*metamodel.ObjectDefinition
	objs []catalogObject
}

type catalogObject struct {
	Object string `json:"object"`
	Title  string `json:"title"`
}

func (p *Plugin) catalog(ctx context.Context) *catalogView {
	c := &catalogView{p: p, ctx: ctx, defs: map[string]*metamodel.ObjectDefinition{}, loc: metamodel.LocaleDE}
	if loc := metamodel.NormalizeLocale(sdk.CallFromContext(ctx).Metadata["locale"]); loc != "" {
		c.loc = loc
	}
	if resp, err := c.call("Translations", map[string]any{"locale": c.loc}); err == nil {
		var out struct {
			Translations map[string]string `json:"translations"`
		}
		if sdk.Decode(resp.Payload, &out) == nil {
			c.tr = metamodel.Translations{c.loc: out.Translations}
		}
	}
	return c
}

func (c *catalogView) call(action string, payload any) (sdk.Response, error) {
	if c.p.host == nil {
		return sdk.Response{}, fmt.Errorf("%w: kein Host", sdk.ErrUnavailable)
	}
	return c.p.host.Handle(c.ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: action, Payload: payload})
}

// def: Definition des Objects (übersetzt) oder nil.
func (c *catalogView) def(object string) *metamodel.ObjectDefinition {
	if d, ok := c.defs[object]; ok {
		return d
	}
	c.defs[object] = nil
	if strings.ContainsAny(object, "*?") {
		return nil
	}
	resp, err := c.call("GetDefinition", map[string]any{"object": object})
	if err != nil {
		return nil
	}
	var out struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
	}
	if sdk.Decode(resp.Payload, &out) != nil {
		return nil
	}
	d := out.Definition.Localize(c.tr, c.loc)
	c.defs[object] = &d
	return &d
}

// objects: alle Objects mit Route oder Definition.
func (c *catalogView) objects() []catalogObject {
	if c.objs != nil {
		return c.objs
	}
	c.objs = []catalogObject{}
	if resp, err := c.call("ListObjects", map[string]any{}); err == nil {
		var out struct {
			Objects []catalogObject `json:"objects"`
		}
		if sdk.Decode(resp.Payload, &out) == nil {
			c.objs = out.Objects
		}
	}
	for i, o := range c.objs {
		if d := c.def(o.Object); d != nil {
			c.objs[i].Title = d.Title
		}
	}
	return c.objs
}

// actions: aufrufbare Actions (Routen) und reine Berechtigungs-Actions.
func (c *catalogView) actions(object string) []metamodel.Option {
	var out []metamodel.Option
	seen := map[string]bool{}
	add := func(name, label string) {
		if seen[name] {
			return
		}
		seen[name] = true
		if label == "" || label == name {
			label = name
		} else {
			label = name + " – " + label
		}
		out = append(out, metamodel.Option{Value: name, Label: label})
	}
	d := c.def(object)
	labels := map[string]string{}
	if d != nil {
		for _, a := range d.Actions {
			labels[a.Name] = a.Label
		}
	}
	if resp, err := c.call("ListActions", map[string]any{"object": object}); err == nil {
		var res struct {
			Actions []struct {
				Action string `json:"action"`
			} `json:"actions"`
		}
		if sdk.Decode(resp.Payload, &res) == nil {
			for _, a := range res.Actions {
				add(a.Action, labels[a.Action])
			}
		}
	}
	if d != nil && d.Authorization != nil {
		for _, a := range d.Authorization.Actions {
			add(a.Name, a.Label)
		}
	}
	return out
}

// authFields: Berechtigungsfelder der Objects, die auf das Muster object
// passen (Schlüssel → Bezeichnung, in Reihenfolge der Deklaration).
func (c *catalogView) authFields(object string) []metamodel.Option {
	var out []metamodel.Option
	collect := func(d *metamodel.ObjectDefinition) {
		if d == nil || d.Authorization == nil {
			return
		}
		for _, k := range d.Authorization.Fields {
			if slices.ContainsFunc(out, func(o metamodel.Option) bool { return o.Value == k }) {
				continue
			}
			label := k
			if k == metamodel.FieldGroupAttr {
				label = "Feldgruppe"
			}
			if i := slices.IndexFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == k }); i >= 0 {
				label = d.Fields[i].Label
			}
			out = append(out, metamodel.Option{Value: k, Label: label})
		}
	}
	if !strings.ContainsAny(object, "*?") {
		collect(c.def(object))
		return out
	}
	for _, o := range c.objects() {
		if ok, _ := path.Match(object, o.Object); ok {
			collect(c.def(o.Object))
		}
	}
	return out
}

// fieldDef: Felddefinition eines Berechtigungsfelds (für Auswahlwerte).
func (c *catalogView) fieldDef(object, field string) *metamodel.FieldDefinition {
	d := c.def(object)
	if d == nil {
		return nil
	}
	if field == metamodel.FieldGroupAttr && d.Authorization != nil {
		// Werte des Berechtigungsfelds field_group: die Feldgruppen des Objects.
		f := metamodel.FieldDefinition{Key: field, Label: "Feldgruppe", Type: metamodel.TypeSelect}
		for _, g := range d.Authorization.FieldGroups {
			f.Options = append(f.Options, metamodel.Option{Value: g.Key, Label: g.Label})
		}
		return &f
	}
	if i := slices.IndexFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == field }); i >= 0 {
		return &d.Fields[i]
	}
	return nil
}

func optionLabel(opts []metamodel.Option, v string) string {
	if i := slices.IndexFunc(opts, func(o metamodel.Option) bool { return o.Value == v }); i >= 0 {
		return opts[i].Label
	}
	return v
}

// --- RoleAuth --------------------------------------------------------------------

func (p *Plugin) grantRecord(c *catalogView, g grant, roleNames map[string]string) map[string]any {
	labels := map[string]any{"role_id": roleNames[g.RoleID]}
	if d := c.def(g.Object); d != nil {
		labels["object"] = d.Title + " (" + g.Object + ")"
	}
	return map[string]any{
		"id": g.ID, "role_id": g.RoleID, "object": g.Object, "action": g.Action,
		"company_codes": strings.Join(g.CompanyCodes, ", "), "restrictions": g.restrictions(), "active": g.Active,
		"_labels": labels,
	}
}

func (p *Plugin) roleNames(ctx context.Context) (map[string]string, error) {
	roles, err := p.listRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range roles {
		out[r.ID] = r.Name
	}
	return out, nil
}

// query liest Filter aus {"query": {...}} oder direkt aus dem Payload.
func query(payload any) map[string]string {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		m = q
	}
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func (p *Plugin) roleAuthList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	gs, err := p.listGrants(ctx, q["role_id"], q["object"], includeHistory(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	names, err := p.roleNames(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	items := make([]any, len(gs))
	for i, g := range gs {
		items[i] = p.grantRecord(c, g, names)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) roleAuthGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	g, err := p.getGrant(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	names, err := p.roleNames(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: p.grantRecord(p.catalog(ctx), *g, names)}, nil
}

func (p *Plugin) roleAuthSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			RoleID       string `json:"role_id"`
			Object       string `json:"object"`
			Action       string `json:"action"`
			CompanyCodes string `json:"company_codes"`
			Active       *bool  `json:"active"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	d := in.Data
	g := grant{ID: in.ID, RoleID: strings.TrimSpace(d.RoleID), Object: strings.TrimSpace(d.Object), Action: strings.TrimSpace(d.Action),
		Active: d.Active == nil || *d.Active}
	if err := checkObjectAction(g.Object, g.Action); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	ccs, err := parseCompanyCodes(d.CompanyCodes)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Buchungskreise: %v", sdk.ErrInvalidArgument, err)
	}
	g.CompanyCodes = ccs
	if create {
		g.ID = newID()
	} else if g.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := p.getRole(ctx, tx, g.RoleID); err != nil {
			return fmt.Errorf("%w: Rolle %q gibt es nicht", sdk.ErrInvalidArgument, g.RoleID)
		}
		if create {
			return p.insertGrant(ctx, tx, g)
		}
		return p.updateGrant(ctx, tx, g)
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.roleAuthGet(ctx, map[string]any{"id": g.ID})
}

func (p *Plugin) roleAuthDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__role_auth SET active = 0 WHERE id = ?`), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: Berechtigung %q", sdk.ErrNotFound, id)
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.roleAuthGet(ctx, payload)
}

// roleAuthFormState: Objects aus dem Catalog, Actions des gewählten Objects.
func (p *Plugin) roleAuthFormState(ctx context.Context, payload any) (sdk.Response, error) {
	var in metamodel.FormStateRequest
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	object := in.Values["object"]
	objects := []metamodel.Option{{Value: "*", Label: "* – alle Objects"}}
	for _, o := range c.objects() {
		label := o.Object
		if o.Title != "" {
			label = o.Title + " (" + o.Object + ")"
		}
		objects = append(objects, metamodel.Option{Value: o.Object, Label: label})
	}
	actions := []metamodel.Option{{Value: "*", Label: "* – alle Actions"}}
	if object != "" && !strings.ContainsAny(object, "*?") {
		actions = append(actions, c.actions(object)...)
	}
	// Werte, die es im Catalog (noch) nicht gibt – z. B. Muster oder ein
	// gestopptes Plugin – bleiben wählbar.
	for _, x := range []struct {
		opts *[]metamodel.Option
		v    string
	}{{&objects, object}, {&actions, in.Values["action"]}} {
		if x.v != "" && !slices.ContainsFunc(*x.opts, func(o metamodel.Option) bool { return o.Value == x.v }) {
			*x.opts = append(*x.opts, metamodel.Option{Value: x.v, Label: x.v})
		}
	}
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{
		"object": {Options: objects},
		"action": {Options: actions},
	}}
	if in.Mode == "create" {
		st.Fields["active"] = hiddenField // neu ist immer aktiv
	}
	if in.Mode == "create" && in.Values["company_codes"] == "" {
		all := AllCompanyCodes
		st.Fields["company_codes"] = metamodel.FieldState{Value: &all}
	}
	var hints []string
	if ccs, err := p.listCompanyCodes(ctx); err == nil && len(ccs) > 0 {
		codes := make([]string, len(ccs))
		for i, cc := range ccs {
			codes[i] = cc.ID
		}
		hints = append(hints, "Buchungskreise: "+strings.Join(codes, ", ")+".")
	}
	if fields := c.authFields(object); len(fields) > 0 {
		labels := make([]string, len(fields))
		for i, f := range fields {
			labels[i] = f.Label
		}
		hints = append(hints, "Berechtigungsfelder: "+strings.Join(labels, ", ")+" – Werte nach dem Speichern unter „Feldwerte“ einschränken.")
	}
	st.Message = strings.Join(hints, " ")
	return sdk.Response{Payload: st}, nil
}

// --- RoleAuthValue ----------------------------------------------------------------

func (p *Plugin) valueRecord(c *catalogView, v authValue, g *grant) map[string]any {
	labels := map[string]any{}
	if g != nil {
		labels["auth_id"] = g.Object + "." + g.Action
		labels["field"] = optionLabel(c.authFields(g.Object), v.Field)
	}
	return map[string]any{"id": v.ID, "auth_id": v.AuthID, "field": v.Field, "low": v.Low, "high": v.High, "active": v.Active, "_labels": labels}
}

func (p *Plugin) roleAuthValueList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	vs, err := p.listValues(ctx, q["auth_id"], includeHistory(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	grants := map[string]*grant{}
	items := make([]any, len(vs))
	for i, v := range vs {
		g, ok := grants[v.AuthID]
		if !ok {
			g, _ = p.getGrant(ctx, p.pool(), v.AuthID)
			grants[v.AuthID] = g
		}
		items[i] = p.valueRecord(c, v, g)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) roleAuthValueGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	v, err := p.getValue(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	g, _ := p.getGrant(ctx, p.pool(), v.AuthID)
	return sdk.Response{Payload: p.valueRecord(p.catalog(ctx), *v, g)}, nil
}

func (p *Plugin) roleAuthValueSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			AuthID string `json:"auth_id"`
			Field  string `json:"field"`
			Low    string `json:"low"`
			High   string `json:"high"`
			Active *bool  `json:"active"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	d := in.Data
	v := authValue{ID: in.ID, AuthID: strings.TrimSpace(d.AuthID), Field: strings.TrimSpace(d.Field),
		Low: strings.TrimSpace(d.Low), High: strings.TrimSpace(d.High), Active: d.Active == nil || *d.Active}
	if err := checkValue(v, v.High != ""); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	g, err := p.getGrant(ctx, p.pool(), v.AuthID)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Berechtigung %q gibt es nicht", sdk.ErrInvalidArgument, v.AuthID)
	}
	// Kennt der Catalog Berechtigungsfelder, sind nur diese erlaubt.
	if fields := p.catalog(ctx).authFields(g.Object); len(fields) > 0 &&
		!slices.ContainsFunc(fields, func(o metamodel.Option) bool { return o.Value == v.Field }) {
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Value
		}
		return sdk.Response{}, fmt.Errorf("%w: %s hat die Berechtigungsfelder %s", sdk.ErrInvalidArgument, g.Object, strings.Join(names, ", "))
	}
	if create {
		v.ID = newID()
	} else if v.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		if create {
			return p.insertValue(ctx, tx, v)
		}
		return p.updateValue(ctx, tx, v)
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.roleAuthValueGet(ctx, map[string]any{"id": v.ID})
}

func (p *Plugin) roleAuthValueDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__role_auth_value SET active = 0 WHERE id = ?`), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: Feldwert %q", sdk.ErrNotFound, id)
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.roleAuthValueGet(ctx, payload)
}

// roleAuthValueFormState: Felder aus metamodel.Authorization des Objects;
// ist das Feld eine Auswahl, werden dessen Werte angeboten.
func (p *Plugin) roleAuthValueFormState(ctx context.Context, payload any) (sdk.Response, error) {
	var in metamodel.FormStateRequest
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	authID := in.Values["auth_id"]
	if authID == "" && in.ID != "" {
		if v, err := p.getValue(ctx, p.pool(), in.ID); err == nil {
			authID = v.AuthID
		}
	}
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if in.Mode == "create" {
		st.Fields["active"] = hiddenField
	}
	g, err := p.getGrant(ctx, p.pool(), authID)
	if err != nil {
		st.Message = "Zuerst die Berechtigung wählen."
		st.Fields["field"] = metamodel.FieldState{Options: []metamodel.Option{}}
		return sdk.Response{Payload: st}, nil
	}
	c := p.catalog(ctx)
	fields := c.authFields(g.Object)
	field := in.Values["field"]
	if field == "" && len(fields) == 1 {
		field = fields[0].Value // einziges Berechtigungsfeld vorbelegen
	}
	if field != "" && !slices.ContainsFunc(fields, func(o metamodel.Option) bool { return o.Value == field }) {
		fields = append(fields, metamodel.Option{Value: field, Label: field})
	}
	st.Fields["field"] = metamodel.FieldState{Options: fields, Value: &field}
	if len(fields) == 0 {
		st.Message = fmt.Sprintf("%s deklariert keine Berechtigungsfelder (metamodel.Authorization).", g.Object)
	} else {
		st.Message = fmt.Sprintf("%s.%s: Einzelwert, Muster (z. B. 4*) oder Bereich von–bis. Mehrere Werte eines Felds ergänzen sich.", g.Object, g.Action)
	}
	if f := c.fieldDef(g.Object, field); f != nil && len(f.Options) > 0 {
		hint := make([]string, len(f.Options))
		for i, o := range f.Options {
			hint[i] = o.Value + " = " + o.Label
		}
		st.Message += " Werte: " + strings.Join(hint, ", ") + "."
	}
	return sdk.Response{Payload: st}, nil
}
