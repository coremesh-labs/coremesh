package ai

import (
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Abhängige Filter (z. B. Vertragsart über den Vertrag) mit Vorgaben festlegen.
func TestBindLookup(t *testing.T) {
	lk := &metamodel.Lookup{Object: "ConditionType", ValueField: "code",
		Filters: map[string]string{"company_code": "company_code", "contract_type": "contract_id.contract_type"}}
	known := func(src string) (string, bool) { return "DLHZ", src == "contract_id.contract_type" }
	got := bindLookup(lk, known)
	if got.Filters["contract_type"] != "=DLHZ" || got.Filters["company_code"] != "company_code" {
		t.Fatalf("Filter: %v", got.Filters)
	}
	if lk.Filters["contract_type"] != "contract_id.contract_type" {
		t.Fatal("Original geändert")
	}
	if bindLookup(lk, func(string) (string, bool) { return "", false }) != lk {
		t.Fatal("ohne Wert: Kopie statt Original")
	}
}
