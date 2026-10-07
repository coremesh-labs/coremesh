package iam

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

// catalogHost spielt den Catalog mit einem Object FiscalPeriod, das
// Berechtigungsfelder und die Berechtigungs-Action post deklariert.
type catalogHost struct{ nopHost }

var periodDef = metamodel.ObjectDefinition{
	Name: "FiscalPeriod", Title: "Buchungsperioden",
	Fields: []metamodel.FieldDefinition{
		{Key: "ledger", Label: "Ledger", Type: metamodel.TypeText},
		{Key: "posting_period", Label: "Periode", Type: metamodel.TypeNumber},
		{Key: "status", Label: "Status", Type: metamodel.TypeSelect, Options: []metamodel.Option{{Value: "OPEN", Label: "offen"}}},
	},
	Actions:       []metamodel.ActionConfig{{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"}},
	Authorization: &metamodel.Authorization{Fields: []string{"ledger", "posting_period"}, Actions: []metamodel.AuthAction{{Name: "post", Label: "Buchen"}}},
}

func (catalogHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Action {
	case "Translations":
		return sdk.Response{Payload: map[string]any{"translations": map[string]string{}}}, nil
	case "ListObjects":
		return sdk.Response{Payload: map[string]any{"objects": []map[string]any{{"object": "FiscalPeriod"}, {"object": "Partner"}}}}, nil
	case "ListActions":
		if req.Payload.(map[string]any)["object"] == "FiscalPeriod" {
			return sdk.Response{Payload: map[string]any{"actions": []map[string]any{{"action": "list"}}}}, nil
		}
		return sdk.Response{}, sdk.ErrNotFound
	case "GetDefinition":
		if req.Payload.(map[string]any)["object"] == "FiscalPeriod" {
			return sdk.Response{Payload: map[string]any{"definition": periodDef}}, nil
		}
	}
	return sdk.Response{}, sdk.ErrNotFound
}

func TestFieldLevelAuthorization(t *testing.T) {
	p, adminID := setup(t)
	p.host = catalogHost{}
	ctx := as(adminID)
	for _, cc := range []string{"1000", "2000"} {
		if _, err := call(t, p, ctx, "CompanyCode", "create", map[string]any{"data": map[string]any{"code": cc}}); err != nil {
			t.Fatal(err)
		}
	}
	role, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": "Buchhaltung"}})
	if err != nil {
		t.Fatal(err)
	}
	roleID := role["id"].(string)

	// Zeile je Object.Action, Feldwerte als Unterzeilen.
	auth, err := call(t, p, ctx, "RoleAuth", "create", map[string]any{"data": map[string]any{
		"role_id": roleID, "object": "FiscalPeriod", "action": "post", "company_codes": "1000"}})
	if err != nil {
		t.Fatal(err)
	}
	authID := auth["id"].(string)
	addValue := func(field, low, high string) (map[string]any, error) {
		return call(t, p, ctx, "RoleAuthValue", "create", map[string]any{"data": map[string]any{
			"auth_id": authID, "field": field, "low": low, "high": high}})
	}
	if _, err := addValue("posting_period", "1", "12"); err != nil {
		t.Fatal(err)
	}
	if _, err := addValue("ledger", "0L", ""); err != nil {
		t.Fatal(err)
	}
	// Nur Felder, die das Object deklariert; Bereiche müssen aufsteigen.
	if _, err := addValue("status", "OPEN", ""); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekanntes Feld: %v", err)
	}
	if _, err := addValue("posting_period", "16", "13"); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("absteigender Bereich: %v", err)
	}
	// Zweite aktive Zeile für dasselbe Object.Action wird abgelehnt.
	if _, err := call(t, p, ctx, "RoleAuth", "create", map[string]any{"data": map[string]any{
		"role_id": roleID, "object": "FiscalPeriod", "action": "post"}}); !errors.Is(err, sdk.ErrAlreadyExists) {
		t.Fatalf("doppelt: %v", err)
	}

	u, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{
		"username": "buchhalter", "password": "buchhalter-pw-1", "roles": "Buchhaltung"}})
	if err != nil {
		t.Fatal(err)
	}
	asUser := as(u["id"].(string))
	check := func(attrs map[string]string) bool {
		t.Helper()
		m, err := call(t, p, asUser, "Account", "Check", map[string]any{"object": "FiscalPeriod", "action": "post", "attrs": attrs})
		if err != nil {
			t.Fatal(err)
		}
		return m["allowed"].(bool)
	}
	ok := map[string]string{"company_code": "1000", "ledger": "0L", "posting_period": "12"}
	if !check(ok) {
		t.Fatal("Periode 12 im Ledger 0L in 1000 ist erlaubt")
	}
	for name, attrs := range map[string]map[string]string{
		"Sonderperiode":       {"company_code": "1000", "ledger": "0L", "posting_period": "13"},
		"anderer Ledger":      {"company_code": "1000", "ledger": "2L", "posting_period": "5"},
		"anderer BK":          {"company_code": "2000", "ledger": "0L", "posting_period": "5"},
		"Feld fehlt":          {"company_code": "1000", "posting_period": "5"},
		"Buchungskreis fehlt": {"ledger": "0L", "posting_period": "5"},
	} {
		if check(attrs) {
			t.Errorf("%s darf nicht erlaubt sein", name)
		}
	}
	// Positive Ergänzung: Ein zweiter Wert erweitert das Recht.
	if _, err := addValue("posting_period", "13", ""); err != nil {
		t.Fatal(err)
	}
	if !check(map[string]string{"company_code": "1000", "ledger": "0L", "posting_period": "13"}) {
		t.Fatal("Periode 13 nach Ergänzung")
	}

	// Granted: Regeln zum lokalen Auswerten; ohne feldfreie Regel kein „All“.
	resp, err := p.Handle(asUser, sdk.Request{Object: "Account", Action: "Granted", Payload: map[string]any{"object": "FiscalPeriod", "action": "post"}})
	if err != nil {
		t.Fatal(err)
	}
	g := resp.Payload.(sdk.GrantSet)
	if g.All || len(g.CompanyCodes) != 0 || len(g.Rules) != 1 || !g.Allows(sdk.Attrs(ok)) {
		t.Fatalf("Granted: %+v", g)
	}
	if got := g.Rules[0].Fields["posting_period"]; len(got) != 2 {
		t.Fatalf("Werte: %v", got)
	}
	// Bisherige API: CheckAccess ohne Feldwerte passt nicht (fail closed).
	if m, _ := call(t, p, asUser, "Account", "Check", map[string]any{"object": "FiscalPeriod", "action": "post", "company_code": "1000"}); m["allowed"].(bool) {
		t.Fatal("Check nur mit Buchungskreis muss bei Feldeinschränkung scheitern")
	}

	// Textform in Rolle und Profil.
	r, _ := call(t, p, ctx, "Role", "get", map[string]any{"id": roleID})
	if r["permissions"] != "FiscalPeriod.post@1000 ledger=0L posting_period=1..12,13" {
		t.Fatalf("Textform: %q", r["permissions"])
	}
	// Die Textform übersteht einen Durchlauf durch Parser und Formatierung.
	gs, err := parsePermissions(r["permissions"].(string))
	if err != nil || len(gs) != 1 || gs[0].String() != r["permissions"] {
		t.Fatalf("Rundlauf: %v %v", gs, err)
	}

	// Liste je Rolle mit lesbaren Texten aus dem Catalog.
	list, err := call(t, p, ctx, "RoleAuth", "list", map[string]any{"query": map[string]any{"role_id": roleID}})
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["restrictions"] != "ledger=0L posting_period=1..12,13" ||
		items[0].(map[string]any)["_labels"].(map[string]any)["object"] != "Buchungsperioden (FiscalPeriod)" {
		t.Fatalf("Liste: %v", items)
	}

	// Entziehen (kein physisches Löschen): danach kein Recht mehr.
	if _, err := call(t, p, ctx, "RoleAuth", "deactivate", map[string]any{"id": authID}); err != nil {
		t.Fatal(err)
	}
	if check(ok) {
		t.Fatal("nach Entzug")
	}
	if ok, _ := p.Allowed(context.Background(), u["id"].(string), "FiscalPeriod", "post"); ok {
		t.Fatal("Allowed nach Entzug")
	}
}

