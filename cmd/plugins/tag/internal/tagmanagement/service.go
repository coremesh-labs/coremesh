package tagmanagement

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/crud"
	"github.com/camel/coremesh/pkg/sdk/tagservice"
)

// TagService: die einzige Schnittstelle für Werte (Object Tags, Payloads und
// Antworten aus pkg/sdk/tagservice).
//
//	Tags.schema   {entity_type, entity_id?|attributes?, company_code?, effective_date?, locale?} → Schema
//	Tags.get      {entity_type, entity_id, company_code?, effective_date?, locale?} → EntityTags
//	Tags.set      {entity_type, entity_id, company_code?, valid_from?, values}      → EntityTags
//	Tags.validate wie set, ohne zu schreiben                                         → {violations, state}
//	Tags.history  {entity_type, entity_id, company_code?, tag?}                     → {values}
//	Tags.find     {entity_type, tag, value?, company_code?, effective_date?}        → {entity_ids}
//
// Zugriff: get/set/history prüfen den Datensatz über dessen eigene Action
// (<entity_type>.get – Existenz und Leserecht, auch je Buchungskreis des
// Fachmoduls). Schreiben braucht zusätzlich <entity_type>.update im
// Buchungskreis des Werts bzw. in allen Buchungskreisen für Werte mit "*".
//
// Bedingungen: Eine Zuordnung kann auf Datensätze mit bestimmten Feldwerten
// beschränkt sein (condition_field/condition_values). Maßgeblich sind die Felder
// aus <entity_type>.get (heute gültige Zeitscheibe); Tags.schema ohne Datensatz
// nimmt attributes oder liefert alle Sets samt Bedingung.

// tagInfo ist ein Tag im Schema mit Regeln und Geltungsbereich.
type tagInfo struct {
	item tagservice.SetItem
	set  string
}

type schemaIndex struct {
	schema tagservice.Schema
	tags   map[string]*tagInfo // Tag-Code → Info (Buchungskreis vor "*")
	rules  []tagservice.Rule
}

// effective: Stichtag (Standard heute).
func effective(date string) (string, error) {
	if strings.TrimSpace(date) == "" {
		return crud.Today(), nil
	}
	return crud.ParseDate(date)
}

func companyScopes(cc string) []any {
	if cc == "" || cc == tagservice.AllCompanyCodes {
		return []any{tagservice.AllCompanyCodes}
	}
	return []any{tagservice.AllCompanyCodes, cc}
}

func inClause(n int) string { return strings.TrimSuffix(strings.Repeat("?, ", n), ", ") }

