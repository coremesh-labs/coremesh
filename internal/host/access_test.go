package host

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/coreplugins/iam"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
)

type handlerFunc func(context.Context, sdk.Request) (sdk.Response, error)

func (f handlerFunc) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) { return f(ctx, req) }

// TestCheckAccessUsesTrustedUser: Ein Modul prüft per sdk.CheckAccess den
// Buchungskreis. Selbst wenn es im Kontext einen anderen Benutzer einträgt,
// gilt der Benutzer der ursprünglichen Anfrage (Dispatcher).
func TestCheckAccessUsesTrustedUser(t *testing.T) {
	ctx := context.Background()
	write := map[string]config.Grant{"main": {Access: "write"}}
	cfg := &config.Config{
		Host:      config.Host{MaxCallDepth: 8},
		Databases: map[string]config.Database{"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "a.db") + "?_pragma=busy_timeout(5000)"}},
		Plugins: map[string]config.Plugin{
			"dbschema": {Kind: config.KindInternal},
			"iam": {Kind: config.KindInternal, Databases: write,
				Settings: map[string]any{"admin_password": "admin-passwort-1"}},
		},
	}
	log := slog.New(slog.DiscardHandler)
	db, err := database.Open(ctx, cfg.Databases, database.Options{}, log)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, err := New(cfg, db, log)
	if err != nil {
		t.Fatal(err)
	}
	var idm *iam.Plugin
	if err := h.StartInternal(ctx, map[string]InternalFactory{
		dbschema.Name: func(d InternalDeps) sdk.Plugin { return dbschema.New(d.DB) },
		iam.Name:      func(d InternalDeps) sdk.Plugin { idm = iam.New(d.DB); return idm },
	}); err != nil {
		t.Fatal(err)
	}

	// Admin anmelden (Bootstrap läuft im Hintergrund).
	var adminID string
	for i := 0; adminID == "" && i < 200; i++ {
		resp, err := h.disp.Handle(ctx, sdk.Request{Object: "Account", Action: "Authenticate",
			Payload: map[string]any{"username": "admin", "password": "admin-passwort-1"}})
		if err == nil {
			adminID = resp.Payload.(map[string]any)["id"].(string)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if adminID == "" {
		t.Fatal("kein Admin")
	}
	asUser := func(id string) context.Context {
		return sdk.WithCall(ctx, sdk.CallContext{UserID: id})
	}
	admin := asUser(adminID)
	mustCall := func(c context.Context, object, action string, payload any) map[string]any {
		t.Helper()
		resp, err := h.disp.Handle(c, sdk.Request{Object: object, Action: action, Payload: payload})
		if err != nil {
			t.Fatalf("%s.%s: %v", object, action, err)
		}
		m, _ := resp.Payload.(map[string]any)
		return m
	}
	for _, cc := range []string{"1000", "3000"} {
		mustCall(admin, "CompanyCode", "create", map[string]any{"data": map[string]any{"code": cc}})
	}
	mustCall(admin, "Role", "create", map[string]any{"data": map[string]any{"name": "Nord", "permissions": "Partner.update@1000"}})
	nordID := mustCall(admin, "User", "create", map[string]any{"data": map[string]any{
		"username": "nord", "password": "nord-passwort-1", "roles": "Nord"}})["id"].(string)

	// Fachmodul Partner: prüft den Buchungskreis des Datensatzes – und
	// versucht dabei, sich als Admin auszugeben.
	svc := h.services("partner", config.Plugin{})
	if err := h.disp.Register("partner", sdk.Manifest{Name: "partner", Capabilities: []sdk.Capability{
		{Object: "Partner", Actions: []string{"update"}},
	}}, handlerFunc(func(ctx context.Context, req sdk.Request) (sdk.Response, error) {
		var in struct {
			CompanyCode string `json:"company_code"`
		}
		sdk.Decode(req.Payload, &in)
		forged := sdk.CallFromContext(ctx)
		forged.UserID = adminID
		ctx = sdk.WithHost(sdk.WithCall(ctx, forged), svc)

		ok, err := sdk.CheckAccess(ctx, "Partner", "update", in.CompanyCode)
		if err != nil {
			return sdk.Response{}, err
		}
		g, err := sdk.GrantedCompanyCodes(ctx, "Partner", "update")
		if err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: map[string]any{"allowed": ok, "granted": g.CompanyCodes, "all": g.All}}, nil
	})); err != nil {
		t.Fatal(err)
	}

	nord := asUser(nordID)
	if got := mustCall(nord, "Partner", "update", map[string]any{"company_code": "1000"}); got["allowed"] != true {
		t.Fatalf("1000: %v", got)
	}
	got := mustCall(nord, "Partner", "update", map[string]any{"company_code": "3000"})
	if got["allowed"] != false || got["all"] != false {
		t.Fatalf("3000 darf trotz gefälschtem Admin-Benutzer nicht erlaubt sein: %v", got)
	}
	if g := got["granted"].([]string); len(g) != 1 || g[0] != "1000" {
		t.Fatalf("Granted: %v", got["granted"])
	}
}
