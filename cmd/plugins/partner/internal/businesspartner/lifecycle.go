package businesspartner

import (
	"context"
	"fmt"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// Lebenszyklus: Physisch gelöscht wird nichts. Wie ein Datensatz endet,
// folgt aus der Entität:
//
//	TimeSlice   (valid_from/valid_to) → expire {id, valid_to}: Enddatum, vom Benutzer gewählt
//	StatusField (z. B. is_active)     → deactivate {id}: Flag = false
//	sonst                             → immutable: weder löschen noch deaktivieren
//
// Eine Entität hat höchstens eines von beiden.

func (e *entity) lifecycle() metamodel.Lifecycle {
	switch {
	case e.TimeSlice:
		return metamodel.Lifecycle{Type: metamodel.LifecycleTimeSlice, ValidFrom: "valid_from", ValidTo: "valid_to"}
	case e.StatusField != "":
		return metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: e.StatusField}
	}
	return metamodel.Lifecycle{Type: metamodel.LifecycleImmutable}
}

// expire beendet die Gültigkeit zum gewählten Datum. Das Datum ist Pflicht –
// es wird nie automatisch das Tagesdatum gesetzt. Rückwirkend ist erlaubt,
// solange valid_to nicht vor valid_from liegt.
func (e *entity) expire(ctx context.Context, payload any) (sdk.Response, error) {
	if !e.TimeSlice {
		return sdk.Response{}, fmt.Errorf("%w: %s hat keine Zeitscheibe", sdk.ErrUnimplemented, e.Title)
	}
	key, err := e.parseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	m, _ := payload.(map[string]any)
	if str(m["valid_to"]) == "" {
		return sdk.Response{}, invalid("Enddatum (valid_to) ist Pflicht")
	}
	validTo, err := parseDate(m["valid_to"])
	if err != nil {
		return sdk.Response{}, err
	}
	return e.endWith(ctx, key, "expire", func(rec record) (string, any, error) {
		if from := str(rec["valid_from"]); validTo < from {
			return "", nil, invalid("Enddatum %s liegt vor dem Beginn der Gültigkeit (%s)", validTo, from)
		}
		return "valid_to", validTo, nil
	})
}

// deactivate setzt das Status-Flag auf false. Bereits inaktive Datensätze
// bleiben unverändert.
func (e *entity) deactivate(ctx context.Context, payload any) (sdk.Response, error) {
	if e.StatusField == "" {
		return sdk.Response{}, fmt.Errorf("%w: %s hat kein Status-Flag", sdk.ErrUnimplemented, e.Title)
	}
	key, err := e.parseID(idOf(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	return e.endWith(ctx, key, "deactivate", func(record) (string, any, error) {
		return e.StatusField, false, nil
	})
}

// endWith lädt den Datensatz, prüft den Zugriff und setzt ein Feld – in einer Transaktion.
func (e *entity) endWith(ctx context.Context, key record, action string, change func(rec record) (string, any, error)) (sdk.Response, error) {
	var saved record
	err := e.m.db.InTx(ctx, nil, func(ctx context.Context) error {
		rec, err := e.load(ctx, key)
		if err != nil {
			return err
		}
		if e.checkRecord != nil {
			if err := e.checkRecord(ctx, action, rec); err != nil {
				return err
			}
		}
		col, value, err := change(rec)
		if err != nil {
			return err
		}
		w, args := e.keyWhere(key)
		if _, err := e.m.db.Exec(ctx, "UPDATE "+e.Table+" SET "+col+" = ? WHERE "+w, append([]any{value}, args...)...); err != nil {
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
