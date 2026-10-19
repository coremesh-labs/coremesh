package script

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Berichte: Zeilen aus einer Datenquelle (Action eines Objects bzw. Skript),
// dann Spalten mit Format, Gruppen mit Verdichtung, Sortierung, Grenze,
// Summenzeile; Ausgabe als Tabelle mit CSV oder über eine Vorlage.

type reportRow struct {
	Code, Name, Description, Kind, Object, Action, Payload, ResultPath, Script string
	GroupBy, SortBy, Template, Language, CompanyParam                          string
	Stream, Totals, Active                                                     bool
	Limit                                                                      int
}

type reportCol struct {
	Field, Label, Aggregate, Format string
}

func (m *Module) loadReport(ctx context.Context, code string) (*reportRow, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, description, source_kind, source_object, source_action, source_payload, result_path, source_script,
		group_by, sort_by, template, template_language, company_param, stream, totals, is_active, row_limit FROM script__report WHERE code = ?`, code)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Bericht %s", sdk.ErrNotFound, code)
	}
	r := res.Rows[0]
	s := func(i int) string { return crud.Str(r[i]) }
	return &reportRow{Code: s(0), Name: s(1), Description: s(2), Kind: s(3), Object: s(4), Action: firstOf(s(5), "list"), Payload: s(6),
		ResultPath: s(7), Script: s(8), GroupBy: s(9), SortBy: s(10), Template: s(11), Language: s(12), CompanyParam: s(13),
		Stream: crud.AsBool(r[14]), Totals: crud.AsBool(r[15]), Active: crud.AsBool(r[16]), Limit: int(toInt(r[17]))}, nil
}

func (m *Module) loadColumns(ctx context.Context, code string) ([]reportCol, error) {
	res, err := m.db.Query(ctx, `SELECT field, label, aggregate, format FROM script__report_column WHERE report = ? ORDER BY position`, code)
	if err != nil {
		return nil, err
	}
	out := make([]reportCol, 0, len(res.Rows))
	for _, r := range res.Rows {
		f := crud.Str(r[0])
		out = append(out, reportCol{Field: f, Label: firstOf(crud.Str(r[1]), f), Aggregate: firstOf(crud.Str(r[2]), "NONE"), Format: firstOf(crud.Str(r[3]), "TEXT")})
	}
	return out, nil
}

func (m *Module) reportFormState(ctx context.Context, req metamodel.FormStateRequest) (metamodel.FormState, error) {
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if req.Mode != "action" || req.Action != "run" {
		return st, nil
	}
	key, err := m.set.Entity(reportObject).ParseID(req.ID)
	if err != nil {
		return st, err
	}
	code := crud.Str(key["code"])
	params, err := m.loadParams(ctx, "script__report_parameter", "report", code)
	if err != nil {
		return st, err
	}
	paramForm(ctx, params, &st, req.Values)
	st.Message = firstOf(m.text(ctx, `SELECT description FROM script__report WHERE code = ?`, code), "Erstellt den Bericht mit diesen Eingaben.")
	return st, nil
}

func (m *Module) reportRunAction(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	key, err := m.set.Entity(reportObject).ParseID(in.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	rep, err := m.loadReport(ctx, crud.Str(key["code"]))
	if err != nil {
		return sdk.Response{}, err
	}
	if !rep.Active {
		return sdk.Response{}, crud.Invalid("Bericht %s ist inaktiv", rep.Code)
	}
	defs, err := m.loadParams(ctx, "script__report_parameter", "report", rep.Code)
	if err != nil {
		return sdk.Response{}, err
	}
	if in.Data == nil {
		in.Data = map[string]any{}
	}
	params, err := paramValues(ctx, defs, in.Data)
	if err != nil {
		return sdk.Response{}, err
	}
	start := m.now()
	out, log, err := m.runReport(ctx, rep, params)
	m.recordRun(ctx, runEntry{Report: rep.Code, Trigger: trigReport, Detail: rep.Name,
		Result: &runResult{Value: map[string]any{"message": crud.Str(out["message"])}, Log: log, Duration: m.now().Sub(start)}, Err: err})
	if err != nil {
		return sdk.Response{}, crud.Invalid("Bericht %s: %v", rep.Code, err)
	}
	return sdk.Response{Payload: out}, nil
}

// runReport rechnet den Bericht und liefert die Antwort der Aktion.
func (m *Module) runReport(ctx context.Context, rep *reportRow, params map[string]any) (map[string]any, string, error) {
	rows, srcCols, log, err := m.reportSource(ctx, rep, params)
	if err != nil {
		return nil, log, err
	}
	cols, err := m.loadColumns(ctx, rep.Code)
	if err != nil {
		return nil, log, err
	}
	if len(cols) == 0 {
		cols = defaultColumns(rows, srcCols)
	}
	out := buildReport(rows, cols, splitList(rep.GroupBy), splitList(rep.SortBy), rep.Limit, rep.Totals)
	table := map[string]any{"columns": out.labels(), "rows": out.formatted()}
	resp := map[string]any{"message": fmt.Sprintf("%s: %d Zeilen", rep.Name, len(out.Rows)), "table": table}
	if rep.Template == "" {
		resp["download"] = metamodel.Content{Content: base64.StdEncoding.EncodeToString(out.csv()), ContentType: "text/csv; charset=utf-8",
			FileName: strings.ToLower(rep.Code) + ".csv"}
		return resp, log, nil
	}
	cc := crud.Str(params[firstOf(rep.CompanyParam, "company_code")])
	tpl, err := renderTemplate(ctx, m, func(o, a string) error {
		return checkCall(ctx, []string{"Template.*"}, map[string]bool{}, o, a, "Bericht "+rep.Code)
	},
		rep.Template, out.templateData(rep, params), cc, rep.Language, 0)
	if err != nil {
		return nil, log, err
	}
	resp["message"] = resp["message"].(string) + " – " + crud.Str(tpl["message"])
	if d, ok := tpl["download"]; ok {
		resp["download"] = d
	}
	return resp, log, nil
}

// reportSource liefert die Zeilen der Datenquelle (und, beim Datenstrom, die Spalten in ihrer Reihenfolge).
func (m *Module) reportSource(ctx context.Context, rep *reportRow, params map[string]any) ([]map[string]any, []string, string, error) {
	if rep.Kind == "SCRIPT" {
		s, err := m.loadScript(ctx, rep.Script)
		if err != nil {
			return nil, nil, "", err
		}
		if !s.Active {
			return nil, nil, "", crud.Invalid("Skript %s ist inaktiv", s.Code)
		}
		res, err := m.execute(ctx, s, runInput{Trigger: trigReport, Detail: rep.Code, Params: params})
		if err != nil {
			return nil, nil, res.Log, fmt.Errorf("Skript %s: %v", s.Code, err)
		}
		v := res.Value
		if mm, ok := v.(map[string]any); ok {
			v = mm["rows"]
		}
		list, ok := v.([]any)
		if !ok && v != nil {
			return nil, nil, res.Log, crud.Invalid("Skript %s: return muss Zeilen liefern (Array von Objekten oder {rows: […]})", s.Code)
		}
		rows, err := asRecords(list)
		return rows, nil, res.Log, err
	}
	payload, err := fillPayload(rep.Payload, params, crud.KeyDate(ctx))
	if err != nil {
		return nil, nil, "", err
	}
	// Die Datenquelle des Berichts ist zugleich seine Erlaubnis; die
	// Berechtigung des Benutzers gilt trotzdem.
	if err := checkCall(ctx, []string{rep.Object + "." + rep.Action}, map[string]bool{}, rep.Object, rep.Action, "Bericht "+rep.Code); err != nil {
		return nil, nil, "", err
	}
	if rep.Stream {
		limit := m.cfg.MaxReadRows
		rows, cols, err := readRows(ctx, m, rep.Object, rep.Action, payload, limit, false)
		return rows, cols, "", err
	}
	resp, err := m.services.Call(ctx, rep.Object, rep.Action, payload)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%s.%s: %w", rep.Object, rep.Action, err)
	}
	var body any
	if err := sdk.Decode(resp.Payload, &body); err != nil {
		return nil, nil, "", err
	}
	list, err := rowsAt(body, rep.ResultPath)
	if err != nil {
		return nil, nil, "", err
	}
	rows, err := asRecords(list)
	return rows, nil, "", err
}

var placeholderRe = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)

// fillPayload: JSON der Anfrage mit Platzhaltern {parameter} (und {key_date}).
// Ein Text, der nur aus einem Platzhalter besteht, erhält den Wert mit Typ.
func fillPayload(src string, params map[string]any, keyDate string) (map[string]any, error) {
	if strings.TrimSpace(src) == "" {
		return map[string]any{}, nil
	}
	var v any
	if err := json.Unmarshal([]byte(src), &v); err != nil {
		return nil, crud.Invalid("Anfrage: kein gültiges JSON (%v)", err)
	}
	lookup := func(name string) (any, bool) {
		if name == "key_date" {
			if p, ok := params[name]; ok {
				return p, true
			}
			return keyDate, true
		}
		p, ok := params[name]
		return p, ok
	}
	var fill func(any) (any, error)
	fill = func(x any) (any, error) {
		switch t := x.(type) {
		case map[string]any:
			for k, e := range t {
				f, err := fill(e)
				if err != nil {
					return nil, err
				}
				t[k] = f
			}
			return t, nil
		case []any:
			for i, e := range t {
				f, err := fill(e)
				if err != nil {
					return nil, err
				}
				t[i] = f
			}
			return t, nil
		case string:
			if mm := placeholderRe.FindStringSubmatch(t); mm != nil && mm[0] == t {
				p, ok := lookup(mm[1])
				if !ok {
					return nil, crud.Invalid("Anfrage: Parameter %s unbekannt", mm[1])
				}
				return p, nil
			}
			var miss string
			out := placeholderRe.ReplaceAllStringFunc(t, func(s string) string {
				p, ok := lookup(s[1 : len(s)-1])
				if !ok {
					miss = s
				}
				return cell(p)
			})
			if miss != "" {
				return nil, crud.Invalid("Anfrage: Parameter %s unbekannt", miss)
			}
			return out, nil
		}
		return x, nil
	}
	out, err := fill(v)
	if err != nil {
		return nil, err
	}
	m, ok := out.(map[string]any)
	if !ok {
		return nil, crud.Invalid("Anfrage: JSON-Objekt erwartet")
	}
	return m, nil
}

// rowsAt: Zeilen unter dem Pfad (leer = die Antwort selbst als Liste, sonst items bzw. rows).
func rowsAt(body any, p string) ([]any, error) {
	if p = strings.TrimSpace(p); p != "" {
		cur := body
		for _, part := range strings.Split(p, ".") {
			mm, ok := cur.(map[string]any)
			if !ok {
				return nil, crud.Invalid("Zeilen unter %q: nicht gefunden", p)
			}
			cur = mm[part]
		}
		l, ok := cur.([]any)
		if !ok && cur != nil {
			return nil, crud.Invalid("Zeilen unter %q: keine Liste", p)
		}
		return l, nil
	}
	if l, ok := body.([]any); ok {
		return l, nil
	}
	if mm, ok := body.(map[string]any); ok {
		for _, k := range []string{"items", "rows"} {
			if l, ok := mm[k].([]any); ok {
				return l, nil
			}
		}
	}
	return nil, crud.Invalid("Antwort ohne Zeilen (items bzw. rows) – Pfad der Zeilen angeben")
}

func asRecords(list []any) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(list))
	for i, x := range list {
		mm, ok := x.(map[string]any)
		if !ok {
			return nil, crud.Invalid("Zeile %d ist kein Objekt", i+1)
		}
		out = append(out, mm)
	}
	return out, nil
}

// defaultColumns: ohne eigene Spalten die des Datenstroms bzw. die Felder
// der ersten Zeile (alphabetisch, ohne _labels u. ä.).
func defaultColumns(rows []map[string]any, src []string) []reportCol {
	fields := src
	if fields == nil && len(rows) > 0 {
		for k := range rows[0] {
			if !strings.HasPrefix(k, "_") {
				fields = append(fields, k)
			}
		}
		sort.Strings(fields)
	}
	out := make([]reportCol, len(fields))
	for i, f := range fields {
		out[i] = reportCol{Field: f, Label: f, Aggregate: "NONE", Format: "TEXT"}
	}
	return out
}

// --- Rechnen ------------------------------------------------------------------------

type reportOut struct {
	Cols   []reportCol
	Rows   [][]any // Rohwerte je Spalte
	Total  []any   // Summenzeile (nil = keine)
	groups bool
}

type aggState struct {
	first      any
	sum        float64
	n, numeric int
	min, max   any
}

func (a *aggState) add(v any) {
	if a.n == 0 {
		a.first = v
	}
	a.n++
	if v == nil {
		return
	}
	if f, ok := toFloat(v); ok {
		a.sum += f
		a.numeric++
	}
	if a.min == nil || less(v, a.min) {
		a.min = v
	}
	if a.max == nil || less(a.max, v) {
		a.max = v
	}
}

func (a *aggState) value(kind string) any {
	switch kind {
	case "SUM":
		return a.sum
	case "COUNT":
		return float64(a.n)
	case "MIN":
		return a.min
	case "MAX":
		return a.max
	case "AVG":
		if a.numeric == 0 {
			return nil
		}
		return a.sum / float64(a.numeric)
	}
	return a.first
}

func less(a, b any) bool {
	fa, oka := toFloat(a)
	fb, okb := toFloat(b)
	if oka && okb {
		return fa < fb
	}
	return cell(a) < cell(b)
}

func buildReport(rows []map[string]any, cols []reportCol, groupBy, sortBy []string, limit int, totals bool) *reportOut {
	out := &reportOut{Cols: cols, groups: len(groupBy) > 0}
	if out.groups {
		type group struct {
			key  []any
			aggs []*aggState
		}
		var order []string
		groups := map[string]*group{}
		for _, r := range rows {
			kv := make([]any, len(groupBy))
			for i, g := range groupBy {
				kv[i] = getPath(r, g)
			}
			b, _ := json.Marshal(kv)
			g, ok := groups[string(b)]
			if !ok {
				g = &group{key: kv, aggs: make([]*aggState, len(cols))}
				for i := range g.aggs {
					g.aggs[i] = &aggState{}
				}
				groups[string(b)], order = g, append(order, string(b))
			}
			for i, c := range cols {
				g.aggs[i].add(getPath(r, c.Field))
			}
		}
		for _, k := range order {
			g := groups[k]
			line := make([]any, len(cols))
			for i, c := range cols {
				line[i] = g.aggs[i].value(c.Aggregate)
				for gi, gf := range groupBy {
					if gf == c.Field {
						line[i] = g.key[gi]
					}
				}
			}
			out.Rows = append(out.Rows, line)
		}
	} else {
		for _, r := range rows {
			line := make([]any, len(cols))
			for i, c := range cols {
				line[i] = getPath(r, c.Field)
			}
			out.Rows = append(out.Rows, line)
		}
	}
	if len(sortBy) > 0 {
		type key struct {
			idx  int
			desc bool
		}
		var keys []key
		for _, s := range sortBy {
			desc := strings.HasPrefix(s, "-")
			f := strings.TrimPrefix(s, "-")
			for i, c := range cols {
				if c.Field == f {
					keys = append(keys, key{i, desc})
				}
			}
		}
		sort.SliceStable(out.Rows, func(a, b int) bool {
			for _, k := range keys {
				x, y := out.Rows[a][k.idx], out.Rows[b][k.idx]
				if less(x, y) {
					return !k.desc
				}
				if less(y, x) {
					return k.desc
				}
			}
			return false
		})
	}
	if limit > 0 && len(out.Rows) > limit {
		out.Rows = out.Rows[:limit]
	}
	if totals {
		// Summenzeile über alle Zeilen der Quelle (nicht über die Gruppen).
		out.Total = make([]any, len(cols))
		has := false
		for i, c := range cols {
			if c.Aggregate == "NONE" {
				continue
			}
			a := &aggState{}
			for _, r := range rows {
				a.add(getPath(r, c.Field))
			}
			out.Total[i], has = a.value(c.Aggregate), true
		}
		if !has {
			out.Total = nil
		} else if len(cols) > 0 && cols[0].Aggregate == "NONE" {
			out.Total[0] = "Summe"
		}
	}
	if out.Rows == nil {
		out.Rows = [][]any{}
	}
	return out
}

// format: Rohwert → Text nach dem Format der Spalte.
func format(v any, f string) string {
	if v == nil {
		return ""
	}
	switch f {
	case "NUMBER", "MONEY":
		if x, ok := toFloat(v); ok {
			return formatDE(x, 2)
		}
	case "INTEGER":
		if x, ok := toFloat(v); ok {
			return formatDE(x, 0)
		}
	case "MONEY_MINOR":
		if x, ok := toFloat(v); ok {
			return formatDE(x/100, 2)
		}
	case "DATE":
		return formatDateDE(cell(v))
	}
	return cell(v)
}

func (o *reportOut) labels() []string {
	out := make([]string, len(o.Cols))
	for i, c := range o.Cols {
		out[i] = c.Label
	}
	return out
}

func (o *reportOut) formatLine(line []any) []string {
	out := make([]string, len(line))
	for i, v := range line {
		out[i] = format(v, o.Cols[i].Format)
	}
	return out
}

// formatted: Zeilen als Texte, die Summenzeile zuletzt.
func (o *reportOut) formatted() [][]string {
	out := make([][]string, 0, len(o.Rows)+1)
	for _, l := range o.Rows {
		out = append(out, o.formatLine(l))
	}
	if o.Total != nil {
		out = append(out, o.formatLine(o.Total))
	}
	return out
}

// csv: Semikolon, deutsche Zahlen, mit BOM (Excel erkennt UTF-8).
func (o *reportOut) csv() []byte {
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf")
	write := func(fields []string) {
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(';')
			}
			if strings.ContainsAny(f, ";\"\n\r") {
				f = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
			}
			b.WriteString(f)
		}
		b.WriteString("\r\n")
	}
	write(o.labels())
	for _, l := range o.formatted() {
		write(l)
	}
	return []byte(b.String())
}

// templateData: Daten für die Vorlage – Spalten, Zeilen formatiert (rows) und
// roh (values), Summenzeile, Parameter.
func (o *reportOut) templateData(rep *reportRow, params map[string]any) map[string]any {
	cols := make([]any, len(o.Cols))
	for i, c := range o.Cols {
		cols[i] = map[string]any{"field": c.Field, "label": c.Label, "format": c.Format, "aggregate": c.Aggregate}
	}
	asMap := func(line []any, formatted bool) map[string]any {
		mm := map[string]any{}
		for i, c := range o.Cols {
			if formatted {
				mm[c.Field] = o.formatLine(line)[i]
			} else {
				mm[c.Field] = line[i]
			}
		}
		return mm
	}
	rows, values := make([]any, len(o.Rows)), make([]any, len(o.Rows))
	for i, l := range o.Rows {
		rows[i], values[i] = asMap(l, true), asMap(l, false)
	}
	data := map[string]any{"title": rep.Name, "description": rep.Description, "report": rep.Code, "params": params,
		"columns": cols, "rows": rows, "values": values}
	if o.Total != nil {
		data["totals"] = asMap(o.Total, true)
	}
	return data
}
