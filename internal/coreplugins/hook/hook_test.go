package hook

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/internal/dispatcher"
	"github.com/camel/coremesh/pkg/sdk"
	hookapi "github.com/camel/coremesh/pkg/sdk/hook"
)

// subscriberHost spielt die Abonnenten (<Callback>.onHook).
type subscriberHost struct {
	sdk.Host
	calls []string
}

func (h *subscriberHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *subscriberHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	in := req.Payload.(hookapi.Request)
	h.calls = append(h.calls, req.Object+"."+in.Action)
	n, _ := in.Data.(map[string]any)["n"].(float64)
	if v, ok := in.Data.(map[string]any)["n"].(int); ok {
		n = float64(v)
	}
	switch req.Object {
	case "Low": // Priorität 100, check
		return sdk.Response{Payload: hookapi.Reply(nil, hookapi.Warning("W-1", "Hinweis"))}, nil
	case "High": // Priorität 200, check
		return sdk.Response{Payload: hookapi.Reply(nil, hookapi.Error("E-1", "Konto fehlt").OnField("account"))}, nil
	case "Plus": // modify: n+1
		return sdk.Response{Payload: hookapi.Reply(map[string]any{"n": n + 1})}, nil
	case "Times": // modify: n*10
		return sdk.Response{Payload: hookapi.Reply(map[string]any{"n": n * 10})}, nil
	case "After": // commit: Fehler wird zur Warnung
		return sdk.Response{Payload: hookapi.Reply(nil, hookapi.Error("E-9", "Folgebeleg fehlgeschlagen"))}, nil
	}
	return sdk.Response{}, sdk.ErrUnavailable
}

