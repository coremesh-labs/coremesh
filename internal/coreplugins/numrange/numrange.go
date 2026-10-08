// Package numrange ist das Core-Plugin "numrange": Nummernkreise für alle
// Module (Schnittstelle: pkg/sdk/numrange).
//
// Object NumberRange:
//   - Define {object, description, owner, per_company_code, per_year, pattern,
//     width, from, to, overflow, warn_percent} – ein Modul meldet sein
//     Nummernkreis-Objekt mit Standardwerten an (bei jedem Start).
//   - Next {object, company_code, key, year, reference} – vergibt die nächste
//     Nummer: atomar (ein UPDATE in einer Transaktion), formatiert, protokolliert.
//     Fehlt das Intervall, entsteht es aus dem Vorjahr desselben Schlüssels,
//     sonst aus den Standardwerten.
//   - list, get, create, update, deactivate – Pflege der Intervalle.
//
// Object NumberRangeObject: list, get (angemeldete Objekte).
// Object NumberRangeLog: list, get (vergebene Nummern).
package numrange

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/internal/txctx"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

const (
	Name    = "numrange"
	Version = "0.2.0"

	defaultKey = "01"
	allCC      = "*"
	maxValue   = int64(999_999_999_999_999) // 15 Stellen
	logLimit   = 500
)

type settings struct {
	Database string `json:"database"`
}

type Plugin struct {
	db       *database.Manager
	host     sdk.Host
	settings settings
	// mu serialisiert die Vergabe im Prozess (SQLite erlaubt nur einen Schreiber;
	// parallele Transaktionen scheiterten sonst mit SQLITE_BUSY). Über Prozesse
	// hinweg schützt das bedingte UPDATE in draw.
	mu sync.Mutex
}

var (
	_ sdk.Plugin = (*Plugin)(nil)

	objectRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	keyRe    = regexp.MustCompile(`^[A-Z0-9_]{1,12}$`)
	ccRe     = regexp.MustCompile(`^(\*|[A-Za-z0-9_-]{1,10})$`)
)

func New(db *database.Manager) *Plugin { return &Plugin{db: db} }

func (p *Plugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name: Name, Version: Version, Description: "Nummernkreise: fortlaufende Nummern für alle Module",
		Capabilities: []sdk.Capability{
			{Object: api.Object, Actions: []string{api.ActionDefine, api.ActionNext, "list", "get", "create", "update", "deactivate"},
				Description: "Nummernkreise anmelden, Nummern vergeben, Intervalle pflegen"},
			{Object: "NumberRangeObject", Actions: []string{"list", "get"}, Description: "Angemeldete Nummernkreis-Objekte"},
			{Object: "NumberRangeLog", Actions: []string{"list", "get"}, Description: "Vergebene Nummern"},
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
			{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
		},
	}, nil
}

func (p *Plugin) Configure(ctx context.Context, cfg sdk.Config) error {
	s := settings{Database: "main"}
	if err := sdk.Decode(cfg.Settings, &s); err != nil {
		return err
	}
	if _, err := p.db.DB(s.Database); err != nil {
		return err
	}
	p.settings, p.host = s, cfg.Host
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
			Objects: []metamodel.ObjectDefinition{metamodel.WithKeys("numranges", intervalDef),
				metamodel.WithKeys("numranges", objectDef), metamodel.WithKeys("numranges", logDef)},
			Modules:      []metamodel.ModuleDefinition{metamodel.ModuleKeys(numrangeModule)},
			Translations: translations,
		}}, nil
	case api.Object + "." + api.ActionDefine:
		return p.define(ctx, req.Payload)
	case api.Object + "." + api.ActionNext:
		return p.next(ctx, req.Payload)
	case api.Object + ".list":
		return p.intervalList(ctx, req.Payload)
	case api.Object + ".get":
		return p.intervalGet(ctx, req.Payload)
	case api.Object + ".create":
		return p.intervalSave(ctx, req.Payload, true)
	case api.Object + ".update":
		return p.intervalSave(ctx, req.Payload, false)
	case api.Object + ".deactivate":
		return p.intervalDeactivate(ctx, req.Payload)
	case "NumberRangeObject.list":
		return p.objectList(ctx)
	case "NumberRangeObject.get":
		return p.objectGet(ctx, req.Payload)
	case "NumberRangeLog.list":
		return p.logList(ctx, req.Payload)
	case "NumberRangeLog.get":
		return p.logGet(ctx, req.Payload)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// --- Objekt und Intervall --------------------------------------------------------------

