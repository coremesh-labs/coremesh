package script

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func (e *env) report(data map[string]any, cols ...map[string]any) {
	e.t.Helper()
	e.must(reportObject, "create", map[string]any{"data": data})
	for i, c := range cols {
		c["report"], c["position"] = data["code"], (i+1)*10
		e.must(reportColObject, "create", map[string]any{"data": c})
	}
}

func rowsOf(t *testing.T, out map[string]any) []string {
	t.Helper()
	var l []string
	for _, r := range out["table"].(map[string]any)["rows"].([]any) {
		l = append(l, fmt.Sprint(r))
	}
	return l
}

func TestReportObjectGroupsTotals(t *testing.T) {
	e := setup(t)
	e.report(map[string]any{"code": "GRUPPEN", "name": "Je Gruppe", "source_kind": "OBJECT", "source_object": "TItem",
		"source_payload": `{"query": {"from": "{stichtag}", "text": "ab {stichtag}"}}`, "group_by": "group", "sort_by": "-amount", "totals": true},
		map[string]any{"field": "group", "label": "Gruppe"},
		map[string]any{"field": "amount", "label": "Betrag", "aggregate": "SUM", "format": "MONEY"},
		map[string]any{"field": "id", "label": "Anzahl", "aggregate": "COUNT", "format": "INTEGER"},
		map[string]any{"field": "date", "label": "Letztes", "aggregate": "MAX", "format": "DATE"})
	e.must(reportParamObject, "create", map[string]any{"data": map[string]any{"report": "GRUPPEN", "name": "stichtag", "label": "Stichtag", "type": "DATE"}})
	out, err := e.call(reportObject, "run", map[string]any{"id": "GRUPPEN", "data": map[string]any{"p_stichtag": "2026-06-30"}})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(rowsOf(t, out), "|")
	if got != "[A 3.000,00 2 31.03.2026]|[B 250,50 1 01.02.2026]|[Summe 3.250,50 3 31.03.2026]" {
		t.Fatalf("Zeilen: %s", got)
	}
	if q := e.h.find("TItem", "list"); len(q) != 1 || fmt.Sprint(q[0]["query"]) != "map[from:2026-06-30 text:ab 2026-06-30]" {
		t.Fatalf("Anfrage: %v", q)
	}
	d := out["download"].(map[string]any)
	csv, _ := base64.StdEncoding.DecodeString(d["content"].(string))
	if !strings.Contains(string(csv), "Gruppe;Betrag;Anzahl;Letztes\r\nA;3.000,00;2;31.03.2026") || d["file_name"] != "gruppen.csv" {
		t.Fatalf("CSV: %q", csv)
	}
	// Unbekannter Platzhalter
	e.must(reportObject, "update", map[string]any{"id": "GRUPPEN", "data": map[string]any{"source_payload": `{"x": "{fehlt}"}`}})
	if _, err := e.call(reportObject, "run", map[string]any{"id": "GRUPPEN", "data": map[string]any{}}); err == nil || !strings.Contains(err.Error(), "fehlt") {
		t.Fatalf("Platzhalter: %v", err)
	}
}

