package businesspartner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Tabellengesteuerte CRUD-Engine: Jede Entität (Object) beschreibt Tabelle,
// Schlüssel und Felder; list/get/create/update/delete, Typumwandlung,
// Pflichtfelder, Verweise auf Kataloge und Zeitscheiben sind generisch.
// Fachregeln hängen als Hooks an der Entität.

type record = map[string]any

// field ist eine Spalte (oder ein virtuelles Feld) einer Entität.
type field struct {
	Key, Label string
	Type       metamodel.FieldType
	Required   bool
	Listable   bool
	Immutable  bool // nach dem Anlegen nicht änderbar (Schlüsselteile)
	ReadOnly   bool // nur Anzeige (z. B. generierte id)
	Virtual    bool // nicht in der Tabelle (wird von Hooks verarbeitet)
	Options    []metamodel.Option
	Ref        *ref              // Verweis auf einen Katalog/eine Entität (prüfen, Label, Lookup)
	Lookup     *metamodel.Lookup // Lookup ohne Ref, z. B. auf ein Object eines anderen Moduls
}

// ref: Der Wert muss in Table.Column existieren; bei TimeSliced zusätzlich
// am Stichtag (valid_from des Datensatzes, sonst heute) gültig sein.
//
// Object und LabelFields machen daraus ein Lookup im Metamodell (Auswahldialog)
// und liefern den lesbaren Text in "_labels" der Datensätze.
type ref struct {
	Table, Column, Label string
	TimeSliced           bool
	ActiveField          string   // Ziel mit Status-Flag: nur aktive Datensätze sind gültige Verweise
	Object               string   // Ziel-Object, z. B. "PartnerAddressRole"
	LabelFields          []string // Spalten des Ziels für den lesbaren Text
}

// lookup liefert die Lookup-Metadaten eines Felds.
func (f *field) lookup() *metamodel.Lookup {
	if f.Lookup != nil {
		return f.Lookup
	}
	if f.Ref == nil || f.Ref.Object == "" {
		return nil
	}
	return &metamodel.Lookup{Object: f.Ref.Object, ValueField: f.Ref.Column, LabelFields: f.Ref.LabelFields}
}

type entity struct {
	m *Module // Ressourcen des Moduls (Datenbank, Services)

	Object, Title, Icon, Table string
	Section                    string   // Gruppe in der Modul-Navigation (Standard: Partnerdaten)
	Keys                       []string // Primärschlüssel
	Surrogate                  bool     // Keys[0] = generierte id
	TimeSlice                  bool     // valid_from/valid_to
	StatusField                string   // Status-Flag (boolean, true = aktiv); leer = keins
	Fields                     []field
	Order                      string   // ORDER BY
	Filters                    []string // erlaubte Filter in list (Query-Parameter)
	Search                     []string // Spalten für den Suchparameter q (LIKE)

	// Detailansicht (Metamodell): Abschnitte, eingebettete Unter-Objects
	TitleField string
	Sections   []metamodel.SectionDefinition

	// Hooks
	validate    func(ctx context.Context, rec record, old record) error // nach Typprüfung, in der Transaktion
	afterCreate func(ctx context.Context, rec record) error             // in der Transaktion
	listScope   func(ctx context.Context) (where string, args []any, none bool, err error)
	checkRecord func(ctx context.Context, action string, rec record) error // Zugriff je Datensatz
	decorate    func(ctx context.Context, rec record) error                // virtuelle Felder füllen
}

func (e *entity) field(key string) *field {
	for i := range e.Fields {
		if e.Fields[i].Key == key {
			return &e.Fields[i]
		}
	}
	return nil
}

func (e *entity) columns() []string {
	var out []string
	for _, f := range e.Fields {
		if !f.Virtual {
			out = append(out, f.Key)
		}
	}
	return out
}

// --- Schlüssel ---------------------------------------------------------------------

// recordID: einzelner Schlüssel = Wert; zusammengesetzt = Teile URL-kodiert mit "|".
func (e *entity) recordID(rec record) string {
	parts := make([]string, len(e.Keys))
	for i, k := range e.Keys {
		parts[i] = str(rec[k])
	}
	if len(parts) == 1 {
		return parts[0]
	}
	for i := range parts {
		parts[i] = url.QueryEscape(parts[i])
	}
	return strings.Join(parts, "|")
}

