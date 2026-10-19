// Package script: Skripte und Berichte (Modul script).
//
//   - Skripte (Script): JavaScript (goja) mit Eingabeparametern
//     (ScriptParameter). Zugriff auf fremde Daten nur über die Objects der
//     Module (AccessObject, ReadObject, CallTemplate) – je Skript auf die
//     erlaubten Aufrufe beschränkt (Muster Object.action) und, wenn ein
//     Benutzer angemeldet ist, auf seine Berechtigungen.
//   - Bindungen (ScriptBinding): Skript als Event-Handler (SystemEvent, nach dem
//     Speichern, asynchron) oder Hook-Abonnent (modify, check, commit; synchron).
//     Abonniert wird je Object/Action bzw. Hook/Phase einmal über die Objects
//     ScriptEvent und ScriptHook; welche Skripte laufen, steht in den Bindungen.
//   - Berichte (Report): Datenquelle Object (Call oder Datenstrom) oder Skript,
//     Spalten mit Format und Verdichtung (ReportColumn), Gruppen, Sortierung,
//     Summenzeile, Ausgabe als Tabelle, CSV oder Vorlage (Modul Vorlagen).
//   - Protokoll (ScriptRun): Läufe mit Meldung, Ausgaben von log() und Dauer.
//   - Export und Import (YAML, format coremesh.scripts) über die Fachobjekte.
package script

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/events"
	"github.com/coremesh-labs/coremesh/pkg/sdk/hook"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
)

// Name ist der Namensraum des Moduls (/m/script).
const Name = "script"

const (
	scriptObject      = "Script"
	paramObject       = "ScriptParameter"
	bindingObject     = "ScriptBinding"
	runObject         = "ScriptRun"
	reportObject      = "Report"
	reportColObject   = "ReportColumn"
	reportParamObject = "ReportParameter"

	// Callbacks der Abos (feste Actions onEvent bzw. onHook).
	eventObject = "ScriptEvent"
	hookObject  = "ScriptHook"
)

// Auslöser eines Laufs.
const (
	trigRun    = "RUN"
	trigEvent  = "EVENT"
	trigHook   = "HOOK"
	trigReport = "REPORT"

	stDone  = "DONE"
	stError = "ERROR"
)

// settings: settings.modules.script der Plugin-Konfiguration.
type settings struct {
	MaxReadRows   int `json:"max_read_rows"`   // Zeilen je ReadObject bzw. Bericht (Standard 100000)
	MaxCalls      int `json:"max_calls"`       // Aufrufe je Lauf (Standard 10000)
	MaxTimeoutSec int `json:"max_timeout_sec"` // obere Grenze für das Zeitlimit eines Skripts (Standard 600)
	LogBytes      int `json:"log_bytes"`       // Ausgaben von log() je Lauf (Standard 65536)
	SyncDelayMs   int `json:"sync_delay_ms"`   // Abos nach Änderungen der Bindungen erneuern (Standard 1500)
}

// Module: Skripte und Berichte.
type Module struct {
	db       module.DB
	services module.Services
	log      *slog.Logger
	set      *crud.Set
	cfg      settings
	now      func() time.Time

	// background: Abos im Hintergrund erneuern (Tests schalten es ab).
	background bool
	syncCh     chan struct{}
	stop       chan struct{}
	done       chan struct{}

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
	m := &Module{now: time.Now, background: true}
	m.set = crud.NewSet(m.scriptEntity(), m.paramEntity(paramObject, "script__parameter", "script", "Skript", refScript),
		m.bindingEntity(), m.runEntity(), m.reportEntity(), m.reportColumnEntity(),
		m.paramEntity(reportParamObject, "script__report_parameter", "report", "Bericht", refReport))
	return m
}

func (m *Module) Descriptor() module.Descriptor {
	return module.Descriptor{Name: Name, Title: "Skripte und Berichte", Icon: "icon-code",
		Description: "JavaScript über die Objects aller Module (Maske, Events, Hooks) und eigene Berichte"}
}

func (m *Module) RegisterRoutes(r *module.Router) {
	m.set.Register(r, "Skripte")
	r.Object(eventObject).Handle(events.CallbackAction, m.onEvent)
	hook.Handle(r, hookObject, m.onHook)
	r.Command(metamodel.CommandDefinition{Name: "export", Object: scriptObject, Action: "export",
		Description: "Skripte und Berichte als Datei ausgeben (YAML, format coremesh.scripts), z. B. --out=skripte.yaml",
		Params:      []metamodel.CommandParam{{Name: "codes", Description: "nur diese Skripte und Berichte (Komma-Liste)"}}})
	r.Command(metamodel.CommandDefinition{Name: "import", Object: scriptObject, Action: "import",
		Description: "Skripte und Berichte aus einer Datei von script:export anlegen bzw. ändern",
		Params: []metamodel.CommandParam{{Name: "file", File: true, Required: true},
			{Name: "dry-run", Type: metamodel.ParamBoolean, Description: "nur prüfen, nichts ändern"}}})
}

