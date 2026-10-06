package dbschema

import (
	"context"
	"database/sql"
	"fmt"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
	"github.com/zclconf/go-cty/cty"

	"github.com/camel/coremesh/pkg/sdk"
)

// dialect bündelt, was Atlas pro Datenbanktyp braucht.
type dialect struct {
	open    func(schema.ExecQuerier) (migrate.Driver, error)
	evalHCL func(data []byte, v any, input map[string]cty.Value) error
	// Standard-Schema für den Präfix-Modus.
	defaultSchema string
	// Unterstützt eigene DB-Schemata pro Modul (IsolationSchema).
	schemas bool
}

var (
	sqliteDialect   = dialect{open: sqlite.Open, evalHCL: sqlite.EvalHCLBytes, defaultSchema: "main"}
	postgresDialect = dialect{open: postgres.Open, evalHCL: postgres.EvalHCLBytes, defaultSchema: "public", schemas: true}
)

// dialects nach database/sql-Treibername.
var dialects = map[string]dialect{
	"sqlite":   sqliteDialect,
	"pgx":      postgresDialect,
	"postgres": postgresDialect,
}

// parseSchema wertet das Atlas-HCL des Moduls aus – rein im Speicher,
// ohne SQL auszuführen – und ordnet alle Tabellen dem Ziel-Schema zu.
// Der Schema-Name im HCL (z. B. "main") ist damit frei wählbar.
func parseSchema(d dialect, hcl, target string) (*schema.Schema, error) {
	var s schema.Schema
	if err := d.evalHCL([]byte(hcl), &s, nil); err != nil {
		return nil, fmt.Errorf("%w: Schema-HCL: %v", sdk.ErrInvalidArgument, err)
	}
	s.Name = target
	for _, t := range s.Tables {
		t.Schema = &s
	}
	return &s, nil
}

// migrateModule plant und wendet die Schema-Differenz eines Moduls innerhalb
// von tx an und liefert die ausgeführten SQL-Statements (für Log/Audit).
func migrateModule(ctx context.Context, d dialect, tx *sql.Tx, sc scope, desired *schema.Schema) ([]string, error) {
	var stmts []string
	if sc.prefix == "" {
		// Schema-Modus: das Modul-Schema anlegen. Der Name stammt aus
		// sdk.SchemaName und enthält nur [a-z0-9_].
		stmt := `CREATE SCHEMA IF NOT EXISTS "` + sc.schema + `"`
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("Schema %s: %w", sc.schema, err)
		}
		stmts = append(stmts, stmt)
	}

	drv, err := d.open(tx)
	if err != nil {
		return nil, err
	}

	// Ist-Zustand: nur Objekte des Moduls.
	opts := &schema.InspectOptions{}
	if sc.prefix != "" {
		opts.Include = []string{sc.prefix + "*"}
	}
	current, err := drv.InspectSchema(ctx, sc.schema, opts)
	if err != nil {
		return nil, fmt.Errorf("Inspect %s: %w", sc.schema, err)
	}
	scopeCurrent(sc, current)

	changes, err := drv.SchemaDiff(current, desired)
	if err != nil {
		return nil, fmt.Errorf("Diff: %w", err)
	}
	if err := checkChanges(sc, changes); err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return stmts, nil
	}

	plan, err := drv.PlanChanges(ctx, sc.module, changes)
	if err != nil {
		return nil, fmt.Errorf("Plan: %w", err)
	}
	for _, c := range plan.Changes {
		stmts = append(stmts, c.Cmd)
	}
	if err := drv.ApplyChanges(ctx, changes); err != nil {
		return nil, fmt.Errorf("Apply: %w", err)
	}
	return stmts, nil
}
