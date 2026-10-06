package iam

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

type nopHost struct{ sdk.Host }

func (nopHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

const adminPW = "admin-passwort-123"

// setup: Datenbank, Schema über das echte DBSchema-Plugin, iam mit Admin.
func setup(t *testing.T) (*Plugin, string) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, map[string]config.Database{
		"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "iam.db") + "?_pragma=busy_timeout(5000)"},
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
		t.Fatalf("iam-Schema verletzt Isolation/Regeln: %v", err)
	}

	p := New(db)
	p.cost = bcrypt.MinCost
	p.dummy, _ = bcrypt.GenerateFromPassword([]byte("x"), p.cost)
	if err := p.Configure(ctx, sdk.Config{Host: nopHost{}, Settings: map[string]any{"admin_password": adminPW, "tenant": "demo"}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.cancel)
	for i := 0; ; i++ {
		if n, _ := p.countUsers(ctx); n > 0 {
			break
		}
		if i > 100 {
			t.Fatal("Admin wurde nicht angelegt")
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, err := p.Handle(ctx, sdk.Request{Object: "Account", Action: "Authenticate",
		Payload: map[string]any{"username": "Admin", "password": adminPW}})
	if err != nil {
		t.Fatal(err)
	}
	return p, resp.Payload.(map[string]any)["id"].(string)
}

func as(userID string) context.Context {
	return sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "r", UserID: userID})
}

func call(t *testing.T, p *Plugin, ctx context.Context, object, action string, payload any) (map[string]any, error) {
	t.Helper()
	resp, err := p.Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
	m, _ := resp.Payload.(map[string]any)
	return m, err
}

func TestPermissionPatterns(t *testing.T) {
	perms, err := parsePermissions("*.*\nPartner.*\n *.list \n# Kommentar\nGreeting.l*\nContract?.get\nPartner.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 5 {
		t.Fatalf("Doppelte/Kommentare: %v", perms)
	}
	cases := []struct {
		perm, object, action string
		want                 bool
	}{
		{"*.*", "Anything", "goes", true},
		{"Partner.*", "Partner", "delete", true},
		{"Partner.*", "PartnerX", "get", false},
		{"*.list", "Contract", "list", true},
		{"*.list", "Contract", "listAll", false},
		{"Greeting.l*", "Greeting", "list", true},
		{"Greeting.l*", "Greeting", "say", false},
		{"Contract?.get", "Contracts", "get", true},
		{"partner.*", "Partner", "get", false}, // Groß-/Kleinschreibung
	}
	for _, c := range cases {
		ps, _ := parsePermissions(c.perm)
		if got := ps[0].matches(c.object, c.action); got != c.want {
			t.Errorf("%s gegen %s.%s: %v", c.perm, c.object, c.action, got)
		}
	}
	for _, bad := range []string{"Partner", "Partner.", ".list", "Part ner.get", "Partner.[x"} {
		if _, err := parsePermissions(bad); err == nil {
			t.Errorf("%q muss abgelehnt werden", bad)
		}
	}
}

func TestBootstrapAndAuthenticate(t *testing.T) {
	p, adminID := setup(t)
	me, err := call(t, p, as(adminID), "Account", "Me", nil)
	if err != nil {
		t.Fatal(err)
	}
	if me["username"] != "admin" || me["tenant_id"] != "demo" || me["permissions"].([]any)[0] != "*.*" {
		t.Fatalf("Me: %v", me)
	}
	if ok, _ := p.Allowed(context.Background(), adminID, "Whatever", "delete"); !ok {
		t.Fatal("Administrator darf alles")
	}
	for _, pw := range []string{"falsch", ""} {
		if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "admin", "password": pw}); !errors.Is(err, sdk.ErrPermissionDenied) {
			t.Fatalf("falsches Passwort: %v", err)
		}
	}
	if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "niemand", "password": "x"}); err == nil || !strings.Contains(err.Error(), "Benutzername oder Passwort falsch") {
		t.Fatalf("unbekannter Benutzer: %v", err)
	}
	// Zweiter Lauf legt keinen weiteren Admin an.
	if created, _, err := p.ensureAdmin(context.Background()); created || err != nil {
		t.Fatalf("ensureAdmin erneut: %v %v", created, err)
	}
}