// loadSchema liest die Tag Sets eines Objekttyps zum Stichtag.
// attrs sind die Feldwerte des Datensatzes für Tag Sets mit Bedingung; nil = nicht
// filtern (Schema ohne Datensatz: alle Sets samt Bedingung).
func (m *Module) loadSchema(ctx context.Context, entityType, cc, at, locale string, attrs map[string]string) (*schemaIndex, error) {
	if !objectRe.MatchString(entityType) {
		return nil, crud.Invalid("entity_type %q: Name eines Objects erwartet", entityType)
	}
	scopes := companyScopes(cc)
	args := append([]any{entityType}, scopes...)
	args = append(args, at, at)
	res, err := m.db.Query(ctx, `SELECT a.company_code, s.code, s.name, s.translation_key, a.condition_field, a.condition_values
		FROM tag__tag_set_assignments a JOIN tag__tag_sets s ON s.code = a.tag_set_code AND s.valid_from <= ? AND s.valid_to >= ?
		WHERE a.entity_type = ? AND a.company_code IN (`+inClause(len(scopes))+`) AND a.valid_from <= ? AND a.valid_to >= ?
		ORDER BY CASE WHEN a.company_code = '*' THEN 0 ELSE 1 END, s.code`, append([]any{at, at}, args...)...)
	if err != nil {
		return nil, err
	}
	idx := &schemaIndex{schema: tagservice.Schema{EntityType: entityType, CompanyCode: cc, EffectiveDate: at, Sets: []tagservice.TagSet{}},
		tags: map[string]*tagInfo{}}
	dict := m.dictionary(ctx, locale)
	types := map[string]*tagservice.TagType{}
	for _, r := range res.Rows {
		set := tagservice.TagSet{CompanyCode: crud.Str(r[0]), Code: crud.Str(r[1]), TranslationKey: crud.Str(r[3]),
			Items: []tagservice.SetItem{}, Rules: []tagservice.Rule{}}
		if field := crud.Str(r[4]); field != "" {
			set.Condition = &tagservice.SetCondition{Field: field, Values: splitValues(crud.Str(r[5]))}
			if attrs != nil && !slices.Contains(set.Condition.Values, attrs[field]) {
				continue // Bedingung nicht erfüllt, z. B. Darlehens- statt Mietvertrag
			}
		}
		set.Name = translate(dict, set.TranslationKey, crud.Str(r[2]))
		items, err := m.db.Query(ctx, `SELECT tag_type_code, mandatory, sort_order FROM tag__tag_set_items
			WHERE tag_set_code = ? AND valid_from <= ? AND valid_to >= ? ORDER BY sort_order, tag_type_code`, set.Code, at, at)
		if err != nil {
			return nil, err
		}
		for _, it := range items.Rows {
			code := crud.Str(it[0])
			t, ok := types[code]
			if !ok {
				if t, err = m.loadTypeWithOptions(ctx, code, at, dict); err != nil {
					return nil, err
				}
				types[code] = t
			}
			item := tagservice.SetItem{Tag: *t, Mandatory: crud.AsBool(it[1]), SortOrder: int(toInt(it[2])), Scope: set.CompanyCode}
			set.Items = append(set.Items, item)
			// Gehört ein Tag global und zu einem Buchungskreis, gilt der Buchungskreis.
			if prev, ok := idx.tags[code]; !ok || prev.item.Scope == tagservice.AllCompanyCodes {
				idx.tags[code] = &tagInfo{item: item, set: set.Code}
			}
		}
		rules, err := m.db.Query(ctx, `SELECT rule_type, source_tag, target_tag, condition_value FROM tag__tag_set_rules
			WHERE tag_set_code = ? AND valid_from <= ? AND valid_to >= ? ORDER BY source_tag, target_tag`, set.Code, at, at)
		if err != nil {
			return nil, err
		}
		for _, ru := range rules.Rows {
			rule := tagservice.Rule{Type: tagservice.RuleType(crud.Str(ru[0])), Source: crud.Str(ru[1]), Target: crud.Str(ru[2]), Condition: crud.Str(ru[3])}
			set.Rules = append(set.Rules, rule)
			idx.rules = append(idx.rules, rule)
		}
		idx.schema.Sets = append(idx.schema.Sets, set)
	}
	// Der Geltungsbereich eines Tags steht in allen Sets gleich (Buchungskreis vor "*").
	for i := range idx.schema.Sets {
		for j := range idx.schema.Sets[i].Items {
			it := &idx.schema.Sets[i].Items[j]
			it.Scope = idx.tags[it.Tag.Code].item.Scope
		}
	}
	return idx, nil
}

func (m *Module) loadTypeWithOptions(ctx context.Context, code, at string, dict map[string]string) (*tagservice.TagType, error) {
	t, err := m.loadType(ctx, code)
	if err != nil {
		return nil, err
	}
	t.Name = translate(dict, t.TranslationKey, t.Name)
	if t.ValueMode == tagservice.ModeOptions {
		res, err := m.db.Query(ctx, `SELECT code, label, translation_key, valid_from, valid_to FROM tag__value_options
			WHERE tag_type_code = ? AND valid_from <= ? AND valid_to >= ? ORDER BY sort_order, code`, code, at, at)
		if err != nil {
			return nil, err
		}
		for _, r := range res.Rows {
			from, _ := crud.ParseDate(r[3])
			to, _ := crud.ParseDate(r[4])
			o := tagservice.ValueOption{Code: crud.Str(r[0]), TranslationKey: crud.Str(r[2]), ValidFrom: from, ValidTo: to}
			o.Label = translate(dict, o.TranslationKey, crud.Str(r[1]))
			t.Options = append(t.Options, o)
		}
	}
	return t, nil
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}