func setup(t *testing.T) (*Plugin, *subscriberHost) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "hook.db")},
	}, database.Options{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	schema := dbschema.New(db)
	if err := schema.Configure(ctx, sdk.Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := schema.Handle(ctx, sdk.Request{Object: "DBSchema", Action: "Activate",
		Payload: map[string]any{"module": Name, "version": Version, "schema": schemaHCL}}); err != nil {
		t.Fatalf("Schema: %v", err)
	}
	routes := func() []dispatcher.Entry {
		var out []dispatcher.Entry
		for _, o := range []string{"Low", "High", "Plus", "Times", "After"} {
			out = append(out, dispatcher.Entry{Object: o, Action: hookapi.CallbackAction, Plugin: "demo"})
		}
		return out
	}
	h := &subscriberHost{}
	p := New(db, routes)
	if err := p.Configure(ctx, sdk.Config{Host: h}); err != nil {
		t.Fatal(err)
	}
	return p, h
}

func do(t *testing.T, p *Plugin, object, action string, payload any) (sdk.Response, error) {
	t.Helper()
	return p.Handle(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r", UserID: "u1"}), sdk.Request{Object: object, Action: action, Payload: payload})
}

func must(t *testing.T, p *Plugin, object, action string, payload any) sdk.Response {
	t.Helper()
	resp, err := do(t, p, object, action, payload)
	if err != nil {
		t.Fatalf("%s.%s: %v", object, action, err)
	}
	return resp
}

func call(t *testing.T, p *Plugin, phase string, data any) hookapi.Result {
	t.Helper()
	return must(t, p, hookapi.Object, hookapi.ActionCall, map[string]any{"hook": "ledger.posting", "action": phase, "data": data}).Payload.(hookapi.Result)
}

func TestHookDispatch(t *testing.T) {
	p, h := setup(t)
	must(t, p, hookapi.Object, hookapi.ActionDefine, hookapi.Definition{Name: "ledger.posting", Description: "Buchen",
		Phases: hookapi.AllPhases, Data: "PostRequest", Owner: "ledger"})
	for _, s := range []hookapi.Subscription{
		{Hook: "ledger.posting", Phase: hookapi.PhaseCheck, Callback: "High", Priority: 200},
		{Hook: "ledger.posting", Phase: hookapi.PhaseCheck, Callback: "Low"}, // Standard 100
		{Hook: "ledger.posting", Phase: hookapi.PhaseModify, Callback: "Times", Priority: 200},
		{Hook: "ledger.posting", Phase: hookapi.PhaseModify, Callback: "Plus", Priority: 100},
		{Hook: "ledger.posting", Phase: hookapi.PhaseCommit, Callback: "After"},
	} {
		must(t, p, hookapi.Object, hookapi.ActionSubscribe, s)
	}

	// modify: Daten laufen in Prioritätsreihenfolge durch (1+1)*10.
	if r := call(t, p, hookapi.PhaseModify, map[string]any{"n": 1}); r.Data.(map[string]any)["n"] != float64(20) || r.Subscribers != 2 {
		t.Fatalf("modify: %+v", r)
	}
	// check: alle Abonnenten, Meldungen mit Quelle und Feld.
	r := call(t, p, hookapi.PhaseCheck, map[string]any{"n": 1})
	if !r.HasErrors() || len(r.Messages) != 2 || r.Messages[0].ID != "W-1" || r.Messages[1].Field != "account" ||
		r.Messages[1].Source != "demo/High" || !strings.Contains(r.Err().Error(), "Konto fehlt") {
		t.Fatalf("check: %+v", r)
	}
	// commit: Fehler wird Warnung.
	if r := call(t, p, hookapi.PhaseCommit, map[string]any{}); r.HasErrors() || r.Messages[0].Type != hookapi.TypeWarning {
		t.Fatalf("commit: %+v", r)
	}

	// Sperren: High wird nicht mehr aufgerufen – auch nach erneutem Anmelden nicht.
	subs := must(t, p, "HookSubscription", "list", map[string]any{"query": map[string]any{"hook": "ledger.posting", "phase": "check"}}).Payload.(map[string]any)["items"].([]any)
	var highID string
	for _, it := range subs {
		if m := it.(map[string]any); m["callback"] == "High" {
			highID = m["id"].(string)
			if m["subscriber"] != "demo" || m["status"] != "active" {
				t.Fatalf("Abo: %v", m)
			}
		}
	}
	must(t, p, "HookSubscription", "lock", map[string]any{"id": highID})
	must(t, p, hookapi.Object, hookapi.ActionSubscribe, hookapi.Subscription{Hook: "ledger.posting", Phase: hookapi.PhaseCheck, Callback: "High", Priority: 200})
	h.calls = nil
	if r := call(t, p, hookapi.PhaseCheck, map[string]any{}); r.HasErrors() || len(h.calls) != 1 || h.calls[0] != "Low.check" {
		t.Fatalf("nach Sperre: %+v %v", r, h.calls)
	}
	got := must(t, p, "HookSubscription", "get", map[string]any{"id": highID}).Payload.(map[string]any)
	if got["status"] != "locked" || got["locked_by"] != "u1" || got["_hidden_actions"].([]any)[0] != "lock" {
		t.Fatalf("gesperrt: %v", got)
	}
	must(t, p, "HookSubscription", "unlock", map[string]any{"id": highID})
	if r := call(t, p, hookapi.PhaseCheck, map[string]any{}); !r.HasErrors() {
		t.Fatal("nach Entsperren wieder aktiv")
	}
}

func TestHookFailureAndValidation(t *testing.T) {
	p, _ := setup(t)
	must(t, p, hookapi.Object, hookapi.ActionDefine, hookapi.Definition{Name: "ledger.posting", Phases: []string{"check"}, OnFailure: hookapi.FailSkip, Owner: "ledger"})
	must(t, p, hookapi.Object, hookapi.ActionSubscribe, hookapi.Subscription{Hook: "ledger.posting", Phase: "check", Callback: "Down"})
	// Nicht erreichbar bei skip: Warnung statt Fehler.
	if r := call(t, p, hookapi.PhaseCheck, map[string]any{}); r.HasErrors() || len(r.Messages) != 1 || r.Messages[0].ID != "HOOK-001" {
		t.Fatalf("skip: %+v", r)
	}
	// Phase nicht definiert.
	if _, err := do(t, p, hookapi.Object, hookapi.ActionCall, map[string]any{"hook": "ledger.posting", "action": "commit"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Phase: %v", err)
	}
	// Anderer Besitzer, ungültige Namen.
	if _, err := do(t, p, hookapi.Object, hookapi.ActionDefine, hookapi.Definition{Name: "ledger.posting", Phases: []string{"check"}, Owner: "rent"}); !errors.Is(err, sdk.ErrAlreadyExists) {
		t.Fatalf("Besitzer: %v", err)
	}
	for _, bad := range []any{
		hookapi.Definition{Name: "Posting", Phases: []string{"check"}},
		hookapi.Definition{Name: "ledger.posting", Phases: []string{"after"}},
	} {
		if _, err := do(t, p, hookapi.Object, hookapi.ActionDefine, bad); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%v: %v", bad, err)
		}
	}
	// Abo auf einen (noch) nicht definierten Hook erscheint in der Übersicht.
	must(t, p, hookapi.Object, hookapi.ActionSubscribe, hookapi.Subscription{Hook: "rent.contract", Phase: "check", Callback: "Low"})
	items := must(t, p, "Hook", "list", nil).Payload.(map[string]any)["items"].([]any)
	if len(items) != 2 || items[1].(map[string]any)["defined"] != false {
		t.Fatalf("Übersicht: %v", items)
	}
}
