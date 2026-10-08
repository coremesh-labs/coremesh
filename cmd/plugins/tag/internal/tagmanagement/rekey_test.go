package tagmanagement

import (
	"testing"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// TestRekey: SystemEvent <Object>.rekey stellt Zuordnungen (id und
// id|Zeitscheibe) und Verweise in Tags mit diesem Object um.
func TestRekey(t *testing.T) {
	e := setup(t)
	db := e.h.db
	for _, q := range []string{
		`INSERT INTO tag__tag_types (code, name, data_type, value_mode, ref_object) VALUES ('BETREUER', 'Betreuer', 'REF', 'SINGLE', 'BusinessPartner')`,
		`INSERT INTO tag__tag_assignments (id, target_entity_type, target_entity_id, company_code, tag_type_code, value_string, valid_from, valid_to, changed_at)
		 VALUES ('a1', 'BusinessPartner', 'GUID1|1900-01-01', '*', 'X', 'v', '2026-01-01', '9999-12-31', '2026-10-08')`,
		`INSERT INTO tag__tag_assignments (id, target_entity_type, target_entity_id, company_code, tag_type_code, value_string, valid_from, valid_to, changed_at)
		 VALUES ('a2', 'BusinessPartner', 'GUID1', '*', 'X', 'v', '2026-01-01', '9999-12-31', '2026-10-08')`,
		`INSERT INTO tag__tag_assignments (id, target_entity_type, target_entity_id, company_code, tag_type_code, value_string, valid_from, valid_to, changed_at)
		 VALUES ('a3', 'BusinessPartner', 'GUID10|1900-01-01', '*', 'X', 'v', '2026-01-01', '9999-12-31', '2026-10-08')`,
		`INSERT INTO tag__tag_assignments (id, target_entity_type, target_entity_id, company_code, tag_type_code, value_ref, valid_from, valid_to, changed_at)
		 VALUES ('a4', 'RentObject', 'WG1', '*', 'BETREUER', 'GUID1', '2026-01-01', '9999-12-31', '2026-10-08')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ev := events.Event{Object: "BusinessPartner", Action: "rekey", Data: map[string]any{"old_id": "GUID1", "new_id": "100000"}}
	if _, err := e.p.Handle(e.ctx, sdk.Request{Object: tagservice.Object, Action: events.CallbackAction, Payload: map[string]any{"event": ev}}); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"a1": "100000|1900-01-01", "a2": "100000", "a3": "GUID10|1900-01-01", "a4": "WG1"} {
		var got string
		if err := db.QueryRow(`SELECT target_entity_id FROM tag__tag_assignments WHERE id = ?`, id).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: %q (%v), erwartet %q", id, got, err, want)
		}
	}
	var ref string
	if err := db.QueryRow(`SELECT value_ref FROM tag__tag_assignments WHERE id = 'a4'`).Scan(&ref); err != nil || ref != "100000" {
		t.Fatalf("value_ref: %q %v", ref, err)
	}
}
