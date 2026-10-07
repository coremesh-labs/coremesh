package iam

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
)

// Berechtigungen einer Rolle: je Object.Action eine Zeile (iam__role_auth)
// mit den Buchungskreisen und – optional – erlaubten Werten der
// Berechtigungsfelder (iam__role_auth_value, je Wert eine Zeile):
//
//	Object        Action  Buchungskreise  Feldwerte
//	*             *       *                                    alles
//	Partner       *       1000                                 alle Actions auf Partner in 1000
//	FiscalPeriod  post    1000,2000       posting_period 1..12 nur normale Perioden
//	DocumentType  post    *               code KR, code KG     nur Kreditorenrechnungen/-gutschriften
//
// Object und Action dürfen Platzhalter enthalten (path.Match). Es gibt nur
// Erlaubnisse, keine Verbote: Zeilen ergänzen sich (ODER), die Felder einer
// Zeile müssen alle passen (UND), die Werte eines Felds ergänzen sich (ODER).
// Inaktive Zeilen und Werte zählen nicht.
//
// Textform (Profil, Rollen-API, Übersicht):
//
//	FiscalPeriod.post@1000,2000 posting_period=1..12 ledger=0L
type grant struct {
	ID           string
	RoleID       string
	Object       string
	Action       string
	CompanyCodes []string // ["*"] = alle Buchungskreise
	Active       bool
	Values       []authValue
}

// authValue: erlaubter Wert eines Berechtigungsfelds – Einzelwert, Muster
// (* ?) oder Bereich Low..High.
type authValue struct {
	ID     string
	AuthID string
	Field  string
	Low    string
	High   string
	Active bool
}

// AllCompanyCodes steht für „alle Buchungskreise“.
const AllCompanyCodes = "*"

func (g grant) matches(object, action string) bool {
	okO, _ := path.Match(g.Object, object)
	okA, _ := path.Match(g.Action, action)
	return okO && okA
}

// activeValues: nur aktive Werte schränken ein.
func (g grant) activeValues() []authValue {
	var out []authValue
	for _, v := range g.Values {
		if v.Active {
			out = append(out, v)
		}
	}
	return out
}

func (g grant) rule() sdk.GrantRule {
	r := sdk.GrantRule{CompanyCodes: slices.Clone(g.CompanyCodes)}
	for _, v := range g.activeValues() {
		if r.Fields == nil {
			r.Fields = map[string][]sdk.ValueRange{}
		}
		r.Fields[v.Field] = append(r.Fields[v.Field], sdk.ValueRange{Low: v.Low, High: v.High})
	}
	return r
}

func (g grant) allCompanies() bool { return slices.Contains(g.CompanyCodes, AllCompanyCodes) }

// restrictions: "posting_period=1..12 ledger=0L" (Felder in Reihenfolge des
// ersten Auftretens, Werte durch Komma getrennt).
func (g grant) restrictions() string {
	var order []string
	vals := map[string][]string{}
	for _, v := range g.activeValues() {
		if _, ok := vals[v.Field]; !ok {
			order = append(order, v.Field)
		}
		vals[v.Field] = append(vals[v.Field], sdk.ValueRange{Low: v.Low, High: v.High}.String())
	}
	parts := make([]string, len(order))
	for i, f := range order {
		parts[i] = f + "=" + strings.Join(vals[f], ",")
	}
	return strings.Join(parts, " ")
}

// String: Textform einer Zeile.
func (g grant) String() string {
	s := g.Object + "." + g.Action
	if !g.allCompanies() {
		ccs := slices.Clone(g.CompanyCodes)
		slices.Sort(ccs)
		s += "@" + strings.Join(ccs, ",")
	}
	if r := g.restrictions(); r != "" {
		s += " " + r
	}
	return s
}

var (
	permPartRe    = regexp.MustCompile(`^[A-Za-z0-9*?]+$`)
	companyCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)
	fieldRe       = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	valueRe       = regexp.MustCompile(`^[^\s,=]{1,64}$`)
	commaRe       = regexp.MustCompile(`\s*,\s*`)
)

