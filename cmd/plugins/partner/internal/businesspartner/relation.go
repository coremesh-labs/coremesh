package businesspartner

import (
	"context"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Beziehungen zwischen Geschäftspartnern (seit 0.18.0): z. B. Person „ist angestellt
// bei“ Firma. Die Art kommt aus dem Katalog Beziehungsarten (Bezeichnung und
// Gegenrichtung); am Partner stehen beide Richtungen.

var refRelationType = &ref{Table: "partner__relation_types", Column: "code", Label: "Beziehungsart", TimeSliced: true,
	Object: "PartnerRelationType", LabelFields: []string{"description"}}

func (m *Module) relationTypeCatalog() *entity {
	return &entity{
		Object: "PartnerRelationType", Customizing: true, Section: "Kataloge", Title: "Beziehungsarten", Icon: "icon-tag", Table: "partner__relation_types",
		Keys: []string{"code", "valid_from"}, TimeSlice: true, Order: "code", Search: []string{"code", "description", "inverse_description"},
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "description", Label: "Bezeichnung", Type: tText, Required: true, Listable: true,
				Help: "Aus Sicht des Partners, an dem die Beziehung steht, z. B. „ist angestellt bei“."},
			field{Key: "inverse_description", Label: "Gegenrichtung", Type: tText, Required: true, Listable: true,
				Help: "Aus Sicht des anderen Partners, z. B. „beschäftigt“."},
		),
	}
}

func (m *Module) partnerRelation() *entity {
	return &entity{
		Object: "PartnerRelation", Title: "Beziehungen", Icon: "icon-link", Table: "partner__relations",
		Keys: []string{"id", "valid_from"}, Surrogate: true, TimeSlice: true, Order: "bp_id, relation_type, related_bp_id, valid_from",
		Filters: []string{"bp_id", "related_bp_id", "relation_type"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Short: "Partner", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "relation_type", Label: "Beziehung", Type: tText, Required: true, Listable: true, Ref: refRelationType},
			field{Key: "related_bp_id", Label: "Zu Geschäftspartner", Short: "Zu", Type: tText, Required: true, Listable: true, Ref: refBP},
			field{Key: "note", Label: "Bemerkung", Type: tText, Listable: true, Help: "z. B. Funktion oder Abteilung"},
		),
		Validate: func(_ context.Context, rec, _ record) error {
			if str(rec["bp_id"]) == str(rec["related_bp_id"]) {
				return invalid("Ein Partner kann keine Beziehung zu sich selbst haben")
			}
			return nil
		},
	}
}

var relationSections = []metamodel.SectionDefinition{
	{Key: "beziehungen", Title: "Beziehungen", Collapsed: true, Relation: &metamodel.Relation{Object: "PartnerRelation", ForeignKey: "bp_id",
		Columns: []string{"relation_type", "related_bp_id", "note", "valid_from", "valid_to"}}},
	{Key: "beziehungen_zu", Title: "Beziehungen anderer Partner zu diesem", Collapsed: true, Relation: &metamodel.Relation{Object: "PartnerRelation", ForeignKey: "related_bp_id",
		Columns: []string{"bp_id", "relation_type", "note", "valid_from", "valid_to"}}},
}
