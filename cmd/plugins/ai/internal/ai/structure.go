package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Struktur einer Objekt-Anfrage: Die Zeilen (AiRequestObject) bilden einen
// Baum – die Wurzel, ihre Unter-Objekte über eine Relation (Abschnitt der
// Wurzel, angelegt über saveAggregate) und eingebettete Objekte über ein
// Verweisfeld (vorher angelegt, z. B. die Adresse einer Partner-Adresse).
// In der JSON-Struktur stehen Wurzel und Relationen oben (Listen bei
// „mehrere“), eingebettete Objekte im übergeordneten Eintrag unter ihrem Namen.

// sField ist ein Feld der Struktur.
type sField struct {
	Key, Label string
	Type       metamodel.FieldType
	Required   bool
	Options    []metamodel.Option // feste Werte (Auswahl) bzw. kleine Kataloge
	Reference  string             // Verweis auf ein großes Object (Name wird zugeordnet)
	Lookup     *metamodel.Lookup
}

// sNode ist ein Objekt der Struktur.
type sNode struct {
	Name, Object, Title, Relation, Via, Hint string
	Multiple                                 bool
	Fields                                   []sField
	Children                                 []*sNode // Relationen (nur an der Wurzel)
	Embedded                                 []*sNode // über Verweisfeld
	FK                                       string   // Fremdschlüssel des Unter-Objects zur Wurzel
}

type reqLine struct {
	Line                                                    int64
	Name, Object, Parent, Relation, Via, Fields, Excl, Hint string
	Multiple                                                bool
}

// structure: Baum der Objekt-Anfrage mit den Feldern aus dem Metamodell.
func (m *Module) structure(ctx context.Context, request, cc string) (*sNode, error) {
	res, err := m.db.Query(ctx, `SELECT line_no, name, object, parent, relation, via_field, fields, exclude, hint, multiple
		FROM ai__request_object WHERE request = ? ORDER BY line_no`, request)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, crud.Invalid("Anfrage %s: keine Objekte (Abschnitt Objekte)", request)
	}
	maxValues := int(toInt(m.text(ctx, `SELECT max_values FROM ai__request WHERE code = ?`, request)))
	if maxValues <= 0 {
		maxValues = 100
	}
	byName := map[string]*sNode{}
	var lines []reqLine
	for _, r := range res.Rows {
		l := reqLine{Line: toInt(r[0]), Name: crud.Str(r[1]), Object: crud.Str(r[2]), Parent: crud.Str(r[3]), Relation: crud.Str(r[4]),
			Via: crud.Str(r[5]), Fields: crud.Str(r[6]), Excl: crud.Str(r[7]), Hint: crud.Str(r[8]), Multiple: crud.AsBool(r[9])}
		lines = append(lines, l)
	}
	var root *sNode
	defs := map[string]metamodel.ObjectDefinition{}
	for _, l := range lines {
		def, err := m.definition(ctx, l.Object)
		if err != nil {
			return nil, err
		}
		defs[l.Name] = def
		n := &sNode{Name: l.Name, Object: l.Object, Title: def.Title, Relation: l.Relation, Via: l.Via, Hint: l.Hint, Multiple: l.Multiple}
		if byName[l.Name] != nil {
			return nil, crud.Invalid("Anfrage %s: Name %s doppelt", request, l.Name)
		}
		byName[l.Name] = n
		if l.Parent == "" {
			if root != nil {
				return nil, crud.Invalid("Anfrage %s: mehr als eine Wurzel (%s, %s)", request, root.Name, l.Name)
			}
			root = n
		}
	}
	if root == nil {
		return nil, crud.Invalid("Anfrage %s: keine Wurzel (Zeile ohne übergeordnetes Objekt)", request)
	}
	// Einhängen und Felder bestimmen
	for _, l := range lines {
		n := byName[l.Name]
		skip := map[string]bool{}
		if l.Parent != "" {
			p := byName[l.Parent]
			if p == nil {
				return nil, crud.Invalid("Anfrage %s: übergeordnetes Objekt %s fehlt", request, l.Parent)
			}
			if l.Relation != "" {
				if p != root {
					return nil, crud.Invalid("Anfrage %s: Relationen nur an der Wurzel (%s)", request, l.Name)
				}
				rel := relationOf(defs[p.Name], l.Relation)
				if rel == nil || rel.Object != l.Object {
					return nil, crud.Invalid("Anfrage %s: %s hat keine Relation %s auf %s", request, p.Object, l.Relation, l.Object)
				}
				n.FK = rel.ForeignKey
				skip[rel.ForeignKey] = true
				for child := range rel.Match {
					skip[child] = true
				}
				p.Children = append(p.Children, n)
			} else {
				if fieldOf(defs[p.Name], l.Via) == nil {
					return nil, crud.Invalid("Anfrage %s: %s hat kein Feld %s", request, p.Object, l.Via)
				}
				p.Embedded = append(p.Embedded, n)
			}
		}
		n.Fields = m.fieldsOf(ctx, defs[l.Name], l, skip, cc, maxValues)
	}
	// Verweisfelder eingebetteter Objekte entfallen beim übergeordneten
	for _, n := range byName {
		for _, e := range n.Embedded {
			for i, f := range n.Fields {
				if f.Key == e.Via {
					n.Fields = append(n.Fields[:i], n.Fields[i+1:]...)
					break
				}
			}
		}
	}
	return root, nil
}

