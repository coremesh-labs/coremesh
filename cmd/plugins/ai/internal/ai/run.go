package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// Schnittstelle für andere Module:
//
//	Ai.run     {request, input (Base64), content_type?, text?, context? (Zeilen), company_code?, keep?}
//	           (text und eine Datei als input zusammen: Text im Auftrag, Datei als Anhang)
//	           → {log_id, status, message, result (OBJECT: Struktur), text (TEXT), notes}
//	Ai.form    {request, result, defaults? {"name.feld": Wert}, company_code?} → {fields [FieldDefinition], values}
//	Ai.collect {request, values {Feld der Maske: Wert}, company_code?} → {result}
//	Ai.create  {request, result, defaults?, company_code?} → {object, id, record}
//	Ai.schema  {request, company_code?} → {schema, empty, values}

const defaultSystem = "Du unterstützt eine Hausverwaltung beim Erfassen von Dokumenten. Antworte ausschließlich mit einem JSON-Objekt " +
	"nach dem vorgegebenen Schema. Fülle nur, was aus dem Eingang hervorgeht – nichts erfinden; Unbekanntes leer lassen. " +
	"Datumsangaben als JJJJ-MM-TT, Beträge als Zahl mit Punkt als Dezimaltrenner, ohne Währungszeichen."

type runRequest struct {
	Request     string `json:"request"`
	Input       string `json:"input"`
	ContentType string `json:"content_type"`
	Text        string `json:"text"`
	Context     any    `json:"context"`
	CompanyCode string `json:"company_code"`
	Keep        bool   `json:"keep"`
}

type runResult struct {
	LogID   string         `json:"log_id"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Result  map[string]any `json:"result,omitempty"`
	Text    string         `json:"text,omitempty"`
	Notes   []string       `json:"notes,omitempty"`
}

type requestRow struct {
	Code, Name, Kind, Provider, Model, System, Task, Extra, Temperature string
	Retention                                                           int
	Active                                                              bool
}

func (m *Module) loadRequest(ctx context.Context, code string) (*requestRow, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, kind, provider, model, system_text, task_text, extra_text, temperature, retention_days, is_active
		FROM ai__request WHERE code = ?`, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: KI-Anfrage %s", sdk.ErrNotFound, code)
	}
	r := res.Rows[0]
	return &requestRow{Code: crud.Str(r[0]), Name: crud.Str(r[1]), Kind: crud.Str(r[2]), Provider: crud.Str(r[3]), Model: crud.Str(r[4]),
		System: crud.Str(r[5]), Task: crud.Str(r[6]), Extra: crud.Str(r[7]), Temperature: crud.Str(r[8]), Retention: int(toInt(r[9])), Active: crud.AsBool(r[10])}, nil
}

