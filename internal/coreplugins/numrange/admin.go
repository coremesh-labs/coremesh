package numrange

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coremesh-labs/coremesh/internal/database"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
	api "github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// --- Intervalle ------------------------------------------------------------------------

func (iv interval) record() map[string]any {
	next := ""
	if v := iv.nextValue(); v > 0 {
		next = iv.format(v)
	} else if iv.Overflow == api.OverflowRestart {
		next = iv.format(iv.From) + " (Neubeginn)"
	} else {
		next = "– erschöpft –"
	}
	return map[string]any{"id": iv.ID, "object": iv.Object, "company_code": iv.CompanyCode, "range_key": iv.Key, "year": iv.Year,
		"description": iv.Description, "from_number": iv.From, "to_number": iv.To, "current_number": iv.Current,
		"used_percent": iv.usedPercent(), "width": iv.Width, "pattern": iv.Pattern, "next_number": next,
		"overflow": iv.Overflow, "next_key": iv.NextKey, "warn_percent": iv.WarnPercent, "active": iv.Active, "updated_at": iv.UpdatedAt}
}

func (p *Plugin) intervalList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	var conds []string
	var args []any
	for _, k := range []string{"object", "company_code", "range_key"} {
		if v := q[k]; v != "" {
			col := k
			conds, args = append(conds, col+" = ?"), append(args, v)
		}
	}
	if !includeHistory(q) {
		conds = append(conds, "active = 1")
	}
	rest := ""
	if len(conds) > 0 {
		rest = "WHERE " + strings.Join(conds, " AND ")
	}
	ivs, err := p.loadIntervals(ctx, p.pool(), rest+" ORDER BY object, company_code, range_key, year", args...)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(ivs))
	for i, iv := range ivs {
		items[i] = iv.record()
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) intervalGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	ivs, err := p.loadIntervals(ctx, p.pool(), "WHERE id = ?", id)
	if err != nil {
		return sdk.Response{}, err
	}
	if len(ivs) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Nummernkreis %q", sdk.ErrNotFound, id)
	}
	return sdk.Response{Payload: ivs[0].record()}, nil
}

