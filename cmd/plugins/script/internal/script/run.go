package script

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// --- Parameter und Maske ------------------------------------------------------------

// paramRow ist ein Parameter eines Skripts oder Berichts.
type paramRow struct {
	Name, Label, Type, Default, Options, LookupObject, LookupValue, LookupLabel string
	Required                                                                    bool
}

// paramPrefix: Felder der Parameter in der Maske (Extra), getrennt von den
// Feldern des Objects.
const paramPrefix = "p_"

func (m *Module) loadParams(ctx context.Context, table, owner, code string) ([]paramRow, error) {
	res, err := m.db.Query(ctx, `SELECT name, label, type, default_value, options, lookup_object, lookup_value, lookup_label, required
		FROM `+table+` WHERE `+owner+` = ? ORDER BY position, name`, code)
	if err != nil {
		return nil, err
	}
	out := make([]paramRow, 0, len(res.Rows))
	for _, r := range res.Rows {
		out = append(out, paramRow{Name: crud.Str(r[0]), Label: crud.Str(r[1]), Type: crud.Str(r[2]), Default: crud.Str(r[3]),
			Options: crud.Str(r[4]), LookupObject: crud.Str(r[5]), LookupValue: crud.Str(r[6]), LookupLabel: crud.Str(r[7]), Required: crud.AsBool(r[8])})
	}
	return out, nil
}

// paramForm: Felder der Parameter mit Vorbelegung (Datum ohne Vorbelegung = Stichtag).
func paramForm(ctx context.Context, params []paramRow, st *metamodel.FormState, values map[string]string) {
	for _, p := range params {
		f := metamodel.FieldDefinition{Key: paramPrefix + p.Name, Label: p.Label, Required: p.Required, Editable: true, Group: "Parameter"}
		switch p.Type {
		case "NUMBER":
			f.Type = metamodel.TypeNumber
		case "DATE":
			f.Type = metamodel.TypeDate
		case "BOOLEAN":
			f.Type = metamodel.TypeBoolean
		case "SELECT":
			f.Type, f.Options = metamodel.TypeSelect, parseOptions(p.Options)
		default:
			f.Type = metamodel.TypeText
			if p.LookupObject != "" {
				f.Lookup = &metamodel.Lookup{Object: p.LookupObject, ValueField: firstOf(p.LookupValue, "code"), LabelFields: splitList(p.LookupLabel)}
			}
		}
		st.Extra = append(st.Extra, f)
		if values[f.Key] != "" {
			continue
		}
		def := p.Default
		if def == "" && p.Type == "DATE" {
			def = crud.KeyDate(ctx)
		}
		if def != "" {
			st.Fields[f.Key] = metamodel.FieldState{Value: &def}
		}
	}
}

// paramValues: Werte aus der Maske (bzw. ohne Präfix aus der Konsole oder
// einem Skript), umgewandelt nach Typ und auf Pflicht geprüft.
func paramValues(ctx context.Context, params []paramRow, data map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, p := range params {
		raw, ok := data[paramPrefix+p.Name]
		if !ok {
			raw, ok = data[p.Name]
		}
		s := strings.TrimSpace(crud.Str(raw))
		if !ok || (s == "" && p.Type != "BOOLEAN") {
			s = p.Default
			if s == "" && p.Type == "DATE" {
				s = crud.KeyDate(ctx)
			}
		}
		if s == "" && p.Type != "BOOLEAN" {
			if p.Required {
				return nil, crud.Invalid("%s fehlt", p.Label)
			}
			out[p.Name] = nil
			continue
		}
		switch p.Type {
		case "NUMBER":
			f, ok := toFloat(s)
			if !ok {
				return nil, crud.Invalid("%s: Zahl erwartet, nicht %q", p.Label, s)
			}
			out[p.Name] = f
		case "DATE":
			d, err := crud.ParseDate(s)
			if err != nil {
				return nil, crud.Invalid("%s: %v", p.Label, err)
			}
			out[p.Name] = d
		case "BOOLEAN":
			out[p.Name] = crud.AsBool(firstOf(s, "false"))
		case "SELECT":
			valid := false
			for _, o := range parseOptions(p.Options) {
				valid = valid || o.Value == s
			}
			if !valid {
				return nil, crud.Invalid("%s: %q ist kein Auswahlwert", p.Label, s)
			}
			out[p.Name] = s
		default:
			out[p.Name] = s
		}
	}
	return out, nil
}

func (m *Module) scriptFormState(ctx context.Context, req metamodel.FormStateRequest) (metamodel.FormState, error) {
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	switch {
	case req.Mode == "create" && strings.TrimSpace(req.Values["allowed_calls"]) == "":
		v := defaultAllowedCalls
		st.Fields["allowed_calls"] = metamodel.FieldState{Value: &v}
	case req.Mode == "action" && req.Action == "run":
		key, err := m.set.Entity(scriptObject).ParseID(req.ID)
		if err != nil {
			return st, err
		}
		code := crud.Str(key["code"])
		params, err := m.loadParams(ctx, "script__parameter", "script", code)
		if err != nil {
			return st, err
		}
		paramForm(ctx, params, &st, req.Values)
		st.Message = firstOf(m.text(ctx, `SELECT description FROM script__script WHERE code = ?`, code), "Führt das Skript mit diesen Eingaben aus.")
	}
	return st, nil
}

