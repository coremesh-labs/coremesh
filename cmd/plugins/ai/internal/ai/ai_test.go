package ai

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

func mustContain(t *testing.T, s string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(s, p) {
			t.Errorf("fehlt: %q", p)
		}
	}
}

// Objekt-Anfrage: Schema und leere Struktur aus dem Metamodell, feste Werte
// und kleiner Katalog als Auswahl, großer Katalog als Verweis; Antwort geprüft;
// Protokoll; Maske, Zurücklesen, Anlegen über saveAggregate.
func TestObjectRequest(t *testing.T) {
	e := setup(t)
	e.partnerRequest()
	e.ai.answer = "```json\n" + `{"partner":{"type":"ORGANIZATION","name1":"Stadtwerke Musterstadt GmbH","parent_id":"Stadtwerke Holding","unbekannt":"x"},
		"addresses":[{"role":"MAIN","address":{"street":"Hauptstr. 5","zip":"12345","city":"Musterstadt"}},{"role":"HOME","address":{"city":"X"}}],
		"contacts":[{"value":"0123 4567"}]}` + "\n```"
	out := e.must(serviceObject, "run", map[string]any{"request": "BP", "input": base64.StdEncoding.EncodeToString([]byte("Stadtwerke Musterstadt GmbH, Hauptstr. 5")),
		"content_type": "text/plain", "context": []any{"Kontext: Eingangsrechnung"}})
	if out["status"] != stDone || out["log_id"] != "KI-2026-000001" {
		t.Fatalf("run: %v", out)
	}
	res := out["result"].(map[string]any)
	if p := res["partner"].(map[string]any); p["name1"] != "Stadtwerke Musterstadt GmbH" || p["unbekannt"] != nil || p["type"] != "ORGANIZATION" {
		t.Errorf("Partner: %v", p)
	}
	addr := res["addresses"].([]any)
	if a := addr[1].(map[string]any); a["role"] != nil {
		t.Errorf("unbekannte Rolle übernommen: %v", a)
	}
	mustContain(t, fmt.Sprint(out["message"]), `addresses[1].role: Wert "HOME" steht nicht zur Auswahl`)

	// gesendete Anfrage
	body := e.ai.bodies[0]
	if body["model"] != "m1" || e.ai.auth[0] != "Bearer geheim-123|ja|https://ai.example/v1/chat/completions" {
		t.Errorf("Anfrage: %v %v", body["model"], e.ai.auth[0])
	}
	msgs, _ := json.Marshal(body["messages"])
	mustContain(t, string(msgs), "nichts erfinden", "Ermittle den Absender.", "Firmen sind ORGANIZATION.", "Kontext: Eingangsrechnung",
		`Leere Struktur`, `\"name1\": \"\"`, `\"address\": {`, "partner.type (Art) kann diese Werte annehmen (Schlüssel = Beschreibung): PERSON = Person; ORGANIZATION = Organisation",
		"addresses.role (Adressrolle) kann diese Werte annehmen (Schlüssel = Beschreibung): MAIN = Hauptanschrift; INVOICE = Rechnungsanschrift",
		"partner.parent_id (Konzern): Name bzw. Bezeichnung angeben", "Eingang:\\nStadtwerke Musterstadt GmbH")
	rf, _ := json.Marshal(body["response_format"])
	mustContain(t, string(rf), `"type":"json_schema"`, `"enum":["MAIN","INVOICE"]`, `"addresses":{"description":"Partner-Adresse","items"`)
	if strings.Contains(string(msgs), "valid_to") || strings.Contains(string(msgs), "bp_id") || strings.Contains(string(msgs), "is_default") ||
		strings.Contains(string(msgs), `"address_id"`) {
		t.Errorf("ausgeschlossene Felder in der Anfrage:\n%s", msgs)
	}

	// Protokoll
	log := e.must(logObject, "get", map[string]any{"id": "KI-2026-000001"})
	if log["status"] != stDone || log["tokens"] != 42.0 || !strings.Contains(fmt.Sprint(log["result"]), "Stadtwerke") ||
		!strings.Contains(fmt.Sprint(log["context"]), "Eingangsrechnung") || log["expires_on"] == "" {
		t.Errorf("Protokoll: %v", log)
	}

	// Maske: Werte aus dem Ergebnis, Vorgaben gehen vor, je Liste eine leere Zeile mehr
	form := e.must(serviceObject, "form", map[string]any{"request": "BP", "result": res, "defaults": map[string]any{"partner.name2": "Vorgabe", "contacts.is_default": "true"}})
	vals := form["values"].(map[string]any)
	if vals["f__partner__name1"] != "Stadtwerke Musterstadt GmbH" || vals["f__partner__name2"] != "Vorgabe" ||
		vals["f__addresses__0__address__street"] != "Hauptstr. 5" || vals["f__addresses__0__role"] != "MAIN" {
		t.Errorf("Werte: %v", vals)
	}
	keys := fmt.Sprint(form["fields"])
	mustContain(t, keys, "f__addresses__2__address__city", "f__contacts__1__value", "Partner-Adresse (weitere)", "Hauptanschrift")
	if strings.Contains(keys, "f__contacts__0__is_default") {
		t.Errorf("ausgeschlossenes Feld in der Maske")
	}

	// Zurücklesen: leere Zeilen entfallen
	vals["f__addresses__0__address__city"] = "Neustadt"
	vals["f__contacts__1__value"] = "info@sw.example"
	col := e.must(serviceObject, "collect", map[string]any{"request": "BP", "values": vals})["result"].(map[string]any)
	if len(col["addresses"].([]any)) != 2 || len(col["contacts"].([]any)) != 2 ||
		col["addresses"].([]any)[0].(map[string]any)["address"].(map[string]any)["city"] != "Neustadt" {
		t.Errorf("collect: %v", col)
	}

	// Anlegen: Adressdaten zuerst, dann Partner mit Relationen
	cr := e.must(serviceObject, "create", map[string]any{"request": "BP", "result": col, "defaults": map[string]any{"partner.type": "PERSON"}})
	if cr["id"] != "100001" || cr["object"] != "TPartner" {
		t.Fatalf("create: %v", cr)
	}
	if n := len(e.h.find("TAddress", "create")); n != 2 {
		t.Errorf("Adressen angelegt: %d", n)
	}
	agg := e.h.find("TPartner", "saveAggregate")[0]
	b, _ := json.Marshal(agg)
	mustContain(t, string(b), `"type":"ORGANIZATION"`, `"adressen":{"create":[{"address_id":"A`, `"role":"MAIN"`, `"kommunikation":{"create":[{"value":"0123 4567"}`)
	if strings.Contains(string(b), `"address":`) {
		t.Errorf("eingebettetes Objekt nicht ersetzt: %s", b)
	}
}