func TestRolesDriveAuthorization(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)

	role, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{
		"name": "Leser", "description": "nur lesen", "permissions": "Greeting.list\n*.get"}})
	if err != nil {
		t.Fatal(err)
	}
	user, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{
		"username": "Leser1", "password": "leser-passwort-1", "roles": "Leser", "active": true, "tenant_id": "t-7"}})
	if err != nil {
		t.Fatal(err)
	}
	uid := user["id"].(string)
	if user["username"] != "leser1" || user["roles"] != "Leser" {
		t.Fatalf("User: %v", user)
	}

	allowed := func(object, action string) bool {
		ok, err := p.Allowed(context.Background(), uid, object, action)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !allowed("Greeting", "list") || !allowed("Partner", "get") || allowed("Greeting", "say") || allowed("User", "list") {
		t.Fatal("Berechtigungen der Rolle Leser falsch angewendet")
	}

	// Änderung an der Rolle wirkt sofort (Cache wird geleert).
	if _, err := call(t, p, ctx, "Role", "update", map[string]any{"id": role["id"], "data": map[string]any{
		"name": "Leser", "permissions": "Greeting.*"}}); err != nil {
		t.Fatal(err)
	}
	if !allowed("Greeting", "say") || allowed("Partner", "get") {
		t.Fatal("Rollenänderung nicht sofort wirksam")
	}

	// Deaktivierter Benutzer: keine Rechte, keine Anmeldung.
	if _, err := call(t, p, ctx, "User", "update", map[string]any{"id": uid, "data": map[string]any{
		"username": "leser1", "roles": "Leser", "active": false}}); err != nil {
		t.Fatal(err)
	}
	if allowed("Greeting", "list") {
		t.Fatal("deaktivierter Benutzer hat noch Rechte")
	}
	if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "leser1", "password": "leser-passwort-1"}); err == nil {
		t.Fatal("deaktivierter Benutzer konnte sich anmelden")
	}
	// Bei Bearbeitung ohne Passwort bleibt es unverändert.
	call(t, p, ctx, "User", "update", map[string]any{"id": uid, "data": map[string]any{"username": "leser1", "roles": "Leser", "active": true}})
	if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "leser1", "password": "leser-passwort-1"}); err != nil {
		t.Fatalf("Passwort darf sich nicht ändern: %v", err)
	}
}

func TestLockoutProtection(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	roles, _ := call(t, p, ctx, "Role", "list", nil)
	adminRole := roles["items"].([]any)[0].(map[string]any)

	cases := map[string]func() error{
		"*.* entziehen": func() error {
			_, err := call(t, p, ctx, "Role", "update", map[string]any{"id": adminRole["id"], "data": map[string]any{"name": AdminRole, "permissions": "User.*"}})
			return err
		},
		"Rolle vom letzten Admin nehmen": func() error {
			_, err := call(t, p, ctx, "User", "update", map[string]any{"id": adminID, "data": map[string]any{"username": "admin", "roles": "", "active": true}})
			return err
		},
		"sich selbst deaktivieren": func() error {
			_, err := call(t, p, ctx, "User", "update", map[string]any{"id": adminID, "data": map[string]any{"username": "admin", "roles": AdminRole, "active": false}})
			return err
		},
		"sich selbst inaktivieren (deactivate)": func() error {
			_, err := call(t, p, ctx, "User", "deactivate", map[string]any{"id": adminID})
			return err
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			if err := fn(); !errors.Is(err, sdk.ErrFailedPrecondition) {
				t.Fatalf("muss abgelehnt werden: %v", err)
			}
		})
	}
	if ok, _ := p.Allowed(context.Background(), adminID, "User", "delete"); !ok {
		t.Fatal("Admin hat Rechte verloren")
	}
}

