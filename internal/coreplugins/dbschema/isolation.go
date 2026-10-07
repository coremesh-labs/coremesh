package dbschema

import (
	"errors"
	"fmt"
	"strings"

	"ariga.io/atlas/sql/schema"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Schutzprüfungen für Modul-Migrationen.
//
// Die Isolation ist dreifach abgesichert:
//
//  1. checkDesired: Das Soll-Schema des Moduls enthält nur Objekte des Moduls
//     und keine Views, Sequenzen o. ä. Fremdschlüssel dürfen nur auf eigene
//     Tabellen zeigen.
//  2. Ist-Zustand: Atlas liest nur die Objekte des Moduls – im Präfix-Modus
//     per Include-Filter plus exaktem Präfix-Filter (scopeCurrent), im
//     Schema-Modus nur das DB-Schema des Moduls.
//  3. checkChanges: Jede von Atlas geplante Änderung wird geprüft, bevor
//     sie ausgeführt wird. Destruktive (DROP TABLE, DROP COLUMN, Renames)
//     und unbekannte Änderungsarten werden abgelehnt (fail closed).

const (
	// IsolationPrefix: Modul-Tabellen liegen im Standard-Schema und tragen
	// das Präfix sdk.TablePrefix(module). Für SQLite und PostgreSQL.
	IsolationPrefix = "prefix"
	// IsolationSchema: Jedes Modul hat ein eigenes DB-Schema
	// sdk.SchemaName(module). Nur PostgreSQL.
	IsolationSchema = "schema"
)

// ErrDestructive: Die Migration würde Tabellen oder Spalten löschen.
var ErrDestructive = fmt.Errorf("%w: destruktive Schema-Änderung", sdk.ErrPermissionDenied)

// scope beschreibt, welche Objekte einem Modul gehören.
type scope struct {
	module string
	schema string // DB-Schema der Modul-Tabellen ("main", "public", "mod_partner")
	prefix string // Namens-Präfix; leer im Schema-Modus
}

func newScope(isolation, module, defaultSchema string) scope {
	if isolation == IsolationSchema {
		return scope{module: module, schema: sdk.SchemaName(module)}
	}
	return scope{module: module, schema: defaultSchema, prefix: sdk.TablePrefix(module)}
}

// ownsTable: Im Präfix-Modus zählt der Name, im Schema-Modus das DB-Schema.
func (s scope) ownsTable(t *schema.Table) bool {
	if t == nil {
		return false
	}
	if s.prefix != "" {
		return owned(s.prefix, t.Name)
	}
	return t.Schema != nil && t.Schema.Name == s.schema
}

// ownsName prüft Index- und Constraint-Namen. Im Schema-Modus sind sie pro
// Schema eindeutig und brauchen kein Präfix.
func (s scope) ownsName(name string) bool {
	return s.prefix == "" || owned(s.prefix, name)
}

func (s scope) describe() string {
	if s.prefix != "" {
		return fmt.Sprintf("Präfix %q", s.prefix)
	}
	return fmt.Sprintf("Schema %q", s.schema)
}

// checkDesired prüft das Soll-Schema eines Moduls.
func checkDesired(sc scope, s *schema.Schema) error {
	var errs []error
	if len(s.Views) > 0 {
		errs = append(errs, fmt.Errorf("Views sind nicht erlaubt (%s)", s.Views[0].Name))
	}
	if len(s.Objects) > 0 {
		errs = append(errs, errors.New("Schema-Objekte (Typen, Sequenzen, …) sind nicht erlaubt"))
	}
	for _, t := range s.Tables {
		if !sc.ownsTable(t) {
			errs = append(errs, fmt.Errorf("Tabelle %q gehört nicht zum Modul (%s)", t.Name, sc.describe()))
			continue
		}
		for _, idx := range t.Indexes {
			if !sc.ownsName(idx.Name) {
				errs = append(errs, fmt.Errorf("Index %q auf %s: Name muss mit %q beginnen", idx.Name, t.Name, sc.prefix))
			}
		}
		for _, fk := range t.ForeignKeys {
			if fk.Symbol != "" && !sc.ownsName(fk.Symbol) {
				errs = append(errs, fmt.Errorf("Fremdschlüssel %q auf %s: Name muss mit %q beginnen", fk.Symbol, t.Name, sc.prefix))
			}
			if !sc.ownsTable(fk.RefTable) {
				errs = append(errs, fmt.Errorf("Fremdschlüssel auf %s: Verweise auf Tabellen anderer Module sind nicht erlaubt", t.Name))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: Schema von Modul %s verletzt die Isolation:\n%w", sdk.ErrPermissionDenied, sc.module, errors.Join(errs...))
	}
	return nil
}

// scopeCurrent reduziert den Ist-Zustand exakt auf die Tabellen des Moduls.
func scopeCurrent(sc scope, s *schema.Schema) {
	tables := s.Tables[:0]
	for _, t := range s.Tables {
		if sc.ownsTable(t) {
			tables = append(tables, t)
		}
	}
	s.Tables = tables
	s.Views, s.Objects = nil, nil
}

// checkChanges prüft jede von Atlas geplante Änderung.
func checkChanges(sc scope, changes []schema.Change) error {
	var errs []error
	for _, c := range changes {
		switch c := c.(type) {
		case *schema.AddTable:
			if !sc.ownsTable(c.T) {
				errs = append(errs, fmt.Errorf("CREATE TABLE %s: fremde Tabelle", c.T.Name))
			}
		case *schema.ModifyTable:
			if !sc.ownsTable(c.T) {
				errs = append(errs, fmt.Errorf("ALTER TABLE %s: fremde Tabelle", c.T.Name))
				continue
			}
			errs = append(errs, checkTableChanges(sc, c.T.Name, c.Changes)...)
		case *schema.DropTable:
			errs = append(errs, fmt.Errorf("%w: DROP TABLE %s", ErrDestructive, c.T.Name))
		case *schema.RenameTable:
			errs = append(errs, fmt.Errorf("%w: RENAME TABLE %s → %s", ErrDestructive, c.From.Name, c.To.Name))
		default:
			// Schemas, Views, Funktionen, Objekte … – fail closed.
			errs = append(errs, fmt.Errorf("nicht erlaubte Änderung %T", c))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: Migration von Modul %s abgelehnt:\n%w", sdk.ErrPermissionDenied, sc.module, errors.Join(errs...))
	}
	return nil
}

func checkTableChanges(sc scope, table string, changes []schema.Change) []error {
	var errs []error
	for _, c := range changes {
		switch c := c.(type) {
		case *schema.AddColumn, *schema.ModifyColumn,
			*schema.AddPrimaryKey, *schema.ModifyPrimaryKey, *schema.DropPrimaryKey,
			*schema.AddCheck, *schema.ModifyCheck, *schema.DropCheck,
			*schema.DropIndex, *schema.DropForeignKey,
			*schema.AddAttr, *schema.ModifyAttr, *schema.DropAttr:
			// Erlaubt: verändert keine fremden Objekte und löscht keine Daten.
		case *schema.DropColumn:
			errs = append(errs, fmt.Errorf("%w: DROP COLUMN %s.%s", ErrDestructive, table, c.C.Name))
		case *schema.RenameColumn:
			errs = append(errs, fmt.Errorf("%w: RENAME COLUMN %s.%s → %s", ErrDestructive, table, c.From.Name, c.To.Name))
		case *schema.AddIndex:
			if !sc.ownsName(c.I.Name) {
				errs = append(errs, fmt.Errorf("CREATE INDEX %s: Name muss mit %q beginnen", c.I.Name, sc.prefix))
			}
		case *schema.ModifyIndex:
			if !sc.ownsName(c.To.Name) {
				errs = append(errs, fmt.Errorf("Index %s: Name muss mit %q beginnen", c.To.Name, sc.prefix))
			}
		case *schema.AddForeignKey:
			if !sc.ownsTable(c.F.RefTable) {
				errs = append(errs, fmt.Errorf("Fremdschlüssel auf %s: Verweis auf fremde Tabelle", table))
			}
		case *schema.ModifyForeignKey:
			if !sc.ownsTable(c.To.RefTable) {
				errs = append(errs, fmt.Errorf("Fremdschlüssel auf %s: Verweis auf fremde Tabelle", table))
			}
		default:
			errs = append(errs, fmt.Errorf("nicht erlaubte Änderung %T an %s", c, table))
		}
	}
	return errs
}

// owned meldet, ob name zum Modul mit diesem Präfix gehört.
func owned(prefix, name string) bool {
	return len(name) > len(prefix) && strings.HasPrefix(name, prefix)
}
