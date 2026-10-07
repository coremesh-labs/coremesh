package iam

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-lab/coremesh/internal/database"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

type userRow struct {
	ID          string
	Username    string
	DisplayName string
	TenantID    string
	Active      bool
	Locale      string // "" = automatisch (Sprachaushandlung im Frontend)
	CreatedAt   string
	Roles       []string // Rollennamen
}

type roleRow struct {
	ID          string
	Name        string
	Description string
	Permissions []grant
	// ReplaceGrants: saveRole ersetzt alle Berechtigungen durch Permissions
	// (Textform über die API, erster Start); sonst bleiben sie unverändert.
	ReplaceGrants bool
}

func (p *Plugin) pool() *sql.DB {
	db, _ := p.db.DB(p.settings.Database)
	return db
}

func (p *Plugin) q(query string) string {
	return database.Rebind(p.db.Driver(p.settings.Database), query)
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func s(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	}
	return fmt.Sprint(v)
}

func b(v any) bool {
	n, _ := strconv.Atoi(s(v))
	return n != 0
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// inTx führt fn in einer Transaktion aus, prüft danach, dass noch ein aktiver
// Administrator (*.*) existiert, und leert den Berechtigungs-Cache.
func (p *Plugin) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := p.pool().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	n, err := p.adminCount(ctx, tx)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: Es muss mindestens ein aktiver Benutzer mit der Berechtigung *.* bleiben", sdk.ErrFailedPrecondition)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	p.invalidate()
	return nil
}

func (p *Plugin) adminCount(ctx context.Context, q database.Querier) (int, error) {
	res, err := database.Query(ctx, q, p.q(`
		SELECT COUNT(DISTINCT u.id) FROM iam__users u
		JOIN iam__user_roles ur ON ur.user_id = u.id
		JOIN iam__role_auth ra ON ra.role_id = ur.role_id
		WHERE u.active = 1 AND ra.active = 1 AND ra.object = '*' AND ra.action = '*' AND ra.company_codes = '*'
		  AND NOT EXISTS (SELECT 1 FROM iam__role_auth_value v WHERE v.auth_id = ra.id AND v.active = 1)`))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s(res.Rows[0][0]))
}

func (p *Plugin) countUsers(ctx context.Context) (int, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT COUNT(*) FROM iam__users`)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(s(res.Rows[0][0]))
}

// --- Benutzer ------------------------------------------------------------------

const userCols = `id, username, display_name, tenant_id, active, created_at, locale`

func scanUser(r []any) userRow {
	return userRow{ID: s(r[0]), Username: s(r[1]), DisplayName: s(r[2]), TenantID: s(r[3]), Active: b(r[4]), CreatedAt: s(r[5]), Locale: s(r[6])}
}

func (p *Plugin) listUsers(ctx context.Context) ([]userRow, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT `+userCols+` FROM iam__users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	roles, err := p.roleNamesByUser(ctx, p.pool(), "")
	if err != nil {
		return nil, err
	}
	out := make([]userRow, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = scanUser(r)
		out[i].Roles = roles[out[i].ID]
	}
	return out, nil
}

func (p *Plugin) getUser(ctx context.Context, q database.Querier, id string) (*userRow, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT `+userCols+` FROM iam__users WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Benutzer %q", sdk.ErrNotFound, id)
	}
	u := scanUser(res.Rows[0])
	roles, err := p.roleNamesByUser(ctx, q, id)
	if err != nil {
		return nil, err
	}
	u.Roles = roles[id]
	return &u, nil
}

// credentials liefert id, Passwort-Hash und Aktiv-Status zum Benutzernamen.
func (p *Plugin) credentials(ctx context.Context, username string) (id, hash string, active bool, err error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT id, password_hash, active FROM iam__users WHERE username = ?`), username)
	if err != nil || len(res.Rows) == 0 {
		return "", "", false, err
	}
	r := res.Rows[0]
	return s(r[0]), s(r[1]), b(r[2]), nil
}

