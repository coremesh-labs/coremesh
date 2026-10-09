package crud

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Warnungen: Eine Prüfung (Validate, CheckRecord, Prepare) kann statt eines
// Fehlers eine Warnung melden – der Datensatz wird gespeichert, die Antwort von
// create/update trägt die Texte im Feld "_warnings" (WebServer: gelbe Meldung,
// Konsole: in der Antwort). So lassen sich Prüfungen einstellbar machen
// (Fehler, Warnung, keine Prüfung), siehe Severity.
//
//	if err := crud.Report(ctx, level, "Fläche %.2f größer als Gebäude", sum); err != nil {
//		return err
//	}

// WarningsField: Feld der Antwort mit den Warnungen.
const WarningsField = "_warnings"

// Severity: Stufe einer einstellbaren Prüfung.
type Severity string

const (
	SeverityError   Severity = "ERROR"   // Speichern verhindern
	SeverityWarning Severity = "WARNING" // speichern, Warnung melden
	SeverityNone    Severity = "NONE"    // nicht prüfen
)

type warnKey struct{}

type warnings struct {
	mu   sync.Mutex
	list []string
}

func withWarnings(ctx context.Context) context.Context {
	if _, ok := ctx.Value(warnKey{}).(*warnings); ok {
		return ctx
	}
	return context.WithValue(ctx, warnKey{}, &warnings{})
}

// Warn merkt eine Warnung für die Antwort vor (doppelte Texte einmal).
func Warn(ctx context.Context, format string, args ...any) {
	w, ok := ctx.Value(warnKey{}).(*warnings)
	if !ok {
		return
	}
	msg := fmt.Sprintf(format, args...)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !slices.Contains(w.list, msg) {
		w.list = append(w.list, msg)
	}
}

// Report meldet einen Befund nach seiner Stufe: ERROR als Invalid-Fehler,
// WARNING als Warnung, NONE gar nicht. Leere Stufe gilt als ERROR.
func Report(ctx context.Context, level Severity, format string, args ...any) error {
	switch level {
	case SeverityNone:
		return nil
	case SeverityWarning:
		Warn(ctx, format, args...)
		return nil
	}
	return Invalid(format, args...)
}

// Warnings: die vorgemerkten Warnungen.
func Warnings(ctx context.Context) []string {
	w, ok := ctx.Value(warnKey{}).(*warnings)
	if !ok {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.list)
}

// SeverityOptions: Auswahlwerte für ein Feld mit der Stufe einer Prüfung.
var SeverityOptions = []metamodel.Option{
	{Value: string(SeverityError), Label: "Fehler (nicht speichern)"},
	{Value: string(SeverityWarning), Label: "Warnung"},
	{Value: string(SeverityNone), Label: "keine Prüfung"},
}
