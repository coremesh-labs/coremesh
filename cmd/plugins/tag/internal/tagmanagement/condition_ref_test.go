package tagmanagement

import (
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// testDefs: Metamodelle der Attrappen-Objects (Catalog.GetDefinition).
var testDefs = map[string]metamodel.ObjectDefinition{
	"Contract": {Name: "Contract", Title: "Verträge", Fields: []metamodel.FieldDefinition{
		{Key: "id", Label: "ID", Type: metamodel.TypeText},
		{Key: "contract_type", Label: "Vertragsart", Type: metamodel.TypeSelect, Options: []metamodel.Option{
			{Value: "RENT", Label: "Mietvertrag"}, {Value: "INSURANCE", Label: "Versicherungsvertrag"}, {Value: "LOAN", Label: "Darlehensvertrag"}}},
		{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean},
	}},
	"RentalObject": {Name: "RentalObject", Title: "Mietobjekte", TitleField: "name", Fields: []metamodel.FieldDefinition{
		{Key: "id", Label: "ID", Type: metamodel.TypeText}, {Key: "name", Label: "Bezeichnung", Type: metamodel.TypeText}}},
}

// defineContracts: RENT_SET nur für Mietverträge (mit Verweis auf das Mietobjekt),
// LOAN_SET für Darlehens- und Versicherungsverträge, GENERAL für alle Verträge.
func (e *env) defineContracts() {
	e.must("TagType", "create", map[string]any{"code": "RENTAL_OBJECT", "name": "Mietobjekt", "data_type": "REFERENCE", "value_mode": "FREE", "ref_object": "RentalObject"})
	e.must("TagType", "create", map[string]any{"code": "INTEREST", "name": "Zinssatz (Basispunkte)", "data_type": "INTEGER", "value_mode": "FREE"})
	e.must("TagType", "create", map[string]any{"code": "NOTE", "name": "Notiz", "data_type": "STRING", "value_mode": "FREE"})
	for set, tag := range map[string]string{"RENT_SET": "RENTAL_OBJECT", "LOAN_SET": "INTEREST", "GENERAL": "NOTE"} {
		e.must("TagSet", "create", map[string]any{"code": set, "name": set, "valid_from": "2000-01-01"})
		e.must("TagSetItem", "create", map[string]any{"tag_set_code": set, "tag_type_code": tag, "valid_from": "2000-01-01", "mandatory": set == "RENT_SET"})
	}
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "RENT_SET",
		"condition_field": "contract_type", "condition_values": "RENT", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "LOAN_SET",
		"condition_field": "contract_type", "condition_values": " LOAN ;INSURANCE, LOAN", "valid_from": "2000-01-01"})
	e.must("TagSetAssignment", "create", map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "GENERAL", "valid_from": "2000-01-01"})
}

func setNames(s tagservice.Schema) string {
	out := ""
	for _, set := range s.Sets {
		out += set.Code + ";"
	}
	return out
}

func TestSetConditionByField(t *testing.T) {
	e := setup(t)
	e.defineContracts()

	// Werte werden normalisiert gespeichert.
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: "TagSetAssignment", Action: "get",
		Payload: map[string]any{"id": "Contract|*|LOAN_SET|2000-01-01"}})
	if err != nil {
		t.Fatal(err)
	}
	if v := resp.Payload.(map[string]any)["condition_values"]; v != "LOAN,INSURANCE" {
		t.Fatalf("condition_values: %v", v)
	}

	// Mietvertrag: RENT_SET + GENERAL, kein LOAN_SET – und umgekehrt.
	for id, want := range map[string]string{"c1": "GENERAL;RENT_SET;", "c2": "GENERAL;LOAN_SET;"} {
		et, err := e.svc.Get(e.ctx, tagservice.GetRequest{EntityType: "Contract", EntityID: id})
		if err != nil {
			t.Fatal(err)
		}
		if got := setNames(et.Schema); got != want {
			t.Fatalf("%s: Sets %s, erwartet %s", id, got, want)
		}
	}
	// Schema ohne Datensatz: alle Sets samt Bedingung; mit Attributen gefiltert.
	all, _ := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "Contract"})
	if setNames(all) != "GENERAL;LOAN_SET;RENT_SET;" || all.Sets[2].Condition == nil || all.Sets[2].Condition.Field != "contract_type" {
		t.Fatalf("Schema ohne Datensatz: %+v", all.Sets)
	}
	ins, _ := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "Contract", Attributes: map[string]string{"contract_type": "INSURANCE"}})
	if setNames(ins) != "GENERAL;LOAN_SET;" {
		t.Fatalf("Schema mit Attributen: %s", setNames(ins))
	}
	byID, _ := e.svc.Schema(e.ctx, tagservice.GetRequest{EntityType: "Contract", EntityID: "c1"})
	if setNames(byID) != "GENERAL;RENT_SET;" {
		t.Fatalf("Schema mit Datensatz: %s", setNames(byID))
	}

	// Tags aus einem Set, dessen Bedingung nicht gilt, sind nicht zugewiesen.
	_, err = e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "Contract", EntityID: "c2",
		Values: map[string]*tagservice.Value{"RENTAL_OBJECT": tagservice.Ref("ro1")}})
	expect(t, err, sdk.ErrInvalidArgument, "Mietobjekt am Darlehensvertrag")

	// Ungültige Bedingungen.
	for name, data := range map[string]map[string]any{
		"unbekanntes Feld":      {"condition_field": "kind", "condition_values": "RENT"},
		"Wert nicht erlaubt":    {"condition_field": "contract_type", "condition_values": "RENT,LEASING"},
		"Feld ohne Werte":       {"condition_field": "contract_type", "condition_values": " , "},
		"Werte ohne Feld":       {"condition_values": "RENT"},
		"Ja/Nein-Feld":          {"condition_field": "active", "condition_values": "ja"},
		"unbekannter Objekttyp": {"entity_type": "Unknown", "condition_field": "x", "condition_values": "1"},
	} {
		d := map[string]any{"entity_type": "Contract", "company_code": "*", "tag_set_code": "GENERAL", "valid_from": "2030-01-01"}
		for k, v := range data {
			d[k] = v
		}
		_, err := e.p.Handle(e.ctx, sdk.Request{Object: "TagSetAssignment", Action: "create", Payload: map[string]any{"data": d}})
		expect(t, err, sdk.ErrInvalidArgument, name)
	}
}

