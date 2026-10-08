package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

type txState int

const (
	txOpen txState = iota
	txAborted
	txCommitted
)

// Tx ist eine Transaktion, die ein Plugin über den HostService geöffnet hat.
// Sie ist an Datenbank, request_id und Eröffner (Plugin) gebunden.
type Tx struct {
	ID        string
	Database  string
	RequestID string
	Owner     string

	mu    sync.Mutex // serialisiert Anweisungen und Abschluss
	tx    *sql.Tx
	state txState
	timer *time.Timer
}

// BeginTx öffnet eine Transaktion für owner innerhalb der Anfrage requestID.
// Sie läuft unabhängig vom ctx des BeginTx-Aufrufs und wird spätestens nach
// Options.TxTimeout automatisch zurückgerollt.
func (m *Manager) BeginTx(owner, requestID, database string, opts sql.TxOptions) (string, error) {
	db, err := m.PoolFor(database, owner)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	if m.opts.MaxTxPerOwner > 0 && m.perOwner[owner] >= m.opts.MaxTxPerOwner {
		m.mu.Unlock()
		return "", fmt.Errorf("%w: max. %d offene Transaktionen für Plugin %s", sdk.ErrResourceExhausted, m.opts.MaxTxPerOwner, owner)
	}
	m.perOwner[owner]++
	m.mu.Unlock()

	sqlTx, err := db.BeginTx(context.Background(), &opts)
	if err != nil {
		m.release(owner)
		return "", err
	}
	t := &Tx{ID: newTxID(), Database: database, RequestID: requestID, Owner: owner, tx: sqlTx}
	timeout := m.opts.TxTimeout
	if opts.ReadOnly && m.opts.ReadTxTimeout > 0 {
		timeout = m.opts.ReadTxTimeout
	}
	t.timer = time.AfterFunc(timeout, func() {
		_ = m.finish(t, false, "Zeitüberschreitung")
	})

	m.mu.Lock()
	m.txs[t.ID] = t
	m.mu.Unlock()
	m.log.Debug("Transaktion geöffnet", "tx", short(t.ID), "db", database, "plugin", owner, "request_id", requestID)
	return t.ID, nil
}

// WithTx führt fn mit der Transaktion txID aus, nachdem geprüft wurde, dass
// sie zur Anfrage und Datenbank gehört und noch offen ist.
func (m *Manager) WithTx(requestID, database, txID string, fn func(q Querier) error) error {
	t, err := m.get(requestID, txID)
	if err != nil {
		return err
	}
	if t.Database != database {
		return fmt.Errorf("%w: Transaktion gehört zu Datenbank %q", sdk.ErrPermissionDenied, t.Database)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state != txOpen {
		return fmt.Errorf("%w: %s", sdk.ErrTxAborted, short(t.ID))
	}
	return fn(t.tx)
}

// CommitTx schließt die Transaktion ab. Nur der Eröffner darf committen.
func (m *Manager) CommitTx(owner, requestID, txID string) error {
	t, err := m.get(requestID, txID)
	if err != nil {
		return err
	}
	if t.Owner != owner {
		return fmt.Errorf("%w: nur der Eröffner (%s) darf committen", sdk.ErrPermissionDenied, t.Owner)
	}
	return m.finish(t, true, "commit")
}

// RollbackTx rollt die Transaktion zurück. Das darf jeder Teilnehmer der Anfrage.
func (m *Manager) RollbackTx(requestID, txID string) error {
	t, err := m.get(requestID, txID)
	if err != nil {
		return err
	}
	return m.finish(t, false, "rollback")
}

// EndRequest rollt alle noch offenen Transaktionen der Anfrage zurück und
// vergisst sie. Der Dispatcher ruft das am Ende jeder Wurzelanfrage auf.
func (m *Manager) EndRequest(requestID string) {
	m.mu.Lock()
	var ended []*Tx
	for id, t := range m.txs {
		if t.RequestID == requestID {
			ended = append(ended, t)
			delete(m.txs, id)
		}
	}
	m.mu.Unlock()
	for _, t := range ended {
		if err := m.finish(t, false, "Anfrage beendet"); err == nil {
			m.log.Warn("offene Transaktion am Anfrageende zurückgerollt", "tx", short(t.ID), "plugin", t.Owner, "request_id", requestID)
		}
	}
}

// AbortOwner rollt alle offenen Transaktionen eines Plugins zurück, z. B.
// wenn sein Prozess abgestürzt ist.
func (m *Manager) AbortOwner(owner string) {
	m.mu.Lock()
	var owned []*Tx
	for _, t := range m.txs {
		if t.Owner == owner {
			owned = append(owned, t)
		}
	}
	m.mu.Unlock()
	for _, t := range owned {
		_ = m.finish(t, false, "Plugin beendet")
	}
}

func (m *Manager) get(requestID, txID string) (*Tx, error) {
	m.mu.Lock()
	t, ok := m.txs[txID]
	m.mu.Unlock()
	if !ok || t.RequestID != requestID {
		return nil, fmt.Errorf("%w: unbekannte Transaktion", sdk.ErrPermissionDenied)
	}
	return t, nil
}

// finish beendet eine offene Transaktion genau einmal.
func (m *Manager) finish(t *Tx, commit bool, reason string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch t.state {
	case txAborted:
		return fmt.Errorf("%w: %s", sdk.ErrTxAborted, short(t.ID))
	case txCommitted:
		return fmt.Errorf("%w: Transaktion %s ist bereits abgeschlossen", sdk.ErrFailedPrecondition, short(t.ID))
	}
	t.timer.Stop()

	var err error
	if commit {
		err = t.tx.Commit()
		t.state = txCommitted
		if err != nil {
			t.state = txAborted
		}
	} else {
		err = t.tx.Rollback()
		t.state = txAborted
	}
	m.release(t.Owner)
	m.log.Debug("Transaktion beendet", "tx", short(t.ID), "grund", reason, "err", err)
	return err
}

func (m *Manager) release(owner string) {
	m.mu.Lock()
	if m.perOwner[owner]--; m.perOwner[owner] <= 0 {
		delete(m.perOwner, owner)
	}
	m.mu.Unlock()
}

func newTxID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