func relationOf(d metamodel.ObjectDefinition, key string) *metamodel.Relation {
	for _, s := range d.Sections {
		if s.Key == key && s.Relation != nil {
			return s.Relation
		}
	}
	return nil
}

func fieldOf(d metamodel.ObjectDefinition, key string) *metamodel.FieldDefinition {
	for i := range d.Fields {
		if d.Fields[i].Key == key {
			return &d.Fields[i]
		}
	}
	return nil
}

// definition: Metamodell eines Objects (Catalog.GetDefinition).
func (m *Module) definition(ctx context.Context, object string) (metamodel.ObjectDefinition, error) {
	resp, err := m.services.Call(ctx, sdk.ObjectCatalog, "GetDefinition", map[string]any{"object": object})
	if err != nil {
		return metamodel.ObjectDefinition{}, fmt.Errorf("Object %s: %w", object, err)
	}
	var out struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
	}
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return metamodel.ObjectDefinition{}, err
	}
	if out.Definition.Name == "" {
		return metamodel.ObjectDefinition{}, crud.Invalid("Object %s hat kein Metamodell", object)
	}
	return out.Definition, nil
}

// standardSkip: Felder, die die KI nicht füllt (Lebenszyklus, Technik).
var standardSkip = map[string]bool{"id": true, "is_active": true, "valid_to": true, "created_at": true, "created_by": true, "changed_at": true,
	"status": true, "legacy_id": true}

// fieldsOf: Felder eines Objects – die genannten oder alle bearbeitbaren;
// Auswahlwerte aus dem Metamodell, kleine Kataloge über ihr Lookup.
func (m *Module) fieldsOf(ctx context.Context, d metamodel.ObjectDefinition, l reqLine, skip map[string]bool, cc string, maxValues int) []sField {
	want := splitList(l.Fields)
	excl := map[string]bool{}
	for _, k := range splitList(l.Excl) {
		excl[k] = true
	}
	var out []sField
	add := func(f metamodel.FieldDefinition) {
		sf := sField{Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required, Options: f.Options, Lookup: f.Lookup}
		if f.Lookup != nil && len(f.Options) == 0 && f.Key != l.Via {
			opts, ok := m.lookupValues(ctx, f.Lookup, cc, maxValues)
			if ok {
				sf.Options = opts
			} else {
				sf.Reference = f.Lookup.Object
			}
		}
		out = append(out, sf)
	}
	if len(want) > 0 {
		for _, k := range want {
			if f := fieldOf(d, k); f != nil && !skip[k] {
				add(*f)
			}
		}
		return out
	}
	for _, f := range d.Fields {
		if !f.Editable || f.ActionOnly || f.Binary || f.Type == metamodel.TypeFile || f.Type == metamodel.TypePassword ||
			skip[f.Key] || excl[f.Key] || standardSkip[f.Key] {
			continue
		}
		add(f)
	}
	return out
}

// lookupValues: Werte eines Verweises, wenn es höchstens maxValues sind.
func (m *Module) lookupValues(ctx context.Context, lk *metamodel.Lookup, cc string, maxValues int) ([]metamodel.Option, bool) {
	q := map[string]any{}
	for target, src := range lk.Filters {
		switch {
		case strings.HasPrefix(src, "="):
			q[target] = strings.TrimPrefix(src, "=")
		case src == "company_code" || src == "company_code_id":
			if cc == "" {
				return nil, false
			}
			q[target] = cc
		default:
			return nil, false // hängt von einem anderen Feld ab
		}
	}
	resp, err := m.services.Call(ctx, lk.Object, "list", map[string]any{"query": q})
	if err != nil {
		return nil, false
	}
	var l struct {
		Items []map[string]any `json:"items"`
	}
	if sdk.Decode(resp.Payload, &l) != nil || len(l.Items) == 0 || len(l.Items) > maxValues {
		return nil, false
	}
	var out []metamodel.Option
	for _, it := range l.Items {
		v := crud.Str(it[lk.ValueField])
		if v == "" {
			continue
		}
		var parts []string
		for _, f := range lk.LabelFields {
			if s := crud.Str(it[f]); s != "" {
				parts = append(parts, s)
			}
		}
		out = append(out, metamodel.Option{Value: v, Label: firstOf(strings.Join(parts, " "), v)})
	}
	return out, len(out) > 0
}