func (e *entity) parseID(id string) (record, error) {
	if id == "" {
		return nil, invalid("id fehlt")
	}
	key := record{}
	if len(e.Keys) == 1 {
		key[e.Keys[0]] = id
		return key, nil
	}
	parts := strings.Split(id, "|")
	if len(parts) != len(e.Keys) {
		return nil, invalid("id %q passt nicht zu %s", id, e.Object)
	}
	for i, k := range e.Keys {
		v, err := url.QueryUnescape(parts[i])
		if err != nil {
			return nil, invalid("id %q: %v", id, err)
		}
		key[k] = v
	}
	return key, nil
}

func (e *entity) keyWhere(key record) (string, []any) {
	conds := make([]string, len(e.Keys))
	args := make([]any, len(e.Keys))
	for i, k := range e.Keys {
		conds[i], args[i] = k+" = ?", key[k]
	}
	return strings.Join(conds, " AND "), args
}

// --- Werte -------------------------------------------------------------------------

// coerce wandelt Eingaben (Formular, JSON, CLI) und Datenbankwerte in den Feldtyp.
func coerce(f *field, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" && f.Type != metamodel.TypeBoolean {
		return nil, nil
	}
	switch f.Type {
	case metamodel.TypeBoolean:
		switch b := v.(type) {
		case bool:
			return b, nil
		case int64:
			return b != 0, nil
		case float64:
			return b != 0, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(b)) {
			case "true", "1", "on", "ja", "yes":
				return true, nil
			case "false", "0", "off", "nein", "no", "":
				return false, nil
			}
		}
		return nil, invalid("%s: Ja/Nein erwartet", f.Label)
	case metamodel.TypeDate:
		d, err := parseDate(v)
		if err != nil {
			return nil, invalid("%s: %v", f.Label, err)
		}
		return d, nil
	case metamodel.TypeSelect:
		s := str(v)
		if !slices.ContainsFunc(f.Options, func(o metamodel.Option) bool { return o.Value == s }) {
			return nil, invalid("%s: ungültiger Wert %q", f.Label, s)
		}
		return s, nil
	}
	return strings.TrimSpace(str(v)), nil
}

func str(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return fmt.Sprint(v)
}

