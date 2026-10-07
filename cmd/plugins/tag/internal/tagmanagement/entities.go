package tagmanagement

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/coremesh-lab/coremesh/pkg/sdk/crud"
	"github.com/coremesh-lab/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-lab/coremesh/pkg/sdk/tagservice"
)

// Verwaltung der Tag-Definitionen (Oberfläche /m/tagmanagement, Engine
// pkg/sdk/crud). Die Werte je Objekt (tag__tag_assignments) haben keine
// CRUD-Oberfläche: Sie entstehen nur über den TagService (Tags.set), der
// Datentypen, Auswahlwerte und Regeln prüft.

const (
	tText   = metamodel.TypeText
	tSelect = metamodel.TypeSelect
	tBool   = metamodel.TypeBoolean
	tNumber = metamodel.TypeNumber
	tArea   = metamodel.TypeTextarea
)

var (
	codeRe   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,39}$`)
	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

	refTagType = &crud.Ref{Table: "tag__tag_types", Column: "code", Label: "Tag", Object: "TagType", LabelFields: []string{"name"}}
	refTagSet  = &crud.Ref{Table: "tag__tag_sets", Column: "code", Label: "Tag Set", TimeSliced: true, Object: "TagSet", LabelFields: []string{"name"}}

	dataTypes = []metamodel.Option{
		{Value: string(tagservice.TypeString), Label: "Text"},
		{Value: string(tagservice.TypeInteger), Label: "Ganzzahl"},
		{Value: string(tagservice.TypeCurrency), Label: "Betrag mit Währung"},
		{Value: string(tagservice.TypeDate), Label: "Datum"},
		{Value: string(tagservice.TypeTimestamp), Label: "Zeitpunkt"},
		{Value: string(tagservice.TypeReference), Label: "Verweis auf Datensatz"},
	}
	valueModes = []metamodel.Option{
		{Value: string(tagservice.ModeFree), Label: "Freie Eingabe"},
		{Value: string(tagservice.ModeOptions), Label: "Auswahlwerte"},
	}
	ruleTypes = []metamodel.Option{
		{Value: string(tagservice.RuleRequires), Label: "Erfordert (REQUIRES)"},
		{Value: string(tagservice.RuleExcludes), Label: "Schließt aus (EXCLUDES)"},
		{Value: string(tagservice.RuleShowIf), Label: "Sichtbar wenn (SHOW_IF)"},
	}
	statuses = []metamodel.Option{{Value: "ACTIVE", Label: "Aktiv"}, {Value: "DEPRECATED", Label: "Veraltet"}}
)

func (m *Module) entities() []*crud.Entity {
	return []*crud.Entity{m.tagType(), m.valueOption(), m.tagSet(), m.setItem(), m.setRule(), m.setAssignment()}
}

func checkCode(rec crud.Record, key string) error {
	if c := crud.Str(rec[key]); !codeRe.MatchString(c) {
		return crud.Invalid("Code %q: Großbuchstaben, Ziffern und _ (2–40 Zeichen), beginnt mit einem Buchstaben", c)
	}
	return nil
}

// TagType: Definition eines Tags. Keine Zeitscheibe, sondern Status
// ACTIVE/DEPRECATED (Lebenszyklus status): Veraltete Tags erscheinen nicht
// mehr für Neu-Eingaben, Bestandswerte bleiben lesbar.
func (m *Module) tagType() *crud.Entity {
	return &crud.Entity{
		Object: "TagType", Title: "Tags", Icon: "icon-tag", Table: "tag__tag_types", Section: "Definition",
		Keys: []string{"code"}, Order: "code", Search: []string{"code", "name"},
		StatusField: "status", StatusActive: "ACTIVE", StatusInactive: "DEPRECATED",
		TitleField: "name",
		Fields: []crud.Field{
			{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "name", Label: "Name", Type: tText, Required: true, Listable: true},
			{Key: "data_type", Label: "Datentyp", Type: tSelect, Required: true, Listable: true, Immutable: true, Options: dataTypes},
			{Key: "value_mode", Label: "Werte", Type: tSelect, Required: true, Listable: true, Immutable: true, Options: valueModes},
			{Key: "ref_object", Label: "Verweis auf Object (nur Verweis, z. B. RentalObject)", Type: tText, Immutable: true},
			{Key: "translation_key", Label: "Übersetzungsschlüssel", Type: tText},
			{Key: "description", Label: "Beschreibung", Type: tArea},
			{Key: "status", Label: "Status", Type: tSelect, Listable: true, ReadOnly: true, Options: statuses},
		},
		Sections: []metamodel.SectionDefinition{
			{Key: "definition", Title: "Definition", Fields: []string{"code", "name", "data_type", "value_mode", "ref_object", "status", "translation_key", "description"}},
			{Key: "options", Title: "Auswahlwerte", Relation: &metamodel.Relation{Object: "TagValueOption", ForeignKey: "tag_type_code",
				Columns: []string{"code", "label", "translation_key", "sort_order", "valid_from", "valid_to"}}},
		},
		Validate: func(ctx context.Context, rec, old crud.Record) error {
			dt := tagservice.DataType(crud.Str(rec["data_type"]))
			if crud.Str(rec["value_mode"]) == string(tagservice.ModeOptions) && (dt == tagservice.TypeCurrency || dt == tagservice.TypeReference) {
				return crud.Invalid("Auswahlwerte gibt es für den Datentyp %s nicht", dt)
			}
			if old != nil {
				return nil
			}
			if err := m.checkRefObject(ctx, rec); err != nil {
				return err
			}
			return checkCode(rec, "code")
		},
	}
}

// TagValueOption: vordefinierter Auswahlwert mit Zeitscheibe – abgelaufene
// Werte erscheinen nicht mehr für Neu-Eingaben, Bestandsdaten bleiben gültig.
func (m *Module) valueOption() *crud.Entity {
	return &crud.Entity{
		Object: "TagValueOption", Title: "Auswahlwerte", Icon: "icon-list", Table: "tag__value_options", Section: "Definition",
		Keys: []string{"tag_type_code", "code", "valid_from"}, TimeSlice: true,
		Order: "tag_type_code, sort_order, code, valid_from", Filters: []string{"tag_type_code"}, Search: []string{"code", "label"},
		Fields: crud.WithTimeSlice(
			crud.Field{Key: "tag_type_code", Label: "Tag", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refTagType},
			crud.Field{Key: "code", Label: "Code (Wert)", Type: tText, Required: true, Listable: true, Immutable: true},
			crud.Field{Key: "label", Label: "Bezeichnung", Type: tText, Required: true, Listable: true},
			crud.Field{Key: "translation_key", Label: "Übersetzungsschlüssel", Type: tText},
			crud.Field{Key: "sort_order", Label: "Reihenfolge", Type: tNumber, Listable: true},
		),
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			t, err := m.loadType(ctx, crud.Str(rec["tag_type_code"]))
			if err != nil {
				return err
			}
			if t.ValueMode != tagservice.ModeOptions {
				return crud.Invalid("Tag %s hat freie Werte – Auswahlwerte nur bei Werte = Auswahlwerte", t.Code)
			}
			// Der Code ist der Wert: Er muss zum Datentyp passen.
			if _, err := parseValue(t, &tagservice.Value{Option: ptr(crud.Str(rec["code"]))}, true); err != nil {
				return crud.Invalid("Code %q passt nicht zum Datentyp %s: %v", crud.Str(rec["code"]), t.DataType, err)
			}
			if rec["sort_order"] == nil {
				rec["sort_order"] = int64(0)
			}
			return nil
		},
	}
}

// TagSet bündelt Tags; Zusammensetzung, Regeln und Zuordnung zu Objekttypen
// sind eingebettete Abschnitte.
func (m *Module) tagSet() *crud.Entity {
	return &crud.Entity{
		Object: "TagSet", Title: "Tag Sets", Icon: "icon-layers", Table: "tag__tag_sets", Section: "Tag Sets",
		Keys: []string{"code", "valid_from"}, TimeSlice: true, Order: "code, valid_from", Search: []string{"code", "name"},
		TitleField: "name",
		Fields: crud.WithTimeSlice(
			crud.Field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			crud.Field{Key: "name", Label: "Name", Type: tText, Required: true, Listable: true},
			crud.Field{Key: "translation_key", Label: "Übersetzungsschlüssel", Type: tText},
			crud.Field{Key: "description", Label: "Beschreibung", Type: tArea},
		),
		Sections: []metamodel.SectionDefinition{
			{Key: "definition", Title: "Definition", Fields: []string{"code", "name", "translation_key", "description"}},
			{Key: "validity", Title: "Gültigkeit", Fields: []string{"valid_from", "valid_to"}},
			{Key: "items", Title: "Tags", Relation: &metamodel.Relation{Object: "TagSetItem", ForeignKey: "tag_set_code",
				Columns: []string{"tag_type_code", "mandatory", "sort_order", "valid_from", "valid_to"}}},
			{Key: "rules", Title: "Regeln", Relation: &metamodel.Relation{Object: "TagSetRule", ForeignKey: "tag_set_code",
				Columns: []string{"rule_type", "source_tag", "condition_value", "target_tag", "valid_from", "valid_to"}}},
			{Key: "assignments", Title: "Objekttypen", Relation: &metamodel.Relation{Object: "TagSetAssignment", ForeignKey: "tag_set_code",
				Columns: []string{"entity_type", "company_code", "condition_field", "condition_values", "valid_from", "valid_to"}}},
		},
		Validate: func(_ context.Context, rec, old crud.Record) error {
			if old != nil {
				return nil
			}
			return checkCode(rec, "code")
		},
	}
}

func (m *Module) setItem() *crud.Entity {
	return &crud.Entity{
		Object: "TagSetItem", Title: "Tags im Tag Set", Icon: "icon-tag", Table: "tag__tag_set_items", Section: "Tag Sets",
		Keys: []string{"tag_set_code", "tag_type_code", "valid_from"}, TimeSlice: true,
		Order: "tag_set_code, sort_order, tag_type_code", Filters: []string{"tag_set_code", "tag_type_code"},
		Fields: crud.WithTimeSlice(
			crud.Field{Key: "tag_set_code", Label: "Tag Set", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refTagSet},
			crud.Field{Key: "tag_type_code", Label: "Tag", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refTagType},
			crud.Field{Key: "mandatory", Label: "Pflicht", Type: tBool, Listable: true},
			crud.Field{Key: "sort_order", Label: "Reihenfolge", Type: tNumber, Listable: true},
		),
		Validate: func(ctx context.Context, rec, old crud.Record) error {
			if rec["sort_order"] == nil {
				rec["sort_order"] = int64(0)
			}
			if old != nil {
				return nil
			}
			t, err := m.loadType(ctx, crud.Str(rec["tag_type_code"]))
			if err != nil {
				return err
			}
			if t.Status != "ACTIVE" {
				return crud.Invalid("Tag %s ist veraltet und kann keinem Tag Set mehr hinzugefügt werden", t.Code)
			}
			return nil
		},
	}
}

func (m *Module) setRule() *crud.Entity {
	return &crud.Entity{
		Object: "TagSetRule", Title: "Regeln", Icon: "icon-branch", Table: "tag__tag_set_rules", Section: "Tag Sets",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true,
		Order: "tag_set_code, source_tag, valid_from", Filters: []string{"tag_set_code"},
		Fields: crud.WithTimeSlice(
			crud.Field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			crud.Field{Key: "tag_set_code", Label: "Tag Set", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refTagSet},
			crud.Field{Key: "rule_type", Label: "Regel", Type: tSelect, Required: true, Listable: true, Options: ruleTypes},
			crud.Field{Key: "source_tag", Label: "Wenn Tag", Type: tText, Required: true, Listable: true, Ref: refTagType},
			crud.Field{Key: "condition_value", Label: "… den Wert hat (leer = irgendeinen)", Type: tText, Listable: true},
			crud.Field{Key: "target_tag", Label: "Dann Tag", Type: tText, Required: true, Listable: true, Ref: refTagType},
		),
		Validate: func(ctx context.Context, rec, _ crud.Record) error {
			src, dst := crud.Str(rec["source_tag"]), crud.Str(rec["target_tag"])
			if src == dst {
				return crud.Invalid("Regel: Wenn-Tag und Dann-Tag müssen verschieden sein")
			}
			// Beide Tags gehören am Beginn der Regel zum Tag Set.
			for _, tag := range []string{src, dst} {
				res, err := m.db.Query(ctx, `SELECT 1 FROM tag__tag_set_items WHERE tag_set_code = ? AND tag_type_code = ?
					AND valid_from <= ? AND valid_to >= ?`, rec["tag_set_code"], tag, rec["valid_from"], rec["valid_from"])
				if err != nil {
					return err
				}
				if len(res.Rows) == 0 {
					return crud.Invalid("Regel: Tag %s gehört am %s nicht zum Tag Set %s", tag, crud.Str(rec["valid_from"]), crud.Str(rec["tag_set_code"]))
				}
			}
			return nil
		},
	}
}

// TagSetAssignment weist einem Objekttyp (Object anderer Module, z. B.
// BusinessPartner) ein Tag Set zu – für einen Buchungskreis oder mit "*" für
// alle. Derselbe Datensatz erhält so je Buchungskreis andere Tags. Optional
// gilt die Zuordnung nur für Datensätze, deren Feld condition_field einen der
// Werte aus condition_values hat (z. B. Mietvertrag vs. Darlehensvertrag).
func (m *Module) setAssignment() *crud.Entity {
	return &crud.Entity{
		Object: "TagSetAssignment", Title: "Objekttypen", Icon: "icon-link", Table: "tag__tag_set_assignments", Section: "Tag Sets",
		Keys: []string{"entity_type", "company_code", "tag_set_code", "valid_from"}, TimeSlice: true,
		Order: "entity_type, company_code, tag_set_code, valid_from", Filters: []string{"entity_type", "company_code", "tag_set_code"},
		Fields: crud.WithTimeSlice(
			crud.Field{Key: "entity_type", Label: "Objekttyp (Object, z. B. BusinessPartner)", Type: tText, Required: true, Listable: true, Immutable: true},
			crud.Field{Key: "company_code", Label: "Buchungskreis (* = alle)", Type: tText, Required: true, Listable: true, Immutable: true,
				Lookup: &metamodel.Lookup{Object: "CompanyCode", ValueField: "code", LabelFields: []string{"description"}}},
			crud.Field{Key: "tag_set_code", Label: "Tag Set", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refTagSet},
			crud.Field{Key: "condition_field", Label: "Nur wenn Feld (optional, z. B. contract_type)", Type: tText, Listable: true},
			crud.Field{Key: "condition_values", Label: "… einen dieser Werte hat (kommagetrennt)", Type: tText, Listable: true},
		),
		Validate: func(ctx context.Context, rec, old crud.Record) error {
			if err := m.checkCondition(ctx, rec); err != nil {
				return err
			}
			if !objectRe.MatchString(strings.TrimSpace(crud.Str(rec["entity_type"]))) {
				return crud.Invalid("Objekttyp %q: Name eines Objects in PascalCase erwartet, z. B. BusinessPartner", crud.Str(rec["entity_type"]))
			}
			if rec["company_code"] == nil {
				rec["company_code"] = tagservice.AllCompanyCodes
			}
			if old == nil {
				return m.checkCompanyCode(ctx, crud.Str(rec["company_code"]))
			}
			return nil
		},
	}
}

func ptr[T any](v T) *T { return &v }

// checkRefObject: REFERENCE braucht ein Object mit Metamodell als Ziel, alle
// anderen Datentypen kein Ziel.
func (m *Module) checkRefObject(ctx context.Context, rec crud.Record) error {
	ref := strings.TrimSpace(crud.Str(rec["ref_object"]))
	if tagservice.DataType(crud.Str(rec["data_type"])) != tagservice.TypeReference {
		if ref != "" {
			return crud.Invalid("Verweis auf Object nur beim Datentyp REFERENCE")
		}
		rec["ref_object"] = nil
		return nil
	}
	if !objectRe.MatchString(ref) {
		return crud.Invalid("Verweis auf Object: Name eines Objects in PascalCase erwartet, z. B. BusinessPartner")
	}
	if _, err := m.objectDef(ctx, ref); err != nil {
		return crud.Invalid("Verweis auf Object %s: unbekannt (%v)", ref, err)
	}
	rec["ref_object"] = ref
	return nil
}

// checkCondition prüft Feld und Werte einer Bedingung gegen das Metamodell des
// Objekttyps und speichert die Werte normalisiert ("RENT,LEASE").
func (m *Module) checkCondition(ctx context.Context, rec crud.Record) error {
	field := strings.TrimSpace(crud.Str(rec["condition_field"]))
	values := splitValues(crud.Str(rec["condition_values"]))
	if field == "" {
		if len(values) > 0 {
			return crud.Invalid("Werte der Bedingung ohne Feld – Feld angeben oder Werte leeren")
		}
		rec["condition_field"], rec["condition_values"] = nil, nil
		return nil
	}
	if len(values) == 0 {
		return crud.Invalid("Bedingung auf %s: mindestens einen Wert angeben", field)
	}
	entity := strings.TrimSpace(crud.Str(rec["entity_type"]))
	def, err := m.objectDef(ctx, entity)
	if err != nil {
		return crud.Invalid("Bedingung: Objekttyp %s ist unbekannt (%v)", entity, err)
	}
	i := slices.IndexFunc(def.Fields, func(f metamodel.FieldDefinition) bool { return f.Key == field })
	if i < 0 {
		return crud.Invalid("Bedingung: %s hat kein Feld %q", entity, field)
	}
	f := def.Fields[i]
	switch {
	case f.Type == metamodel.TypeBoolean:
		for _, v := range values {
			if v != "true" && v != "false" {
				return crud.Invalid("Bedingung: %s ist ein Ja/Nein-Feld – Werte true oder false", field)
			}
		}
	case len(f.Options) > 0:
		for _, v := range values {
			if !slices.ContainsFunc(f.Options, func(o metamodel.Option) bool { return o.Value == v }) {
				return crud.Invalid("Bedingung: %q ist kein Wert von %s.%s", v, entity, field)
			}
		}
	}
	rec["condition_field"], rec["condition_values"] = field, strings.Join(values, ",")
	return nil
}
