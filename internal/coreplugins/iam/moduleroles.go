package iam

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/docservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
	"github.com/coremesh-labs/coremesh/pkg/sdk/textservice"
)

// Modulrollen (seit 0.11.0): „Modulrollen anlegen …“ an den Rollen legt je
// gewähltem Modul zwei Rollen an bzw. ergänzt sie – abgeleitet aus dem Catalog,
// ohne feste Listen:
//
//   - Sachbearbeiter: alle Actions der Objects und Services des Moduls, die keine
//     Einstellung sind; Einstellungen (ModuleDefinition.Customizing: Kategorien,
//     Arten, Kataloge, Kontenplan, Einrichtung) nur lesen (list, get, formState).
//   - Fachverantwortlicher: zusätzlich alle Actions der Einstellungen.
//
// Beide lesen die Objects anderer Module, auf die Felder (Auswahl) und Abschnitte
// verweisen, und pflegen Merkmale, Texte und Dokumente der Datensätze, wenn das
// Modul diese Abschnitte hat. Vorhandene Berechtigungen bleiben unverändert (auch
// eingeschränkte Buchungskreise); fehlende kommen hinzu.

const setupModuleRolesAction = "setupModuleRoles"

// Rollenarten: Schlüssel, Namenszusatz und Beschreibung.
var moduleRoleKinds = []struct{ key, suffix, desc string }{
	{"clerk", "Sachbearbeiter", "Modulrolle %s: Objekte anlegen und pflegen, Einstellungen lesen"},
	{"expert", "Fachverantwortlicher", "Modulrolle %s: Objekte und Einstellungen (Kategorien, Arten, Kontenplan) pflegen"},
}

// readActions: lesende Actions einer Einstellung für Sachbearbeiter.
var readActions = []string{"list", "get", formStateAction}

var roleNameStrip = regexp.MustCompile(`[^\p{L}0-9 _.-]+`)

// moduleInfo: Modul aus dem Catalog.
type moduleInfo struct {
	metamodel.ModuleDefinition
}

func (c *catalogView) modules() []moduleInfo {
	var out struct {
		Modules []metamodel.ModuleDefinition `json:"modules"`
	}
	resp, err := c.call("ListModules", map[string]any{})
	if err != nil || sdk.Decode(resp.Payload, &out) != nil {
		return nil
	}
	ms := make([]moduleInfo, len(out.Modules))
	for i, m := range out.Modules {
		ms[i] = moduleInfo{m} // Titel deutsch: Rollennamen bleiben gleich, egal in welcher Sprache angelegt
	}
	return ms
}

// routes: Actions (Routen) eines Objects laut Catalog.
func (c *catalogView) routes(object string) []string {
	var res struct {
		Actions []struct {
			Action string `json:"action"`
		} `json:"actions"`
	}
	resp, err := c.call("ListActions", map[string]any{"object": object})
	if err != nil || sdk.Decode(resp.Payload, &res) != nil {
		return nil
	}
	out := make([]string, len(res.Actions))
	for i, a := range res.Actions {
		out[i] = a.Action
	}
	return out
}

// moduleGrants: Object.Action-Paare der Rollenart (clerk, expert) für ein Modul.
func (c *catalogView) moduleGrants(m metamodel.ModuleDefinition, kind string) [][2]string {
	var out [][2]string
	seen := map[[2]string]bool{}
	add := func(object, action string) {
		k := [2]string{object, action}
		if !seen[k] && !seen[[2]string{object, "*"}] {
			seen[k] = true
			out = append(out, k)
		}
	}
	own := map[string]bool{}
	var names []string
	for _, o := range m.Objects {
		names = append(names, o.Object)
	}
	names = append(names, m.Services...)
	for _, n := range names {
		own[n] = true
	}
	for _, n := range names {
		if kind == "clerk" && slices.Contains(m.Customizing, n) {
			routes := c.routes(n)
			for _, a := range readActions {
				if slices.Contains(routes, a) {
					add(n, a)
				}
			}
			continue
		}
		add(n, "*")
	}
	// Verweise auf Objects anderer Module lesen; Abschnitte Merkmale, Texte, Dokumente pflegen.
	var refs []string
	for _, o := range m.Objects {
		d := c.def(o.Object)
		if d == nil {
			continue
		}
		for _, f := range d.Fields {
			if f.Lookup != nil && f.Lookup.Object != "" {
				refs = append(refs, f.Lookup.Object)
			}
		}
		for _, s := range d.Sections {
			switch {
			case s.Relation != nil && s.Relation.Object != "":
				refs = append(refs, s.Relation.Object)
			case s.Tags:
				add(tagservice.Object, "*")
			case s.Texts:
				add(textservice.Object, "*")
			case s.Documents:
				add(docservice.Object, "*")
			}
		}
	}
	for _, r := range refs {
		if own[r] {
			continue
		}
		routes := c.routes(r)
		for _, a := range []string{"list", "get"} {
			if slices.Contains(routes, a) {
				add(r, a)
			}
		}
	}
	return out
}