// --- Ausführen über die Maske -------------------------------------------------------

func (m *Module) runAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	key, err := m.set.Entity(scriptObject).ParseID(in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	s, err := m.loadScript(ctx, crud.Str(key["code"]))
	if err != nil {
		return sdk.Response{}, err
	}
	if !s.Active {
		return sdk.Response{}, crud.Invalid("Skript %s ist inaktiv", s.Code)
	}
	defs, err := m.loadParams(ctx, "script__parameter", "script", s.Code)
	if err != nil {
		return sdk.Response{}, err
	}
	if in.Data == nil { // Konsole: Parameter direkt im Payload
		in.Data, _ = req.Payload.(map[string]any)
	}
	params, err := paramValues(ctx, defs, in.Data)
	if err != nil {
		return sdk.Response{}, err
	}
	res, err := m.execute(ctx, s, runInput{Trigger: trigRun, Params: params})
	runID := m.recordRun(ctx, runEntry{Script: s, Trigger: trigRun, Result: res, Err: err})
	if err != nil {
		return sdk.Response{}, crud.Invalid("Skript %s: %v", s.Code, err)
	}
	out := resultPayload(res.Value)
	if runID != "" {
		out["run_id"] = runID
	}
	if res.Log != "" {
		out["message"] = strings.TrimSpace(crud.Str(out["message"]) + "\n\n" + res.Log)
	}
	return sdk.Response{Payload: out}, nil
}

// resultPayload: Rückgabe des Skripts → Antwort einer Aktion (message, table, download).
func resultPayload(v any) map[string]any {
	switch x := v.(type) {
	case nil:
		return map[string]any{"message": "Fertig."}
	case string:
		return map[string]any{"message": x}
	case []any:
		return map[string]any{"message": fmt.Sprintf("%d Zeilen", len(x)), "table": makeTable(x, nil)}
	case map[string]any:
		_, hasMsg := x["message"]
		_, hasTable := x["table"]
		_, hasRows := x["rows"]
		_, hasDl := x["download"]
		if hasRows && !hasMsg {
			if l, ok := x["rows"].([]any); ok {
				x["message"] = fmt.Sprintf("%d Zeilen", len(l))
			}
		}
		if !hasMsg && !hasTable && !hasRows && !hasDl {
			b, _ := json.MarshalIndent(x, "", "  ")
			return map[string]any{"message": string(b)}
		}
		out := map[string]any{"message": crud.Str(x["message"])}
		if t, ok := x["table"].(map[string]any); ok {
			out["table"] = makeTable(t["rows"], t["columns"])
		} else if rows, ok := x["rows"].([]any); ok {
			out["table"] = makeTable(rows, x["columns"])
		}
		if d, ok := x["download"].(map[string]any); ok && crud.Str(d["content"]) != "" {
			out["download"] = metamodel.Content{Content: crud.Str(d["content"]), ContentType: firstOf(crud.Str(d["content_type"]), "application/octet-stream"),
				FileName: firstOf(crud.Str(d["file_name"]), "ergebnis")}
		}
		if out["message"] == "" {
			out["message"] = "Fertig."
		}
		return out
	}
	return map[string]any{"message": cell(v)}
}

// --- Events -------------------------------------------------------------------------

func (m *Module) onEvent(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	ev, err := events.Decode(req.Payload)
	if err != nil {
		return sdk.Response{}, err
	}
	res, err := m.db.Query(ctx, `SELECT b.id, b.script FROM script__binding b JOIN script__script s ON s.code = b.script
		WHERE b.kind = ? AND b.is_active = ? AND s.is_active = ?
		  AND (b.event_object = '*' OR b.event_object = ?) AND (b.event_action = '*' OR b.event_action = ?)
		  AND (b.company_code = '*' OR b.company_code = ?)
		ORDER BY b.priority, b.id`, trigEvent, true, true, ev.Object, ev.Action, ev.CompanyCode)
	if err != nil {
		return sdk.Response{}, err
	}
	detail := ev.Object + "." + ev.Action
	if ev.EntityID != "" {
		detail += " " + ev.EntityID
	}
	ran := 0
	for _, r := range res.Rows {
		s, err := m.loadScript(ctx, crud.Str(r[1]))
		if err != nil {
			continue
		}
		out, err := m.execute(ctx, s, runInput{Trigger: trigEvent, Detail: detail, Event: ev})
		m.recordRun(ctx, runEntry{Script: s, Trigger: trigEvent, Detail: detail, Result: out, Err: err})
		if err != nil {
			m.log.WarnContext(ctx, "Skript (Event) fehlgeschlagen", "script", s.Code, "event", detail, "err", err.Error())
		}
		ran++
	}
	return sdk.Response{Payload: map[string]any{"scripts": ran}}, nil
}

