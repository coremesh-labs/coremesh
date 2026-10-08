// Package bpref stellt Verweise auf Geschäftspartner um, deren Schlüssel sich
// geändert hat (partner 0.10.0: BP-Nummer statt GUID).
//
// Ein Modul mit Partnerverweisen ruft Remap in seinem Migrator auf
// (module.Migrator): Die Werte der genannten Spalten gehen an
// BusinessPartnerService.resolve; was dort als alte ID bekannt ist, wird in der
// eigenen Tabelle durch die neue ersetzt. Wiederholbar – Umgestelltes kennt
// resolve nicht mehr.
//
//	func (m *Module) Migrate(ctx context.Context) error {
//		return bpref.Remap(ctx, m.db, m.services, m.log,
//			bpref.Column{Table: "contract__contract", Column: "partner_id"})
//	}
package bpref

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Object und Action der Auflösung (Partnermodul).
const (
	Object        = "BusinessPartnerService"
	ActionResolve = "resolve"
)

// Column: Spalte einer eigenen Tabelle mit Partnerverweisen.
type Column struct {
	Table  string
	Column string
}

// Resolve liefert zu alten Partner-IDs die neuen (unbekannte und aktuelle fehlen).
func Resolve(ctx context.Context, s module.Services, ids []string) (map[string]string, error) {
	resp, err := s.Call(ctx, Object, ActionResolve, map[string]any{"ids": ids})
	if err != nil {
		return nil, err
	}
	var out struct {
		IDs map[string]string `json:"ids"`
	}
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return nil, err
	}
	return out.IDs, nil
}

// Remap stellt die Spalten um. Ist das Partnermodul nicht erreichbar, bleibt
// alles, wie es ist (Warnung; der nächste Prozessstart versucht es erneut).
func Remap(ctx context.Context, db module.DB, s module.Services, log *slog.Logger, cols ...Column) error {
	var ids []string
	seen := map[string]bool{}
	for _, c := range cols {
		res, err := db.Query(ctx, "SELECT DISTINCT "+c.Column+" FROM "+c.Table+" WHERE "+c.Column+" IS NOT NULL AND "+c.Column+" <> ''")
		if err != nil {
			return err
		}
		for _, r := range res.Rows {
			if id := fmt.Sprint(r[0]); !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	mapping, err := Resolve(ctx, s, ids)
	if errors.Is(err, sdk.ErrUnimplemented) || errors.Is(err, sdk.ErrUnavailable) {
		log.WarnContext(ctx, "Partnerverweise nicht umgestellt – Partnermodul nicht erreichbar", "err", err.Error())
		return nil
	}
	if err != nil || len(mapping) == 0 {
		return err
	}
	return db.InTx(ctx, nil, func(ctx context.Context) error {
		for old, id := range mapping {
			for _, c := range cols {
				if _, err := db.Exec(ctx, "UPDATE "+c.Table+" SET "+c.Column+" = ? WHERE "+c.Column+" = ?", id, old); err != nil {
					return err
				}
			}
		}
		log.InfoContext(ctx, "Partnerverweise umgestellt", "partner", len(mapping))
		return nil
	})
}
