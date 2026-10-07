package iam

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

// Darstellungsregeln (Administration → Darstellung): Je Object legt der
// Administrator Regeln an, die – optional nur für bestimmte Rollen und nur,
// wenn alle Bedingungen auf Feldwerte zutreffen – Felder ausblenden oder
// unänderbar machen. Beispiel: JournalEntryItem, Bedingung source_module =
// RENT, Felder der SD-Kontierung ausblenden.
//
// Das ist reine Darstellung: Der WebServer wendet die Regeln als letzte Schicht
// an (nach Metamodell, FormState und Feldberechtigungen) und kann damit nur
// weiter einschränken. Über die API bleiben die Felder sichtbar; geschützt wird
// über Berechtigungen (crud.Access).
//
// Account.Display {object} liefert die Regeln, die für den aufrufenden Benutzer
// gelten; ausgewertet wird je Datensatz bzw. Formularstand im WebServer.

const (
	modeHidden   = "hidden"
	modeReadonly = "readonly"
)

var objectNameRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

type displayRule struct {
	ID, Object, Name, Roles string
	Active                  bool
	Conds                   []displayCond
	Fields                  []displayField
}

type displayCond struct {
	ID, RuleID, Field, Values string
	Active                    bool
}

type displayField struct {
	ID, RuleID, Field, Mode string
	Active                  bool
}

// splitList: "a, b,,c" → [a b c].
func splitList(text string) []string {
	var out []string
	for _, v := range strings.Split(text, ",") {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// --- Speicherung -------------------------------------------------------------------

// loadRules lädt Regeln aus "iam__display_rule r <rest>" samt Bedingungen und Feldern.
func (p *Plugin) loadRules(ctx context.Context, q database.Querier, rest string, args ...any) ([]displayRule, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT r.id, r.object, r.name, r.roles, r.active FROM iam__display_rule r `+rest+` ORDER BY r.object, r.name, r.id`), args...)
	if err != nil {
		return nil, err
	}
	out := make([]displayRule, len(res.Rows))
	index := map[string]int{}
	for i, r := range res.Rows {
		out[i] = displayRule{ID: s(r[0]), Object: s(r[1]), Name: s(r[2]), Roles: s(r[3]), Active: b(r[4])}
		index[out[i].ID] = i
	}
	if len(out) == 0 {
		return out, nil
	}
	sub := `SELECT r.id FROM iam__display_rule r ` + rest
	conds, err := database.Query(ctx, q, p.q(`SELECT id, rule_id, field, field_values, active FROM iam__display_rule_cond
		WHERE rule_id IN (`+sub+`) ORDER BY created_at, id`), args...)
	if err != nil {
		return nil, err
	}
	for _, r := range conds.Rows {
		c := displayCond{ID: s(r[0]), RuleID: s(r[1]), Field: s(r[2]), Values: s(r[3]), Active: b(r[4])}
		if i, ok := index[c.RuleID]; ok {
			out[i].Conds = append(out[i].Conds, c)
		}
	}
	fields, err := database.Query(ctx, q, p.q(`SELECT id, rule_id, field, mode, active FROM iam__display_rule_field
		WHERE rule_id IN (`+sub+`) ORDER BY created_at, id`), args...)
	if err != nil {
		return nil, err
	}
	for _, r := range fields.Rows {
		f := displayField{ID: s(r[0]), RuleID: s(r[1]), Field: s(r[2]), Mode: s(r[3]), Active: b(r[4])}
		if i, ok := index[f.RuleID]; ok {
			out[i].Fields = append(out[i].Fields, f)
		}
	}
	return out, nil
}

func (p *Plugin) getRule(ctx context.Context, q database.Querier, id string) (*displayRule, error) {
	rs, err := p.loadRules(ctx, q, "WHERE r.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, fmt.Errorf("%w: Darstellungsregel %q", sdk.ErrNotFound, id)
	}
	return &rs[0], nil
}

// childRow: Bedingung oder Feld einer Regel (gleicher Aufbau, eigene Tabelle).
type childRow struct {
	ID, RuleID, Field, Value string // Value: field_values bzw. mode
	Active                   bool
}

type childTable struct {
	table, valueCol, title string
}

var (
	condTable  = childTable{"iam__display_rule_cond", "field_values", "Bedingung"}
	fieldTable = childTable{"iam__display_rule_field", "mode", "Feld"}
)

func (p *Plugin) getChild(ctx context.Context, t childTable, id string) (*childRow, error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT id, rule_id, field, `+t.valueCol+`, active FROM `+t.table+` WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: %s %q", sdk.ErrNotFound, t.title, id)
	}
	r := res.Rows[0]
	return &childRow{ID: s(r[0]), RuleID: s(r[1]), Field: s(r[2]), Value: s(r[3]), Active: b(r[4])}, nil
}

func (p *Plugin) listChildren(ctx context.Context, t childTable, ruleID string, all bool) ([]childRow, error) {
	var conds []string
	var args []any
	if ruleID != "" {
		conds, args = append(conds, "rule_id = ?"), append(args, ruleID)
	}
	if !all {
		conds = append(conds, "active = 1")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT id, rule_id, field, `+t.valueCol+`, active FROM `+t.table+where+` ORDER BY created_at, id`), args...)
	if err != nil {
		return nil, err
	}
	out := make([]childRow, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = childRow{ID: s(r[0]), RuleID: s(r[1]), Field: s(r[2]), Value: s(r[3]), Active: b(r[4])}
	}
	return out, nil
}

