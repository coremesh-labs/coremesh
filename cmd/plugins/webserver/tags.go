package main

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// TagEditor: Abschnitt „Tags“ der Detailansicht (SectionDefinition.Tags).
//
// Die Komponente ist generisch: Sie kennt nur den Objekttyp und die ID des
// Datensatzes und baut die Eingabefelder aus dem Schema des TagService
// (Tag Sets je Objekttyp und Buchungskreis, Datentypen, Auswahlwerte, Regeln).
//
//	GET  /tags/{entity}/{id}?effectiveDate=…&companyCode=…   Editor (Fragment)
//	POST /tags/{entity}/{id}/preview                         Regeln neu auswerten (ohne Speichern)
//	POST /tags/{entity}/{id}                                 speichern ab „Gültig ab“
//
// Eingaben je Datentyp: STRING Text, INTEGER Zahl, CURRENCY Betrag + Währung,
// DATE Datum, TIMESTAMP Datum/Uhrzeit (UTC), Auswahlwerte als Liste.
// SHOW_IF blendet Felder aus (ausgeblendete Werte werden beim Speichern
// beendet), Pflicht folgt aus mandatory und REQUIRES. Bei jeder Änderung
// wertet der Server die Regeln neu aus (preview); verbindlich prüft Tags.set.

// tagEditor sind die Daten des Templates "tag-editor".
type tagEditor struct {
	URL           string // /tags/{entity}/{id}
	EntityType    string
	EntityID      string
	EffectiveDate string
	ValidFrom     string
	CompanyCode   string
	CompanyCodes  []tagOption // Auswahl des Buchungskreises ("" = nur globale Tags)
	Sets          []tagEditorSet
	ReadOnly      bool
	Errors        []string
	Saved         bool
	Unavailable   string
}

type tagEditorSet struct {
	Code, Name string
	Fields     []tagField
}

type tagField struct {
	Code, Name, DataType string
	Options              []tagOption
	RefObject, RefLabel  string // REFERENCE: Ziel-Object und lesbarer Text des Werts
	Value, Amount, Curr  string // Formularwerte
	Since                string // Beginn der Zeitscheibe des Werts
	Scope                string // Buchungskreis des Werts oder "*"
	Required, Visible    bool
	Deprecated           bool
	Hidden               bool   // geschützt, der Benutzer darf den Wert nicht sehen
	Pattern, Hint        string // Prüfmuster (Text) und Hinweis
	Error                string
}

type tagOption struct{ Value, Label string }

func tagEditorURL(entityType, entityID string) string {
	return "/tags/" + entityType + "/" + url.PathEscape(entityID)
}

// GET /tags/{entity}/{id}
func (s *server) tagEditorGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ed := s.newTagEditor(r, q.Get("effectiveDate"), q.Get("companyCode"))
	var et tagservice.EntityTags
	resp, err := s.call(r, tagservice.Object, "get", tagservice.GetRequest{EntityType: ed.EntityType, EntityID: ed.EntityID,
		CompanyCode: ed.CompanyCode, EffectiveDate: ed.EffectiveDate, Locale: localeFrom(r)})
	if err == nil {
		err = sdk.Decode(resp.Payload, &et)
	}
	if err != nil {
		s.tagUnavailable(w, r, ed, err)
		return
	}
	ed.fill(et.Schema, et.Values, et.State, nil)
	s.render(w, r, http.StatusOK, "tag-editor", ed, "", "")
}