// --- JSON-Schema, leere Struktur, Werte -------------------------------------------------

func fieldSchema(f sField) map[string]any {
	desc := f.Label
	s := map[string]any{}
	switch {
	case len(f.Options) > 0:
		vals := make([]string, len(f.Options))
		for i, o := range f.Options {
			vals[i] = o.Value
		}
		s["type"], s["enum"] = "string", vals
	case f.Type == metamodel.TypeNumber:
		s["type"] = "number"
	case f.Type == metamodel.TypeBoolean:
		s["type"] = "boolean"
	case f.Type == metamodel.TypeDate:
		s["type"] = "string"
		desc += " (JJJJ-MM-TT)"
	default:
		s["type"] = "string"
	}
	if f.Reference != "" {
		desc += " (Name bzw. Bezeichnung, wird zugeordnet)"
	}
	s["description"] = desc
	return s
}

func nodeSchema(n *sNode) map[string]any {
	props := map[string]any{}
	for _, f := range n.Fields {
		props[f.Key] = fieldSchema(f)
	}
	for _, e := range n.Embedded {
		props[e.Name] = itemSchema(e)
	}
	obj := map[string]any{"type": "object", "properties": props, "description": firstOf(n.Hint, n.Title)}
	return obj
}

func itemSchema(n *sNode) map[string]any {
	if n.Multiple {
		return map[string]any{"type": "array", "items": nodeSchema(n), "description": firstOf(n.Hint, n.Title)}
	}
	return nodeSchema(n)
}

// jsonSchema: Schema der ganzen Struktur (Wurzel und Relationen oben).
func jsonSchema(root *sNode) map[string]any {
	props := map[string]any{root.Name: itemSchema(root)}
	for _, c := range root.Children {
		props[c.Name] = itemSchema(c)
	}
	return map[string]any{"type": "object", "properties": props}
}

func emptyNode(n *sNode) map[string]any {
	out := map[string]any{}
	for _, f := range n.Fields {
		switch f.Type {
		case metamodel.TypeNumber:
			out[f.Key] = nil
		case metamodel.TypeBoolean:
			out[f.Key] = false
		default:
			out[f.Key] = ""
		}
	}
	for _, e := range n.Embedded {
		out[e.Name] = emptyItem(e)
	}
	return out
}

func emptyItem(n *sNode) any {
	if n.Multiple {
		return []any{emptyNode(n)}
	}
	return emptyNode(n)
}

// emptyStructure: leere Struktur (Beispiel für die KI).
func emptyStructure(root *sNode) map[string]any {
	out := map[string]any{root.Name: emptyItem(root)}
	for _, c := range root.Children {
		out[c.Name] = emptyItem(c)
	}
	return out
}

// valueLines: „partner.type kann diese Werte annehmen: PERSON = Person, …“.
func valueLines(root *sNode) []string {
	var out []string
	var walk func(prefix string, n *sNode)
	walk = func(prefix string, n *sNode) {
		p := prefix + n.Name
		for _, f := range n.Fields {
			if len(f.Options) > 0 {
				parts := make([]string, len(f.Options))
				for i, o := range f.Options {
					parts[i] = o.Value + " = " + o.Label
				}
				out = append(out, fmt.Sprintf("%s.%s (%s) kann diese Werte annehmen (Schlüssel = Beschreibung): %s", p, f.Key, f.Label, strings.Join(parts, "; ")))
			}
			if f.Reference != "" {
				out = append(out, fmt.Sprintf("%s.%s (%s): Name bzw. Bezeichnung angeben, das Programm ordnet den Datensatz zu", p, f.Key, f.Label))
			}
		}
		for _, e := range n.Embedded {
			walk(p+".", e)
		}
	}
	walk("", root)
	for _, c := range root.Children {
		walk("", c)
	}
	return out
}

// --- Ergebnis prüfen ------------------------------------------------------------------

