package ai

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

func envLookup(k string) string { return os.Getenv(k) }

var (
	codeRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,39}$`)
	nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	envRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,99}$`)

	authMethods = []metamodel.Option{{Value: "BEARER", Label: "Authorization: Bearer <Schlüssel>"}, {Value: "HEADER", Label: "eigener Header (Name unten)"},
		{Value: "QUERY", Label: "URL-Parameter (Name unten)"}, {Value: "NONE", Label: "ohne Anmeldung"}}
	requestKinds = []metamodel.Option{{Value: kindObject, Label: "Objekt-Anfrage (Strukturen füllen)"}, {Value: kindText, Label: "Text-Anfrage"}}
	logStatus    = []metamodel.Option{{Value: stDone, Label: "Fertig"}, {Value: stError, Label: "Fehler"}}

	refProvider = &crud.Ref{Table: "ai__provider", Column: "code", Label: "KI-Dienst", Object: providerObject, LabelFields: []string{"name"}, ActiveField: "is_active"}
	refRequest  = &crud.Ref{Table: "ai__request", Column: "code", Label: "Anfrage", Object: requestObject, LabelFields: []string{"name"}}
)

func checkCode(rec crud.Record, key, label string) error {
	v := strings.ToUpper(strings.TrimSpace(crud.Str(rec[key])))
	if !codeRe.MatchString(v) {
		return crud.Invalid("%s %q: Großbuchstaben, Ziffern, _ und - (höchstens 40 Zeichen)", label, v)
	}
	rec[key] = v
	return nil
}

// --- Dienste --------------------------------------------------------------------------

func (m *Module) provider() *crud.Entity {
	return &crud.Entity{
		Object: providerObject, Title: "KI-Dienste", Icon: "icon-cloud", Table: "ai__provider", Section: "Einstellungen",
		Keys: []string{"code"}, Order: "code", StatusField: "is_active", TitleField: "name",
		Fields: []crud.Field{
			{Key: "code", Label: "Schlüssel", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "base_url", Label: "URL der Schnittstelle (OpenAI-kompatibel, ohne /chat/completions)", Type: tText, Required: true, Listable: true},
			{Key: "model", Label: "Modell (z. B. gemini-flash-latest)", Type: tText, Required: true, Listable: true},
			{Key: "auth_method", Label: "Anmeldung", Type: tSel, Required: true, Options: authMethods},
			{Key: "auth_name", Label: "Name des Headers bzw. Parameters (bei eigenem Header / URL-Parameter)", Type: tText,
				ShowIf: &metamodel.Condition{Field: "auth_method", Values: []string{"HEADER", "QUERY"}}},
			{Key: "api_key", Label: "API-Schlüssel (nur schreiben; leer = unverändert)", Type: tPass, Virtual: true},
			{Key: "has_key", Label: "Schlüssel gespeichert", Type: tBool, Listable: true, ReadOnly: true, Virtual: true},
			{Key: "api_key_env", Label: "Umgebungsvariable mit dem Schlüssel (geht dem gespeicherten vor; in der Konfiguration des Plugins setzen)", Type: tText, Listable: true},
			{Key: "extra_headers", Label: "Weitere Header (je Zeile „Name: Wert“)", Type: tArea},
			{Key: "plain_json", Label: "Ohne JSON-Schema (für Dienste ohne response_format; die Struktur steht dann nur im Text)", Type: tBool},
			{Key: "timeout_seconds", Label: "Zeitlimit (Sekunden)", Type: tNum},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Validate: m.checkProvider,
		Decorate: func(ctx context.Context, rec crud.Record) error {
			rec["api_key"] = nil
			rec["has_key"] = m.text(ctx, `SELECT provider FROM ai__provider_secret WHERE provider = ?`, rec["code"]) != ""
			return nil
		},
		Actions: []crud.Action{
			{ActionConfig: metamodel.ActionConfig{Name: "test", Label: "Verbindung prüfen", Record: true, Confirm: "Kurze Testanfrage an den Dienst senden?"},
				Handle: m.testProviderAction},
		},
	}
}

func (m *Module) checkProvider(ctx context.Context, rec, _ crud.Record) error {
	if err := checkCode(rec, "code", "Schlüssel"); err != nil {
		return err
	}
	u, err := url.Parse(strings.TrimSpace(crud.Str(rec["base_url"])))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return crud.Invalid("URL %q: http(s)://… erwartet", crud.Str(rec["base_url"]))
	}
	rec["base_url"] = strings.TrimRight(u.String(), "/")
	rec["auth_method"] = strings.ToUpper(strings.TrimSpace(crud.Str(rec["auth_method"])))
	switch rec["auth_method"] {
	case "BEARER", "NONE":
		rec["auth_name"] = nil
	case "HEADER", "QUERY":
		if strings.TrimSpace(crud.Str(rec["auth_name"])) == "" {
			return crud.Invalid("Name des Headers bzw. Parameters fehlt")
		}
	default:
		return crud.Invalid("Anmeldung %q unbekannt", rec["auth_method"])
	}
	if e := strings.TrimSpace(crud.Str(rec["api_key_env"])); e != "" && !envRe.MatchString(e) {
		return crud.Invalid("Umgebungsvariable %q: Buchstaben, Ziffern, _", e)
	}
	rec["api_key_env"] = nilIfEmpty(crud.Str(rec["api_key_env"]))
	if _, err := parseHeaders(crud.Str(rec["extra_headers"])); err != nil {
		return err
	}
	if toInt(rec["timeout_seconds"]) <= 0 {
		rec["timeout_seconds"] = 90
	}
	// Schlüssel: eigene Tabelle (in derselben Transaktion), nie im Datensatz.
	if key := strings.TrimSpace(crud.Str(rec["api_key"])); key != "" {
		if _, err := m.db.Exec(ctx, `DELETE FROM ai__provider_secret WHERE provider = ?`, rec["code"]); err != nil {
			return err
		}
		if _, err := m.db.Exec(ctx, `INSERT INTO ai__provider_secret (provider, api_key, changed_at) VALUES (?, ?, ?)`,
			rec["code"], key, m.now().UTC().Format("2006-01-02T15:04:05Z")); err != nil {
			return err
		}
	}
	delete(rec, "api_key")
	return nil
}