func (p *Plugin) saveChild(ctx context.Context, t childTable, c childRow, create bool) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		if create {
			_, err := tx.ExecContext(ctx, p.q(`INSERT INTO `+t.table+` (id, rule_id, field, `+t.valueCol+`, active, created_at) VALUES (?, ?, ?, ?, ?, ?)`),
				c.ID, c.RuleID, c.Field, c.Value, boolInt(c.Active), createdAt())
			return err
		}
		res, err := tx.ExecContext(ctx, p.q(`UPDATE `+t.table+` SET rule_id = ?, field = ?, `+t.valueCol+` = ?, active = ? WHERE id = ?`),
			c.RuleID, c.Field, c.Value, boolInt(c.Active), c.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: %s %q", sdk.ErrNotFound, t.title, c.ID)
		}
		return nil
	})
}

func (p *Plugin) deactivateRow(ctx context.Context, table, title, id string) error {
	return p.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, p.q(`UPDATE `+table+` SET active = 0 WHERE id = ?`), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: %s %q", sdk.ErrNotFound, title, id)
		}
		return nil
	})
}

// --- Darstellung der Datensätze -------------------------------------------------

// fieldLabels: Feld → Bezeichnung aus der Definition des Objects.
func fieldLabels(d *metamodel.ObjectDefinition) map[string]string {
	out := map[string]string{}
	if d != nil {
		for _, f := range d.Fields {
			out[f.Key] = f.Label
		}
	}
	return out
}

func labelOr(labels map[string]string, k string) string {
	if l := labels[k]; l != "" {
		return l
	}
	return k
}

// summary: "wenn Herkunft = RENT: ausblenden Kunde, Auftrag · unänderbar Text".
func (r displayRule) summary(labels map[string]string) string {
	var conds []string
	for _, c := range r.Conds {
		if c.Active {
			conds = append(conds, labelOr(labels, c.Field)+" = "+strings.Join(splitList(c.Values), " | "))
		}
	}
	var hidden, readonly []string
	for _, f := range r.Fields {
		switch {
		case !f.Active:
		case f.Mode == modeHidden:
			hidden = append(hidden, labelOr(labels, f.Field))
		case f.Mode == modeReadonly:
			readonly = append(readonly, labelOr(labels, f.Field))
		}
	}
	var effects []string
	if len(hidden) > 0 {
		effects = append(effects, "ausblenden "+strings.Join(hidden, ", "))
	}
	if len(readonly) > 0 {
		effects = append(effects, "unänderbar "+strings.Join(readonly, ", "))
	}
	when := "immer"
	if len(conds) > 0 {
		when = "wenn " + strings.Join(conds, " und ")
	}
	if len(effects) == 0 {
		return when + ": (noch keine Felder)"
	}
	return when + ": " + strings.Join(effects, " · ")
}

func (p *Plugin) ruleRecord(c *catalogView, r displayRule) map[string]any {
	d := c.def(r.Object)
	labels := map[string]any{}
	if d != nil {
		labels["object"] = d.Title + " (" + r.Object + ")"
	}
	return map[string]any{"id": r.ID, "name": r.Name, "object": r.Object, "roles": r.Roles, "active": r.Active,
		"summary": r.summary(fieldLabels(d)), "_labels": labels}
}