// dictionary holt die Übersetzungen aller Module (Catalog.Translations) für
// translation_key von Tags, Tag Sets und Auswahlwerten. Die Schlüssel
// stammen aus den Modulen, die die Tags fachlich nutzen.
func (m *Module) dictionary(ctx context.Context, locale string) map[string]string {
	if locale == "" {
		return nil
	}
	resp, err := m.services.Call(ctx, sdk.ObjectCatalog, "Translations", map[string]any{"locale": locale})
	if err != nil {
		return nil
	}
	var out struct {
		Translations map[string]string `json:"translations"`
	}
	_ = sdk.Decode(resp.Payload, &out)
	return out.Translations
}

func translate(dict map[string]string, key, fallback string) string {
	if v, ok := dict[key]; ok && key != "" && v != "" {
		return v
	}
	return fallback
}

// --- Werte lesen --------------------------------------------------------------

const assignmentCols = `id, company_code, tag_type_code, value_string, value_integer, value_amount, value_currency,
	value_date, value_timestamp, option_code, valid_from, valid_to, value_ref`

func scanAssignment(r []any) tagservice.Assignment {
	s := stored{}
	strp := func(v any) *string {
		if v == nil {
			return nil
		}
		x := crud.Str(v)
		return &x
	}
	s.String, s.Amount, s.Currency, s.Timestamp, s.Option = strp(r[3]), strp(r[5]), strp(r[6]), strp(r[8]), strp(r[9])
	s.Ref = strp(r[12])
	if r[4] != nil {
		i := toInt(r[4])
		s.Integer = &i
	}
	if r[7] != nil {
		d, _ := crud.ParseDate(r[7])
		s.Date = &d
	}
	from, _ := crud.ParseDate(r[10])
	to, _ := crud.ParseDate(r[11])
	return tagservice.Assignment{ID: crud.Str(r[0]), CompanyCode: crud.Str(r[1]), Tag: crud.Str(r[2]), Value: s.value(), ValidFrom: from, ValidTo: to}
}

// currentValues: die am Stichtag gültigen Werte, je Tag aus seinem Geltungsbereich.
func (m *Module) currentValues(ctx context.Context, idx *schemaIndex, entityType, entityID, cc, at string) (map[string]tagservice.Assignment, error) {
	scopes := companyScopes(cc)
	args := append([]any{entityType, entityID}, scopes...)
	args = append(args, at, at)
	res, err := m.db.Query(ctx, `SELECT `+assignmentCols+` FROM tag__tag_assignments
		WHERE target_entity_type = ? AND target_entity_id = ? AND company_code IN (`+inClause(len(scopes))+`)
		AND valid_from <= ? AND valid_to >= ?`, args...)
	if err != nil {
		return nil, err
	}
	out := map[string]tagservice.Assignment{}
	for _, r := range res.Rows {
		a := scanAssignment(r)
		info, ok := idx.tags[a.Tag]
		if !ok || info.item.Scope != a.CompanyCode {
			continue // Tag nicht (mehr) im Schema oder aus einem anderen Geltungsbereich
		}
		if a.Value.Option != nil {
			a.OptionLabel = optionLabel(info.item.Tag, *a.Value.Option)
		}
		if a.Value.Ref != nil {
			a.RefLabel = m.refLabel(ctx, info.item.Tag.RefObject, *a.Value.Ref)
		}
		out[a.Tag] = a
	}
	return out, nil
}

