package iam

import (
	"slices"
	"testing"
)

// Personen: Auswahl der eigenen Rollen und Buchungskreise; Wechsel über UpdateProfile,
// wirkt sofort auf Account.Check und das Profil (Menü), zurück auf alle Rollen jederzeit.
func TestPersonas(t *testing.T) {
	p, adminID := setup(t)
	ctx := as(adminID)
	for _, cc := range []string{"1000", "2000"} {
		if _, err := call(t, p, ctx, "CompanyCode", "create", map[string]any{"data": map[string]any{"code": cc}}); err != nil {
			t.Fatal(err)
		}
	}
	for name, perms := range map[string]string{"Buchhaltung": "Journal.*", "Vermietung": "Contract.*", "Privat": "Asset.*"} {
		if _, err := call(t, p, ctx, "Role", "create", map[string]any{"data": map[string]any{"name": name, "permissions": perms}}); err != nil {
			t.Fatal(err)
		}
	}
	u, err := call(t, p, ctx, "User", "create", map[string]any{"data": map[string]any{
		"username": "faust", "password": "faust-passwort-1", "roles": "Buchhaltung\nVermietung"}})
	if err != nil {
		t.Fatal(err)
	}
	uid := u["id"].(string)
	asFaust := as(uid)
	check := func(object, cc string) bool {
		t.Helper()
		m, err := call(t, p, asFaust, "Account", "Check", map[string]any{"object": object, "action": "list", "company_code": cc})
		if err != nil {
			t.Fatal(err)
		}
		return m["allowed"].(bool)
	}

	// fremde Rolle bzw. Buchungskreis wird abgewiesen
	if _, err := call(t, p, ctx, "Persona", "create", map[string]any{"data": map[string]any{"user_id": uid, "name": "Alles", "roles": "Privat"}}); err == nil {
		t.Fatal("Rolle, die der Benutzer nicht hat, angenommen")
	}
	pe, err := call(t, p, ctx, "Persona", "create", map[string]any{"data": map[string]any{"user_id": uid, "name": "Buchhaltung 1000", "roles": "Buchhaltung", "company_codes": "1000", "modules": "ledger,bank"}})
	if err != nil {
		t.Fatal(err)
	}
	pid := pe["id"].(string)
	if _, err := call(t, p, ctx, "Persona", "create", map[string]any{"data": map[string]any{"user_id": uid, "name": "Vermietung", "roles": "Vermietung", "sort_order": 2}}); err != nil {
		t.Fatal(err)
	}
	if l, _ := call(t, p, ctx, "Persona", "list", map[string]any{"query": map[string]any{"user_id": uid}}); len(l["items"].([]any)) != 2 {
		t.Fatalf("Liste: %v", l)
	}

	// ohne Person: alle Rollen
	if !check("Journal", "2000") || !check("Contract", "1000") {
		t.Fatal("ohne Person alle Rollen")
	}
	me, _ := call(t, p, asFaust, "Account", "Me", nil)
	if len(me["personas"].([]any)) != 2 || me["persona_id"] != "" {
		t.Fatalf("Profil: %v / %v", me["personas"], me["persona_id"])
	}

	// wechseln: nur Buchhaltung in 1000
	me, err = call(t, p, asFaust, "Account", "UpdateProfile", map[string]any{"persona_id": pid})
	if err != nil || me["persona_id"] != pid {
		t.Fatalf("wechseln: %v %v", err, me["persona_id"])
	}
	if !check("Journal", "1000") || check("Journal", "2000") || check("Contract", "1000") {
		t.Fatal("Person schränkt nicht ein")
	}
	if mods := me["persona_modules"].([]any); !slices.Equal(mods, []any{"ledger", "bank"}) {
		t.Fatalf("Module im Menü: %v", mods)
	}
	if me["persona_hide_settings"] != true {
		t.Fatal("Einstellungen sollten ohne settings_menu ausgeblendet sein")
	}
	if perms := me["permissions"].([]any); !slices.Equal(perms, []any{"Journal.*@1000"}) {
		t.Fatalf("Rechte im Profil (Menü): %v", perms)
	}
	// Person eines anderen Benutzers nicht wählbar
	if _, err := call(t, p, ctx, "Account", "UpdateProfile", map[string]any{"persona_id": pid}); err == nil {
		t.Fatal("fremde Person gewählt")
	}
	// Rolle beim Benutzer entzogen → wirkt auch in der Person nicht mehr
	if _, err := call(t, p, ctx, "User", "update", map[string]any{"id": uid, "data": map[string]any{"username": "faust", "roles": "Vermietung"}}); err != nil {
		t.Fatal(err)
	}
	if check("Journal", "1000") {
		t.Fatal("entzogene Rolle wirkt über die Person")
	}
	// zurück: alle Rollen
	if _, err := call(t, p, asFaust, "Account", "UpdateProfile", map[string]any{"persona_id": ""}); err != nil {
		t.Fatal(err)
	}
	if !check("Contract", "2000") {
		t.Fatal("zurück auf alle Rollen")
	}
	// gewählte Person inaktiviert → wieder alle Rollen
	_, _ = call(t, p, asFaust, "Account", "UpdateProfile", map[string]any{"persona_id": pid})
	if _, err := call(t, p, ctx, "Persona", "deactivate", map[string]any{"id": pid}); err != nil {
		t.Fatal(err)
	}
	if me, _ := call(t, p, asFaust, "Account", "Me", nil); me["persona_id"] != "" || len(me["personas"].([]any)) != 1 {
		t.Fatalf("nach Inaktivieren: %v %v", me["persona_id"], me["personas"])
	}
}
