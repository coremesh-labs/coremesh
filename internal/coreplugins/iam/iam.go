// Package iam ist das Core-Plugin für Benutzer, Rollen und Berechtigungen
// (Identity & Access Management). Es läuft im Host-Prozess:
//
//   - Es ist der Authorizer des Dispatchers: Jede Wurzelanfrage mit UserID
//     wird gegen die Berechtigungen der Rollen des Benutzers geprüft.
//   - Es prüft Anmeldedaten für Ingress-Plugins (Account.Authenticate).
//   - Benutzer und Rollen werden über die generische Oberfläche verwaltet
//     (Objects User und Role mit Metamodell).
//
// Beschreibung für Betrieb und Entwicklung: README.md.
package iam

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

const (
	Name    = "iam"
	Version = "0.6.0"

	// AdminRole ist die beim ersten Start angelegte Rolle mit *.*.
	AdminRole      = "Administrator"
	minPasswordLen = 10
)

type Plugin struct {
	db       *database.Manager
	settings settings
	cost     int // bcrypt-Kosten
	dummy    []byte
	cache    permCache
	cancel   context.CancelFunc
	host     sdk.Host
}

type settings struct {
	Database      string `json:"database"`
	AdminUser     string `json:"admin_user"`
	AdminPassword string `json:"admin_password"`
	Tenant        string `json:"tenant"` // Mandant des ersten Benutzers
}

func New(db *database.Manager) *Plugin {
	p := &Plugin{db: db, cost: bcrypt.DefaultCost + 2, cache: permCache{entries: map[string]cacheEntry{}}}
	p.dummy, _ = bcrypt.GenerateFromPassword([]byte("coremesh-dummy-password"), p.cost)
	return p
}

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:        Name,
		Version:     Version,
		Description: "Benutzer, Rollen und Berechtigungen",
		Capabilities: []sdk.Capability{
			{Object: "Account", Actions: []string{"Authenticate", "Me", "UpdateProfile", "ChangePassword", "Check", "Granted", "Display"}, Description: "Anmeldung, eigenes Konto, Rechteprüfung, Darstellungsregeln"},
			{Object: "User", Actions: []string{"list", "get", "create", "update", "deactivate"}, Description: "Benutzerverwaltung"},
			{Object: "Role", Actions: []string{"list", "get", "create", "update"}, Description: "Rollen und Berechtigungen"},
			{Object: "RoleAuth", Actions: []string{"list", "get", "create", "update", "deactivate", formStateAction}, Description: "Berechtigungen je Rolle und Object.Action"},
			{Object: "RoleAuthValue", Actions: []string{"list", "get", "create", "update", "deactivate", formStateAction}, Description: "Erlaubte Feldwerte einer Berechtigung"},
			{Object: "DisplayRule", Actions: []string{"list", "get", "create", "update", "deactivate", formStateAction}, Description: "Darstellungsregeln"},
			{Object: "DisplayRuleCondition", Actions: []string{"list", "get", "create", "update", "deactivate", formStateAction}, Description: "Bedingungen der Darstellungsregeln"},
			{Object: "DisplayRuleField", Actions: []string{"list", "get", "create", "update", "deactivate", formStateAction}, Description: "Felder der Darstellungsregeln"},
			{Object: "CompanyCode", Actions: []string{"list", "get", "create", "update"}, Description: "Buchungskreise"},
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
			{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
		},
	}, nil
}

// Configure liest die Einstellungen und legt im Hintergrund den ersten
// Administrator an, sobald die Tabellen existieren (DBSchema.Init läuft nach
// Configure).
func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Database: "main", AdminUser: "admin"}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	if _, err := p.db.DB(s.Database); err != nil {
		return err
	}
	p.settings = s
	p.host = cfg.Host
	if p.cancel != nil {
		p.cancel()
	}
	bg, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.bootstrap(bg, cfg.Host)
	return nil
}