// refLabel: lesbarer Text eines Verweises – TitleField des Ziels (Metamodell),
// sonst die id. Ohne Leserecht auf das Ziel bleibt es bei der id.
func (m *Module) refLabel(ctx context.Context, object, id string) string {
	def, err := m.objectDef(ctx, object)
	if err != nil || def.TitleField == "" {
		return id
	}
	rec, err := m.fetch(ctx, object, id)
	if err != nil {
		return id
	}
	if l := attrText(rec[def.TitleField]); l != "" {
		return l
	}
	return id
}

func optionLabel(t tagservice.TagType, code string) string {
	for _, o := range t.Options {
		if o.Code == code {
			return o.Label
		}
	}
	return code
}

// --- Regeln -------------------------------------------------------------------

// text ist die Textform eines Werts für Regelbedingungen.
func text(v tagservice.Value) string {
	switch {
	case v.Option != nil:
		return *v.Option
	case v.String != nil:
		return *v.String
	case v.Integer != nil:
		return fmt.Sprint(*v.Integer)
	case v.Amount != nil:
		return *v.Amount + " " + crud.Str(deref(v.Currency))
	case v.Date != nil:
		return *v.Date
	case v.Timestamp != nil:
		return *v.Timestamp
	case v.Ref != nil:
		return *v.Ref
	}
	return ""
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// label: Anzeigename eines Tags für Meldungen, z. B. "Letzte Bonitätsprüfung (AUDIT_DATE)".
func (idx *schemaIndex) label(code string) string {
	if info, ok := idx.tags[code]; ok && info.item.Tag.Name != "" {
		return info.item.Tag.Name + " (" + code + ")"
	}
	return code
}

// evaluate wertet Pflicht und Regeln für einen Wertestand aus.
func (idx *schemaIndex) evaluate(values map[string]tagservice.Value) (tagservice.State, []tagservice.Violation) {
	st := tagservice.State{Visible: map[string]bool{}, Required: map[string]bool{}}
	for code, info := range idx.tags {
		st.Visible[code] = true
		st.Required[code] = info.item.Mandatory
	}
	holds := func(r tagservice.Rule) bool {
		v, ok := values[r.Source]
		return ok && (r.Condition == "" || text(v) == r.Condition)
	}
	var out []tagservice.Violation
	for _, r := range idx.rules {
		switch r.Type {
		case tagservice.RuleShowIf:
			if !holds(r) {
				st.Visible[r.Target] = false
			}
		case tagservice.RuleRequires:
			if holds(r) {
				st.Required[r.Target] = true
			}
		case tagservice.RuleExcludes:
			if _, has := values[r.Target]; has && holds(r) {
				out = append(out, tagservice.Violation{Tag: r.Target, Code: "excludes",
					Message: fmt.Sprintf("%s ist nicht erlaubt, wenn %s gesetzt ist", idx.label(r.Target), describe(r))})
			}
		}
	}
	for _, code := range slices.Sorted(maps.Keys(idx.tags)) {
		_, has := values[code]
		switch {
		case !st.Visible[code] && has:
			out = append(out, tagservice.Violation{Tag: code, Code: "hidden", Message: fmt.Sprintf("%s ist hier nicht vorgesehen (Regel SHOW_IF) und muss leer sein", idx.label(code))})
		case st.Visible[code] && st.Required[code] && !has:
			out = append(out, tagservice.Violation{Tag: code, Code: "required", Message: fmt.Sprintf("%s ist Pflicht", idx.label(code))})
		}
		if !st.Visible[code] {
			st.Required[code] = false
		}
	}
	return st, out
}

func describe(r tagservice.Rule) string {
	if r.Condition != "" {
		return r.Source + " = " + r.Condition
	}
	return r.Source
}

// --- Zugriff ------------------------------------------------------------------

// checkEntity: Den Datensatz gibt es und der Benutzer darf ihn lesen – über
// die eigene Action des Fachmoduls. Ergebnis sind die Feldwerte des Datensatzes
// (heute gültige Zeitscheibe) für Tag Sets mit Bedingung.
func (m *Module) checkEntity(ctx context.Context, entityType, entityID string) (map[string]string, error) {
	if entityID == "" {
		return nil, crud.Invalid("entity_id fehlt")
	}
	rec, err := m.fetch(ctx, entityType, entityID)
	if err != nil {
		if errors.Is(err, sdk.ErrUnimplemented) {
			return nil, fmt.Errorf("%w: Objekttyp %s hat keine Action get", sdk.ErrInvalidArgument, entityType)
		}
		return nil, err
	}
	attrs := map[string]string{}
	for k, v := range rec {
		if !strings.HasPrefix(k, "_") {
			attrs[k] = attrText(v)
		}
	}
	return attrs, nil
}

// checkWrite: <entity_type>.update im Buchungskreis des Werts (bzw. überall bei "*").
func (m *Module) checkWrite(ctx context.Context, entityType, scope string) error {
	g, err := sdk.GrantedCompanyCodes(ctx, entityType, "update")
	if err != nil {
		return err
	}
	if g.All || (scope != tagservice.AllCompanyCodes && g.Allows(scope)) {
		return nil
	}
	if scope == tagservice.AllCompanyCodes {
		return fmt.Errorf("%w: Tags für alle Buchungskreise brauchen %s.update ohne Einschränkung", sdk.ErrPermissionDenied, entityType)
	}
	return fmt.Errorf("%w: keine Berechtigung %s.update im Buchungskreis %s", sdk.ErrPermissionDenied, entityType, scope)
}

// --- Actions ------------------------------------------------------------------

func (m *Module) schemaAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in tagservice.GetRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	at, err := effective(in.EffectiveDate)
	if err != nil {
		return sdk.Response{}, err
	}
	// Bedingungen: Felder des Datensatzes, sonst die mitgegebenen Attribute, sonst alle Sets.
	attrs := in.Attributes
	if in.EntityID != "" {
		if attrs, err = m.checkEntity(ctx, in.EntityType, in.EntityID); err != nil {
			return sdk.Response{}, err
		}
	}
	idx, err := m.loadSchema(ctx, in.EntityType, in.CompanyCode, at, in.Locale, attrs)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: idx.schema}, nil
}

