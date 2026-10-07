package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

var partnerDef = metamodel.ObjectDefinition{
	Name: "BusinessPartner", Title: "Geschäftspartner",
	Fields: []metamodel.FieldDefinition{
		{Key: "company_name", Label: "Firmenname", Type: metamodel.TypeText, Required: true, Editable: true},
		{Key: "email", Label: "E-Mail", Type: metamodel.TypeEmail, Editable: true},
		{Key: "employees", Label: "Mitarbeitende", Type: metamodel.TypeNumber, Editable: true},
		{Key: "founded", Label: "Gründung", Type: metamodel.TypeDate, Editable: true},
		{Key: "kind", Label: "Art", Type: metamodel.TypeSelect, Required: true, Editable: true,
			Options: []metamodel.Option{{Value: "customer", Label: "Kunde"}, {Value: "supplier", Label: "Lieferant"}}},
		{Key: "active", Label: "Aktiv", Type: metamodel.TypeBoolean, Editable: true},
		{Key: "created_at", Label: "Angelegt", Type: metamodel.TypeText}, // nicht bearbeitbar → nicht in der Vorlage
	},
}

func TestSampleYAMLDefault(t *testing.T) {
	b, format, ctype, err := renderSample(partnerDef, "")
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatYAML || ctype != "application/yaml" {
		t.Fatalf("Standardformat: %s %s", format, ctype)
	}
	out := string(b)
	t.Logf("\n%s", out)
	for _, want := range []string{
		"# Beispieldatei für Geschäftspartner (BusinessPartner)",
		"# Pflichtfelder: company_name, kind",
		"# Firmenname\n#   Typ: Text · Pflichtfeld\ncompany_name: Beispiel Firmenname",
		"#   Typ: E-Mail-Adresse · optional\nemail: max.mustermann@example.com",
		"employees: 100",
		`founded: "2026-01-01"`, // als String, nicht als Zeitstempel
		"#   Erlaubte Werte: customer (Kunde), supplier (Lieferant)\nkind: customer",
		"active: true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fehlt: %q", want)
		}
	}
	if strings.Contains(out, "created_at") {
		t.Error("nicht bearbeitbares Feld in der Vorlage")
	}
	// Gültiges YAML mit den richtigen Typen.
	var parsed map[string]any
	if err := yaml.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["employees"] != 100 || parsed["founded"] != "2026-01-01" || parsed["active"] != true {
		t.Fatalf("Typen: %#v", parsed)
	}
}

func TestSampleJSONAndCSV(t *testing.T) {
	b, _, _, err := renderSample(partnerDef, "json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["kind"] != "customer" || m["employees"] != float64(100) {
		t.Fatalf("JSON: %v", m)
	}
	if strings.Index(string(b), "company_name") > strings.Index(string(b), "email") {
		t.Error("Reihenfolge des Metamodells nicht eingehalten")
	}

	b, format, _, err := renderSample(partnerDef, "CSV")
	if err != nil || format != FormatCSV {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "company_name" || rows[1][0] != "Beispiel Firmenname" || rows[1][4] != "customer" {
		t.Fatalf("CSV: %v", rows)
	}

	if _, _, _, err := renderSample(partnerDef, "xml"); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("unbekanntes Format: %v", err)
	}
	if got := sampleFilename("BusinessPartner", "yaml"); got != "business_partner_sample.yaml" {
		t.Fatal(got)
	}
}