func (p *Plugin) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch req.Object + "." + req.Action {
	case sdk.ObjectDBSchema + "." + sdk.ActionInit:
		var in sdk.SchemaInitRequest
		if err := sdk.Decode(req.Payload, &in); err != nil {
			return sdk.Response{}, err
		}
		return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL}}, nil
	case sdk.ObjectCatalog + "." + sdk.ActionDescribe:
		return sdk.Response{Payload: metamodel.DescribeResponse{
			Objects: []metamodel.ObjectDefinition{
				metamodel.WithKeys("admin", userDef), metamodel.WithKeys("admin", roleDef), metamodel.WithKeys("admin", companyCodeDef),
				metamodel.WithKeys("admin", roleAuthDef), metamodel.WithKeys("admin", roleAuthValueDef),
				metamodel.WithKeys("admin", displayRuleDef), metamodel.WithKeys("admin", displayRuleCondDef), metamodel.WithKeys("admin", displayRuleFieldDef)},
			Modules:      []metamodel.ModuleDefinition{metamodel.ModuleKeys(adminModule)},
			Translations: translations,
		}}, nil

	case "Account.Authenticate":
		return p.authenticate(ctx, req.Payload)
	case "Account.UpdateProfile":
		return p.updateProfile(ctx, req.Payload)
	case "Account.Me":
		return p.me(ctx)
	case "Account.ChangePassword":
		return p.changePassword(ctx, req.Payload)
	case "Account.Check":
		return p.check(ctx, req.Payload)
	case "Account.Granted":
		return p.granted(ctx, req.Payload)
	case "Account.Display":
		return p.display(ctx, req.Payload)

	case "User.list":
		return p.userList(ctx, req.Payload)
	case "User.get":
		return p.userGet(ctx, req.Payload)
	case "User.create":
		return p.userSave(ctx, req.Payload, true)
	case "User.update":
		return p.userSave(ctx, req.Payload, false)
	case "User.deactivate":
		return p.userDeactivate(ctx, req.Payload)

	case "Role.list":
		return p.roleList(ctx)
	case "Role.get":
		return p.roleGet(ctx, req.Payload)
	case "Role.create":
		return p.roleSave(ctx, req.Payload, true)
	case "Role.update":
		return p.roleSave(ctx, req.Payload, false)

	case "RoleAuth.list":
		return p.roleAuthList(ctx, req.Payload)
	case "RoleAuth.get":
		return p.roleAuthGet(ctx, req.Payload)
	case "RoleAuth.create":
		return p.roleAuthSave(ctx, req.Payload, true)
	case "RoleAuth.update":
		return p.roleAuthSave(ctx, req.Payload, false)
	case "RoleAuth.deactivate":
		return p.roleAuthDeactivate(ctx, req.Payload)
	case "RoleAuth." + formStateAction:
		return p.roleAuthFormState(ctx, req.Payload)
	case "RoleAuthValue.list":
		return p.roleAuthValueList(ctx, req.Payload)
	case "RoleAuthValue.get":
		return p.roleAuthValueGet(ctx, req.Payload)
	case "RoleAuthValue.create":
		return p.roleAuthValueSave(ctx, req.Payload, true)
	case "RoleAuthValue.update":
		return p.roleAuthValueSave(ctx, req.Payload, false)
	case "RoleAuthValue.deactivate":
		return p.roleAuthValueDeactivate(ctx, req.Payload)
	case "RoleAuthValue." + formStateAction:
		return p.roleAuthValueFormState(ctx, req.Payload)

	case "DisplayRule.list":
		return p.displayRuleList(ctx, req.Payload)
	case "DisplayRule.get":
		return p.displayRuleGet(ctx, req.Payload)
	case "DisplayRule.create":
		return p.displayRuleSave(ctx, req.Payload, true)
	case "DisplayRule.update":
		return p.displayRuleSave(ctx, req.Payload, false)
	case "DisplayRule.deactivate":
		return p.displayRuleDeactivate(ctx, req.Payload)
	case "DisplayRule." + formStateAction:
		return p.displayRuleFormState(ctx, req.Payload)
	case "DisplayRuleCondition.list":
		return p.childList(ctx, condTable, req.Payload)
	case "DisplayRuleCondition.get":
		return p.childGet(ctx, condTable, req.Payload)
	case "DisplayRuleCondition.create":
		return p.childSave(ctx, condTable, req.Payload, true)
	case "DisplayRuleCondition.update":
		return p.childSave(ctx, condTable, req.Payload, false)
	case "DisplayRuleCondition.deactivate":
		return p.childDeactivate(ctx, condTable, req.Payload)
	case "DisplayRuleCondition." + formStateAction:
		return p.childFormState(ctx, condTable, req.Payload)
	case "DisplayRuleField.list":
		return p.childList(ctx, fieldTable, req.Payload)
	case "DisplayRuleField.get":
		return p.childGet(ctx, fieldTable, req.Payload)
	case "DisplayRuleField.create":
		return p.childSave(ctx, fieldTable, req.Payload, true)
	case "DisplayRuleField.update":
		return p.childSave(ctx, fieldTable, req.Payload, false)
	case "DisplayRuleField.deactivate":
		return p.childDeactivate(ctx, fieldTable, req.Payload)
	case "DisplayRuleField." + formStateAction:
		return p.childFormState(ctx, fieldTable, req.Payload)

	case "CompanyCode.list":
		return p.ccList(ctx)
	case "CompanyCode.get":
		return p.ccGet(ctx, req.Payload)
	case "CompanyCode.create":
		return p.ccSave(ctx, req.Payload, true)
	case "CompanyCode.update":
		return p.ccSave(ctx, req.Payload, false)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// --- Account -------------------------------------------------------------------

