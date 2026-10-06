package crud

import (
	"context"
	"fmt"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Lebenszyklus: Physisch gelöscht wird nichts.
//
//	TimeSlice   → Expire {id, valid_to}: Enddatum, vom Benutzer gewählt
//	StatusField → Deactivate {id}: Status = StatusInactive
//	sonst       → immutable

// Lifecycle liefert den Lebenszyklus für das Metamodell.
func (e *Entity) Lifecycle() metamodel.Lifecycle {
	switch {
	case e.TimeSlice:
		return metamodel.Lifecycle{Type: metamodel.LifecycleTimeSlice, ValidFrom: "valid_from", ValidTo: "valid_to"}
	case e.StatusField != "":
		l := metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: e.StatusField}
		if _, inactive := e.statusValues(); inactive != false {
			l.InactiveValue = Str(inactive)
		}
		return l
	}
	return metamodel.Lifecycle{Type: metamodel.LifecycleImmutable}
}

// Expire beendet die Gültigkeit zum gewählten Datum (Pflicht, nie automatisch
// heute). Rückwirkend ist erlaubt, solange valid_to nicht vor valid_from
// liegt und nicht in eine folgende Zeitscheibe reicht.
func (e *Entity) Expire(ctx context.Context, payload any) (sdk.Response, error) {
	if !e.TimeSlice {
		return sdk.Response{}, fmt.Errorf("%w: %s hat keine Zeitscheibe", sdk.ErrUnimplemented, e.Title)
	}
	key, err := e.ParseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	m, _ := payload.(map[string]any)
	if Str(m["valid_to"]) == "" {
		return sdk.Response{}, Invalid("Enddatum (valid_to) ist Pflicht")
	}
	validTo, err := ParseDate(m["valid_to"])
	if err != nil {
		return sdk.Response{}, err
	}
	return e.endWith(ctx, key, "expire", func(ctx context.Context, rec Record) (string, any, error) {
		if from := Str(rec["valid_from"]); validTo < from {
			return "", nil, Invalid("Enddatum %s liegt vor dem Beginn der Gültigkeit (%s)", validTo, from)
		}
		changed := Record{}
		for k, v := range rec {
			changed[k] = v
		}
		changed["valid_to"] = validTo
		if err := e.checkOverlap(ctx, changed); err != nil {
			return "", nil, err
		}
		return "valid_to", validTo, nil
	})
}

// Deactivate setzt das Status-Feld auf StatusInactive.
func (e *Entity) Deactivate(ctx context.Context, payload any) (sdk.Response, error) {
	if e.StatusField == "" {
		return sdk.Response{}, fmt.Errorf("%w: %s hat kein Status-Feld", sdk.ErrUnimplemented, e.Title)
	}
	key, err := e.ParseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	_, inactive := e.statusValues()
	return e.endWith(ctx, key, "deactivate", func(context.Context, Record) (string, any, error) {
		return e.StatusField, inactive, nil
	})
}

func (e *Entity) endWith(ctx context.Context, key Record, action string, change func(ctx context.Context, rec Record) (string, any, error)) (sdk.Response, error) {
	err := e.DB().InTx(ctx, nil, func(ctx context.Context) error {
		rec, err := e.Load(ctx, key)
		if err != nil {
			return err
		}
		key = e.KeyOf(rec) // genau diese Zeitscheibe
		if e.CheckRecord != nil {
			if err := e.CheckRecord(ctx, action, rec); err != nil {
				return err
			}
		}
		col, value, err := change(ctx, rec)
		if err != nil {
			return err
		}
		w, args := e.keyWhere(key)
		_, err = e.DB().Exec(ctx, "UPDATE "+e.Table+" SET "+col+" = ? WHERE "+w, append([]any{value}, args...)...)
		return err
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return e.respond(ctx, key)
}

// checkOverlap: Zeitscheiben desselben fachlichen Schlüssels (alle
// Schlüsselteile außer valid_from) dürfen sich nicht überschneiden.
func (e *Entity) checkOverlap(ctx context.Context, rec Record) error {
	where := []string{"valid_from <> ?", "valid_from <= ?", "valid_to >= ?"}
	args := []any{rec["valid_from"], rec["valid_to"], rec["valid_from"]}
	for _, k := range e.Keys {
		if k != "valid_from" {
			where, args = append(where, k+" = ?"), append(args, rec[k])
		}
	}
	res, err := e.DB().Query(ctx, "SELECT valid_from, valid_to FROM "+e.Table+" WHERE "+strings.Join(where, " AND "), args...)
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		from, _ := ParseDate(res.Rows[0][0])
		to, _ := ParseDate(res.Rows[0][1])
		return Invalid("Zeitscheibe %s–%s überschneidet sich mit der bestehenden Zeitscheibe %s–%s", Str(rec["valid_from"]), Str(rec["valid_to"]), from, to)
	}
	return nil
}
