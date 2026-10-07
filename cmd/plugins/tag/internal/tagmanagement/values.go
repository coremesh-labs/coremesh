package tagmanagement

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/tagservice"
)

// Werte: Prüfung und Umwandlung je Datentyp.
//
//	STRING     value_string     Text, höchstens 1000 Zeichen
//	INTEGER    value_integer    64-Bit-Ganzzahl
//	CURRENCY   value_amount     Dezimaltext (bis 15 Vor-, 4 Nachkommastellen) + value_currency (ISO 4217)
//	DATE       value_date       JJJJ-MM-TT
//	TIMESTAMP  value_timestamp  RFC 3339, gespeichert in UTC
//	REFERENCE  value_ref        fachlicher Schlüssel (id) eines Datensatzes von ref_object
//
// Bei Tags mit Auswahlwerten ist der Code des Auswahlwerts der Wert: Er steht
// in option_code und – nach Datentyp umgewandelt – in der Wertspalte.

var amountRe = regexp.MustCompile(`^-?\d{1,15}(\.\d{1,4})?$`)

// stored ist ein Wert in Spaltenform.
type stored struct {
	String, Amount, Currency, Date, Timestamp, Option, Ref *string
	Integer                                                *int64
}

func (s stored) value() tagservice.Value {
	return tagservice.Value{String: s.String, Integer: s.Integer, Amount: s.Amount, Currency: s.Currency,
		Date: s.Date, Timestamp: s.Timestamp, Option: s.Option, Ref: s.Ref}
}

// parseValue prüft einen Wert gegen den Tag-Typ. codeOnly: nur den Code eines
// Auswahlwerts nach Datentyp prüfen (ohne Existenz am Stichtag).
func parseValue(t *tagservice.TagType, v *tagservice.Value, codeOnly bool) (stored, error) {
	var out stored
	if v == nil {
		return out, fmt.Errorf("kein Wert")
	}
	if t.ValueMode == tagservice.ModeOptions {
		if v.Option == nil || strings.TrimSpace(*v.Option) == "" {
			return out, fmt.Errorf("Auswahlwert (option) erwartet")
		}
		code := strings.TrimSpace(*v.Option)
		typed, err := parseScalar(t.DataType, code)
		if err != nil {
			return out, err
		}
		typed.Option = &code
		return typed, nil
	}
	if v.Option != nil {
		return out, fmt.Errorf("Tag %s hat freie Werte – option nicht erlaubt", t.Code)
	}
	set := 0
	for _, p := range []bool{v.String != nil, v.Integer != nil, v.Amount != nil || v.Currency != nil, v.Date != nil, v.Timestamp != nil, v.Ref != nil} {
		if p {
			set++
		}
	}
	if set != 1 {
		return out, fmt.Errorf("genau ein Wert passend zum Datentyp %s erwartet", t.DataType)
	}
	switch t.DataType {
	case tagservice.TypeString:
		if v.String == nil {
			return out, fmt.Errorf("Text (string) erwartet")
		}
		return parseScalar(t.DataType, *v.String)
	case tagservice.TypeInteger:
		if v.Integer == nil {
			return out, fmt.Errorf("Ganzzahl (integer) erwartet")
		}
		return stored{Integer: v.Integer}, nil
	case tagservice.TypeCurrency:
		if v.Amount == nil || v.Currency == nil {
			return out, fmt.Errorf("Betrag (amount) und Währung (currency) erwartet")
		}
		amount := strings.TrimSpace(*v.Amount)
		if !amountRe.MatchString(amount) {
			return out, fmt.Errorf("Betrag %q: Dezimalzahl mit Punkt, höchstens 15 Vor- und 4 Nachkommastellen", amount)
		}
		cur := strings.ToUpper(strings.TrimSpace(*v.Currency))
		if !iso4217[cur] {
			return out, fmt.Errorf("Währung %q ist kein ISO-4217-Code", cur)
		}
		return stored{Amount: &amount, Currency: &cur}, nil
	case tagservice.TypeDate:
		if v.Date == nil {
			return out, fmt.Errorf("Datum (date) erwartet")
		}
		return parseScalar(t.DataType, *v.Date)
	case tagservice.TypeTimestamp:
		if v.Timestamp == nil {
			return out, fmt.Errorf("Zeitpunkt (timestamp) erwartet")
		}
		return parseScalar(t.DataType, *v.Timestamp)
	case tagservice.TypeReference:
		if v.Ref == nil {
			return out, fmt.Errorf("Verweis (ref) auf %s erwartet", t.RefObject)
		}
		return parseScalar(t.DataType, *v.Ref)
	}
	return out, fmt.Errorf("unbekannter Datentyp %s", t.DataType)
}