var errInvalidCredentials = fmt.Errorf("%w: Benutzername oder Passwort falsch", sdk.ErrPermissionDenied)

// authenticate prüft Anmeldedaten. Unbekannter Benutzer, falsches Passwort und
// deaktivierter Benutzer sind nach außen nicht zu unterscheiden – weder an der
// Meldung noch an der Laufzeit. Nur als Wurzelanfrage erreichbar (Ingress).
func (p *Plugin) authenticate(ctx context.Context, payload any) (sdk.Response, error) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	id, hash, active, err := p.credentials(ctx, normalizeUsername(in.Username))
	if err != nil {
		return sdk.Response{}, err
	}
	if id == "" {
		_ = bcrypt.CompareHashAndPassword(p.dummy, []byte(in.Password))
		return sdk.Response{}, errInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil || !active {
		return sdk.Response{}, errInvalidCredentials
	}
	return p.profile(ctx, id)
}

// me liefert das Profil des aufrufenden Benutzers inkl. Berechtigungen.
func (p *Plugin) me(ctx context.Context) (sdk.Response, error) {
	id := sdk.CallFromContext(ctx).UserID
	if id == "" {
		return sdk.Response{}, fmt.Errorf("%w: keine Anmeldung", sdk.ErrPermissionDenied)
	}
	return p.profile(ctx, id)
}

