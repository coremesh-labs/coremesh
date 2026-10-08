package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// readPlugin ist Handler und Reader: Read liefert n Zeilen (Mandant je Zeile)
// oder ruft über nested einen weiteren Strom auf.
type readPlugin struct {
	n      int
	nested func(ctx context.Context, w sdk.RowWriter) (sdk.ReadEnd, error)
}

func (p *readPlugin) Handle(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
	return sdk.Response{Payload: sdk.CallFromContext(ctx)}, nil
}

func (p *readPlugin) Read(ctx context.Context, _ sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	if p.nested != nil {
		return p.nested(ctx, w)
	}
	if err := w.Header(sdk.ReadHeader{Columns: []string{"i", "tenant"}}); err != nil {
		return sdk.ReadEnd{}, err
	}
	tenant := sdk.CallFromContext(ctx).TenantID
	for i := range p.n {
		if err := w.Rows([][]any{{i, tenant}}); err != nil {
			return sdk.ReadEnd{}, err
		}
	}
	return sdk.ReadEnd{Rows: int64(p.n)}, nil
}

func TestReadRoutesOnlyReadActions(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	err := d.Register("items", manifest("items", sdk.Capability{Object: "Item", Actions: []string{"list", "get"}, ReadActions: []string{"list"}}), &readPlugin{n: 3})
	if err != nil {
		t.Fatal(err)
	}
	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-1", TenantID: "t-1"})
	res, end, err := sdk.ReadAll(ctx, d, sdk.Request{Object: "Item", Action: "list"})
	if err != nil || end.Rows != 3 || len(res.Rows) != 3 || res.Rows[2][1] != "t-1" {
		t.Fatalf("Read: %v %+v %v", err, end, res)
	}
	// get ist Route, aber kein Datenstrom; Unbekanntes ebenso Unimplemented.
	for _, a := range []string{"get", "nope"} {
		if _, _, err := sdk.ReadAll(ctx, d, sdk.Request{Object: "Item", Action: a}); !errors.Is(err, sdk.ErrUnimplemented) {
			t.Errorf("%s: Unimplemented erwartet: %v", a, err)
		}
	}
	if e := d.Catalog(); len(e) != 2 || !e[1].Read || e[0].Read {
		t.Errorf("Catalog: %+v", e)
	}
}

func TestReadActionsAreValidated(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	cases := map[string]struct {
		cap sdk.Capability
		h   sdk.Handler
	}{
		"nicht in actions": {sdk.Capability{Object: "A", Actions: []string{"get"}, ReadActions: []string{"list"}}, &readPlugin{}},
		"ohne Reader":      {sdk.Capability{Object: "B", Actions: []string{"list"}, ReadActions: []string{"list"}}, echoCall()},
		"Host-Route":       {sdk.Capability{Object: sdk.ObjectCatalog, Actions: []string{"Register"}, ReadActions: []string{"Register"}}, &readPlugin{}},
	}
	for name, c := range cases {
		if err := d.Register("p", manifest("p", c.cap), c.h); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%s: ErrInvalidArgument erwartet: %v", name, err)
		}
	}
}

func TestReadAuthorizedLikeHandle(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	d.Register("items", manifest("items", sdk.Capability{Object: "Item", Actions: []string{"list"}, ReadActions: []string{"list"}}), &readPlugin{n: 1})
	a := &fakeAuthz{allow: map[string]bool{"anna|Item.list": true}}
	d.SetAuthorizer(a)
	for user, ok := range map[string]bool{"anna": true, "bob": false} {
		ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-" + user, UserID: user})
		_, _, err := sdk.ReadAll(ctx, d, sdk.Request{Object: "Item", Action: "list"})
		if ok != (err == nil) || (!ok && !errors.Is(err, sdk.ErrPermissionDenied)) {
			t.Errorf("%s: %v", user, err)
		}
	}
}

func TestReadNestedTrustedAndDepth(t *testing.T) {
	d := New(2, nil, slog.New(slog.DiscardHandler))
	d.Register("inner", manifest("inner", sdk.Capability{Object: "Inner", Actions: []string{"list"}, ReadActions: []string{"list"}}), &readPlugin{n: 2})
	// Outer reicht den Strom von Inner durch – mit gefälschtem Mandanten.
	d.Register("outer", manifest("outer", sdk.Capability{Object: "Outer", Actions: []string{"list"}, ReadActions: []string{"list"}}),
		&readPlugin{nested: func(ctx context.Context, w sdk.RowWriter) (sdk.ReadEnd, error) {
			forged := sdk.CallFromContext(ctx)
			forged.TenantID = "fremd"
			return d.ReadNested(sdk.WithCall(context.Background(), forged), sdk.Request{Object: "Inner", Action: "list"}, w)
		}})
	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-1", TenantID: "t-42"})
	res, _, err := sdk.ReadAll(ctx, d, sdk.Request{Object: "Outer", Action: "list"})
	if err != nil || len(res.Rows) != 2 || res.Rows[0][1] != "t-42" {
		t.Fatalf("verschachtelt: %v %v", err, res)
	}
	// Ohne laufende Wurzelanfrage kein verschachtelter Strom.
	_, err = d.ReadNested(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "unbekannt"}), sdk.Request{Object: "Inner", Action: "list"}, &sdk.StrictWriter{})
	if !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("unbekannte request_id: %v", err)
	}
	// Zyklus: Loop ruft sich selbst als Strom auf, bis die Tiefe erreicht ist.
	d.Register("loop", manifest("loop", sdk.Capability{Object: "Loop", Actions: []string{"list"}, ReadActions: []string{"list"}}),
		&readPlugin{nested: func(ctx context.Context, w sdk.RowWriter) (sdk.ReadEnd, error) {
			return d.ReadNested(ctx, sdk.Request{Object: "Loop", Action: "list"}, w)
		}})
	_, _, err = sdk.ReadAll(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-2"}), d, sdk.Request{Object: "Loop", Action: "list"})
	if !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("Tiefe: %v", err)
	}
}

// stopWriter bricht nach dem ersten Block ab (Empfänger hat genug).
type stopWriter struct{ rows int }

var errEnough = errors.New("genug")

func (s *stopWriter) Header(sdk.ReadHeader) error { return nil }
func (s *stopWriter) Rows(r [][]any) error        { s.rows += len(r); return errEnough }

func TestReadConsumerStops(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	d.Register("items", manifest("items", sdk.Capability{Object: "Item", Actions: []string{"list"}, ReadActions: []string{"list"}}), &readPlugin{n: 100})
	var w stopWriter
	_, err := d.Read(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-1"}), sdk.Request{Object: "Item", Action: "list"}, &w)
	if !errors.Is(err, errEnough) || w.rows != 1 {
		t.Fatalf("Abbruch: %v, %d Zeilen", err, w.rows)
	}
}