func TestReferenceTag(t *testing.T) {
	e := setup(t)
	e.defineContracts()

	// Definition: Ziel nötig und bekannt, nur bei REFERENCE, keine Auswahlwerte.
	for name, data := range map[string]map[string]any{
		"ohne Ziel":         {"code": "R1", "name": "x", "data_type": "REFERENCE", "value_mode": "FREE"},
		"unbekanntes Ziel":  {"code": "R2", "name": "x", "data_type": "REFERENCE", "value_mode": "FREE", "ref_object": "Spaceship"},
		"Ziel bei STRING":   {"code": "R3", "name": "x", "data_type": "STRING", "value_mode": "FREE", "ref_object": "RentalObject"},
		"mit Auswahlwerten": {"code": "R4", "name": "x", "data_type": "REFERENCE", "value_mode": "OPTIONS", "ref_object": "RentalObject"},
	} {
		_, err := e.p.Handle(e.ctx, sdk.Request{Object: "TagType", Action: "create", Payload: map[string]any{"data": data}})
		expect(t, err, sdk.ErrInvalidArgument, name)
	}

	// Das Ziel muss existieren.
	_, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "Contract", EntityID: "c1", ValidFrom: "2026-01-01",
		Values: map[string]*tagservice.Value{"RENTAL_OBJECT": tagservice.Ref("ro9")}})
	expect(t, err, sdk.ErrInvalidArgument, "unbekanntes Mietobjekt")
	vs, _ := e.svc.Validate(e.ctx, tagservice.SetRequest{EntityType: "Contract", EntityID: "c1",
		Values: map[string]*tagservice.Value{"RENTAL_OBJECT": tagservice.Ref("ro9")}})
	if len(vs) != 1 || vs[0].Code != "reference" {
		t.Fatalf("Verstöße: %+v", vs)
	}

	et, err := e.svc.Set(e.ctx, tagservice.SetRequest{EntityType: "Contract", EntityID: "c1", ValidFrom: "2026-01-01",
		Values: map[string]*tagservice.Value{"RENTAL_OBJECT": tagservice.Ref("ro1")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(et.Values) != 1 || *et.Values[0].Value.Ref != "ro1" || et.Values[0].RefLabel != "Wohnung 3. OG" {
		t.Fatalf("Werte: %+v", et.Values)
	}
	if et.Schema.Sets[1].Items[0].Tag.RefObject != "RentalObject" {
		t.Fatalf("RefObject fehlt im Schema: %+v", et.Schema.Sets[1].Items[0].Tag)
	}
	// Suche über den Verweis: Welche Verträge betreffen Mietobjekt ro1?
	ids, err := e.svc.Find(e.ctx, tagservice.FindRequest{EntityType: "Contract", Tag: "RENTAL_OBJECT", Value: tagservice.Ref("ro1"), EffectiveDate: "2026-06-01"})
	if err != nil || len(ids) != 1 || ids[0] != "c1" {
		t.Fatalf("Find: %v %v", ids, err)
	}
}
