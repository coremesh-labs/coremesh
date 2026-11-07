package iam

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Personen (seit 0.12.0): Ein Benutzer arbeitet in verschiedenen Rollen (z. B. Buchhaltung,
// Mietverwaltung, Privat). Eine Person ist eine Auswahl seiner Rollen und optional seiner
// Buchungskreise. Ist sie gewählt (iam__users.persona_id, über Account.UpdateProfile), gelten
// nur deren Berechtigungen – Menü und Rechte. Eine Person schränkt nur ein: Rollen, die der
// Benutzer nicht (mehr) hat, wirken nie. Wechseln darf jeder (Account.UpdateProfile ist Grundrecht).

type personaRow struct {
	ID, UserID, Name string
	Roles            []string // Rollennamen
	CompanyCodes     []string // leer = die des Benutzers
	SortOrder        int
	Active           bool
}

const personaCols = `id, user_id, name, company_codes, sort_order, active`

func (p *Plugin) scanPersonas(ctx context.Context, q database.Querier, where string, args ...any) ([]personaRow, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT `+personaCols+` FROM iam__personas `+where+` ORDER BY user_id, sort_order, name`), args...)
	if err != nil {
		return nil, err
	}
	out := make([]personaRow, len(res.Rows))
	for i, r := range res.Rows {
		ccs, _ := parseCompanyCodes(s(r[3]))
		if slices.Contains(ccs, AllCompanyCodes) {
			ccs = nil
		}
		out[i] = personaRow{ID: s(r[0]), UserID: s(r[1]), Name: s(r[2]), CompanyCodes: ccs, SortOrder: int(num(r[4])), Active: b(r[5])}
		rr, err := database.Query(ctx, q, p.q(`SELECT r.name FROM iam__persona_roles pr JOIN iam__roles r ON r.id = pr.role_id WHERE pr.persona_id = ? ORDER BY r.name`), out[i].ID)
		if err != nil {
			return nil, err
		}
		for _, x := range rr.Rows {
			out[i].Roles = append(out[i].Roles, s(x[0]))
		}
	}
	return out, nil
}

func num(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	var n int64
	_, _ = fmt.Sscan(s(v), &n)
	return n
}

// listPersonas: Personen eines Benutzers ("" = aller), ohne inaktive außer all.
func (p *Plugin) listPersonas(ctx context.Context, q database.Querier, userID string, all bool) ([]personaRow, error) {
	var conds []string
	var args []any
	if userID != "" {
		conds, args = append(conds, "user_id = ?"), append(args, userID)
	}
	if !all {
		conds = append(conds, "active = 1")
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	return p.scanPersonas(ctx, q, where, args...)
}

func (p *Plugin) getPersona(ctx context.Context, q database.Querier, id string) (*personaRow, error) {
	rows, err := p.scanPersonas(ctx, q, "WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: Person %q", sdk.ErrNotFound, id)
	}
	return &rows[0], nil
}

// personaGrants: mit gewählter, aktiver Person nur die Berechtigungen ihrer Rollen, in ihren
// Buchungskreisen. Ohne Person (oder inaktiv, gelöscht) bleibt alles wie beim Benutzer.
func (p *Plugin) personaGrants(ctx context.Context, q database.Querier, userID string, gs []grant) ([]grant, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT pe.id, pe.company_codes FROM iam__users u
		JOIN iam__personas pe ON pe.id = u.persona_id AND pe.user_id = u.id AND pe.active = 1 WHERE u.id = ?`), userID)
	if err != nil || len(res.Rows) == 0 {
		return gs, err
	}
	pid := s(res.Rows[0][0])
	rr, err := database.Query(ctx, q, p.q(`SELECT role_id FROM iam__persona_roles WHERE persona_id = ?`), pid)
	if err != nil {
		return nil, err
	}
	roles := map[string]bool{}
	for _, r := range rr.Rows {
		roles[s(r[0])] = true
	}
	out := make([]grant, 0, len(gs))
	for _, g := range gs {
		if roles[g.RoleID] {
			out = append(out, g)
		}
	}
	if ccs, _ := parseCompanyCodes(s(res.Rows[0][1])); len(ccs) > 0 {
		out = restrictGrants(out, ccs)
	}
	return out, nil
}

func personaRecord(pr personaRow) map[string]any {
	return map[string]any{"id": pr.ID, "user_id": pr.UserID, "name": pr.Name, "roles": strings.Join(pr.Roles, "\n"),
		"company_codes": strings.Join(pr.CompanyCodes, ","), "sort_order": pr.SortOrder, "active": pr.Active}
}

