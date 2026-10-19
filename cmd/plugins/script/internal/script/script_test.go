package script

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

func TestRunWithParametersAndTable(t *testing.T) {
	e := setup(t)
	e.script("SUMME", `
var items = AccessObject("TItem", "list", {query: {group: params.group}}).items;
log("Zeilen:", items.length);
var rows = items.filter(function(i) { return i.amount >= params.min; })
  .map(function(i) { return {id: i.id, betrag: formatNumber(i.amount), datum: formatDate(i.date)}; });
return {message: "Ergebnis " + rows.length, rows: rows, columns: ["id", "betrag", "datum"]};`, nil)
	e.must(paramObject, "create", map[string]any{"data": map[string]any{"script": "SUMME", "name": "group", "label": "Gruppe", "type": "TEXT", "default_value": "A"}})
	e.must(paramObject, "create", map[string]any{"data": map[string]any{"script": "SUMME", "name": "min", "label": "Mindestens", "type": "NUMBER", "required": true}})

	// Maske: Parameter als Zusatzfelder, Vorbelegung
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: scriptObject, Action: "formState", Payload: metamodel.FormStateRequest{Mode: "action", Action: "run", ID: "SUMME"}})
	if err != nil {
		t.Fatal(err)
	}
	st := resp.Payload.(metamodel.FormState)
	if len(st.Extra) != 2 || st.Extra[0].Key != "p_group" || st.Extra[1].Type != metamodel.TypeNumber || *st.Fields["p_group"].Value != "A" {
		t.Fatalf("Maske: %+v %+v", st.Extra, st.Fields)
	}
	if v := st.Fields["source"].Visible; v == nil || *v {
		t.Fatalf("Maske: Felder des Skripts sichtbar: %+v", st.Fields["source"])
	}
	if _, err := e.run("SUMME", map[string]any{}); err == nil || !strings.Contains(err.Error(), "Mindestens fehlt") {
		t.Fatalf("Pflicht: %v", err)
	}
	out, err := e.run("SUMME", map[string]any{"p_min": "1500"})
	if err != nil {
		t.Fatal(err)
	}
	table := out["table"].(map[string]any)
	rows := table["rows"].([]any)
	if len(rows) != 1 || fmt.Sprint(rows[0]) != "[3 2.000,00 31.03.2026]" || !strings.Contains(out["message"].(string), "Zeilen: 3") {
		t.Fatalf("Ergebnis: %v", out)
	}
	if q := e.h.find("TItem", "list"); len(q) != 1 || fmt.Sprint(q[0]["query"]) != "map[group:A]" {
		t.Fatalf("Aufruf: %v", q)
	}
	// Protokoll
	runs := e.must(runObject, "list", map[string]any{"query": map[string]any{"script": "SUMME"}})["items"].([]any)
	if len(runs) != 1 || runs[0].(map[string]any)["status"] != stDone {
		t.Fatalf("Protokoll: %v", runs)
	}
}

func TestColumnOrderFromScript(t *testing.T) {
	e := setup(t)
	e.script("ORDER", `return [{zeta: 1, alpha: 2, mitte: 3}];`, nil)
	out, err := e.run("ORDER", nil)
	if err != nil {
		t.Fatal(err)
	}
	cols := out["table"].(map[string]any)["columns"]
	if !strings.Contains(fmt.Sprint(cols), "zeta") || strings.Index(fmt.Sprint(cols), "zeta") > strings.Index(fmt.Sprint(cols), "alpha") {
		t.Fatalf("Spalten: %v", cols)
	}
}

func TestSyntaxErrorAndPatterns(t *testing.T) {
	e := setup(t)
	if _, err := e.call(scriptObject, "create", map[string]any{"data": map[string]any{"code": "KAPUTT", "name": "x", "source": "return (1 +;"}}); err == nil ||
		!strings.Contains(err.Error(), "Quelltext") {
		t.Fatalf("Syntaxfehler: %v", err)
	}
	if _, err := e.call(scriptObject, "create", map[string]any{"data": map[string]any{"code": "MUSTER", "name": "x", "source": "return 1;", "allowed_calls": "nurwort"}}); err == nil {
		t.Fatal("Muster ohne Punkt angenommen")
	}
	// Vorschlag der erlaubten Aufrufe beim Anlegen
	resp, err := e.p.Handle(e.ctx, sdk.Request{Object: scriptObject, Action: "formState", Payload: metamodel.FormStateRequest{Mode: "create"}})
	if err != nil {
		t.Fatal(err)
	}
	if v := resp.Payload.(metamodel.FormState).Fields["allowed_calls"].Value; v == nil || !strings.Contains(*v, "*.list") {
		t.Fatalf("Vorschlag: %v", v)
	}
}

