package businesspartner

import (
	"context"
	"strings"

	"github.com/camel/coremesh/pkg/sdk/metamodel"
)

const (
	tText   = metamodel.TypeText
	tDate   = metamodel.TypeDate
	tBool   = metamodel.TypeBoolean
	tSelect = metamodel.TypeSelect
)

// Zeitscheiben-Felder für alle zeitabhängigen Entitäten.
var timeSliceFields = []field{
	{Key: "valid_from", Label: "Gültig ab", Type: tDate, Listable: true},
	{Key: "valid_to", Label: "Gültig bis", Type: tDate, Listable: true},
}

func withTimeSlice(fs ...field) []field { return append(fs, timeSliceFields...) }

var (
	refBP          = &ref{Table: "partner__bp", Column: "id", Label: "Geschäftspartner"}
	refAddress     = &ref{Table: "partner__addresses", Column: "id", Label: "Adresse"}
	refAddressRole = &ref{Table: "partner__address_roles", Column: "code", Label: "Adressrolle", TimeSliced: true}
	refCommCat     = &ref{Table: "partner__comm_categories", Column: "code", Label: "Kommunikationskategorie"}
	refCommType    = &ref{Table: "partner__comm_types", Column: "code", Label: "Kommunikationstyp", TimeSliced: true}
	refRoleType    = &ref{Table: "partner__role_types", Column: "code", Label: "Rolle", TimeSliced: true}
)

// entities in der Reihenfolge der Registrierung (Navigation).
func (m *Module) entities() []*entity {
	return []*entity{
		m.businessPartner(), m.partnerRole(), m.partnerCompanyCode(),
		m.address(), m.partnerAddress(), m.partnerContact(), m.partnerBankDetail(),
		m.addressRoleCatalog(), m.commCategoryCatalog(), m.commTypeCatalog(), m.roleTypeCatalog(),
	}
}

// --- Kataloge ----------------------------------------------------------------------

func (m *Module) addressRoleCatalog() *entity {
	return &entity{
		Object: "PartnerAddressRole", Section: "Kataloge", Title: "Adressrollen", Icon: "icon-tag", Table: "partner__address_roles",
		Keys: []string{"code"}, TimeSlice: true, Order: "code",
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_main", Label: "Hauptanschrift", Type: tBool, Listable: true},
		),
		// Nur eine Adressrolle darf die Hauptanschrift kennzeichnen.
		validate: func(ctx context.Context, rec, _ record) error {
			if !asBool(rec["is_main"]) {
				return nil
			}
			return m.uniqueMain(ctx, "partner__address_roles", "code", str(rec["code"]), "", nil, "Hauptanschrift")
		},
		beforeDelete: func(ctx context.Context, rec record) error {
			return m.inUse(ctx, "Adressrolle", str(rec["code"]), [2]string{"partner__bp_addresses", "address_role_code"})
		},
	}
}

func (m *Module) commCategoryCatalog() *entity {
	return &entity{
		Object: "PartnerCommCategory", Section: "Kataloge", Title: "Kommunikationskategorien", Icon: "icon-tag", Table: "partner__comm_categories",
		Keys: []string{"code"}, Order: "code",
		Fields: []field{
			{Key: "code", Label: "Code (maschinenlesbar: EMAIL, PHONE, FAX, WEB …)", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
		},
		beforeDelete: func(ctx context.Context, rec record) error {
			return m.inUse(ctx, "Kommunikationskategorie", str(rec["code"]), [2]string{"partner__comm_types", "category_code"})
		},
	}
}

func (m *Module) commTypeCatalog() *entity {
	return &entity{
		Object: "PartnerCommType", Section: "Kataloge", Title: "Kommunikationstypen", Icon: "icon-tag", Table: "partner__comm_types",
		Keys: []string{"code"}, TimeSlice: true, Order: "category_code, code", Filters: []string{"category_code"},
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "category_code", Label: "Kategorie", Type: tText, Required: true, Listable: true, Ref: refCommCat},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_main", Label: "Haupttyp der Kategorie", Type: tBool, Listable: true},
		),
		// Pro Kategorie höchstens ein Haupt-Kommunikationstyp.
		validate: func(ctx context.Context, rec, _ record) error {
			if !asBool(rec["is_main"]) {
				return nil
			}
			return m.uniqueMain(ctx, "partner__comm_types", "code", str(rec["code"]), "category_code", rec["category_code"], "Haupttyp der Kategorie "+str(rec["category_code"]))
		},
		beforeDelete: func(ctx context.Context, rec record) error {
			return m.inUse(ctx, "Kommunikationstyp", str(rec["code"]), [2]string{"partner__contacts", "comm_type_code"})
		},
	}
}

