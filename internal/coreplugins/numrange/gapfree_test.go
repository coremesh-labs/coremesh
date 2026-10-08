package numrange

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/coremesh-labs/coremesh/internal/txctx"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// Lückenlos: Vergabe in der Transaktion des Aufrufers; ein Rollback gibt die
// Nummer zurück, ohne Transaktion wird nicht vergeben.
func TestGapFreeInCallerTransaction(t *testing.T) {
	p := setup(t)
	must(t, p, api.Object, api.ActionDefine, api.Definition{Object: "JournalEntry", Owner: "ledger", PerCompanyCode: true, PerYear: true,
		Width: 10, From: 1000000001, To: 1999999999, GapFree: true, Disjoint: true})
	req := api.Request{Object: "JournalEntry", CompanyCode: "1000", Key: "0L", Year: 2026}

	if _, err := do(p, api.Object, api.ActionNext, req); !errors.Is(err, sdk.ErrFailedPrecondition) {
		t.Fatalf("ohne Transaktion: %v", err)
	}
	inTx := func(commit bool) string {
		txID, err := p.db.BeginTx("ledger", "r1", "main", sql.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ctx := txctx.With(testCtx, "main", txID)
		resp, err := p.Handle(ctx, sdk.Request{Object: api.Object, Action: api.ActionNext, Payload: req})
		if err != nil {
			t.Fatal(err)
		}
		if commit {
			err = p.db.CommitTx("ledger", "r1", txID)
		} else {
			err = p.db.RollbackTx("r1", txID)
		}
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(api.Result).Number
	}
	if n := inTx(false); n != "1000000001" {
		t.Fatalf("erste Nummer: %s", n)
	}
	if n := inTx(true); n != "1000000001" {
		t.Fatalf("nach Rollback dieselbe Nummer erwartet: %s", n)
	}
	if n := inTx(true); n != "1000000002" {
		t.Fatalf("zweite Nummer: %s", n)
	}
}

// Überschneidungsfrei: Intervalle anderer Schlüssel im selben Buchungskreis
// und Jahr dürfen sich nicht überlappen – beim Pflegen und beim automatischen
// Anlegen aus den Standardwerten.
func TestDisjointIntervals(t *testing.T) {
	p := setup(t)
	must(t, p, api.Object, api.ActionDefine, api.Definition{Object: "Doc", Owner: "ledger", PerCompanyCode: true, PerYear: true,
		Width: 10, From: 1000000001, To: 1999999999, Disjoint: true})
	if n := next(t, p, api.Request{Object: "Doc", CompanyCode: "1000", Key: "0L", Year: 2026}); n.Number != "1000000001" {
		t.Fatalf("0L: %v", n)
	}
	// 2L aus den Standardwerten überschnitte 0L
	if _, err := do(p, api.Object, api.ActionNext, api.Request{Object: "Doc", CompanyCode: "1000", Key: "2L", Year: 2026}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("automatisch angelegt mit Überschneidung: %v", err)
	}
	create := func(from, to any) error {
		_, err := do(p, api.Object, "create", map[string]any{"data": map[string]any{"object": "Doc", "company_code": "1000", "range_key": "2L",
			"year": 2026, "from_number": from, "to_number": to}})
		return err
	}
	if err := create("1500000000", "2500000000"); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Fatalf("Pflege mit Überschneidung: %v", err)
	}
	// Zahlen aus JSON (float64), wie sie über gRPC ankommen
	if err := create(float64(2000000001), float64(2999999999)); err != nil {
		t.Fatalf("überschneidungsfrei: %v", err)
	}
	if n := next(t, p, api.Request{Object: "Doc", CompanyCode: "1000", Key: "2L", Year: 2026}); n.Number != "2000000001" {
		t.Fatalf("2L: %v", n)
	}
	// Anderes Jahr oder anderer Buchungskreis: unabhängig
	if n := next(t, p, api.Request{Object: "Doc", CompanyCode: "2000", Key: "2L", Year: 2026}); n.Number != "1000000001" {
		t.Fatalf("anderer Buchungskreis: %v", n)
	}
}
