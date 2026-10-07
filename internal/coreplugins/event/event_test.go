package event

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/internal/dispatcher"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
)

// fakeHost nimmt Zustellungen entgegen; fail liefert je Callback Fehler für die
// ersten Versuche.
type fakeHost struct {
	sdk.Host
	mu    sync.Mutex
	got   []sdk.Request
	ctxs  []sdk.CallContext
	fail  map[string][]error
	delay time.Duration
}

func (h *fakeHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *fakeHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	time.Sleep(h.delay)
	h.mu.Lock()
	defer h.mu.Unlock()
	if errs := h.fail[req.Object]; len(errs) > 0 {
		h.fail[req.Object] = errs[1:]
		return sdk.Response{}, errs[0]
	}
	h.got = append(h.got, req)
	h.ctxs = append(h.ctxs, sdk.CallFromContext(ctx))
	return sdk.Response{}, nil
}

func (h *fakeHost) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.got)
}

func routes() []dispatcher.Entry {
	return []dispatcher.Entry{{Object: "RentContract", Action: "onEvent"}, {Object: "Report", Action: "onEvent"}, {Object: "Other", Action: "list"}}
}

func start(t *testing.T, h *fakeHost, s map[string]any) *Plugin {
	t.Helper()
	p := New(routes)
	p.retryDelay = time.Millisecond
	if err := p.Configure(context.Background(), sdk.Config{Host: h, Settings: s}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	return p
}

func call(t *testing.T, p *Plugin, ctx context.Context, action string, payload any) map[string]any {
	t.Helper()
	resp, err := p.Handle(ctx, sdk.Request{Object: events.Object, Action: action, Payload: payload})
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
	m, _ := resp.Payload.(map[string]any)
	return m
}

func wait(t *testing.T, h *fakeHost, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for h.count() < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := h.count(); got != n {
		t.Fatalf("%d Zustellungen, erwartet %d", got, n)
	}
}

func TestRegisterAndPush(t *testing.T) {
	h := &fakeHost{fail: map[string][]error{}}
	p := start(t, h, nil)
	bg := context.Background()

	call(t, p, bg, events.ActionRegister, events.Subscription{Object: "JournalEntry", Action: "post", CompanyCode: "1000", Callback: "RentContract"})
	call(t, p, bg, events.ActionRegister, events.Subscription{Object: "JournalEntry", Callback: "Report"}) // Action und BK: *
	// Doppelt registrieren (Neustart des Plugins): kein zweites Abonnement.
	r := call(t, p, bg, events.ActionRegister, events.Subscription{Object: "JournalEntry", Action: "post", CompanyCode: "1000", Callback: "RentContract"})
	if r["subscriptions"] != 2 {
		t.Fatalf("Abonnements: %v", r)
	}
	// Route (noch) unbekannt: Abonnement gilt, gekennzeichnet als pending.
	if r := call(t, p, bg, events.ActionRegister, events.Subscription{Object: "Tenant", Callback: "Later"}); r["pending"] != true {
		t.Fatalf("pending: %v", r)
	}
	if r := call(t, p, bg, events.ActionRegister, events.Subscription{Object: "Tenant", Callback: "Report"}); r["pending"] != false {
		t.Fatalf("bekannte Route: %v", r)
	}
	for name, s := range map[string]events.Subscription{
		"Object ungültig": {Object: "journal entry", Callback: "Report"},
		"ohne Callback":   {Object: "JournalEntry"},
	} {
		if _, err := p.Handle(bg, sdk.Request{Object: events.Object, Action: events.ActionRegister, Payload: s}); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Push aus einer Benutzeranfrage: Mandant, Benutzer und Request-ID landen im Event.
	ctx := sdk.WithCall(bg, sdk.CallContext{RequestID: "req-1", TenantID: "demo", UserID: "u-7"})
	r = call(t, p, ctx, events.ActionPush, events.Event{Object: "JournalEntry", Action: "post", CompanyCode: "1000",
		EntityID: "e1", Source: "ledger", Data: map[string]any{"document_number": "1000000001"}})
	if r["subscribers"] != 2 || r["event_id"] == "" {
		t.Fatalf("Push: %v", r)
	}
	wait(t, h, 2)
	ev, err := events.Decode(h.got[0].Payload)
	if err != nil {
		t.Fatal(err)
	}
	if h.got[0].Action != events.CallbackAction || ev.EntityID != "e1" || ev.UserID != "u-7" || ev.TenantID != "demo" ||
		ev.RequestID != "req-1" || ev.OccurredAt == "" || ev.Data["document_number"] != "1000000001" {
		t.Fatalf("Zustellung: %+v %+v", h.got[0], ev)
	}
	// Zustellung als Systemanfrage: ohne Benutzer, eigene Wurzelanfrage.
	if c := h.ctxs[0]; c.UserID != "" || c.RequestID != "" || c.TenantID != "demo" || c.Metadata["ingress"] != "event" {
		t.Fatalf("CallContext der Zustellung: %+v", c)
	}

	// Andere Buchungskreise bzw. Actions: nur das Wildcard-Abonnement.
	if r := call(t, p, bg, events.ActionPush, events.Event{Object: "JournalEntry", Action: "reverse", CompanyCode: "2000"}); r["subscribers"] != 1 {
		t.Fatalf("Wildcard: %v", r)
	}
	if r := call(t, p, bg, events.ActionPush, events.Event{Object: "GLAccount", Action: "update"}); r["subscribers"] != 0 {
		t.Fatalf("ohne Abonnement: %v", r)
	}
	wait(t, h, 3)
	if _, err := p.Handle(bg, sdk.Request{Object: events.Object, Action: events.ActionPush, Payload: events.Event{Object: "JournalEntry"}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Push ohne action: %v", err)
	}
	if l := call(t, p, bg, events.ActionList, nil); len(l["subscriptions"].([]events.Subscription)) != 4 {
		t.Fatalf("List: %v", l)
	}
}

func TestRetryAndGiveUp(t *testing.T) {
	h := &fakeHost{fail: map[string][]error{
		"RentContract": {sdk.ErrUnavailable, sdk.ErrUnavailable},         // dritter Versuch klappt
		"Report":       {sdk.ErrInvalidArgument, sdk.ErrInvalidArgument}, // kein Wiederholen
	}}
	p := start(t, h, map[string]any{"max_attempts": 3})
	bg := context.Background()
	call(t, p, bg, events.ActionRegister, events.Subscription{Object: "*", Callback: "RentContract"})
	call(t, p, bg, events.ActionRegister, events.Subscription{Object: "*", Callback: "Report"})
	call(t, p, bg, events.ActionPush, events.Event{Object: "JournalEntry", Action: "post"})
	wait(t, h, 1)
	if h.got[0].Object != "RentContract" {
		t.Fatalf("Wiederholung: %+v", h.got)
	}
	h.mu.Lock()
	left := len(h.fail["Report"])
	h.mu.Unlock()
	if left != 1 {
		t.Fatalf("Fachfehler wiederholt: noch %d Fehler offen", left)
	}
}

func TestQueueFullAndShutdown(t *testing.T) {
	h := &fakeHost{fail: map[string][]error{}, delay: 20 * time.Millisecond}
	p := start(t, h, map[string]any{"workers": 1, "queue_size": 2})
	bg := context.Background()
	call(t, p, bg, events.ActionRegister, events.Subscription{Object: "*", Callback: "Report"})
	queued := 0
	for range 10 {
		queued += call(t, p, bg, events.ActionPush, events.Event{Object: "JournalEntry", Action: "post"})["subscribers"].(int)
	}
	if queued >= 10 || queued < 2 {
		t.Fatalf("Warteschlange: %d angenommen", queued)
	}
	// Shutdown stellt die angenommenen Events noch zu.
	if err := p.Shutdown(bg); err != nil {
		t.Fatal(err)
	}
	if h.count() != queued {
		t.Fatalf("nach Shutdown %d von %d zugestellt", h.count(), queued)
	}
}
