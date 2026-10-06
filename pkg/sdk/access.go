package sdk

import (
	"context"
	"fmt"
	"slices"
)

// Rechteprüfung je Buchungskreis.
//
// Der Dispatcher prüft beim Einstieg nur, ob der Benutzer object.action
// überhaupt aufrufen darf – in irgendeinem Buchungskreis. Zu welchem
// Buchungskreis ein Datensatz gehört, weiß nur das Fachmodul; es prüft das mit
// den Funktionen hier. Der Benutzer kommt aus ctx (CallContext): Der
// Dispatcher setzt ihn aus der ursprünglichen Anfrage, ein Plugin kann ihn
// nicht fälschen. Anfragen ohne Benutzer (System) dürfen alles.
//
//	ok, err := sdk.CheckAccess(ctx, "Partner", "update", partner.CompanyCode)
//	if err != nil { return sdk.Response{}, err }
//	if !ok { return sdk.Response{}, fmt.Errorf("%w: Buchungskreis %s", sdk.ErrPermissionDenied, partner.CompanyCode) }
//
//	g, err := sdk.GrantedCompanyCodes(ctx, "Partner", "list")
//	// SQL: … WHERE company_code IN (g.CompanyCodes) – oder ohne Filter, wenn g.All

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

// CheckAccess prüft, ob der aufrufende Benutzer object.action im Buchungskreis
// companyCode ausführen darf (Account.Check im Core-Plugin iam).
func CheckAccess(ctx context.Context, object, action, companyCode string) (bool, error) {
	resp, err := HostFrom(ctx).Handle(ctx, Request{Object: "Account", Action: "Check",
		Payload: map[string]any{"object": object, "action": action, "company_code": companyCode}})
	if err != nil {
		return false, fmt.Errorf("CheckAccess: %w", err)
	}
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := Decode(resp.Payload, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

// GrantedCompanyCodes ermittelt, in welchen Buchungskreisen der aufrufende
// Benutzer object.action ausführen darf (Account.Granted im Core-Plugin iam) –
// z. B. um eine Liste in SQL vorzufiltern.
func GrantedCompanyCodes(ctx context.Context, object, action string) (CompanyCodeGrant, error) {
	resp, err := HostFrom(ctx).Handle(ctx, Request{Object: "Account", Action: "Granted",
		Payload: map[string]any{"object": object, "action": action}})
	if err != nil {
		return CompanyCodeGrant{}, fmt.Errorf("GrantedCompanyCodes: %w", err)
	}
	var g CompanyCodeGrant
	if err := Decode(resp.Payload, &g); err != nil {
		return CompanyCodeGrant{}, err
	}
	return g, nil
}
