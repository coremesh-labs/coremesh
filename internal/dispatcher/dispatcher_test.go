package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

type handlerFunc func(context.Context, sdk.Request) (sdk.Response, error)

func (f handlerFunc) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	return f(ctx, req)
}

func manifest(name string, caps ...sdk.Capability) sdk.Manifest {
	return sdk.Manifest{Name: name, Version: "1.0.0", Capabilities: caps}
}

func echoCall() sdk.Handler {
	return handlerFunc(func(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
		return sdk.Response{Payload: sdk.CallFromContext(ctx)}, nil
	})
}

func TestRegisterAndConflicts(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	bp := sdk.Capability{Object: "BusinessPartner", Actions: []string{"get", "list"}}
	if err := d.Register("partner", manifest("partner", bp), echoCall()); err != nil {
		t.Fatal(err)
	}
	err := d.Register("other", manifest("other",
		sdk.Capability{Object: "SalesOrder", Actions: []string{"get"}},
		sdk.Capability{Object: "BusinessPartner", Actions: []string{"get"}}), echoCall())
	if !errors.Is(err, sdk.ErrAlreadyExists) {
		t.Fatalf("Konflikt erwartet: %v", err)
	}
	// Alles oder nichts: SalesOrder.get darf nicht eingetragen sein.
	if len(d.Routes()) != 2 {
		t.Fatalf("Routen: %v", d.Routes())
	}
	for _, m := range []sdk.Manifest{
		manifest("x"), // keine Capabilities
		manifest("y", sdk.Capability{Object: "salesOrder", Actions: []string{"get"}}),
		manifest("z", sdk.Capability{Object: "Order", Actions: []string{"get", "get"}}),
	} {
		if err := d.Register(m.Name, m, echoCall()); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%s: ErrInvalidArgument erwartet: %v", m.Name, err)
		}
	}
	if err := d.Register("wrong", manifest("other-name", bp), echoCall()); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Errorf("Name-Abweichung: %v", err)
	}
}

func TestNestedUsesTrustedCallContext(t *testing.T) {
	var ended []string
	d := New(8, func(id string) { ended = append(ended, id) }, slog.New(slog.DiscardHandler))
	d.Register("inner", manifest("inner", sdk.Capability{Object: "Inner", Actions: []string{"get"}}), echoCall())

	// Outer simuliert ein Plugin, das einen gefälschten Mandanten mitschickt.
	d.Register("outer", manifest("outer", sdk.Capability{Object: "Outer", Actions: []string{"get"}}),
		handlerFunc(func(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
			forged := sdk.CallFromContext(ctx)
			forged.TenantID = "fremder-mandant"
			forged.Metadata = map[string]string{"traceparent": "abc"}
			return d.HandleNested(sdk.WithCall(context.Background(), forged), sdk.Request{Object: "Inner", Action: "get"})
		}))

	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r-1", TenantID: "t-42"})
	resp, err := d.Handle(ctx, sdk.Request{Object: "Outer", Action: "get"})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Payload.(sdk.CallContext)
	if got.TenantID != "t-42" || got.Metadata["traceparent"] != "abc" {
		t.Fatalf("CallContext: %+v", got)
	}
	if len(ended) != 1 || ended[0] != "r-1" {
		t.Fatalf("onRequestEnd: %v", ended)
	}

	// Nach Ende der Wurzelanfrage ist die request_id ungültig.
	_, err = d.HandleNested(ctx, sdk.Request{Object: "Inner", Action: "get"})
	if !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("ErrPermissionDenied erwartet: %v", err)
	}
	if _, err := d.Handle(ctx, sdk.Request{Object: "Gibt", Action: "esNicht"}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("ErrUnimplemented erwartet: %v", err)
	}
}

func TestCallDepthStopsCycles(t *testing.T) {
	d := New(3, nil, slog.New(slog.DiscardHandler))
	var calls int
	d.Register("loop", manifest("loop", sdk.Capability{Object: "Loop", Actions: []string{"run"}}),
		handlerFunc(func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
			calls++
			return d.HandleNested(ctx, req) // ruft sich selbst auf
		}))
	_, err := d.Handle(context.Background(), sdk.Request{Object: "Loop", Action: "run"})
	if !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("ErrFailedPrecondition erwartet: %v", err)
	}
	if calls != 4 { // Wurzel + 3 verschachtelte
		t.Fatalf("Aufrufe: %d", calls)
	}
}

func TestUnregisterDrains(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	started, release := make(chan struct{}), make(chan struct{})
	d.Register("slow", manifest("slow", sdk.Capability{Object: "Slow", Actions: []string{"run"}}),
		handlerFunc(func(context.Context, sdk.Request) (sdk.Response, error) {
			close(started)
			<-release
			return sdk.Response{}, nil
		}))
	go d.Handle(context.Background(), sdk.Request{Object: "Slow", Action: "run"})
	<-started

	drain := d.Unregister("slow")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := drain(ctx); err == nil {
		t.Fatal("Drain darf bei laufendem Aufruf nicht sofort fertig sein")
	}
	close(release)
	if err := drain(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleCapabilityIsNotRouted(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	initCap := sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}}
	a := manifest("a", sdk.Capability{Object: "A", Actions: []string{"get"}}, initCap)
	b := manifest("b", sdk.Capability{Object: "B", Actions: []string{"get"}}, initCap)

	// Mehrere Module dürfen DBSchema.Init melden – kein Konflikt.
	if err := d.Register("a", a, echoCall()); err != nil {
		t.Fatal(err)
	}
	if err := d.Register("b", b, echoCall()); err != nil {
		t.Fatal(err)
	}
	if !HasLifecycle(a, sdk.ObjectDBSchema, sdk.ActionInit) || HasLifecycle(a, "A", "get") {
		t.Fatal("HasLifecycle")
	}
	// Von außen nicht erreichbar.
	_, err := d.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: sdk.ActionInit})
	if !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("DBSchema.Init darf nicht geroutet werden: %v", err)
	}
}