// sanitize: nur bekannte Felder, Auswahlwerte geprüft, Typen angeglichen; notes: Verworfenes.
func sanitize(root *sNode, raw map[string]any) (map[string]any, []string) {
	var notes []string
	out := map[string]any{}
	var item func(path string, n *sNode, v any) any
	var node func(path string, n *sNode, v any) map[string]any
	node = func(path string, n *sNode, v any) map[string]any {
		in, _ := v.(map[string]any)
		rec := map[string]any{}
		for _, f := range n.Fields {
			x, ok := in[f.Key]
			if !ok || x == nil {
				continue
			}
			switch {
			case len(f.Options) > 0:
				s := strings.TrimSpace(fmt.Sprint(x))
				if s == "" {
					continue
				}
				if !hasOption(f.Options, s) {
					notes = append(notes, fmt.Sprintf("%s.%s: Wert %q steht nicht zur Auswahl", path, f.Key, s))
					continue
				}
				rec[f.Key] = s
			case f.Type == metamodel.TypeNumber:
				if n, ok := toFloat(x); ok {
					rec[f.Key] = n
				}
			case f.Type == metamodel.TypeBoolean:
				rec[f.Key] = crud.AsBool(x)
			default:
				if s := strings.TrimSpace(fmt.Sprint(x)); s != "" {
					rec[f.Key] = s
				}
			}
		}
		for _, e := range n.Embedded {
			if x, ok := in[e.Name]; ok {
				rec[e.Name] = item(path+"."+e.Name, e, x)
			}
		}
		return rec
	}
	item = func(path string, n *sNode, v any) any {
		if n.Multiple {
			l, _ := v.([]any)
			if l == nil {
				if mv, ok := v.(map[string]any); ok {
					l = []any{mv}
				}
			}
			var rows []any
			for i, x := range l {
				if r := node(fmt.Sprintf("%s[%d]", path, i), n, x); len(r) > 0 {
					rows = append(rows, r)
				}
			}
			return rows
		}
		if l, ok := v.([]any); ok && len(l) > 0 {
			v = l[0]
		}
		return node(path, n, v)
	}
	out[root.Name] = item(root.Name, root, raw[root.Name])
	for _, c := range root.Children {
		out[c.Name] = item(c.Name, c, raw[c.Name])
	}
	return out, notes
}

func hasOption(opts []metamodel.Option, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(x, " ", ""))
		if strings.Contains(s, ",") {
			s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return 0, false
}

// --- Eingabemaske ---------------------------------------------------------------------

// formKey: Feld der Maske, z. B. f__addresses__1__address__street.
func formKey(parts ...string) string { return "f__" + strings.Join(parts, "__") }

// formFields: flache Felder (mit Gruppe) und Werte aus dem Ergebnis; bei
// Listen eine leere Zeile zusätzlich. defaults: „name.feld“ → Wert (geht dem Ergebnis vor;
// bei Listen für jede Zeile).
func formFields(root *sNode, result map[string]any, defaults map[string]string) ([]metamodel.FieldDefinition, map[string]string) {
	var defs []metamodel.FieldDefinition
	values := map[string]string{}
	var node func(keyParts []string, path, group string, n *sNode, rec map[string]any, fill bool)
	node = func(keyParts []string, path, group string, n *sNode, rec map[string]any, fill bool) {
		for _, f := range n.Fields {
			k := formKey(append(keyParts, f.Key)...)
			fd := metamodel.FieldDefinition{Key: k, Label: f.Label, Type: f.Type, Group: group, Editable: true, ActionOnly: true}
			switch {
			case len(f.Options) > 0:
				fd.Type, fd.Options = metamodel.TypeSelect, f.Options
			case f.Type == metamodel.TypeSelect:
				fd.Type = metamodel.TypeText
			case f.Type == metamodel.TypeTextarea:
				fd.Type = metamodel.TypeTextarea
			}
			if f.Reference != "" {
				fd.Label += " (Schlüssel des Datensatzes)"
			}
			defs = append(defs, fd)
			// Vorgabe des Aufrufers (z. B. Buchungskreis, Lieferant) geht dem Ergebnis der KI vor.
			if v, ok := defaults[path+"."+f.Key]; ok && v != "" && fill {
				values[k] = v
			} else if x, ok := rec[f.Key]; ok && x != nil {
				values[k] = valueText(x)
			}
		}
		for _, e := range n.Embedded {
			sub, _ := rec[e.Name].(map[string]any)
			if e.Multiple {
				rows, _ := rec[e.Name].([]any)
				for i := 0; i <= len(rows); i++ {
					var r map[string]any
					if i < len(rows) {
						r, _ = rows[i].(map[string]any)
					}
					node(append(append([]string{}, keyParts...), e.Name, strconv.Itoa(i)), path+"."+e.Name, fmt.Sprintf("%s – %s %d", group, e.Title, i+1), e, r, fill && i < len(rows))
				}
				continue
			}
			node(append(append([]string{}, keyParts...), e.Name), path+"."+e.Name, group+" – "+e.Title, e, sub, fill)
		}
	}
	top := func(n *sNode) {
		if n.Multiple {
			rows, _ := result[n.Name].([]any)
			for i := 0; i <= len(rows); i++ {
				var r map[string]any
				if i < len(rows) {
					r, _ = rows[i].(map[string]any)
				}
				label := fmt.Sprintf("%s %d", n.Title, i+1)
				if i == len(rows) {
					label = n.Title + " (weitere)"
				}
				node([]string{n.Name, strconv.Itoa(i)}, n.Name, label, n, r, i < len(rows))
			}
			return
		}
		r, _ := result[n.Name].(map[string]any)
		node([]string{n.Name}, n.Name, n.Title, n, r, true)
	}
	top(root)
	for _, c := range root.Children {
		top(c)
	}
	return defs, values
}