func TestCallsRestricted(t *testing.T) {
	e := setup(t)
	// update ist nicht in den erlaubten Aufrufen
	e.script("SCHREIBT", `return AccessObject("TItem", "update", {id: "1", data: {amount: 0}});`, nil)
	if _, err := e.run("SCHREIBT", nil); err == nil || !strings.Contains(err.Error(), "darf TItem.update nicht aufrufen") {
		t.Fatalf("Muster: %v", err)
	}
	e.must(scriptObject, "update", map[string]any{"id": "SCHREIBT", "data": map[string]any{"allowed_calls": "TItem.*"}})
	if _, err := e.run("SCHREIBT", nil); err != nil {
		t.Fatalf("erlaubt: %v", err)
	}
	// Berechtigung des Benutzers
	e.h.deny["TItem.update"] = true
	if _, err := e.run("SCHREIBT", nil); err == nil || !strings.Contains(err.Error(), "keine Berechtigung für TItem.update") {
		t.Fatalf("Berechtigung: %v", err)
	}
	// Fehler im Skript abfangbar
	e.script("FAENGT", `try { AccessObject("TItem", "update", {}); } catch (x) { return "abgefangen: " + x.message; }`, nil)
	out, err := e.run("FAENGT", nil)
	if err != nil || !strings.Contains(out["message"].(string), "abgefangen") {
		t.Fatalf("try/catch: %v %v", out, err)
	}
}

func TestTimeout(t *testing.T) {
	e := setup(t)
	e.script("ENDLOS", `while (true) {}`, map[string]any{"timeout_sec": 1})
	_, err := e.run("ENDLOS", nil)
	if err == nil || !strings.Contains(err.Error(), "Zeitlimit") {
		t.Fatalf("Zeitlimit: %v", err)
	}
	runs := e.must(runObject, "list", map[string]any{"query": map[string]any{"script": "ENDLOS"}})["items"].([]any)
	if len(runs) != 1 || runs[0].(map[string]any)["status"] != stError {
		t.Fatalf("Protokoll: %v", runs)
	}
}

