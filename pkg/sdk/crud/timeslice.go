package crud

import (
	"fmt"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Zeitscheiben: valid_from/valid_to als Datum (JJJJ-MM-TT, lexikografisch
// sortierbar). Standard valid_to = DateMax.
const (
	DateMin = "1900-01-01"
	DateMax = "9999-12-31"
)

// TimeSliceFields sind die Felder einer Zeitscheibe.
var TimeSliceFields = []Field{
	{Key: "valid_from", Label: "Gültig ab", Type: metamodel.TypeDate, Listable: true},
	{Key: "valid_to", Label: "Gültig bis", Type: metamodel.TypeDate, Listable: true},
}

// WithTimeSlice hängt die Zeitscheiben-Felder an.
func WithTimeSlice(fs ...Field) []Field { return append(fs, TimeSliceFields...) }

// Invalid ist ein sdk.ErrInvalidArgument mit Text.
func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{sdk.ErrInvalidArgument}, args...)...)
}

// Today ist das heutige Datum (JJJJ-MM-TT).
func Today() string { return time.Now().Format(time.DateOnly) }

// ParseDate akzeptiert JJJJ-MM-TT, auch mit Zeitanteil (RFC 3339) oder als
// time.Time – so liefern SQL-Treiber DATE-Spalten.
func ParseDate(v any) (string, error) {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.DateOnly), nil
	}
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s = s[:10]
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", Invalid("Datum im Format JJJJ-MM-TT erwartet: %q", v)
	}
	return s, nil
}

// CheckTimeSlice setzt Standardwerte (valid_from = heute, valid_to = DateMax)
// und prüft valid_from <= valid_to.
func CheckTimeSlice(rec Record) error {
	if rec["valid_from"] == nil {
		rec["valid_from"] = Today()
	}
	if rec["valid_to"] == nil {
		rec["valid_to"] = DateMax
	}
	from, err := ParseDate(rec["valid_from"])
	if err != nil {
		return err
	}
	to, err := ParseDate(rec["valid_to"])
	if err != nil {
		return err
	}
	if from > to {
		return Invalid("Zeitscheibe: gültig ab (%s) liegt nach gültig bis (%s)", from, to)
	}
	rec["valid_from"], rec["valid_to"] = from, to
	return nil
}

// IncludeHistory: Query-Parameter includeHistory=true (auch 1/on) – list
// liefert dann auch beendete, künftige und inaktive Datensätze.
func IncludeHistory(query map[string]any) bool {
	switch v := query["includeHistory"].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "on"
	}
	return false
}