func (m *Module) getAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in tagservice.GetRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	at, err := effective(in.EffectiveDate)
	if err != nil {
		return sdk.Response{}, err
	}
	attrs, err := m.checkEntity(ctx, in.EntityType, in.EntityID)
	if err != nil {
		return sdk.Response{}, err
	}
	out, err := m.entityTags(ctx, in.EntityType, in.EntityID, in.CompanyCode, at, in.Locale, attrs)
	return sdk.Response{Payload: out}, err
}

func (m *Module) entityTags(ctx context.Context, entityType, entityID, cc, at, locale string, attrs map[string]string) (tagservice.EntityTags, error) {
	idx, err := m.loadSchema(ctx, entityType, cc, at, locale, attrs)
	if err != nil {
		return tagservice.EntityTags{}, err
	}
	cur, err := m.currentValues(ctx, idx, entityType, entityID, cc, at)
	if err != nil {
		return tagservice.EntityTags{}, err
	}
	values := map[string]tagservice.Value{}
	out := tagservice.EntityTags{Schema: idx.schema, EntityID: entityID, Values: []tagservice.Assignment{}}
	for _, code := range slices.Sorted(maps.Keys(cur)) {
		out.Values = append(out.Values, cur[code])
		values[code] = cur[code].Value
	}
	out.State, _ = idx.evaluate(values)
	return out, nil
}

// plan ist ein geprüfter Schreibauftrag.
type plan struct {
	idx     *schemaIndex
	state   tagservice.State // Regel-Zustand des geprüften Gesamtstands
	cur     map[string]tagservice.Assignment
	changes map[string]*stored // Tag → neuer Wert (nil = entfernen)
	at      string
}

