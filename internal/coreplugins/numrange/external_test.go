package numrange

import (
	"errors"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// Externe Vergabe: Intervall entscheidet, ob die Nummer vergeben (intern)
// oder eingegeben und geprüft wird (extern: Muster oder von–bis).
func TestExternalAssignment(t *testing.T) {
	p := setup(t)
	must(t, p, api.Object, api.ActionDefine, api.Definition{Object: "BP", Owner: "partner", Width: 6, From: 100000, To: 199999})
	assign := func(key, value string) (api.Result, error) {
		resp, err := do(p, api.Object, api.ActionAssign, api.AssignRequest{Request: api.Request{Object: "BP", Key: key}, Value: value})
		if err != nil {
			return api.Result{}, err
		}
		return resp.Payload.(api.Result), nil
	}
	info := func(key string) api.Info {
		return must(t, p, api.Object, api.ActionInfo, api.Request{Object: "BP", Key: key}).Payload.(api.Info)
	}

	// Intern (Standard): nächste Nummer, eingegebene abgelehnt
	if i := info("STD"); i.External || i.Exists {
		t.Fatalf("Info vor Anlage: %+v", i)
	}
	if r, err := assign("STD", ""); err != nil || r.Number != "100000" || r.External {
		t.Fatalf("intern: %+v %v", r, err)
	}
	if _, err := assign("STD", "100500"); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("intern mit Wert: %v", err)
	}

	// Extern mit Muster (sprechende Schlüssel)
	must(t, p, api.Object, "create", map[string]any{"data": map[string]any{"object": "BP", "range_key": "EXT", "from_number": 1, "to_number": 1,
		"pattern": "{N}", "external": true, "external_pattern": "[A-Z][A-Z0-9-]{2,11}"}})
	if i := info("EXT"); !i.External || !i.Exists || i.ExternalPattern == "" {
		t.Fatalf("Info extern: %+v", i)
	}
	if r, err := assign("EXT", " mueller-a "); err != nil || r.Number != "MUELLER-A" || !r.External {
		t.Fatalf("extern: %+v %v", r, err)
	}
	for _, bad := range []string{"", "M", "1ABC", "MÜLLER"} {
		if _, err := assign("EXT", bad); !errors.Is(err, sdk.ErrInvalidArgument) {
			t.Fatalf("extern %q: %v", bad, err)
		}
	}
	if _, err := do(p, api.Object, api.ActionNext, api.Request{Object: "BP", Key: "EXT"}); !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("Next bei externem Intervall: %v", err)
	}

	// Extern ohne Muster: Zahl im Bereich, formatiert
	must(t, p, api.Object, "create", map[string]any{"data": map[string]any{"object": "BP", "range_key": "NUM", "from_number": 500000,
		"to_number": 599999, "width": 6, "pattern": "{N}", "external": true}})
	if r, err := assign("NUM", "512345"); err != nil || r.Number != "512345" || r.Value != 512345 {
		t.Fatalf("extern Zahl: %+v %v", r, err)
	}
	if _, err := assign("NUM", "612345"); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("extern außerhalb: %v", err)
	}

	// Muster nur bei externer Vergabe, und gültig
	_, err := do(p, api.Object, "create", map[string]any{"data": map[string]any{"object": "BP", "range_key": "X1", "from_number": 1, "to_number": 9,
		"pattern": "{N}", "external_pattern": "[A-Z]+"}})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Muster ohne extern: %v", err)
	}
	_, err = do(p, api.Object, "create", map[string]any{"data": map[string]any{"object": "BP", "range_key": "X2", "from_number": 1, "to_number": 9,
		"pattern": "{N}", "external": true, "external_pattern": "[A-Z"}})
	if !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("ungültiges Muster: %v", err)
	}
}
