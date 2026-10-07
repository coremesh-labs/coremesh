package sdk

import (
	"fmt"
	"testing"
)

func TestValueRange(t *testing.T) {
	cases := []struct {
		r    ValueRange
		v    string
		want bool
	}{
		{ValueRange{Low: "SA"}, "SA", true},
		{ValueRange{Low: "SA"}, "KR", false},
		{ValueRange{Low: "*"}, "egal", true},
		{ValueRange{Low: "4*"}, "4400", true},
		{ValueRange{Low: "4*"}, "6000", false},
		{ValueRange{Low: "1", High: "12"}, "9", true}, // numerisch, nicht "9" > "12"
		{ValueRange{Low: "1", High: "12"}, "13", false},
		{ValueRange{Low: "A", High: "C"}, "B9", true},
		{ValueRange{Low: "A", High: "C"}, "D", false},
	}
	for _, c := range cases {
		if got := c.r.Matches(c.v); got != c.want {
			t.Errorf("%s matches %q = %v", c.r, c.v, got)
		}
	}
}

func TestGrantSet(t *testing.T) {
	g := GrantSet{Rules: []GrantRule{
		{CompanyCodes: []string{"1000"}, Fields: map[string][]ValueRange{"posting_period": {{Low: "1", High: "12"}}}},
		{CompanyCodes: []string{"*"}, Fields: map[string][]ValueRange{"posting_period": {{Low: "13"}}, "ledger": {{Low: "0L"}}}},
	}}
	for _, c := range []struct {
		name  string
		attrs Attrs
		want  bool
	}{
		{"Regel 1", Attrs{"company_code": "1000", "posting_period": "5"}, true},
		{"Regel 1 nur in 1000", Attrs{"company_code": "2000", "posting_period": "5"}, false},
		{"Regel 2 überall", Attrs{"company_code": "2000", "posting_period": "13", "ledger": "0L"}, true},
		{"Ledger passt nicht", Attrs{"company_code": "2000", "posting_period": "13", "ledger": "2L"}, false},
		{"ledger fehlt (fail closed)", Attrs{"company_code": "1000", "posting_period": "13"}, false},
	} {
		if got := g.Allows(c.attrs); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}

	where, args := g.SQL(map[string]string{"company_code": "cc", "posting_period": "period", "ledger": "ledger"})
	if want := "((cc IN (?) AND (period BETWEEN ? AND ?)) OR ((ledger = ?) AND (period = ?)))"; where != want {
		t.Fatalf("SQL: %s", where)
	}
	if fmt.Sprint(args) != "[1000 1 12 0L 13]" {
		t.Fatalf("args: %v", args)
	}
	// Feld ohne Spalte: Regel entfällt (fail closed).
	if where, _ := g.SQL(map[string]string{"company_code": "cc", "posting_period": "period"}); where != "((cc IN (?) AND (period BETWEEN ? AND ?)))" {
		t.Fatalf("ohne ledger: %s", where)
	}
	if where, _ := (GrantSet{}).SQL(nil); where != "1=0" {
		t.Fatal(where)
	}
	if where, _ := (GrantSet{Rules: []GrantRule{{CompanyCodes: []string{"*"}}}}).SQL(nil); where != "1=1" {
		t.Fatal(where)
	}
}
