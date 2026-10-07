package database

import (
	"context"
	"log/slog"
	"testing"

	"github.com/coremesh-labs/coremesh/internal/config"
	"github.com/coremesh-labs/coremesh/internal/testutil/pgtest"
)

func TestWithSearchPath(t *testing.T) {
	if got := withSearchPath("postgres://u:p@h:5432/db?sslmode=disable", "mod_partner"); got != "postgres://u:p@h:5432/db?search_path=mod_partner&sslmode=disable" {
		t.Error(got)
	}
	if got := withSearchPath("host=h dbname=db", "mod_partner"); got != "host=h dbname=db search_path=mod_partner" {
		t.Error(got)
	}
}

func TestSetScopeRequiresPostgres(t *testing.T) {
	m := openTest(t, Options{})
	if err := m.SetScope("main", "partner", "mod_partner"); err == nil {
		t.Fatal("SQLite darf keinen search_path pro Plugin bekommen")
	}
}

func TestPoolForPostgres(t *testing.T) {
	dsn := pgtest.New(t)
	m, err := Open(context.Background(), map[string]config.Database{"main": {Driver: "pgx", DSN: dsn}}, Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.SetScope("main", "partner", "mod_partner"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetScope("main", "evil", "x; DROP SCHEMA public"); err == nil {
		t.Fatal("ungültiger search_path muss abgelehnt werden")
	}
	searchPath := func(owner string) string {
		db, err := m.PoolFor("main", owner)
		if err != nil {
			t.Fatal(err)
		}
		var sp string
		if err := db.QueryRow(`SHOW search_path`).Scan(&sp); err != nil {
			t.Fatal(err)
		}
		return sp
	}
	if sp := searchPath("partner"); sp != "mod_partner" {
		t.Fatalf("Modul-Pool: search_path=%q", sp)
	}
	if sp := searchPath("orders"); sp == "mod_partner" {
		t.Fatalf("Standard-Pool darf nicht verändert sein: %q", sp)
	}
}
