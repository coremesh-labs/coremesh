// Package database verwaltet die SQL-Verbindungspools des Hosts und die
// Transaktionen, die Plugins über den HostService öffnen.
//
// SQL-Treiber werden vom Host-Binary per Blank-Import registriert
// (cmd/host: modernc.org/sqlite; weitere z. B. github.com/jackc/pgx/v5/stdlib).
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/internal/config"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Options steuert die Transaktionsverwaltung.
type Options struct {
	TxTimeout     time.Duration
	MaxTxPerOwner int
}

type pool struct {
	db     *sql.DB
	driver string
	cfg    config.Database

	mu     sync.Mutex
	scoped map[string]*sql.DB // search_path -> eigener Pool (nur PostgreSQL)
	owners map[string]string  // Plugin -> search_path
}

// Manager hält alle Pools und die offenen Transaktionen.
type Manager struct {
	pools map[string]*pool
	opts  Options
	log   *slog.Logger

	mu       sync.Mutex
	txs      map[string]*Tx
	perOwner map[string]int
}

// Open öffnet und prüft (Ping) alle konfigurierten Pools.
func Open(ctx context.Context, cfgs map[string]config.Database, opts Options, log *slog.Logger) (*Manager, error) {
	m := &Manager{
		pools:    make(map[string]*pool, len(cfgs)),
		opts:     opts,
		log:      log,
		txs:      map[string]*Tx{},
		perOwner: map[string]int{},
	}
	for _, name := range slices.Sorted(maps.Keys(cfgs)) {
		c := cfgs[name]
		db, err := sql.Open(c.Driver, c.DSN)
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("Datenbank %s: %w", name, err)
		}
		if c.MaxOpenConns > 0 {
			db.SetMaxOpenConns(c.MaxOpenConns)
		}
		if c.MaxIdleConns > 0 {
			db.SetMaxIdleConns(c.MaxIdleConns)
		}
		if c.ConnMaxLifetime > 0 {
			db.SetConnMaxLifetime(c.ConnMaxLifetime)
		}
		m.pools[name] = &pool{db: db, driver: c.Driver, cfg: c, scoped: map[string]*sql.DB{}, owners: map[string]string{}}

		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err = db.PingContext(pctx)
		cancel()
		if err != nil {
			m.Close()
			return nil, fmt.Errorf("Datenbank %s nicht erreichbar: %w", name, err)
		}
	}
	return m, nil
}

// Names liefert die logischen Pool-Namen, sortiert.
func (m *Manager) Names() []string {
	return slices.Sorted(maps.Keys(m.pools))
}

// DB liefert den Pool name.
func (m *Manager) DB(name string) (*sql.DB, error) {
	p, err := m.pool(name)
	if err != nil {
		return nil, err
	}
	return p.db, nil
}

// Driver liefert den Treibernamen des Pools name ("" wenn unbekannt).
func (m *Manager) Driver(name string) string {
	if p, ok := m.pools[name]; ok {
		return p.driver
	}
	return ""
}

func (m *Manager) pool(name string) (*pool, error) {
	p, ok := m.pools[name]
	if !ok {
		return nil, fmt.Errorf("%w: Datenbank %q", sdk.ErrNotFound, name)
	}
	return p, nil
}

// Close rollt offene Transaktionen zurück und schließt alle Pools.
func (m *Manager) Close() error {
	m.mu.Lock()
	txs := slices.Collect(maps.Values(m.txs))
	m.txs = map[string]*Tx{}
	m.mu.Unlock()
	for _, t := range txs {
		_ = m.finish(t, false, "Host wird beendet")
	}
	var errs []error
	for _, p := range m.pools {
		errs = append(errs, p.close()...)
	}
	return errors.Join(errs...)
}
