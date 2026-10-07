// Package dbschema ist das Core-Plugin für Datenbankschemata. Es läuft im
// Host-Prozess mit direktem Zugriff auf die Pools.
//
// Migrationen werden pro (module_name, version) in coremesh_schema_migrations
// registriert. Actions des Objects DBSchema:
//
//   - CheckVersion {module, version} -> {migrated, executed_at, installed}
//   - Activate {module, version, schema} gleicht das Soll-Schema (Atlas-HCL)
//     mit Atlas gegen die Tabellen des Moduls ab – isoliert über das
//     Modul-Präfix und ohne destruktive Änderungen (siehe isolation.go) – und
//     registriert (module, version) in derselben Transaktion. Bereits
//     registrierte Versionen werden übersprungen (idempotent).
//   - Activate ohne Payload (Host-Start): legt die Migrationstabelle an und
//     wendet die Host-eigenen SQL-Migrationen aus settings.migrations_dir an –
//     jede Datei als Version des Moduls "coremesh".
//   - Status: alle registrierten Migrationen.
//
// Activate und CheckVersion sind Host-Routen; Plugins erreichen sie nicht.
// Aufrufbeschreibung für Modul-Entwickler: README.md.
package dbschema

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coremesh-lab/coremesh/internal/config"
	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

const (
	Name       = "dbschema"
	Version    = "0.5.0"
	HostModule = "coremesh"
	table      = "coremesh_schema_migrations"
)

type Plugin struct {
	db       *database.Manager
	settings settings
}

type settings struct {
	Database      string `json:"database"`
	MigrationsDir string `json:"migrations_dir"`
	// prefix (Standard) oder schema (eigenes DB-Schema pro Modul, nur PostgreSQL).
	Isolation string `json:"isolation"`
}

func New(db *database.Manager) *Plugin { return &Plugin{db: db} }

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:        Name,
		Version:     Version,
		Description: "Datenbankschemata und Migrationen pro Modulversion",
		Capabilities: []sdk.Capability{
			{Object: sdk.ObjectDBSchema, Actions: []string{"Activate", "CheckVersion", "Status"}},
		},
	}, nil
}

