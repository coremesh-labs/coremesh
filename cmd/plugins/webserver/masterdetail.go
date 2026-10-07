package main

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Detailansicht im Stil von LeanIX und Lookup-Dialoge.
//
// Abschnitte (metamodel.SectionDefinition) erscheinen als aufklappbare
// <details>-Blöcke. Ein Abschnitt mit Relation lädt seine eingebettete
// Tabelle per HTMX nach (GET …/{id}/rel/{section}) und neu, sobald ein
// Formular des Abschnitts gespeichert wurde (Ereignis coremesh-changed).
// Hinzufügen und Bearbeiten nutzen die normalen Formulare des Unter-Objects
// mit festem Fremdschlüssel (_lock) und _view=refresh.
//
// Lookup-Felder (FieldDefinition.Lookup) bekommen eine Schaltfläche, die den
// Auswahldialog lädt (GET /lookup?from=<Object>&field=<Feld>). Der Dialog
// listet das Nachschlage-Object über dessen list-Action, mit Suche.

var keyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// inputNameRe: Name eines Eingabefelds für Lookups ohne Metamodell-Feld, z. B. v.RENTAL_OBJECT.
var inputNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]{0,80}$`)

// sectionView ist ein Abschnitt der Detailansicht.
type sectionView struct {
	Key, Title string
	Collapsed  bool
	Fields     []metamodel.FieldDefinition
	Relation   *metamodel.Relation
	Tags       bool   // Abschnitt mit dem TagEditor (Plugin tag)
	URL        string // nur Relation: Fragment der eingebetteten Tabelle
}

// Sections gliedert die Detailansicht. Felder ohne Abschnitt stehen in
// „Allgemein“ vorne (nur Felder, die der Datensatz liefert). Ohne Abschnitte
// im Metamodell: nil (einfache Liste).
func (v view) Sections() []sectionView {
	d := v.Def
	if len(d.Sections) == 0 {
		return nil
	}
	placed := map[string]bool{}
	for _, s := range d.Sections {
		for _, k := range s.Fields {
			placed[k] = true
		}
	}
	var out []sectionView
	var rest []metamodel.FieldDefinition
	for _, f := range d.Fields {
		// Felder, die der Datensatz gar nicht liefert (z. B. virtuelle Eingabefelder
		// einer custom-Action), gehören nicht in die Anzeige.
		if _, ok := v.Record[f.Key]; !placed[f.Key] && (ok || v.Record == nil) {
			rest = append(rest, f)
		}
	}
	if len(rest) > 0 {
		out = append(out, sectionView{Key: "allgemein", Title: v.T("core.section.general"), Fields: rest})
	}
	for _, s := range d.Sections {
		sv := sectionView{Key: s.Key, Title: s.Title, Collapsed: s.Collapsed, Relation: s.Relation, Tags: s.Tags}
		for _, k := range s.Fields {
			if i := slices.IndexFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == k }); i >= 0 {
				sv.Fields = append(sv.Fields, d.Fields[i])
			}
		}
		if s.Tags {
			// Tags hängen am fachlichen Schlüssel, nicht an der Zeitscheibe.
			sv.URL = tagEditorURL(v.Object, businessKey(v.Record))
		}
		if s.Relation != nil {
			// Unter-Objects verweisen auf den fachlichen Schlüssel, nicht auf die Zeitscheibe.
			sv.URL = v.URL + "/" + pathEscape(businessKey(v.Record)) + "/rel/" + s.Key
		}
		out = append(out, sv)
	}
	return out
}

// RecordTitle ist der Wert des TitleField (Überschrift der Detailansicht).
func (v view) RecordTitle() string {
	if v.Def.TitleField == "" || v.Record == nil {
		return ""
	}
	return scalar(v.Record[v.Def.TitleField])
}

// definition holt das Metamodell eines Objects als objectCtx im Modul der Anfrage.
func (s *server) definition(r *http.Request, object string) (objectCtx, error) {
	resp, err := s.call(r, sdk.ObjectCatalog, "GetDefinition", map[string]any{"object": object})
	if err != nil {
		return objectCtx{}, err
	}
	var def struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
		Available  bool                       `json:"available"`
	}
	if err := sdk.Decode(resp.Payload, &def); err != nil {
		return objectCtx{}, err
	}
	if !def.Available {
		return objectCtx{}, fmt.Errorf("%w: %s", sdk.ErrUnavailable, s.T(r, "core.error.unavailable", def.Definition.Title))
	}
	return s.withLocale(r, newObjectCtx(moduleFrom(r).Name, object, s.localizeDef(r, def.Definition)).visibleFor(userFrom(r))), nil
}

// relationView ist eine eingebettete Tabelle (Fragment "relation").
type relationView struct {
	Child      objectCtx
	Section    sectionView
	ForeignKey string
	Columns    []metamodel.FieldDefinition
	Rows       []record
	AddURL     string            // Formular für ein neues Unter-Object (leer = nicht erlaubt)
	LookupEdit map[string]string // Feld → Basis-URL des Lookup-Ziels im Modul (Bearbeiten-Link)
	Message    string            // statt der Tabelle, z. B. fehlende Berechtigung
	History    bool              // inkl. beendeter / inaktiver Einträge
}

// GET /m/{module}/{object}/{id}/rel/{section}
func (s *server) relation(w http.ResponseWriter, r *http.Request) {
	oc, err := s.loadObject(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	id, key := r.PathValue("id"), r.PathValue("section")
	i := slices.IndexFunc(oc.Def.Sections, func(sd metamodel.SectionDefinition) bool { return sd.Key == key && sd.Relation != nil })
	if i < 0 {
		s.fail(w, r, fmt.Errorf("%w: %s hat keinen Abschnitt %q", sdk.ErrNotFound, oc.Def.Title, key))
		return
	}
	sd := oc.Def.Sections[i]
	rel := sd.Relation
	mod := moduleFrom(r)
	rv := relationView{Section: sectionView{Key: sd.Key, Title: sd.Title, URL: r.URL.Path}, ForeignKey: rel.ForeignKey,
		LookupEdit: map[string]string{}, History: includeHistory(r)}

	if _, ok := mod.object(rel.Object); !ok {
		// Unter-Object läuft nicht oder der Benutzer darf es gar nicht sehen.
		rv.Message = s.T(r, "core.relation.no_access")
		s.render(w, r, http.StatusOK, "relation", rv, "", "")
		return
	}
	child, err := s.definition(r, rel.Object)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rv.Child = child
	act, err := need(child, metamodel.KindList)
	if err != nil {
		rv.Message = s.T(r, "core.relation.no_permission", child.Def.Title)
		s.render(w, r, http.StatusOK, "relation", rv, "", "")
		return
	}
	query := map[string]any{rel.ForeignKey: id}
	if includeHistory(r) {
		query["includeHistory"] = "true"
	}
	resp, err := s.call(r, child.Object, act.Name, map[string]any{"query": query})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if rv.Rows, err = records(resp.Payload); err != nil {
		s.fail(w, r, err)
		return
	}
	rv.Columns = relationColumns(child.Def, rel)
	for _, f := range rv.Columns {
		if f.Lookup == nil {
			continue
		}
		if _, ok := mod.object(f.Lookup.Object); ok && f.Lookup.ValueField == "id" {
			rv.LookupEdit[f.Key] = moduleURL(mod.Name, f.Lookup.Object)
		}
	}
	if child.Has["create"] != nil {
		rv.AddURL = child.URL + "/new?" + rel.ForeignKey + "=" + pathEscape(id) + "&_lock=" + rel.ForeignKey + "&_view=refresh"
	}
	s.render(w, r, http.StatusOK, "relation", rv, "", "")
}

// relationColumns: Columns der Relation, sonst listable Felder ohne Fremdschlüssel.
func relationColumns(d metamodel.ObjectDefinition, rel *metamodel.Relation) []metamodel.FieldDefinition {
	var out []metamodel.FieldDefinition
	if len(rel.Columns) > 0 {
		for _, k := range rel.Columns {
			if i := slices.IndexFunc(d.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == k }); i >= 0 {
				out = append(out, d.Fields[i])
			}
		}
		return out
	}
	for _, f := range d.Fields {
		if f.Listable && f.Key != rel.ForeignKey {
			out = append(out, f)
		}
	}
	return out
}

// --- Lookup-Dialog -------------------------------------------------------------

const lookupLimit = 100

type lookupView struct {
	From, Field, Title, Query string
	Src                       string // Parameter für die Suche (from+field bzw. object+field)
	Columns                   []metamodel.FieldDefinition
	Rows                      []lookupRow
	More                      bool
}

type lookupRow struct {
	Value, Label string
	Cells        []string
}

// GET /lookup?from=<Object>&field=<Feld>[&q=<Suche>][&rows=1]
// GET /lookup?object=<Object>&field=<Eingabefeld>[&q=…][&rows=1]
//
// Die zweite Form sucht einen Datensatz eines beliebigen Objects (Verweis-Tags
// im TagEditor): Übernommen wird die id, angezeigt das TitleField. field ist
// dann der Name des Eingabefelds, z. B. v.RENTAL_OBJECT.
//
// Das Nachschlage-Object kommt aus dem Metamodell des Felds – nicht aus der
// URL. Gelesen wird über dessen list-Action; der Dispatcher prüft die
// Berechtigung. Lookups dürfen Modulgrenzen überschreiten (z. B. Buchungskreise
// aus iam), aber nur lesend über die Actions des Ziels.
func (s *server) lookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, field, object := q.Get("from"), q.Get("field"), q.Get("object")
	var lk *metamodel.Lookup
	srcParams := url.Values{"field": {field}}
	if object != "" {
		if !objectRe.MatchString(object) || !inputNameRe.MatchString(field) {
			s.fail(w, r, fmt.Errorf("%w: object und field erwartet", sdk.ErrInvalidArgument))
			return
		}
		lk = &metamodel.Lookup{Object: object, ValueField: "id"}
		srcParams.Set("object", object)
	} else {
		if !objectRe.MatchString(from) || !keyRe.MatchString(field) {
			s.fail(w, r, fmt.Errorf("%w: from und field erwartet", sdk.ErrInvalidArgument))
			return
		}
		src, err := s.definition(r, from)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		i := slices.IndexFunc(src.Def.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == field && f.Lookup != nil })
		if i < 0 {
			s.fail(w, r, fmt.Errorf("%w: %s.%s ist kein Lookup-Feld", sdk.ErrNotFound, from, field))
			return
		}
		lk = src.Def.Fields[i].Lookup
		srcParams.Set("from", from)
	}
	tgt, err := s.definition(r, lk.Object)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if object != "" && tgt.Def.TitleField != "" {
		lk.LabelFields = []string{tgt.Def.TitleField}
	}
	act, err := need(tgt, metamodel.KindList)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	query := map[string]any{}
	if search != "" {
		query["q"] = search
	}
	// Filter aus dem Formular (Lookup.Filters); die aufgelösten Werte wandern
	// als filter.<feld> mit, damit die Suche im Dialog sie behält.
	for target, src := range lk.Filters {
		v := q.Get("filter." + target)
		if v == "" {
			v = s.filterValue(r, from, src, q)
		}
		if v != "" {
			query[target] = v
			srcParams.Set("filter."+target, v)
		}
	}
	resp, err := s.call(r, tgt.Object, act.Name, map[string]any{"query": query})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recs, err := records(resp.Payload)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	lv := lookupView{From: from, Field: field, Title: tgt.Def.Title, Query: search, Columns: lookupColumns(tgt.Def, lk), Src: srcParams.Encode()}
	needle := strings.ToLower(search)
	for _, rec := range recs {
		row := lookupRow{Value: scalar(rec[lk.ValueField])}
		if row.Value == "" && object != "" {
			row.Value = recordID(rec) // Objects ohne fachliche id
		}
		var labels []string
		for _, k := range lk.LabelFields {
			if v := rec[k]; v != nil && scalar(v) != "" {
				labels = append(labels, scalar(v))
			}
		}
		row.Label = strings.Join(labels, " ")
		if row.Label == "" {
			row.Label = row.Value
		}
		for _, f := range lv.Columns {
			row.Cells = append(row.Cells, displayValue(rec, f))
		}
		// Unterstützt das Ziel keine Suche (q), filtert der WebServer selbst.
		if needle != "" && !strings.Contains(strings.ToLower(strings.Join(row.Cells, " ")+" "+row.Label), needle) {
			continue
		}
		if len(lv.Rows) == lookupLimit {
			lv.More = true
			break
		}
		lv.Rows = append(lv.Rows, row)
	}
	fragment := "lookup"
	if q.Get("rows") == "1" {
		fragment = "lookup-rows"
	}
	s.render(w, r, http.StatusOK, fragment, lv, tgt.Def.Title, "")
}

// lookupColumns: Columns des Lookups, sonst listable Felder des Ziels.
func lookupColumns(d metamodel.ObjectDefinition, lk *metamodel.Lookup) []metamodel.FieldDefinition {
	var out []metamodel.FieldDefinition
	for _, f := range d.Fields {
		if (len(lk.Columns) == 0 && f.Listable) || slices.Contains(lk.Columns, f.Key) {
			out = append(out, f)
		}
	}
	return out
}

// sectionCtx sind die Daten des Blocks detail-section.
type sectionCtx struct {
	View    view
	Section sectionView
}

// relRow sind die Daten des Blocks relation-row.
type relRow struct {
	Rel relationView
	Row record
	ID  string
}

// filterValue löst die Quelle eines Lookup-Filters auf: "feld" ist ein Feld
// des Formulars, "feld.zielfeld" das Feld des Datensatzes, auf den das
// Lookup-Feld feld des Formulars zeigt (z. B. draft_id.company_code_id),
// "=wert" ein fester Wert.
func (s *server) filterValue(r *http.Request, from, src string, q url.Values) string {
	if v, ok := strings.CutPrefix(src, "="); ok { // fester Wert, z. B. "=false"
		return v
	}
	field, remote, nested := strings.Cut(src, ".")
	v := strings.TrimSpace(q.Get(field))
	if !nested || v == "" || from == "" {
		return v
	}
	def, err := s.definition(r, from)
	if err != nil {
		return ""
	}
	i := slices.IndexFunc(def.Def.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == field && f.Lookup != nil })
	if i < 0 {
		return ""
	}
	tgt, err := s.targetDef(r, def.Def.Fields[i].Lookup.Object)
	if err != nil {
		return ""
	}
	rec, err := s.fetchRecord(r, tgt, v)
	if err != nil {
		return ""
	}
	return scalar(rec[remote])
}
