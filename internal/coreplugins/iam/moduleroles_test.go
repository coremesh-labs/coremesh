package iam

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// moduleHost spielt den Catalog mit einem Modul contract: Contract (Objekt mit
// Verweis auf BusinessPartner und Abschnitt Merkmale), ContractType (Einstellung),
// Dienst ContractSetup (Einstellung) und ContractPostingService.
type moduleHost struct{ nopHost }

var moduleDefs = map[string]metamodel.ObjectDefinition{
	"Contract": {Name: "Contract", Title: "Verträge",
		Fields:   []metamodel.FieldDefinition{{Key: "partner_id", Label: "Partner", Type: metamodel.TypeText, Lookup: &metamodel.Lookup{Object: "BusinessPartner", ValueField: "id"}}},
		Sections: []metamodel.SectionDefinition{{Key: "merkmale", Title: "Merkmale", Tags: true}}},
	"ContractType": {Name: "ContractType", Title: "Vertragsarten"},
}

func (moduleHost) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	p, _ := req.Payload.(map[string]any)
	switch req.Action {
	case "Translations":
		return sdk.Response{Payload: map[string]any{"translations": map[string]string{}}}, nil
	case "ListModules":
		return sdk.Response{Payload: map[string]any{"modules": []metamodel.ModuleDefinition{{Name: "contract", Title: "Verträge",
			Objects:     []metamodel.ModuleObject{{Object: "Contract"}, {Object: "ContractType"}},
			Services:    []string{"ContractSetup", "ContractPostingService"},
			Customizing: []string{"ContractType", "ContractSetup"}}}}}, nil
	case "ListActions":
		acts := map[string][]string{"Contract": {"list", "get", "create", "update"}, "ContractType": {"list", "get", "create", "update"},
			"BusinessPartner": {"list", "get", "create"}}[p["object"].(string)]
		var out []map[string]any
		for _, a := range acts {
			out = append(out, map[string]any{"action": a})
		}
		return sdk.Response{Payload: map[string]any{"actions": out}}, nil
	case "GetDefinition":
		if d, ok := moduleDefs[p["object"].(string)]; ok {
			return sdk.Response{Payload: map[string]any{"definition": d}}, nil
		}
	}
	return sdk.Response{}, sdk.ErrNotFound
}

func TestSetupModuleRoles(t *testing.T) {
	p, adminID := setup(t)
	p.host = moduleHost{}
	ctx := as(adminID)
	st, err := p.Handle(ctx, sdk.Request{Object: "Role", Action: formStateAction, Payload: map[string]any{"mode": "action", "action": setupModuleRolesAction, "values": map[string]any{}}})
	if err != nil || !strings.Contains(fmt.Sprintf("%+v", st.Payload), "Value:contract") {
		t.Fatalf("Formular: %+v %v", st.Payload, err)
	}
	r, err := call(t, p, ctx, "Role", setupModuleRolesAction, map[string]any{"data": map[string]any{"modules": "contract"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r["message"].(string), "Verträge - Sachbearbeiter (angelegt") {
		t.Fatalf("Meldung: %v", r)
	}
	perms := func(name string) []string {
		roles, err := p.listRoles(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, ro := range roles {
			if ro.Name == name {
				return formatPermissions(ro.Permissions)
			}
		}
		t.Fatalf("Rolle %s fehlt", name)
		return nil
	}
	clerk, expert := perms("Verträge - Sachbearbeiter"), perms("Verträge - Fachverantwortlicher")
	for _, want := range []string{"Contract.*", "ContractType.list", "ContractType.get", "ContractPostingService.*",
		"BusinessPartner.list", "BusinessPartner.get", "Tags.*"} {
		if !slices.Contains(clerk, want) {
			t.Errorf("Sachbearbeiter ohne %s: %v", want, clerk)
		}
	}
	if slices.Contains(clerk, "ContractType.*") || slices.Contains(clerk, "ContractSetup.*") || slices.Contains(clerk, "BusinessPartner.create") {
		t.Errorf("Sachbearbeiter darf zu viel: %v", clerk)
	}
	for _, want := range []string{"ContractType.*", "ContractSetup.*", "Contract.*"} {
		if !slices.Contains(expert, want) {
			t.Errorf("Fachverantwortlicher ohne %s: %v", want, expert)
		}
	}
	// Zweiter Lauf: nichts neu, eingeschränkte Buchungskreise bleiben.
	r, err = call(t, p, ctx, "Role", setupModuleRolesAction, map[string]any{"data": map[string]any{"modules": "contract"}})
	if err != nil || !strings.HasPrefix(r["message"].(string), "Modulrollen: 0 Berechtigungen neu") {
		t.Fatalf("zweiter Lauf: %v %v", r, err)
	}
}