func (m *Module) roleTypeCatalog() *entity {
	return &entity{
		Object: "PartnerRoleType", Section: "Kataloge", Title: "Rollentypen", Icon: "icon-tag", Table: "partner__role_types",
		Keys: []string{"code"}, TimeSlice: true, Order: "code",
		Fields: withTimeSlice(
			field{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			field{Key: "description", Label: "Beschreibung", Type: tText, Required: true, Listable: true},
			field{Key: "is_debitor", Label: "Debitor (Finanzrolle)", Type: tBool, Listable: true},
			field{Key: "is_creditor", Label: "Kreditor (Finanzrolle)", Type: tBool, Listable: true},
		),
		beforeDelete: func(ctx context.Context, rec record) error {
			return m.inUse(ctx, "Rolle", str(rec["code"]),
				[2]string{"partner__roles", "role_code"}, [2]string{"partner__company_codes", "role_code"})
		},
	}
}

// uniqueMain: Kein anderer Eintrag (gleiche Gruppe) darf is_main tragen.
func (m *Module) uniqueMain(ctx context.Context, table, keyCol, key, groupCol string, group any, what string) error {
	sql := "SELECT " + keyCol + " FROM " + table + " WHERE is_main = ? AND " + keyCol + " <> ?"
	args := []any{true, key}
	if groupCol != "" {
		sql += " AND " + groupCol + " = ?"
		args = append(args, group)
	}
	res, err := m.db.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		return invalid("%s ist bereits %q – erst dort entfernen", what, str(res.Rows[0][0]))
	}
	return nil
}

// --- Geschäftspartner --------------------------------------------------------------