// --- Hooks --------------------------------------------------------------------------

func (m *Module) onHook(ctx context.Context, req hook.Request) (hook.Response, error) {
	res, err := m.db.Query(ctx, `SELECT b.id, b.script FROM script__binding b JOIN script__script s ON s.code = b.script
		WHERE b.kind = ? AND b.is_active = ? AND s.is_active = ? AND b.hook = ? AND b.phase = ?
		ORDER BY b.priority, b.id`, trigHook, true, true, req.Hook, req.Action)
	if err != nil {
		return hook.Response{}, err
	}
	data := req.Data
	changed := false
	var msgs []hook.Message
	detail := req.Hook + " (" + req.Action + ")"
	for _, r := range res.Rows {
		s, err := m.loadScript(ctx, crud.Str(r[1]))
		if err != nil {
			continue
		}
		out, err := m.execute(ctx, s, runInput{Trigger: trigHook, Detail: detail, Hook: &hookData{Name: req.Hook, Phase: req.Action, Data: data}})
		m.recordRun(ctx, runEntry{Script: s, Trigger: trigHook, Detail: detail, Result: out, Err: err})
		if err != nil {
			m.log.WarnContext(ctx, "Skript (Hook) fehlgeschlagen", "script", s.Code, "hook", detail, "err", err.Error())
			msgs = append(msgs, hook.Error("SCRIPT-001", fmt.Sprintf("Skript %s: %v", s.Code, err)))
			continue
		}
		for _, mm := range out.Hook.Messages {
			typ := strings.ToUpper(crud.Str(mm["type"]))
			if typ != hook.TypeError && typ != hook.TypeWarning {
				typ = hook.TypeSuccess
			}
			msgs = append(msgs, hook.Message{Type: typ, ID: firstOf(crud.Str(mm["id"]), "SCRIPT-"+s.Code), Text: crud.Str(mm["text"]),
				Field: crud.Str(mm["field"])})
		}
		if req.Action == hook.PhaseModify {
			data, changed = out.Hook.Data, true
		}
	}
	if !changed {
		return hook.Reply(nil, msgs...), nil
	}
	return hook.Reply(data, msgs...), nil
}

// --- Protokoll ----------------------------------------------------------------------

type runEntry struct {
	Script          *scriptRow
	Report          string
	Trigger, Detail string
	Result          *runResult
	Err             error
}

// recordRun schreibt das Protokoll (nach der Einstellung des Skripts) und
// liefert die Nummer des Laufs ("" = nicht protokolliert).
func (m *Module) recordRun(ctx context.Context, e runEntry) string {
	level, retention, script := "ALL", 30, ""
	if e.Script != nil {
		level, retention, script = firstOf(e.Script.LogRuns, "ALL"), orDefault(e.Script.RetentionDays, 30), e.Script.Code
	}
	if level == "NONE" || (level == "ERRORS" && e.Err == nil) {
		return ""
	}
	if e.Result == nil {
		e.Result = &runResult{}
	}
	status, msg := stDone, "Fertig"
	if e.Err != nil {
		status, msg = stError, e.Err.Error()
	} else if s, ok := e.Result.Value.(string); ok {
		msg = s
	} else if mm, ok := e.Result.Value.(map[string]any); ok && crud.Str(mm["message"]) != "" {
		msg = crud.Str(mm["message"])
	}
	var result any
	if e.Result.Value != nil {
		b, _ := json.MarshalIndent(e.Result.Value, "", "  ")
		result = clip(string(b), 200000)
	}
	now := m.now()
	id := now.UTC().Format("20060102-150405") + "-" + crud.NewID()[:6]
	_, err := m.db.Exec(ctx, `INSERT INTO script__run (run_id, script, report, trigger, detail, status, message, log, result, duration_ms, keep,
		created_at, created_by, expires_on) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, nilIfEmpty(script), nilIfEmpty(e.Report), e.Trigger, nilIfEmpty(e.Detail), status, clip(firstLine(msg), 500), nilIfEmpty(e.Result.Log), result,
		e.Result.Duration.Milliseconds(), false, now.Format("2006-01-02 15:04:05"), nilIfEmpty(sdk.CallFromContext(ctx).UserID),
		now.AddDate(0, 0, retention).Format("2006-01-02"))
	if err != nil {
		m.log.WarnContext(ctx, "Protokoll nicht geschrieben", "script", script, "err", err.Error())
		return ""
	}
	return id
}

// cleanup löscht abgelaufene Läufe (höchstens einmal je Stunde).
func (m *Module) cleanup(ctx context.Context) {
	m.cleanMu.Lock()
	if m.now().Sub(m.lastClean) < time.Hour {
		m.cleanMu.Unlock()
		return
	}
	m.lastClean = m.now()
	m.cleanMu.Unlock()
	if _, err := m.db.Exec(ctx, `DELETE FROM script__run WHERE expires_on < ? AND keep = ?`, m.now().Format("2006-01-02"), false); err != nil {
		m.log.WarnContext(ctx, "Protokoll nicht bereinigt", "err", err.Error())
	}
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + " …"
}
