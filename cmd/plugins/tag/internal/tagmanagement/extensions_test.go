package tagmanagement

import (
	"errors"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// TestSecondConditionMultiValue: zweite Bedingung (UND); Felder mit mehreren
// Werten (Rollen "TENANT,DEBITOR") genügen mit einem Wert.
func TestSecondConditionMultiValue(t *testing.T) {
	e := setup(t)
	e.must("TagType", "create", map[string]any{"code": "TAX_ID", "name": "Steuer-ID", "data_type": "STRING", "value_mode": "FREE"})
	e.must("TagSet", "create", map[string]any{"code": "TENANT_SET", "name": "Mieter", "valid_from": "2000-01-01"})
	e.must("TagSetItem", "create", map[string]any{"tag_set_code": "TENANT_SET", "tag_type_code": "TAX_ID", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "TENANT_SET",
		"condition_field": "contract_type", "condition_values": "RENT,LOAN", "condition_field_2": "roles", "condition_values_2": "TENANT",
		"valid_from": "2000-01-01"})
	for id, want := range map[string]string{"c1": "TENANT_SET;", "c2": ""} {
		et, err := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "Contract", EntityID: id})
		if err != nil {
			t.Fatal(err)
		}
		if got := setNames(et.Schema); got != want {
			t.Fatalf("%s: %q, erwartet %q", id, got, want)
		}
	}
	all, _ := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "Contract"})
	if c := all.Sets[0].Condition; c == nil || c.And == nil || c.And.Field != "roles" {
		t.Fatalf("Bedingung im Schema: %+v", all.Sets)
	}
	// nur zweite Bedingung angegeben: wird zur ersten
	e.must("TagSet", "create", map[string]any{"code": "ONLY2", "name": "x", "valid_from": "2000-01-01"})
	r := e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "ONLY2",
		"condition_field_2": "roles", "condition_values_2": "CREDITOR", "valid_from": "2000-01-01"})
	if r["condition_field"] != "roles" || r["condition_field_2"] != nil {
		t.Fatalf("Bedingung verschoben: %v", r)
	}
}

// TestPatternAndProtection: Prüfmuster für Text-Tags; geschützte Tags nur mit
// TagType.readValue/changeValue (je Code).
func TestPatternAndProtection(t *testing.T) {
	e := setup(t)
	if _, err := e.p.Handle(e.ctx, sdk.Request{Object: "TagType", Action: "create", Payload: map[string]any{"data": map[string]any{
		"code": "NUM", "name": "Zahl", "data_type": "INTEGER", "value_mode": "FREE", "pattern": `\d+`}}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Muster bei Ganzzahl: %v", err)
	}
	e.must("TagType", "create", map[string]any{"code": "TAX_ID", "name": "Steuer-ID", "data_type": "STRING", "value_mode": "FREE",
		"pattern": `[1-9]\d{10}`, "pattern_hint": "11 Ziffern", "protected": true})
	e.must("TagSet", "create", map[string]any{"code": "PRIV", "name": "Privat", "valid_from": "2000-01-01"})
	e.must("TagSetItem", "create", map[string]any{"tag_set_code": "PRIV", "tag_type_code": "TAX_ID", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "PRIV", "valid_from": "2000-01-01"})
	set := func(v string) error {
		_, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "Contract", EntityID: "c1", Values: map[string]*tagservice.Value{"TAX_ID": tagservice.String(v)}})
		return err
	}
	e.h.granted["TagType.changeValue"] = []string{"TAX_ID"}
	if err := set("123"); !errors.Is(err, sdk.ErrInvalidArgument) || !strings.Contains(err.Error(), "11 Ziffern") {
		t.Fatalf("Muster: %v", err)
	}
	if err := set("12345678901"); err != nil {
		t.Fatal(err)
	}
	get := func() tagservice.EntityTags {
		et, err := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "Contract", EntityID: "c1"})
		if err != nil {
			t.Fatal(err)
		}
		return et
	}
	if et := get(); len(et.Values) != 0 || !et.Schema.Sets[0].Items[0].Tag.Hidden {
		t.Fatalf("ohne Leserecht sichtbar: %+v", et.Values)
	}
	e.h.granted["TagType.readValue"] = []string{"TAX_ID"}
	if et := get(); len(et.Values) != 1 || et.Schema.Sets[0].Items[0].Tag.Hidden {
		t.Fatalf("mit Leserecht: %+v", et.Values)
	}
	e.h.granted["TagType.changeValue"] = nil
	if err := set("98765432109"); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("ohne Änderungsrecht: %v", err)
	}
}
