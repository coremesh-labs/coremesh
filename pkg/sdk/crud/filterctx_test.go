package crud

import (
	"context"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// TestFilterCtx: Filter mit Aufrufkontext, z. B. „nur meine“ über den Benutzer.
func TestFilterCtx(t *testing.T) {
	e, user, _ := contracts(t)
	e.Filters = []string{"mine"}
	e.FilterCtx = map[string]func(ctx context.Context, v any) (string, []any, error){"mine": func(ctx context.Context, v any) (string, []any, error) {
		if sdk.CallFromContext(ctx).UserID == "u1" {
			return "property = ?", []any{"P1"}, nil
		}
		return "1 = 0", nil, nil
	}}
	resp, err := e.List(user, map[string]any{"query": map[string]any{"mine": "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(t, resp); !slices.Equal(got, []string{"c1", "c2"}) {
		t.Fatalf("meine: %v", got)
	}
}
