// Package pgtest stellt Tests eine frische PostgreSQL-Datenbank bereit.
//
// Die Tests laufen nur, wenn COREMESH_TEST_POSTGRES auf einen Server zeigt
// (URL-Form, Benutzer mit CREATEDB), z. B.:
//
//	COREMESH_TEST_POSTGRES=postgres://postgres:secret@localhost:5432/postgres?sslmode=disable
//
// Jeder Test bekommt eine eigene Datenbank coremesh_test_<zufall>, die am
// Ende wieder gelöscht wird.
package pgtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const Env = "COREMESH_TEST_POSTGRES"

// New liefert die DSN einer frischen, leeren Datenbank oder überspringt den Test.
func New(t *testing.T) string {
	t.Helper()
	base := os.Getenv(Env)
	if base == "" {
		t.Skipf("%s nicht gesetzt – PostgreSQL-Test übersprungen", Env)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("%s: %v", Env, err)
	}
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	rand.Read(b)
	name := "coremesh_test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		admin.Close()
		t.Fatalf("CREATE DATABASE: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		admin.Close()
	})
	u.Path = "/" + name
	return u.String()
}
