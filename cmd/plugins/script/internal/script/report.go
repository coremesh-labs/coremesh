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
// Abschnitte (Zwischenüberschrift und -summe je Wert), Summenzeile; Ausgabe
// als Tabelle mit CSV und, mit Vorlage, zusätzlich deren Datei (z. B. PDF).

type reportRow struct {
	Code, Name, Description, Kind, Object, Action, Payload, ResultPath, Script string
	GroupBy, SortBy, Template, Language, CompanyParam, SectionBy, Subtitle     string
	Stream, Totals, Active                                                     bool
	Limit                                                                      int
}

type reportCol struct {
	Field, Label, Aggregate, Format string
	Width                           float64 // relativ (PDF); 0 = Text 3, Zahl 1
}

func (m *Module) loadReport(ctx context.Context, code string) (*reportRow, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, description, source_kind, source_object, source_action, source_payload, result_path, source_script,
		group_by, sort_by, template, template_language, company_param, stream, totals, is_active, row_limit, section_by, subtitle FROM script__report WHERE code = ?`, code)
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
		Stream: crud.AsBool(r[14]), Totals: crud.AsBool(r[15]), Active: crud.AsBool(r[16]), Limit: int(toInt(r[17])), SectionBy: s(18), Subtitle: s(19)}, nil
}

func (m *Module) loadColumns(ctx context.Context, code string) ([]reportCol, error) {
	res, err := m.db.Query(ctx, `SELECT field, label, aggregate, format, width FROM script__report_column WHERE report = ? ORDER BY position`, code)
	if err != nil {
		return nil, err
	}
	out := make([]reportCol, 0, len(res.Rows))
	for _, r := range res.Rows {
		f := crud.Str(r[0])
		w, _ := toFloat(r[4])
		out = append(out, reportCol{Field: f, Label: firstOf(crud.Str(r[1]), f), Aggregate: firstOf(crud.Str(r[2]), "NONE"), Format: firstOf(crud.Str(r[3]), "TEXT"), Width: w})
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
	m.onlyParams(reportObject, &st)
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
	if in.Data == nil { // Konsole: Parameter direkt im Payload
		in.Data, _ = req.Payload.(map[string]any)
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
	src, err := m.reportSource(ctx, rep, params)
	if err != nil {
		return nil, src.log, err
	}
	log := src.log
	cols, err := m.loadColumns(ctx, rep.Code)
	if err != nil {
		return nil, log, err
	}
	if len(cols) == 0 {
		cols = src.cols
	}
	if len(cols) == 0 {
		cols = defaultColumns(src.rows, nil)
	}
	out := buildReport(src.rows, cols, splitList(rep.GroupBy), splitList(rep.SortBy), rep.Limit, rep.Totals, rep.SectionBy)
	table := map[string]any{"columns": out.labels(), "rows": out.formatted()}
	resp := map[string]any{"message": fmt.Sprintf("%s: %d Zeilen", rep.Name, len(out.Rows)), "table": table}
	if src.message != "" {
		resp["message"] = resp["message"].(string) + " · " + src.message
	}
	csvFile := metamodel.Content{Content: base64.StdEncoding.EncodeToString(out.csv()), ContentType: "text/csv; charset=utf-8",
		FileName: strings.ToLower(rep.Code) + ".csv"}
	if rep.Template == "" {
		resp["download"] = csvFile
		return resp, log, nil
	}
	subtitle := firstOf(src.subtitle, fillText(rep.Subtitle, params, crud.KeyDate(ctx)))
	cc := crud.Str(params[firstOf(rep.CompanyParam, "company_code")])
	tpl, err := renderTemplate(ctx, m, func(o, a string) error {
		return checkCall(ctx, []string{"Template.*"}, map[string]bool{}, o, a, "Bericht "+rep.Code)
	},
		rep.Template, out.templateData(rep, params, firstOf(src.title, rep.Name), subtitle, src.data), cc, rep.Language, 0)
	if err != nil {
		return nil, log, err
	}
	resp["message"] = resp["message"].(string) + " – " + crud.Str(tpl["message"])
	// Datei der Vorlage (z. B. PDF) und die CSV-Datei
	downloads := []any{}
	if d, ok := tpl["download"]; ok {
		downloads = append(downloads, d)
	}
	resp["downloads"] = append(downloads, csvFile)
	return resp, log, nil
}

// fillText ersetzt {parameter} und {key_date} in einem Text (Datumswerte deutsch).
func fillText(src string, params map[string]any, keyDate string) string {
	return placeholderRe.ReplaceAllStringFunc(src, func(ph string) string {
		name := ph[1 : len(ph)-1]
		v, ok := params[name]
		if name == "key_date" && !ok {
			v, ok = keyDate, true
		}
		if !ok {
			return ph
		}
		t := cell(v)
		if dateRe.MatchString(t) {
			return formatDateDE(t)
		}
		return t
	})
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// sourceOut: Zeilen der Datenquelle; Spalten in ihrer Reihenfolge (Datenstrom,
// Skript), vom Skript auch Titel, Untertitel und Meldung.
type sourceOut struct {
	rows                     []map[string]any
	cols                     []reportCol
	title, subtitle, message string
	data                     map[string]any // weitere Daten für die Vorlage (z. B. company)
	log                      string
}

// reportSource liefert die Zeilen der Datenquelle.
func (m *Module) reportSource(ctx context.Context, rep *reportRow, params map[string]any) (sourceOut, error) {
	if rep.Kind == "SCRIPT" {
		s, err := m.loadScript(ctx, rep.Script)
		if err != nil {
			return sourceOut{}, err
		}
		if !s.Active {
			return sourceOut{}, crud.Invalid("Skript %s ist inaktiv", s.Code)
		}
		res, err := m.execute(ctx, s, runInput{Trigger: trigReport, Detail: rep.Code, Params: params})
		out := sourceOut{log: res.Log}
		if err != nil {
			return out, fmt.Errorf("Skript %s: %v", s.Code, err)
		}
		v := res.Value
		if mm, ok := v.(map[string]any); ok {
			v = mm["rows"]
			out.cols = scriptColumns(mm["columns"])
			out.title, out.subtitle, out.message = crud.Str(mm["title"]), crud.Str(mm["subtitle"]), crud.Str(mm["message"])
			out.data, _ = mm["data"].(map[string]any)
		}
		list, ok := v.([]any)
		if !ok && v != nil {
			return out, crud.Invalid("Skript %s: return muss Zeilen liefern (Array von Objekten oder {rows: […]})", s.Code)
		}
		out.rows, err = asRecords(list)
		return out, err
	}
	payload, err := fillPayload(rep.Payload, params, crud.KeyDate(ctx))
	if err != nil {
		return sourceOut{}, err
	}
	// Die Datenquelle des Berichts ist zugleich seine Erlaubnis; die
	// Berechtigung des Benutzers gilt trotzdem.
	if err := checkCall(ctx, []string{rep.Object + "." + rep.Action}, map[string]bool{}, rep.Object, rep.Action, "Bericht "+rep.Code); err != nil {
		return sourceOut{}, err
	}
	if rep.Stream {
		limit := m.cfg.MaxReadRows
		rows, cols, err := readRows(ctx, m, rep.Object, rep.Action, payload, limit, false)
		return sourceOut{rows: rows, cols: defaultColumns(nil, cols)}, err
	}
	resp, err := m.services.Call(ctx, rep.Object, rep.Action, payload)
	if err != nil {
		return sourceOut{}, fmt.Errorf("%s.%s: %w", rep.Object, rep.Action, err)
	}
	var body any
	if err := sdk.Decode(resp.Payload, &body); err != nil {
		return sourceOut{}, err
	}
	list, err := rowsAt(body, rep.ResultPath)
	if err != nil {
		return sourceOut{}, err
	}
	rows, err := asRecords(list)
	return sourceOut{rows: rows}, err
}

// scriptColumns: columns des Skripts – Feldnamen oder Objekte
// {field, label, format, aggregate, width}.
func scriptColumns(v any) []reportCol {
	list, _ := v.([]any)
	var out []reportCol
	for _, c := range list {
		if mm, ok := c.(map[string]any); ok {
			f := crud.Str(mm["field"])
			if f == "" {
				continue
			}
			w, _ := toFloat(mm["width"])
			out = append(out, reportCol{Field: f, Label: firstOf(crud.Str(mm["label"]), f), Aggregate: firstOf(strings.ToUpper(crud.Str(mm["aggregate"])), "NONE"),
				Format: firstOf(strings.ToUpper(crud.Str(mm["format"])), "TEXT"), Width: w})
			continue
		}
		if f := crud.Str(c); f != "" {
			out = append(out, reportCol{Field: f, Label: f, Aggregate: "NONE", Format: "TEXT"})
		}
	}
	return out
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
	Cols     []reportCol
	Rows     [][]any          // Rohwerte je Spalte
	Extras   []map[string]any // _style (Zeilenart, z. B. HEAD, SUM, TOTAL) und _indent (Einrückung) der Quelle, für die Vorlage
	Total    []any            // Summenzeile (nil = keine)
	Sections []section
	secCol   int // Spalte des Abschnittsfelds (-1 = keine Spalte)
	groups   bool
}

// section: Abschnitt der Zeilen [Start, End) mit Zwischensumme (nil = keine).
type section struct {
	Label      string
	Start, End int
	Total      []any
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

func buildReport(rows []map[string]any, cols []reportCol, groupBy, sortBy []string, limit int, totals bool, sectionBy string) *reportOut {
	out := &reportOut{Cols: cols, groups: len(groupBy) > 0, secCol: -1}
	type line struct {
		vals []any
		src  map[string]any // erste Zeile der Quelle (Gruppe)
	}
	var lines []line
	if out.groups {
		type group struct {
			key  []any
			src  map[string]any
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
				g = &group{key: kv, src: r, aggs: make([]*aggState, len(cols))}
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
			l := make([]any, len(cols))
			for i, c := range cols {
				l[i] = g.aggs[i].value(c.Aggregate)
				for gi, gf := range groupBy {
					if gf == c.Field {
						l[i] = g.key[gi]
					}
				}
			}
			lines = append(lines, line{l, g.src})
		}
	} else {
		for _, r := range rows {
			l := make([]any, len(cols))
			for i, c := range cols {
				l[i] = getPath(r, c.Field)
			}
			lines = append(lines, line{l, r})
		}
	}
	for i, c := range cols {
		if sectionBy != "" && c.Field == sectionBy {
			out.secCol = i
		}
	}
	secKey := func(l line) string {
		if out.secCol >= 0 {
			return cell(l.vals[out.secCol])
		}
		return cell(getPath(l.src, sectionBy))
	}
	// Abschnitte in der Reihenfolge ihres ersten Auftretens, darin sortiert.
	secOrder := map[string]int{}
	if sectionBy != "" {
		for _, l := range lines {
			if _, ok := secOrder[secKey(l)]; !ok {
				secOrder[secKey(l)] = len(secOrder)
			}
		}
	}
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
	if len(keys) > 0 || sectionBy != "" {
		sort.SliceStable(lines, func(a, b int) bool {
			if sectionBy != "" {
				if sa, sb := secOrder[secKey(lines[a])], secOrder[secKey(lines[b])]; sa != sb {
					return sa < sb
				}
			}
			for _, k := range keys {
				x, y := lines[a].vals[k.idx], lines[b].vals[k.idx]
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
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	out.Rows, out.Extras = make([][]any, len(lines)), make([]map[string]any, len(lines))
	for i, l := range lines {
		ind, _ := toFloat(l.src["_indent"])
		out.Rows[i], out.Extras[i] = l.vals, map[string]any{"_style": strings.ToUpper(crud.Str(l.src["_style"])), "_indent": int(ind)}
	}
	if sectionBy != "" {
		for i, l := range lines {
			k := secKey(l)
			if n := len(out.Sections); n == 0 || out.Sections[n-1].Label != k {
				out.Sections = append(out.Sections, section{Label: k, Start: i})
			}
			out.Sections[len(out.Sections)-1].End = i + 1
		}
		if totals {
			for si := range out.Sections {
				sec := &out.Sections[si]
				part := make([]map[string]any, 0, sec.End-sec.Start)
				for _, l := range lines[sec.Start:sec.End] {
					mm := map[string]any{}
					for i, c := range cols {
						mm[c.Field] = l.vals[i]
					}
					part = append(part, mm)
				}
				if sec.Total = totalLine(part, cols, out.secCol, "Summe "+sec.Label); sec.Total != nil && out.secCol >= 0 {
					sec.Total[out.secCol] = sec.Label
				}
			}
		}
	}
	if totals {
		// Summenzeile über alle Zeilen der Quelle (nicht über die Gruppen).
		out.Total = totalLine(rows, cols, out.secCol, "Summe")
	}
	return out
}

// totalLine verdichtet die Zeilen je Spalte (nil = keine Spalte mit Verdichtung);
// label steht in der ersten Spalte außer der Abschnittsspalte skip, wenn sie
// keine Verdichtung hat.
func totalLine(rows []map[string]any, cols []reportCol, skip int, label string) []any {
	total := make([]any, len(cols))
	has := false
	for i, c := range cols {
		if c.Aggregate == "NONE" {
			continue
		}
		a := &aggState{}
		for _, r := range rows {
			a.add(getPath(r, c.Field))
		}
		total[i], has = a.value(c.Aggregate), true
	}
	if !has {
		return nil
	}
	first := 0
	if skip == 0 {
		first = 1
	}
	if first < len(cols) && cols[first].Aggregate == "NONE" {
		total[first] = label
	}
	return total
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

// formatted: Zeilen als Texte; je Abschnitt eine Überschrift (wenn das Feld
// keine Spalte ist) und die Zwischensumme, die Summenzeile zuletzt.
func (o *reportOut) formatted() [][]string {
	out := make([][]string, 0, len(o.Rows)+1)
	if len(o.Sections) == 0 {
		for _, l := range o.Rows {
			out = append(out, o.formatLine(l))
		}
	}
	for _, s := range o.Sections {
		if o.secCol < 0 && len(o.Cols) > 0 {
			head := make([]string, len(o.Cols))
			head[0] = s.Label
			out = append(out, head)
		}
		for _, l := range o.Rows[s.Start:s.End] {
			out = append(out, o.formatLine(l))
		}
		if s.Total != nil {
			out = append(out, o.formatLine(s.Total))
		}
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

// templateData: Daten für die Vorlage – Titel, Untertitel, Spalten (mit
// numeric, share = Anteil an der Breite ohne Abschnittsspalte, section;
// table_columns ohne die Abschnittsspalte), Zeilen formatiert (rows, mit _style
// und _indent) und roh (values), Abschnitte mit Zeilen und
// Zwischensumme, Summenzeile, Parameter; dazu data des Skripts (z. B. company).
func (o *reportOut) templateData(rep *reportRow, params map[string]any, title, subtitle string, extra map[string]any) map[string]any {
	weights, sum := make([]float64, len(o.Cols)), 0.0
	for i, c := range o.Cols {
		if i == o.secCol {
			continue
		}
		w := c.Width
		if w <= 0 {
			w = 3
			if numericFormat(c.Format) {
				w = 1
			}
		}
		weights[i], sum = w, sum+w
	}
	cols, visible := make([]any, len(o.Cols)), []any{}
	for i, c := range o.Cols {
		share := 0.0
		if sum > 0 {
			share = weights[i] / sum
		}
		cols[i] = map[string]any{"field": c.Field, "label": c.Label, "format": c.Format, "aggregate": c.Aggregate,
			"numeric": numericFormat(c.Format), "section": i == o.secCol, "share": fmt.Sprintf("%.4f", share)}
		if i != o.secCol {
			visible = append(visible, cols[i])
		}
	}
	asMap := func(line []any, formatted bool) map[string]any {
		mm := map[string]any{}
		f := o.formatLine(line)
		for i, c := range o.Cols {
			if formatted {
				mm[c.Field] = f[i]
			} else {
				mm[c.Field] = line[i]
			}
		}
		return mm
	}
	rows, values := make([]any, len(o.Rows)), make([]any, len(o.Rows))
	for i, l := range o.Rows {
		rows[i], values[i] = asMap(l, true), asMap(l, false)
		for k, v := range o.Extras[i] {
			rows[i].(map[string]any)[k] = v
		}
	}
	sections := make([]any, len(o.Sections))
	for i, s := range o.Sections {
		sec := map[string]any{"label": s.Label, "rows": rows[s.Start:s.End]}
		if s.Total != nil {
			sec["totals"] = asMap(s.Total, true)
		}
		sections[i] = sec
	}
	data := map[string]any{"title": title, "subtitle": subtitle, "description": rep.Description, "report": rep.Code, "params": params,
		"columns": cols, "table_columns": visible, "rows": rows, "values": values, "sections": sections}
	if o.Total != nil {
		data["totals"] = asMap(o.Total, true)
	}
	for k, v := range extra {
		if _, ok := data[k]; !ok {
			data[k] = v
		}
	}
	return data
}

func numericFormat(f string) bool {
	switch f {
	case "NUMBER", "INTEGER", "MONEY", "MONEY_MINOR":
		return true
	}
	return false
}