func (p *Plugin) passwordHash(ctx context.Context, id string) (string, error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT password_hash FROM iam__users WHERE id = ?`), id)
	if err != nil {
		return "", err
	}
	if len(res.Rows) == 0 {
		return "", fmt.Errorf("%w: Benutzer %q", sdk.ErrNotFound, id)
	}
	return s(res.Rows[0][0]), nil
}

func (p *Plugin) roleNamesByUser(ctx context.Context, q database.Querier, userID string) (map[string][]string, error) {
	query := `SELECT ur.user_id, r.name FROM iam__user_roles ur JOIN iam__roles r ON r.id = ur.role_id`
	var args []any
	if userID != "" {
		query += ` WHERE ur.user_id = ?`
		args = append(args, userID)
	}
	res, err := database.Query(ctx, q, p.q(query+` ORDER BY r.name`), args...)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, r := range res.Rows {
		out[s(r[0])] = append(out[s(r[0])], s(r[1]))
	}
	return out, nil
}

func (p *Plugin) insertUser(ctx context.Context, tx *sql.Tx, u userRow, hash string) error {
	now := ts(time.Now())
	_, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__users (id, username, password_hash, display_name, tenant_id, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), u.ID, u.Username, hash, u.DisplayName, u.TenantID, boolInt(u.Active), now, now)
	if err != nil && isUnique(err) {
		return fmt.Errorf("%w: Benutzername %q ist vergeben", sdk.ErrInvalidArgument, u.Username)
	}
	return err
}

func (p *Plugin) updateUser(ctx context.Context, tx *sql.Tx, u userRow, hash string) error {
	query := `UPDATE iam__users SET username = ?, display_name = ?, tenant_id = ?, active = ?, updated_at = ?`
	args := []any{u.Username, u.DisplayName, u.TenantID, boolInt(u.Active), ts(time.Now())}
	if hash != "" {
		query += `, password_hash = ?`
		args = append(args, hash)
	}
	res, err := tx.ExecContext(ctx, p.q(query+` WHERE id = ?`), append(args, u.ID)...)
	if err != nil {
		if isUnique(err) {
			return fmt.Errorf("%w: Benutzername %q ist vergeben", sdk.ErrInvalidArgument, u.Username)
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: Benutzer %q", sdk.ErrNotFound, u.ID)
	}
	return nil
}

func (p *Plugin) setPassword(ctx context.Context, id, hash string) error {
	_, err := p.pool().ExecContext(ctx, p.q(`UPDATE iam__users SET password_hash = ?, updated_at = ? WHERE id = ?`), hash, ts(time.Now()), id)
	return err
}

// setUserRoles ersetzt die Rollen eines Benutzers (über Rollennamen).
func (p *Plugin) setUserRoles(ctx context.Context, tx *sql.Tx, userID string, names []string) error {
	if _, err := tx.ExecContext(ctx, p.q(`DELETE FROM iam__user_roles WHERE user_id = ?`), userID); err != nil {
		return err
	}
	for _, n := range names {
		res, err := database.Query(ctx, tx, p.q(`SELECT id FROM iam__roles WHERE name = ?`), n)
		if err != nil {
			return err
		}
		if len(res.Rows) == 0 {
			return fmt.Errorf("%w: Rolle %q gibt es nicht", sdk.ErrInvalidArgument, n)
		}
		if _, err := tx.ExecContext(ctx, p.q(`INSERT INTO iam__user_roles (user_id, role_id) VALUES (?, ?)`), userID, s(res.Rows[0][0])); err != nil {
			return err
		}
	}
	return nil
}

// userGrants: aktive Berechtigungen aller Rollen eines aktiven Benutzers.
func (p *Plugin) userGrants(ctx context.Context, q database.Querier, userID string) ([]grant, error) {
	return p.queryGrants(ctx, q, `JOIN iam__user_roles ur ON ur.role_id = ra.role_id
		JOIN iam__users u ON u.id = ur.user_id
		WHERE ur.user_id = ? AND u.active = 1 AND ra.active = 1`, userID)
}

// --- Rollen --------------------------------------------------------------------

func (p *Plugin) listRoles(ctx context.Context) ([]roleRow, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT id, name, description FROM iam__roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	perms, err := p.grantsByRole(ctx, p.pool(), "")
	if err != nil {
		return nil, err
	}
	out := make([]roleRow, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = roleRow{ID: s(r[0]), Name: s(r[1]), Description: s(r[2]), Permissions: perms[s(r[0])]}
	}
	return out, nil
}

func (p *Plugin) getRole(ctx context.Context, q database.Querier, id string) (*roleRow, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT id, name, description FROM iam__roles WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Rolle %q", sdk.ErrNotFound, id)
	}
	r := res.Rows[0]
	perms, err := p.grantsByRole(ctx, q, id)
	if err != nil {
		return nil, err
	}
	return &roleRow{ID: s(r[0]), Name: s(r[1]), Description: s(r[2]), Permissions: perms[id]}, nil
}

func (p *Plugin) saveRole(ctx context.Context, tx *sql.Tx, r roleRow, insert bool) error {
	var err error
	if insert {
		_, err = tx.ExecContext(ctx, p.q(`INSERT INTO iam__roles (id, name, description, created_at) VALUES (?, ?, ?, ?)`),
			r.ID, r.Name, r.Description, ts(time.Now()))
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx, p.q(`UPDATE iam__roles SET name = ?, description = ? WHERE id = ?`), r.Name, r.Description, r.ID)
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				return fmt.Errorf("%w: Rolle %q", sdk.ErrNotFound, r.ID)
			}
		}
	}
	if err != nil {
		if isUnique(err) {
			return fmt.Errorf("%w: Rollenname %q ist vergeben", sdk.ErrInvalidArgument, r.Name)
		}
		return err
	}
	if !r.ReplaceGrants {
		return nil
	}
	return p.replaceGrants(ctx, tx, r.ID, r.Permissions)
}

// isUnique erkennt Verletzungen eindeutiger Indizes (SQLite, PostgreSQL).
func isUnique(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "unique") || strings.Contains(m, "duplicate key")
}

// --- Buchungskreise --------------------------------------------------------------

type companyCode struct {
	ID          string
	Description string
}

// checkCompanyCodes: In Rollen sind nur vorhandene Buchungskreise erlaubt.
func (p *Plugin) checkCompanyCodes(ctx context.Context, q database.Querier, ccs []string) error {
	var missing []string
	for _, cc := range ccs {
		if cc == AllCompanyCodes {
			continue
		}
		res, err := database.Query(ctx, q, p.q(`SELECT 1 FROM iam__company_codes WHERE id = ?`), cc)
		if err != nil {
			return err
		}
		if len(res.Rows) == 0 && !slices.Contains(missing, cc) {
			missing = append(missing, cc)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: Buchungskreis %s gibt es nicht", sdk.ErrInvalidArgument, strings.Join(missing, ", "))
	}
	return nil
}

func (p *Plugin) listCompanyCodes(ctx context.Context) ([]companyCode, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT id, description FROM iam__company_codes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	out := make([]companyCode, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = companyCode{s(r[0]), s(r[1])}
	}
	return out, nil
}

func (p *Plugin) getCompanyCode(ctx context.Context, id string) (*companyCode, error) {
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT id, description FROM iam__company_codes WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Buchungskreis %q", sdk.ErrNotFound, id)
	}
	return &companyCode{s(res.Rows[0][0]), s(res.Rows[0][1])}, nil
}

func (p *Plugin) saveCompanyCode(ctx context.Context, cc companyCode, insert bool) error {
	var err error
	if insert {
		_, err = p.pool().ExecContext(ctx, p.q(`INSERT INTO iam__company_codes (id, description, created_at) VALUES (?, ?, ?)`),
			cc.ID, cc.Description, ts(time.Now()))
		if err != nil && isUnique(err) {
			return fmt.Errorf("%w: Buchungskreis %q gibt es bereits", sdk.ErrInvalidArgument, cc.ID)
		}
		return err
	}
	res, err := p.pool().ExecContext(ctx, p.q(`UPDATE iam__company_codes SET description = ? WHERE id = ?`), cc.Description, cc.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: Buchungskreis %q", sdk.ErrNotFound, cc.ID)
	}
	return nil
}