func TestReportStreamScriptTemplate(t *testing.T) {
	e := setup(t)
	// Datenstrom, Spalten aus dem Strom, Grenze
	e.report(map[string]any{"code": "STROM", "name": "Strom", "source_kind": "OBJECT", "source_object": "TItem", "stream": true, "row_limit": 2})
	out, err := e.call(reportObject, "run", map[string]any{"id": "STROM"})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(out["table"].(map[string]any)["columns"]) != "[id group amount date]" || len(rowsOf(t, out)) != 2 {
		t.Fatalf("Strom: %v", out["table"])
	}
	// Berechtigung des Benutzers für die Datenquelle
	e.h.deny["TItem.list"] = true
	if _, err := e.call(reportObject, "run", map[string]any{"id": "STROM"}); err == nil || !strings.Contains(err.Error(), "keine Berechtigung") {
		t.Fatalf("Berechtigung: %v", err)
	}
	delete(e.h.deny, "TItem.list")

	// Skript als Quelle, Vorlage als Ausgabe
	e.script("ZEILEN", `return [{konto: "4100", saldo: params.faktor * 100}, {konto: "4200", saldo: params.faktor * -40}];`, nil)
	e.must(paramObject, "create", map[string]any{"data": map[string]any{"script": "ZEILEN", "name": "faktor", "label": "Faktor", "type": "NUMBER", "default_value": "2"}})
	e.report(map[string]any{"code": "SKRIPT", "name": "Aus Skript", "source_kind": "SCRIPT", "source_script": "ZEILEN", "totals": true, "template": "BERICHT"},
		map[string]any{"field": "konto"}, map[string]any{"field": "saldo", "aggregate": "SUM", "format": "MONEY"})
	e.must(reportParamObject, "create", map[string]any{"data": map[string]any{"report": "SKRIPT", "name": "faktor", "label": "Faktor", "type": "NUMBER", "default_value": "3"}})
	e.must(reportParamObject, "create", map[string]any{"data": map[string]any{"report": "SKRIPT", "name": "company_code", "label": "Buchungskreis", "type": "TEXT", "default_value": "2000"}})
	out, err = e.call(reportObject, "run", map[string]any{"id": "SKRIPT"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rowsOf(t, out), "|"); got != "[4100 300,00]|[4200 -120,00]|[Summe 180,00]" {
		t.Fatalf("Skript: %s", got)
	}
	r := e.h.find("Template", "render")
	if len(r) != 1 || r[0]["company_code"] != "2000" || r[0]["template"] != "BERICHT" {
		t.Fatalf("Vorlage: %v", r)
	}
	data := r[0]["data"].(map[string]any)
	if fmt.Sprint(data["totals"]) != "map[konto:Summe saldo:180,00]" || len(data["rows"].([]any)) != 2 {
		t.Fatalf("Daten der Vorlage: %v", data)
	}
	if out["download"].(map[string]any)["file_name"] != "bericht.pdf" {
		t.Fatalf("Datei: %v", out["download"])
	}
}

func TestExportImport(t *testing.T) {
	e := setup(t)
	e.script("EINS", `return 1;`, map[string]any{"description": "erstes"})
	e.must(paramObject, "create", map[string]any{"data": map[string]any{"script": "EINS", "name": "jahr", "label": "Jahr", "type": "NUMBER"}})
	e.must(bindingObject, "create", map[string]any{"data": map[string]any{"script": "EINS", "kind": trigHook, "hook": "ledger.posting", "phase": "check"}})
	e.report(map[string]any{"code": "LISTE", "name": "Liste", "source_kind": "OBJECT", "source_object": "TItem"},
		map[string]any{"field": "id"}, map[string]any{"field": "amount", "format": "MONEY"})
	out := e.must(scriptObject, "export", map[string]any{"data": map[string]any{}})
	file := out["file_content"].(string)
	if !strings.Contains(file, "format: coremesh.scripts") || !strings.Contains(file, "hook: ledger.posting") || out["message"] != "1 Skripte, 1 Berichte" {
		t.Fatalf("Export: %v", out)
	}

	// Prüflauf (Rücknahme der Transaktion leistet der Host; der Test-Host kennt keine)
	dry, err := setup(t).call(scriptObject, "import", map[string]any{"data": map[string]any{"file": file, "dry_run": true}})
	if err != nil || !strings.Contains(fmt.Sprint(dry["message"]), "nichts gespeichert") {
		t.Fatalf("Prüflauf: %v %v", dry, err)
	}
	f := setup(t)
	// Upload als Base64 (Datei der Maske) und zweimal: unverändert wiederholbar
	for i := 0; i < 2; i++ {
		res, err := f.call(scriptObject, "import", map[string]any{"data": map[string]any{"file": base64.StdEncoding.EncodeToString([]byte(file))}})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && fmt.Sprint(res["created"]) != "6" {
			t.Fatalf("Import: %v", res)
		}
	}
	if b := f.must(bindingObject, "list", map[string]any{})["items"].([]any); len(b) != 1 {
		t.Fatalf("Bindungen doppelt: %v", b)
	}
	if s := f.must(scriptObject, "get", map[string]any{"id": "EINS"}); s["description"] != "erstes" {
		t.Fatalf("Skript: %v", s)
	}
	again := f.must(scriptObject, "export", map[string]any{"data": map[string]any{}})["file_content"].(string)
	if again != file {
		t.Fatalf("Rundreise:\n%s\n---\n%s", file, again)
	}
	if _, err := f.call(scriptObject, "import", map[string]any{"file": "format: anders\nversion: 1"}); err == nil {
		t.Fatal("falsches Format angenommen")
	}
}
