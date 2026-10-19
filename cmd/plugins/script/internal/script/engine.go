package script

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Laufzeit eines Skripts (goja, ECMAScript 5.1 mit vielen Teilen von ES6+).
//
// Das Skript ist der Rumpf einer Funktion: return liefert das Ergebnis. Global
// stehen bereit:
//
//	params                          Werte der Parameter (Maske, Bericht)
//	event                           das SystemEvent (Bindung EVENT)
//	hook                            {name, phase, data, messages, error(id, text, field), warning(…), info(…)} (Bindung HOOK)
//	context                         {script, trigger, user_id, key_date}
//	AccessObject(object, action, payload)       Action eines Moduls → Antwort (Payload)
//	ReadObject(object, action, payload[, limit | fn])  Datenstrom → Zeilen als Objekte (fn: je Zeile, false bricht ab)
//	CallTemplate(template, data, {company_code, language, wait})  Vorlage → {message, job_number, download}
//	log(…)  keyDate()  table(rows[, columns])  download(text, file_name[, content_type])
//	formatNumber(v[, decimals])  formatDate(iso)
//
// Jeder Aufruf eines Objects muss zu den erlaubten Aufrufen des Skripts passen
// und – mit angemeldetem Benutzer – zu dessen Berechtigungen (der Dispatcher
// prüft verschachtelte Aufrufe nicht noch einmal).

// scriptRow ist ein Skript aus der Tabelle.
type scriptRow struct {
	Code, Name, Description, Source, AllowedCalls, LogRuns string
	TimeoutSec, RetentionDays                              int
	Active                                                 bool
}