func TestHostOnlyRoutes(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	d.Register("dbschema", manifest("dbschema", sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{"Activate", "Status"}}), echoCall())
	activate := sdk.Request{Object: sdk.ObjectDBSchema, Action: "Activate"}

	if _, err := d.Handle(context.Background(), activate); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("von außen: %v", err)
	}
	ctx, end, _ := d.Begin(context.Background())
	defer end()
	if _, err := d.HandleNested(ctx, activate); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("aus Plugin: %v", err)
	}
	if _, err := d.Call(ctx, activate); err != nil {
		t.Fatalf("vom Host: %v", err)
	}
	if _, err := d.Handle(context.Background(), sdk.Request{Object: sdk.ObjectDBSchema, Action: "Status"}); err != nil {
		t.Fatalf("Status bleibt öffentlich: %v", err)
	}
}

type fakeAuthz struct {
	allow map[string]bool // "user|Object.action"
	err   error
	calls int
}

func (a *fakeAuthz) Allowed(_ context.Context, userID, object, action string) (bool, error) {
	a.calls++
	return a.allow[userID+"|"+object+"."+action], a.err
}

func TestAuthorization(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	d.Register("orders", manifest("orders", sdk.Capability{Object: "Order", Actions: []string{"create"}}),
		handlerFunc(func(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
			// Modul-interner Aufruf: braucht keine eigene Berechtigung des Benutzers.
			return d.HandleNested(ctx, sdk.Request{Object: "Stock", Action: "reserve"})
		}))
	d.Register("stock", manifest("stock", sdk.Capability{Object: "Stock", Actions: []string{"reserve"}}), echoCall())
	d.Register("catalog", manifest("catalog", sdk.Capability{Object: sdk.ObjectCatalog, Actions: []string{"ListObjects"}}), echoCall())
	d.Register("iam", manifest("iam", sdk.Capability{Object: "Account", Actions: []string{"Authenticate"}}), echoCall())

	a := &fakeAuthz{allow: map[string]bool{"u1|Order.create": true}}
	d.SetAuthorizer(a)
	as := func(user string) context.Context {
		return sdk.WithCall(context.Background(), sdk.CallContext{UserID: user})
	}

	if _, err := d.Handle(as("u1"), sdk.Request{Object: "Order", Action: "create"}); err != nil {
		t.Fatalf("erlaubt, inkl. verschachteltem Stock.reserve: %v", err)
	}
	if _, err := d.Handle(as("u2"), sdk.Request{Object: "Order", Action: "create"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("ohne Recht: %v", err)
	}
	if _, err := d.Handle(as("u2"), sdk.Request{Object: "Stock", Action: "reserve"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("direkter Aufruf ohne Recht: %v", err)
	}
	if _, err := d.Handle(as("u2"), sdk.Request{Object: sdk.ObjectCatalog, Action: "ListObjects"}); err != nil {
		t.Fatalf("Grundrecht Katalog: %v", err)
	}
	if _, err := d.Handle(context.Background(), sdk.Request{Object: "Order", Action: "create"}); err != nil {
		t.Fatalf("System-Anfrage ohne Benutzer: %v", err)
	}
	a.err = errors.New("DB weg")
	if _, err := d.Handle(as("u1"), sdk.Request{Object: "Order", Action: "create"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Fehler im Authorizer muss ablehnen: %v", err)
	}

	// Account.Authenticate nur als Wurzelanfrage, nie verschachtelt.
	ctx, end, _ := d.Begin(context.Background())
	defer end()
	if _, err := d.HandleNested(ctx, sdk.Request{Object: "Account", Action: "Authenticate"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Authenticate verschachtelt: %v", err)
	}
}

// TestObjectBelongsToOnePlugin: Ein Object gehört genau einem Plugin, auch bei
// unterschiedlichen Actions; Lebenszyklus-Capabilities teilen sich alle.
func TestObjectBelongsToOnePlugin(t *testing.T) {
	d := New(8, nil, slog.New(slog.DiscardHandler))
	life := sdk.Capability{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}}
	if err := d.Register("iam", manifest("iam", sdk.Capability{Object: "Account", Actions: []string{"Check", "Authenticate"}}, life), echoCall()); err != nil {
		t.Fatal(err)
	}
	err := d.Register("ledger", manifest("ledger", sdk.Capability{Object: "Account", Actions: []string{"create", "list"}}, life), echoCall())
	if !errors.Is(err, sdk.ErrAlreadyExists) || !strings.Contains(err.Error(), "Object Account gehört bereits Plugin iam") {
		t.Fatalf("Kollision erwartet: %v", err)
	}
	if err := d.Register("ledger", manifest("ledger", sdk.Capability{Object: "GLAccount", Actions: []string{"create"}}, life), echoCall()); err != nil {
		t.Fatalf("eigenes Object: %v", err)
	}
	// Nach dem Abmelden (Neustart) darf dasselbe Plugin sein Object wieder anmelden.
	_ = d.Unregister("ledger")
	if err := d.Register("ledger", manifest("ledger", sdk.Capability{Object: "GLAccount", Actions: []string{"create", "list"}}), echoCall()); err != nil {
		t.Fatalf("Neustart: %v", err)
	}
}