func (p *Plugin) profile(ctx context.Context, id string) (sdk.Response, error) {
	u, err := p.getUser(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	if !u.Active {
		return sdk.Response{}, fmt.Errorf("%w: Benutzer ist deaktiviert", sdk.ErrNotFound)
	}
	perms, err := p.userGrants(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	formatted := formatPermissions(perms)
	ps := make([]any, len(formatted))
	for i, f := range formatted {
		ps[i] = f
	}
	roles := make([]any, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = r
	}
	return sdk.Response{Payload: map[string]any{
		"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "tenant_id": u.TenantID,
		"roles": roles, "permissions": ps, "locale": u.Locale,
	}}, nil
}

func (p *Plugin) changePassword(ctx context.Context, payload any) (sdk.Response, error) {
	id := sdk.CallFromContext(ctx).UserID
	if id == "" {
		return sdk.Response{}, fmt.Errorf("%w: keine Anmeldung", sdk.ErrPermissionDenied)
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	hash, err := p.passwordHash(ctx, id)
	if err != nil {
		return sdk.Response{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Current)) != nil {
		return sdk.Response{}, fmt.Errorf("%w: Das aktuelle Passwort ist falsch", sdk.ErrInvalidArgument)
	}
	newHash, err := p.hashPassword(in.New)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{}, p.setPassword(ctx, id, newHash)
}

func (p *Plugin) hashPassword(pw string) (string, error) {
	if len([]rune(pw)) < minPasswordLen {
		return "", fmt.Errorf("%w: Das Passwort muss mindestens %d Zeichen haben", sdk.ErrInvalidArgument, minPasswordLen)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pw), p.cost)
	return string(h), err
}

// --- User (Verwaltung) -----------------------------------------------------------

func userRecord(u userRow) map[string]any {
	return map[string]any{
		"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "tenant_id": u.TenantID,
		"roles": strings.Join(u.Roles, "\n"), "active": u.Active, "created_at": u.CreatedAt,
	}
}

// userList liefert standardmäßig nur aktive Benutzer (Lebenszyklus status);
// {"query": {"includeHistory": "true"}} liefert auch inaktive.
func (p *Plugin) userList(ctx context.Context, payload any) (sdk.Response, error) {
	users, err := p.listUsers(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	all := includeHistory(payload)
	items := []any{}
	for _, u := range users {
		if u.Active || all {
			items = append(items, userRecord(u))
		}
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) userGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	u, err := p.getUser(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: userRecord(*u)}, nil
}

var usernameRe = regexp.MustCompile(`^[a-z0-9._@-]{2,64}$`)

func (p *Plugin) userSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			TenantID    string `json:"tenant_id"`
			Roles       string `json:"roles"`
			Active      *bool  `json:"active"`
			Password    string `json:"password"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	d := in.Data
	u := userRow{
		ID: in.ID, Username: normalizeUsername(d.Username),
		DisplayName: strings.TrimSpace(d.DisplayName), TenantID: strings.TrimSpace(d.TenantID),
		Active: d.Active == nil || *d.Active, Roles: lines(d.Roles),
	}
	if !usernameRe.MatchString(u.Username) {
		return sdk.Response{}, fmt.Errorf("%w: Benutzername: 2–64 Zeichen aus a–z, 0–9, . _ @ -", sdk.ErrInvalidArgument)
	}
	var hash string
	if create || d.Password != "" {
		if create && d.Password == "" {
			return sdk.Response{}, fmt.Errorf("%w: Passwort ist Pflicht", sdk.ErrInvalidArgument)
		}
		var err error
		if hash, err = p.hashPassword(d.Password); err != nil {
			return sdk.Response{}, err
		}
	}
	if !create && in.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	if !create && in.ID == sdk.CallFromContext(ctx).UserID && !u.Active {
		return sdk.Response{}, fmt.Errorf("%w: Sie können sich nicht selbst deaktivieren", sdk.ErrFailedPrecondition)
	}
	if create {
		u.ID = newID()
	}
	err := p.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if create {
			err = p.insertUser(ctx, tx, u, hash)
		} else {
			err = p.updateUser(ctx, tx, u, hash)
		}
		if err != nil {
			return err
		}
		return p.setUserRoles(ctx, tx, u.ID, u.Roles)
	})
	if err != nil {
		return sdk.Response{}, err
	}
	saved, err := p.getUser(ctx, p.pool(), u.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: userRecord(*saved)}, nil
}

// userDeactivate inaktiviert einen Benutzer (Lebenszyklus status: active =
// false). Physisch gelöscht wird nichts; die Zuordnungen bleiben erhalten.
// Wie bei jeder Änderung muss ein aktiver Administrator (*.*) bleiben.
func (p *Plugin) userDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	if id == sdk.CallFromContext(ctx).UserID {
		return sdk.Response{}, fmt.Errorf("%w: Sie können sich nicht selbst inaktivieren", sdk.ErrFailedPrecondition)
	}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, p.q(`UPDATE iam__users SET active = 0 WHERE id = ?`), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: Benutzer %q", sdk.ErrNotFound, id)
		}
		return nil
	})
	if err != nil {
		return sdk.Response{}, err
	}
	return p.userGet(ctx, payload)
}

// --- Role (Verwaltung) -----------------------------------------------------------

func roleRecord(r roleRow) map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name, "description": r.Description, "permissions": strings.Join(formatPermissions(r.Permissions), "\n")}
}

func (p *Plugin) roleList(ctx context.Context) (sdk.Response, error) {
	roles, err := p.listRoles(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(roles))
	for i, r := range roles {
		items[i] = roleRecord(r)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) roleGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	r, err := p.getRole(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: roleRecord(*r)}, nil
}

var roleNameRe = regexp.MustCompile(`^[\p{L}0-9 _.-]{2,64}$`)

func (p *Plugin) roleSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Permissions *string `json:"permissions"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	r := roleRow{ID: in.ID, Name: strings.TrimSpace(in.Data.Name), Description: strings.TrimSpace(in.Data.Description)}
	// Die Textform ersetzt alle Berechtigungen (API); das Formular pflegt sie
	// zeilenweise über RoleAuth und schickt das Feld nicht.
	if in.Data.Permissions != nil {
		perms, err := parsePermissions(*in.Data.Permissions)
		if err != nil {
			return sdk.Response{}, fmt.Errorf("%w: Berechtigungen: %v", sdk.ErrInvalidArgument, err)
		}
		r.Permissions, r.ReplaceGrants = perms, true
	}
	if !roleNameRe.MatchString(r.Name) {
		return sdk.Response{}, fmt.Errorf("%w: Rollenname: 2–64 Zeichen", sdk.ErrInvalidArgument)
	}
	if create {
		r.ID = newID()
	} else if r.ID == "" {
		return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	if err := p.inTx(ctx, func(tx *sql.Tx) error { return p.saveRole(ctx, tx, r, create) }); err != nil {
		return sdk.Response{}, err
	}
	saved, err := p.getRole(ctx, p.pool(), r.ID)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: roleRecord(*saved)}, nil
}

// --- Erster Start ------------------------------------------------------------------

// bootstrap legt die Rolle Administrator (*.*) und den ersten Benutzer an,
// wenn es noch keinen Benutzer gibt. Ohne admin_password wird ein zufälliges
// Passwort erzeugt und einmalig geloggt.
func (p *Plugin) bootstrap(ctx context.Context, host sdk.Host) {
	logCtx := sdk.WithCall(ctx, sdk.CallContext{RequestID: newID(), Metadata: map[string]string{"component": Name}})
	for attempt := 0; ; attempt++ {
		migrated, err := p.migrateLegacy(ctx)
		if migrated > 0 {
			_ = host.Log(logCtx, sdk.LogInfo, "Berechtigungen aus iam__role_permissions übernommen", map[string]string{"rows": fmt.Sprint(migrated)})
		}
		created, generated := false, ""
		if err == nil {
			created, generated, err = p.ensureAdmin(ctx)
		}
		if err == nil {
			if created {
				fields := map[string]string{"username": normalizeUsername(p.settings.AdminUser), "role": AdminRole}
				msg := "Erster Benutzer angelegt"
				if generated != "" {
					fields["password"] = generated
					msg += " – Initialpasswort bitte sofort ändern"
				}
				_ = host.Log(logCtx, sdk.LogWarn, msg, fields)
			}
			return
		}
		if attempt == 240 { // 2 Minuten: Tabellen kommen nicht
			_ = host.Log(logCtx, sdk.LogError, "Erster Benutzer konnte nicht angelegt werden", map[string]string{"err": err.Error()})
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (p *Plugin) ensureAdmin(ctx context.Context) (created bool, generated string, err error) {
	n, err := p.countUsers(ctx)
	if err != nil || n > 0 {
		return false, "", err
	}
	pw := p.settings.AdminPassword
	if pw == "" {
		raw := make([]byte, 18)
		if _, err := rand.Read(raw); err != nil {
			return false, "", err
		}
		pw = base64.RawURLEncoding.EncodeToString(raw)
		generated = pw
	}
	hash, err := p.hashPassword(pw)
	if err != nil {
		return false, "", fmt.Errorf("admin_password: %w", err)
	}
	role := roleRow{ID: newID(), Name: AdminRole, Description: "Alle Rechte", Permissions: []grant{{Object: "*", Action: "*", CompanyCodes: []string{AllCompanyCodes}, Active: true}}, ReplaceGrants: true}
	admin := userRow{ID: newID(), Username: normalizeUsername(p.settings.AdminUser), DisplayName: "Administrator",
		TenantID: p.settings.Tenant, Active: true}
	err = p.inTx(ctx, func(tx *sql.Tx) error {
		if err := p.saveRole(ctx, tx, role, true); err != nil {
			return err
		}
		if err := p.insertUser(ctx, tx, admin, hash); err != nil {
			return err
		}
		return p.setUserRoles(ctx, tx, admin.ID, []string{AdminRole})
	})
	return err == nil, generated, err
}

// --- Hilfsfunktionen ---------------------------------------------------------------

func normalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func lines(text string) []string {
	var out []string
	for _, l := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ',' || r == '\r' }) {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func idParam(payload any) (string, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return "", err
	}
	if in.ID == "" {
		return "", fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
	}
	return in.ID, nil
}

func newID() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// --- Rechteprüfung (Account.Check / Account.Granted) ------------------------------

type accessQuery struct {
	Object      string            `json:"object"`
	Action      string            `json:"action"`
	CompanyCode string            `json:"company_code"` // Kurzform für attrs.company_code
	Attrs       map[string]string `json:"attrs"`
}

func decodeAccess(payload any) (accessQuery, error) {
	var q accessQuery
	if err := sdk.Decode(payload, &q); err != nil {
		return q, err
	}
	if q.Object == "" || q.Action == "" {
		return q, fmt.Errorf("%w: object und action sind Pflicht", sdk.ErrInvalidArgument)
	}
	if q.Attrs == nil {
		q.Attrs = map[string]string{}
	}
	if q.CompanyCode != "" {
		q.Attrs[sdk.AttrCompanyCode] = q.CompanyCode
	}
	return q, nil
}

// check: Darf der aufrufende Benutzer object.action mit den Werten attrs?
// Der Benutzer kommt aus dem CallContext (vom Dispatcher gesetzt, nicht
// fälschbar). System-Anfragen ohne Benutzer dürfen alles – wie im Dispatcher.
func (p *Plugin) check(ctx context.Context, payload any) (sdk.Response, error) {
	q, err := decodeAccess(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	allowed := true
	if uid := sdk.CallFromContext(ctx).UserID; uid != "" {
		if allowed, err = p.Check(ctx, uid, q.Object, q.Action, q.Attrs); err != nil {
			return sdk.Response{}, err
		}
	}
	return sdk.Response{Payload: map[string]any{"allowed": allowed}}, nil
}

// granted: alle Erlaubnisse des Benutzers für object.action (sdk.GrantSet).
func (p *Plugin) granted(ctx context.Context, payload any) (sdk.Response, error) {
	q, err := decodeAccess(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	g := systemGrants
	if uid := sdk.CallFromContext(ctx).UserID; uid != "" {
		if g, err = p.Granted(ctx, uid, q.Object, q.Action); err != nil {
			return sdk.Response{}, err
		}
	}
	return sdk.Response{Payload: g}, nil
}

// --- CompanyCode (Verwaltung) ------------------------------------------------------

func ccRecord(c companyCode) map[string]any {
	return map[string]any{"id": c.ID, "code": c.ID, "description": c.Description}
}

func (p *Plugin) ccList(ctx context.Context) (sdk.Response, error) {
	ccs, err := p.listCompanyCodes(ctx)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(ccs))
	for i, c := range ccs {
		items[i] = ccRecord(c)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) ccGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	c, err := p.getCompanyCode(ctx, id)
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: ccRecord(*c)}, nil
}

func (p *Plugin) ccSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string `json:"id"`
		Data struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	c := companyCode{ID: strings.TrimSpace(in.Data.Code), Description: strings.TrimSpace(in.Data.Description)}
	if !create {
		if in.ID == "" {
			return sdk.Response{}, fmt.Errorf("%w: id fehlt", sdk.ErrInvalidArgument)
		}
		if c.ID != "" && c.ID != in.ID {
			return sdk.Response{}, fmt.Errorf("%w: Die Nummer eines Buchungskreises kann nicht geändert werden", sdk.ErrInvalidArgument)
		}
		c.ID = in.ID
	}
	if c.ID == AllCompanyCodes || !companyCodeRe.MatchString(c.ID) {
		return sdk.Response{}, fmt.Errorf("%w: Buchungskreis: 1–20 Zeichen aus A–Z, a–z, 0–9, _ und -", sdk.ErrInvalidArgument)
	}
	if err := p.saveCompanyCode(ctx, c, create); err != nil {
		return sdk.Response{}, err
	}
	p.invalidate()
	return sdk.Response{Payload: ccRecord(c)}, nil
}

// updateProfile ändert die Einstellungen des eigenen Kontos: {"locale": "de"|"en"|"zh-CN"|""}.
// Leer = automatisch (Sprachwähler, Accept-Language, Standard).
func (p *Plugin) updateProfile(ctx context.Context, payload any) (sdk.Response, error) {
	id := sdk.CallFromContext(ctx).UserID
	if id == "" {
		return sdk.Response{}, fmt.Errorf("%w: keine Anmeldung", sdk.ErrPermissionDenied)
	}
	var in struct {
		Locale *string `json:"locale"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if in.Locale != nil {
		loc := metamodel.NormalizeLocale(*in.Locale)
		if *in.Locale != "" && loc == "" {
			return sdk.Response{}, fmt.Errorf("%w: Sprache %q nicht unterstützt (%v)", sdk.ErrInvalidArgument, *in.Locale, metamodel.Locales)
		}
		var value any
		if loc != "" {
			value = loc
		}
		if _, err := p.pool().ExecContext(ctx, p.q(`UPDATE iam__users SET locale = ?, updated_at = ? WHERE id = ?`),
			value, time.Now().UTC().Format(time.RFC3339), id); err != nil {
			return sdk.Response{}, err
		}
	}
	return p.profile(ctx, id)
}

// includeHistory: {"query": {"includeHistory": "true"}} (oder direkt im Payload).
func includeHistory(payload any) bool {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		m = q
	}
	switch v := m["includeHistory"].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "on"
	}
	return false
}