// POST /tags/{entity}/{id}/preview und POST /tags/{entity}/{id}
func (s *server) tagEditorPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err))
		return
	}
	save := !strings.HasSuffix(r.URL.Path, "/preview")
	ed := s.newTagEditor(r, r.PostForm.Get("effectiveDate"), r.PostForm.Get("companyCode"))
	ed.ValidFrom = r.PostForm.Get("validFrom")
	if ed.ValidFrom == "" {
		ed.ValidFrom = ed.EffectiveDate
	}
	// Schema zum Beginn der neuen Werte.
	var schema tagservice.Schema
	// Mit EntityID: Tag Sets mit Bedingung gelten nach den Feldern des Datensatzes.
	resp, err := s.call(r, tagservice.Object, "schema", tagservice.GetRequest{EntityType: ed.EntityType, EntityID: ed.EntityID,
		CompanyCode: ed.CompanyCode, EffectiveDate: ed.ValidFrom, Locale: localeFrom(r)})
	if err == nil {
		err = sdk.Decode(resp.Payload, &schema)
	}
	if err != nil {
		s.tagUnavailable(w, r, ed, err)
		return
	}
	values, fieldErrs := formTagValues(schema, r.PostForm)
	req := tagservice.SetRequest{EntityType: ed.EntityType, EntityID: ed.EntityID, CompanyCode: ed.CompanyCode,
		ValidFrom: ed.ValidFrom, Values: values, Locale: localeFrom(r)}

	// Regeln auswerten: Zustand und Verstöße für den eingegebenen Stand.
	var check struct {
		Violations []tagservice.Violation `json:"violations"`
		State      tagservice.State       `json:"state"`
	}
	resp, err = s.call(r, tagservice.Object, "validate", req)
	if err == nil {
		err = sdk.Decode(resp.Payload, &check)
	}
	if err != nil {
		s.tagUnavailable(w, r, ed, err)
		return
	}
	// Ausgeblendete Tags (SHOW_IF) werden beim Speichern beendet.
	for code, visible := range check.State.Visible {
		if !visible {
			values[code] = nil
		}
	}
	if !save {
		ed.fillForm(schema, r.PostForm, check.State, nil, fieldErrs)
		s.refLabels(r, ed)
		s.render(w, r, http.StatusOK, "tag-editor", ed, "", "")
		return
	}
	if len(fieldErrs) == 0 {
		resp, err = s.call(r, tagservice.Object, "set", req)
		if err == nil {
			var et tagservice.EntityTags
			if err = sdk.Decode(resp.Payload, &et); err == nil {
				ed.EffectiveDate = ed.ValidFrom
				ed.fill(et.Schema, et.Values, et.State, nil)
				ed.Saved = true
				s.render(w, r, http.StatusOK, "tag-editor", ed, "", "")
				return
			}
		}
		ed.Errors = append(ed.Errors, strings.TrimPrefix(err.Error(), sdk.ErrInvalidArgument.Error()+": "))
	}
	byTag := map[string]string{}
	for _, v := range check.Violations {
		byTag[v.Tag] = v.Message
	}
	maps.Copy(byTag, fieldErrs)
	ed.fillForm(schema, r.PostForm, check.State, byTag, fieldErrs)
	s.refLabels(r, ed)
	s.render(w, r, http.StatusUnprocessableEntity, "tag-editor", ed, "", "")
}

func (s *server) newTagEditor(r *http.Request, effectiveDate, companyCode string) *tagEditor {
	entity, id := r.PathValue("entity"), r.PathValue("id")
	if effectiveDate == "" {
		effectiveDate = time.Now().Format(time.DateOnly)
	}
	ed := &tagEditor{URL: tagEditorURL(entity, id), EntityType: entity, EntityID: id,
		EffectiveDate: effectiveDate, ValidFrom: effectiveDate, CompanyCode: companyCode}
	if u := userFrom(r); u != nil && !(u.Can(tagservice.Object, "set") && u.Can(entity, "update")) {
		ed.ReadOnly = true
	}
	// Buchungskreise zur Auswahl (iam); ohne Leserecht nur die globalen Tags.
	if resp, err := s.call(r, "CompanyCode", "list", nil); err == nil {
		if recs, err := records(resp.Payload); err == nil {
			for _, rec := range recs {
				code := scalar(rec["code"])
				ed.CompanyCodes = append(ed.CompanyCodes, tagOption{Value: code, Label: code + " " + scalar(rec["description"])})
			}
		}
	}
	return ed
}

func (s *server) tagUnavailable(w http.ResponseWriter, r *http.Request, ed *tagEditor, err error) {
	if status := errStatus(err); status >= 500 && status != http.StatusServiceUnavailable {
		s.logError(r, "TagEditor", err)
	}
	ed.Unavailable = s.T(r, "core.tags.unavailable")
	if code := errStatus(err); code == http.StatusForbidden || code == http.StatusNotFound || code == http.StatusUnprocessableEntity {
		ed.Unavailable = strings.SplitN(err.Error(), ": ", 2)[len(strings.SplitN(err.Error(), ": ", 2))-1]
	}
	s.render(w, r, http.StatusOK, "tag-editor", ed, "", "")
}

// fill baut die Felder aus Schema und gespeicherten Werten.
func (ed *tagEditor) fill(schema tagservice.Schema, values []tagservice.Assignment, st tagservice.State, errs map[string]string) {
	byTag := map[string]tagservice.Assignment{}
	for _, a := range values {
		byTag[a.Tag] = a
	}
	ed.build(schema, st, errs, func(f *tagField) {
		a, ok := byTag[f.Code]
		if !ok {
			return
		}
		f.Since, f.Scope = a.ValidFrom, a.CompanyCode
		v := a.Value
		switch {
		case v.Option != nil:
			f.Value = *v.Option
		case v.String != nil:
			f.Value = *v.String
		case v.Integer != nil:
			f.Value = strconv.FormatInt(*v.Integer, 10)
		case v.Amount != nil:
			f.Amount, f.Curr = *v.Amount, scalar(deref(v.Currency))
		case v.Date != nil:
			f.Value = *v.Date
		case v.Timestamp != nil:
			if t, err := time.Parse(time.RFC3339, *v.Timestamp); err == nil {
				f.Value = t.UTC().Format("2006-01-02T15:04")
			}
		case v.Ref != nil:
			f.Value, f.RefLabel = *v.Ref, a.RefLabel
		}
	})
}