// parseScalar wandelt Text in den Datentyp (CURRENCY nicht als einzelner Text).
func parseScalar(dt tagservice.DataType, s string) (stored, error) {
	s = strings.TrimSpace(s)
	switch dt {
	case tagservice.TypeString:
		if s == "" {
			return stored{}, fmt.Errorf("Text darf nicht leer sein")
		}
		if len([]rune(s)) > 1000 {
			return stored{}, fmt.Errorf("Text ist länger als 1000 Zeichen")
		}
		return stored{String: &s}, nil
	case tagservice.TypeInteger:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return stored{}, fmt.Errorf("%q ist keine Ganzzahl", s)
		}
		return stored{Integer: &i}, nil
	case tagservice.TypeDate:
		d, err := crud.ParseDate(s)
		if err != nil || len(s) != 10 {
			return stored{}, fmt.Errorf("Datum %q im Format JJJJ-MM-TT erwartet", s)
		}
		return stored{Date: &d}, nil
	case tagservice.TypeTimestamp:
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return stored{}, fmt.Errorf("Zeitpunkt %q im Format RFC 3339 erwartet, z. B. 2026-10-06T14:30:00+02:00", s)
		}
		u := ts.UTC().Format(time.RFC3339)
		return stored{Timestamp: &u}, nil
	case tagservice.TypeReference:
		if s == "" {
			return stored{}, fmt.Errorf("Verweis darf nicht leer sein")
		}
		if len(s) > 200 {
			return stored{}, fmt.Errorf("Verweis ist länger als 200 Zeichen")
		}
		return stored{Ref: &s}, nil
	case tagservice.TypeCurrency:
		return stored{}, fmt.Errorf("Auswahlwerte gibt es für CURRENCY nicht")
	}
	return stored{}, fmt.Errorf("unbekannter Datentyp %s", dt)
}

// loadType liest einen Tag-Typ.
func (m *Module) loadType(ctx context.Context, code string) (*tagservice.TagType, error) {
	res, err := m.db.Query(ctx, `SELECT code, name, translation_key, data_type, value_mode, status, ref_object FROM tag__tag_types WHERE code = ?`, code)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		return nil, fmt.Errorf("%w: Tag %q gibt es nicht", sdk.ErrInvalidArgument, code)
	}
	r := res.Rows[0]
	return &tagservice.TagType{Code: crud.Str(r[0]), Name: crud.Str(r[1]), TranslationKey: crud.Str(r[2]),
		DataType: tagservice.DataType(crud.Str(r[3])), ValueMode: tagservice.ValueMode(crud.Str(r[4])), Status: crud.Str(r[5]), RefObject: crud.Str(r[6])}, nil
}

// checkCompanyCode: "*" oder ein Buchungskreis aus iam (über den Dispatcher).
func (m *Module) checkCompanyCode(ctx context.Context, cc string) error {
	if cc == tagservice.AllCompanyCodes {
		return nil
	}
	if _, err := m.services.Call(ctx, "CompanyCode", "get", map[string]any{"id": cc}); err != nil {
		return fmt.Errorf("%w: Buchungskreis %q: %v", sdk.ErrInvalidArgument, cc, err)
	}
	return nil
}

// iso4217 sind die aktiven Währungscodes nach ISO 4217 (Stand 2026).
var iso4217 = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range strings.Fields(`AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BHD BIF BMD BND BOB BOV BRL BSD
		BTN BWP BYN BZD CAD CDF CHE CHF CHW CLF CLP CNY COP COU CRC CUP CVE CZK DJF DKK DOP DZD EGP ERN ETB EUR FJD FKP
		GBP GEL GHS GIP GMD GNF GTQ GYD HKD HNL HTG HUF IDR ILS INR IQD IRR ISK JMD JOD JPY KES KGS KHR KMF KPW KRW KWD
		KYD KZT LAK LBP LKR LRD LSL LYD MAD MDL MGA MKD MMK MNT MOP MRU MUR MVR MWK MXN MXV MYR MZN NAD NGN NIO NOK NPR
		NZD OMR PAB PEN PGK PHP PKR PLN PYG QAR RON RSD RUB RWF SAR SBD SCR SDG SEK SGD SHP SLE SOS SRD SSP STN SVC SYP
		SZL THB TJS TMT TND TOP TRY TTD TWD TZS UAH UGX USD USN UYI UYU UYW UZS VED VES VND VUV WST XAF XAG XAU XBA XBB
		XBC XBD XCD XCG XDR XOF XPD XPF XPT XSU XTS XUA XXX YER ZAR ZMW ZWG`) {
		m[c] = true
	}
	return m
}()

// objectDef liest das Metamodell eines Objects aus dem Catalog (Felder für
// Bedingungen, TitleField für lesbare Verweise).
func (m *Module) objectDef(ctx context.Context, object string) (*metamodel.ObjectDefinition, error) {
	resp, err := m.services.Call(ctx, sdk.ObjectCatalog, "GetDefinition", map[string]any{"object": object})
	if err != nil {
		return nil, err
	}
	var out struct {
		Definition metamodel.ObjectDefinition `json:"definition"`
	}
	if err := sdk.Decode(resp.Payload, &out); err != nil {
		return nil, err
	}
	return &out.Definition, nil
}

// fetch liest einen Datensatz eines anderen Objects über dessen Action get
// (Existenz und Leserecht prüft das Fachmodul bzw. der Dispatcher).
func (m *Module) fetch(ctx context.Context, object, id string) (map[string]any, error) {
	resp, err := m.services.Call(ctx, object, "get", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	rec := map[string]any{}
	if err := sdk.Decode(resp.Payload, &rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// attrText ist die Textform eines Feldwerts für Bedingungen ("true", "42", "RENT").
func attrText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case bool:
		return strconv.FormatBool(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// splitValues zerlegt "RENT, LEASE" bzw. zeilenweise Eingaben in Werte.
func splitValues(s string) []string {
	var out []string
	for _, v := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' || r == '\r' }) {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
