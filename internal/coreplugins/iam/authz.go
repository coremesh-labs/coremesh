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
)

// Berechtigung: "Object.Action[@Buchungskreis,…]" mit Platzhaltern in Object
// und Action (path.Match):
//
//	*.*                  alles, in allen Buchungskreisen
//	Partner.*            alle Actions auf Partner, alle Buchungskreise
//	Partner.*@1000       … nur im Buchungskreis 1000
//	*.list@1000,2000     list auf allen Objects in 1000 und 2000
//	Greeting.l*          Greeting.list, Greeting.load, …
//
// Groß-/Kleinschreibung zählt. Es gibt nur Erlaubnisse, keine Verbote.
// Buchungskreise sind feste IDs aus iam__company_codes; "*" (oder kein @)
// bedeutet alle.
type permission struct {
	Object      string
	Action      string
	CompanyCode string // "*" = alle Buchungskreise
}

// AllCompanyCodes steht für „alle Buchungskreise“.
const AllCompanyCodes = "*"

func (p permission) matches(object, action string) bool {
	okO, _ := path.Match(p.Object, object)
	okA, _ := path.Match(p.Action, action)
	return okO && okA
}

func (p permission) allowsCompany(cc string) bool {
	return p.CompanyCode == AllCompanyCodes || p.CompanyCode == cc
}

var (
	permPartRe    = regexp.MustCompile(`^[A-Za-z0-9*?]+$`)
	companyCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)
)

// parsePermissions liest eine Berechtigung pro Zeile; jede Zeile wird zu einer
// Zeile je Buchungskreis. Leere Zeilen und # werden übersprungen, Doppelte
// entfernt. Ob die Buchungskreise existieren, prüft saveRole.
func parsePermissions(text string) ([]permission, error) {
	var out []permission
	var errs []error
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spec, codes, hasCodes := strings.Cut(line, "@")
		obj, act, ok := strings.Cut(strings.TrimSpace(spec), ".")
		if !ok || !permPartRe.MatchString(obj) || !permPartRe.MatchString(act) {
			errs = append(errs, fmt.Errorf("%q: Format Object.Action[@Buchungskreis,…] erwartet", line))
			continue
		}
		if _, err := path.Match(obj, ""); err != nil {
			errs = append(errs, fmt.Errorf("%q: %v", line, err))
			continue
		}
		ccs := []string{AllCompanyCodes}
		if hasCodes {
			ccs = nil
			for _, cc := range strings.Split(codes, ",") {
				cc = strings.TrimSpace(cc)
				if cc != AllCompanyCodes && !companyCodeRe.MatchString(cc) {
					errs = append(errs, fmt.Errorf("%q: ungültiger Buchungskreis %q", line, cc))
					continue
				}
				ccs = append(ccs, cc)
			}
			if len(ccs) == 0 {
				errs = append(errs, fmt.Errorf("%q: Buchungskreis fehlt nach @", line))
				continue
			}
			if slices.Contains(ccs, AllCompanyCodes) {
				ccs = []string{AllCompanyCodes} // * schließt alle anderen ein
			}
		}
		for _, cc := range ccs {
			p := permission{obj, act, cc}
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out, errors.Join(errs...)
}

// formatPermissions fasst Zeilen je Object.Action zusammen:
// "Partner.*@1000,2000", ohne @ bei allen Buchungskreisen.
func formatPermissions(ps []permission) []string {
	type key struct{ o, a string }
	var order []key
	codes := map[key][]string{}
	for _, p := range ps {
		k := key{p.Object, p.Action}
		if _, ok := codes[k]; !ok {
			order = append(order, k)
		}
		codes[k] = append(codes[k], p.CompanyCode)
	}
	out := make([]string, len(order))
	for i, k := range order {
		s := k.o + "." + k.a
		if ccs := codes[k]; !slices.Contains(ccs, AllCompanyCodes) {
			slices.Sort(ccs)
			s += "@" + strings.Join(ccs, ",")
		}
		out[i] = s
	}
	return out
}

// --- Prüfungen ------------------------------------------------------------------

// Allowed (dispatcher.Authorizer) prüft, ob der Benutzer object.action
// überhaupt aufrufen darf – in irgendeinem Buchungskreis. Der Dispatcher
// kennt den Buchungskreis eines Datensatzes nicht; die feine Prüfung macht
// das Fachmodul mit Check bzw. sdk.CheckAccess.
func (p *Plugin) Allowed(ctx context.Context, userID, object, action string) (bool, error) {
	perms, err := p.cachedPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(perms, func(pm permission) bool { return pm.matches(object, action) }), nil
}

// Check prüft object.action im Buchungskreis companyCode.
func (p *Plugin) Check(ctx context.Context, userID, object, action, companyCode string) (bool, error) {
	g, err := p.Granted(ctx, userID, object, action)
	if err != nil {
		return false, err
	}
	return g.All || slices.Contains(g.CompanyCodes, companyCode), nil
}

// Grant sind die Buchungskreise, in denen eine Berechtigung gewährt ist.
type Grant struct {
	All          bool     `json:"all"`           // alle Buchungskreise
	CompanyCodes []string `json:"company_codes"` // sonst diese
}

// Granted ermittelt, in welchen Buchungskreisen der Benutzer object.action darf.
func (p *Plugin) Granted(ctx context.Context, userID, object, action string) (Grant, error) {
	perms, err := p.cachedPermissions(ctx, userID)
	if err != nil {
		return Grant{}, err
	}
	g := Grant{CompanyCodes: []string{}}
	for _, pm := range perms {
		if !pm.matches(object, action) {
			continue
		}
		if pm.CompanyCode == AllCompanyCodes {
			return Grant{All: true, CompanyCodes: []string{}}, nil
		}
		if !slices.Contains(g.CompanyCodes, pm.CompanyCode) {
			g.CompanyCodes = append(g.CompanyCodes, pm.CompanyCode)
		}
	}
	slices.Sort(g.CompanyCodes)
	return g, nil
}

// --- Cache ----------------------------------------------------------------------

const cacheTTL = 30 * time.Second

type permCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry // userID → Berechtigungen
}

type cacheEntry struct {
	perms []permission
	at    time.Time
}

// cachedPermissions: 30 s Cache; jede Änderung an Benutzern, Rollen oder
// Buchungskreisen leert ihn sofort. Inaktive Benutzer haben keine Rechte.
func (p *Plugin) cachedPermissions(ctx context.Context, userID string) ([]permission, error) {
	p.cache.mu.Lock()
	e, ok := p.cache.entries[userID]
	p.cache.mu.Unlock()
	if ok && time.Since(e.at) < cacheTTL {
		return e.perms, nil
	}
	perms, err := p.userPermissions(ctx, p.pool(), userID)
	if err != nil {
		return nil, err
	}
	p.cache.mu.Lock()
	p.cache.entries[userID] = cacheEntry{perms: perms, at: time.Now()}
	p.cache.mu.Unlock()
	return perms, nil
}

func (p *Plugin) invalidate() {
	p.cache.mu.Lock()
	p.cache.entries = map[string]cacheEntry{}
	p.cache.mu.Unlock()
}
