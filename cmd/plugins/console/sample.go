package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Beispieldateien aus dem Metamodell (Catalog.GetDefinition).
//
// Sie dienen als Vorlage zum Anlegen von Datensätzen; deshalb enthalten sie
// nur bearbeitbare Felder (Editable). Reihenfolge = Reihenfolge im Metamodell.

const (
	FormatYAML = "yaml" // Standard
	FormatJSON = "json"
	FormatCSV  = "csv"
)

// sampleField ist ein Feld mit Beispielwert.
type sampleField struct {
	Def   metamodel.FieldDefinition
	Value any
}

// sampleValue erzeugt den Beispielwert eines Felds.
func sampleValue(f metamodel.FieldDefinition) any {
	switch f.Type {
	case metamodel.TypeEmail:
		return "max.mustermann@example.com"
	case metamodel.TypeNumber:
		return 100
	case metamodel.TypeDate:
		return "2026-01-01"
	case metamodel.TypeSelect:
		if len(f.Options) > 0 {
			return f.Options[0].Value
		}
		return ""
	case metamodel.TypeBoolean:
		return true
	case metamodel.TypePassword:
		return "Bitte-aendern-123"
	default: // text, textarea
		return "Beispiel " + f.Label
	}
}

func sampleFields(d metamodel.ObjectDefinition) []sampleField {
	var out []sampleField
	for _, f := range d.Fields {
		if f.Editable && !f.ActionOnly { // Aktionsfelder sind keine Daten des Objects
			out = append(out, sampleField{Def: f, Value: sampleValue(f)})
		}
	}
	return out
}

// renderSample erzeugt die Datei im gewünschten Format (leer = YAML).
func renderSample(d metamodel.ObjectDefinition, format string) (content []byte, fileFormat, contentType string, err error) {
	fields := sampleFields(d)
	if len(fields) == 0 {
		return nil, "", "", fmt.Errorf("%w: %s hat keine bearbeitbaren Felder", sdk.ErrFailedPrecondition, d.Name)
	}
	switch strings.ToLower(format) {
	case "", FormatYAML, "yml":
		b, err := sampleYAML(d, fields)
		return b, FormatYAML, "application/yaml", err
	case FormatJSON:
		b, err := sampleJSON(fields)
		return b, FormatJSON, "application/json", err
	case FormatCSV:
		b, err := sampleCSV(fields)
		return b, FormatCSV, "text/csv", err
	}
	return nil, "", "", fmt.Errorf("%w: Format %q (erlaubt: yaml, json, csv)", sdk.ErrInvalidArgument, format)
}

// --- YAML mit Kommentaren ------------------------------------------------------

var typeLabels = map[metamodel.FieldType]string{
	metamodel.TypeText:     "Text",
	metamodel.TypeTextarea: "Text, mehrzeilig",
	metamodel.TypeEmail:    "E-Mail-Adresse",
	metamodel.TypeNumber:   "Zahl",
	metamodel.TypeDate:     "Datum, Format JJJJ-MM-TT",
	metamodel.TypeSelect:   "Auswahl",
	metamodel.TypeBoolean:  "Ja/Nein (true oder false)",
	metamodel.TypePassword: "Passwort",
}

// fieldComment beschreibt ein Feld als Kopfkommentar:
//
//	# Firmenname
//	#   Typ: Text · Pflichtfeld
//	#   Erlaubte Werte: customer (Kunde), supplier (Lieferant)
func fieldComment(f metamodel.FieldDefinition) string {
	typ := typeLabels[f.Type]
	if typ == "" {
		typ = string(f.Type)
	}
	need := "optional"
	if f.Required {
		need = "Pflichtfeld"
	}
	lines := []string{f.Label, fmt.Sprintf("  Typ: %s · %s", typ, need)}
	if f.Type == metamodel.TypeSelect && len(f.Options) > 0 {
		opts := make([]string, len(f.Options))
		for i, o := range f.Options {
			opts[i] = fmt.Sprintf("%s (%s)", o.Value, o.Label)
		}
		lines = append(lines, "  Erlaubte Werte: "+strings.Join(opts, ", "))
	}
	return strings.Join(lines, "\n")
}

func sampleYAML(d metamodel.ObjectDefinition, fields []sampleField) ([]byte, error) {
	var required []string
	for _, f := range fields {
		if f.Def.Required {
			required = append(required, f.Def.Key)
		}
	}
	head := []string{
		fmt.Sprintf("Beispieldatei für %s (%s)", d.Title, d.Name),
		"Erzeugt aus dem Metamodell im Catalog; enthält alle bearbeitbaren Felder.",
	}
	if len(required) > 0 {
		head = append(head, "Pflichtfelder: "+strings.Join(required, ", "))
	}

	m := &yaml.Node{Kind: yaml.MappingNode, HeadComment: strings.Join(head, "\n")}
	for _, f := range fields {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.Def.Key, HeadComment: fieldComment(f.Def)}
		m.Content = append(m.Content, key, yamlScalar(f.Value))
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// yamlScalar setzt den Typ explizit, damit z. B. "2026-01-01" ein String
// bleibt (und gequotet wird) statt als Zeitstempel gelesen zu werden.
func yamlScalar(v any) *yaml.Node {
	switch v := v.(type) {
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(v)}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v)}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprint(v)}
	}
}

// --- JSON und CSV ---------------------------------------------------------------

// sampleJSON schreibt ein Objekt mit Feldern in Metamodell-Reihenfolge.
func sampleJSON(fields []sampleField) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, f := range fields {
		k, _ := json.Marshal(f.Def.Key)
		v, err := json.Marshal(f.Value)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf.WriteString(",")
		}
		buf.Write(k)
		buf.WriteString(":")
		buf.Write(v)
	}
	buf.WriteString("}")
	var out bytes.Buffer
	if err := json.Indent(&out, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteString("\n")
	return out.Bytes(), nil
}

// sampleCSV: Zeile 1 die Feld-Keys, Zeile 2 die Beispielwerte.
func sampleCSV(fields []sampleField) ([]byte, error) {
	header := make([]string, len(fields))
	row := make([]string, len(fields))
	for i, f := range fields {
		header[i] = f.Def.Key
		row[i] = fmt.Sprint(f.Value)
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(header)
	w.Write(row)
	w.Flush()
	return buf.Bytes(), w.Error()
}

// sampleFilename: BusinessPartner + yaml → business_partner_sample.yaml
func sampleFilename(object, format string) string {
	var b strings.Builder
	for i, r := range object {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String() + "_sample." + format
}