// parseCompanyCodes: "1000, 2000" → [1000 2000]; leer oder * → [*].
func parseCompanyCodes(text string) ([]string, error) {
	var ccs []string
	for _, cc := range strings.Split(text, ",") {
		cc = strings.TrimSpace(cc)
		switch {
		case cc == "":
		case cc != AllCompanyCodes && !companyCodeRe.MatchString(cc):
			return nil, fmt.Errorf("ungültiger Buchungskreis %q", cc)
		case !slices.Contains(ccs, cc):
			ccs = append(ccs, cc)
		}
	}
	if len(ccs) == 0 || slices.Contains(ccs, AllCompanyCodes) {
		return []string{AllCompanyCodes}, nil // * schließt alle anderen ein
	}
	slices.Sort(ccs)
	return ccs, nil
}

// parseValue: "13..16" → {13 16}, "SA" → {SA}, "4*" → Muster.
func parseValue(field, text string) (authValue, error) {
	low, high, isRange := strings.Cut(strings.TrimSpace(text), "..")
	v := authValue{Field: field, Low: strings.TrimSpace(low), High: strings.TrimSpace(high), Active: true}
	return v, checkValue(v, isRange)
}

func checkValue(v authValue, isRange bool) error {
	switch {
	case !fieldRe.MatchString(v.Field):
		return fmt.Errorf("Feld %q: [a-z][a-z0-9_]* erwartet", v.Field)
	case v.Field == sdk.AttrCompanyCode:
		return fmt.Errorf("company_code ist kein Feldwert – Buchungskreise der Zeile verwenden")
	case !valueRe.MatchString(v.Low) || (isRange && !valueRe.MatchString(v.High)):
		return fmt.Errorf("Feld %s: Wert fehlt oder enthält Leerzeichen, Komma oder =", v.Field)
	case v.High != "" && strings.ContainsAny(v.Low+v.High, "*?"):
		return fmt.Errorf("Feld %s: Bereich ohne Platzhalter angeben", v.Field)
	case v.High != "" && !(sdk.ValueRange{Low: v.Low, High: v.High}).Matches(v.Low):
		return fmt.Errorf("Feld %s: Bereich %s..%s – von ist größer als bis", v.Field, v.Low, v.High)
	}
	if strings.ContainsAny(v.Low, "*?") {
		if _, err := path.Match(v.Low, ""); err != nil {
			return fmt.Errorf("Feld %s: Muster %q: %v", v.Field, v.Low, err)
		}
	}
	return nil
}

func checkObjectAction(object, action string) error {
	if !permPartRe.MatchString(object) || !permPartRe.MatchString(action) {
		return fmt.Errorf("Object und Action: Buchstaben, Ziffern, * und ?")
	}
	if _, err := path.Match(object, ""); err != nil {
		return err
	}
	return nil
}

// parsePermissions liest die Textform, eine Zeile je Object.Action:
//
//	Object.Action[@Buchungskreis,…] [feld=wert[,wert…] …]
//
// Leere Zeilen und # werden übersprungen. Doppelte Object.Action werden
// zusammengeführt, wenn sie keine Feldwerte haben. Ob die Buchungskreise
// existieren, prüft saveRole.
func parsePermissions(text string) ([]grant, error) {
	var out []grant
	var errs []error
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens := strings.Fields(commaRe.ReplaceAllString(line, ","))
		spec, codes, hasCodes := strings.Cut(tokens[0], "@")
		obj, act, ok := strings.Cut(spec, ".")
		if !ok {
			errs = append(errs, fmt.Errorf("%q: Format Object.Action[@Buchungskreis,…] [feld=wert,…] erwartet", line))
			continue
		}
		if err := checkObjectAction(obj, act); err != nil {
			errs = append(errs, fmt.Errorf("%q: %v", line, err))
			continue
		}
		g := grant{Object: obj, Action: act, CompanyCodes: []string{AllCompanyCodes}, Active: true}
		if hasCodes {
			ccs, err := parseCompanyCodes(codes)
			if err == nil && codes == "" {
				err = errors.New("Buchungskreis fehlt nach @")
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%q: %v", line, err))
				continue
			}
			g.CompanyCodes = ccs
		}
		bad := false
		for _, tok := range tokens[1:] {
			field, list, ok := strings.Cut(tok, "=")
			if !ok || list == "" {
				errs = append(errs, fmt.Errorf("%q: %q – feld=wert erwartet", line, tok))
				bad = true
				break
			}
			for _, text := range strings.Split(list, ",") {
				v, err := parseValue(field, text)
				if err != nil {
					errs = append(errs, fmt.Errorf("%q: %v", line, err))
					bad = true
					break
				}
				g.Values = append(g.Values, v)
			}
		}
		if bad {
			continue
		}
		i := slices.IndexFunc(out, func(o grant) bool {
			return o.Object == g.Object && o.Action == g.Action && len(o.Values) == 0 && len(g.Values) == 0
		})
		if i < 0 {
			out = append(out, g)
			continue
		}
		merged, _ := parseCompanyCodes(strings.Join(append(out[i].CompanyCodes, g.CompanyCodes...), ","))
		out[i].CompanyCodes = merged
	}
	return out, errors.Join(errs...)
}

