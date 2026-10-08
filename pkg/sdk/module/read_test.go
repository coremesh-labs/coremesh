package module

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

func readNames(_ context.Context, req sdk.Request, w sdk.RowWriter) (sdk.ReadEnd, error) {
	if err := w.Header(sdk.ReadHeader{Columns: []string{"name"}}); err != nil {
		return sdk.ReadEnd{}, err
	}
	return sdk.ReadEnd{}, w.Rows([][]any{{req.Object + "." + req.Action}})
}

func TestReadRoutesAndManifest(t *testing.T) {
	var events []string
	m := &fakeModule{desc: Descriptor{Name: "fi", Title: "FI"}, events: &events, routes: func(r *Router) {
		r.Object("Item").Handle("list", echo).Handle("get", echo).Read("list", readNames)
	}}
	p := NewPlugin(Info{Name: "fi", Version: "1.0.0"}, m)
	if p.Err() != nil {
		t.Fatal(p.Err())
	}
	man, _ := p.Manifest(context.Background())
	if c := man.Capabilities[0]; !slices.Equal(c.ReadActions, []string{"list"}) {
		t.Fatalf("Manifest: %+v", c)
	}
	res, _, err := sdk.ReadAll(context.Background(), p, sdk.Request{Object: "Item", Action: "list"})
	if err != nil || res.Rows[0][0] != "Item.list" {
		t.Fatalf("Read: %v %v", err, res)
	}
	if _, _, err := sdk.ReadAll(context.Background(), p, sdk.Request{Object: "Item", Action: "get"}); !errors.Is(err, sdk.ErrUnimplemented) {
		t.Fatalf("get ist kein Datenstrom: %v", err)
	}
}

func TestReadRegistrationErrors(t *testing.T) {
	var events []string
	for name, routes := range map[string]func(r *Router){
		"ohne Handle": func(r *Router) { r.Object("Item").Handle("get", echo).Read("list", readNames) },
		"doppelt":     func(r *Router) { r.Object("Item").Handle("list", echo).Read("list", readNames).Read("list", readNames) },
		"nil":         func(r *Router) { r.Object("Item").Handle("list", echo).Read("list", nil) },
	} {
		p := NewPlugin(Info{Name: "fi", Version: "1.0.0"}, &fakeModule{desc: Descriptor{Name: "fi", Title: "FI"}, events: &events, routes: routes})
		if p.Err() == nil || !strings.Contains(p.Err().Error(), "Read") {
			t.Errorf("%s: Fehler erwartet: %v", name, p.Err())
		}
	}
}