func (m *Module) businessPartner() *entity {
	return &entity{
		Object: "BusinessPartner", Title: "Geschäftspartner", Icon: "icon-users", Table: "partner__bp",
		Keys: []string{"id"}, Surrogate: true, Order: "search_term, name1",
		Search: []string{"name1", "name2", "search_term"},
		Fields: []field{
			{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			{Key: "type", Label: "Art", Type: tSelect, Required: true, Listable: true, Options: []metamodel.Option{
				{Value: "ORGANIZATION", Label: "Organisation"}, {Value: "PERSON", Label: "Person"}}},
			{Key: "name1", Label: "Name 1 (Firma / Nachname)", Type: tText, Required: true, Listable: true},
			{Key: "name2", Label: "Name 2 (Vorname / Zusatz)", Type: tText, Listable: true},
			{Key: "search_term", Label: "Suchbegriff", Type: tText, Listable: true},
			{Key: "is_blocked", Label: "Gesperrt", Type: tBool, Listable: true},
		},
		validate: func(_ context.Context, rec, _ record) error {
			if rec["search_term"] == nil { // Matchcode aus Name 1
				rec["search_term"] = strings.ToUpper(str(rec["name1"]))
			}
			return nil
		},
		// Löschen entfernt alle Beziehungen des Partners (Adressen selbst bleiben,
		// sie sind wiederverwendbar). Buchungskreisdaten nur mit Zugriff darauf.
		beforeDelete: func(ctx context.Context, rec record) error {
			id := str(rec["id"])
			res, err := m.db.Query(ctx, "SELECT company_code FROM partner__company_codes WHERE bp_id = ?", id)
			if err != nil {
				return err
			}
			for _, r := range res.Rows {
				if err := requireCompanyCode(ctx, "delete", str(r[0])); err != nil {
					return err
				}
			}
			for _, t := range []string{"partner__company_codes", "partner__roles", "partner__bp_addresses", "partner__contacts", "partner__bank_details"} {
				if _, err := m.db.Exec(ctx, "DELETE FROM "+t+" WHERE bp_id = ?", id); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func (m *Module) address() *entity {
	return &entity{
		Object: "PartnerAddressData", Title: "Adressen", Icon: "icon-map", Table: "partner__addresses",
		Keys: []string{"id"}, Surrogate: true, Order: "country, zip_code, street", Search: []string{"street", "city", "zip_code"},
		Fields: []field{
			{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			{Key: "street", Label: "Straße", Type: tText, Required: true, Listable: true},
			{Key: "house_no", Label: "Hausnummer", Type: tText, Listable: true},
			{Key: "zip_code", Label: "PLZ", Type: tText, Required: true, Listable: true},
			{Key: "city", Label: "Ort", Type: tText, Required: true, Listable: true},
			{Key: "country", Label: "Land (ISO-2)", Type: tText, Required: true, Listable: true},
		},
		validate: func(_ context.Context, rec, _ record) error {
			c, err := normalizeCountry(str(rec["country"]))
			rec["country"] = c
			return err
		},
		beforeDelete: func(ctx context.Context, rec record) error {
			return m.inUse(ctx, "Adresse", str(rec["id"]), [2]string{"partner__bp_addresses", "address_id"})
		},
	}
}

func (m *Module) partnerAddress() *entity {
	return &entity{
		Object: "PartnerAddress", Title: "Partner-Adressen", Icon: "icon-map", Table: "partner__bp_addresses",
		Keys: []string{"id"}, Surrogate: true, TimeSlice: true, Order: "bp_id, address_role_code, valid_from",
		Filters: []string{"bp_id", "address_id", "address_role_code"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "address_id", Label: "Adresse (ID)", Type: tText, Required: true, Listable: true, Ref: refAddress},
			field{Key: "address_role_code", Label: "Adressrolle", Type: tText, Required: true, Listable: true, Ref: refAddressRole},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
	}
}

func (m *Module) partnerContact() *entity {
	return &entity{
		Object: "PartnerContact", Title: "Kommunikation", Icon: "icon-phone", Table: "partner__contacts",
		Keys: []string{"id"}, Surrogate: true, TimeSlice: true, Order: "bp_id, comm_type_code, valid_from",
		Filters: []string{"bp_id", "comm_type_code"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "comm_type_code", Label: "Kommunikationstyp", Type: tText, Required: true, Listable: true, Ref: refCommType},
			field{Key: "value", Label: "Wert", Type: tText, Required: true, Listable: true},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
		// Prüfung des Werts anhand der Kategorie des Kommunikationstyps.
		validate: func(ctx context.Context, rec, _ record) error {
			res, err := m.db.Query(ctx, "SELECT category_code FROM partner__comm_types WHERE code = ?", rec["comm_type_code"])
			if err != nil {
				return err
			}
			if len(res.Rows) == 0 {
				return invalid("Kommunikationstyp %q gibt es nicht", str(rec["comm_type_code"]))
			}
			return validateContact(str(res.Rows[0][0]), str(rec["value"]))
		},
	}
}

func (m *Module) partnerBankDetail() *entity {
	return &entity{
		Object: "PartnerBankDetail", Title: "Bankverbindungen", Icon: "icon-bank", Table: "partner__bank_details",
		Keys: []string{"id"}, Surrogate: true, TimeSlice: true, Order: "bp_id, valid_from", Filters: []string{"bp_id"},
		Fields: withTimeSlice(
			field{Key: "id", Label: "ID", Type: tText, ReadOnly: true},
			field{Key: "bp_id", Label: "Geschäftspartner (ID)", Type: tText, Required: true, Listable: true, Immutable: true, Ref: refBP},
			field{Key: "iban", Label: "IBAN", Type: tText, Required: true, Listable: true},
			field{Key: "bic", Label: "BIC", Type: tText, Listable: true},
			field{Key: "bank_name", Label: "Bank", Type: tText, Listable: true},
			field{Key: "account_holder", Label: "Kontoinhaber", Type: tText},
			field{Key: "is_default", Label: "Standard", Type: tBool, Listable: true},
		),
		validate: func(_ context.Context, rec, _ record) error {
			iban, err := normalizeIBAN(str(rec["iban"]))
			if err != nil {
				return err
			}
			rec["iban"] = iban
			if rec["bic"] != nil {
				bic, err := normalizeBIC(str(rec["bic"]))
				if err != nil {
					return err
				}
				rec["bic"] = bic
			}
			return nil
		},
	}
}