func TestRoleAuthFormState(t *testing.T) {
	p, adminID := setup(t)
	p.host = catalogHost{}
	ctx := as(adminID)
	fs := func(object string, payload metamodel.FormStateRequest) metamodel.FormState {
		t.Helper()
		resp, err := p.Handle(ctx, sdk.Request{Object: object, Action: formStateAction, Payload: payload})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(metamodel.FormState)
	}
	values := func(opts []metamodel.Option) []string {
		var out []string
		for _, o := range opts {
			out = append(out, o.Value)
		}
		return out
	}
	st := fs("RoleAuth", metamodel.FormStateRequest{Mode: "create", Values: map[string]string{"object": "FiscalPeriod"}})
	if got := values(st.Fields["object"].Options); !slices.Equal(got, []string{"*", "FiscalPeriod", "Partner"}) {
		t.Fatalf("Objects: %v", got)
	}
	// Routen und reine Berechtigungs-Actions.
	if got := values(st.Fields["action"].Options); !slices.Equal(got, []string{"*", "list", "post"}) {
		t.Fatalf("Actions: %v", got)
	}
	if a := st.Fields["active"].Visible; a == nil || *a {
		t.Fatal("Neuanlage: Aktiv ausgeblendet (immer aktiv)")
	}
	if v := st.Fields["company_codes"].Value; v == nil || *v != "*" || !strings.Contains(st.Message, "Ledger, Periode") {
		t.Fatalf("Vorbelegung/Hinweis: %+v", st)
	}

	role, _ := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": "Rolle R"}})
	auth, err := call(t, p, ctx, "RoleAuth", "create", map[string]any{"data": map[string]any{
		"role_id": role["id"], "object": "Fiscal*", "action": "post"}})
	if err != nil {
		t.Fatal(err)
	}
	// Muster: Felder aller passenden Objects.
	st = fs("RoleAuthValue", metamodel.FormStateRequest{Mode: "create", Values: map[string]string{"auth_id": auth["id"].(string)}})
	if got := values(st.Fields["field"].Options); !slices.Equal(got, []string{"ledger", "posting_period"}) {
		t.Fatalf("Felder: %v", got)
	}
}

// TestAdminKeepsUnrestrictedAll: Feldwerte am *.* des letzten Administrators
// würden ihn aussperren.
func TestAdminKeepsUnrestrictedAll(t *testing.T) {
	p, adminID := setup(t)
	p.host = catalogHost{}
	ctx := as(adminID)
	list, err := call(t, p, ctx, "RoleAuth", "list", nil)
	if err != nil {
		t.Fatal(err)
	}
	authID := list["items"].([]any)[0].(map[string]any)["id"]
	_, err = call(t, p, ctx, "RoleAuthValue", "create", map[string]any{"data": map[string]any{"auth_id": authID, "field": "ledger", "low": "0L"}})
	if !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("erwartet FailedPrecondition, got %v", err)
	}
	if _, err := call(t, p, ctx, "RoleAuth", "deactivate", map[string]any{"id": authID}); !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("Entzug *.*: %v", err)
	}
}
