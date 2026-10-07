package sdk

import (
	"context"
	"fmt"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Rechteprüfung bis auf Feldwerte (Berechtigungsobjekte).
//
// Der Dispatcher prüft beim Einstieg nur, ob der Benutzer object.action
// überhaupt aufrufen darf – in irgendeinem Buchungskreis, mit irgendwelchen
// Werten. Zu welchem Buchungskreis und welchen Werten ein Vorgang gehört, weiß
// nur das Fachmodul; es prüft das mit den Funktionen hier. Der Benutzer kommt
// aus ctx (CallContext): Der Dispatcher setzt ihn aus der ursprünglichen
// Anfrage, ein Plugin kann ihn nicht fälschen. Anfragen ohne Benutzer (System)
// dürfen alles.
//
// Eine Rolle erlaubt Object.Action (Platzhalter * möglich) in Buchungskreisen
// und optional nur für bestimmte Werte der Berechtigungsfelder, die das
// Object im Metamodell deklariert (metamodel.Authorization). Es gibt nur
// Erlaubnisse, keine Verbote: Mehrere Regeln ergänzen sich (ODER), die Felder
// einer Regel müssen alle passen (UND), die Werte eines Felds ergänzen sich.
//
//	ok, err := sdk.Authorize(ctx, "FiscalPeriod", "post", sdk.Attrs{
//		"company_code": "1000", "ledger": "0L", "posting_period": "13"})
//	if err != nil { return sdk.Response{}, err }
//	if !ok { return sdk.Response{}, fmt.Errorf("%w: Periode 13", sdk.ErrPermissionDenied) }
//
// Für viele Prüfungen (Positionen, Listen, Auswahlwerte) einmal die Regeln
// holen und lokal auswerten:
//
//	g, err := sdk.Grants(ctx, "DocumentType", "post")
//	for _, dt := range types { if g.Allows(sdk.Attrs{"code": dt}) { … } }
//	where, args := g.SQL(map[string]string{"company_code": "company_code_id", "code": "code"})
//
// Fehlt ein Wert, den eine Regel einschränkt, passt die Regel nicht
// (fail closed) – der Buchungskreis ist dabei der Schlüssel "company_code".

// Attrs sind die Werte eines Vorgangs je Berechtigungsfeld; "company_code"
// ist der Buchungskreis.
type Attrs map[string]string

// AttrCompanyCode ist der Schlüssel des Buchungskreises in Attrs.
const AttrCompanyCode = "company_code"

// ValueRange ist ein erlaubter Wert eines Berechtigungsfelds: Einzelwert
// (High leer), Muster mit * und ? (path.Match) oder Bereich Low..High
// (numerisch, wenn beide Grenzen und der Wert ganze Zahlen sind, sonst nach
// Zeichenfolge).
type ValueRange struct {
	Low  string `json:"low"`
	High string `json:"high,omitempty"`
}

// Matches meldet, ob v im Wert liegt.
func (r ValueRange) Matches(v string) bool {
	if r.High == "" {
		if strings.ContainsAny(r.Low, "*?") {
			ok, _ := path.Match(r.Low, v)
			return ok
		}
		return r.Low == v
	}
	lo, e1 := strconv.ParseInt(r.Low, 10, 64)
	hi, e2 := strconv.ParseInt(r.High, 10, 64)
	n, e3 := strconv.ParseInt(v, 10, 64)
	if e1 == nil && e2 == nil && e3 == nil {
		return lo <= n && n <= hi
	}
	return r.Low <= v && v <= r.High
}

// String: "13..16", "4*", "SA".
func (r ValueRange) String() string {
	if r.High == "" {
		return r.Low
	}
	return r.Low + ".." + r.High
}

// GrantRule ist eine Erlaubnis: Buchungskreise (["*"] = alle) und je
// eingeschränktem Feld die erlaubten Werte. Nicht aufgeführte Felder sind
// frei.
type GrantRule struct {
	CompanyCodes []string                `json:"company_codes"`
	Fields       map[string][]ValueRange `json:"fields,omitempty"`
}

// AllCompanies meldet, ob die Regel in allen Buchungskreisen gilt.
func (r GrantRule) AllCompanies() bool { return slices.Contains(r.CompanyCodes, "*") }

// Allows prüft die Werte eines Vorgangs gegen die Regel.
func (r GrantRule) Allows(attrs Attrs) bool {
	if !r.AllCompanies() {
		cc, ok := attrs[AttrCompanyCode]
		if !ok || !slices.Contains(r.CompanyCodes, cc) {
			return false
		}
	}
	for field, ranges := range r.Fields {
		v, ok := attrs[field]
		if !ok || !slices.ContainsFunc(ranges, func(vr ValueRange) bool { return vr.Matches(v) }) {
			return false
		}
	}
	return true
}

// GrantSet sind alle Erlaubnisse des Benutzers für ein Object.Action.
// All/CompanyCodes fassen die Regeln ohne Feldeinschränkung zusammen
// (Buchungskreise, in denen object.action uneingeschränkt gilt).
type GrantSet struct {
	CompanyCodeGrant
	Rules []GrantRule `json:"rules"`
}

// Allows: Passt irgendeine Regel?
func (g GrantSet) Allows(attrs Attrs) bool {
	return slices.ContainsFunc(g.Rules, func(r GrantRule) bool { return r.Allows(attrs) })
}

// Empty: Der Benutzer darf object.action nirgends.
func (g GrantSet) Empty() bool { return len(g.Rules) == 0 }

// SQL übersetzt die Regeln in eine WHERE-Bedingung mit Platzhaltern ?.
// columns ordnet Berechtigungsfeldern (und "company_code") die Spalten zu;
// schränkt eine Regel ein Feld ohne Spalte ein, entfällt die Regel (fail
// closed). Ohne Regel: "1=0"; uneingeschränkt: "1=1".
func (g GrantSet) SQL(columns map[string]string) (string, []any) {
	var ors []string
	var args []any
	for _, r := range g.Rules {
		var ands []string
		var rargs []any
		ok := true
		if !r.AllCompanies() {
			col, has := columns[AttrCompanyCode]
			if !has {
				continue
			}
			ands = append(ands, col+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(r.CompanyCodes)), ",")+")")
			for _, cc := range r.CompanyCodes {
				rargs = append(rargs, cc)
			}
		}
		for _, field := range sortedFields(r.Fields) {
			col, has := columns[field]
			if !has {
				ok = false
				break
			}
			var vs []string
			for _, vr := range r.Fields[field] {
				cond, a := vr.sql(col)
				vs = append(vs, cond)
				rargs = append(rargs, a...)
			}
			ands = append(ands, "("+strings.Join(vs, " OR ")+")")
		}
		if !ok {
			continue
		}
		if len(ands) == 0 {
			return "1=1", nil
		}
		ors = append(ors, "("+strings.Join(ands, " AND ")+")")
		args = append(args, rargs...)
	}
	if len(ors) == 0 {
		return "1=0", nil
	}
	return "(" + strings.Join(ors, " OR ") + ")", args
}