// formatPermissions: Textform der aktiven Zeilen.
func formatPermissions(gs []grant) []string {
	out := []string{}
	for _, g := range gs {
		if g.Active {
			out = append(out, g.String())
		}
	}
	return out
}

// --- Prüfungen ------------------------------------------------------------------

// Allowed (dispatcher.Authorizer) prüft, ob der Benutzer object.action
// überhaupt aufrufen darf – in irgendeinem Buchungskreis, mit irgendwelchen
// Werten. Die feine Prüfung macht das Fachmodul mit sdk.Authorize bzw.
// sdk.Grants.
func (p *Plugin) Allowed(ctx context.Context, userID, object, action string) (bool, error) {
	gs, err := p.cachedGrants(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(gs, func(g grant) bool { return g.matches(object, action) }), nil
}

// Check prüft object.action mit den Werten attrs (Buchungskreis unter
// "company_code").
func (p *Plugin) Check(ctx context.Context, userID, object, action string, attrs sdk.Attrs) (bool, error) {
	g, err := p.Granted(ctx, userID, object, action)
	if err != nil {
		return false, err
	}
	return g.Allows(attrs), nil
}

// Granted liefert alle Erlaubnisse des Benutzers für object.action.
func (p *Plugin) Granted(ctx context.Context, userID, object, action string) (sdk.GrantSet, error) {
	gs, err := p.cachedGrants(ctx, userID)
	if err != nil {
		return sdk.GrantSet{}, err
	}
	return grantSet(gs, object, action), nil
}

func grantSet(gs []grant, object, action string) sdk.GrantSet {
	out := sdk.GrantSet{CompanyCodeGrant: sdk.CompanyCodeGrant{CompanyCodes: []string{}}, Rules: []sdk.GrantRule{}}
	for _, g := range gs {
		if !g.matches(object, action) {
			continue
		}
		out.Rules = append(out.Rules, g.rule())
		if len(g.activeValues()) > 0 || out.All {
			continue
		}
		if g.allCompanies() {
			out.All, out.CompanyCodes = true, []string{}
			continue
		}
		for _, cc := range g.CompanyCodes {
			if !slices.Contains(out.CompanyCodes, cc) {
				out.CompanyCodes = append(out.CompanyCodes, cc)
			}
		}
	}
	slices.Sort(out.CompanyCodes)
	return out
}

// systemGrants: Anfragen ohne Benutzer dürfen alles.
var systemGrants = sdk.GrantSet{
	CompanyCodeGrant: sdk.CompanyCodeGrant{All: true, CompanyCodes: []string{}},
	Rules:            []sdk.GrantRule{{CompanyCodes: []string{AllCompanyCodes}}},
}

// --- Cache ----------------------------------------------------------------------

const cacheTTL = 30 * time.Second

type permCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry // userID → Erlaubnisse
}

type cacheEntry struct {
	grants []grant
	at     time.Time
}

// cachedGrants: 30 s Cache; jede Änderung an Benutzern, Rollen oder
// Buchungskreisen leert ihn sofort. Inaktive Benutzer haben keine Rechte.
func (p *Plugin) cachedGrants(ctx context.Context, userID string) ([]grant, error) {
	p.cache.mu.Lock()
	e, ok := p.cache.entries[userID]
	p.cache.mu.Unlock()
	if ok && time.Since(e.at) < cacheTTL {
		return e.grants, nil
	}
	gs, err := p.userGrants(ctx, p.pool(), userID)
	if err != nil {
		return nil, err
	}
	p.cache.mu.Lock()
	p.cache.entries[userID] = cacheEntry{grants: gs, at: time.Now()}
	p.cache.mu.Unlock()
	return gs, nil
}

func (p *Plugin) invalidate() {
	p.cache.mu.Lock()
	p.cache.entries = map[string]cacheEntry{}
	p.cache.mu.Unlock()
}
