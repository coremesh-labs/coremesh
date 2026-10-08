package businesspartner

import (
	"context"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Anreden (Katalog PartnerSalutation): Bezeichnung, Vorlage der Briefanrede,
// für welche Art Partner und welches Geschlecht sie gilt. Ohne Anrede am
// Partner schlägt die Pflege die erste passende vor (Art, Geschlecht).
// Platzhalter der Vorlage: {name1} (Nachname/Firma), {name2} (Vorname/Zusatz).

var (
	personTypeOptions = []metamodel.Option{{Value: "PERSON", Label: "natürliche Person"}, {Value: "ORGANIZATION", Label: "juristische Person (Organisation)"}}
	genderOptions     = []metamodel.Option{
		{Value: "FEMALE", Label: "weiblich"}, {Value: "MALE", Label: "männlich"}, {Value: "DIVERSE", Label: "divers"}, {Value: "UNKNOWN", Label: "unbekannt"}}
	showPerson = &metamodel.Condition{Field: "type", Values: []string{"PERSON"}}
)

func (m *Module) salutationCatalog() *entity {
	return &entity{
		Object: "PartnerSalutation", Section: "Kataloge", Title: "Anreden", Icon: "icon-tag", Table: "partner__salutations",
		Keys: []string{"code"}, Order: "sort_order, code", Search: []string{"code", "description"}, StatusField: "is_active", TitleField: "description",
		Fields: []field{
			{Key: "code", Label: "Code", Type: tText, Required: true, Listable: true, Immutable: true},
			{Key: "description", Label: "Anrede (z. B. Herr, Frau, Firma)", Type: tText, Required: true, Listable: true},
			{Key: "letter_text", Label: "Briefanrede ({name1} = Nachname/Firma, {name2} = Vorname)", Type: tText, Required: true, Listable: true},
			{Key: "person_type", Label: "Für Art (leer = alle)", Type: tSelect, Listable: true, Options: personTypeOptions},
			{Key: "gender", Label: "Für Geschlecht (leer = alle; Vorschlag)", Type: tSelect, Listable: true, Options: genderOptions},
			{Key: "sort_order", Label: "Reihenfolge", Type: metamodel.TypeNumber},
			{Key: "is_active", Label: "Aktiv", Type: tBool, Listable: true, ReadOnly: true},
		},
		Validate: func(_ context.Context, rec, _ record) error {
			rec["code"] = strings.ToUpper(strings.TrimSpace(str(rec["code"])))
			for _, k := range []string{"person_type", "gender"} {
				if str(rec[k]) == "" {
					rec[k] = nil
				}
			}
			if rec["sort_order"] == nil || str(rec["sort_order"]) == "" {
				rec["sort_order"] = 0
			}
			return nil
		},
	}
}

// checkPerson: Geschlecht nur bei natürlichen Personen; Anrede passend zur Art,
// ohne Anrede der Vorschlag aus dem Katalog.
func (m *Module) checkPerson(ctx context.Context, rec record) error {
	typ := str(rec["type"])
	if typ != "PERSON" {
		rec["gender"] = nil
	} else if str(rec["gender"]) == "" {
		rec["gender"] = nil
	}
	code := strings.ToUpper(strings.TrimSpace(str(rec["salutation_code"])))
	if code == "" {
		res, err := m.db.Query(ctx, `SELECT code FROM partner__salutations WHERE is_active = ? AND (person_type IS NULL OR person_type = ?)
			AND (gender IS NULL OR gender = ?) ORDER BY CASE WHEN gender IS NULL THEN 1 ELSE 0 END, sort_order, code LIMIT 1`, true, typ, str(rec["gender"]))
		if err != nil {
			return err
		}
		if len(res.Rows) > 0 && (typ == "ORGANIZATION" || str(rec["gender"]) != "") {
			code = str(res.Rows[0][0])
		}
	}
	if code == "" {
		rec["salutation_code"] = nil
		return nil
	}
	res, err := m.db.Query(ctx, `SELECT person_type FROM partner__salutations WHERE code = ? AND is_active = ?`, code, true)
	if err != nil {
		return err
	}
	if len(res.Rows) == 0 {
		return invalid("Anrede %q gibt es nicht (Katalog Anreden)", code)
	}
	if pt := str(res.Rows[0][0]); pt != "" && pt != typ {
		return invalid("Anrede %s gilt nicht für %s", code, typeLabel(typ))
	}
	rec["salutation_code"] = code
	return nil
}

func typeLabel(t string) string {
	for _, o := range personTypeOptions {
		if o.Value == t {
			return o.Label
		}
	}
	return t
}

// decoratePerson: Briefanrede aus der Vorlage der Anrede.
func (m *Module) decoratePerson(ctx context.Context, rec record) error {
	code := str(rec["salutation_code"])
	if code == "" {
		return nil
	}
	res, err := m.db.Query(ctx, `SELECT letter_text, description FROM partner__salutations WHERE code = ?`, code)
	if err != nil || len(res.Rows) == 0 {
		return err
	}
	text := strings.NewReplacer("{name1}", str(rec["name1"]), "{name2}", str(rec["name2"])).Replace(str(res.Rows[0][0]))
	rec["letter_salutation"] = strings.Join(strings.Fields(text), " ")
	return nil
}