func TestValidation(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	cases := map[string][2]any{
		"Berechtigungsformat":   {"Role", map[string]any{"data": map[string]any{"name": "X1", "permissions": "Partner"}}},
		"Rolle unbekannt":       {"User", map[string]any{"data": map[string]any{"username": "u1", "password": "passwort-123", "roles": "Gibtsnicht"}}},
		"Passwort zu kurz":      {"User", map[string]any{"data": map[string]any{"username": "u2", "password": "kurz"}}},
		"Passwort fehlt":        {"User", map[string]any{"data": map[string]any{"username": "u3"}}},
		"Benutzername doppelt":  {"User", map[string]any{"data": map[string]any{"username": "ADMIN", "password": "passwort-123"}}},
		"Benutzername ungültig": {"User", map[string]any{"data": map[string]any{"username": "a b", "password": "passwort-123"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := call(t, p, ctx, c[0].(string), "create", c[1]); !errors.Is(err, sdk.ErrInvalidArgument) {
				t.Fatalf("ErrInvalidArgument erwartet: %v", err)
			}
		})
	}
	// Nichts davon wurde gespeichert.
	users, _ := call(t, p, ctx, "User", "list", nil)
	if n := len(users["items"].([]any)); n != 1 {
		t.Fatalf("Benutzer: %d", n)
	}
}

func TestChangePassword(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	if _, err := call(t, p, ctx, "Account", "ChangePassword", map[string]any{"current": "falsch", "new": "neues-passwort-1"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("falsches aktuelles Passwort: %v", err)
	}
	if _, err := call(t, p, ctx, "Account", "ChangePassword", map[string]any{"current": adminPW, "new": "neues-passwort-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "admin", "password": "neues-passwort-1"}); err != nil {
		t.Fatalf("Anmeldung mit neuem Passwort: %v", err)
	}
}

// TestLifecycle: Benutzer werden inaktiviert (Status-Flag), Rollen und
// Buchungskreise sind immutable – gelöscht wird nichts.
func TestLifecycle(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	u, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{"username": "weg", "password": "weg-passwort-12", "active": true}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := call(t, p, ctx, "User", "deactivate", map[string]any{"id": u["id"]})
	if err != nil || got["active"] != false {
		t.Fatalf("deactivate: %v %v", got, err)
	}
	if _, err := call(t, p, context.Background(), "Account", "Authenticate", map[string]any{"username": "weg", "password": "weg-passwort-12"}); err == nil {
		t.Fatal("inaktiver Benutzer kann sich anmelden")
	}
	for _, obj := range []string{"User", "Role", "CompanyCode"} {
		if _, err := call(t, p, ctx, obj, "delete", map[string]any{"id": "x"}); !errors.Is(err, sdk.ErrUnimplemented) {
			t.Errorf("%s.delete: %v", obj, err)
		}
	}
	for _, d := range []metamodel.ObjectDefinition{userDef, roleDef, companyCodeDef} {
		if err := d.Validate(); err != nil {
			t.Error(err)
		}
	}
	if userDef.Lifecycle.Kind() != metamodel.LifecycleStatus || roleDef.Lifecycle.Kind() != metamodel.LifecycleImmutable {
		t.Fatal("Lifecycle")
	}
}

// TestProfileLocaleAndHistory: Sprache im eigenen Profil; Benutzerliste nur
// mit aktiven Benutzern, auf Wunsch mit inaktiven.
func TestProfileLocaleAndHistory(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	me, err := call(t, p, ctx, "Account", "UpdateProfile", map[string]any{"locale": "zh"})
	if err != nil || me["locale"] != "zh-CN" {
		t.Fatalf("UpdateProfile: %v %v", me, err)
	}
	if me, _ := call(t, p, ctx, "Account", "Me", nil); me["locale"] != "zh-CN" {
		t.Fatalf("Me: %v", me)
	}
	if _, err := call(t, p, ctx, "Account", "UpdateProfile", map[string]any{"locale": "zh-TW"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("nicht unterstützte Sprache: %v", err)
	}
	if me, _ := call(t, p, ctx, "Account", "UpdateProfile", map[string]any{"locale": ""}); me["locale"] != "" {
		t.Fatalf("automatisch: %v", me)
	}

	u, _ := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{"username": "alt", "password": "alt-passwort-123", "active": true}})
	call(t, p, ctx, "User", "deactivate", map[string]any{"id": u["id"]})
	count := func(payload any) int {
		l, _ := call(t, p, ctx, "User", "list", payload)
		return len(l["items"].([]any))
	}
	if active, all := count(nil), count(map[string]any{"query": map[string]any{"includeHistory": "true"}}); active+1 != all {
		t.Fatalf("aktiv %d, mit Historie %d", active, all)
	}

	resp, _ := p.Handle(context.Background(), sdk.Request{Object: sdk.ObjectCatalog, Action: sdk.ActionDescribe})
	d := resp.Payload.(metamodel.DescribeResponse)
	if d.Objects[0].Fields[0].LabelKey != "admin.User.fields.username" || d.Translations["zh-CN"]["admin.User.fields.username"] != "用户名" {
		t.Fatalf("Übersetzungen: %+v", d.Objects[0].Fields[0])
	}
}
