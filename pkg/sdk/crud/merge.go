package crud

import (
	"context"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Zeitscheiben zusammenfassen: Eine Zeitscheibe, die lückenlos an die vorherige
// desselben fachlichen Schlüssels anschließt und dieselben Werte hat (alle
// gespeicherten Felder außer valid_from/valid_to), geht in der vorherigen auf –
// diese reicht dann bis zum Ende der späteren. Es geht keine Information
// verloren; die spätere Zeile entfällt (doppelte Abgrenzung, z. B. aus einem
// Import mit Ständen ohne Änderung).

// MergeAction: Aktion „Mit vorheriger Zeitscheibe zusammenfassen“ (je Datensatz).
const MergeAction = "mergeSlice"

func (e *Entity) mergeable() bool { return e.TimeSlice && !e.ReadOnly && !e.CreateOnly }

func (e *Entity) mergeActionConfig() metamodel.ActionConfig {
	return metamodel.ActionConfig{Name: MergeAction, Kind: metamodel.KindCustom, Label: "Mit vorheriger Zeitscheibe zusammenfassen", Record: true,
		Confirm: "Diese Zeitscheibe hat dieselben Werte wie die vorherige. Zusammenfassen?",
		// Standardtexte des WebServers (wie „Wieder aktivieren“), kein Modul übersetzt sie.
		LabelKey: MergeLabelKey, ConfirmKey: MergeLabelKey + ".confirm"}
}

// MergeLabelKey: Übersetzungsschlüssel der Aktion (und + ".confirm").
const MergeLabelKey = "core.action.mergeSlice"

// dayBefore: Vortag eines Datums JJJJ-MM-TT.
func dayBefore(d string) string {
	t, err := time.Parse(time.DateOnly, d)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -1).Format(time.DateOnly)
}

// sameSlice: gleiche Werte außer der Gültigkeit; leer und NULL gelten als gleich.
func (e *Entity) sameSlice(a, b Record) bool {
	for _, c := range e.Columns() {
		if c == "valid_from" || c == "valid_to" {
			continue
		}
		if Str(a[c]) != Str(b[c]) {
			return false
		}
	}
	return true
}

// previousSlice: die direkt vorangehende Zeitscheibe desselben fachlichen Schlüssels (nil: keine).
func (e *Entity) previousSlice(ctx context.Context, rec Record) (Record, error) {
	key := e.KeyOf(rec)
	key["valid_from"] = nil
	delete(key, "valid_from")
	w, args := e.keyWhere(key)
	cols := e.Columns()
	res, err := e.DB().Query(ctx, "SELECT "+strings.Join(cols, ", ")+" FROM "+e.Table+" WHERE "+w+" AND valid_to = ?",
		append(args, dayBefore(Str(rec["valid_from"])))...)
	if err != nil || len(res.Rows) == 0 {
		return nil, err
	}
	return e.scan(ctx, cols, res.Rows[0])
}

// MergeSlice fasst die Zeitscheibe {id} mit der vorherigen zusammen.
func (e *Entity) MergeSlice(ctx context.Context, payload any) (sdk.Response, error) {
	if !e.mergeable() {
		return sdk.Response{}, Invalid("%s: Zeitscheiben lassen sich nicht zusammenfassen", e.Title)
	}
	key, err := e.ParseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	var prevKey Record
	err = e.DB().InTx(ctx, nil, func(ctx context.Context) error {
		rec, err := e.Load(ctx, key)
		if err != nil {
			return err
		}
		if _, err := e.checkReadable(ctx, rec); err != nil {
			return err
		}
		prev, err := e.previousSlice(ctx, rec)
		if err != nil {
			return err
		}
		if prev == nil {
			return Invalid("Zeitscheibe ab %s: keine direkt vorangehende Zeitscheibe", Str(rec["valid_from"]))
		}
		if !e.sameSlice(prev, rec) {
			return Invalid("Zeitscheibe ab %s unterscheidet sich von der vorherigen (ab %s) – nicht zusammenfassbar", Str(rec["valid_from"]), Str(prev["valid_from"]))
		}
		if e.CheckRecord != nil {
			for _, r := range []Record{prev, rec} {
				if err := e.CheckRecord(ctx, "update", r); err != nil {
					return err
				}
			}
		}
		w, args := e.keyWhere(e.KeyOf(rec))
		if _, err := e.DB().Exec(ctx, "DELETE FROM "+e.Table+" WHERE "+w, args...); err != nil {
			return err
		}
		prevKey = e.KeyOf(prev)
		w, args = e.keyWhere(prevKey)
		_, err = e.DB().Exec(ctx, "UPDATE "+e.Table+" SET valid_to = ? WHERE "+w, append([]any{rec["valid_to"]}, args...)...)
		return err
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return e.respondEvent(ctx, prevKey, "update")
}

// hideMerge: Aktion nur an Zeitscheiben, die sich mit der vorherigen in recs zusammenfassen lassen.
func (e *Entity) hideMerge(recs []Record) {
	if !e.mergeable() {
		return
	}
	byEnd := map[string]Record{}
	bk := func(r Record) string {
		k := e.KeyOf(r)
		delete(k, "valid_from")
		return e.RecordID(k)
	}
	for _, r := range recs {
		byEnd[bk(r)+"@"+Str(r["valid_to"])] = r
	}
	for _, r := range recs {
		if prev, ok := byEnd[bk(r)+"@"+dayBefore(Str(r["valid_from"]))]; ok && e.sameSlice(prev, r) {
			continue
		}
		r["_hidden_actions"] = addHidden(r["_hidden_actions"], MergeAction)
	}
}

func addHidden(cur any, name string) []any {
	list, _ := cur.([]any)
	for _, v := range list {
		if v == name {
			return list
		}
	}
	return append(list, name)
}
