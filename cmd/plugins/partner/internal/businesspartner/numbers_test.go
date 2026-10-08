package businesspartner

import (
	"context"
	"strings"
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// TestPartnerNumbers: BP-Nummer aus dem Intervall der Partnergruppe (intern
// fortlaufend, extern eingegeben und geprüft), Kurzname mit Vorschlag und
// Eindeutigkeit.
func TestPartnerNumbers(t *testing.T) {
	e := setup(t)
	create := func(kv ...any) (map[string]any, error) {
		return e.do("BusinessPartner", "create", data(append([]any{"type", "PERSON"}, kv...)...))
	}
	a := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Müller", "name2", "Anna"))
	b := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Müller", "name2", "Bernd"))
	if a["id"] != "100000" || b["id"] != "100001" {
		t.Fatalf("intern fortlaufend: %v %v", a["id"], b["id"])
	}
	if a["search_term"] != "MUELLER" || b["search_term"] != "MUELLER2" {
		t.Fatalf("Kurzname-Vorschlag: %v %v", a["search_term"], b["search_term"])
	}
	// Eingegebener Kurzname vergeben: abgelehnt; frei: übernommen (groß)
	_, err := create("name1", "Müller", "search_term", "mueller")
	expect(t, err, sdk.ErrInvalidArgument, "Kurzname doppelt")
	if !strings.Contains(err.Error(), "100000") {
		t.Fatalf("Meldung nennt den Partner nicht: %v", err)
	}
	c := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Müller", "search_term", "mueller-c"))
	if c["search_term"] != "MUELLER-C" {
		t.Fatalf("Kurzname: %v", c["search_term"])
	}
	_, err = e.do("BusinessPartner", "update", map[string]any{"id": c["_id"], "data": row("search_term", "MUELLER")})
	expect(t, err, sdk.ErrInvalidArgument, "Kurzname ändern auf vergebenen")
	e.must("BusinessPartner", "update", map[string]any{"id": c["_id"], "data": row("search_term", "MUELLER-CLARA")})
	// Interne Gruppe: Nummer eingeben nicht erlaubt
	_, err = create("name1", "X", "id", "123456")
	expect(t, err, sdk.ErrInvalidArgument, "Nummer bei interner Vergabe")
	// BP-Nummer ist fest
	_, err = e.do("BusinessPartner", "update", map[string]any{"id": a["_id"], "data": row("id", "100099")})
	expect(t, err, sdk.ErrInvalidArgument, "BP-Nummer ändern")

	// Externe Gruppe: sprechender Schlüssel, geprüft und frei
	e.must("PartnerGroup", "create", data("code", "ext", "description", "Eigene Schlüssel"))
	e.h.ranges["EXT"] = &fakeRange{external: true, pattern: "[A-Z][A-Z0-9-]{2,11}"}
	x := e.must("BusinessPartner", "create", data("type", "PERSON", "name1", "Schmidt", "group_code", "EXT", "id", "schmidt-h"))
	if x["id"] != "SCHMIDT-H" || x["group_code"] != "EXT" {
		t.Fatalf("extern: %v", x)
	}
	_, err = create("name1", "Schmidt", "group_code", "EXT", "id", "SCHMIDT-H")
	expect(t, err, sdk.ErrInvalidArgument, "extern vergeben")
	if !strings.Contains(err.Error(), "bereits vergeben") {
		t.Fatalf("Meldung: %v", err)
	}
	_, err = create("name1", "Schmidt", "group_code", "EXT")
	expect(t, err, sdk.ErrInvalidArgument, "extern ohne Nummer")
	_, err = create("name1", "Schmidt", "group_code", "GIBTSNICHT")
	expect(t, err, sdk.ErrInvalidArgument, "unbekannte Gruppe")

	// Maske: Nummer nur bei externer Vergabe, Kurzname-Vorschlag
	state := func(values map[string]string) metamodel.FormState {
		resp, err := e.p.Handle(e.ctx, sdk.Request{Object: "BusinessPartner", Action: "formState",
			Payload: metamodel.FormStateRequest{Mode: "create", Values: values}})
		if err != nil {
			t.Fatal(err)
		}
		return resp.Payload.(metamodel.FormState)
	}
	st := state(map[string]string{"name1": "Schmidt"})
	if v := st.Fields["id"].Visible; v == nil || *v {
		t.Fatalf("intern: Nummer ausgeblendet erwartet: %+v", st.Fields["id"])
	}
	if v := st.Fields["search_term"].Value; v == nil || *v != "SCHMIDT2" { // SCHMIDT ist vergeben
		t.Fatalf("Vorschlag: %+v", st.Fields["search_term"])
	}
	if v := st.Fields["group_code"].Value; v == nil || *v != "STD" {
		t.Fatalf("Standardgruppe: %+v", st.Fields["group_code"])
	}
	st = state(map[string]string{"group_code": "EXT"})
	if r := st.Fields["id"].Required; r == nil || !*r || !strings.Contains(st.Message, "[A-Z]") {
		t.Fatalf("extern: %+v %s", st.Fields["id"], st.Message)
	}
}

// TestMigrateLegacyIDs: Partner mit GUID (bis 0.9.0) bekommen eine BP-Nummer
// der Standardgruppe; Verweise im Modul folgen, die GUID bleibt in legacy_id,
// rekey-Event und resolve für andere Module.
func TestMigrateLegacyIDs(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	old := "7cb54679998db9f43b1fcf9c82da68f3"
	for _, q := range []string{
		`INSERT INTO partner__bp (id, type, name1, search_term, valid_from, valid_to) VALUES ('` + old + `', 'PERSON', 'Müller', 'MÜLLER', '2026-01-01', '9999-12-31')`,
		`INSERT INTO partner__roles (bp_id, role_code, valid_from, valid_to) VALUES ('` + old + `', 'DEBITOR', '2026-01-01', '9999-12-31')`,
		`INSERT INTO partner__company_codes (bp_id, company_code, role_code, reconciliation_account) VALUES ('` + old + `', '1000', 'DEBITOR', '1200')`,
	} {
		if _, err := e.h.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	got := e.items("BusinessPartner", map[string]any{})
	if len(got) != 1 || got[0]["id"] != "100000" || got[0]["legacy_id"] != old || got[0]["group_code"] != "STD" {
		t.Fatalf("migriert: %v", got)
	}
	if cc := e.items(ccObject, map[string]any{"bp_id": "100000"}); len(cc) != 1 {
		t.Fatalf("Buchungskreisdaten umgestellt: %v", cc)
	}
	if roles := e.items("PartnerRole", map[string]any{"bp_id": "100000"}); len(roles) != 1 {
		t.Fatalf("Rollen umgestellt: %v", roles)
	}
	if len(e.h.events) != 1 || e.h.events[0]["action"] != "rekey" {
		t.Fatalf("rekey-Event: %v", e.h.events)
	}
	res := e.must(serviceObject, actionResolve, map[string]any{"ids": []any{old, "100000", "unbekannt"}})
	if ids := res["ids"].(map[string]string); len(ids) != 1 || ids[old] != "100000" {
		t.Fatalf("resolve: %v", res)
	}
	// Ein zweiter Lauf ändert nichts (Migration einmal je Prozess, wiederholbar)
	e.must("BusinessPartner", "update", map[string]any{"id": got[0]["_id"], "data": row("name2", "Anna")})
}