func (m *Module) loadScript(ctx context.Context, code string) (*scriptRow, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, description, source, allowed_calls, log_runs, timeout_sec, retention_days, is_active
		FROM script__script WHERE code = ?`, code)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Skript %s", sdk.ErrNotFound, code)
	}
	r := res.Rows[0]
	return &scriptRow{Code: crud.Str(r[0]), Name: crud.Str(r[1]), Description: crud.Str(r[2]), Source: crud.Str(r[3]),
		AllowedCalls: crud.Str(r[4]), LogRuns: crud.Str(r[5]), TimeoutSec: int(toInt(r[6])), RetentionDays: int(toInt(r[7])),
		Active: crud.AsBool(r[8])}, nil
}

// compile übersetzt den Quelltext als Rumpf einer Funktion (return erlaubt).
// Die erste Zeile bleibt Zeile 1 der Fehlermeldungen.
func compile(name, src string) (*goja.Program, error) {
	return goja.Compile(name, "(function(){"+src+"\n})()", false)
}

// runInput: Auslöser und Eingaben eines Laufs.
type runInput struct {
	Trigger, Detail string
	Params          map[string]any
	Event           any       // EVENT
	Hook            *hookData // HOOK
	// Patterns ersetzen die erlaubten Aufrufe des Skripts (nur für Berichte ohne Skript).
	Patterns []string
}

type hookData struct {
	Name, Phase string
	Data        any
}

// runResult: Ergebnis und Ausgaben eines Laufs.
type runResult struct {
	Value    any // JSON-Wert von return
	Log      string
	Hook     *hookResult
	Duration time.Duration
}

type hookResult struct {
	Data     any
	Messages []map[string]any
}

// runner hält den Zustand eines Laufs.
type runner struct {
	m        *Module
	ctx      context.Context
	vm       *goja.Runtime
	script   *scriptRow
	patterns []string
	grants   map[string]bool
	calls    int
	log      strings.Builder
	cut      bool
	parse    goja.Callable
	strfy    goja.Callable
}

var errInterrupted = errors.New("abgebrochen")

// execute führt ein Skript aus.
func (m *Module) execute(ctx context.Context, s *scriptRow, in runInput) (*runResult, error) {
	start := m.now()
	prog, err := compile(s.Code, s.Source)
	if err != nil {
		return &runResult{}, fmt.Errorf("Quelltext: %v", err)
	}
	patterns := in.Patterns
	if patterns == nil {
		if patterns, err = parsePatterns(s.AllowedCalls); err != nil {
			return &runResult{}, err
		}
	}
	timeout := time.Duration(orDefault(s.TimeoutSec, 30)) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r := &runner{m: m, ctx: ctx, vm: goja.New(), script: s, patterns: patterns, grants: map[string]bool{}}
	r.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	jsonObj := r.vm.Get("JSON").ToObject(r.vm)
	r.parse, _ = goja.AssertFunction(jsonObj.Get("parse"))
	r.strfy, _ = goja.AssertFunction(jsonObj.Get("stringify"))
	if err := r.globals(in); err != nil {
		return &runResult{}, err
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			r.vm.Interrupt(errInterrupted)
		case <-stop:
		}
	}()
	v, err := r.vm.RunProgram(prog)
	out := &runResult{Log: r.log.String(), Duration: m.now().Sub(start)}
	if err != nil {
		return out, r.describe(err, timeout)
	}
	if out.Value, err = r.export(v); err != nil {
		return out, err
	}
	if in.Hook != nil {
		out.Hook, err = r.hookOutcome(out.Value)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// describe macht aus einem Laufzeitfehler eine lesbare Meldung mit Zeile.
func (r *runner) describe(err error, timeout time.Duration) error {
	var intr *goja.InterruptedError
	if errors.As(err, &intr) {
		if errors.Is(r.ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("Zeitlimit von %s überschritten", timeout)
		}
		return fmt.Errorf("Lauf abgebrochen")
	}
	var ex *goja.Exception
	if errors.As(err, &ex) {
		return errors.New(strings.TrimSpace(ex.Error()))
	}
	return err
}

// --- Werte zwischen Go und JavaScript (über JSON) -------------------------------------

func (r *runner) toJS(v any) goja.Value {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return goja.Null()
	}
	out, err := r.parse(goja.Undefined(), r.vm.ToValue(string(b)))
	if err != nil {
		panic(err)
	}
	return out
}

func (r *runner) export(v goja.Value) (any, error) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, nil
	}
	s, err := r.strfy(goja.Undefined(), v)
	if err != nil {
		return nil, fmt.Errorf("Ergebnis: %v", err)
	}
	if goja.IsUndefined(s) {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(s.String()), &out); err != nil {
		return nil, fmt.Errorf("Ergebnis: %v", err)
	}
	return out, nil
}

func (r *runner) arg(c goja.FunctionCall, i int) any {
	if i >= len(c.Arguments) {
		return nil
	}
	v, err := r.export(c.Arguments[i])
	if err != nil {
		r.throw(err)
	}
	return v
}

func (r *runner) throw(err error) {
	panic(r.vm.NewGoError(err))
}

// --- Globale Werte und Funktionen ---------------------------------------------------

func (r *runner) globals(in runInput) error {
	vm := r.vm
	call := sdk.CallFromContext(r.ctx)
	params := in.Params
	if params == nil {
		params = map[string]any{}
	}
	set := func(name string, v any) {
		if err := vm.Set(name, v); err != nil {
			panic(err)
		}
	}
	set("params", r.toJS(params))
	set("context", r.toJS(map[string]any{"script": r.script.Code, "trigger": in.Trigger, "detail": in.Detail,
		"user_id": call.UserID, "key_date": crud.KeyDate(r.ctx)}))
	if in.Event != nil {
		set("event", r.toJS(in.Event))
	}
	set("AccessObject", r.accessObject)
	set("ReadObject", r.readObject)
	set("CallTemplate", r.callTemplate)
	set("log", r.logFn)
	set("keyDate", func() string { return crud.KeyDate(r.ctx) })
	set("table", r.tableFn)
	set("download", r.downloadFn)
	set("formatNumber", func(v goja.Value, d goja.Value) string {
		dec := 2
		if d != nil && !goja.IsUndefined(d) {
			dec = int(d.ToInteger())
		}
		f, _ := toFloat(r.argValue(v))
		return formatDE(f, dec)
	})
	set("formatDate", func(v goja.Value) string { return formatDateDE(crud.Str(r.argValue(v))) })
	if in.Hook != nil {
		set("__hookData", r.toJS(map[string]any{"name": in.Hook.Name, "phase": in.Hook.Phase, "data": in.Hook.Data}))
		if _, err := vm.RunString(`var hook = (function(h) {
  function add(type) { return function(id, text, field) { h.messages.push({type: type, id: String(id || ""), text: String(text || ""), field: field ? String(field) : ""}); }; }
  h.messages = []; h.error = add("E"); h.warning = add("W"); h.info = add("S");
  return h; })(__hookData); delete this.__hookData;`); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) argValue(v goja.Value) any {
	out, err := r.export(v)
	if err != nil {
		r.throw(err)
	}
	return out
}

// baseline: darf jeder angemeldete Benutzer (wie im Dispatcher).
var baseline = map[string]bool{"Catalog.ListObjects": true, "Catalog.GetDefinition": true, "Catalog.ListActions": true,
	"Catalog.ListModules": true, "Catalog.GetModule": true, "Account.Me": true}

// allow prüft einen Aufruf gegen die erlaubten Aufrufe, die Grenze je Lauf und
// die Berechtigungen des angemeldeten Benutzers.
func (r *runner) allow(object, action string) error {
	r.calls++
	if r.calls > r.m.cfg.MaxCalls {
		return fmt.Errorf("mehr als %d Aufrufe in einem Lauf", r.m.cfg.MaxCalls)
	}
	if !objectRe.MatchString(object) || object == "*" || !actionRe.MatchString(action) || action == "*" {
		return fmt.Errorf("%w: Aufruf %q.%q", sdk.ErrInvalidArgument, object, action)
	}
	return checkCall(r.ctx, r.patterns, r.grants, object, action, "Skript "+r.script.Code)
}

// checkCall: Muster und Berechtigung (Ergebnis je Object.action zwischengespeichert).
func checkCall(ctx context.Context, patterns []string, cache map[string]bool, object, action, who string) error {
	key := object + "." + action
	ok := false
	for _, p := range patterns {
		if hit, _ := path.Match(p, key); hit {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("%w: %s darf %s nicht aufrufen (erlaubte Aufrufe)", sdk.ErrPermissionDenied, who, key)
	}
	if sdk.CallFromContext(ctx).UserID == "" || baseline[key] {
		return nil // Systemanfrage (Event) bzw. Grundrecht
	}
	allowed, seen := cache[key]
	if !seen {
		g, err := sdk.Grants(ctx, object, action)
		if err != nil {
			return err
		}
		allowed = g.All || len(g.CompanyCodes) > 0 || len(g.Rules) > 0
		cache[key] = allowed
	}
	if !allowed {
		return fmt.Errorf("%w: keine Berechtigung für %s", sdk.ErrPermissionDenied, key)
	}
	return nil
}

func (r *runner) accessObject(c goja.FunctionCall) goja.Value {
	object, action := c.Argument(0).String(), c.Argument(1).String()
	if err := r.allow(object, action); err != nil {
		r.throw(err)
	}
	resp, err := r.m.services.Call(r.ctx, object, action, r.arg(c, 2))
	if err != nil {
		r.throw(fmt.Errorf("%s.%s: %w", object, action, err))
	}
	return r.toJS(resp.Payload)
}

func (r *runner) readObject(c goja.FunctionCall) goja.Value {
	object, action := c.Argument(0).String(), c.Argument(1).String()
	if err := r.allow(object, action); err != nil {
		r.throw(err)
	}
	limit := r.m.cfg.MaxReadRows
	var fn goja.Callable
	if len(c.Arguments) > 3 {
		if f, ok := goja.AssertFunction(c.Arguments[3]); ok {
			fn = f
		} else if n := int(c.Arguments[3].ToInteger()); n > 0 && n < limit {
			limit = n
		}
	}
	payload, _ := r.arg(c, 2).(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}
	rows, _, err := readRows(r.ctx, r.m, object, action, payload, limit, fn == nil && limit < r.m.cfg.MaxReadRows)
	if err != nil {
		r.throw(err)
	}
	if fn == nil {
		return r.toJS(rows)
	}
	n := 0
	for i, row := range rows {
		v, err := fn(goja.Undefined(), r.toJS(row), r.vm.ToValue(i))
		if err != nil {
			panic(err)
		}
		n++
		if v != nil && v.StrictEquals(r.vm.ToValue(false)) {
			break
		}
	}
	return r.vm.ToValue(n)
}

var errEnough = errors.New("genug Zeilen")

// rowCollector sammelt einen Datenstrom bis zur Grenze.
type rowCollector struct {
	cols  []string
	rows  []map[string]any
	limit int
	full  bool
}

func (w *rowCollector) Header(h sdk.ReadHeader) error { w.cols = h.Columns; return nil }

func (w *rowCollector) Rows(rows [][]any) error {
	for _, r := range rows {
		if len(w.rows) >= w.limit {
			w.full = true
			return errEnough
		}
		rec := make(map[string]any, len(w.cols))
		for i, c := range w.cols {
			if i < len(r) {
				rec[c] = r[i]
			}
		}
		w.rows = append(w.rows, rec)
	}
	return nil
}

// readRows liest einen Datenstrom. truncate: bei der Grenze still abschneiden
// (eigene Grenze des Skripts), sonst Fehler (Grenze der Konfiguration).
func readRows(ctx context.Context, m *Module, object, action string, payload map[string]any, limit int, truncate bool) ([]map[string]any, []string, error) {
	if _, ok := payload["limit"]; !ok && truncate {
		payload["limit"] = limit
	}
	w := &rowCollector{limit: limit}
	_, err := m.services.Read(ctx, object, action, payload, w)
	if err != nil && !(errors.Is(err, errEnough) || (w.full && strings.Contains(err.Error(), errEnough.Error()))) {
		return nil, nil, fmt.Errorf("%s.%s: %w", object, action, err)
	}
	if w.full && !truncate {
		return nil, nil, fmt.Errorf("%s.%s: mehr als %d Zeilen (Grenze max_read_rows) – Auswahl einschränken", object, action, limit)
	}
	if w.rows == nil {
		w.rows = []map[string]any{}
	}
	return w.rows, w.cols, nil
}

func (r *runner) callTemplate(c goja.FunctionCall) goja.Value {
	opts, _ := r.arg(c, 2).(map[string]any)
	out, err := renderTemplate(r.ctx, r.m, func(o, a string) error { return r.allow(o, a) }, c.Argument(0).String(), r.arg(c, 1),
		crud.Str(opts["company_code"]), crud.Str(opts["language"]), int(toInt(opts["wait"])))
	if err != nil {
		r.throw(err)
	}
	return r.toJS(out)
}

// renderTemplate: Auftrag im Modul Vorlagen (Template.render, Template.get) –
// Ergebnis {message, job_number, download}.
func renderTemplate(ctx context.Context, m *Module, allow func(object, action string) error, template string, data any, cc, lang string, wait int) (map[string]any, error) {
	if strings.TrimSpace(template) == "" {
		return nil, crud.Invalid("Vorlage fehlt")
	}
	if strings.TrimSpace(cc) == "" {
		return nil, crud.Invalid("Vorlage %s: Buchungskreis (company_code) fehlt", template)
	}
	for _, a := range []string{"render", "get"} {
		if err := allow("Template", a); err != nil {
			return nil, err
		}
	}
	if wait <= 0 {
		wait = 180
	}
	resp, err := m.services.Call(ctx, "Template", "render", map[string]any{"template": strings.TrimSpace(template), "language": strings.TrimSpace(lang),
		"company_code": strings.TrimSpace(cc), "data": data, "wait": wait})
	if err != nil {
		return nil, fmt.Errorf("Vorlagen: %w", err)
	}
	var job struct {
		JobNumber string `json:"job_number"`
	}
	if err := sdk.Decode(resp.Payload, &job); err != nil {
		return nil, err
	}
	resp, err = m.services.Call(ctx, "Template", "get", map[string]any{"job_number": job.JobNumber})
	if err != nil {
		return nil, fmt.Errorf("Vorlagen: %w", err)
	}
	var res struct {
		Status      string `json:"status"`
		StatusText  string `json:"status_text"`
		ContentType string `json:"content_type"`
		FileName    string `json:"file_name"`
		Content     string `json:"content"`
	}
	if err := sdk.Decode(resp.Payload, &res); err != nil {
		return nil, err
	}
	out := map[string]any{"job_number": job.JobNumber}
	switch res.Status {
	case "DONE":
		out["message"] = "Auftrag " + job.JobNumber + ": " + res.FileName
		if _, err := base64.StdEncoding.DecodeString(res.Content); err == nil && res.Content != "" {
			out["download"] = metamodel.Content{Content: res.Content, ContentType: res.ContentType, FileName: res.FileName}
		}
	case "ERROR":
		return nil, crud.Invalid("Auftrag %s: %s (Vorlagen → Aufträge)", job.JobNumber, res.StatusText)
	default:
		out["message"] = "Auftrag " + job.JobNumber + " läuft noch – das Ergebnis steht dann unter Vorlagen → Aufträge"
	}
	return out, nil
}

func (r *runner) logFn(c goja.FunctionCall) goja.Value {
	if r.cut {
		return goja.Undefined()
	}
	parts := make([]string, len(c.Arguments))
	for i, a := range c.Arguments {
		if o, ok := a.(*goja.Object); ok && o.ClassName() != "Function" {
			if s, err := r.strfy(goja.Undefined(), a); err == nil && !goja.IsUndefined(s) {
				parts[i] = s.String()
				continue
			}
		}
		parts[i] = a.String()
	}
	line := strings.Join(parts, " ") + "\n"
	if r.log.Len()+len(line) > r.m.cfg.LogBytes {
		r.log.WriteString("… (weitere Ausgaben abgeschnitten)\n")
		r.cut = true
		return goja.Undefined()
	}
	r.log.WriteString(line)
	return goja.Undefined()
}

func (r *runner) tableFn(c goja.FunctionCall) goja.Value {
	return r.toJS(makeTable(r.arg(c, 0), r.arg(c, 1)))
}

func (r *runner) downloadFn(c goja.FunctionCall) goja.Value {
	ct := "text/plain; charset=utf-8"
	if len(c.Arguments) > 2 {
		ct = c.Argument(2).String()
	}
	return r.toJS(metamodel.Content{Content: base64.StdEncoding.EncodeToString([]byte(c.Argument(0).String())), ContentType: ct,
		FileName: firstOf(c.Argument(1).String(), "ergebnis.txt")})
}

// hookOutcome: return {data, messages} oder die Werte von hook (data, messages).
func (r *runner) hookOutcome(ret any) (*hookResult, error) {
	h, err := r.export(r.vm.Get("hook"))
	if err != nil {
		return nil, err
	}
	hm, _ := h.(map[string]any)
	out := &hookResult{Data: hm["data"]}
	msgs, _ := hm["messages"].([]any)
	if rm, ok := ret.(map[string]any); ok {
		if d, ok := rm["data"]; ok {
			out.Data = d
		}
		if l, ok := rm["messages"].([]any); ok {
			msgs = append(msgs, l...)
		}
	}
	for _, x := range msgs {
		if mm, ok := x.(map[string]any); ok {
			out.Messages = append(out.Messages, mm)
		}
	}
	return out, nil
}

// --- Tabellen und Formate -----------------------------------------------------------

// makeTable: Zeilen (Objekte oder Arrays) → {columns, rows} mit Texten.
// columns: Liste von Feldnamen oder {field, label}; leer = Felder der ersten Zeile.
func makeTable(rowsArg, colsArg any) map[string]any {
	rows, _ := rowsArg.([]any)
	var fields, labels []string
	if cl, ok := colsArg.([]any); ok {
		for _, c := range cl {
			switch x := c.(type) {
			case map[string]any:
				f := crud.Str(x["field"])
				fields, labels = append(fields, f), append(labels, firstOf(crud.Str(x["label"]), f))
			default:
				fields, labels = append(fields, crud.Str(x)), append(labels, crud.Str(x))
			}
		}
	}
	if fields == nil && len(rows) > 0 {
		if first, ok := rows[0].(map[string]any); ok {
			for k := range first {
				fields = append(fields, k)
			}
			sort.Strings(fields)
			labels = fields
		}
	}
	out := [][]string{}
	for _, row := range rows {
		switch x := row.(type) {
		case map[string]any:
			line := make([]string, len(fields))
			for i, f := range fields {
				line[i] = cell(getPath(x, f))
			}
			out = append(out, line)
		case []any:
			line := make([]string, len(x))
			for i, v := range x {
				line[i] = cell(v)
			}
			out = append(out, line)
		default:
			out = append(out, []string{cell(x)})
		}
	}
	if labels == nil {
		labels = []string{}
	}
	return map[string]any{"columns": labels, "rows": out}
}

// cell: Wert als Text (Objekte als JSON).
func cell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "ja"
		}
		return "nein"
	case map[string]any, []any:
		b, _ := json.Marshal(x)
		return string(b)
	}
	return crud.Str(v)
}

// getPath: Feld mit Punkten (a.b.c) in verschachtelten Objekten.
func getPath(rec map[string]any, field string) any {
	if v, ok := rec[field]; ok {
		return v
	}
	var cur any = rec
	for _, p := range strings.Split(field, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		if strings.Contains(s, ",") && !strings.Contains(s, ".") {
			s = strings.ReplaceAll(s, ",", ".")
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return 0, false
}

// formatDE: Zahl deutsch mit Tausenderpunkten (1.234,56).
func formatDE(v float64, dec int) string {
	if dec < 0 {
		dec = 0
	}
	neg := v < 0
	s := strconv.FormatFloat(math.Abs(v), 'f', dec, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if neg && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

// formatDateDE: JJJJ-MM-TT… → TT.MM.JJJJ (sonst unverändert).
func formatDateDE(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[8:10] + "." + s[5:7] + "." + s[0:4]
	}
	return s
}