func asBool(v any) bool {
	b, _ := coerce(&field{Type: metamodel.TypeBoolean}, v)
	ok, _ := b.(bool)
	return ok
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Lesen -------------------------------------------------------------------------

func (e *entity) scan(ctx context.Context, cols []string, row []any) (record, error) {
	rec := record{}
	for i, c := range cols {
		v, err := coerce(e.field(c), row[i])
		if err != nil {
			return nil, err
		}
		rec[c] = v
	}
	rec["id"] = e.recordID(rec)
	if e.decorate != nil {
		if err := e.decorate(ctx, rec); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

func (e *entity) list(ctx context.Context, payload any) (sdk.Response, error) {
	query := listQuery(payload)
	cols := e.columns()
	var where []string
	var args []any
	for _, f := range e.Filters {
		if v, ok := query[f]; ok && str(v) != "" {
			where, args = append(where, f+" = ?"), append(args, str(v))
		}
	}
	if q := str(query["q"]); q != "" && len(e.Search) > 0 {
		var ors []string
		for _, c := range e.Search {
			ors, args = append(ors, "LOWER("+c+") LIKE ?"), append(args, "%"+strings.ToLower(q)+"%")
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if e.listScope != nil {
		w, a, none, err := e.listScope(ctx)
		if err != nil {
			return sdk.Response{}, err
		}
		if none {
			return sdk.Response{Payload: map[string]any{"items": []any{}}}, nil
		}
		if w != "" {
			where, args = append(where, w), append(args, a...)
		}
	}
	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + e.Table
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	if e.Order != "" {
		sql += " ORDER BY " + e.Order
	}
	res, err := e.m.db.Query(ctx, sql, args...)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, 0, len(res.Rows))
	for _, r := range res.Rows {
		rec, err := e.scan(ctx, cols, r)
		if err != nil {
			return sdk.Response{}, err
		}
		items = append(items, rec)
	}
	recs := make([]record, len(items))
	for i, it := range items {
		recs[i] = it.(record)
	}
	if err := e.withLabels(ctx, recs...); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

// listQuery: WebServer schickt {"query": {...}}, die CLI die Parameter direkt.
func listQuery(payload any) map[string]any {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		return q
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

// load liest einen Datensatz über seinen Schlüssel (oder sdk.ErrNotFound).
func (e *entity) load(ctx context.Context, key record) (record, error) {
	cols := e.columns()
	w, args := e.keyWhere(key)
	res, err := e.m.db.Query(ctx, "SELECT "+strings.Join(cols, ", ")+" FROM "+e.Table+" WHERE "+w, args...)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: %s %s", sdk.ErrNotFound, e.Title, e.recordID(key))
	}
	return e.scan(ctx, cols, res.Rows[0])
}

func (e *entity) get(ctx context.Context, payload any) (sdk.Response, error) {
	key, err := e.parseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	rec, err := e.load(ctx, key)
	if err != nil {
		return sdk.Response{}, err
	}
	if e.checkRecord != nil {
		if err := e.checkRecord(ctx, "get", rec); err != nil {
			return sdk.Response{}, err
		}
	}
	if err := e.withLabels(ctx, rec); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: rec}, nil
}

// --- Schreiben ---------------------------------------------------------------------

// input übernimmt die bekannten Felder aus {"data": {...}} (oder dem Payload selbst).
func (e *entity) input(payload any) (record, error) {
	m, _ := payload.(map[string]any)
	data, ok := m["data"].(map[string]any)
	if !ok {
		data = m
	}
	rec := record{}
	for i := range e.Fields {
		f := &e.Fields[i]
		v, present := data[f.Key]
		if !present || f.ReadOnly {
			continue
		}
		if f.Virtual {
			rec[f.Key] = v // roh, verarbeitet der Hook
			continue
		}
		cv, err := coerce(f, v)
		if err != nil {
			return nil, err
		}
		rec[f.Key] = cv
	}
	return rec, nil
}

// check: Pflichtfelder, Booleans, Zeitscheibe, Verweise, Fachregeln.
func (e *entity) check(ctx context.Context, rec, old record) error {
	for _, f := range e.Fields {
		if f.Virtual || f.ReadOnly {
			continue
		}
		if f.Type == metamodel.TypeBoolean && rec[f.Key] == nil {
			rec[f.Key] = false
		}
	}
	if e.TimeSlice {
		if err := checkTimeSlice(rec); err != nil {
			return err
		}
	}
	for _, f := range e.Fields {
		if f.Virtual || f.ReadOnly {
			continue
		}
		if f.Required && rec[f.Key] == nil {
			return invalid("%s ist Pflicht", f.Label)
		}
		if f.Ref != nil && rec[f.Key] != nil {
			at := today()
			if e.TimeSlice {
				at = str(rec["valid_from"])
			}
			if err := e.m.checkRef(ctx, f.Ref, str(rec[f.Key]), at); err != nil {
				return err
			}
		}
	}
	if e.validate != nil {
		return e.validate(ctx, rec, old)
	}
	return nil
}

func (m *Module) checkRef(ctx context.Context, r *ref, value, at string) error {
	sql := "SELECT 1 FROM " + r.Table + " WHERE " + r.Column + " = ?"
	args := []any{value}
	if r.TimeSliced {
		sql += " AND valid_from <= ? AND valid_to >= ?"
		args = append(args, at, at)
	}
	if r.ActiveField != "" {
		sql += " AND " + r.ActiveField + " = ?"
		args = append(args, true)
	}
	res, err := m.db.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	if len(res.Rows) == 0 {
		if r.TimeSliced {
			return invalid("%s %q gibt es nicht oder ist am %s nicht gültig", r.Label, value, at)
		}
		if r.ActiveField != "" {
			return invalid("%s %q gibt es nicht oder ist inaktiv", r.Label, value)
		}
		return invalid("%s %q gibt es nicht", r.Label, value)
	}
	return nil
}

func (e *entity) create(ctx context.Context, payload any) (sdk.Response, error) {
	rec, err := e.input(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if e.Surrogate {
		rec[e.Keys[0]] = newID()
	}
	if e.StatusField != "" { // neue Datensätze sind aktiv
		rec[e.StatusField] = true
	}
	err = e.m.db.InTx(ctx, nil, func(ctx context.Context) error {
		if err := e.check(ctx, rec, nil); err != nil {
			return err
		}
		if e.checkRecord != nil {
			if err := e.checkRecord(ctx, "create", rec); err != nil {
				return err
			}
		}
		if _, err := e.load(ctx, rec); err == nil {
			return fmt.Errorf("%w: %s %s gibt es bereits", sdk.ErrAlreadyExists, e.Title, e.recordID(rec))
		}
		cols := e.columns()
		marks := make([]string, len(cols))
		args := make([]any, len(cols))
		for i, c := range cols {
			marks[i], args[i] = "?", rec[c]
		}
		if _, err := e.m.db.Exec(ctx, "INSERT INTO "+e.Table+" ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(marks, ", ")+")", args...); err != nil {
			return err
		}
		if e.afterCreate != nil {
			return e.afterCreate(ctx, rec)
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	saved, err := e.load(ctx, rec)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := e.withLabels(ctx, saved); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: saved}, nil
}

func (e *entity) update(ctx context.Context, payload any) (sdk.Response, error) {
	key, err := e.parseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	in, err := e.input(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	var saved record
	err = e.m.db.InTx(ctx, nil, func(ctx context.Context) error {
		old, err := e.load(ctx, key)
		if err != nil {
			return err
		}
		if e.checkRecord != nil {
			if err := e.checkRecord(ctx, "update", old); err != nil {
				return err
			}
		}
		rec := record{}
		for k, v := range old {
			rec[k] = v
		}
		for k, v := range in {
			f := e.field(k)
			if f.Virtual {
				continue // virtuelle Felder nur beim Anlegen
			}
			if (f.Immutable || slices.Contains(e.Keys, k)) && str(v) != str(old[k]) {
				return invalid("%s kann nach dem Anlegen nicht geändert werden", f.Label)
			}
			rec[k] = v
		}
		if err := e.check(ctx, rec, old); err != nil {
			return err
		}
		var sets []string
		var args []any
		for _, c := range e.columns() {
			if !slices.Contains(e.Keys, c) {
				sets, args = append(sets, c+" = ?"), append(args, rec[c])
			}
		}
		w, wargs := e.keyWhere(key)
		if _, err := e.m.db.Exec(ctx, "UPDATE "+e.Table+" SET "+strings.Join(sets, ", ")+" WHERE "+w, append(args, wargs...)...); err != nil {
			return err
		}
		saved, err = e.load(ctx, key)
		return err
	})
	if err != nil {
		return sdk.Response{}, err
	}
	if err := e.withLabels(ctx, saved); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: saved}, nil
}

func idOf(payload any) string {
	m, _ := payload.(map[string]any)
	return str(m["id"])
}

// --- Metamodell --------------------------------------------------------------------

func (e *entity) definition() metamodel.ObjectDefinition {
	d := metamodel.ObjectDefinition{Name: e.Object, Title: e.Title, Icon: e.Icon, TitleField: e.TitleField, Sections: e.Sections}
	for i := range e.Fields {
		f := &e.Fields[i]
		d.Fields = append(d.Fields, metamodel.FieldDefinition{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required,
			Listable: f.Listable, Editable: !f.ReadOnly, Options: f.Options, Lookup: f.lookup(),
		})
	}
	d.Actions = []metamodel.ActionConfig{
		{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "get", Kind: metamodel.KindItem, Label: "Anzeigen"},
		{Name: "create", Kind: metamodel.KindCreate, Label: "Neu"},
		{Name: "update", Kind: metamodel.KindUpdate, Label: "Bearbeiten"},
	}
	d.Lifecycle = e.lifecycle()
	switch d.Lifecycle.Kind() {
	case metamodel.LifecycleTimeSlice:
		d.Actions = append(d.Actions, metamodel.ActionConfig{Name: "expire", Kind: metamodel.KindExpire, Label: "Beenden …"})
	case metamodel.LifecycleStatus:
		d.Actions = append(d.Actions, metamodel.ActionConfig{Name: "deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren", Confirm: e.Title + " inaktivieren?"})
	}
	return d
}
