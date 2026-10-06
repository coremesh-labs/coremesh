package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/pkg/sdk"
)

func openTest(t *testing.T, opts Options) *Manager {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=busy_timeout(5000)"
	m, err := Open(context.Background(), map[string]config.Database{"main": {Driver: "sqlite", DSN: dsn}}, opts, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	db, _ := m.DB("main")
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	return m
}

func count(t *testing.T, m *Manager) int {
	db, _ := m.DB("main")
	res, err := Query(context.Background(), db, `SELECT COUNT(*) FROM t`)
	if err != nil {
		t.Fatal(err)
	}
	return int(res.Rows[0][0].(int64))
}

func insert(m *Manager, req, txID string, id int) error {
	return m.WithTx(req, "main", txID, func(q Querier) error {
		_, err := Exec(context.Background(), q, `INSERT INTO t (id, name) VALUES (?, ?)`, float64(id), "x")
		return err
	})
}

func TestTxCommitOnlyByOwner(t *testing.T) {
	m := openTest(t, Options{TxTimeout: time.Minute})
	id, err := m.BeginTx("orders", "r-1", "main", sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := insert(m, "r-1", id, 1); err != nil {
		t.Fatal(err)
	}
	if err := insert(m, "r-2", id, 2); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("fremde request_id: %v", err)
	}
	if err := m.CommitTx("stock", "r-1", id); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Commit durch Teilnehmer: %v", err)
	}
	if err := m.CommitTx("orders", "r-1", id); err != nil {
		t.Fatal(err)
	}
	if n := count(t, m); n != 1 {
		t.Fatalf("Zeilen: %d", n)
	}
}

func TestTxRollbackByParticipantAborts(t *testing.T) {
	m := openTest(t, Options{TxTimeout: time.Minute})
	id, _ := m.BeginTx("orders", "r-1", "main", sql.TxOptions{})
	insert(m, "r-1", id, 1)
	if err := m.RollbackTx("r-1", id); err != nil {
		t.Fatal(err)
	}
	if err := insert(m, "r-1", id, 2); !errors.Is(err, sdk.ErrTxAborted) {
		t.Fatalf("Zugriff nach Rollback: %v", err)
	}
	if err := m.CommitTx("orders", "r-1", id); !errors.Is(err, sdk.ErrTxAborted) {
		t.Fatalf("Commit nach Rollback: %v", err)
	}
	if n := count(t, m); n != 0 {
		t.Fatalf("Zeilen: %d", n)
	}
}

func TestTxEndRequestTimeoutAndLimit(t *testing.T) {
	m := openTest(t, Options{TxTimeout: 100 * time.Millisecond, MaxTxPerOwner: 1})

	id, _ := m.BeginTx("orders", "r-1", "main", sql.TxOptions{})
	if _, err := m.BeginTx("orders", "r-1", "main", sql.TxOptions{}); !errors.Is(err, sdk.ErrResourceExhausted) {
		t.Fatalf("Limit: %v", err)
	}
	insert(m, "r-1", id, 1)
	m.EndRequest("r-1") // vergessenes Commit → Rollback
	if n := count(t, m); n != 0 {
		t.Fatalf("Zeilen nach EndRequest: %d", n)
	}

	id2, err := m.BeginTx("orders", "r-2", "main", sql.TxOptions{}) // Limit wieder frei
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := insert(m, "r-2", id2, 1); !errors.Is(err, sdk.ErrTxAborted) {
		t.Fatalf("nach Zeitüberschreitung: %v", err)
	}
}
