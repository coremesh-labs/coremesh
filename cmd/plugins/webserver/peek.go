package main

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Kopfdaten-Vorschau (Peek): Neben jedem Verweis – einem Feld mit Lookup – steht
// in Listen, eingebetteten Tabellen und der Detailansicht ein Hinweissymbol (ⓘ).
// Beim Überfahren (oder Fokus per Tastatur) lädt es
//
//	GET /peek/{object}/{id}
//
// in ein kleines Dialogfenster: Überschrift (TitleField), die Kopfdaten des
// Ziels und ein Link in dessen Detailansicht. Gelesen wird über die get-Action
// des Ziels, die Berechtigung prüft der Dispatcher. Das Metamodell liefert der
// Catalog – die Fähigkeit gilt damit für alle bestehenden und künftigen Module
// ohne eigenen Code.

// peekMaxFields begrenzt die Kopfdaten im Dialog.
const peekMaxFields = 8

// peekView sind die Daten des Blocks "peek".
type peekView struct {
	Object    string
	ID        string
	Title     string // Titel des Objects, z. B. "Rollentypen"
	Heading   string // Wert des TitleField, sonst die id
	Record    record
	Fields    []metamodel.FieldDefinition
	DetailURL string // leer = keine Detailansicht (kein Modul oder kein Recht)
	Message   string // statt der Kopfdaten, z. B. fehlende Berechtigung
}

// peekURL ist die Adresse der Vorschau eines Datensatzes ("" = keine).
func peekURL(object, id string) string {
	if !objectRe.MatchString(object) || id == "" {
		return ""
	}
	return "/peek/" + object + "/" + url.PathEscape(id)
}

// peekFor: Vorschau für ein Feld eines Datensatzes – nur bei Verweisen (Lookup).
func peekFor(rec record, f metamodel.FieldDefinition) string {
	if f.Lookup == nil {
		return ""
	}
	return peekURL(f.Lookup.Object, scalar(rec[f.Key]))
}

// targetDef holt Metamodell und fachliches Modul eines beliebigen Objects –
// auch aus einem anderen Modul als dem der Anfrage.
func (s *server) targetDef(r *http.Request, object string) (objectCtx, error) {
	resp, err := s.call(r, sdk.ObjectCatalog, "GetDefinition", map[string]any{"object": object})
	if err != nil {
		return objectCtx{}, err
	}
	var def struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
		Available  bool                       `json:"available"`
		UIModule   string                     `json:"ui_module"`
	}
	if err := sdk.Decode(resp.Payload, &def); err != nil {
		return objectCtx{}, err
	}
	if !def.Available {
		return objectCtx{}, fmt.Errorf("%w: %s", sdk.ErrUnavailable, s.T(r, "core.error.unavailable", def.Definition.Title))
	}
	return s.withLocale(r, newObjectCtx(def.UIModule, object, s.localizeDef(r, def.Definition)).visibleFor(userFrom(r))), nil
}

// fetchRecord liest einen Datensatz über die get-Action (Kind item) des Objects.
func (s *server) fetchRecord(r *http.Request, oc objectCtx, id string) (record, error) {
	act, err := need(oc, metamodel.KindItem)
	if err != nil {
		return nil, err
	}
	resp, err := s.call(r, oc.Object, act.Name, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	rec := record{}
	if err := sdk.Decode(resp.Payload, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// headerFields sind die Kopfdaten: die Felder des ersten Feld-Abschnitts der
// Detailansicht, sonst die listable Felder (ohne Passwörter und TitleField).
func headerFields(d metamodel.ObjectDefinition) []metamodel.FieldDefinition {
	byKey := map[string]metamodel.FieldDefinition{}
	for _, f := range d.Fields {
		byKey[f.Key] = f
	}
	var keys []string
	for _, sd := range d.Sections {
		if len(sd.Fields) > 0 {
			keys = sd.Fields
			break
		}
	}
	if keys == nil {
		for _, f := range d.Fields {
			if f.Listable {
				keys = append(keys, f.Key)
			}
		}
	}
	var out []metamodel.FieldDefinition
	for _, k := range keys {
		f, ok := byKey[k]
		if !ok || f.Type == metamodel.TypePassword || k == d.TitleField {
			continue
		}
		if out = append(out, f); len(out) == peekMaxFields {
			break
		}
	}
	return out
}

// GET /peek/{object}/{id}
//
// Fehler erscheinen im Dialog selbst (Status 200), damit ein fehlendes Recht
// auf das Ziel die Liste nicht stört.
func (s *server) peek(w http.ResponseWriter, r *http.Request) {
	object, id := r.PathValue("object"), r.PathValue("id")
	if !objectRe.MatchString(object) || id == "" {
		s.fail(w, r, fmt.Errorf("%w: Object und id erwartet", sdk.ErrInvalidArgument))
		return
	}
	pv := peekView{Object: object, ID: id, Heading: id, Title: object}
	oc, err := s.targetDef(r, object)
	if err == nil {
		pv.Title = oc.Def.Title
		pv.Record, err = s.fetchRecord(r, oc, id)
	}
	if err != nil {
		switch errStatus(err) {
		case http.StatusForbidden:
			pv.Message = s.T(r, "core.peek.no_permission")
		case http.StatusNotFound:
			pv.Message = s.T(r, "core.peek.not_found")
		default:
			s.logError(r, "Peek", err)
			pv.Message = s.T(r, "core.peek.unavailable")
		}
		s.render(w, r, http.StatusOK, "peek", pv, "", "")
		return
	}
	if v := scalar(pv.Record[oc.Def.TitleField]); oc.Def.TitleField != "" && v != "" {
		pv.Heading = v
	}
	pv.Fields = headerFields(oc.Def)
	if oc.Module != "" && oc.Has[string(metamodel.KindItem)] != nil {
		pv.DetailURL = oc.URL + "/" + url.PathEscape(recordID(pv.Record))
	}
	s.render(w, r, http.StatusOK, "peek", pv, "", "")
}

// refLabel: lesbarer Text eines Datensatzes (TitleField), sonst die id – für
// Verweis-Tags im TagEditor.
func (s *server) refLabel(r *http.Request, object, id string) string {
	oc, err := s.targetDef(r, object)
	if err != nil || oc.Def.TitleField == "" {
		return id
	}
	rec, err := s.fetchRecord(r, oc, id)
	if err != nil {
		return id
	}
	if v := scalar(rec[oc.Def.TitleField]); v != "" {
		return v
	}
	return id
}
