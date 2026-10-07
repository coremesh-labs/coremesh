package sdk

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/coremesh-lab/coremesh/internal/txctx"
)

// InTx führt fn in einer Transaktion auf der Datenbank database aus.
// Alle Host-Aufrufe mit dem an fn übergebenen ctx laufen in ihr – Query und
// Exec ebenso wie Plugins, die per HostFrom(ctx).Handle aufgerufen werden:
//
//	err := sdk.InTx(ctx, "main", nil, func(ctx context.Context) error {
//		host := sdk.HostFrom(ctx)
//		if _, err := host.Exec(ctx, "main", "UPDATE orders SET status = ? WHERE id = ?", "confirmed", id); err != nil {
//			return err
//		}
//		_, err := host.Handle(ctx, sdk.Request{Object: "Stock", Action: "reserve", Payload: items})
//		return err
//	})
//
// Regeln:
//   - fn liefert nil: Commit. Fehler oder panic: Rollback.
//   - Läuft auf database bereits eine Transaktion (auch aus einem aufrufenden
//     Plugin), nimmt fn an ihr teil. Commit bleibt dem Eröffner vorbehalten;
//     scheitert fn, wird die Transaktion sofort zurückgerollt und jeder
//     weitere Zugriff liefert ErrTxAborted.
//   - opts gilt nur beim Eröffnen (nil = Standard der Datenbank).
//   - Eine Transaktion verbindet keine anderen Datenbanken atomar.
func InTx(ctx context.Context, database string, opts *sql.TxOptions, fn func(ctx context.Context) error) (err error) {
	mgr, ok := HostFrom(ctx).(txctx.Manager)
	if !ok {
		return fmt.Errorf("%w: Host unterstützt keine Transaktionen", ErrUnimplemented)
	}

	if txID, ok := txctx.Get(ctx, database); ok {
		// Teilnahme an einer laufenden Transaktion.
		return runTx(ctx, fn, mgr, txID)
	}

	var o sql.TxOptions
	if opts != nil {
		o = *opts
	}
	txID, err := mgr.BeginTx(ctx, database, o)
	if err != nil {
		return fmt.Errorf("BeginTx %s: %w", database, err)
	}
	txCtx := txctx.With(ctx, database, txID)
	if err := runTx(txCtx, fn, mgr, txID); err != nil {
		return err
	}
	if err := mgr.CommitTx(txCtx, txID); err != nil {
		return fmt.Errorf("CommitTx: %w", err)
	}
	return nil
}

// runTx führt fn aus und rollt bei Fehler oder panic zurück.
func runTx(ctx context.Context, fn func(context.Context) error, mgr txctx.Manager, txID string) (err error) {
	// Rollback auch dann, wenn ctx bereits abgebrochen ist.
	rollback := func() error { return mgr.RollbackTx(context.WithoutCancel(ctx), txID) }

	defer func() {
		if p := recover(); p != nil {
			_ = rollback()
			panic(p)
		}
	}()

	if err := fn(ctx); err != nil {
		if rbErr := rollback(); rbErr != nil && !errors.Is(rbErr, ErrTxAborted) {
			return errors.Join(err, fmt.Errorf("RollbackTx: %w", rbErr))
		}
		return err
	}
	return nil
}

// WithoutTx liefert einen ctx ohne laufende Transaktionen, z. B. für einen
// Audit-Eintrag, der einen Rollback überleben soll.
func WithoutTx(ctx context.Context) context.Context {
	return txctx.WithAll(ctx, map[string]string{})
}

// InTransaction meldet, ob auf database eine Transaktion läuft.
func InTransaction(ctx context.Context, database string) bool {
	_, ok := txctx.Get(ctx, database)
	return ok
}