// fillForm baut die Felder aus den Formulareingaben (Vorschau, Fehler).
func (ed *tagEditor) fillForm(schema tagservice.Schema, form url.Values, st tagservice.State, errs, fieldErrs map[string]string) {
	for k, v := range fieldErrs {
		if errs == nil {
			errs = map[string]string{}
		}
		errs[k] = v
	}
	ed.build(schema, st, errs, func(f *tagField) {
		f.Value = form.Get("v." + f.Code)
		f.Amount, f.Curr = form.Get("v."+f.Code+".amount"), form.Get("v."+f.Code+".currency")
	})
}

func (ed *tagEditor) build(schema tagservice.Schema, st tagservice.State, errs map[string]string, value func(*tagField)) {
	seen := map[string]bool{}
	for _, set := range schema.Sets {
		es := tagEditorSet{Code: set.Code, Name: set.Name}
		for _, it := range set.Items {
			if seen[it.Tag.Code] {
				continue // in mehreren Sets: einmal anzeigen
			}
			seen[it.Tag.Code] = true
			f := tagField{Code: it.Tag.Code, Name: it.Tag.Name, DataType: string(it.Tag.DataType), Scope: it.Scope,
				Required: st.Required[it.Tag.Code], Visible: st.Visible[it.Tag.Code] || len(st.Visible) == 0,
				Deprecated: it.Tag.Status != "ACTIVE", Error: errs[it.Tag.Code], RefObject: it.Tag.RefObject,
				Hidden: it.Tag.Hidden, Pattern: it.Tag.Pattern, Hint: it.Tag.PatternHint}
			if it.Tag.ValueMode == tagservice.ModeOptions {
				for _, o := range it.Tag.Options {
					f.Options = append(f.Options, tagOption{Value: o.Code, Label: o.Label})
				}
			}
			value(&f)
			es.Fields = append(es.Fields, f)
		}
		ed.Sets = append(ed.Sets, es)
	}
}

// formTagValues liest die Eingaben: leer = Wert beenden (nil).
func formTagValues(schema tagservice.Schema, form url.Values) (map[string]*tagservice.Value, map[string]string) {
	values, errs := map[string]*tagservice.Value{}, map[string]string{}
	for _, set := range schema.Sets {
		for _, it := range set.Items {
			code := it.Tag.Code
			if _, done := values[code]; done || it.Tag.Status != "ACTIVE" || it.Tag.Hidden {
				continue // veraltete und verborgene (geschützte) Tags sind nur lesbar – nicht beenden
			}
			raw := strings.TrimSpace(form.Get("v." + code))
			if it.Tag.ValueMode == tagservice.ModeOptions {
				values[code] = optional(raw, tagservice.Option)
				continue
			}
			switch it.Tag.DataType {
			case tagservice.TypeString:
				values[code] = optional(raw, tagservice.String)
			case tagservice.TypeDate:
				values[code] = optional(raw, tagservice.Date)
			case tagservice.TypeReference:
				values[code] = optional(raw, tagservice.Ref)
			case tagservice.TypeInteger:
				if raw == "" {
					values[code] = nil
				} else if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
					values[code] = tagservice.Integer(i)
				} else {
					errs[code] = "core.validation.number"
				}
			case tagservice.TypeTimestamp:
				if raw == "" {
					values[code] = nil
				} else if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
					values[code] = tagservice.Timestamp(t.UTC().Format(time.RFC3339))
				} else {
					errs[code] = "core.validation.date"
				}
			case tagservice.TypeCurrency:
				amount := strings.ReplaceAll(strings.TrimSpace(form.Get("v."+code+".amount")), ",", ".")
				cur := strings.ToUpper(strings.TrimSpace(form.Get("v." + code + ".currency")))
				if amount == "" && cur == "" {
					values[code] = nil
				} else {
					values[code] = tagservice.Money(amount, cur)
				}
			}
		}
	}
	return values, errs
}

func optional(s string, f func(string) *tagservice.Value) *tagservice.Value {
	if s == "" {
		return nil
	}
	return f(s)
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// tagScopes: Text für den Geltungsbereich eines Werts.
func (f tagField) Global() bool { return f.Scope == tagservice.AllCompanyCodes || f.Scope == "" }

// tagFieldCtx sind die Daten des Blocks tag-field.
type tagFieldCtx struct {
	Ed *tagEditor
	F  tagField
}

// refLabels ergänzt den lesbaren Text der Verweise aus den Formulareingaben
// (Vorschau, Fehler) – gespeicherte Werte bringen ihn vom TagService mit.
func (s *server) refLabels(r *http.Request, ed *tagEditor) {
	for i := range ed.Sets {
		for j := range ed.Sets[i].Fields {
			if f := &ed.Sets[i].Fields[j]; f.RefObject != "" && f.Value != "" {
				f.RefLabel = s.refLabel(r, f.RefObject, f.Value)
			}
		}
	}
}

// Peek: Vorschau des Verweisziels ("" = kein Verweis bzw. kein Wert).
func (f tagField) Peek() string {
	if f.RefObject == "" {
		return ""
	}
	return peekURL(f.RefObject, f.Value)
}
