package iam

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// Speicherung der Berechtigungen: iam__role_auth (je Rolle und Object.Action)
// und iam__role_auth_value (erlaubte Feldwerte). Beendet wird über active = 0,
// nur das Ersetzen über die Textform (Rollen-API) löscht.

const grantCols = `ra.id, ra.role_id, ra.object, ra.action, ra.company_codes, ra.active`

// queryGrants lädt Zeilen aus "iam__role_auth ra <rest>" samt allen Feldwerten.
func (p *Plugin) queryGrants(ctx context.Context, q database.Querier, rest string, args ...any) ([]grant, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT `+grantCols+` FROM iam__role_auth ra `+rest+` ORDER BY ra.object, ra.action, ra.created_at, ra.id`), args...)
	if err != nil {
		return nil, err
	}
	out := make([]grant, len(res.Rows))
	index := map[string]int{}
	for i, r := range res.Rows {
		ccs, _ := parseCompanyCodes(s(r[4]))
		out[i] = grant{ID: s(r[0]), RoleID: s(r[1]), Object: s(r[2]), Action: s(r[3]), CompanyCodes: ccs, Active: b(r[5])}
		index[out[i].ID] = i
	}
	if len(out) == 0 {
		return out, nil
	}
	vres, err := database.Query(ctx, q, p.q(`SELECT `+valueCols+` FROM iam__role_auth_value v
		WHERE v.auth_id IN (SELECT ra.id FROM iam__role_auth ra `+rest+`) ORDER BY v.field, v.created_at, v.id`), args...)
	if err != nil {
		return nil, err
	}
	for _, r := range vres.Rows {
		v := scanValue(r)
		if i, ok := index[v.AuthID]; ok {
			out[i].Values = append(out[i].Values, v)
		}
	}
	return out, nil
}

// grantsByRole: alle Zeilen (auch inaktive) je Rolle; roleID leer = alle Rollen.
func (p *Plugin) grantsByRole(ctx context.Context, q database.Querier, roleID string) (map[string][]grant, error) {
	rest, args := "", []any{}
	if roleID != "" {
		rest, args = "WHERE ra.role_id = ?", []any{roleID}
	}
	gs, err := p.queryGrants(ctx, q, rest, args...)
	if err != nil {
		return nil, err
	}
	out := map[string][]grant{}
	for _, g := range gs {
		out[g.RoleID] = append(out[g.RoleID], g)
	}
	return out, nil
}

func (p *Plugin) getGrant(ctx context.Context, q database.Querier, id string) (*grant, error) {
	gs, err := p.queryGrants(ctx, q, "WHERE ra.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(gs) == 0 {
		return nil, fmt.Errorf("%w: Berechtigung %q", sdk.ErrNotFound, id)
	}
	return &gs[0], nil
}

// listGrants filtert nach role_id und object; inaktive nur mit includeHistory.
func (p *Plugin) listGrants(ctx context.Context, roleID, object string, all bool) ([]grant, error) {
	var conds []string
	var args []any
	if roleID != "" {
		conds, args = append(conds, "ra.role_id = ?"), append(args, roleID)
	}
	if object != "" {
		conds, args = append(conds, "ra.object = ?"), append(args, object)
	}
	if !all {
		conds = append(conds, "ra.active = 1")
	}
	rest := ""
	if len(conds) > 0 {
		rest = "WHERE " + strings.Join(conds, " AND ")
	}
	return p.queryGrants(ctx, p.pool(), rest, args...)
}

// replaceGrants ersetzt alle Berechtigungen einer Rolle (Textform).
func (p *Plugin) replaceGrants(ctx context.Context, tx *sql.Tx, roleID string, gs []grant) error {
	if _, err := tx.ExecContext(ctx, p.q(`DELETE FROM iam__role_auth_value WHERE auth_id IN (SELECT id FROM iam__role_auth WHERE role_id = ?)`), roleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, p.q(`DELETE FROM iam__role_auth WHERE role_id = ?`), roleID); err != nil {
		return err
	}
	for _, g := range gs {
		g.ID, g.RoleID, g.Active = newID(), roleID, true
		if err := p.insertGrant(ctx, tx, g); err != nil {
			return err
		}
		for _, v := range g.Values {
			v.ID, v.AuthID, v.Active = newID(), g.ID, true
			if err := p.insertValue(ctx, tx, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkGrant: vorhandene Buchungskreise, keine zweite aktive Zeile für
// dasselbe Object.Action in der Rolle.
func (p *Plugin) checkGrant(ctx context.Context, q database.Querier, g grant) error {
	if err := p.checkCompanyCodes(ctx, q, g.CompanyCodes); err != nil {
		return err
	}
	if !g.Active {
		return nil
	}
	res, err := database.Query(ctx, q, p.q(`SELECT 1 FROM iam__role_auth
		WHERE role_id = ? AND object = ? AND action = ? AND active = 1 AND id <> ?`), g.RoleID, g.Object, g.Action, g.ID)
	if err != nil {
		return err
	}
	if len(res.Rows) > 0 {
		return fmt.Errorf("%w: Die Rolle hat bereits eine Berechtigung %s.%s – dort Buchungskreise und Feldwerte ergänzen", sdk.ErrAlreadyExists, g.Object, g.Action)
	}
	return nil
}

func (p *Plugin) insertGrant(ctx context.Context, tx *sql.Tx, g grant) error {
	if err := p.checkGrant(ctx, tx, g); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__role_auth (id, role_id, object, action, company_codes, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`), g.ID, g.RoleID, g.Object, g.Action, strings.Join(g.CompanyCodes, ","), boolInt(g.Active), createdAt())
	return err
}

