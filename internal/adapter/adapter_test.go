package adapter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// testPlugin ruft bei Handle den Host per Query zurück, um den Rückkanal zu prüfen.
type testPlugin struct {
	host     sdk.Host
	settings map[string]any
}

func (p *testPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Capabilities: []sdk.Capability{{Object: "BusinessPartner", Actions: []string{"get", "list"}}},
	}, nil
}

func (p *testPlugin) Configure(_ context.Context, cfg sdk.Config) error {
	p.host, p.settings = cfg.Host, cfg.Settings
	return nil
}

func (p *testPlugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if in.ID == "missing" {
		return sdk.Response{}, fmt.Errorf("%w: partner %s", sdk.ErrNotFound, in.ID)
	}
	res, err := p.host.Query(ctx, "main", "SELECT name FROM partner WHERE id = ?", in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	call := sdk.CallFromContext(ctx)
	return sdk.Response{
		Payload: struct {
			Name   string `json:"name"`
			Tenant string `json:"tenant"`
		}{res.Rows[0][0].(string), call.TenantID},
		Metadata: map[string]string{"source": "test"},
	}, nil
}

// fakeHost merkt sich den CallContext des letzten Aufrufs und routet Handle
// wie ein minimaler Dispatcher über "Object.action".
type fakeHost struct {
	mu     sync.Mutex
	last   sdk.CallContext
	args   []any
	routes map[string]sdk.Handler
}

func (h *fakeHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	target, ok := h.routes[req.Object+"."+req.Action]
	if !ok {
		return sdk.Response{}, fmt.Errorf("%w: kein Plugin für %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
	}
	return target.Handle(ctx, req)
}

func (h *fakeHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *fakeHost) Query(ctx context.Context, database, sql string, args ...any) (*sdk.QueryResult, error) {
	h.mu.Lock()
	h.last, h.args = sdk.CallFromContext(ctx), args
	h.mu.Unlock()
	if database != "main" {
		return nil, fmt.Errorf("%w: database %s", sdk.ErrPermissionDenied, database)
	}
	return &sdk.QueryResult{Columns: []string{"name"}, Rows: [][]any{{"ACME AG"}}}, nil
}

func (h *fakeHost) Exec(context.Context, string, string, ...any) (sdk.ExecResult, error) {
	return sdk.ExecResult{}, nil
}

func dispense(t *testing.T, impl sdk.Plugin) sdk.Plugin {
	t.Helper()
	client, server := goplugin.TestPluginGRPCConn(t, false, goplugin.PluginSet{PluginName: &GRPCPlugin{Impl: impl}})
	t.Cleanup(func() { client.Close(); server.Stop() })
	raw, err := client.Dispense(PluginName)
	if err != nil {
		t.Fatal(err)
	}
	return raw.(sdk.Plugin)
}

func TestRoundTrip(t *testing.T) {
	impl := &testPlugin{}
	p := dispense(t, impl)
	host := &fakeHost{}
	ctx := context.Background()

	m, err := p.Manifest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Capabilities[0]; got.Object != "BusinessPartner" || len(got.Actions) != 2 {
		t.Fatalf("Manifest: %+v", m)
	}

	if err := p.Configure(ctx, sdk.Config{Settings: map[string]any{"limit": 10}, Host: host}); err != nil {
		t.Fatal(err)
	}
	if impl.settings["limit"] != float64(10) {
		t.Fatalf("Settings: %v", impl.settings)
	}

	call := sdk.CallContext{RequestID: "r-1", TenantID: "t-42", UserID: "u-7", Metadata: map[string]string{"locale": "de-CH"}}
	resp, err := p.Handle(sdk.WithCall(ctx, call), sdk.Request{
		Object:  "BusinessPartner",
		Action:  "get",
		Payload: map[string]string{"id": "4711"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := resp.Payload.(map[string]any)
	if got["name"] != "ACME AG" || got["tenant"] != "t-42" {
		t.Fatalf("Payload: %v", got)
	}
	if resp.Metadata["source"] != "test" {
		t.Fatalf("Metadata: %v", resp.Metadata)
	}
	// Der Rückkanal zum Host muss den CallContext des Handle-Aufrufs tragen.
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.last.RequestID != "r-1" || host.last.TenantID != "t-42" || host.last.Metadata["locale"] != "de-CH" {
		t.Fatalf("Host-Call-Context: %+v", host.last)
	}
	if len(host.args) != 1 || host.args[0] != "4711" {
		t.Fatalf("Query-Args: %v", host.args)
	}
}

func TestErrorMapping(t *testing.T) {
	p := dispense(t, &testPlugin{})
	ctx := context.Background()
	if err := p.Configure(ctx, sdk.Config{Host: &fakeHost{}}); err != nil {
		t.Fatal(err)
	}

	_, err := p.Handle(ctx, sdk.Request{Object: "BusinessPartner", Action: "get", Payload: map[string]any{"id": "missing"}})
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Fatalf("erwartet ErrNotFound, bekommen: %v", err)
	}
	if err.Error() != "not found: partner missing" {
		t.Fatalf("Fehlermeldung: %q", err.Error())
	}

	_, err = p.Handle(ctx, sdk.Request{Object: "BusinessPartner", Action: "get", Payload: "kein-objekt"})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("erwartet ErrInvalidArgument, bekommen: %v", err)
	}
}

// orderPlugin bedient SalesOrder.get und holt den Geschäftspartner über den
// Host von einem anderen Plugin – ohne zu wissen, welches das ist.
type orderPlugin struct{}

func (p *orderPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{Name: "orders", Capabilities: []sdk.Capability{{Object: "SalesOrder", Actions: []string{"get"}}}}, nil
}

// Configure muss den Host nicht speichern – Handle holt ihn per sdk.HostFrom(ctx).
func (p *orderPlugin) Configure(context.Context, sdk.Config) error { return nil }

func (p *orderPlugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	partner, err := sdk.HostFrom(ctx).Handle(ctx, sdk.Request{
		Object:  "BusinessPartner",
		Action:  "get",
		Payload: map[string]any{"id": "4711"},
	})
	if err != nil {
		return sdk.Response{}, fmt.Errorf("Partner laden: %w", err)
	}
	return sdk.Response{Payload: map[string]any{"order": "SO-1", "partner": partner.Payload}}, nil
}

func TestPluginCallsPluginViaHost(t *testing.T) {
	ctx := context.Background()

	partners := dispense(t, &testPlugin{})
	orders := dispense(t, &orderPlugin{})
	host := &fakeHost{routes: map[string]sdk.Handler{
		"BusinessPartner.get": partners,
		"SalesOrder.get":      orders,
	}}
	for _, p := range []sdk.Plugin{partners, orders} {
		if err := p.Configure(ctx, sdk.Config{Host: host}); err != nil {
			t.Fatal(err)
		}
	}

	// Host -> orders -> Host.Dispatch -> partners -> Host.Query
	call := sdk.CallContext{RequestID: "r-9", TenantID: "t-42"}
	resp, err := host.Handle(sdk.WithCall(ctx, call), sdk.Request{Object: "SalesOrder", Action: "get"})
	if err != nil {
		t.Fatal(err)
	}
	partner := resp.Payload.(map[string]any)["partner"].(map[string]any)
	if partner["name"] != "ACME AG" || partner["tenant"] != "t-42" {
		t.Fatalf("Partner: %v", partner)
	}
	host.mu.Lock()
	if host.last.RequestID != "r-9" || host.last.TenantID != "t-42" {
		t.Fatalf("CallContext ging in der Kette verloren: %+v", host.last)
	}
	host.mu.Unlock()

	// Fehler aus dem Host-Dispatcher kommen beim aufrufenden Plugin als Sentinel an
	// und überleben den Rückweg über zwei Prozessgrenzen.
	delete(host.routes, "BusinessPartner.get")
	_, err = host.Handle(sdk.WithCall(ctx, call), sdk.Request{Object: "SalesOrder", Action: "get"})
	if !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("erwartet ErrUnimplemented, bekommen: %v", err)
	}
}

func TestHostUnavailableBeforeConfigure(t *testing.T) {
	orders := dispense(t, &orderPlugin{})
	_, err := orders.Handle(context.Background(), sdk.Request{Object: "SalesOrder", Action: "get"})
	if !errors.Is(err, sdk.ErrUnavailable) {
		t.Fatalf("erwartet ErrUnavailable, bekommen: %v", err)
	}
}