func (r ValueRange) sql(col string) (string, []any) {
	if r.High == "" {
		if r.Low == "*" {
			return "1=1", nil
		}
		if strings.ContainsAny(r.Low, "*?") {
			return col + " LIKE ?", []any{strings.NewReplacer("*", "%", "?", "_").Replace(r.Low)}
		}
		return col + " = ?", []any{r.Low}
	}
	lo, e1 := strconv.ParseInt(r.Low, 10, 64)
	hi, e2 := strconv.ParseInt(r.High, 10, 64)
	if e1 == nil && e2 == nil {
		return col + " BETWEEN ? AND ?", []any{lo, hi}
	}
	return col + " BETWEEN ? AND ?", []any{r.Low, r.High}
}

func sortedFields(m map[string][]ValueRange) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Authorize prüft, ob der aufrufende Benutzer object.action mit den Werten
// attrs ausführen darf (Account.Check im Core-Plugin iam).
func Authorize(ctx context.Context, object, action string, attrs Attrs) (bool, error) {
	if attrs == nil {
		attrs = Attrs{}
	}
	resp, err := HostFrom(ctx).Handle(ctx, Request{Object: "Account", Action: "Check",
		Payload: map[string]any{"object": object, "action": action, "attrs": map[string]string(attrs)}})
	if err != nil {
		return false, fmt.Errorf("Authorize: %w", err)
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := Decode(resp.Payload, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

// Grants liefert alle Erlaubnisse des aufrufenden Benutzers für
// object.action (Account.Granted im Core-Plugin iam) – zum lokalen Auswerten
// mit Allows oder SQL.
func Grants(ctx context.Context, object, action string) (GrantSet, error) {
	resp, err := HostFrom(ctx).Handle(ctx, Request{Object: "Account", Action: "Granted",
		Payload: map[string]any{"object": object, "action": action}})
	if err != nil {
		return GrantSet{}, fmt.Errorf("Grants: %w", err)
	}
	var g GrantSet
	if err := Decode(resp.Payload, &g); err != nil {
		return GrantSet{}, err
	}
	return g, nil
}

// --- Buchungskreis allein (bisherige API) ---------------------------------------

// CompanyCodeGrant sind die Buchungskreise, in denen eine Berechtigung gilt.
type CompanyCodeGrant struct {
	All          bool     `json:"all"`           // alle Buchungskreise
	CompanyCodes []string `json:"company_codes"` // sonst genau diese
}

// Allows meldet, ob der Buchungskreis companyCode enthalten ist.
func (g CompanyCodeGrant) Allows(companyCode string) bool {
	return g.All || slices.Contains(g.CompanyCodes, companyCode)
}

// None meldet, dass die Berechtigung in keinem Buchungskreis gilt.
func (g CompanyCodeGrant) None() bool { return !g.All && len(g.CompanyCodes) == 0 }

// CheckAccess prüft object.action im Buchungskreis companyCode – wie
// Authorize mit Attrs{"company_code": companyCode}. Regeln mit
// Feldeinschränkungen passen dabei nicht.
func CheckAccess(ctx context.Context, object, action, companyCode string) (bool, error) {
	return Authorize(ctx, object, action, Attrs{AttrCompanyCode: companyCode})
}

// GrantedCompanyCodes ermittelt, in welchen Buchungskreisen der aufrufende
// Benutzer object.action ohne Feldeinschränkung ausführen darf – z. B. um eine
// Liste in SQL vorzufiltern.
func GrantedCompanyCodes(ctx context.Context, object, action string) (CompanyCodeGrant, error) {
	g, err := Grants(ctx, object, action)
	if err != nil {
		return CompanyCodeGrant{}, fmt.Errorf("GrantedCompanyCodes: %w", err)
	}
	if g.CompanyCodes == nil {
		g.CompanyCodes = []string{}
	}
	return g.CompanyCodeGrant, nil
}
