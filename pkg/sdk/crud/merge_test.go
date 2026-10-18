package crud

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Zeitscheiben zusammenfassen: nur lückenlos anschließend und mit gleichen Werten;
// die Aktion steht nur an solchen Zeitscheiben.
func TestMergeSlice(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE cond (contract TEXT, kind TEXT, object TEXT, valid_from TEXT, valid_to TEXT, amount TEXT)`); err != nil {
		t.Fatal(err)
	}
	e := &Entity{Object: "Cond", Title: "Kondition", Table: "cond", Keys: []string{"contract", "kind", "object", "valid_from"}, TimeSlice: true,
		Order: "contract, kind, valid_from",
		Fields: WithTimeSlice(
			Field{Key: "contract", Label: "Vertrag", Type: metamodel.TypeText, Required: true},
			Field{Key: "kind", Label: "Art", Type: metamodel.TypeText, Required: true},
			Field{Key: "object", Label: "Objekt", Type: metamodel.TypeText},
			Field{Key: "amount", Label: "Betrag", Type: metamodel.TypeText},
		)}
	NewSet(e).Bind(sqlDB{db})
	ctx := sdk.WithHost(context.Background(), grantHost{})
	for _, r := range [][3]string{{"2018-01-01", "2022-12-31", "480"}, {"2023-01-01", "2026-03-31", "480"}, {"2026-04-01", "9999-12-31", "480"},
		{"2015-01-01", "2017-12-31", "400"}} {
		if _, err := e.Create(ctx, map[string]any{"contract": "V1", "kind": "KM", "amount": r[2], "valid_from": r[0], "valid_to": r[1]}); err != nil {
			t.Fatal(err)
		}
	}
	offered := func() []string {
		resp, err := e.List(ctx, map[string]any{"query": map[string]any{"includeHistory": "true"}})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range resp.Payload.(map[string]any)["items"].([]any) {
			r := it.(Record)
			if h, _ := r["_hidden_actions"].([]any); !slices.Contains(h, any(MergeAction)) {
				out = append(out, Str(r["valid_from"]))
			}
		}
		return out
	}
	if got := offered(); !slices.Equal(got, []string{"2023-01-01", "2026-04-01"}) {
		t.Fatalf("angeboten: %v", got) // nicht 2018 (anderer Betrag davor), nicht 2015 (erste)
	}
	_, err = e.MergeSlice(ctx, map[string]any{"id": "V1|KM||2018-01-01"})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("anderer Betrag: %v", err)
	}
	for _, id := range []string{"V1|KM||2026-04-01", "V1|KM||2023-01-01"} {
		if _, err := e.MergeSlice(ctx, map[string]any{"id": id}); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	var to string
	_ = db.QueryRow(`SELECT COUNT(*), MAX(valid_to) FROM cond WHERE valid_from >= '2018-01-01'`).Scan(&n, &to)
	if n != 1 || to != "9999-12-31" {
		t.Fatalf("zusammengefasst: %d Zeilen, bis %s", n, to)
	}
	if got := offered(); len(got) != 0 {
		t.Fatalf("danach angeboten: %v", got)
	}
}
