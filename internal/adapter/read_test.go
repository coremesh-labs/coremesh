package adapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// streamPlugin liefert Datenströme über gRPC:
//   - Big.list: n Zeilen mit breitem Text (zwingt zu mehreren batches),
//   - Relay.list: reicht Source.list des Hosts durch (HostService.DispatchRead),
//   - Fail.list: zwei Zeilen, dann ErrNotFound.
type streamPlugin struct {
	host sdk.Host
	n    int

	mu      sync.Mutex
	lastErr error // Ergebnis des letzten Read auf Plugin-Seite
}

func (p *streamPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	var caps []sdk.Capability
	for _, o := range []string{"Big", "Relay", "Fail"} {
		caps = append(caps, sdk.Capability{Object: o, Actions: []string{"list"}, ReadActions: []string{"list"}})
	}
	return sdk.Manifest{Name: "stream", Version: "1.0.0", Capabilities: caps}, nil
}

func (p *streamPlugin) Configure(_ context.Context, cfg sdk.Config) error {
	p.host = cfg.Host
	return nil
}

func (p *streamPlugin) Handle(context.Context, sdk.Request) (sdk.Response, error) {
	return sdk.Response{}, nil
}

func (p *streamPlugin) Read(ctx context.Context, req sdk.Request, w sdk.RowWriter) (end sdk.ReadEnd, err error) {
	defer func() { p.mu.Lock(); p.lastErr = err; p.mu.Unlock() }()
	switch req.Object {
	case "Relay":
		// Der Host aus dem Kontext ist derselbe wie aus Configure.
		return sdk.HostFrom(ctx).Read(ctx, sdk.Request{Object: "Source", Action: "list", Payload: req.Payload}, w)
	case "Fail":
		if err := w.Header(sdk.ReadHeader{Columns: []string{"i"}}); err != nil {
			return end, err
		}
		if err := w.Rows([][]any{{1}, {2}}); err != nil {
			return end, err
		}
		return end, fmt.Errorf("%w: Konto 4711", sdk.ErrNotFound)
	}
	if err := w.Header(sdk.ReadHeader{Columns: []string{"i", "text", "tenant"}, Metadata: map[string]string{"object": "Big"}}); err != nil {
		return end, err
	}
	text := strings.Repeat("x", 300)
	tenant := sdk.CallFromContext(ctx).TenantID
	// Blöcke zu 1000 Zeilen, wie ein Anbieter, der seitenweise liest.
	for i := 0; i < p.n; i += 1000 {
		rows := make([][]any, 0, 1000)
		for j := i; j < min(i+1000, p.n); j++ {
			rows = append(rows, []any{j, text, tenant})
		}
		if err := w.Rows(rows); err != nil {
			return end, err
		}
	}
	return sdk.ReadEnd{Cursor: "weiter"}, nil
}

// countWriter zählt Blöcke und prüft die Reihenfolge der Zeilen.
type countWriter struct {
	h       sdk.ReadHeader
	batches int
	rows    [][]any
	stopAt  int // >0: nach so vielen Blöcken abbrechen
}

func (c *countWriter) Header(h sdk.ReadHeader) error { c.h = h; return nil }
func (c *countWriter) Rows(r [][]any) error {
	c.batches++
	c.rows = append(c.rows, r...)
	if c.stopAt > 0 && c.batches >= c.stopAt {
		return errEnough
	}
	return nil
}

var errEnough = errors.New("genug")

func TestReadStreamsInBatches(t *testing.T) {
	impl := &streamPlugin{n: 12000}
	p := dispense(t, impl)
	ctx := context.Background()
	if err := p.Configure(ctx, sdk.Config{Host: &fakeHost{}}); err != nil {
		t.Fatal(err)
	}
	m, _ := p.Manifest(ctx)
	if got := m.Capabilities[0].ReadActions; len(got) != 1 || got[0] != "list" {
		t.Fatalf("ReadActions im Manifest: %v", got)
	}

	var w countWriter
	call := sdk.WithCall(ctx, sdk.CallContext{RequestID: "r-1", TenantID: "t-42"})
	end, err := p.(sdk.Reader).Read(call, sdk.Request{Object: "Big", Action: "list"}, &w)
	if err != nil {
		t.Fatal(err)
	}
	if end.Rows != 12000 || end.Cursor != "weiter" || len(w.rows) != 12000 {
		t.Fatalf("Ende %+v, %d Zeilen", end, len(w.rows))
	}
	if w.h.Metadata["object"] != "Big" || len(w.h.Columns) != 3 {
		t.Fatalf("Header: %+v", w.h)
	}
	// 12000 × ~310 Bytes ≈ 3,7 MB: mehrere Blöcke unter maxBatchBytes.
	if w.batches < 4 {
		t.Fatalf("nur %d Blöcke", w.batches)
	}
	for i, r := range w.rows {
		if r[0] != float64(i) || r[2] != "t-42" {
			t.Fatalf("Zeile %d: %v", i, r[:1])
		}
	}
}

func TestReadThroughHost(t *testing.T) {
	impl := &streamPlugin{}
	p := dispense(t, impl)
	ctx := context.Background()
	source := &streamPlugin{n: 3} // läuft lokal hinter dem fakeHost
	host := &fakeHost{routes: map[string]sdk.Handler{"Source.list": source}}
	if err := p.Configure(ctx, sdk.Config{Host: host}); err != nil {
		t.Fatal(err)
	}
	call := sdk.WithCall(ctx, sdk.CallContext{RequestID: "r-1", TenantID: "t-7"})
	res, end, err := sdk.ReadAll(call, p.(sdk.Reader), sdk.Request{Object: "Relay", Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	// Host -> Plugin -> Host -> Source und zurück, Mandant über die ganze Kette.
	if end.Rows != 3 || len(res.Rows) != 3 || res.Rows[1][2] != "t-7" {
		t.Fatalf("Ende %+v, Zeilen %v", end, res.Rows)
	}
}

func TestReadErrors(t *testing.T) {
	impl := &streamPlugin{n: 50000}
	p := dispense(t, impl)
	ctx := context.Background()
	if err := p.Configure(ctx, sdk.Config{Host: &fakeHost{}}); err != nil {
		t.Fatal(err)
	}
	r := p.(sdk.Reader)

	// Fehler des Anbieters mitten im Strom: Sentinel bleibt erhalten.
	var w countWriter
	if _, err := r.Read(ctx, sdk.Request{Object: "Fail", Action: "list"}, &w); !errors.Is(err, sdk.ErrNotFound) || err.Error() != "not found: Konto 4711" {
		t.Fatalf("Fehler: %v", err)
	}

	// Empfänger bricht ab: Fehler des Empfängers, Anbieter hört auf.
	stop := countWriter{stopAt: 1}
	if _, err := r.Read(ctx, sdk.Request{Object: "Big", Action: "list"}, &stop); !errors.Is(err, errEnough) {
		t.Fatalf("Abbruch: %v", err)
	}
	// Der Anbieter merkt den Abbruch asynchron (Send liefert Canceled).
	deadline := time.Now().Add(5 * time.Second)
	for {
		impl.mu.Lock()
		lastErr := impl.lastErr
		impl.mu.Unlock()
		if lastErr != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Anbieter hat den Abbruch nicht bemerkt")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Plugin ohne Reader: Unimplemented.
	plain := dispense(t, &testPlugin{})
	if err := plain.Configure(ctx, sdk.Config{Host: &fakeHost{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.(sdk.Reader).Read(ctx, sdk.Request{Object: "BusinessPartner", Action: "list"}, &w); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("ohne Reader: %v", err)
	}
}