func valueText(x any) string {
	switch v := x.(type) {
	case bool:
		if v {
			return "true"
		}
		return ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprint(x)
}

// collect: Werte der Maske → Struktur (leere Zeilen entfallen).
func collect(root *sNode, values map[string]string) map[string]any {
	var node func(keyParts []string, n *sNode) map[string]any
	node = func(keyParts []string, n *sNode) map[string]any {
		rec := map[string]any{}
		for _, f := range n.Fields {
			v := strings.TrimSpace(values[formKey(append(keyParts, f.Key)...)])
			if v == "" {
				continue
			}
			switch f.Type {
			case metamodel.TypeNumber:
				if x, ok := toFloat(v); ok {
					rec[f.Key] = x
				}
			case metamodel.TypeBoolean:
				rec[f.Key] = v == "true" || v == "on" || v == "1"
			default:
				rec[f.Key] = v
			}
		}
		for _, e := range n.Embedded {
			if e.Multiple {
				var rows []any
				for i := 0; ; i++ {
					p := append(append([]string{}, keyParts...), e.Name, strconv.Itoa(i))
					if !hasPrefix(values, formKey(p...)+"__") {
						break
					}
					if r := node(p, e); len(r) > 0 {
						rows = append(rows, r)
					}
				}
				if len(rows) > 0 {
					rec[e.Name] = rows
				}
				continue
			}
			if r := node(append(append([]string{}, keyParts...), e.Name), e); len(r) > 0 {
				rec[e.Name] = r
			}
		}
		return rec
	}
	out := map[string]any{}
	top := func(n *sNode) {
		if n.Multiple {
			var rows []any
			for i := 0; ; i++ {
				p := []string{n.Name, strconv.Itoa(i)}
				if !hasPrefix(values, formKey(p...)+"__") {
					break
				}
				if r := node(p, n); len(r) > 0 {
					rows = append(rows, r)
				}
			}
			out[n.Name] = rows
			return
		}
		out[n.Name] = node([]string{n.Name}, n)
	}
	top(root)
	for _, c := range root.Children {
		top(c)
	}
	return out
}

func hasPrefix(values map[string]string, prefix string) bool {
	for k := range values {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

// sortedKeys: für stabile Ausgaben.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// applyDefaults: Vorgaben „name.feld“ in fehlende Felder aller Zeilen (vor dem Anlegen).
func applyDefaults(root *sNode, result map[string]any, defaults map[string]string) {
	var node func(path string, n *sNode, rec map[string]any)
	node = func(path string, n *sNode, rec map[string]any) {
		for _, f := range n.Fields {
			if v, ok := defaults[path+"."+f.Key]; ok && v != "" {
				if x, has := rec[f.Key]; !has || x == nil || x == "" {
					rec[f.Key] = v
				}
			}
		}
		for _, e := range n.Embedded {
			for _, r := range rowsOf(rec[e.Name]) {
				node(path+"."+e.Name, e, r)
			}
		}
	}
	for _, n := range append([]*sNode{root}, root.Children...) {
		for _, r := range rowsOf(result[n.Name]) {
			node(n.Name, n, r)
		}
	}
}

// rowsOf: Einträge eines Objekts der Struktur (einer oder eine Liste).
func rowsOf(v any) []map[string]any {
	switch x := v.(type) {
	case map[string]any:
		return []map[string]any{x}
	case []any:
		var out []map[string]any
		for _, e := range x {
			if r, ok := e.(map[string]any); ok {
				out = append(out, r)
			}
		}
		return out
	case []map[string]any:
		return x
	}
	return nil
}
