package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestVerbArgs: Ziel und Muster sind Positionsargumente, CLI-Optionen gehen an
// flag, alles andere ist Parameter der Action (auch --param k=v).
func TestVerbArgs(t *testing.T) {
	p := params{}
	pos, flags := verbArgs([]string{"CompanyCode.get", "--id=2000", "--out", "x.json", "--format=json", "--param", "q=1", "--flag"}, p)
	if !reflect.DeepEqual(pos, []string{"CompanyCode.get"}) || !reflect.DeepEqual(flags, []string{"--out=x.json", "--format=json"}) {
		t.Fatalf("pos %v flags %v", pos, flags)
	}
	if p["id"] != "2000" || p["q"] != 1.0 || p["flag"] != "true" {
		t.Fatalf("Parameter: %v", p)
	}
	if pos, _ := verbArgs([]string{"hook", "contract.activate"}, params{}); len(pos) != 2 {
		t.Fatalf("details hook: %v", pos)
	}
}

func TestMatches(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"", "Contract", true}, {"contract*", "ContractLoan", true}, {"*loan", "ContractLoan", true},
		{"Bank", "BankAccount", true}, {"Bank?ccount", "BankAccount", true}, {"Loan*", "ContractLoan", false},
	} {
		if got := matches(c.pattern, c.name); got != c.want {
			t.Errorf("matches(%q, %q) = %v", c.pattern, c.name, got)
		}
	}
}

// TestFieldParams: Textfelder bleiben Text (--company_code=2000), Zahl und
// Schalter nach Feldtyp, id immer Text, data als JSON; Datei-Felder lokal gelesen.
func TestFieldParams(t *testing.T) {
	var def defResponse
	def.Definition.Name = "X"
	for _, f := range []struct{ key, typ string }{{"company_code", "text"}, {"amount", "number"}, {"active", "boolean"}, {"file", "file"}} {
		def.Definition.Fields = append(def.Definition.Fields, fieldDef{Key: f.key, Type: f.typ})
	}
	path := filepath.Join(t.TempDir(), "k.csv")
	if err := os.WriteFile(path, []byte("a;b\n1;2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := params{"company_code": "2000", "amount": "12,5", "active": "true", "id": "1000", "data": `{"a":1}`, "file": path, "other": "7"}
	if err := convertParams(fieldParams(&def), p); err != nil {
		t.Fatal(err)
	}
	if err := readFileFields(&def, p); err != nil {
		t.Fatal(err)
	}
	want := params{"company_code": "2000", "amount": 12.5, "active": true, "id": "1000", "data": map[string]any{"a": 1.0},
		"file": "a;b\n1;2\n", "file_name": "k.csv", "other": 7.0}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("%v", p)
	}
}