// contextLines: Zusatzzeilen als Text oder Liste.
func contextLines(v any) []string {
	var out []string
	add := func(s string) {
		for _, l := range strings.Split(s, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
	}
	switch x := v.(type) {
	case string:
		add(x)
	case []any:
		for _, e := range x {
			add(fmt.Sprint(e))
		}
	case []string:
		for _, e := range x {
			add(e)
		}
	}
	return out
}

// run führt die Anfrage aus und schreibt das Protokoll. Ein Fehler des Dienstes
// ist kein Fehler des Aufrufs: Status ERROR mit Meldung.
func (m *Module) run(ctx context.Context, in runRequest) (*runResult, error) {
	rq, err := m.loadRequest(ctx, in.Request)
	if err != nil {
		return nil, err
	}
	if !rq.Active {
		return nil, crud.Invalid("KI-Anfrage %s ist inaktiv", rq.Code)
	}
	if ok, err := sdk.CheckAccess(ctx, requestObject, "run", in.CompanyCode); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("%w: KI-Anfrage ausführen (AiRequest.run) nicht erlaubt", sdk.ErrPermissionDenied)
	}
	m.cleanup(ctx)
	// Eingang
	// Text und Datei zusammen: der Text im Auftrag, die Datei (PDF, Bild) als Anhang –
	// z. B. wenn der Briefkopf nur im Bild steht.
	text, ctype := strings.TrimSpace(in.Text), "text/plain"
	var file []byte
	if in.Input != "" {
		b, err := base64.StdEncoding.DecodeString(in.Input)
		if err != nil {
			return nil, crud.Invalid("Eingang: nicht Base64-kodiert")
		}
		ctype = firstOf(in.ContentType, "text/plain")
		if strings.HasPrefix(ctype, "text/") || ctype == "application/json" {
			if !utf8.Valid(b) {
				return nil, crud.Invalid("Eingang: Text ist kein UTF-8")
			}
			if text == "" {
				text = string(b)
			}
			ctype = "text/plain"
		} else {
			file = b
		}
	}
	lines := contextLines(in.Context)

	// Nachrichten
	var b strings.Builder
	b.WriteString(rq.Task)
	if rq.Extra != "" {
		b.WriteString("\n\n" + rq.Extra)
	}
	if len(lines) > 0 {
		b.WriteString("\n\n" + strings.Join(lines, "\n"))
	}
	var root *sNode
	var schema map[string]any
	if rq.Kind == kindObject {
		root, err = m.structure(ctx, rq.Code, in.CompanyCode)
		if err != nil {
			return nil, err
		}
		schema = jsonSchema(root)
		empty, _ := json.MarshalIndent(emptyStructure(root), "", "  ")
		b.WriteString("\n\nLeere Struktur, die ausgefüllt zurückkommen soll:\n" + string(empty))
		if vl := valueLines(root); len(vl) > 0 {
			b.WriteString("\n\nFeste Werte:\n" + strings.Join(vl, "\n"))
		}
	}
	if text != "" {
		b.WriteString("\n\nEingang:\n" + text)
	}
	system := rq.System
	if system == "" && rq.Kind == kindObject {
		system = defaultSystem
	}
	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	user := chatMessage{Role: "user", Content: b.String()}
	if file != nil {
		user.Content = []any{map[string]any{"type": "text", "text": b.String()}, filePart(ctype, file)}
	}
	msgs = append(msgs, user)

	out := &runResult{}
	logInput, logType := text, "text/plain"
	if file != nil {
		logInput, logType = base64.StdEncoding.EncodeToString(file), ctype
	}
	p, perr := m.loadProvider(ctx, rq.Provider)
	var answer *chatAnswer
	start := m.now()
	model := ""
	if perr == nil {
		model = firstOf(rq.Model, p.Model)
		req := chatRequest{Model: model, Messages: msgs}
		if rq.Temperature != "" {
			if t, err := parseTemperature(rq.Temperature); err == nil {
				req.Temperature = &t
			}
		}
		if schema != nil && p.Structured {
			req.ResponseFormat = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "result", "schema": schema}}
		}
		var used *providerRow
		var fbNotes []string
		answer, used, fbNotes, err = m.chatFallback(ctx, p, req)
		if used != nil && used != p {
			p, model = used, used.Model
		}
		out.Notes = fbNotes
	} else {
		err = perr
	}
	dur := m.now().Sub(start).Milliseconds()
	switch {
	case err != nil:
		out.Status, out.Message = stError, err.Error()
	case rq.Kind == kindObject:
		raw, jerr := extractJSON(answer.Text)
		if jerr != nil {
			out.Status, out.Message = stError, jerr.Error()
			break
		}
		var notes []string
		out.Result, notes = sanitize(root, raw)
		out.Notes = append(out.Notes, notes...)
		out.Status, out.Message = stDone, "Struktur gefüllt"
		if len(out.Notes) > 0 {
			out.Message += " – " + strings.Join(out.Notes, "; ")
		}
	default:
		out.Status, out.Text, out.Message = stDone, strings.TrimSpace(answer.Text), "Antwort erhalten"
		if len(out.Notes) > 0 {
			out.Message += " – " + strings.Join(out.Notes, "; ")
		}
	}
	// Protokoll
	nr, err := numrange.Next(ctx, m.services, numrange.Request{Object: logObject, Reference: rq.Code})
	if err != nil {
		return nil, fmt.Errorf("Protokollnummer: %w", err)
	}
	out.LogID = nr.Number
	prompt, _ := json.MarshalIndent(promptForLog(msgs), "", "  ")
	var result string
	if out.Result != nil {
		r, _ := json.MarshalIndent(out.Result, "", "  ")
		result = string(r)
	} else {
		result = out.Text
	}
	var response string
	var tokens int64
	if answer != nil {
		response, tokens = answer.Raw, answer.Tokens
	}
	days := max(rq.Retention, 1)
	expires := m.now().AddDate(0, 0, days).Format(time.DateOnly)
	providerCode := ""
	if p != nil {
		providerCode = p.Code
	}
	if _, err := m.db.Exec(ctx, `INSERT INTO ai__log (log_id, request, kind, company_code, provider, model, status, message, input, input_type, context, prompt,
		response, result, duration_ms, tokens, keep, created_at, created_by, expires_on) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		out.LogID, rq.Code, rq.Kind, nilIfEmpty(in.CompanyCode), nilIfEmpty(providerCode), nilIfEmpty(model), out.Status, out.Message,
		nilIfEmpty(logInput), logType, nilIfEmpty(strings.Join(lines, "\n")), string(prompt), nilIfEmpty(response), nilIfEmpty(result),
		dur, tokens, in.Keep, m.now().UTC().Format(time.RFC3339), nilIfEmpty(sdk.CallFromContext(ctx).UserID), expires); err != nil {
		return nil, err
	}
	return out, nil
}

// promptForLog: Nachrichten ohne Dateiinhalt.
func promptForLog(msgs []chatMessage) []chatMessage {
	out := make([]chatMessage, len(msgs))
	for i, mm := range msgs {
		out[i] = mm
		if parts, ok := mm.Content.([]any); ok {
			var cp []any
			for _, p := range parts {
				if pm, ok := p.(map[string]any); ok && pm["type"] == "image_url" {
					cp = append(cp, map[string]any{"type": "image_url", "image_url": "[Datei]"})
					continue
				}
				cp = append(cp, p)
			}
			out[i].Content = cp
		}
	}
	return out
}

// cleanup löscht abgelaufene Protokolle (ohne „Nicht löschen“) – höchstens alle 10 Minuten.
func (m *Module) cleanup(ctx context.Context) {
	m.cleanMu.Lock()
	if !m.lastClean.IsZero() && m.now().Sub(m.lastClean) < 10*time.Minute {
		m.cleanMu.Unlock()
		return
	}
	m.lastClean = m.now()
	m.cleanMu.Unlock()
	res, err := m.db.Exec(ctx, `DELETE FROM ai__log WHERE keep = ? AND expires_on < ?`, false, m.now().Format(time.DateOnly))
	if err != nil {
		m.log.WarnContext(ctx, "Protokolle nicht gelöscht", "err", err.Error())
		return
	}
	if res.RowsAffected > 0 {
		m.log.InfoContext(ctx, "Abgelaufene KI-Protokolle gelöscht", "anzahl", res.RowsAffected)
	}
}

// --- Actions ---------------------------------------------------------------------------

// flat: Felder der Schnittstelle auch unter "data" (Konsole, Oberfläche).
func flat(payload any) map[string]any {
	var p map[string]any
	_ = sdk.Decode(payload, &p)
	if d, ok := p["data"].(map[string]any); ok {
		for k, v := range p {
			if k != "data" {
				d[k] = v
			}
		}
		return d
	}
	return p
}

func (m *Module) runAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in runRequest
	if err := sdk.Decode(flat(req.Payload), &in); err != nil {
		return sdk.Response{}, err
	}
	out, err := m.run(ctx, in)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: out}, nil
}

type structIn struct {
	Request     string            `json:"request"`
	CompanyCode string            `json:"company_code"`
	Result      map[string]any    `json:"result"`
	Defaults    map[string]string `json:"defaults"`
	Values      map[string]any    `json:"values"`
}

func (m *Module) structIn(ctx context.Context, req sdk.Request) (*sNode, structIn, error) {
	var in structIn
	if err := sdk.Decode(flat(req.Payload), &in); err != nil {
		return nil, in, err
	}
	rq, err := m.loadRequest(ctx, in.Request)
	if err != nil {
		return nil, in, err
	}
	if rq.Kind != kindObject {
		return nil, in, crud.Invalid("KI-Anfrage %s ist keine Objekt-Anfrage", rq.Code)
	}
	root, err := m.structure(ctx, rq.Code, in.CompanyCode)
	return root, in, err
}

func (m *Module) formAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	root, in, err := m.structIn(ctx, req)
	if err != nil {
		return sdk.Response{}, err
	}
	fields, values := formFields(root, in.Result, in.Defaults)
	return sdk.Response{Payload: map[string]any{"fields": fields, "values": values}}, nil
}

func (m *Module) collectAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	root, in, err := m.structIn(ctx, req)
	if err != nil {
		return sdk.Response{}, err
	}
	values := map[string]string{}
	for k, v := range in.Values {
		if v != nil {
			values[k] = valueText(v)
			if b, ok := v.(bool); ok && b {
				values[k] = "true"
			}
		}
	}
	return sdk.Response{Payload: map[string]any{"result": collect(root, values)}}, nil
}

func (m *Module) createAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	root, in, err := m.structIn(ctx, req)
	if err != nil {
		return sdk.Response{}, err
	}
	result, _ := sanitize(root, in.Result)
	applyDefaults(root, result, in.Defaults)
	rows := rowsOf(result[root.Name])
	if len(rows) != 1 {
		return sdk.Response{}, crud.Invalid("%s: genau ein Datensatz erwartet", root.Title)
	}
	rec := rows[0]
	if err := m.createEmbedded(ctx, root, rec); err != nil {
		return sdk.Response{}, err
	}
	relations := map[string]any{}
	for _, c := range root.Children {
		var creates []any
		for _, r := range rowsOf(result[c.Name]) {
			if err := m.createEmbedded(ctx, c, r); err != nil {
				return sdk.Response{}, err
			}
			delete(r, c.FK)
			creates = append(creates, r)
		}
		if len(creates) > 0 {
			relations[c.Relation] = map[string]any{"create": creates}
		}
	}
	resp, err := m.services.Call(ctx, root.Object, "saveAggregate", map[string]any{"data": rec, "relations": relations})
	if err != nil {
		return sdk.Response{}, err
	}
	var out struct {
		Record map[string]any `json:"record"`
	}
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{"object": root.Object, "id": crud.Str(out.Record["id"]), "record": out.Record}}, nil
}

// createEmbedded legt die eingebetteten Objekte an und setzt ihre ID ins Verweisfeld.
func (m *Module) createEmbedded(ctx context.Context, n *sNode, rec map[string]any) error {
	for _, e := range n.Embedded {
		sub := rowsOf(rec[e.Name])
		delete(rec, e.Name)
		if len(sub) == 0 || len(sub[0]) == 0 {
			continue
		}
		if err := m.createEmbedded(ctx, e, sub[0]); err != nil {
			return err
		}
		resp, err := m.services.Call(ctx, e.Object, "create", map[string]any{"data": sub[0]})
		if err != nil {
			return fmt.Errorf("%s: %w", e.Title, err)
		}
		var created map[string]any
		if err := sdk.Decode(resp.Payload, &created); err != nil {
			return err
		}
		rec[e.Via] = crud.Str(created["id"])
	}
	return nil
}

func (m *Module) schemaAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	root, _, err := m.structIn(ctx, req)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{"schema": jsonSchema(root), "empty": emptyStructure(root), "values": valueLines(root)}}, nil
}

// --- Oberfläche ------------------------------------------------------------------------

func (m *Module) idOf(req sdk.Request, object string) (string, map[string]any, error) {
	var in struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return "", nil, err
	}
	key, err := m.set.Entity(object).ParseID(in.ID)
	if err != nil {
		return "", nil, err
	}
	return crud.Str(key[m.set.Entity(object).Keys[0]]), in.Data, nil
}

func resultMessage(out *runResult) string {
	msg := fmt.Sprintf("Protokoll %s: %s – %s", out.LogID, out.Status, out.Message)
	if out.Result != nil {
		b, _ := json.MarshalIndent(out.Result, "", "  ")
		msg += "\n" + string(b)
	} else if out.Text != "" {
		msg += "\n" + out.Text
	}
	return msg
}

func (m *Module) tryAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	code, data, err := m.idOf(req, requestObject)
	if err != nil {
		return sdk.Response{}, err
	}
	out, err := m.run(ctx, runRequest{Request: code, Text: crud.Str(data["try_input"]), Context: crud.Str(data["try_context"])})
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{"message": resultMessage(out), "log_id": out.LogID}}, nil
}

func (m *Module) showSchemaAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	code, _, err := m.idOf(req, requestObject)
	if err != nil {
		return sdk.Response{}, err
	}
	root, err := m.structure(ctx, code, "")
	if err != nil {
		return sdk.Response{}, err
	}
	empty, _ := json.MarshalIndent(emptyStructure(root), "", "  ")
	schema, _ := json.MarshalIndent(jsonSchema(root), "", "  ")
	msg := "Leere Struktur:\n" + string(empty)
	if vl := valueLines(root); len(vl) > 0 {
		msg += "\n\nFeste Werte:\n" + strings.Join(vl, "\n")
	}
	return sdk.Response{Payload: map[string]any{"message": msg + "\n\nJSON-Schema:\n" + string(schema)}}, nil
}

func (m *Module) requestFormState(ctx context.Context, req metamodel.FormStateRequest) (metamodel.FormState, error) {
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if req.Mode == "action" && req.Action == "try" && strings.TrimSpace(req.Values["try_input"]) == "" {
		if key, err := m.set.Entity(requestObject).ParseID(req.ID); err == nil {
			if s := m.text(ctx, `SELECT sample_input FROM ai__request WHERE code = ?`, key["code"]); s != "" {
				st.Fields["try_input"] = metamodel.FieldState{Value: &s}
			}
		}
		st.Message = "Sendet die Anfrage mit diesem Eingang an den KI-Dienst; das Ergebnis steht im Protokoll."
	}
	return st, nil
}

func (m *Module) repeatAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	id, _, err := m.idOf(req, logObject)
	if err != nil {
		return sdk.Response{}, err
	}
	if _, err := m.set.Entity(logObject).Get(ctx, map[string]any{"id": id}); err != nil {
		return sdk.Response{}, err
	}
	res, err := m.db.Query(ctx, `SELECT request, input, input_type, context, company_code FROM ai__log WHERE log_id = ?`, id)
	if err != nil || len(res.Rows) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Protokoll %s", sdk.ErrNotFound, id)
	}
	r := res.Rows[0]
	in := runRequest{Request: crud.Str(r[0]), Context: crud.Str(r[3]), CompanyCode: crud.Str(r[4])}
	if t := crud.Str(r[2]); t == "text/plain" || t == "" {
		in.Text = crud.Str(r[1])
	} else {
		in.Input, in.ContentType = crud.Str(r[1]), t
	}
	out, err := m.run(ctx, in)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{"message": resultMessage(out), "log_id": out.LogID}}, nil
}

func (m *Module) testProviderAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	code, _, err := m.idOf(req, providerObject)
	if err != nil {
		return sdk.Response{}, err
	}
	if err := requireWrite(ctx, providerObject, "update"); err != nil {
		return sdk.Response{}, err
	}
	p, err := m.loadProvider(ctx, code)
	if err != nil {
		return sdk.Response{}, err
	}
	start := m.now()
	a, err := m.chat(ctx, p, chatRequest{Model: p.Model, Messages: []chatMessage{{Role: "user", Content: "Antworte nur mit: OK"}}})
	if err != nil {
		return sdk.Response{}, crud.Invalid("%v", err)
	}
	return sdk.Response{Payload: map[string]any{"message": fmt.Sprintf("%s antwortet (%s, %d ms): %s", p.Name, firstOf(a.Model, p.Model),
		m.now().Sub(start).Milliseconds(), strings.TrimSpace(a.Text))}}, nil
}

// filePart: Datei als Teil der Nachricht – Bilder als image_url, andere Dateien
// (z. B. PDF) als file mit file_data (OpenAI-kompatibel).
func filePart(ctype string, file []byte) map[string]any {
	data := "data:" + ctype + ";base64," + base64.StdEncoding.EncodeToString(file)
	if strings.HasPrefix(ctype, "image/") {
		return map[string]any{"type": "image_url", "image_url": map[string]any{"url": data}}
	}
	return map[string]any{"type": "file", "file": map[string]any{"filename": "dokument" + extFor(ctype), "file_data": data}}
}

func extFor(ctype string) string {
	switch ctype {
	case "application/pdf":
		return ".pdf"
	}
	return ""
}
