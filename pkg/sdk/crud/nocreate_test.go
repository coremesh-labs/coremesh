package crud

import (
	"context"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"

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

// TestRoutes: Routes sind Actions ohne Metamodell.
func TestRoutes(t *testing.T) {
	e := &Entity{Object: "Doc", Title: "Dokumente", Table: "doc", Keys: []string{"id"}, Fields: []Field{
		{Key: "id", Label: "ID", Type: metamodel.TypeText, Required: true},
	}, Routes: map[string]module.HandlerFunc{"content": func(context.Context, sdk.Request) (sdk.Response, error) {
		return sdk.Response{Payload: "x"}, nil
	}}}
	set := NewSet(e)
	for _, a := range e.Definition().Actions {
		if a.Name == "content" {
			t.Fatal("content im Metamodell")
		}
	}
	p := module.NewPlugin(module.Info{Name: "tt", Version: "1"}, routesModule{set})
	if err := p.Err(); err != nil {
		t.Fatal(err)
	}
	resp, err := p.Handle(context.Background(), sdk.Request{Object: "Doc", Action: "content"})
	if err != nil || resp.Payload != "x" {
		t.Fatalf("%v %v", resp.Payload, err)
	}
}

type routesModule struct{ set *Set }

func (m routesModule) Descriptor() module.Descriptor              { return module.Descriptor{Name: "tt", Title: "T"} }
func (m routesModule) RegisterRoutes(r *module.Router)            { m.set.Register(r, "T") }
func (routesModule) Initialize(context.Context, module.Env) error { return nil }
func (routesModule) Shutdown(context.Context) error               { return nil }