// roleFormState: Auswahl der Module für „Modulrollen anlegen …“ (alle vorbelegt).
func (p *Plugin) roleFormState(ctx context.Context, payload any) (sdk.Response, error) {
	var in metamodel.FormStateRequest
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	st := metamodel.FormState{Fields: map[string]metamodel.FieldState{}}
	if in.Mode != "action" || in.Action != setupModuleRolesAction {
		return sdk.Response{Payload: st}, nil
	}
	c := p.catalog(ctx)
	var opts []metamodel.Option
	var all []string
	for _, m := range c.modules() {
		opts = append(opts, metamodel.Option{Value: m.Name, Label: c.tr.Lookup(c.loc, m.TitleKey, m.Title) + " (" + m.Name + ")"})
		all = append(all, m.Name)
	}
	fs := metamodel.FieldState{Options: opts}
	if in.Values["modules"] == "" {
		v := strings.Join(all, ",")
		fs.Value = &v
	}
	st.Fields["modules"] = fs
	if in.Values["company_codes"] == "" {
		v := AllCompanyCodes
		st.Fields["company_codes"] = metamodel.FieldState{Value: &v}
	}
	st.Message = "Je Modul entstehen die Rollen „<Modul> - Sachbearbeiter“ und „<Modul> - Fachverantwortlicher“. " +
		"Vorhandene Berechtigungen bleiben unverändert, fehlende kommen hinzu."
	return sdk.Response{Payload: st}, nil
}

// setupModuleRoles legt die Modulrollen an bzw. ergänzt sie.
func (p *Plugin) setupModuleRoles(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		Data struct {
			Modules      string `json:"modules"`
			CompanyCodes string `json:"company_codes"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	ccs, err := parseCompanyCodes(firstNonEmpty(in.Data.CompanyCodes, AllCompanyCodes))
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Buchungskreise: %v", sdk.ErrInvalidArgument, err)
	}
	var want []string
	for _, s := range strings.Split(in.Data.Modules, ",") {
		if s = strings.TrimSpace(s); s != "" {
			want = append(want, s)
		}
	}
	if len(want) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Module fehlen", sdk.ErrInvalidArgument)
	}
	c := p.catalog(ctx)
	byName := map[string]moduleInfo{}
	for _, m := range c.modules() {
		byName[m.Name] = m
	}
	var report []string
	added := 0
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		for _, name := range want {
			m, ok := byName[name]
			if !ok {
				return fmt.Errorf("%w: Modul %s gibt es nicht (Plugin gestoppt?)", sdk.ErrInvalidArgument, name)
			}
			for _, k := range moduleRoleKinds {
				roleName := strings.Join(strings.Fields(roleNameStrip.ReplaceAllString(m.Title, " ")), " ") + " - " + k.suffix
				id, created, err := p.ensureRole(ctx, tx, roleName, fmt.Sprintf(k.desc, m.Name))
				if err != nil {
					return err
				}
				n, err := p.addMissingGrants(ctx, tx, id, c.moduleGrants(m.ModuleDefinition, k.key), ccs)
				if err != nil {
					return err
				}
				added += n
				state := "ergänzt"
				if created {
					state = "angelegt"
				}
				report = append(report, fmt.Sprintf("%s (%s, %d neu)", roleName, state, n))
			}
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	p.invalidate()
	return sdk.Response{Payload: map[string]any{"message": fmt.Sprintf("Modulrollen: %d Berechtigungen neu – %s", added, strings.Join(report, "; "))}}, nil
}

// ensureRole: Rolle mit dem Namen (anlegen, wenn es sie nicht gibt).
func (p *Plugin) ensureRole(ctx context.Context, tx *sql.Tx, name, desc string) (string, bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, p.q(`SELECT id FROM iam__roles WHERE name = ?`), name).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	r := roleRow{ID: newID(), Name: name, Description: desc}
	return r.ID, true, p.saveRole(ctx, tx, r, true)
}

// addMissingGrants: Object.Action-Paare, die die Rolle noch nicht aktiv hat.
// Ein vorhandenes Object.* deckt die einzelnen Actions ab.
func (p *Plugin) addMissingGrants(ctx context.Context, tx *sql.Tx, roleID string, pairs [][2]string, ccs []string) (int, error) {
	rows, err := tx.QueryContext(ctx, p.q(`SELECT object, action FROM iam__role_auth WHERE role_id = ? AND active = 1`), roleID)
	if err != nil {
		return 0, err
	}
	have := map[[2]string]bool{}
	for rows.Next() {
		var o, a string
		if err := rows.Scan(&o, &a); err != nil {
			rows.Close()
			return 0, err
		}
		have[[2]string{o, a}] = true
	}
	rows.Close()
	n := 0
	for _, k := range pairs {
		if have[k] || have[[2]string{k[0], "*"}] {
			continue
		}
		if err := p.insertGrant(ctx, tx, grant{ID: newID(), RoleID: roleID, Object: k[0], Action: k[1], CompanyCodes: ccs, Active: true}); err != nil {
			return n, err
		}
		have[k] = true
		n++
	}
	return n, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