func (p *Plugin) childRecord(c *catalogView, t childTable, row childRow, rule *displayRule) map[string]any {
	labels := map[string]any{}
	if rule != nil {
		labels["rule_id"] = rule.Name
		labels["field"] = labelOr(fieldLabels(c.def(rule.Object)), row.Field)
	}
	return map[string]any{"id": row.ID, "rule_id": row.RuleID, "field": row.Field, t.valueCol: row.Value, "active": row.Active, "_labels": labels}
}

// --- Handler: DisplayRule ----------------------------------------------------------

func (p *Plugin) displayRuleList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	var conds []string
	var args []any
	if o := q["object"]; o != "" {
		conds, args = append(conds, "r.object = ?"), append(args, o)
	}
	if !includeHistory(payload) {
		conds = append(conds, "r.active = 1")
	}
	rest := ""
	if len(conds) > 0 {
		rest = "WHERE " + strings.Join(conds, " AND ")
	}
	rs, err := p.loadRules(ctx, p.pool(), rest, args...)
	if err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	items := make([]any, len(rs))
	for i, r := range rs {
		items[i] = p.ruleRecord(c, r)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) displayRuleGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	r, err := p.getRule(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: p.ruleRecord(p.catalog(ctx), *r)}, nil
}

func (p *Plugin) displayRuleSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			Name   string `json:"name"`
			Object string `json:"object"`
			Roles  string `json:"roles"`
			Active *bool  `json:"active"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	r := displayRule{ID: in.ID, Name: strings.TrimSpace(in.Data.Name), Object: strings.TrimSpace(in.Data.Object),
		Roles: strings.Join(splitList(in.Data.Roles), ", "), Active: in.Data.Active == nil || *in.Data.Active}
	switch {
	case r.Name == "":
		return sdk.Response{}, fmt.Errorf("%w: Bezeichnung ist Pflicht", sdk.ErrInvalidArgument)
	case !objectNameRe.MatchString(r.Object):
		return sdk.Response{}, fmt.Errorf("%w: Object %q: PascalCase erwartet (keine Platzhalter)", sdk.ErrInvalidArgument, r.Object)
	}
	for _, name := range splitList(r.Roles) {
		res, err := database.Query(ctx, p.pool(), p.q(`SELECT 1 FROM iam__roles WHERE name = ?`), name)
		if err != nil {
			return sdk.Response{}, err
		}
		if len(res.Rows) == 0 {
			return sdk.Response{}, fmt.Errorf("%w: Rolle %q gibt es nicht", sdk.ErrInvalidArgument, name)
		}
	}
	if create {
		r.ID = newID()
	} else if r.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	err := p.inTx(ctx, func(tx *sql.Tx) error {
		if create {
			_, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__display_rule (id, object, name, roles, active, created_at) VALUES (?, ?, ?, ?, ?, ?)`),
				r.ID, r.Object, r.Name, nullable(r.Roles), boolInt(r.Active), createdAt())
			return err
		}
		res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__display_rule SET object = ?, name = ?, roles = ?, active = ? WHERE id = ?`),
			r.Object, r.Name, nullable(r.Roles), boolInt(r.Active), r.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: Darstellungsregel %q", sdk.ErrNotFound, r.ID)
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.displayRuleGet(ctx, map[string]any{"id": r.ID})
}

func (p *Plugin) displayRuleDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := p.deactivateRow(ctx, "iam__display_rule", "Darstellungsregel", id); err != nil {
		return sdk.Response{}, err
	}
	return p.displayRuleGet(ctx, payload)
}

// displayRuleFormState: Objects mit Metamodell aus dem Catalog, Rollen als Hinweis.
func (p *Plugin) displayRuleFormState(ctx context.Context, payload any) (sdk.Response, error) {
	var in metamodel.FormStateRequest
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	var objects []metamodel.Option
	for _, o := range c.objects() {
		if d := c.def(o.Object); d != nil {
			objects = append(objects, metamodel.Option{Value: o.Object, Label: d.Title + " (" + o.Object + ")"})
		}
	}
	if v := in.Values["object"]; v != "" && !slices.ContainsFunc(objects, func(o metamodel.Option) bool { return o.Value == v }) {
		objects = append(objects, metamodel.Option{Value: v, Label: v})
	}
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{"object": {Options: objects}}}
	if in.Mode == "create" {
		st.Fields["active"] = hiddenField
	}
	if roles, err := p.listRoles(ctx); err == nil {
		names := make([]string, len(roles))
		for i, r := range roles {
			names[i] = r.Name
		}
		st.Message = "Rollen: " + strings.Join(names, ", ") + ". Bedingungen und Felder nach dem Speichern in den Abschnitten unten."
	}
	return sdk.Response{Payload: st}, nil
}

// --- Handler: Bedingungen und Felder ------------------------------------------------

func (p *Plugin) childList(ctx context.Context, t childTable, payload any) (sdk.Response, error) {
	rows, err := p.listChildren(ctx, t, query(payload)["rule_id"], includeHistory(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	c := p.catalog(ctx)
	rules := map[string]*displayRule{}
	items := make([]any, len(rows))
	for i, row := range rows {
		r, ok := rules[row.RuleID]
		if !ok {
			r, _ = p.getRule(ctx, p.pool(), row.RuleID)
			rules[row.RuleID] = r
		}
		items[i] = p.childRecord(c, t, row, r)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) childGet(ctx context.Context, t childTable, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	row, err := p.getChild(ctx, t, id)
	if err != nil {
		return sdk.Response{}, err
	}
	r, _ := p.getRule(ctx, p.pool(), row.RuleID)
	return sdk.Response{Payload: p.childRecord(p.catalog(ctx), t, *row, r)}, nil
}

func (p *Plugin) childSave(ctx context.Context, t childTable, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	str := func(k string) string { v, _ := in.Data[k].(string); return strings.TrimSpace(v) }
	row := childRow{ID: in.ID, RuleID: str("rule_id"), Field: str("field"), Value: str(t.valueCol), Active: true}
	if a, ok := in.Data["active"].(bool); ok {
		row.Active = a
	}
	rule, err := p.getRule(ctx, p.pool(), row.RuleID)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Darstellungsregel %q gibt es nicht", sdk.ErrInvalidArgument, row.RuleID)
	}
	if !fieldRe.MatchString(row.Field) {
		return sdk.Response{}, fmt.Errorf("%w: Feld fehlt", sdk.ErrInvalidArgument)
	}
	d := p.catalog(ctx).def(rule.Object)
	var f *metamodel.FieldDefinition
	if d != nil {
		i := slices.IndexFunc(d.Fields, func(x metamodel.FieldDefinition) bool { return x.Key == row.Field })
		if i < 0 {
			return sdk.Response{}, fmt.Errorf("%w: %s hat kein Feld %q", sdk.ErrInvalidArgument, rule.Object, row.Field)
		}
		f = &d.Fields[i]
	}
	switch t {
	case condTable:
		vals := splitList(row.Value)
		if len(vals) == 0 {
			return sdk.Response{}, fmt.Errorf("%w: mindestens ein Wert", sdk.ErrInvalidArgument)
		}
		row.Value = strings.Join(vals, ", ")
	case fieldTable:
		switch {
		case row.Value != modeHidden && row.Value != modeReadonly:
			return sdk.Response{}, fmt.Errorf("%w: Darstellung: ausblenden oder unänderbar", sdk.ErrInvalidArgument)
		case row.Value == modeHidden && f != nil && f.Required:
			return sdk.Response{}, fmt.Errorf("%w: %s ist ein Pflichtfeld und kann nicht ausgeblendet werden – „unänderbar“ verwenden", sdk.ErrInvalidArgument, f.Label)
		}
	}
	if create {
		row.ID = newID()
	} else if row.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	if err := p.saveChild(ctx, t, row, create); err != nil {
		return sdk.Response{}, err
	}
	return p.childGet(ctx, t, map[string]any{"id": row.ID})
}

func (p *Plugin) childDeactivate(ctx context.Context, t childTable, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := p.deactivateRow(ctx, t.table, t.title, id); err != nil {
		return sdk.Response{}, err
	}
	return p.childGet(ctx, t, payload)
}

// childFormState: Felder aus der Definition des Objects der Regel; bei
// Bedingungen die Auswahlwerte des Felds als Hinweis.
func (p *Plugin) childFormState(ctx context.Context, t childTable, payload any) (sdk.Response, error) {
	var in metamodel.FormStateRequest
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if in.Mode == "create" {
		st.Fields["active"] = hiddenField
	}
	ruleID := in.Values["rule_id"]
	if ruleID == "" && in.ID != "" {
		if row, err := p.getChild(ctx, t, in.ID); err == nil {
			ruleID = row.RuleID
		}
	}
	rule, err := p.getRule(ctx, p.pool(), ruleID)
	if err != nil {
		st.Message = "Zuerst die Darstellungsregel wählen."
		st.Fields["field"] = metamodel.FieldState{Options: []metamodel.Option{}}
		return sdk.Response{Payload: st}, nil
	}
	d := p.catalog(ctx).def(rule.Object)
	var opts []metamodel.Option
	if d != nil {
		for _, f := range d.Fields {
			if t == fieldTable && in.Values["mode"] == modeHidden && f.Required {
				continue // Pflichtfelder lassen sich nicht ausblenden
			}
			opts = append(opts, metamodel.Option{Value: f.Key, Label: f.Label + " (" + f.Key + ")"})
		}
	}
	field := in.Values["field"]
	if field != "" && !slices.ContainsFunc(opts, func(o metamodel.Option) bool { return o.Value == field }) {
		opts = append(opts, metamodel.Option{Value: field, Label: field})
	}
	st.Fields["field"] = metamodel.FieldState{Options: opts}
	if d == nil {
		st.Message = fmt.Sprintf("Für %s liefert der Catalog (noch) keine Definition.", rule.Object)
		return sdk.Response{Payload: st}, nil
	}
	st.Message = fmt.Sprintf("Regel „%s“ für %s.", rule.Name, d.Title)
	if t == condTable && field != "" {
		if i := slices.IndexFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == field }); i >= 0 && len(d.Fields[i].Options) > 0 {
			hint := make([]string, len(d.Fields[i].Options))
			for j, o := range d.Fields[i].Options {
				hint[j] = o.Value + " = " + o.Label
			}
			st.Message += " Werte: " + strings.Join(hint, ", ") + "."
		}
	}
	return sdk.Response{Payload: st}, nil
}

// --- Account.Display ---------------------------------------------------------------

// DisplayRuleSet: Regeln eines Objects für den aufrufenden Benutzer.
type DisplayRuleSet struct {
	Rules []DisplayRuleView `json:"rules"`
}

type DisplayRuleView struct {
	Conditions []DisplayCondition `json:"conditions,omitempty"`
	Hidden     []string           `json:"hidden,omitempty"`
	Readonly   []string           `json:"readonly,omitempty"`
}

type DisplayCondition struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

// display: aktive Regeln des Objects, deren Rollen der Benutzer hat (ohne
// Rollen: alle). System-Anfragen ohne Benutzer erhalten alle Regeln.
func (p *Plugin) display(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		Object string `json:"object"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if in.Object == "" {
		return sdk.Response{}, fmt.Errorf("%w: object fehlt", sdk.ErrInvalidArgument)
	}
	rs, err := p.loadRules(ctx, p.pool(), "WHERE r.object = ? AND r.active = 1", in.Object)
	if err != nil {
		return sdk.Response{}, err
	}
	var mine []string
	uid := sdk.CallFromContext(ctx).UserID
	if uid != "" && len(rs) > 0 {
		roles, err := p.roleNamesByUser(ctx, p.pool(), uid)
		if err != nil {
			return sdk.Response{}, err
		}
		mine = roles[uid]
	}
	out := DisplayRuleSet{Rules: []DisplayRuleView{}}
	for _, r := range rs {
		if roles := splitList(r.Roles); uid != "" && len(roles) > 0 &&
			!slices.ContainsFunc(roles, func(n string) bool { return slices.Contains(mine, n) }) {
			continue
		}
		v := DisplayRuleView{}
		for _, c := range r.Conds {
			if c.Active {
				v.Conditions = append(v.Conditions, DisplayCondition{Field: c.Field, Values: splitList(c.Values)})
			}
		}
		for _, f := range r.Fields {
			switch {
			case !f.Active:
			case f.Mode == modeHidden:
				v.Hidden = append(v.Hidden, f.Field)
			case f.Mode == modeReadonly:
				v.Readonly = append(v.Readonly, f.Field)
			}
		}
		if len(v.Hidden)+len(v.Readonly) > 0 {
			out.Rules = append(out.Rules, v)
		}
	}
	return sdk.Response{Payload: out}, nil
}
