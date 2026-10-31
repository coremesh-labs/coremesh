package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestTxLockImmediate: Im WAL-Modus scheitert eine Transaktion, die erst liest und dann
// schreibt, sofort mit SQLITE_BUSY, wenn zwischendurch eine andere Verbindung geschrieben
// hat – busy_timeout hilft dann nicht. Mit _txlock=immediate wartet der zweite Schreiber.
func TestTxLockImmediate(t *testing.T) {
	run := func(dsnExtra string) error {
		dsn := "file:" + filepath.Join(t.TempDir(), "t.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" + dsnExtra
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		db.SetMaxOpenConns(4)
		if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := db.Exec(`INSERT INTO t (id) VALUES (1)`); done <- err }()
		waited := false
		select {
		case err := <-done: // deferred: der andere Schreiber kommt sofort durch
			if err != nil {
				return err
			}
		case <-time.After(300 * time.Millisecond): // immediate: er wartet auf uns
			waited = true
		}
		if _, err := tx.Exec(`INSERT INTO t (id) VALUES (2)`); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if waited {
			return <-done
		}
		return nil
	}
	if err := run(""); err == nil || !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "BUSY") {
		t.Fatalf("deferred: erwartet SQLITE_BUSY, war %v", err)
	}
	if err := run("&_txlock=immediate"); err != nil {
		t.Fatalf("immediate: %v", err)
	}
}