// intervalSave: create (alle Felder) bzw. update (Objekt, Buchungskreis,
// Schlüssel und Jahr bleiben).
func (p *Plugin) intervalSave(ctx context.Context, payload any, create bool) (sdk.Response, error) {
	var in struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	if err := sdk.Decode(payload, &in); err != nil {
		return sdk.Response{}, err
	}
	str := func(k string) string { return strings.TrimSpace(s(in.Data[k])) }
	has := func(k string) bool { _, ok := in.Data[k]; return ok }
	num := func(k string, cur int64) (int64, error) {
		if !has(k) || str(k) == "" {
			return cur, nil
		}
		n, err := strconv.ParseInt(strings.TrimSuffix(str(k), ".0"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s: ganze Zahl erwartet", sdk.ErrInvalidArgument, k)
		}
		return n, nil
	}
	var iv interval
	if create {
		obj, err := p.loadObject(ctx, p.pool(), str("object"))
		if err != nil {
			return sdk.Response{}, err
		}
		if obj == nil {
			return sdk.Response{}, fmt.Errorf("%w: Nummernkreis-Objekt %q ist nicht angemeldet", sdk.ErrInvalidArgument, str("object"))
		}
		iv = interval{Object: obj.Object, CompanyCode: str("company_code"), Key: strings.ToUpper(str("range_key")), Pattern: obj.Pattern,
			Width: obj.Width, From: obj.From, To: obj.To, Overflow: obj.Overflow, WarnPercent: obj.WarnPercent, Active: true}
		if iv.CompanyCode == "" {
			iv.CompanyCode = allCC
		}
		year, err := num("year", 0)
		if err != nil {
			return sdk.Response{}, err
		}
		iv.Year = int(year)
		switch {
		case !keyRe.MatchString(iv.Key):
			return sdk.Response{}, fmt.Errorf("%w: Intervallschlüssel %q: A–Z, 0–9, _ (höchstens 12)", sdk.ErrInvalidArgument, iv.Key)
		case !ccRe.MatchString(iv.CompanyCode):
			return sdk.Response{}, fmt.Errorf("%w: Buchungskreis %q", sdk.ErrInvalidArgument, iv.CompanyCode)
		case !obj.PerCompanyCode && iv.CompanyCode != allCC:
			return sdk.Response{}, fmt.Errorf("%w: %s gilt nicht je Buchungskreis – Buchungskreis *", sdk.ErrInvalidArgument, obj.Object)
		case obj.PerYear && (iv.Year < 1900 || iv.Year > 2999):
			return sdk.Response{}, fmt.Errorf("%w: %s gilt je Jahr – Jahr angeben", sdk.ErrInvalidArgument, obj.Object)
		case !obj.PerYear && iv.Year != 0:
			return sdk.Response{}, fmt.Errorf("%w: %s gilt nicht je Jahr – Jahr 0", sdk.ErrInvalidArgument, obj.Object)
		}
		if iv.CompanyCode != allCC {
			if err := p.requireCompanyCode(ctx, iv.CompanyCode); err != nil {
				return sdk.Response{}, err
			}
		}
		iv.ID = intervalID(iv.Object, iv.CompanyCode, iv.Key, iv.Year)
	} else {
		ivs, err := p.loadIntervals(ctx, p.pool(), "WHERE id = ?", in.ID)
		if err != nil {
			return sdk.Response{}, err
		}
		if len(ivs) == 0 {
			return sdk.Response{}, fmt.Errorf("%w: Nummernkreis %q", sdk.ErrNotFound, in.ID)
		}
		iv = ivs[0]
	}
	var err error
	if iv.From, err = num("from_number", iv.From); err != nil {
		return sdk.Response{}, err
	}
	if iv.To, err = num("to_number", iv.To); err != nil {
		return sdk.Response{}, err
	}
	if iv.Current, err = num("current_number", iv.Current); err != nil {
		return sdk.Response{}, err
	}
	width, err := num("width", int64(iv.Width))
	if err != nil {
		return sdk.Response{}, err
	}
	warn, err := num("warn_percent", int64(iv.WarnPercent))
	if err != nil {
		return sdk.Response{}, err
	}
	iv.Width, iv.WarnPercent = int(width), int(warn)
	for k, dst := range map[string]*string{"description": &iv.Description, "pattern": &iv.Pattern, "overflow": &iv.Overflow, "next_key": &iv.NextKey} {
		if has(k) {
			*dst = str(k)
		}
	}
	iv.NextKey = strings.ToUpper(iv.NextKey)
	if iv.NextKey != "" && !keyRe.MatchString(iv.NextKey) {
		return sdk.Response{}, fmt.Errorf("%w: Folgeintervall %q", sdk.ErrInvalidArgument, iv.NextKey)
	}
	if err := iv.check(); err != nil {
		return sdk.Response{}, fmt.Errorf("%w: %v", sdk.ErrInvalidArgument, err)
	}
	now := ts()
	if create {
		if dup, err := p.loadIntervals(ctx, p.pool(), "WHERE id = ?", iv.ID); err != nil {
			return sdk.Response{}, err
		} else if len(dup) > 0 {
			return sdk.Response{}, fmt.Errorf("%w: Nummernkreis %s gibt es schon", sdk.ErrAlreadyExists, iv.ID)
		}
		_, err = p.pool().ExecContext(ctx, p.q(`INSERT INTO numrange__interval (id, object, company_code, range_key, year, description, from_number, to_number,
			current_number, width, pattern, overflow, next_key, warn_percent, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`),
			iv.ID, iv.Object, iv.CompanyCode, iv.Key, iv.Year, nullable(iv.Description), iv.From, iv.To, iv.Current, iv.Width, iv.Pattern,
			iv.Overflow, nullable(iv.NextKey), iv.WarnPercent, now, now)
	} else {
		_, err = p.pool().ExecContext(ctx, p.q(`UPDATE numrange__interval SET description = ?, from_number = ?, to_number = ?, current_number = ?, width = ?,
			pattern = ?, overflow = ?, next_key = ?, warn_percent = ?, active = ?, updated_at = ? WHERE id = ?`),
			nullable(iv.Description), iv.From, iv.To, iv.Current, iv.Width, iv.Pattern, iv.Overflow, nullable(iv.NextKey), iv.WarnPercent,
			b2i(iv.Active || has("active") && truthy(in.Data["active"])), now, iv.ID)
	}
	if err != nil {
		return sdk.Response{}, err
	}
	p.log(ctx, sdk.LogInfo, "Nummernkreis gepflegt", map[string]string{"id": iv.ID, "stand": strconv.FormatInt(iv.Current, 10)})
	return p.intervalGet(ctx, map[string]any{"id": iv.ID})
}

func (p *Plugin) intervalDeactivate(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	res, err := p.pool().ExecContext(ctx, p.q(`UPDATE numrange__interval SET active = 0, updated_at = ? WHERE id = ?`), ts(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Nummernkreis %q", sdk.ErrNotFound, id)
	}
	return p.intervalGet(ctx, payload)
}

// requireCompanyCode: Buchungskreis aus iam (ohne iam keine Prüfung).
func (p *Plugin) requireCompanyCode(ctx context.Context, cc string) error {
	if p.host == nil {
		return nil
	}
	_, err := p.host.Handle(ctx, sdk.Request{Object: "CompanyCode", Action: "get", Payload: map[string]any{"id": cc}})
	switch {
	case err == nil, errors.Is(err, sdk.ErrUnimplemented), errors.Is(err, sdk.ErrUnavailable):
		return nil
	case errors.Is(err, sdk.ErrNotFound):
		return fmt.Errorf("%w: Buchungskreis %s gibt es nicht", sdk.ErrInvalidArgument, cc)
	}
	return err
}

// --- Objekte ---------------------------------------------------------------------------

func (o objectRow) record() map[string]any {
	return map[string]any{"id": o.Object, "object": o.Object, "owner": o.Owner, "description": o.Description,
		"per_company_code": o.PerCompanyCode, "per_year": o.PerYear, "pattern": o.Pattern, "width": o.Width,
		"from_number": o.From, "to_number": o.To, "overflow": o.Overflow, "warn_percent": o.WarnPercent, "updated_at": o.UpdatedAt}
}

func (p *Plugin) objectList(ctx context.Context) (sdk.Response, error) {
	res, err := database.Query(ctx, p.pool(), `SELECT object FROM numrange__object ORDER BY object`)
	if err != nil {
		return sdk.Response{}, err
	}
	items := []any{}
	for _, r := range res.Rows {
		o, err := p.loadObject(ctx, p.pool(), s(r[0]))
		if err != nil {
			return sdk.Response{}, err
		}
		items = append(items, o.record())
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) objectGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	o, err := p.loadObject(ctx, p.pool(), id)
	if err != nil {
		return sdk.Response{}, err
	}
	if o == nil {
		return sdk.Response{}, fmt.Errorf("%w: Nummernkreis-Objekt %q", sdk.ErrNotFound, id)
	}
	return sdk.Response{Payload: o.record()}, nil
}

// --- Protokoll -------------------------------------------------------------------------

const logCols = `id, interval_id, object, company_code, range_key, year, value, number, reference, user_id, request_id, drawn_at`

func logRecord(r []any) map[string]any {
	return map[string]any{"id": s(r[0]), "interval_id": s(r[1]), "object": s(r[2]), "company_code": s(r[3]), "range_key": s(r[4]),
		"year": i64(r[5]), "value": i64(r[6]), "number": s(r[7]), "reference": s(r[8]), "user_id": s(r[9]), "request_id": s(r[10]), "drawn_at": s(r[11])}
}

func (p *Plugin) logList(ctx context.Context, payload any) (sdk.Response, error) {
	q := query(payload)
	var conds []string
	var args []any
	for _, k := range []string{"object", "company_code", "range_key", "interval_id"} {
		if v := q[k]; v != "" {
			conds, args = append(conds, k+" = ?"), append(args, v)
		}
	}
	rest := ""
	if len(conds) > 0 {
		rest = "WHERE " + strings.Join(conds, " AND ")
	}
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT `+logCols+` FROM numrange__log `+rest+` ORDER BY drawn_at DESC, value DESC LIMIT `+strconv.Itoa(logLimit)), args...)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(res.Rows))
	for i, r := range res.Rows {
		items[i] = logRecord(r)
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (p *Plugin) logGet(ctx context.Context, payload any) (sdk.Response, error) {
	id, err := idParam(payload)
	if err != nil {
		return sdk.Response{}, err
	}
	res, err := database.Query(ctx, p.pool(), p.q(`SELECT `+logCols+` FROM numrange__log WHERE id = ?`), id)
	if err != nil {
		return sdk.Response{}, err
	}
	if len(res.Rows) == 0 {
		return sdk.Response{}, fmt.Errorf("%w: Eintrag %q", sdk.ErrNotFound, id)
	}
	return sdk.Response{Payload: logRecord(res.Rows[0])}, nil
}

func includeHistory(q map[string]string) bool {
	v := q["includeHistory"]
	return v == "true" || v == "1" || v == "on"
}

func truthy(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "on"
	}
	return false
}