func (m *Module) Initialize(ctx context.Context, env module.Env) error {
	m.db, m.services, m.log = env.DB, env.Services, env.Log
	m.set.Bind(env.DB)
	if err := env.Config(&m.cfg); err != nil {
		return err
	}
	m.cfg.MaxReadRows = orDefault(m.cfg.MaxReadRows, 100000)
	m.cfg.MaxCalls = orDefault(m.cfg.MaxCalls, 10000)
	m.cfg.MaxTimeoutSec = orDefault(m.cfg.MaxTimeoutSec, 600)
	m.cfg.LogBytes = orDefault(m.cfg.LogBytes, 65536)
	m.cfg.SyncDelayMs = orDefault(m.cfg.SyncDelayMs, 1500)
	if m.background {
		m.syncCh, m.stop, m.done = make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
		go m.syncLoop()
		m.requestSync() // nach dem Start: Event-Abos liegen beim Dispatcher nur im Speicher
	}
	m.log.InfoContext(ctx, "Modul bereit", "database", env.DB.Name())
	return nil
}

func (m *Module) Shutdown(context.Context) error {
	if m.stop != nil {
		close(m.stop)
		select {
		case <-m.done:
		case <-time.After(time.Second):
		}
	}
	return nil
}

func (m *Module) Schema() module.Schema { return module.Schema{HCL: schemaHCL} }

//go:embed i18n/*.json
var i18nFiles embed.FS

var translations = module.MustLoadTranslations(i18nFiles, "i18n")

func (m *Module) Translations() metamodel.Translations { return translations }

// --- Abos -----------------------------------------------------------------------------

// requestSync: Abos demnächst erneuern (nach dem Commit, im Hintergrund).
func (m *Module) requestSync() {
	if m.syncCh == nil {
		return
	}
	select {
	case m.syncCh <- struct{}{}:
	default:
	}
}

// syncLoop erneuert die Abos nach einer kurzen Pause – außerhalb der
// Transaktion, in der die Bindung gespeichert wurde (eigene Systemanfrage,
// das Plugin braucht dafür ingress: true).
func (m *Module) syncLoop() {
	defer close(m.done)
	for {
		select {
		case <-m.stop:
			return
		case <-m.syncCh:
		}
		select {
		case <-m.stop:
			return
		case <-time.After(time.Duration(m.cfg.SyncDelayMs) * time.Millisecond):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := m.syncSubscriptions(ctx); err != nil {
			m.log.WarnContext(ctx, "Abos der Skripte nicht erneuert", "err", err.Error())
		}
		cancel()
	}
}

// syncSubscriptions meldet je Object/Action/Buchungskreis (Events) bzw.
// Hook/Phase (mit der kleinsten Priorität der Bindungen) ein Abo an. Abos
// ohne aktive Bindung bleiben bestehen und laufen ins Leere.
func (m *Module) syncSubscriptions(ctx context.Context) error {
	res, err := m.db.Query(ctx, `SELECT DISTINCT b.event_object, b.event_action, b.company_code FROM script__binding b
		JOIN script__script s ON s.code = b.script WHERE b.kind = ? AND b.is_active = ? AND s.is_active = ?`, trigEvent, true, true)
	if err != nil {
		return err
	}
	for _, r := range res.Rows {
		sub := events.Subscription{Object: crud.Str(r[0]), Action: firstOf(crud.Str(r[1]), events.All),
			CompanyCode: firstOf(crud.Str(r[2]), events.All), Callback: eventObject}
		if err := events.Register(ctx, m.services, sub); err != nil {
			m.log.WarnContext(ctx, "Event-Abo fehlgeschlagen", "object", sub.Object, "action", sub.Action, "err", err.Error())
		}
	}
	res, err = m.db.Query(ctx, `SELECT b.hook, b.phase, MIN(b.priority) FROM script__binding b
		JOIN script__script s ON s.code = b.script WHERE b.kind = ? AND b.is_active = ? AND s.is_active = ? GROUP BY b.hook, b.phase`, trigHook, true, true)
	if err != nil {
		return err
	}
	for _, r := range res.Rows {
		sub := hook.Subscription{Hook: crud.Str(r[0]), Phase: crud.Str(r[1]), Callback: hookObject, Priority: int(toInt(r[2])),
			Description: "Skripte (Bindungen im Modul Skripte und Berichte)"}
		if err := hook.Subscribe(ctx, m.services, sub); err != nil {
			m.log.WarnContext(ctx, "Hook-Abo fehlgeschlagen", "hook", sub.Hook, "phase", sub.Phase, "err", err.Error())
		}
	}
	return nil
}

// --- gemeinsame Hilfsfunktionen ------------------------------------------------------

const (
	tText = metamodel.TypeText
	tNum  = metamodel.TypeNumber
	tBool = metamodel.TypeBoolean
	tSel  = metamodel.TypeSelect
	tArea = metamodel.TypeTextarea
	tDate = metamodel.TypeDate
	tFile = metamodel.TypeFile
)

func (m *Module) text(ctx context.Context, q string, args ...any) string {
	res, err := m.db.Query(ctx, q, args...)
	if err != nil || len(res.Rows) == 0 {
		return ""
	}
	return crud.Str(res.Rows[0][0])
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
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
