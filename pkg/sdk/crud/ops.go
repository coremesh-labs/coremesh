package crud

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// --- Lesen -------------------------------------------------------------------

func (e *Entity) scan(ctx context.Context, cols []string, row []any) (Record, error) {
	rec := Record{}
	for i, c := range cols {
		v, err := Coerce(e.Field(c), row[i])
		if err != nil {
			return nil, err
		}
		rec[c] = v
	}
	// _id identifiziert den Datensatz (bei Zeitscheiben inkl. valid_from). "id"
	// ist der fachliche Schlüssel, auf den Verweise zeigen: die Spalte id, sonst
	// der Schlüssel ohne valid_from (z. B. der Code eines Katalogs).
	rec["_id"] = e.RecordID(rec)
	if e.Field("id") == nil {
		rec["id"] = e.BusinessKey(rec)
	}
	if e.Decorate != nil {
		if err := e.Decorate(ctx, rec); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

// List liefert {"items": […]}: Filter, Suche q, ListScope; standardmäßig nur
// heute gültige bzw. aktive Datensätze (includeHistory=true: alle).
func (e *Entity) List(ctx context.Context, payload any) (sdk.Response, error) {
	query := listQuery(payload)
	cols := e.Columns()
	var where []string
	var args []any
	for _, f := range e.Filters {
		if v, ok := query[f]; ok && Str(v) != "" {
			where, args = append(where, f+" = ?"), append(args, e.filterArg(f, v))
		}
	}
	if !IncludeHistory(query) {
		if e.TimeSlice {
			where, args = append(where, "valid_from <= ? AND valid_to >= ?"), append(args, Today(), Today())
		}
		if e.StatusField != "" {
			active, _ := e.statusValues()
			where, args = append(where, e.StatusField+" = ?"), append(args, active)
		}
	}
	if q := Str(query["q"]); q != "" && len(e.Search) > 0 {
		var ors []string
		for _, c := range e.Search {
			ors, args = append(ors, "LOWER("+c+") LIKE ?"), append(args, "%"+strings.ToLower(q)+"%")
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if e.ListScope != nil {
		w, a, none, err := e.ListScope(ctx)
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
	res, err := e.DB().Query(ctx, sql, args...)
	if err != nil {
		return sdk.Response{}, err
	}
	recs := make([]Record, 0, len(res.Rows))
	for _, r := range res.Rows {
		rec, err := e.scan(ctx, cols, r)
		if err != nil {
			return sdk.Response{}, err
		}
		recs = append(recs, rec)
	}
	if err := e.WithLabels(ctx, recs...); err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(recs))
	for i, r := range recs {
		items[i] = r
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

// Load liest einen Datensatz über seinen Schlüssel (oder sdk.ErrNotFound).
// Fehlt bei Zeitscheiben valid_from, gilt die heute gültige, sonst die jüngste.
func (e *Entity) Load(ctx context.Context, key Record) (Record, error) {
	cols := e.Columns()
	w, args := e.keyWhere(key)
	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + e.Table + " WHERE " + w
	if _, full := key["valid_from"]; e.TimeSlice && !full {
		sql += " ORDER BY CASE WHEN valid_from <= ? AND valid_to >= ? THEN 1 ELSE 0 END DESC, valid_from DESC LIMIT 1"
		args = append(args, Today(), Today())
	}
	res, err := e.DB().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: %s %s", sdk.ErrNotFound, e.Title, e.RecordID(key))
	}
	return e.scan(ctx, cols, res.Rows[0])
}

// Get liefert einen Datensatz ({"id": …}).
func (e *Entity) Get(ctx context.Context, payload any) (sdk.Response, error) {
	key, err := e.ParseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	rec, err := e.Load(ctx, key)
	if err != nil {
		return sdk.Response{}, err
	}
	if e.CheckRecord != nil {
		if err := e.CheckRecord(ctx, "get", rec); err != nil {
			return sdk.Response{}, err
		}
	}
	if err := e.WithLabels(ctx, rec); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: rec}, nil
}

// --- Schreiben ---------------------------------------------------------------

// Input übernimmt die bekannten Felder aus {"data": {...}} (oder dem Payload selbst).
func (e *Entity) Input(payload any) (Record, error) {
	m, _ := payload.(map[string]any)
	data, ok := m["data"].(map[string]any)
	if !ok {
		data = m
	}
	rec := Record{}
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
		cv, err := Coerce(f, v)
		if err != nil {
			return nil, err
		}
		rec[f.Key] = cv
	}
	return rec, nil
}

// Check: Pflichtfelder, Booleans, Zeitscheibe, Überschneidung, Verweise, Validate-Hook.
func (e *Entity) Check(ctx context.Context, rec, old Record) error {
	for _, f := range e.Fields {
		if f.Virtual || f.ReadOnly {
			continue
		}
		if f.Type == metamodel.TypeBoolean && rec[f.Key] == nil {
			rec[f.Key] = false
		}
	}
	if e.TimeSlice {
		if err := CheckTimeSlice(rec); err != nil {
			return err
		}
		if err := e.checkOverlap(ctx, rec); err != nil {
			return err
		}
	}
	for _, f := range e.Fields {
		if f.Virtual || f.ReadOnly {
			continue
		}
		if f.Required && rec[f.Key] == nil {
			return Invalid("%s ist Pflicht", f.Label)
		}
		if f.Ref != nil && rec[f.Key] != nil {
			at := Today()
			if e.TimeSlice {
				at = Str(rec["valid_from"])
			}
			if err := e.CheckRef(ctx, f.Ref, Str(rec[f.Key]), at); err != nil {
				return err
			}
		}
	}
	if e.Validate != nil {
		return e.Validate(ctx, rec, old)
	}
	return nil
}

// CheckRef prüft einen Verweis zum Stichtag at.
func (e *Entity) CheckRef(ctx context.Context, r *Ref, value, at string) error {
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
	res, err := e.DB().Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	if len(res.Rows) == 0 {
		switch {
		case r.TimeSliced:
			return Invalid("%s %q gibt es nicht oder ist am %s nicht gültig", r.Label, value, at)
		case r.ActiveField != "":
			return Invalid("%s %q gibt es nicht oder ist inaktiv", r.Label, value)
		}
		return Invalid("%s %q gibt es nicht", r.Label, value)
	}
	return nil
}

// Insert prüft und schreibt einen neuen Datensatz – ohne eigene Transaktion
// (für Hooks, die im Rahmen einer laufenden Transaktion anlegen).
func (e *Entity) Insert(ctx context.Context, rec Record) error {
	if err := e.Check(ctx, rec, nil); err != nil {
		return err
	}
	if e.CheckRecord != nil {
		if err := e.CheckRecord(ctx, "create", rec); err != nil {
			return err
		}
	}
	if _, err := e.Load(ctx, rec); err == nil {
		return fmt.Errorf("%w: %s %s gibt es bereits", sdk.ErrAlreadyExists, e.Title, e.RecordID(rec))
	}
	cols := e.Columns()
	marks := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		marks[i], args[i] = "?", rec[c]
	}
	if _, err := e.DB().Exec(ctx, "INSERT INTO "+e.Table+" ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(marks, ", ")+")", args...); err != nil {
		return err
	}
	if e.AfterCreate != nil {
		return e.AfterCreate(ctx, rec)
	}
	return nil
}

// Create legt einen Datensatz an ({"data": {...}}), in einer Transaktion.
func (e *Entity) Create(ctx context.Context, payload any) (sdk.Response, error) {
	rec, err := e.Input(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if e.Surrogate {
		rec[e.Keys[0]] = NewID()
	}
	if e.StatusField != "" { // neue Datensätze sind aktiv
		rec[e.StatusField], _ = e.statusValues()
	}
	if err := e.DB().InTx(ctx, nil, func(ctx context.Context) error { return e.Insert(ctx, rec) }); err != nil {
		return sdk.Response{}, err
	}
	return e.respondEvent(ctx, rec, "create")
}

func (e *Entity) respond(ctx context.Context, key Record) (sdk.Response, error) {
	saved, err := e.Load(ctx, e.KeyOf(key))
	if err != nil {
		return sdk.Response{}, err
	}
	if err := e.WithLabels(ctx, saved); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: saved}, nil
}

// Update ändert einen Datensatz ({"id", "data"}) – genau eine Zeitscheibe.
// Schlüssel und Immutable-Felder sind nach dem Anlegen fest.
func (e *Entity) Update(ctx context.Context, payload any) (sdk.Response, error) {
	key, err := e.ParseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	in, err := e.Input(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	err = e.DB().InTx(ctx, nil, func(ctx context.Context) error {
		old, err := e.Load(ctx, key)
		if err != nil {
			return err
		}
		key = e.KeyOf(old)
		if e.CheckRecord != nil {
			if err := e.CheckRecord(ctx, "update", old); err != nil {
				return err
			}
		}
		rec := Record{}
		for k, v := range old {
			rec[k] = v
		}
		for k, v := range in {
			f := e.Field(k)
			if f.Virtual {
				continue // virtuelle Felder nur beim Anlegen
			}
			if (f.Immutable || slices.Contains(e.Keys, k)) && Str(v) != Str(old[k]) {
				return Invalid("%s kann nach dem Anlegen nicht geändert werden", f.Label)
			}
			rec[k] = v
		}
		if err := e.Check(ctx, rec, old); err != nil {
			return err
		}
		var sets []string
		var args []any
		for _, c := range e.Columns() {
			if !slices.Contains(e.Keys, c) {
				sets, args = append(sets, c+" = ?"), append(args, rec[c])
			}
		}
		w, wargs := e.keyWhere(key)
		_, err = e.DB().Exec(ctx, "UPDATE "+e.Table+" SET "+strings.Join(sets, ", ")+" WHERE "+w, append(args, wargs...)...)
		return err
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return e.respondEvent(ctx, key, "update")
}

// filterArg wandelt einen Filterwert nach dem Feldtyp um: Ja/Nein-Felder
// boolesch ("true"/"false" aus Formularen und URLs), Zahlen numerisch.
func (e *Entity) filterArg(field string, v any) any {
	f := e.Field(field)
	if f == nil {
		return Str(v)
	}
	switch f.Type {
	case metamodel.TypeBoolean:
		return AsBool(v)
	case metamodel.TypeNumber:
		if n, err := strconv.ParseInt(Str(v), 10, 64); err == nil {
			return n
		}
		if x, err := strconv.ParseFloat(Str(v), 64); err == nil {
			return x
		}
	}
	return Str(v)
}