// prepare prüft einen SetRequest vollständig: Datentypen, Auswahlwerte am
// Stichtag, veraltete Tags, Regeln auf dem Gesamtstand.
func (m *Module) prepare(ctx context.Context, in tagservice.SetRequest, attrs map[string]string) (*plan, []tagservice.Violation, error) {
	at, err := effective(in.ValidFrom)
	if err != nil {
		return nil, nil, err
	}
	idx, err := m.loadSchema(ctx, in.EntityType, in.CompanyCode, at, in.Locale, attrs)
	if err != nil {
		return nil, nil, err
	}
	cur, err := m.currentValues(ctx, idx, in.EntityType, in.EntityID, in.CompanyCode, at)
	if err != nil {
		return nil, nil, err
	}
	p := &plan{idx: idx, cur: cur, changes: map[string]*stored{}, at: at}
	merged := map[string]tagservice.Value{}
	for code, a := range cur {
		merged[code] = a.Value
	}
	var violations []tagservice.Violation
	for _, code := range slices.Sorted(maps.Keys(in.Values)) {
		v := in.Values[code]
		info, ok := idx.tags[code]
		if !ok {
			violations = append(violations, tagservice.Violation{Tag: code, Code: "unknown",
				Message: fmt.Sprintf("Tag %s ist %s am %s nicht zugewiesen", code, in.EntityType, at)})
			continue
		}
		if v == nil {
			if _, has := cur[code]; has {
				p.changes[code] = nil
				delete(merged, code)
			}
			continue
		}
		t := info.item.Tag
		s, err := parseValue(&t, v, false)
		if err != nil {
			violations = append(violations, tagservice.Violation{Tag: code, Code: "type", Message: fmt.Sprintf("%s: %v", idx.label(code), err)})
			continue
		}
		if s.Option != nil && !slices.ContainsFunc(t.Options, func(o tagservice.ValueOption) bool { return o.Code == *s.Option }) {
			violations = append(violations, tagservice.Violation{Tag: code, Code: "option",
				Message: fmt.Sprintf("%s: Auswahlwert %q gibt es am %s nicht", idx.label(code), *s.Option, at)})
			continue
		}
		if s.Ref != nil {
			if _, err := m.fetch(ctx, t.RefObject, *s.Ref); err != nil {
				violations = append(violations, tagservice.Violation{Tag: code, Code: "reference",
					Message: fmt.Sprintf("%s: %s %q nicht gefunden oder nicht lesbar", idx.label(code), t.RefObject, *s.Ref)})
				continue
			}
		}
		if a, has := cur[code]; has && text(a.Value) == text(s.value()) {
			continue // unverändert
		}
		if t.Status != "ACTIVE" {
			violations = append(violations, tagservice.Violation{Tag: code, Code: "deprecated",
				Message: fmt.Sprintf("%s ist veraltet – keine neuen Werte", idx.label(code))})
			continue
		}
		p.changes[code] = &s
		merged[code] = s.value()
	}
	// Ungültige Werte melden nur ihren eigenen Fehler, nicht zusätzlich "Pflicht".
	failed := map[string]bool{}
	for _, v := range violations {
		failed[v.Tag] = true
	}
	state, ruleViolations := idx.evaluate(merged)
	p.state = state
	for _, v := range ruleViolations {
		if !(v.Code == "required" && failed[v.Tag]) {
			violations = append(violations, v)
		}
	}
	return p, violations, nil
}

func (m *Module) validateAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in tagservice.SetRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	attrs, err := m.checkEntity(ctx, in.EntityType, in.EntityID)
	if err != nil {
		return sdk.Response{}, err
	}
	p, violations, err := m.prepare(ctx, in, attrs)
	if err != nil {
		return sdk.Response{}, err
	}
	if violations == nil {
		violations = []tagservice.Violation{}
	}
	// state: Sichtbarkeit und Pflicht für den geprüften Stand (für Eingabemasken).
	return sdk.Response{Payload: map[string]any{"violations": violations, "state": p.state}}, nil
}

// violationError fasst Verstöße zu einem ErrInvalidArgument zusammen.
func violationError(vs []tagservice.Violation) error {
	msgs := make([]string, len(vs))
	for i, v := range vs {
		msgs[i] = v.Message
	}
	return fmt.Errorf("%w: %s", sdk.ErrInvalidArgument, strings.Join(msgs, "; "))
}