func (p *Plugin) updateGrant(ctx context.Context, tx *sql.Tx, g grant) error {
	if err := p.checkGrant(ctx, tx, g); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__role_auth SET role_id = ?, object = ?, action = ?, company_codes = ?, active = ? WHERE id = ?`),
		g.RoleID, g.Object, g.Action, strings.Join(g.CompanyCodes, ","), boolInt(g.Active), g.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: Berechtigung %q", sdk.ErrNotFound, g.ID)
	}
	return nil
}

// --- Feldwerte -------------------------------------------------------------------

const valueCols = `v.id, v.auth_id, v.field, v.low, v.high, v.active`

func scanValue(r []any) authValue {
	return authValue{ID: s(r[0]), AuthID: s(r[1]), Field: s(r[2]), Low: s(r[3]), High: s(r[4]), Active: b(r[5])}
}

func (p *Plugin) getValue(ctx context.Context, q database.Querier, id string) (*authValue, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT `+valueCols+` FROM iam__role_auth_value v WHERE v.id = ?`), id)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Feldwert %q", sdk.ErrNotFound, id)
	}
	v := scanValue(res.Rows[0])
	return &v, nil
}

func (p *Plugin) listValues(ctx context.Context, authID string, all bool) ([]authValue, error) {
	var conds []string
	var args []any
	if authID != "" {
		conds, args = append(conds, "v.auth_id = ?"), append(args, authID)
	}
	if !all {
		conds = append(conds, "v.active = 1")
	}
	rest := ""
	if len(conds) > 0 {
		rest = " WHERE " + strings.Join(conds, " AND ")
	}
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT `+valueCols+` FROM iam__role_auth_value v`+rest+` ORDER BY v.field, v.created_at, v.id`), args...)
	if err != nil {
		return nil, err
	}
	out := make([]authValue, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = scanValue(r)
	}
	return out, nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (p *Plugin) insertValue(ctx context.Context, tx *sql.Tx, v authValue) error {
	_, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__role_auth_value (id, auth_id, field, low, high, active, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`),
		v.ID, v.AuthID, v.Field, v.Low, nullable(v.High), boolInt(v.Active), createdAt())
	return err
}

func (p *Plugin) updateValue(ctx context.Context, tx *sql.Tx, v authValue) error {
	res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__role_auth_value SET auth_id = ?, field = ?, low = ?, high = ?, active = ? WHERE id = ?`),
		v.AuthID, v.Field, v.Low, nullable(v.High), boolInt(v.Active), v.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: Feldwert %q", sdk.ErrNotFound, v.ID)
	}
	return nil
}

// createdAt mit Nanosekunden: hält die Reihenfolge der Erfassung.
func createdAt() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// --- Übernahme aus 0.4.0 ---------------------------------------------------------

// migrateLegacy übernimmt iam__role_permissions (eine Zeile je Buchungskreis)
// einmalig nach iam__role_auth (eine Zeile je Object.Action), solange dort
// noch nichts steht.
func (p *Plugin) migrateLegacy(ctx context.Context) (int, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT COUNT(*) FROM iam__role_auth`)
	if err != nil {
		return 0, err
	}
	if n, _ := strconv.Atoi(s(res.Rows[0][0])); n > 0 {
		return 0, nil
	}
	old, err := database.Query(ctx, p.pool(), `SELECT role_id, object, action, company_code FROM iam__role_permissions ORDER BY role_id, object, action, company_code`)
	if err != nil || len(old.Rows) == 0 {
		return 0, err
	}
	type key struct{ role, object, action string }
	var order []key
	codes := map[key][]string{}
	for _, r := range old.Rows {
		k := key{s(r[0]), s(r[1]), s(r[2])}
		if _, ok := codes[k]; !ok {
			order = append(order, k)
		}
		codes[k] = append(codes[k], s(r[3]))
	}
	tx, err := p.pool().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, k := range order {
		ccs, _ := parseCompanyCodes(strings.Join(codes[k], ","))
		if _, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__role_auth (id, role_id, object, action, company_codes, active, created_at)
			VALUES (?, ?, ?, ?, ?, 1, ?)`), newID(), k.role, k.object, k.action, strings.Join(ccs, ","), createdAt()); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	p.invalidate()
	return len(order), nil
}
