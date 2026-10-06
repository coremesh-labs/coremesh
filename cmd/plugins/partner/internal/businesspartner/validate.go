package businesspartner

import (
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{sdk.ErrInvalidArgument}, args...)...)
}

// --- Zeitscheiben -----------------------------------------------------------------

func today() string { return time.Now().Format(time.DateOnly) }

// parseDate akzeptiert JJJJ-MM-TT, auch mit Zeitanteil (RFC 3339) oder als
// time.Time – so liefern SQL-Treiber DATE-Spalten.
func parseDate(v any) (string, error) {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.DateOnly), nil
	}
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s = s[:10]
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", invalid("Datum im Format JJJJ-MM-TT erwartet: %q", v)
	}
	return s, nil
}

// checkTimeSlice setzt Standardwerte (valid_from = heute, valid_to = 9999-12-31)
// und prüft valid_from <= valid_to.
func checkTimeSlice(rec record) error {
	if rec["valid_from"] == nil {
		rec["valid_from"] = today()
	}
	if rec["valid_to"] == nil {
		rec["valid_to"] = dateMax
	}
	from, err := parseDate(rec["valid_from"])
	if err != nil {
		return err
	}
	to, err := parseDate(rec["valid_to"])
	if err != nil {
		return err
	}
	if from > to { // JJJJ-MM-TT ist lexikografisch sortierbar
		return invalid("Zeitscheibe: gültig ab (%s) liegt nach gültig bis (%s)", from, to)
	}
	rec["valid_from"], rec["valid_to"] = from, to
	return nil
}

// --- Kommunikation (nach Kategorie) -------------------------------------------------

var (
	phoneRe  = regexp.MustCompile(`^\+?[0-9][0-9 ()/.\-]{2,29}$`)
	digitsRe = regexp.MustCompile(`[0-9]`)
)

// validateContact prüft value anhand der maschinenlesbaren Kategorie des
// Kommunikationstyps. Unbekannte Kategorien werden nicht geprüft.
func validateContact(category, value string) error {
	switch category {
	case "EMAIL":
		if a, err := mail.ParseAddress(value); err != nil || a.Address != value {
			return invalid("%q ist keine gültige E-Mail-Adresse", value)
		}
	case "PHONE", "FAX":
		if !phoneRe.MatchString(value) || len(digitsRe.FindAllString(value, -1)) < 3 {
			return invalid("%q ist keine gültige Telefonnummer (erlaubt: Ziffern, Leerzeichen, + ( ) / . -)", value)
		}
	case "WEB":
		if u, err := url.Parse(value); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return invalid("%q ist keine gültige URL (http:// oder https://)", value)
		}
	}
	return nil
}

// --- Bank und Adresse -------------------------------------------------------------

var (
	ibanRe    = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`)
	bicRe     = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// normalizeIBAN entfernt Leerzeichen, schreibt groß und prüft die Prüfziffer (ISO 13616, mod 97).
func normalizeIBAN(s string) (string, error) {
	iban := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	if !ibanRe.MatchString(iban) {
		return "", invalid("IBAN %q hat ein ungültiges Format", s)
	}
	var num strings.Builder
	for _, r := range iban[4:] + iban[:4] {
		if r >= 'A' && r <= 'Z' {
			fmt.Fprintf(&num, "%d", r-'A'+10)
		} else {
			num.WriteRune(r)
		}
	}
	n, _ := new(big.Int).SetString(num.String(), 10)
	if new(big.Int).Mod(n, big.NewInt(97)).Int64() != 1 {
		return "", invalid("IBAN %q: Prüfziffer stimmt nicht", s)
	}
	return iban, nil
}

func normalizeBIC(s string) (string, error) {
	bic := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	if !bicRe.MatchString(bic) {
		return "", invalid("BIC %q hat ein ungültiges Format (8 oder 11 Zeichen)", s)
	}
	return bic, nil
}

func normalizeCountry(s string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(s))
	if !countryRe.MatchString(c) {
		return "", invalid("Land %q: zweistelliger ISO-Code erwartet (z. B. CH, DE)", s)
	}
	return c, nil
}
