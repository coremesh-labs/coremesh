package textmanagement

import (
	"context"
	"regexp"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

const (
	tText   = metamodel.TypeText
	tNumber = metamodel.TypeNumber
	tBool   = metamodel.TypeBoolean
	tSel    = metamodel.TypeSelect
)

var (
	codeRe     = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)
	languageRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
	objectRe   = regexp.MustCompile(`^[A-Z][A-Za-z0-9]{1,63}$`)

	cardinalities = []metamodel.Option{{Value: textservice.Single, Label: "einmal je Datensatz"},
		{Value: textservice.Multiple, Label: "mehrfach je Datensatz"}}
	refCategory = &metamodel.Lookup{Object: "TextCategory", ValueField: "code", LabelFields: []string{"name"}}
)

func sortDefault(rec crud.Record) {
	if rec["sort_order"] == nil || crud.Str(rec["sort_order"]) == "" {
		rec["sort_order"] = 0
	}
}

// category: Textkategorien (für alle Buchungskreise).
func (m *Module) category() *crud.Entity {
	return &crud.Entity{
		Object: "TextCategory", Customizing: true, Title: "Textkategorien", Icon: "icon-tag", Table: "tag__text_category", Section: "Einstellungen",
		Keys: []string{"code"}, Order: "sort_order, code", StatusField: "is_active", TitleField: "name", Search: []string{"code", "name"},
		Fields: []crud.Field{
			{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "cardinality", Label: "Häufigkeit", Type: tSel, Required: true, Listable: true, Options: cardinalities},
			{Key: "description", Label: "Beschreibung", Type: metamodel.TypeTextarea},
			{Key: "sort_order", Label: "Reihenfolge", Type: tNumber, Listable: true},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Validate: func(ctx context.Context, rec, old crud.Record) error {
			sortDefault(rec)
			if old == nil {
				rec["code"] = strings.ToUpper(strings.TrimSpace(crud.Str(rec["code"])))
				if !codeRe.MatchString(crud.Str(rec["code"])) {
					return crud.Invalid("Code %q: Großbuchstaben, Ziffern, _ (höchstens 32 Zeichen)", crud.Str(rec["code"]))
				}
				return nil
			}
			// mehrfach → einmal nur, wenn kein Datensatz eine Position > 1 hat
			if crud.Str(rec["cardinality"]) == textservice.Single && crud.Str(old["cardinality"]) != textservice.Single {
				res, err := m.db.Query(ctx, `SELECT COUNT(*) FROM tag__text WHERE category = ? AND position > 1`, old["code"])
				if err != nil {
					return err
				}
				if n := crud.Str(res.Rows[0][0]); n != "0" {
					return crud.Invalid("Kategorie %s: %s Texte stehen auf Position > 1 – erst entfernen, dann „einmal je Datensatz“", crud.Str(old["code"]), n)
				}
			}
			return nil
		},
	}
}

// language: Sprachschlüssel (z. B. de, en, zh-CN).
func (m *Module) language() *crud.Entity {
	return &crud.Entity{
		Object: "TextLanguage", Customizing: true, Title: "Sprachen", Icon: "icon-globe", Table: "tag__text_language", Section: "Einstellungen",
		Keys: []string{"code"}, Order: "sort_order, code", StatusField: "is_active", TitleField: "name", Search: []string{"code", "name"},
		Fields: []crud.Field{
			{Key: "code", Label: "Sprachschlüssel (z. B. de, en, zh-CN)", Short: "Sprache", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			{Key: "sort_order", Label: "Reihenfolge", Type: tNumber, Listable: true},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Validate: func(_ context.Context, rec, old crud.Record) error {
			sortDefault(rec)
			if old == nil {
				rec["code"] = strings.TrimSpace(crud.Str(rec["code"]))
				if !languageRe.MatchString(crud.Str(rec["code"])) {
					return crud.Invalid("Sprachschlüssel %q: z. B. de, en, zh-CN", crud.Str(rec["code"]))
				}
			}
			return nil
		},
	}
}

// assignment: Kategorie an einem Object – Pflicht oder optional, je Buchungskreis
// oder für alle (*), optional nur für Datensätze mit bestimmten Feldwerten.
func (m *Module) assignment() *crud.Entity {
	return &crud.Entity{
		Object: "TextAssignment", Customizing: true, Title: "Zuordnungen", Icon: "icon-link", Table: "tag__text_assignment", Section: "Einstellungen",
		Keys: []string{"entity_type", "company_code", "category"}, Order: "entity_type, sort_order, category, company_code",
		Filters: []string{"entity_type", "company_code", "category"},
		Fields: []crud.Field{
			{Key: "entity_type", Label: "Objekttyp (Object, z. B. Contract, RentObject)", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "company_code", Label: "Buchungskreis (* = alle)", Short: "BuKr", Type: tText, Required: true, Listable: true, Immutable: true,
				Lookup: &metamodel.Lookup{Object: "CompanyCode", ValueField: "code", LabelFields: []string{"description"}}},
			{Key: "category", Label: "Textkategorie", Short: "Kategorie", Type: tText, Required: true, Listable: true, Immutable: true, Lookup: refCategory},
			{Key: "mandatory", Label: "Pflicht", Type: tBool, Listable: true},
			{Key: "condition_field", Label: "Nur wenn Feld (optional, z. B. contract_type)", Type: tText, Listable: true},
			{Key: "condition_values", Label: "… einen dieser Werte hat (kommagetrennt)", Short: "Werte", Type: tText, Listable: true},
			{Key: "sort_order", Label: "Reihenfolge", Type: tNumber, Listable: true},
		},
		Validate: func(ctx context.Context, rec, old crud.Record) error {
			sortDefault(rec)
			if rec["mandatory"] == nil {
				rec["mandatory"] = false
			}
			rec["condition_field"] = nullable(strings.TrimSpace(crud.Str(rec["condition_field"])))
			rec["condition_values"] = nullable(strings.Join(splitValues(crud.Str(rec["condition_values"])), ","))
			if (rec["condition_field"] == nil) != (rec["condition_values"] == nil) {
				return crud.Invalid("Bedingung: Feld und Werte zusammen angeben")
			}
			if old != nil {
				return nil
			}
			rec["entity_type"] = strings.TrimSpace(crud.Str(rec["entity_type"]))
			if !objectRe.MatchString(crud.Str(rec["entity_type"])) {
				return crud.Invalid("Objekttyp %q: Name eines Objects erwartet, z. B. Contract", crud.Str(rec["entity_type"]))
			}
			if cc := strings.TrimSpace(crud.Str(rec["company_code"])); cc == "" {
				rec["company_code"] = "*"
			} else {
				rec["company_code"] = cc
			}
			rec["category"] = strings.ToUpper(strings.TrimSpace(crud.Str(rec["category"])))
			if _, err := m.categoryOf(ctx, crud.Str(rec["category"])); err != nil {
				return err
			}
			return nil
		},
	}
}

func splitValues(s string) []string {
	var out []string
	for _, v := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
