package main

import (
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// „Wieder aktivieren“ erscheint nur bei inaktiven Datensätzen.
func TestReactivateOnlyInactive(t *testing.T) {
	d := metamodel.ObjectDefinition{Name: "Acc", Title: "Konto",
		Lifecycle: metamodel.Lifecycle{Type: metamodel.LifecycleStatus, StatusField: "is_active"},
		Actions: []metamodel.ActionConfig{{Name: "deactivate", Kind: metamodel.KindDeactivate, Label: "Inaktivieren"},
			{Name: crud.ReactivateAction, Kind: metamodel.KindCustom, Label: "Wieder aktivieren", Record: true}}}
	oc := newObjectCtx("m", "Acc", d)
	names := func(rec record) (out []string) {
		for _, a := range recordActions(oc, rec) {
			out = append(out, a.Name)
		}
		return out
	}
	if n := names(record{"is_active": true}); len(n) != 0 {
		t.Fatalf("aktiv: %v", n)
	}
	if n := names(record{"is_active": false}); len(n) != 1 || n[0] != crud.ReactivateAction {
		t.Fatalf("inaktiv: %v", n)
	}
}
