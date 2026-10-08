package database

import (
	"errors"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

func TestCheckStatement(t *testing.T) {
	own := WriteScope{Plugin: "contract-billing", Prefix: "contract_billing__", Schema: "mod_contract_billing", Write: true}
	ok := []string{
		"SELECT * FROM contract__contract WHERE id = ?", // fremde Tabellen lesen bleibt erlaubt
		"SELECT 1;",
		"INSERT INTO contract_billing__run (id) VALUES (?)",
		"insert into main.contract_billing__run (id) values ($1)",
		`INSERT INTO "contract_billing__run" (id) VALUES (?)`,
		"INSERT OR REPLACE INTO contract_billing__run (id) VALUES (?)",
		"REPLACE INTO contract_billing__run (id) VALUES (?)",
		"UPDATE contract_billing__run SET status = 'DELETE FROM contract__contract' WHERE id = ?",
		"UPDATE OR IGNORE contract_billing__run SET status = ?",
		"DELETE FROM contract_billing__posting WHERE run_id = ?",
		"INSERT INTO contract_billing__run (id) VALUES (?) ON CONFLICT (id) DO UPDATE SET status = excluded.status",
		"SELECT * FROM contract__contract FOR UPDATE",
		"SELECT replace(name, 'a', 'b') FROM contract__contract",
		"WITH x AS (SELECT id FROM contract__contract) INSERT INTO contract_billing__run (id) SELECT id FROM x",
		"-- DELETE FROM contract__contract\nSELECT 1",
		"/* UPDATE ledger__x SET a = 1 */ SELECT 1",
		"SELECT $tag$ DELETE FROM ledger__x $tag$",
		"SELECT E'it\\'s DELETE FROM ledger__x'",
		"SELECT 'a;b' FROM contract__contract",
	}
	for _, q := range ok {
		if err := CheckStatement(q, own); err != nil {
			t.Errorf("erlaubt erwartet: %q: %v", q, err)
		}
	}
	denied := []string{
		"UPDATE contract__contract SET status = 'X'",
		"DELETE FROM ledger__journal_entry_item",
		"INSERT INTO ledger__journal_entry_header (id) VALUES (?)",
		"INSERT OR REPLACE INTO iam__users (id) VALUES (?)",
		"REPLACE INTO webserver__sessions (id) VALUES (?)",
		"UPDATE main.contract__contract SET a = 1",
		`UPDATE "Contract_billing__run" SET a = 1`, // quotiert: anderer Name
		"WITH d AS (DELETE FROM contract__contract RETURNING *) SELECT * FROM d",
		"SELECT * FROM (SELECT 1) x WHERE EXISTS (SELECT 1); DELETE FROM contract__contract",
		"SELECT 1; SELECT 2",
		"UPDATE contract_billing__run SET a = (SELECT 1); UPDATE contract__contract SET a = 1",
		"CREATE TABLE contract_billing__x (id text)",
		"DROP TABLE contract_billing__run",
		"ALTER TABLE contract_billing__run ADD COLUMN x text",
		"PRAGMA foreign_keys = OFF",
		"ATTACH DATABASE 'x.db' AS x",
		"SET search_path = mod_other",
		"COMMIT",
		"BEGIN",
		"VACUUM",
		"SELECT * INTO contract_billing__copy FROM contract__contract",
		"EXPLAIN ANALYZE DELETE FROM contract__contract",
		"MERGE INTO contract__contract USING x ON true WHEN MATCHED THEN DELETE",
		"UPDATE ONLY contract__contract SET a = 1",
		"SELECT 'offen",
		"",
	}
	for _, q := range denied {
		if err := CheckStatement(q, own); !errors.Is(err, sdk.ErrPermissionDenied) {
			t.Errorf("abgelehnt erwartet: %q: %v", q, err)
		}
	}
}

func TestCheckStatementReadOnlyAndSchema(t *testing.T) {
	ro := WriteScope{Plugin: "report", Prefix: "report__", Write: false}
	if err := CheckStatement("SELECT * FROM ledger__journal_entry_item", ro); err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if err := CheckStatement("INSERT INTO report__x (a) VALUES (1)", ro); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("schreiben ohne write-Freigabe: %v", err)
	}
	sc := WriteScope{Plugin: "report", Prefix: "report__", Schema: "mod_report", SchemaIsolation: true, Write: true}
	for q, want := range map[string]bool{
		"INSERT INTO x (a) VALUES (1)":                true,  // search_path = eigenes Schema
		"INSERT INTO mod_report.x (a) VALUES (1)":     true,
		"INSERT INTO mod_ledger.ledger__x VALUES (1)": false, // fremdes Schema
		"INSERT INTO public.report__x VALUES (1)":     false, // nicht das eigene Schema
	} {
		err := CheckStatement(q, sc)
		if (err == nil) != want {
			t.Errorf("Schema-Isolation %q: %v", q, err)
		}
	}
}