type objectRow struct {
	Object, Owner, Description, Pattern, Overflow, UpdatedAt string
	PerCompanyCode, PerYear, GapFree, Disjoint               bool
	Width, WarnPercent                                       int
	From, To                                                 int64
}

type interval struct {
	ID, Object, CompanyCode, Key, Description, Pattern, Overflow, NextKey, UpdatedAt string
	Year, Width, WarnPercent                                                        int
	From, To, Current                                                               int64
	Active                                                                          bool
}

// format setzt die Platzhalter ein.
func (iv interval) format(n int64) string {
	num := strconv.FormatInt(n, 10)
	if pad := iv.Width - len(num); pad > 0 {
		num = strings.Repeat("0", pad) + num
	}
	yy := ""
	if iv.Year > 0 {
		yy = fmt.Sprintf("%02d", iv.Year%100)
	}
	r := strings.NewReplacer(api.PlaceKey, iv.Key, api.PlaceCompanyCode, iv.CompanyCode,
		api.PlaceYear, yearText(iv.Year), api.PlaceYear2, yy, api.PlaceNumber, num)
	return r.Replace(iv.Pattern)
}

func yearText(y int) string {
	if y == 0 {
		return ""
	}
	return strconv.Itoa(y)
}

// nextValue: die Nummer, die der nächste Abruf vergäbe (0 = Intervall voll).
func (iv interval) nextValue() int64 {
	if iv.Current < iv.From {
		return iv.From
	}
	if iv.Current >= iv.To {
		return 0
	}
	return iv.Current + 1
}

func (iv interval) usedPercent() int64 {
	if iv.Current < iv.From {
		return 0
	}
	return (iv.Current - iv.From + 1) * 100 / (iv.To - iv.From + 1)
}

// check prüft die Einstellungen eines Intervalls.
func (iv *interval) check() error {
	switch {
	case iv.From < 1:
		return fmt.Errorf("von muss mindestens 1 sein")
	case iv.To < iv.From:
		return fmt.Errorf("bis (%d) liegt vor von (%d)", iv.To, iv.From)
	case iv.To > maxValue:
		return fmt.Errorf("bis höchstens %d", maxValue)
	case iv.Width < 0 || iv.Width > 15:
		return fmt.Errorf("Stellenzahl 0 bis 15")
	case iv.Width > 0 && len(strconv.FormatInt(iv.To, 10)) > iv.Width:
		return fmt.Errorf("bis (%d) hat mehr als %d Stellen", iv.To, iv.Width)
	case strings.Count(iv.Pattern, api.PlaceNumber) != 1:
		return fmt.Errorf("Format braucht genau einmal %s, z. B. %s-%s-%s", api.PlaceNumber, api.PlaceKey, api.PlaceYear, api.PlaceNumber)
	case !slices.Contains([]string{api.OverflowError, api.OverflowRestart, api.OverflowNext}, iv.Overflow):
		return fmt.Errorf("bei Überlauf: %s, %s oder %s", api.OverflowError, api.OverflowRestart, api.OverflowNext)
	case iv.Overflow == api.OverflowNext && (iv.NextKey == "" || iv.NextKey == iv.Key):
		return fmt.Errorf("Folgeintervall (anderer Schlüssel) ist bei %s Pflicht", api.OverflowNext)
	case iv.WarnPercent < 0 || iv.WarnPercent > 100:
		return fmt.Errorf("Warnschwelle 0 bis 100")
	case iv.Current != 0 && (iv.Current < iv.From-1 || iv.Current > iv.To):
		return fmt.Errorf("Stand %d liegt außerhalb des Intervalls %d–%d", iv.Current, iv.From, iv.To)
	}
	if iv.Year == 0 && (strings.Contains(iv.Pattern, api.PlaceYear) || strings.Contains(iv.Pattern, api.PlaceYear2)) {
		return fmt.Errorf("Format mit Jahr, aber Intervall ohne Jahr")
	}
	return nil
}