// parseHeaders: „Name: Wert“ je Zeile.
func parseHeaders(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok || strings.TrimSpace(k) == "" || strings.ContainsAny(k, " \t") {
			return nil, crud.Invalid("Header %q: „Name: Wert“ erwartet", l)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// --- Anfragen -------------------------------------------------------------------------

func (m *Module) request() *crud.Entity {
	return &crud.Entity{
		Object: requestObject, Title: "Anfragen", Icon: "icon-message-square", Table: "ai__request", Section: "KI",
		Keys: []string{"code"}, Order: "code", StatusField: "is_active", TitleField: "name",
		Filters: []string{"kind", "provider", "is_active"}, Search: []string{"code", "name", "task_text"},
		Fields: []crud.Field{
			{Key: "code", Label: "Schlüssel", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "kind", Label: "Art", Type: tSel, Required: true, Listable: true, Options: requestKinds},
			{Key: "provider", Label: "KI-Dienst (leer = erster aktiver)", Type: tText, Listable: true, Ref: refProvider},
			{Key: "model", Label: "Modell (leer = das des Dienstes)", Type: tText},
			{Key: "task_text", Label: "Aufgabe", Type: tArea, Required: true},
			{Key: "extra_text", Label: "Weitere Textzeilen (Hinweise, Möglichkeiten aus dem Kontext)", Type: tArea},
			{Key: "system_text", Label: "Systemanweisung (leer = Vorgabe: nur JSON, nichts erfinden)", Type: tArea},
			{Key: "temperature", Label: "Temperatur (leer = Vorgabe des Dienstes, z. B. 0.2)", Type: tText},
			{Key: "retention_days", Label: "Protokoll aufbewahren (Tage)", Type: tNum, Listable: true},
			{Key: "max_values", Label: "Verweise mit höchstens so vielen Werten als Auswahl mitgeben", Type: tNum},
			{Key: "description", Label: "Beschreibung", Type: tArea},
			{Key: "sample_input", Label: "Beispieleingabe (für die Probeanfrage)", Type: tArea},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
			{Key: "try_input", Label: "Eingabe (Text)", Type: tArea, ActionOnly: true},
			{Key: "try_context", Label: "Zusatzzeilen (Kontext)", Type: tArea, ActionOnly: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "anfrage", Title: "Anfrage", Fields: []string{"code", "name", "kind", "provider", "model", "task_text", "extra_text", "system_text",
				"temperature", "retention_days", "max_values", "description", "is_active"}},
			{Key: "objekte", Title: "Objekte (Objekt-Anfrage)", Relation: &metamodel.Relation{Object: reqObjObject, ForeignKey: "request",
				Columns: []string{"line_no", "name", "object", "parent", "relation", "via_field", "multiple", "fields"}}},
			{Key: "beispiel", Title: "Beispieleingabe", Collapsed: true, Fields: []string{"sample_input"}},
			{Key: "protokoll", Title: "Protokoll", Collapsed: true, Relation: &metamodel.Relation{Object: logObject, ForeignKey: "request",
				Columns: []string{"log_id", "status", "message", "duration_ms", "created_at", "expires_on"}}},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			if err := checkCode(rec, "code", "Schlüssel"); err != nil {
				return err
			}
			rec["kind"] = strings.ToUpper(strings.TrimSpace(crud.Str(rec["kind"])))
			if rec["kind"] != kindObject && rec["kind"] != kindText {
				return crud.Invalid("Art %q: OBJECT oder TEXT", rec["kind"])
			}
			if toInt(rec["retention_days"]) <= 0 {
				rec["retention_days"] = 1
			}
			if toInt(rec["max_values"]) <= 0 {
				rec["max_values"] = 100
			}
			if t := strings.TrimSpace(crud.Str(rec["temperature"])); t != "" {
				if _, err := parseTemperature(t); err != nil {
					return err
				}
			}
			rec["temperature"] = nilIfEmpty(crud.Str(rec["temperature"]))
			return nil
		},
		Actions: []crud.Action{
			{ActionConfig: metamodel.ActionConfig{Name: "try", Label: "Probeanfrage …", Record: true, Fields: []string{"try_input", "try_context"}, FormState: true},
				Handle: m.tryAction},
			{ActionConfig: metamodel.ActionConfig{Name: "showSchema", Label: "Struktur anzeigen", Record: true, Confirm: "Leere Struktur, Werte und JSON-Schema der Anfrage anzeigen?"},
				Handle: m.showSchemaAction},
		},
		FormState: m.requestFormState,
		Authorization: &metamodel.Authorization{Actions: []metamodel.AuthAction{
			{Name: "run", Label: "KI-Anfragen ausführen (Ai.run, auch aus Workflows)"}}},
	}
}

func (m *Module) requestObject() *crud.Entity {
	return &crud.Entity{
		Object: reqObjObject, Title: "Objekte der Anfragen", Icon: "icon-box", Table: "ai__request_object", Section: "KI",
		Keys: []string{"request", "line_no"}, Order: "request, line_no", Filters: []string{"request", "object"},
		Fields: []crud.Field{
			{Key: "request", Label: "Anfrage", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refRequest},
			{Key: "line_no", Label: "Zeile", Type: tNum, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Name in der Struktur (z. B. partner, addresses)", Type: tText, Required: true, Listable: true},
			{Key: "object", Label: "Object (z. B. BusinessPartner)", Type: tText, Required: true, Listable: true},
			{Key: "parent", Label: "Übergeordnet (Name; leer = Wurzel)", Type: tText, Listable: true},
			{Key: "relation", Label: "Relation am übergeordneten Object (Abschnitt, z. B. adressen)", Type: tText, Listable: true},
			{Key: "via_field", Label: "oder Verweisfeld des übergeordneten Objects (eingebettet, z. B. address_id)", Type: tText, Listable: true},
			{Key: "multiple", Label: "Mehrere (Liste)", Type: tBool, Listable: true},
			{Key: "fields", Label: "Felder (Komma-Liste; leer = alle bearbeitbaren)", Type: tText, Listable: true},
			{Key: "exclude", Label: "Ohne Felder (Komma-Liste)", Type: tText},
			{Key: "hint", Label: "Hinweis an die KI", Type: tArea},
		},
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			n := strings.TrimSpace(crud.Str(rec["name"]))
			if !nameRe.MatchString(n) {
				return crud.Invalid("Name %q: Kleinbuchstaben, Ziffern, _ (beginnt mit einem Buchstaben)", n)
			}
			rec["name"] = n
			rec["parent"], rec["relation"], rec["via_field"] = nilIfEmpty(crud.Str(rec["parent"])), nilIfEmpty(crud.Str(rec["relation"])), nilIfEmpty(crud.Str(rec["via_field"]))
			if rec["parent"] != nil && (rec["relation"] == nil) == (rec["via_field"] == nil) {
				return crud.Invalid("Untergeordnetes Objekt: genau eines von Relation und Verweisfeld angeben")
			}
			if rec["parent"] == nil && (rec["relation"] != nil || rec["via_field"] != nil) {
				return crud.Invalid("Die Wurzel hat weder Relation noch Verweisfeld")
			}
			return nil
		},
	}
}

// --- Protokoll ------------------------------------------------------------------------

func (m *Module) logEntity() *crud.Entity {
	return &crud.Entity{
		Object: logObject, Title: "Anfrageprotokoll", Icon: "icon-list", Table: "ai__log", Section: "KI",
		Keys: []string{"log_id"}, Order: "created_at DESC, log_id DESC", NoCreate: true, TitleField: "log_id",
		Filters: []string{"request", "status", "company_code", "keep"}, Search: []string{"log_id", "request", "message"},
		Fields: []crud.Field{
			{Key: "log_id", Label: "Protokoll", Type: tText, Listable: true, ReadOnly: true},
			{Key: "request", Label: "Anfrage", Type: tText, Listable: true, ReadOnly: true, Ref: refRequest},
			{Key: "kind", Label: "Art", Type: tSel, Listable: true, ReadOnly: true, Options: requestKinds},
			{Key: "status", Label: "Status", Type: tSel, Listable: true, ReadOnly: true, Options: logStatus},
			{Key: "message", Label: "Meldung", Type: tText, Listable: true, ReadOnly: true},
			{Key: "company_code", Label: "Buchungskreis", Type: tText, ReadOnly: true},
			{Key: "provider", Label: "KI-Dienst", Type: tText, ReadOnly: true},
			{Key: "model", Label: "Modell", Type: tText, ReadOnly: true},
			{Key: "duration_ms", Label: "Dauer (ms)", Type: tNum, Listable: true, ReadOnly: true},
			{Key: "tokens", Label: "Token", Type: tNum, ReadOnly: true},
			{Key: "keep", Label: "Nicht löschen", Type: tBool, Listable: true},
			{Key: "created_at", Label: "Angelegt", Type: tText, Listable: true, ReadOnly: true},
			{Key: "created_by", Label: "Angelegt von", Type: tText, ReadOnly: true},
			{Key: "expires_on", Label: "Verfällt am", Type: tText, Listable: true, ReadOnly: true},
			{Key: "input_type", Label: "Eingang (Inhaltstyp)", Type: tText, ReadOnly: true},
			{Key: "context", Label: "Zusatzzeilen (Kontext)", Type: tArea, ReadOnly: true},
			{Key: "result", Label: "Ergebnis", Type: tArea, ReadOnly: true},
			{Key: "prompt", Label: "Gesendete Nachrichten", Type: tArea, ReadOnly: true},
			{Key: "response", Label: "Antwort des Dienstes", Type: tArea, ReadOnly: true},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "protokoll", Title: "Protokoll", Fields: []string{"log_id", "request", "kind", "status", "message", "company_code", "provider", "model",
				"duration_ms", "tokens", "keep", "created_at", "created_by", "expires_on", "input_type"}},
			{Key: "ergebnis", Title: "Ergebnis", Fields: []string{"result"}},
			{Key: "anfrage", Title: "Anfrage", Collapsed: true, Fields: []string{"context", "prompt"}},
			{Key: "antwort", Title: "Antwort", Collapsed: true, Fields: []string{"response"}},
		},
		Access:    &crud.Access{Records: true, CompanyCode: "company_code"},
		ListScope: func(ctx context.Context) (string, []any, bool, error) { m.cleanup(ctx); return "", nil, false, nil },
		Actions: []crud.Action{
			{ActionConfig: metamodel.ActionConfig{Name: "repeat", Label: "Wiederholen", Record: true,
				Confirm: "Anfrage mit derselben Eingabe, aber der aktuellen Fassung der Anfrage wiederholen?"}, Handle: m.repeatAction},
		},
		Routes: map[string]module.HandlerFunc{},
	}
}