func (p *Plugin) personaList(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		Query map[string]any `json:"query"`
	}
	_ = sdk.Decode(payload, &in)
	rows, err := p.listPersonas(ctx, p.pool(), s(in.Query["user_id"]), includeHistory(payload))
	if err != nil {
		return sdk.Response{}, err
	}
	items := []any{}
	for _, pr := range rows {
		items = append(items, personaRecord(pr))
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) personaGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	pr, err := p.getPersona(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: personaRecord(*pr)}, nil
}

func (p *Plugin) personaSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			UserID       string `json:"user_id"`
			Name         string `json:"name"`
			Roles        string `json:"roles"`
			CompanyCodes string `json:"company_codes"`
			SortOrder    any    `json:"sort_order"`
			Active       *bool  `json:"active"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	d := in.Data
	pr := personaRow{ID: in.ID, UserID: strings.TrimSpace(d.UserID), Name: strings.TrimSpace(d.Name), Roles: lines(d.Roles),
		SortOrder: int(num(d.SortOrder)), Active: d.Active == nil || *d.Active}
	if !create {
		old, err := p.getPersona(ctx, p.pool(), in.ID)
		if err != nil {
			return sdk.Response{}, err
		}
		if pr.UserID == "" {
			pr.UserID = old.UserID
		}
		if pr.UserID != old.UserID {
			return sdk.Response{}, fmt.Errorf("%w: Der Benutzer einer Person lässt sich nicht ändern", sdk.ErrInvalidArgument)
		}
	}
	if pr.Name == "" {
		return sdk.Response{}, fmt.Errorf("%w: Name ist Pflicht", sdk.ErrInvalidArgument)
	}
	u, err := p.getUser(ctx, p.pool(), pr.UserID)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Benutzer %q gibt es nicht", sdk.ErrInvalidArgument, pr.UserID)
	}
	if len(pr.Roles) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: mindestens eine Rolle", sdk.ErrInvalidArgument)
	}
	for _, r := range pr.Roles {
		if !slices.Contains(u.Roles, r) {
			return sdk.Response{}, fmt.Errorf("%w: Rolle %q hat der Benutzer %s nicht (%s)", sdk.ErrInvalidArgument, r, u.Username, strings.Join(u.Roles, ", "))
		}
	}
	ccs, err := parseCompanyCodes(d.CompanyCodes)
	if err != nil {
		return sdk.Response{}, fmt.Errorf("%w: Buchungskreise: %v", sdk.ErrInvalidArgument, err)
	}
	if slices.Contains(ccs, AllCompanyCodes) {
		ccs = nil
	}
	if err := p.checkCompanyCodes(ctx, p.pool(), ccs); err != nil {
		return sdk.Response{}, err
	}
	if len(ccs) > 0 && !slices.Contains(u.CompanyCodes, AllCompanyCodes) && len(u.CompanyCodes) > 0 {
		for _, cc := range ccs {
			if !slices.Contains(u.CompanyCodes, cc) {
				return sdk.Response{}, fmt.Errorf("%w: Buchungskreis %s hat der Benutzer %s nicht", sdk.ErrInvalidArgument, cc, u.Username)
			}
		}
	}
	pr.CompanyCodes = ccs
	if create {
		pr.ID = newID()
	}
	now := ts(time.Now())
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if create {
			_, err = tx.ExecContext(ctx, p.q(`INSERT INTO iam__personas (id, user_id, name, company_codes, sort_order, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`),
				pr.ID, pr.UserID, pr.Name, userCompanyCodes(pr.CompanyCodes), pr.SortOrder, boolInt(pr.Active), now, now)
		} else {
			_, err = tx.ExecContext(ctx, p.q(`UPDATE iam__personas SET name = ?, company_codes = ?, sort_order = ?, active = ?, updated_at = ? WHERE id = ?`),
				pr.Name, userCompanyCodes(pr.CompanyCodes), pr.SortOrder, boolInt(pr.Active), now, pr.ID)
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, p.q(`DELETE FROM iam__persona_roles WHERE persona_id = ?`), pr.ID); err != nil {
			return err
		}
		for _, n := range pr.Roles {
			if _, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__persona_roles (persona_id, role_id) SELECT ?, id FROM iam__roles WHERE name = ?`), pr.ID, n); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	p.invalidate()
	return p.personaGet(ctx, map[string]any{"id": pr.ID})
}

func (p *Plugin) personaDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if _, err := p.pool().ExecContext(ctx, p.q(`UPDATE iam__personas SET active = 0, updated_at = ? WHERE id = ?`), ts(time.Now()), id); err != nil {
		return sdk.Response{}, err
	}
	// gewählt? dann wieder alle Rollen
	if _, err := p.pool().ExecContext(ctx, p.q(`UPDATE iam__users SET persona_id = NULL WHERE persona_id = ?`), id); err != nil {
		return sdk.Response{}, err
	}
	p.invalidate()
	return p.personaGet(ctx, map[string]any{"id": id})
}
