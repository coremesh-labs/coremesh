package crud

import (
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestNoCreate: ändern ja, anlegen nicht (weder Action noch Route).
func TestNoCreate(t *testing.T) {
	e := &Entity{Object: "Doc", Title: "Dokumente", Table: "doc", Keys: []string{"id"}, NoCreate: true, Fields: []Field{
		{Key: "id", Label: "ID", Type: metamodel.TypeText, Required: true},
	}}
	NewSet(e)
	var kinds []metamodel.ActionKind
	for _, a := range e.Definition().Actions {
		kinds = append(kinds, a.Kind)
	}
	for _, k := range kinds {
		if k == metamodel.KindCreate {
			t.Fatalf("create angeboten: %v", kinds)
		}
	}
	if len(kinds) != 3 || kinds[2] != metamodel.KindUpdate {
		t.Fatalf("Actions: %v", kinds)
	}
}