// Text-Anfrage, Fehler des Dienstes, Schlüssel nie lesbar, Umgebungsvariable, Rechte, Wiederholen.
func TestTextRequestAndProvider(t *testing.T) {
	e := setup(t)
	e.partnerRequest()
	e.must(requestObject, "create", map[string]any{"data": map[string]any{"code": "SUMMARY", "name": "Zusammenfassung", "kind": kindText,
		"task_text": "Fasse zusammen.", "retention_days": 2}})
	e.ai.answer = "Kurz: Rechnung über 100 EUR."
	out := e.must(serviceObject, "run", map[string]any{"request": "SUMMARY", "text": "Langer Text"})
	if out["text"] != "Kurz: Rechnung über 100 EUR." || out["status"] != stDone {
		t.Fatalf("Text: %v", out)
	}
	if e.ai.bodies[0]["response_format"] != nil {
		t.Error("Text-Anfrage mit Schema")
	}
	// Schlüssel: nie im Datensatz, in der Liste oder im Protokoll
	p := e.must(providerObject, "get", map[string]any{"id": "TEST"})
	if p["api_key"] != nil || p["has_key"] != true {
		t.Errorf("Schlüssel sichtbar: %v", p)
	}
	l, _ := json.Marshal(e.must(providerObject, "list", map[string]any{"query": map[string]any{}}))
	if strings.Contains(string(l), "geheim") {
		t.Errorf("Schlüssel in der Liste")
	}
	// Ändern ohne Schlüssel lässt ihn stehen; Umgebungsvariable geht vor
	e.must(providerObject, "update", map[string]any{"id": "TEST", "data": map[string]any{"code": "TEST", "name": "Testdienst", "base_url": "https://ai.example/v1",
		"model": "m1", "auth_method": "BEARER", "api_key_env": "TEST_AI_KEY"}})
	e.must(serviceObject, "run", map[string]any{"request": "SUMMARY", "text": "x"})
	if got := e.ai.auth[len(e.ai.auth)-1]; !strings.HasPrefix(got, "Bearer env-key|") {
		t.Errorf("Umgebungsvariable: %s", got)
	}
	e.must(providerObject, "update", map[string]any{"id": "TEST", "data": map[string]any{"code": "TEST", "name": "Testdienst", "base_url": "https://ai.example/v1",
		"model": "m1", "auth_method": "HEADER", "auth_name": "x-goog-api-key", "api_key_env": ""}})
	if k := e.m.text(e.ctx, `SELECT api_key FROM ai__provider_secret WHERE provider = ?`, "TEST"); k != "geheim-123" {
		t.Errorf("Schlüssel nach Änderung: %q", k)
	}
	// Überlast: wiederholt
	retryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	e.ai.failFirst = 2
	n := len(e.ai.bodies)
	if out := e.must(serviceObject, "run", map[string]any{"request": "SUMMARY", "text": "x"}); out["status"] != stDone || len(e.ai.bodies)-n != 3 {
		t.Errorf("Wiederholung: %v, %d Versuche", out["status"], len(e.ai.bodies)-n)
	}
	// Fehler des Dienstes: Status ERROR, kein Abbruch, Schlüssel nicht in der Meldung
	e.ai.status = 401
	out = e.must(serviceObject, "run", map[string]any{"request": "SUMMARY", "text": "x"})
	if out["status"] != stError || !strings.Contains(fmt.Sprint(out["message"]), "HTTP 401 – Schlüssel ungültig") {
		t.Errorf("Fehler: %v", out)
	}
	e.ai.status = 0
	// Wiederholen mit der verbesserten Anfrage
	e.must(requestObject, "update", map[string]any{"id": "SUMMARY", "data": map[string]any{"code": "SUMMARY", "name": "Zusammenfassung", "kind": kindText,
		"task_text": "Fasse in einem Satz zusammen.", "retention_days": 2}})
	rep := e.must(logObject, "repeat", map[string]any{"id": "KI-2026-000001"})
	mustContain(t, fmt.Sprint(rep["message"]), "KI-2026-000005")
	msgs, _ := json.Marshal(e.ai.bodies[len(e.ai.bodies)-1]["messages"])
	mustContain(t, string(msgs), "Fasse in einem Satz zusammen.", "Langer Text")
	// Rechte
	e.h.deny[requestObject+".run"] = true
	if _, err := e.call(serviceObject, "run", map[string]any{"request": "SUMMARY", "text": "x"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Errorf("ohne Recht: %v", err)
	}
}

// Verfall: Protokolle nach den Tagen der Anfrage gelöscht, außer „Nicht löschen“.
func TestLogRetention(t *testing.T) {
	e := setup(t)
	e.partnerRequest()
	e.ai.answer = `{"partner":{"name1":"A"}}`
	for i := 0; i < 2; i++ {
		e.must(serviceObject, "run", map[string]any{"request": "BP", "text": "x", "keep": i == 1})
	}
	if _, err := e.m.db.Exec(e.ctx, `UPDATE ai__log SET expires_on = ?`, "2000-01-01"); err != nil {
		t.Fatal(err)
	}
	e.m.lastClean = e.m.now().AddDate(0, 0, -1)
	l := e.must(logObject, "list", map[string]any{"query": map[string]any{}})
	if items := l["items"].([]any); len(items) != 1 || items[0].(map[string]any)["keep"] != true {
		t.Errorf("nach dem Verfall: %v", l["items"])
	}
}

// Ausweich-Dienst: Ist der gewählte Dienst gestört, antwortet der Ausweich-Dienst mit seinem Modell.
func TestFallbackProvider(t *testing.T) {
	e := setup(t)
	e.partnerRequest()
	retryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	e.must(providerObject, "create", map[string]any{"data": map[string]any{"code": "ZBACKUP", "name": "Ersatz", "base_url": "https://backup.example/v1",
		"model": "m2", "auth_method": "BEARER", "api_key": "k2"}})
	e.must(providerObject, "update", map[string]any{"id": "TEST", "data": map[string]any{"code": "TEST", "name": "Testdienst", "base_url": "https://ai.example/v1",
		"model": "m1", "auth_method": "BEARER", "fallback_provider": "ZBACKUP"}})
	if _, err := e.call(providerObject, "update", map[string]any{"id": "TEST", "data": map[string]any{"code": "TEST", "name": "Testdienst",
		"base_url": "https://ai.example/v1", "model": "m1", "auth_method": "BEARER", "fallback_provider": "TEST"}}); !errors.Is(err, sdk.ErrInvalidArgument) {
		t.Errorf("sich selbst als Ausweich: %v", err)
	}
	e.ai.answer = `{"partner":{"name1":"A"}}`
	e.ai.failFirst = 3 // erster Dienst: Versuch und zwei Wiederholungen gestört
	out := e.must(serviceObject, "run", map[string]any{"data": map[string]any{"request": "BP", "text": "x"}})
	if out["status"] != stDone || !strings.Contains(fmt.Sprint(out["message"]), "TEST gestört") {
		t.Fatalf("Ausweich: %v %v", out, e.ai.auth)
	}
	last := e.ai.auth[len(e.ai.auth)-1]
	if !strings.HasPrefix(last, "Bearer k2|") || e.ai.bodies[len(e.ai.bodies)-1]["model"] != "m2" {
		t.Errorf("Ausweich-Dienst nicht genutzt: %s %v", last, e.ai.bodies[len(e.ai.bodies)-1]["model"])
	}
	if l := e.must(logObject, "get", map[string]any{"id": fmt.Sprint(out["log_id"])}); l["provider"] != "ZBACKUP" || l["model"] != "m2" {
		t.Errorf("Protokoll: %v %v", l["provider"], l["model"])
	}
	// Fehler der Anfrage selbst (401): kein Ausweich
	e.ai.status = 401
	n := len(e.ai.bodies)
	if out := e.must(serviceObject, "run", map[string]any{"request": "BP", "text": "x"}); out["status"] != stError || len(e.ai.bodies)-n != 1 {
		t.Errorf("401: %v, %d Aufrufe", out["status"], len(e.ai.bodies)-n)
	}
}

// Text und Datei zusammen: Text im Auftrag, PDF als Anhang (file), Bild als image_url.
func TestTextWithFile(t *testing.T) {
	e := setup(t)
	e.partnerRequest()
	e.ai.answer = `{"partner":{"name1":"Sanitär Beaker"}}`
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 x"))
	e.must(serviceObject, "run", map[string]any{"request": "BP", "text": "Rechnung 250526", "input": pdf, "content_type": "application/pdf"})
	msgs, _ := json.Marshal(e.ai.bodies[0]["messages"])
	mustContain(t, string(msgs), `Eingang:\nRechnung 250526`, `"type":"image_url"`, `"url":"data:application/pdf;base64,`+pdf)
	e.must(serviceObject, "run", map[string]any{"request": "BP", "text": "Brief", "input": pdf, "content_type": "image/png"})
	msgs, _ = json.Marshal(e.ai.bodies[1]["messages"])
	mustContain(t, string(msgs), `"type":"image_url"`, `data:image/png;base64,`)
	// Dienst lehnt PDF als image_url ab: einmal als file
	e.ai.noPDFImage = true
	out := e.must(serviceObject, "run", map[string]any{"request": "BP", "text": "Rechnung", "input": pdf, "content_type": "application/pdf"})
	msgs, _ = json.Marshal(e.ai.bodies[len(e.ai.bodies)-1]["messages"])
	if out["status"] != stDone {
		t.Fatalf("Wiederholung als file: %v", out)
	}
	mustContain(t, string(msgs), `"type":"file"`, `"filename":"dokument.pdf"`, `"file_data":"data:application/pdf;base64,`+pdf)
}