func TestReadObjectAndDownload(t *testing.T) {
	e := setup(t)
	e.script("STROM", `
var n = 0, sum = 0;
ReadObject("TItem", "list", {}, function(r) { n++; sum += r.amount; if (n == 2) return false; });
var all = ReadObject("TItem", "list", {}, 2);
return {message: n + "/" + all.length + "/" + sum, download: download("a;b", "x.csv", "text/csv")};`, nil)
	out, err := e.run("STROM", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["message"] != "2/2/1250.5" {
		t.Fatalf("Strom: %v", out["message"])
	}
	d := out["download"].(map[string]any)
	if b, _ := base64.StdEncoding.DecodeString(d["content"].(string)); string(b) != "a;b" || d["file_name"] != "x.csv" {
		t.Fatalf("Download: %v", d)
	}
}

func TestHookBindings(t *testing.T) {
	e := setup(t)
	e.script("PRUEF", `if (!hook.data.tax_code) hook.error("TAX-1", "Steuerkennzeichen fehlt", "tax_code"); hook.warning("W-1", "nur Hinweis");`, nil)
	e.script("AENDERT", `hook.data.note = "von " + context.script; return {data: hook.data};`, nil)
	e.must(bindingObject, "create", map[string]any{"data": map[string]any{"script": "PRUEF", "kind": trigHook, "hook": "ledger.posting", "phase": "check", "priority": 50}})
	e.must(bindingObject, "create", map[string]any{"data": map[string]any{"script": "AENDERT", "kind": trigHook, "hook": "ledger.posting", "phase": "modify"}})

	call := func(phase string, data map[string]any) hook.Response {
		resp, err := e.p.Handle(e.ctx, sdk.Request{Object: hookObject, Action: hook.CallbackAction, Payload: hook.Request{Hook: "ledger.posting", Action: phase, Data: data}})
		if err != nil {
			t.Fatal(err)
		}
		var out hook.Response
		if err := sdk.Decode(resp.Payload, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := call("check", map[string]any{"amount": 1})
	if len(out.Messages) != 2 || out.Messages[0].Type != "E" || out.Messages[0].Field != "tax_code" || out.Messages[1].Type != "W" || out.ReturnData != nil {
		t.Fatalf("check: %+v", out)
	}
	if out := call("check", map[string]any{"tax_code": "V19"}); len(out.Messages) != 1 {
		t.Fatalf("check ok: %+v", out)
	}
	out = call("modify", map[string]any{"amount": 1})
	if d, _ := out.ReturnData.(map[string]any); d["note"] != "von AENDERT" || fmt.Sprint(d["amount"]) != "1" {
		t.Fatalf("modify: %+v", out)
	}
	// Fehler im Skript → Meldung E
	e.must(scriptObject, "update", map[string]any{"id": "PRUEF", "data": map[string]any{"source": `throw new Error("kaputt");`}})
	if out := call("check", map[string]any{}); len(out.Messages) != 1 || out.Messages[0].Type != "E" || !strings.Contains(out.Messages[0].Text, "kaputt") {
		t.Fatalf("Fehler: %+v", out)
	}
	// Abos: je Hook und Phase eins, Priorität der Bindung
	if err := e.m.syncSubscriptions(e.ctx); err != nil {
		t.Fatal(err)
	}
	subs := e.h.find("Hook", "Subscribe")
	if len(subs) != 2 {
		t.Fatalf("Abos: %v", subs)
	}
	for _, s := range subs {
		if s["callback"] != hookObject || (s["phase"] == "check" && fmt.Sprint(s["priority"]) != "50") {
			t.Fatalf("Abo: %v", s)
		}
	}
	// inaktive Bindung läuft nicht
	ids := e.must(bindingObject, "list", map[string]any{"query": map[string]any{"script": "AENDERT"}})["items"].([]any)
	e.must(bindingObject, "deactivate", map[string]any{"id": ids[0].(map[string]any)["id"]})
	if out := call("modify", map[string]any{"amount": 1}); out.ReturnData != nil {
		t.Fatalf("inaktiv: %+v", out)
	}
}

func TestEventBinding(t *testing.T) {
	e := setup(t)
	e.script("FOLGE", `log(event.object, event.action, event.data.document_number, context.user_id === "");
var n = AccessObject("TItem", "list", {}).items.length; return "gelesen " + n;`, nil)
	e.must(bindingObject, "create", map[string]any{"data": map[string]any{"script": "FOLGE", "kind": trigEvent, "event_object": "JournalEntry", "event_action": "post"}})
	e.must(bindingObject, "create", map[string]any{"data": map[string]any{"script": "FOLGE", "kind": trigEvent, "event_object": "Contract", "company_code": "2000"}})
	if err := e.m.syncSubscriptions(e.ctx); err != nil {
		t.Fatal(err)
	}
	if subs := e.h.find("SystemEvent", "Register"); len(subs) != 2 {
		t.Fatalf("Abos: %v", subs)
	}
	ev := func(object, action, cc string) map[string]any {
		out, err := e.callIn(e.sys, eventObject, events.CallbackAction, map[string]any{"event": events.Event{Object: object, Action: action, CompanyCode: cc,
			Data: map[string]any{"document_number": "100"}}})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := ev("JournalEntry", "post", "1000"); fmt.Sprint(out["scripts"]) != "1" {
		t.Fatalf("Event: %v", out)
	}
	if out := ev("JournalEntry", "reverse", "1000"); fmt.Sprint(out["scripts"]) != "0" {
		t.Fatalf("andere Action: %v", out)
	}
	if out := ev("Contract", "update", "1000"); fmt.Sprint(out["scripts"]) != "0" {
		t.Fatalf("anderer Buchungskreis: %v", out)
	}
	runs := e.must(runObject, "list", map[string]any{"query": map[string]any{"script": "FOLGE"}})["items"].([]any)
	if len(runs) != 1 {
		t.Fatalf("Protokoll: %v", runs)
	}
	run := e.must(runObject, "get", map[string]any{"id": runs[0].(map[string]any)["run_id"]})
	if run["status"] != stDone || run["message"] != "gelesen 3" || !strings.Contains(fmt.Sprint(run["log"]), "JournalEntry post 100 true") {
		t.Fatalf("Lauf: %v", run)
	}
}

func TestLogLevels(t *testing.T) {
	e := setup(t)
	e.script("STILL", `return "ok";`, map[string]any{"log_runs": "ERRORS"})
	if _, err := e.run("STILL", nil); err != nil {
		t.Fatal(err)
	}
	if runs := e.must(runObject, "list", map[string]any{"query": map[string]any{"script": "STILL"}})["items"].([]any); len(runs) != 0 {
		t.Fatalf("nur Fehler: %v", runs)
	}
}
