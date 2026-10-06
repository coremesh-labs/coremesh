// Package txctx transportiert die laufenden Transaktionen einer Aufrufkette
// (Datenbank -> tx_id) im context.Context. Es liegt in internal/, damit
// Plugins tx_ids nicht selbst setzen können; öffentlich ist nur sdk.InTx.
package txctx

import (
	"context"
	"database/sql"
	"maps"
)

type key struct{}

// Manager ist die Transaktionssteuerung des Hosts. Im Plugin implementiert der
// gRPC-Host-Client das Interface, auf Host-Seite die Transaktionsverwaltung.
type Manager interface {
	BeginTx(ctx context.Context, database string, opts sql.TxOptions) (txID string, err error)
	CommitTx(ctx context.Context, txID string) error
	RollbackTx(ctx context.Context, txID string) error
}

// Get liefert die tx_id der laufenden Transaktion auf database.
func Get(ctx context.Context, database string) (string, bool) {
	id, ok := All(ctx)[database]
	return id, ok
}

// All liefert alle laufenden Transaktionen (nicht verändern).
func All(ctx context.Context) map[string]string {
	m, _ := ctx.Value(key{}).(map[string]string)
	return m
}

// With ergänzt eine Transaktion; bestehende Einträge bleiben erhalten.
func With(ctx context.Context, database, txID string) context.Context {
	m := maps.Clone(All(ctx))
	if m == nil {
		m = make(map[string]string, 1)
	}
	m[database] = txID
	return context.WithValue(ctx, key{}, m)
}

// WithAll ersetzt alle Transaktionen, z. B. beim Empfang über gRPC.
func WithAll(ctx context.Context, m map[string]string) context.Context {
	if len(m) == 0 && All(ctx) == nil {
		return ctx
	}
	return context.WithValue(ctx, key{}, m)
}