func intervalID(object, cc, key string, year int) string {
	return object + "|" + cc + "|" + key + "|" + strconv.Itoa(year)
}

// --- Define ----------------------------------------------------------------------------

func (p *Plugin) define(ctx context.Context, payload any) (sdk.Response, error) {
	var d api.Definition
	if err := sdk.Decode(payload, &d); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	d.Object = strings.TrimSpace(d.Object)
	if !objectRe.MatchString(d.Object) {
		return sdk.Response{}, fmt.Errorf("%w: Objekt %q: PascalCase, z. B. Contract", sdk.ErrInvalidArgument, d.Object)
	}
	defaults(&d)
	tmpl := interval{Object: d.Object, Key: defaultKey, Pattern: d.Pattern, Width: d.Width, From: d.From, To: d.To,
		Overflow: d.Overflow, WarnPercent: d.WarnPercent}
	if d.PerYear {
		tmpl.Year = 2000
	}
	if d.Overflow == api.OverflowNext {
		return sdk.Response{}, fmt.Errorf("%w: Standard bei Überlauf: %s oder %s (Folgeintervalle pflegt der Administrator)",
			sdk.ErrInvalidArgument, api.OverflowError, api.OverflowRestart)
	}
	if err := tmpl.check(); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %s: %v", sdk.ErrInvalidArgument, d.Object, err)
	}
	now := ts()
	res, err := p.pool().ExecContext(ctx, p.q(`UPDATE numrange__object SET owner = ?, description = ?, per_company_code = ?, per_year = ?,
		pattern = ?, width = ?, from_number = ?, to_number = ?, overflow = ?, warn_percent = ?, gap_free = ?, disjoint = ?, updated_at = ? WHERE object = ?`),
		d.Owner, d.Description, b2i(d.PerCompanyCode), b2i(d.PerYear), d.Pattern, d.Width, d.From, d.To, d.Overflow, d.WarnPercent,
		b2i(d.GapFree), b2i(d.Disjoint), now, d.Object)
	if err != nil {
		return sdk.Response{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := p.pool().ExecContext(ctx, p.q(`INSERT INTO numrange__object (object, owner, description, per_company_code, per_year,
			pattern, width, from_number, to_number, overflow, warn_percent, gap_free, disjoint, defined_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
			d.Object, d.Owner, d.Description, b2i(d.PerCompanyCode), b2i(d.PerYear), d.Pattern, d.Width, d.From, d.To, d.Overflow, d.WarnPercent,
			b2i(d.GapFree), b2i(d.Disjoint), now, now); err != nil {
			return sdk.Response{}, err
		}
	}
	p.log(ctx, sdk.LogInfo, "Nummernkreis angemeldet", map[string]string{"object": d.Object, "owner": d.Owner})
	return sdk.Response{Payload: map[string]any{"object": d.Object}}, nil
}

func defaults(d *api.Definition) {
	if strings.TrimSpace(d.Pattern) == "" {
		d.Pattern = api.PlaceNumber
	}
	if d.From == 0 {
		d.From = 1
	}
	if d.To == 0 {
		d.To = 999_999_999
		if d.Width > 0 && d.Width < 16 {
			d.To = pow10(d.Width) - 1
		}
	}
	if d.Overflow == "" {
		d.Overflow = api.OverflowError
	}
	if d.WarnPercent == 0 {
		d.WarnPercent = 90
	}
}

func pow10(n int) int64 {
	v := int64(1)
	for range n {
		v *= 10
	}
	return v
}

func (p *Plugin) loadObject(ctx context.Context, q database.Querier, object string) (*objectRow, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT object, owner, description, per_company_code, per_year, pattern, width,
		from_number, to_number, overflow, warn_percent, updated_at, gap_free, disjoint FROM numrange__object WHERE object = ?`), object)
	if err != nil || len(res.Rows) == 0 {
		return nil, err
	}
	r := res.Rows[0]
	return &objectRow{Object: s(r[0]), Owner: s(r[1]), Description: s(r[2]), PerCompanyCode: i64(r[3]) != 0, PerYear: i64(r[4]) != 0,
		Pattern: s(r[5]), Width: int(i64(r[6])), From: i64(r[7]), To: i64(r[8]), Overflow: s(r[9]), WarnPercent: int(i64(r[10])), UpdatedAt: s(r[11]),
		GapFree: i64(r[12]) != 0, Disjoint: i64(r[13]) != 0}, nil
}

// --- Next ------------------------------------------------------------------------------

func (p *Plugin) next(ctx context.Context, payload any) (sdk.Response, error) {
	var r api.Request
	if err := sdk.Decode(payload, &r); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	obj, err := p.loadObject(ctx, p.pool(), strings.TrimSpace(r.Object))
	if err != nil {
		return sdk.Response{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var out api.Result
	draw := func(q database.Querier) error {
		var err error
		out, err = p.draw(ctx, q, r)
		return err
	}
	if obj != nil && obj.GapFree {
		// Lückenlos: in der Transaktion des Aufrufers – ein Rollback gibt die Nummer zurück.
		txID, ok := txctx.Get(ctx, p.settings.Database)
		if !ok {
			return sdk.Response{}, fmt.Errorf("%w: Nummernkreis %s ist lückenlos – Next nur in einer Transaktion (sdk.InTx) auf %s",
				sdk.ErrFailedPrecondition, obj.Object, p.settings.Database)
		}
		err = p.db.WithTx(sdk.CallFromContext(ctx).RequestID, p.settings.Database, txID, draw)
	} else {
		err = p.inTx(ctx, func(tx *sql.Tx) error { return draw(tx) })
	}
	if err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: out}, nil
}

// draw vergibt die Nummer in der Transaktion tx.
func (p *Plugin) draw(ctx context.Context, tx database.Querier, r api.Request) (api.Result, error) {
	obj, err := p.loadObject(ctx, tx, strings.TrimSpace(r.Object))
	if err != nil {
		return api.Result{}, err
	}
	if obj == nil {
		return api.Result{}, fmt.Errorf("%w: Nummernkreis-Objekt %q ist nicht angemeldet", sdk.ErrNotFound, r.Object)
	}
	cc, key, year := allCC, strings.ToUpper(strings.TrimSpace(r.Key)), 0
	if key == "" {
		key = defaultKey
	}
	if !keyRe.MatchString(key) {
		return api.Result{}, fmt.Errorf("%w: Intervallschlüssel %q: A–Z, 0–9, _ (höchstens 12)", sdk.ErrInvalidArgument, key)
	}
	if obj.PerCompanyCode {
		if cc = strings.TrimSpace(r.CompanyCode); cc == "" {
			return api.Result{}, fmt.Errorf("%w: %s: Buchungskreis ist Pflicht", sdk.ErrInvalidArgument, obj.Object)
		}
	}
	if obj.PerYear {
		if year = r.Year; year == 0 {
			year = time.Now().Year()
		}
	}
	var warnings []string
	seen := map[string]bool{}
	for {
		iv, err := p.findOrCreate(ctx, tx, obj, cc, key, year)
		if err != nil {
			return api.Result{}, err
		}
		if seen[iv.ID] {
			return api.Result{}, fmt.Errorf("%w: Folgeintervalle von %s bilden einen Kreis", sdk.ErrInvalidArgument, iv.ID)
		}
		seen[iv.ID] = true
		value := iv.nextValue()
		if value == 0 { // Überlauf
			switch iv.Overflow {
			case api.OverflowNext:
				warnings = append(warnings, fmt.Sprintf("Intervall %s ist voll – weiter in %s", iv.Key, iv.NextKey))
				key = iv.NextKey
				continue
			case api.OverflowRestart:
				value = iv.From
				warnings = append(warnings, fmt.Sprintf("Intervall %s war voll – beginnt wieder bei %d", iv.Key, iv.From))
			default:
				return api.Result{}, fmt.Errorf("%w: Nummernkreis %s ist erschöpft (%d–%d) – Intervall erweitern oder Folgeintervall einrichten",
					sdk.ErrResourceExhausted, iv.ID, iv.From, iv.To)
			}
		}
		// Atomar: Der Stand ändert sich nur, wenn er noch der gelesene ist.
		res, err := tx.ExecContext(ctx, p.q(`UPDATE numrange__interval SET current_number = ?, updated_at = ? WHERE id = ? AND current_number = ?`),
			value, ts(), iv.ID, iv.Current)
		if err != nil {
			return api.Result{}, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			delete(seen, iv.ID) // gleichzeitig vergeben – neu lesen
			continue
		}
		iv.Current = value
		number := iv.format(value)
		call := sdk.CallFromContext(ctx)
		if _, err := tx.ExecContext(ctx, p.q(`INSERT INTO numrange__log (id, interval_id, object, company_code, range_key, year, value, number, reference, user_id, request_id, drawn_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`), newID(), iv.ID, iv.Object, iv.CompanyCode, iv.Key, iv.Year, value, number,
			nullable(r.Reference), nullable(call.UserID), nullable(call.RequestID), ts()); err != nil {
			return api.Result{}, err
		}
		if iv.WarnPercent > 0 && iv.usedPercent() >= int64(iv.WarnPercent) {
			warnings = append(warnings, fmt.Sprintf("Nummernkreis %s ist zu %d %% belegt (%d von %d–%d)", iv.ID, iv.usedPercent(), value, iv.From, iv.To))
		}
		return api.Result{Number: number, Value: value, Interval: iv.ID, Warning: strings.Join(warnings, "; ")}, nil
	}
}

// findOrCreate: Intervall des Buchungskreises, sonst für alle (*); fehlt es,
// entsteht es aus dem jüngsten Vorjahr desselben Schlüssels oder aus den
// Standardwerten des Objekts.
func (p *Plugin) findOrCreate(ctx context.Context, tx database.Querier, obj *objectRow, cc, key string, year int) (*interval, error) {
	for _, c := range slices.Compact([]string{cc, allCC}) {
		ivs, err := p.loadIntervals(ctx, tx, "WHERE object = ? AND company_code = ? AND range_key = ? AND year = ?", obj.Object, c, key, year)
		if err != nil {
			return nil, err
		}
		if len(ivs) > 0 {
			if !ivs[0].Active {
				return nil, fmt.Errorf("%w: Nummernkreis %s ist inaktiv", sdk.ErrFailedPrecondition, ivs[0].ID)
			}
			return &ivs[0], nil
		}
	}
	iv := interval{Object: obj.Object, CompanyCode: cc, Key: key, Year: year, Pattern: obj.Pattern, Width: obj.Width,
		From: obj.From, To: obj.To, Overflow: obj.Overflow, WarnPercent: obj.WarnPercent, Active: true}
	if year > 0 {
		prev, err := p.loadIntervals(ctx, tx, "WHERE object = ? AND company_code IN (?, ?) AND range_key = ? AND year < ? ORDER BY year DESC, company_code DESC",
			obj.Object, cc, allCC, key, year)
		if err != nil {
			return nil, err
		}
		if len(prev) > 0 {
			pv := prev[0]
			iv.CompanyCode, iv.Description, iv.Pattern, iv.Width = pv.CompanyCode, pv.Description, pv.Pattern, pv.Width
			iv.From, iv.To, iv.Overflow, iv.NextKey, iv.WarnPercent = pv.From, pv.To, pv.Overflow, pv.NextKey, pv.WarnPercent
		}
	}
	iv.ID = intervalID(iv.Object, iv.CompanyCode, iv.Key, iv.Year)
	if obj.Disjoint {
		if err := p.checkDisjoint(ctx, tx, iv); err != nil {
			return nil, fmt.Errorf("%w (Intervall für Schlüssel %s im Jahr %d unter Nummernkreise mit eigenem Bereich anlegen)", err, iv.Key, iv.Year)
		}
	}
	now := ts()
	if _, err := tx.ExecContext(ctx, p.q(`INSERT INTO numrange__interval (id, object, company_code, range_key, year, description, from_number, to_number,
		current_number, width, pattern, overflow, next_key, warn_percent, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, 1, ?, ?)`),
		iv.ID, iv.Object, iv.CompanyCode, iv.Key, iv.Year, nullable(iv.Description), iv.From, iv.To, iv.Width, iv.Pattern, iv.Overflow,
		nullable(iv.NextKey), iv.WarnPercent, now, now); err != nil {
		return nil, err
	}
	p.log(ctx, sdk.LogInfo, "Nummernkreis-Intervall angelegt", map[string]string{"id": iv.ID})
	return &iv, nil
}

// checkDisjoint: Bei überschneidungsfreien Objekten darf das Intervall keines
// eines anderen Schlüssels im selben Buchungskreis (oder *) und Jahr überlappen.
func (p *Plugin) checkDisjoint(ctx context.Context, q database.Querier, iv interval) error {
	others, err := p.loadIntervals(ctx, q, `WHERE object = ? AND company_code IN (?, ?) AND year = ? AND range_key <> ?
		AND from_number <= ? AND to_number >= ?`, iv.Object, iv.CompanyCode, allCC, iv.Year, iv.Key, iv.To, iv.From)
	if err != nil {
		return err
	}
	if len(others) > 0 {
		o := others[0]
		return fmt.Errorf("%w: %s %d–%d überschneidet sich mit Schlüssel %s (%d–%d) – Intervalle dieses Objekts müssen überschneidungsfrei sein",
			sdk.ErrInvalidArgument, iv.ID, iv.From, iv.To, o.Key, o.From, o.To)
	}
	return nil
}

const intervalCols = `id, object, company_code, range_key, year, description, from_number, to_number, current_number, width, pattern,
	overflow, next_key, warn_percent, active, updated_at`

func (p *Plugin) loadIntervals(ctx context.Context, q database.Querier, rest string, args ...any) ([]interval, error) {
	res, err := database.Query(ctx, q, p.q(`SELECT `+intervalCols+` FROM numrange__interval `+rest), args...)
	if err != nil {
		return nil, err
	}
	out := make([]interval, len(res.Rows))
	for i, r := range res.Rows {
		out[i] = interval{ID: s(r[0]), Object: s(r[1]), CompanyCode: s(r[2]), Key: s(r[3]), Year: int(i64(r[4])), Description: s(r[5]),
			From: i64(r[6]), To: i64(r[7]), Current: i64(r[8]), Width: int(i64(r[9])), Pattern: s(r[10]), Overflow: s(r[11]),
			NextKey: s(r[12]), WarnPercent: int(i64(r[13])), Active: i64(r[14]) != 0, UpdatedAt: s(r[15])}
	}
	return out, nil
}

// --- Hilfsfunktionen -------------------------------------------------------------------

func (p *Plugin) pool() *sql.DB {
	db, _ := p.db.DB(p.settings.Database)
	return db
}

func (p *Plugin) q(query string) string { return database.Rebind(p.db.Driver(p.settings.Database), query) }

func (p *Plugin) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.pool().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (p *Plugin) log(ctx context.Context, level sdk.LogLevel, msg string, fields map[string]string) {
	if p.host != nil {
		_ = p.host.Log(ctx, level, msg, fields)
	}
}

func ts() string { return time.Now().UTC().Format(time.RFC3339) }

func s(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case float64:
		// Zahlen aus JSON: 1000000001 nicht als 1.000000001e+09
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func i64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	}
	n, _ := strconv.ParseInt(s(v), 10, 64)
	return n
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var errNoID = errors.New("id fehlt")

func idParam(payload any) (string, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return "", err
	}
	if in.ID == "" {
		return "", fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, errNoID)
	}
	return in.ID, nil
}

// query liest Filter aus {"query": {...}} oder direkt aus dem Payload.
func query(payload any) map[string]string {
	m, _ := payload.(map[string]any)
	if q, ok := m["query"].(map[string]any); ok {
		m = q
	}
	out := map[string]string{}
	for k, v := range m {
		switch v := v.(type) {
		case string:
			out[k] = v
		case bool:
			out[k] = strconv.FormatBool(v)
		}
	}
	return out
}
