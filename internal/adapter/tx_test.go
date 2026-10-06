package adapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/camel/coremesh/internal/txctx"
	"github.com/camel/coremesh/pkg/sdk"
)

// txHost ist ein Fake-Host mit Transaktionsverwaltung. Er protokolliert alle
// Tx-Ereignisse und unter welcher tx_id jedes Exec lief.
type txHost struct {
	fakeHost
	mu     sync.Mutex
	nextID int
	state  map[string]string // tx_id -> open | aborted | committed
	events []string
}

var _ txctx.Manager = (*txHost)(nil)

func newTxHost() *txHost {
	return &txHost{fakeHost: fakeHost{routes: map[string]sdk.Handler{}}, state: map[string]string{}}
}

func (h *txHost) log(format string, args ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, fmt.Sprintf(format, args...))
}

func (h *txHost) Exec(ctx context.Context, database, sql string, _ ...any) (sdk.ExecResult, error) {
	id, _ := txctx.Get(ctx, database)
	h.mu.Lock()
	aborted := id != "" && h.state[id] != "open"
	h.mu.Unlock()
	if aborted {
		return sdk.ExecResult{}, sdk.ErrTxAborted
	}
	h.log("exec %q tx=%s", sql, id)
	return sdk.ExecResult{RowsAffected: 1}, nil
}

func (h *txHost) BeginTx(_ context.Context, database string, opts sql.TxOptions) (string, error) {
	h.mu.Lock()
	h.nextID++
	id := fmt.Sprintf("tx-%d", h.nextID)
	h.state[id] = "open"
	h.mu.Unlock()
	h.log("begin %s %s readonly=%v", database, id, opts.ReadOnly)
	return id, nil
}

func (h *txHost) CommitTx(_ context.Context, id string) error {
	return h.finish(id, "committed", "commit")
}

func (h *txHost) RollbackTx(_ context.Context, id string) error {
	return h.finish(id, "aborted", "rollback")
}

func (h *txHost) finish(id, to, event string) error {
	h.mu.Lock()
	st := h.state[id]
	if st == "open" {
		h.state[id] = to
	}
	h.mu.Unlock()
	if st != "open" {
		return fmt.Errorf("%w: %s ist %s", sdk.ErrTxAborted, id, st)
	}
	h.log("%s %s", event, id)
	return nil
}

// orderTxPlugin bestätigt einen Auftrag in einer Transaktion und reserviert
// über den Host Lagerbestand bei einem anderen Plugin.
type orderTxPlugin struct{}

func (orderTxPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{Name: "orders", Capabilities: []sdk.Capability{{Object: "SalesOrder", Actions: []string{"confirm"}}}}, nil
}
func (orderTxPlugin) Configure(context.Context, sdk.Config) error { return nil }

func (orderTxPlugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	err := sdk.InTx(ctx, "main", nil, func(ctx context.Context) error {
		host := sdk.HostFrom(ctx)
		if _, err := host.Exec(ctx, "main", "UPDATE orders"); err != nil {
			return err
		}
		// Audit soll einen Rollback überleben.
		if _, err := host.Exec(sdk.WithoutTx(ctx), "main", "INSERT audit"); err != nil {
			return err
		}
		_, err := host.Handle(ctx, sdk.Request{Object: "Stock", Action: "reserve", Payload: req.Payload})
		return err
	})
	return sdk.Response{}, err
}

// stockTxPlugin nutzt ebenfalls InTx – und nimmt damit an der Transaktion
// des Aufrufers teil, statt eine eigene zu öffnen.
type stockTxPlugin struct{}

func (stockTxPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{Name: "stock", Capabilities: []sdk.Capability{{Object: "Stock", Actions: []string{"reserve"}}}}, nil
}
func (stockTxPlugin) Configure(context.Context, sdk.Config) error { return nil }

func (stockTxPlugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	err := sdk.InTx(ctx, "main", nil, func(ctx context.Context) error {
		if _, err := sdk.HostFrom(ctx).Exec(ctx, "main", "UPDATE stock"); err != nil {
			return err
		}
		if req.Payload == "out-of-stock" {
			return fmt.Errorf("%w: kein Bestand", sdk.ErrFailedPrecondition)
		}
		return nil
	})
	return sdk.Response{}, err
}

func setupTx(t *testing.T) (*txHost, sdk.Plugin) {
	t.Helper()
	host := newTxHost()
	orders := dispense(t, orderTxPlugin{})
	stock := dispense(t, stockTxPlugin{})
	host.routes["Stock.reserve"] = stock
	for _, p := range []sdk.Plugin{orders, stock} {
		if err := p.Configure(context.Background(), sdk.Config{Host: host}); err != nil {
			t.Fatal(err)
		}
	}
	return host, orders
}

func TestTxSpansPluginsViaDispatch(t *testing.T) {
	host, orders := setupTx(t)
	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-1"})

	if _, err := orders.Handle(ctx, sdk.Request{Object: "SalesOrder", Action: "confirm", Payload: "ok"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"begin main tx-1 readonly=false",
		`exec "UPDATE orders" tx=tx-1`,
		`exec "INSERT audit" tx=`,     // WithoutTx
		`exec "UPDATE stock" tx=tx-1`, // anderes Plugin, gleiche Transaktion
		"commit tx-1",
	}
	if !slices.Equal(host.events, want) {
		t.Fatalf("Ereignisse:\n got %q\nwant %q", host.events, want)
	}
}

func TestTxRollbackByParticipant(t *testing.T) {
	host, orders := setupTx(t)
	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-2"})

	_, err := orders.Handle(ctx, sdk.Request{Object: "SalesOrder", Action: "confirm", Payload: "out-of-stock"})
	if !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("erwartet ErrFailedPrecondition, bekommen: %v", err)
	}
	want := []string{
		"begin main tx-1 readonly=false",
		`exec "UPDATE orders" tx=tx-1`,
		`exec "INSERT audit" tx=`,
		`exec "UPDATE stock" tx=tx-1`,
		"rollback tx-1", // durch das Stock-Plugin; das Rollback des Eröffners ist dann ein No-op
	}
	if !slices.Equal(host.events, want) {
		t.Fatalf("Ereignisse:\n got %q\nwant %q", host.events, want)
	}
}
