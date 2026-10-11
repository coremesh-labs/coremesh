// Package ai ist die KI-Anbindung (Modul ai).
//
//   - Dienste (AiProvider): OpenAI-kompatible Schnittstelle (/chat/completions)
//     mit URL, Verfahren der Anmeldung, Schlüssel (nur schreiben; eigene
//     Tabelle ohne Leseschnittstelle) oder Umgebungsvariable, weiteren Headern.
//   - Anfragen (AiRequest, AiRequestObject): TEXT – Eingang plus Aufgabe;
//     OBJECT – leere Strukturen der genannten Objects (aus dem Metamodell, feste
//     Werte und kleine Kataloge als Auswahl) werden als JSON-Schema mitgegeben,
//     die Antwort ist die gefüllte Struktur. Jede Anfrage ohne Gedächtnis.
//   - Protokoll (AiLog): Eingang, Nachrichten, Antwort, Ergebnis; Verfall nach
//     den Tagen der Anfrage; wiederholbar mit der aktuellen Anfrage.
//   - Schnittstelle (Ai): run, form, collect, create, schema – andere Module
//     (z. B. ein Workflow-Schritt) füllen damit eine Eingabemaske vor und legen
//     das Objekt mit seinen Unter-Objekten an (saveAggregate).
package ai

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/numrange"
)

// Name ist der Namensraum des Moduls (/m/ai).
const Name = "ai"

const (
	providerObject = "AiProvider"
	requestObject  = "AiRequest"
	reqObjObject   = "AiRequestObject"
	logObject      = "AiLog"
	serviceObject  = "Ai"
)

const (
	kindObject = "OBJECT"
	kindText   = "TEXT"

	stDone  = "DONE"
	stError = "ERROR"
)

// httpDoer: HTTP-Aufrufe (Tests ersetzen ihn).
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Module ist die KI-Anbindung.
type Module struct {
	db       module.DB
	services module.Services
	log      *slog.Logger
	set      *crud.Set
	http     httpDoer
	getenv   func(string) string
	now      func() time.Time

	cleanMu   sync.Mutex
	lastClean time.Time
}

var (
	_ module.Module         = (*Module)(nil)
	_ module.SchemaProvider = (*Module)(nil)
	_ module.Translator     = (*Module)(nil)
)

// New erzeugt das Modul.
func New() *Module {
	m := &Module{http: &http.Client{}, getenv: envLookup, now: time.Now}
	m.set = crud.NewSet(m.request(), m.requestObject(), m.logEntity(), m.provider())
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{Name: Name, Title: "KI", Icon: "icon-cpu",
		Description: "KI-Dienste, Anfragen (Text, Objekte füllen) und Protokoll"}
}

func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "KI")
	r.Object(serviceObject).
		Handle("run", m.runAction).
		Handle("form", m.formAction).
		Handle("collect", m.collectAction).
		Handle("create", m.createAction).
		Handle("schema", m.schemaAction)
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	if err := numrange.Define(ctx, env.Services, numrange.Definition{Object: logObject, Owner: Name,
		Description: "Nummern der KI-Anfrageprotokolle je Jahr", PerYear: true, Pattern: "KI-{YYYY}-{N}", Width: 6}); err != nil {
		m.log.WarnContext(ctx, "Nummernkreis nicht angemeldet", "object", logObject, "err", err.Error())
	}
	m.log.InfoContext(ctx, "Modul bereit", "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error { return nil }

func (m *Module) Schema() module.Schema { return module.Schema{HCL: schemaHCL} }

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

func (m *Module) Translations() metamodel.Translations { return translations }

// --- gemeinsame Hilfsfunktionen ------------------------------------------------------

const (
	tText = metamodel.TypeText
	tNum  = metamodel.TypeNumber
	tBool = metamodel.TypeBoolean
	tSel  = metamodel.TypeSelect
	tArea = metamodel.TypeTextarea
	tPass = metamodel.TypePassword
)

func requireWrite(ctx context.Context, object, action string) error {
	ok, err := sdk.CheckAccess(ctx, object, action, "")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s.%s nicht erlaubt", sdk.ErrPermissionDenied, object, action)
	}
	return nil
}

func (m *Module) text(ctx context.Context, q string, args ...any) string {
	res, err := m.db.Query(ctx, q, args...)
	if err != nil || len(res.Rows) == 0 {
		return ""
	}
	return crud.Str(res.Rows[0][0])
}

func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}

func firstOf(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func toInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case nil:
		return 0
	}
	var n int64
	_, _ = fmt.Sscan(crud.Str(v), &n)
	return n
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