// Configure liest die Einstellungen und legt die Migrationstabelle an, damit
// CheckVersion schon vor dem ersten Activate funktioniert.
func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	if err := sdk.Decode(cfg.Settings, &p.settings); err != nil {
		return err
	}
	if p.settings.Database == "" {
		p.settings.Database = "main"
	}
	switch p.settings.Isolation {
	case "":
		p.settings.Isolation = IsolationPrefix
	case IsolationPrefix:
	case IsolationSchema:
		if d, ok := dialects[p.db.Driver(p.settings.Database)]; !ok || !d.schemas {
			return fmt.Errorf("%w: isolation: schema braucht PostgreSQL (Treiber ist %q)", sdk.ErrInvalidArgument, p.db.Driver(p.settings.Database))
		}
	default:
		return fmt.Errorf("%w: isolation muss prefix oder schema sein", sdk.ErrInvalidArgument)
	}
	db, err := p.db.DB(p.settings.Database)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+table+` (
		module_name VARCHAR(255) NOT NULL,
		version     VARCHAR(64)  NOT NULL,
		checksum    VARCHAR(64)  NOT NULL,
		executed_at VARCHAR(40)  NOT NULL,
		PRIMARY KEY (module_name, version))`)
	if err != nil {
		return fmt.Errorf("Tabelle %s: %w", table, err)
	}
	return nil
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Action {
	case "CheckVersion":
		var in moduleVersion
		if err := decodeModule(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return p.checkVersion(ctx, in)
	case "Activate":
		if req.Payload == nil {
			return p.bootstrap(ctx)
		}
		var in activateInput
		if err := decodeModule(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return p.activateModule(ctx, in)
	case "Status":
		return p.status(ctx)
	}
	return sdk.Response{}, fmt.Errorf("%w: DBSchema.%s", sdk.ErrUnimplemented, req.Action)
}

type moduleVersion struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

func (m moduleVersion) key() moduleVersion { return m }

type activateInput struct {
	moduleVersion
	Schema string           `json:"schema"` // Atlas-HCL
	Seed   []sdk.SchemaSeed `json:"seed"`   // Stammdaten, nur fehlende Zeilen
}

type keyed interface{ key() moduleVersion }

func decodeModule[T keyed](payload any, dst *T) error {
	if err := sdk.Decode(payload, dst); err != nil {
		return err
	}
	if k := (*dst).key(); k.Module == "" || k.Version == "" {
		return fmt.Errorf("%w: module und version sind Pflicht", sdk.ErrInvalidArgument)
	}
	return nil
}

// checkVersion prüft, ob (module, version) bereits migriert ist.
func (p *Plugin) checkVersion(ctx context.Context, in moduleVersion) (sdk.Response, error) {
	db, _ := p.db.DB(p.settings.Database)
	rebind := p.rebinder(p.settings.Database)

	res, err := database.Query(ctx, db,
		rebind(`SELECT version, executed_at FROM `+table+` WHERE module_name = ? ORDER BY executed_at, version`), in.Module)
	if err != nil {
		return sdk.Response{}, err
	}
	out := map[string]any{"module": in.Module, "version": in.Version, "migrated": false, "installed": []any{}}
	installed := []any{}
	for _, r := range res.Rows {
		installed = append(installed, r[0])
		if r[0] == in.Version {
			out["migrated"] = true
			out["executed_at"] = r[1]
		}
	}
	out["installed"] = installed
	return sdk.Response{Payload: out}, nil
}

// activateModule migriert das Schema eines Moduls mit Atlas:
//
//  1. Modulname prüfen (gültig, nicht reserviert)
//  2. Soll-Schema (HCL) im Speicher auswerten
//  3. Isolation prüfen: alle Namen mit Modul-Präfix, keine fremden Verweise
//  4. In einer Transaktion: Version registrieren, Ist-Zustand der
//     Modul-Tabellen lesen, Diff bilden, Änderungen prüfen (keine DROPs),
//     anwenden, committen.
func (p *Plugin) activateModule(ctx context.Context, in activateInput) (sdk.Response, error) {
	if err := validModule(in.Module); err != nil {
		return sdk.Response{}, err
	}
	d, ok := dialects[p.db.Driver(p.settings.Database)]
	if !ok {
		return sdk.Response{}, fmt.Errorf("%w: Modul-Migrationen für Treiber %q", sdk.ErrUnimplemented, p.db.Driver(p.settings.Database))
	}
	sc := newScope(p.settings.Isolation, in.Module, d.defaultSchema)
	desired, err := parseSchema(d, in.Schema, sc.schema)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := checkDesired(sc, desired); err != nil {
		return sdk.Response{}, err
	}

	var stmts []string
	var seeded int64
	applied, err := p.inTx(ctx, in.Module, in.Version, checksum(in.Schema), func(tx *sql.Tx) (err error) {
		if stmts, err = migrateModule(ctx, d, tx, sc, desired); err != nil {
			return err
		}
		seeded, err = applySeeds(ctx, tx, p.db.Driver(p.settings.Database), d, sc, desired, in.Seed)
		return err
	})
	if err != nil {
		return sdk.Response{}, fmt.Errorf("Migration %s %s: %w", in.Module, in.Version, err)
	}
	if stmts == nil {
		stmts = []string{}
	}
	return sdk.Response{Payload: map[string]any{
		"module": in.Module, "version": in.Version, "database": p.settings.Database,
		"isolation": p.settings.Isolation, "schema": sc.schema, "seeded": seeded,
		"applied": applied, "statements": stmts,
	}}, nil
}

// inTx registriert (module, version) und führt fn in derselben Transaktion
// aus. Der Eintrag wird zuerst geschrieben: Starten zwei Hosts gleichzeitig,
// scheitert der zweite am Primärschlüssel und seine Änderungen werden
// zurückgerollt. Liefert false, wenn die Version bereits registriert war.
func (p *Plugin) inTx(ctx context.Context, module, version, sum string, fn func(*sql.Tx) error) (bool, error) {
	db, err := p.db.DB(p.settings.Database)
	if err != nil {
		return false, err
	}
	rebind := p.rebinder(p.settings.Database)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() // no-op nach Commit

	if exists, err := migrated(ctx, tx, rebind, module, version); err != nil || exists {
		return false, err
	}
	if _, err := tx.ExecContext(ctx,
		rebind(`INSERT INTO `+table+` (module_name, version, checksum, executed_at) VALUES (?, ?, ?, ?)`),
		module, version, sum, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return false, fmt.Errorf("Registrierung: %w", err)
	}
	if err := fn(tx); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// validModule: Modulname nach Plugin-Konvention, "coremesh" ist dem Host vorbehalten.
func validModule(module string) error {
	if !config.ValidPluginName(module) {
		return fmt.Errorf("%w: ungültiger Modulname %q", sdk.ErrInvalidArgument, module)
	}
	if module == HostModule {
		return fmt.Errorf("%w: Modulname %q ist reserviert", sdk.ErrPermissionDenied, module)
	}
	return nil
}

func checksum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func migrated(ctx context.Context, q database.Querier, rebind func(string) string, module, version string) (bool, error) {
	res, err := database.Query(ctx, q,
		rebind(`SELECT 1 FROM `+table+` WHERE module_name = ? AND version = ?`), module, version)
	if err != nil {
		return false, err
	}
	return len(res.Rows) > 0, nil
}

// bootstrap wendet die Host-eigenen Migrationen aus migrations_dir an.
func (p *Plugin) bootstrap(ctx context.Context) (sdk.Response, error) {
	files, err := p.hostMigrations()
	if err != nil {
		return sdk.Response{}, err
	}
	applied := []any{}
	for _, f := range files {
		// Host-Migrationen sind vertrauenswürdig und laufen als rohes SQL.
		ok, err := p.inTx(ctx, HostModule, f.version, checksum(f.sql), func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, f.sql)
			return err
		})
		if err != nil {
			return sdk.Response{}, fmt.Errorf("Migration %s: %w", f.version, err)
		}
		if ok {
			applied = append(applied, f.version)
		}
	}
	return sdk.Response{Payload: map[string]any{
		"module": HostModule, "database": p.settings.Database, "applied": applied,
	}}, nil
}

func (p *Plugin) status(ctx context.Context) (sdk.Response, error) {
	db, _ := p.db.DB(p.settings.Database)
	res, err := database.Query(ctx, db,
		`SELECT module_name, version, executed_at FROM `+table+` ORDER BY module_name, executed_at, version`)
	if err != nil {
		return sdk.Response{}, err
	}
	out := make([]any, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = map[string]any{"module": r[0], "version": r[1], "executed_at": r[2]}
	}
	return sdk.Response{Payload: out}, nil
}

type hostMigration struct{ version, sql string }

// hostMigrations liest *.sql aus migrations_dir, alphanumerisch sortiert;
// der Dateiname ohne .sql ist die Version.
func (p *Plugin) hostMigrations() ([]hostMigration, error) {
	if p.settings.MigrationsDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(p.settings.MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("migrations_dir: %w", err)
	}
	var out []hostMigration
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || !strings.EqualFold(ext, ".sql") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p.settings.MigrationsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, hostMigration{version: strings.TrimSuffix(e.Name(), ext), sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func (p *Plugin) rebinder(dbName string) func(string) string {
	driver := p.db.Driver(dbName)
	return func(q string) string { return database.Rebind(driver, q) }
}
