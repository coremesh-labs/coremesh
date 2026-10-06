package metamodel

import (
	"strings"
	"testing"
)

func partner() ObjectDefinition {
	return ObjectDefinition{
		Name:  "BusinessPartner",
		Title: "Geschäftspartner",
		Icon:  "icon-users",
		Fields: []FieldDefinition{
			{Key: "company_name", Label: "Firmenname", Type: TypeText, Required: true, Listable: true, Editable: true},
			{Key: "email", Label: "E-Mail", Type: TypeEmail, Listable: true, Editable: true},
			{Key: "kind", Label: "Art", Type: TypeSelect, Editable: true, Options: []Option{
				{Value: "customer", Label: "Kunde"}, {Value: "supplier", Label: "Lieferant"},
			}},
		},
		Actions: []ActionConfig{
			{Name: "list", Kind: KindList, Label: "Übersicht"},
			{Name: "get", Kind: KindItem, Label: "Anzeigen"},
			{Name: "delete", Kind: KindDelete, Label: "Löschen", Confirm: "Wirklich löschen?"},
		},
	}
}

func TestValidateOK(t *testing.T) {
	if err := partner().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]func(d *ObjectDefinition){
		"name":             func(d *ObjectDefinition) { d.Name = "businessPartner" },
		"title fehlt":      func(d *ObjectDefinition) { d.Title = "" },
		"key doppelt":      func(d *ObjectDefinition) { d.Fields[1].Key = "company_name" },
		"key":              func(d *ObjectDefinition) { d.Fields[0].Key = "CompanyName" },
		"unbekannter type": func(d *ObjectDefinition) { d.Fields[0].Type = "money" },
		"select braucht":   func(d *ObjectDefinition) { d.Fields[2].Options = nil },
		"options nur":      func(d *ObjectDefinition) { d.Fields[0].Options = []Option{{Value: "x", Label: "X"}} },
		"value leer":       func(d *ObjectDefinition) { d.Fields[2].Options[1].Value = "customer" },
		"unbekannter kind": func(d *ObjectDefinition) { d.Actions[0].Kind = "export" },
		"name doppelt":     func(d *ObjectDefinition) { d.Actions[1].Name = "list" },
	}
	for want, mutate := range cases {
		t.Run(want, func(t *testing.T) {
			d := partner()
			mutate(&d)
			err := d.Validate()
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("Fehler mit %q erwartet, bekommen: %v", want, err)
			}
		})
	}
}
