package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCommandArgs(t *testing.T) {
	p := params{}
	rest := commandArgs([]string{"--chart=SKR04", "--user", "admin", "--file", "./a.json", "--dry-run", "--out=x.json"}, p)
	if !reflect.DeepEqual(rest, []string{"--user", "admin", "--out=x.json"}) {
		t.Fatalf("CLI-Optionen: %v", rest)
	}
	if p["chart"] != "SKR04" || p["file"] != "./a.json" || p["dry-run"] != "true" {
		t.Fatalf("Parameter: %v", p)
	}
}

// TestConvertParams: Text bleibt Text (--company=2000), Zahl/Schalter/JSON nach Typ,
// nicht deklarierte wie bisher.
func TestConvertParams(t *testing.T) {
	def := &commandDef{Params: []commandParam{{Name: "company"}, {Name: "year", Type: "number"}, {Name: "dry", Type: "boolean"},
		{Name: "file", File: true}, {Name: "q", Type: "json"}}}
	p := params{"company": "2000", "year": "2026", "dry": "true", "file": "./1.json", "q": `{"a":1}`, "other": "12"}
	if err := convertParams(def, p); err != nil {
		t.Fatal(err)
	}
	want := params{"company": "2000", "year": 2026.0, "dry": true, "file": "./1.json", "q": map[string]any{"a": 1.0}, "other": 12.0}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("%v", p)
	}
	if err := convertParams(def, params{"year": "x"}); err == nil {
		t.Fatal("Zahl nicht geprüft")
	}
}

func TestReadParamFile(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "k.csv")
	os.WriteFile(csvPath, []byte("\xef\xbb\xbfaccount_number;name\n4400; Erlöse 19 %\n"), 0o644)
	v, err := readParamFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if rows := v.([]any); len(rows) != 1 || rows[0].(map[string]any)["name"] != "Erlöse 19 %" || rows[0].(map[string]any)["account_number"] != "4400" {
		t.Fatalf("CSV: %v", v)
	}
	jsonPath := filepath.Join(dir, "k.json")
	os.WriteFile(jsonPath, []byte(`{"accounts":[{"account_number":"1800"}]}`), 0o644)
	if v, err := readParamFile(jsonPath); err != nil || v.(map[string]any)["accounts"] == nil {
		t.Fatalf("JSON: %v %v", v, err)
	}
}