func (m *Module) setAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in tagservice.SetRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	attrs, err := m.checkEntity(ctx, in.EntityType, in.EntityID)
	if err != nil {
		return sdk.Response{}, err
	}
	p, violations, err := m.prepare(ctx, in, attrs)
	if err != nil {
		return sdk.Response{}, err
	}
	if len(violations) > 0 {
		return sdk.Response{}, violationError(violations)
	}
	for code := range p.changes {
		if err := m.checkWrite(ctx, in.EntityType, p.idx.tags[code].item.Scope); err != nil {
			return sdk.Response{}, err
		}
	}
	err = m.db.InTx(ctx, nil, func(ctx context.Context) error {
		for _, code := range slices.Sorted(maps.Keys(p.changes)) {
			if err := m.writeSlice(ctx, in, p, code); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	out, err := m.entityTags(ctx, in.EntityType, in.EntityID, in.CompanyCode, p.at, in.Locale, attrs)
	return sdk.Response{Payload: out}, err
}

// writeSlice schreibt die Zeitscheiben eines Tags ab p.at: Die laufende
// Zeitscheibe endet am Vortag, die neue gilt bis zum bisherigen Ende. Es wird
// nichts gelöscht; künftige Zeitscheiben lassen sich so nicht überschreiben.
func (m *Module) writeSlice(ctx context.Context, in tagservice.SetRequest, p *plan, code string) error {
	scope := p.idx.tags[code].item.Scope
	future, err := m.db.Query(ctx, `SELECT valid_from FROM tag__tag_assignments WHERE target_entity_type = ? AND target_entity_id = ?
		AND company_code = ? AND tag_type_code = ? AND valid_from > ?`, in.EntityType, in.EntityID, scope, code, p.at)
	if err != nil {
		return err
	}
	if len(future.Rows) > 0 {
		from, _ := crud.ParseDate(future.Rows[0][0])
		return crud.Invalid("%s hat ab %s bereits eine künftige Zeitscheibe – Stichtag danach wählen", code, from)
	}
	until := crud.DateMax
	if cur, ok := p.cur[code]; ok {
		until = cur.ValidTo
		if cur.ValidFrom == p.at {
			if p.changes[code] == nil {
				return crud.Invalid("%s: Der Wert gilt erst ab %s – er kann nicht zum selben Tag beendet werden", code, p.at)
			}
			// Gleicher Beginn: Wert der Zeitscheibe ersetzen.
			return m.writeValue(ctx, "UPDATE", cur.ID, in, scope, code, p.changes[code], p.at, until)
		}
		prev, _ := time.Parse(time.DateOnly, p.at)
		if _, err := m.db.Exec(ctx, `UPDATE tag__tag_assignments SET valid_to = ? WHERE id = ? AND valid_from = ?`,
			prev.AddDate(0, 0, -1).Format(time.DateOnly), cur.ID, cur.ValidFrom); err != nil {
			return err
		}
	}
	if p.changes[code] == nil {
		return nil // entfernt: nur beendet
	}
	id := crud.NewID()
	if cur, ok := p.cur[code]; ok {
		id = cur.ID // dieselbe Zuordnung, neue Zeitscheibe
	}
	return m.writeValue(ctx, "INSERT", id, in, scope, code, p.changes[code], p.at, until)
}

func (m *Module) writeValue(ctx context.Context, op, id string, in tagservice.SetRequest, scope, code string, s *stored, from, to string) error {
	vals := []any{deref(s.String), intOrNil(s.Integer), deref(s.Amount), deref(s.Currency), deref(s.Date), deref(s.Timestamp), deref(s.Option), deref(s.Ref),
		time.Now().UTC().Format(time.RFC3339), nilIfEmpty(sdk.CallFromContext(ctx).UserID)}
	if op == "UPDATE" {
		_, err := m.db.Exec(ctx, `UPDATE tag__tag_assignments SET value_string = ?, value_integer = ?, value_amount = ?, value_currency = ?,
			value_date = ?, value_timestamp = ?, option_code = ?, value_ref = ?, changed_at = ?, changed_by = ? WHERE id = ? AND valid_from = ?`,
			append(vals, id, from)...)
		return err
	}
	_, err := m.db.Exec(ctx, `INSERT INTO tag__tag_assignments (value_string, value_integer, value_amount, value_currency, value_date,
		value_timestamp, option_code, value_ref, changed_at, changed_by, id, target_entity_type, target_entity_id, company_code, tag_type_code, valid_from, valid_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append(vals, id, in.EntityType, in.EntityID, scope, code, from, to)...)
	return err
}

func intOrNil(i *int64) any {
	if i == nil {
		return nil
	}
	return *i
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (m *Module) historyAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		tagservice.GetRequest
		Tag string `json:"tag"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if _, err := m.checkEntity(ctx, in.EntityType, in.EntityID); err != nil {
		return sdk.Response{}, err
	}
	scopes := companyScopes(in.CompanyCode)
	args := append([]any{in.EntityType, in.EntityID}, scopes...)
	sql := `SELECT ` + assignmentCols + ` FROM tag__tag_assignments WHERE target_entity_type = ? AND target_entity_id = ?
		AND company_code IN (` + inClause(len(scopes)) + `)`
	if in.Tag != "" {
		sql += " AND tag_type_code = ?"
		args = append(args, in.Tag)
	}
	res, err := m.db.Query(ctx, sql+" ORDER BY tag_type_code, company_code, valid_from", args...)
	if err != nil {
		return sdk.Response{}, err
	}
	values := []tagservice.Assignment{}
	for _, r := range res.Rows {
		values = append(values, scanAssignment(r))
	}
	return sdk.Response{Payload: map[string]any{"values": values}}, nil
}

// findAction liefert die Datensätze mit einem Tag (und Wert) am Stichtag.
// Geprüft wird nur das Recht Tags.find; die IDs selbst sind keine Fachdaten.
func (m *Module) findAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in tagservice.FindRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	at, err := effective(in.EffectiveDate)
	if err != nil {
		return sdk.Response{}, err
	}
	if in.Tag == "" || !objectRe.MatchString(in.EntityType) {
		return sdk.Response{}, crud.Invalid("entity_type und tag sind Pflicht")
	}
	scopes := companyScopes(in.CompanyCode)
	args := append([]any{in.EntityType, in.Tag, at, at}, scopes...)
	sql := `SELECT DISTINCT target_entity_id FROM tag__tag_assignments WHERE target_entity_type = ? AND tag_type_code = ?
		AND valid_from <= ? AND valid_to >= ? AND company_code IN (` + inClause(len(scopes)) + `)`
	if in.Value != nil {
		t, err := m.loadType(ctx, in.Tag)
		if err != nil {
			return sdk.Response{}, err
		}
		s, err := parseValue(t, in.Value, true)
		if err != nil {
			return sdk.Response{}, crud.Invalid("%s: %v", in.Tag, err)
		}
		for col, v := range map[string]any{"value_string": deref(s.String), "value_integer": intOrNil(s.Integer), "value_amount": deref(s.Amount),
			"value_currency": deref(s.Currency), "value_date": deref(s.Date), "value_timestamp": deref(s.Timestamp), "option_code": deref(s.Option), "value_ref": deref(s.Ref)} {
			if v != nil {
				sql, args = sql+" AND "+col+" = ?", append(args, v)
			}
		}
	}
	res, err := m.db.Query(ctx, sql+" ORDER BY target_entity_id", args...)
	if err != nil {
		return sdk.Response{}, err
	}
	ids := []string{}
	for _, r := range res.Rows {
		ids = append(ids, crud.Str(r[0]))
	}
	slices.SortFunc(ids, cmp.Compare[string])
	return sdk.Response{Payload: map[string]any{"entity_ids": ids}}, nil
}
