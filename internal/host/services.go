package host

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/coremesh-lab/coremesh/internal/config"
	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/internal/txctx"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

// pluginHost sind die Host-Dienste für genau ein Plugin. Jeder Aufruf wird
// gegen die laufende Wurzelanfrage (request_id) und die Datenbank-Freigaben
// des Plugins aus der Konfiguration geprüft. Modul-SQL nutzt immer
// ?-Platzhalter; für PostgreSQL wandelt der Host sie in $1, $2, … um.
type pluginHost struct {
	h       *Host
	name    string
	grants  map[string]config.Grant
	ingress bool
}

var (
	_ sdk.Host      = (*pluginHost)(nil)
	_ txctx.Manager = (*pluginHost)(nil)
)

func (h *Host) services(name string, pc config.Plugin) *pluginHost {
	return &pluginHost{h: h, name: name, grants: pc.Databases, ingress: pc.Ingress}
}

// Handle ruft über den Dispatcher ein anderes Plugin auf – innerhalb der
// laufenden Wurzelanfrage. Ein Ingress-Plugin (ingress: true) darf außerhalb
// einer laufenden Anfrage eine neue Wurzelanfrage starten; sein CallContext
// (Mandant, Benutzer) wird dann übernommen.
func (s *pluginHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	if s.ingress {
		if _, err := s.h.disp.Verify(ctx); err != nil {
			return s.h.disp.Handle(ctx, req)
		}
	}
	return s.h.disp.HandleNested(ctx, req)
}

func (s *pluginHost) Log(ctx context.Context, level sdk.LogLevel, msg string, fields map[string]string) error {
	call := sdk.CallFromContext(ctx)
	attrs := []any{"plugin", s.name}
	if call.RequestID != "" {
		attrs = append(attrs, "request_id", call.RequestID)
	}
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		attrs = append(attrs, k, fields[k])
	}
	s.h.log.Log(ctx, slogLevel(level), msg, attrs...)
	return nil
}

func (s *pluginHost) Query(ctx context.Context, db, query string, args ...any) (*sdk.QueryResult, error) {
	ctx, end, err := s.request(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	call, err := s.authorize(ctx, db, false)
	if err != nil {
		return nil, err
	}
	var res *sdk.QueryResult
	err = s.withQuerier(ctx, call, db, func(q database.Querier) (err error) {
		res, err = database.Query(ctx, q, database.Rebind(s.h.db.Driver(db), query), args...)
		return err
	})
	return res, err
}

func (s *pluginHost) Exec(ctx context.Context, db, query string, args ...any) (sdk.ExecResult, error) {
	ctx, end, err := s.request(ctx)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	defer end()
	call, err := s.authorize(ctx, db, true)
	if err != nil {
		return sdk.ExecResult{}, err
	}
	var res sdk.ExecResult
	err = s.withQuerier(ctx, call, db, func(q database.Querier) (err error) {
		res, err = database.Exec(ctx, q, database.Rebind(s.h.db.Driver(db), query), args...)
		return err
	})
	return res, err
}

func (s *pluginHost) BeginTx(ctx context.Context, db string, opts sql.TxOptions) (string, error) {
	call, err := s.authorize(ctx, db, !opts.ReadOnly)
	if err != nil {
		return "", err
	}
	return s.h.db.BeginTx(s.name, call.RequestID, db, opts)
}

func (s *pluginHost) CommitTx(ctx context.Context, txID string) error {
	call, err := s.h.disp.Verify(ctx)
	if err != nil {
		return err
	}
	return s.h.db.CommitTx(s.name, call.RequestID, txID)
}

func (s *pluginHost) RollbackTx(ctx context.Context, txID string) error {
	call, err := s.h.disp.Verify(ctx)
	if err != nil {
		return err
	}
	return s.h.db.RollbackTx(call.RequestID, txID)
}

// request stellt sicher, dass ctx zu einer laufenden Wurzelanfrage gehört.
// Ein Ingress-Plugin darf außerhalb einer Anfrage Query/Exec ausführen: Für
// die Dauer des einen Aufrufs wird eine eigene Wurzelanfrage eröffnet
// (Transaktionen sind so nicht möglich – sie endeten mit dem Aufruf).
func (s *pluginHost) request(ctx context.Context) (context.Context, func(), error) {
	if !s.ingress {
		return ctx, func() {}, nil
	}
	if _, err := s.h.disp.Verify(ctx); err == nil {
		return ctx, func() {}, nil
	}
	return s.h.disp.Begin(ctx)
}

// authorize prüft laufende Anfrage und Datenbank-Freigabe.
func (s *pluginHost) authorize(ctx context.Context, db string, write bool) (sdk.CallContext, error) {
	call, err := s.h.disp.Verify(ctx)
	if err != nil {
		return call, err
	}
	g, ok := s.grants[db]
	if !ok {
		return call, fmt.Errorf("%w: Plugin %s hat keinen Zugriff auf Datenbank %q", sdk.ErrPermissionDenied, s.name, db)
	}
	if write && g.Access != "write" {
		return call, fmt.Errorf("%w: Plugin %s darf auf %q nur lesen", sdk.ErrPermissionDenied, s.name, db)
	}
	return call, nil
}

// withQuerier führt fn in der laufenden Transaktion auf db aus, sonst auf dem Pool.
func (s *pluginHost) withQuerier(ctx context.Context, call sdk.CallContext, db string, fn func(database.Querier) error) error {
	if txID, ok := txctx.Get(ctx, db); ok {
		return s.h.db.WithTx(call.RequestID, db, txID, fn)
	}
	pool, err := s.h.db.PoolFor(db, s.name) // bei Schema-Isolation mit eigenem search_path
	if err != nil {
		return err
	}
	return fn(pool)
}

func slogLevel(l sdk.LogLevel) slog.Level {
	switch l {
	case sdk.LogDebug:
		return slog.LevelDebug
	case sdk.LogWarn:
		return slog.LevelWarn
	case sdk.LogError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
