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
			{Key: "active", Label: "Aktiv", Type: TypeBoolean, Listable: true},
		},
		Actions: []ActionConfig{
			{Name: "list", Kind: KindList, Label: "Übersicht"},
			{Name: "get", Kind: KindItem, Label: "Anzeigen"},
			{Name: "deactivate", Kind: KindDeactivate, Label: "Inaktivieren", Confirm: "Wirklich inaktivieren?"},
		},
		Lifecycle: Lifecycle{Type: LifecycleStatus, StatusField: "active"},
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
		"lookup.object": func(d *ObjectDefinition) {
			d.Fields[0].Lookup = &Lookup{Object: "x", ValueField: "code", LabelFields: []string{"d"}}
		},
		"label_fields": func(d *ObjectDefinition) { d.Fields[0].Lookup = &Lookup{Object: "Role", ValueField: "code"} },
		"lookup nicht": func(d *ObjectDefinition) {
			d.Fields[2].Lookup = &Lookup{Object: "Role", ValueField: "code", LabelFields: []string{"d"}}
		},
		"title_field": func(d *ObjectDefinition) { d.TitleField = "gibts_nicht" },
		"genau eines": func(d *ObjectDefinition) { d.Sections = []SectionDefinition{{Key: "a", Title: "A"}} },
		"gibt es nicht": func(d *ObjectDefinition) {
			d.Sections = []SectionDefinition{{Key: "a", Title: "A", Fields: []string{"x"}}}
		},
		"steht schon": func(d *ObjectDefinition) {
			d.Sections = []SectionDefinition{{Key: "a", Title: "A", Fields: []string{"email"}}, {Key: "b", Title: "B", Fields: []string{"email"}}}
		},
		"kind delete gibt es nicht": func(d *ObjectDefinition) {
			d.Actions[2] = ActionConfig{Name: "delete", Kind: "delete", Label: "Löschen"}
		},
		"passt nicht zum Lifecycle":   func(d *ObjectDefinition) { d.Actions[2].Kind = KindExpire },
		"status_field":                func(d *ObjectDefinition) { d.Lifecycle.StatusField = "email" },
		"valid_to":                    func(d *ObjectDefinition) { d.Lifecycle = Lifecycle{Type: LifecycleTimeSlice, ValidFrom: "x"} },
		"keine Felder angeben":        func(d *ObjectDefinition) { d.Lifecycle = Lifecycle{Type: LifecycleImmutable, StatusField: "active"} },
		"lifecycle: unbekannter type": func(d *ObjectDefinition) { d.Lifecycle.Type = "soft" },
		"relation braucht": func(d *ObjectDefinition) {
			d.Sections = []SectionDefinition{{Key: "a", Title: "A", Relation: &Relation{Object: "Contact"}}}
		},
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

func TestValidateSectionsAndLookups(t *testing.T) {
	d := partner()
	d.TitleField = "company_name"
	d.Fields[0].Lookup = &Lookup{Object: "Company", ValueField: "id", LabelFields: []string{"name"}, Columns: []string{"id", "name"}}
	d.Sections = []SectionDefinition{
		{Key: "base", Title: "Stammdaten", Fields: []string{"company_name", "email"}},
		{Key: "contacts", Title: "Kontakte", Collapsed: true, Relation: &Relation{Object: "Contact", ForeignKey: "partner_id"}},
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}
