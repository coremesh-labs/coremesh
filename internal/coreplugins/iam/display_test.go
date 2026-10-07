package iam

import (
	"errors"
	"strings"
	"testing"

	"github.com/coremesh-lab/coremesh/pkg/sdk"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
)

func TestDisplayRules(t *testing.T) {
	p, adminID := setup(t)
	p.host = catalogHost{}
	ctx := as(adminID)
	if _, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": "Buchhaltung"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, p, ctx, "DisplayRule", "create", map[string]any{"data": map[string]any{
		"name": "X", "object": "FiscalPeriod", "roles": "Gibtsnicht"}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekannte Rolle: %v", err)
	}
	rule, err := call(t, p, ctx, "DisplayRule", "create", map[string]any{"data": map[string]any{
		"name": "Offene Perioden", "object": "FiscalPeriod", "roles": "Buchhaltung"}})
	if err != nil {
		t.Fatal(err)
	}
	ruleID := rule["id"]
	add := func(object string, data map[string]any) error {
		data["rule_id"] = ruleID
		_, err := call(t, p, ctx, object, "create", map[string]any{"data": data})
		return err
	}
	if err := add("DisplayRuleCondition", map[string]any{"field": "status", "field_values": "OPEN, OPEN"}); err != nil {
		t.Fatal(err)
	}
	if err := add("DisplayRuleField", map[string]any{"field": "posting_period", "mode": "hidden"}); err != nil {
		t.Fatal(err)
	}
	// Pflichtfeld nicht ausblenden, aber unänderbar; unbekanntes Feld abgelehnt.
	if err := add("DisplayRuleField", map[string]any{"field": "ledger", "mode": "hidden"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Pflichtfeld ausblenden: %v", err)
	}
	if err := add("DisplayRuleField", map[string]any{"field": "ledger", "mode": "readonly"}); err != nil {
		t.Fatal(err)
	}
	if err := add("DisplayRuleField", map[string]any{"field": "gibtsnicht", "mode": "readonly"}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekanntes Feld: %v", err)
	}

	got, _ := call(t, p, ctx, "DisplayRule", "get", map[string]any{"id": ruleID})
	if got["summary"] != "wenn Status = OPEN: ausblenden Periode · unänderbar Ledger" {
		t.Fatalf("Zusammenfassung: %q", got["summary"])
	}

	// Auswahl der Felder und Werte aus dem Catalog.
	resp, err := p.Handle(ctx, sdk.Request{Object: "DisplayRuleCondition", Action: formStateAction, Payload: metamodel.FormStateRequest{
		Mode: "create", Values: map[string]string{"rule_id": ruleID.(string), "field": "status"}}})
	if err != nil {
		t.Fatal(err)
	}
	st := resp.Payload.(metamodel.FormState)
	if len(st.Fields["field"].Options) != 3 || !strings.Contains(st.Message, "OPEN = offen") {
		t.Fatalf("FormState: %+v", st)
	}
	resp, _ = p.Handle(ctx, sdk.Request{Object: "DisplayRuleField", Action: formStateAction, Payload: metamodel.FormStateRequest{
		Mode: "create", Values: map[string]string{"rule_id": ruleID.(string), "mode": "hidden"}}})
	for _, o := range resp.Payload.(metamodel.FormState).Fields["field"].Options {
		if o.Value == "ledger" {
			t.Fatal("Pflichtfeld zum Ausblenden angeboten")
		}
	}

	// Account.Display: nur für Benutzer mit der Rolle.
	display := func(uid string) DisplayRuleSet {
		t.Helper()
		resp, err := p.Handle(as(uid), sdk.Request{Object: "Account", Action: "Display", Payload: map[string]any{"object": "FiscalPeriod"}})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(DisplayRuleSet)
	}
	if n := len(display(adminID).Rules); n != 0 {
		t.Fatalf("Admin ohne Rolle Buchhaltung: %d Regeln", n)
	}
	u, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{
		"username": "bh", "password": "buchhalter-pw-1", "roles": "Buchhaltung"}})
	if err != nil {
		t.Fatal(err)
	}
	rs := display(u["id"].(string))
	if len(rs.Rules) != 1 || rs.Rules[0].Conditions[0].Field != "status" || rs.Rules[0].Hidden[0] != "posting_period" || rs.Rules[0].Readonly[0] != "ledger" {
		t.Fatalf("Regeln: %+v", rs)
	}
	// Inaktiv: wirkt nicht mehr.
	if _, err := call(t, p, ctx, "DisplayRule", "deactivate", map[string]any{"id": ruleID}); err != nil {
		t.Fatal(err)
	}
	if n := len(display(u["id"].(string)).Rules); n != 0 {
		t.Fatalf("nach Inaktivieren: %d", n)
	}
}
