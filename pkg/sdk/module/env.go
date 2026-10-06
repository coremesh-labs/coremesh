package module

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	"github.com/camel/coremesh/pkg/sdk"
)

// Env sind die Ressourcen, die ein Modul in Initialize erhält. Ein Modul
// greift ausschließlich über Env auf seine Umgebung zu – nicht auf globale
// Variablen und nicht direkt auf andere Module.
type Env struct {
	Plugin  string // Name des Plugins (Prozess, DB-Präfix)
	Version string // Version des Plugins
	Module  string // Name des Moduls

	// Log schreibt in das Log des Hosts, mit den Feldern plugin und module.
	// Mit InfoContext(ctx, …) usw. erhält der Eintrag die request_id.
	Log *slog.Logger
	// DB ist die Datenbank des Moduls (Setting database, Standard "main").
	// Zugriffsrechte vergibt die Host-Config (plugins.<name>.databases).
	DB DB
	// Services ruft Actions anderer Module auf – der einzige Weg zu fremden
	// Daten (keine Querverweise auf Datenbankebene).
	Services Services

	config map[string]any
}

// Config dekodiert die Konfiguration des Moduls in v (JSON-Tags). Quelle ist
// settings.modules.<Modul> der Plugin-Config:
//
//	plugins:
//	  partner:
//	    settings:
//	      database: main              # Standard für alle Module
//	      modules:
//	        businesspartner:
//	          default_country: CH     # → Env.Config
func (e Env) Config(v any) error { return sdk.Decode(e.config, v) }

// DB ist der Datenbankzugang eines Moduls, gebunden an eine logische
// Datenbank. Läuft eine Transaktion (InTx), nehmen Query und Exec mit dem
// ctx aus fn automatisch an ihr teil.
type DB interface {
	Name() string
	Query(ctx context.Context, query string, args ...any) (*sdk.QueryResult, error)
	Exec(ctx context.Context, query string, args ...any) (sdk.ExecResult, error)
	InTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error
}

// Services sind Aufrufe an andere Module über den Dispatcher des Hosts. Der
// Aufrufer kennt nur (Object, Action) – nicht das Plugin dahinter.
type Services interface {
	Call(ctx context.Context, object, action string, payload any) (sdk.Response, error)
}

// --- Implementierung über den Host-Rückkanal ---------------------------------

// bind liefert ctx mit Host: In Handle setzt das SDK ihn selbst; in eigenen
// Goroutinen mit frischem Kontext springt der Host aus Configure ein.
func bind(ctx context.Context, fallback sdk.Host) context.Context {
	if sdk.HasHost(ctx) || fallback == nil {
		return ctx
	}
	return sdk.WithHost(ctx, fallback)
}

type hostDB struct {
	name string
	host sdk.Host
}

func (d hostDB) Name() string { return d.name }

func (d hostDB) Query(ctx context.Context, query string, args ...any) (*sdk.QueryResult, error) {
	ctx = bind(ctx, d.host)
	return sdk.HostFrom(ctx).Query(ctx, d.name, query, args...)
}

func (d hostDB) Exec(ctx context.Context, query string, args ...any) (sdk.ExecResult, error) {
	ctx = bind(ctx, d.host)
	return sdk.HostFrom(ctx).Exec(ctx, d.name, query, args...)
}

func (d hostDB) InTx(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
	return sdk.InTx(bind(ctx, d.host), d.name, opts, fn)
}

type hostServices struct{ host sdk.Host }

func (s hostServices) Call(ctx context.Context, object, action string, payload any) (sdk.Response, error) {
	ctx = bind(ctx, s.host)
	return sdk.HostFrom(ctx).Handle(ctx, sdk.Request{Object: object, Action: action, Payload: payload})
}

// hostLog ist ein slog.Handler, der in das Log des Hosts schreibt.
type hostLog struct {
	host  sdk.Host
	level slog.Leveler
	attrs []slog.Attr
	group string
}

func newLogger(host sdk.Host, module string) *slog.Logger {
	return slog.New(&hostLog{host: host, level: slog.LevelDebug}).With("module", module)
}

func (h *hostLog) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level.Level() }

func (h *hostLog) Handle(ctx context.Context, r slog.Record) error {
	fields := map[string]string{}
	var add func(prefix string, a slog.Attr)
	add = func(prefix string, a slog.Attr) {
		a.Value = a.Value.Resolve()
		if a.Value.Kind() == slog.KindGroup {
			for _, g := range a.Value.Group() {
				add(prefix+a.Key+".", g)
			}
			return
		}
		fields[prefix+a.Key] = a.Value.String()
	}
	for _, a := range h.attrs {
		add("", a)
	}
	prefix := ""
	if h.group != "" {
		prefix = h.group + "."
	}
	r.Attrs(func(a slog.Attr) bool { add(prefix, a); return true })

	level := sdk.LogInfo
	switch {
	case r.Level >= slog.LevelError:
		level = sdk.LogError
	case r.Level >= slog.LevelWarn:
		level = sdk.LogWarn
	case r.Level < slog.LevelInfo:
		level = sdk.LogDebug
	}
	ctx = bind(ctx, h.host)
	return sdk.HostFrom(ctx).Log(ctx, level, r.Message, fields)
}

func (h *hostLog) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	if h.group != "" {
		for i := range attrs {
			attrs[i].Key = h.group + "." + attrs[i].Key
		}
	}
	c.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &c
}

func (h *hostLog) WithGroup(name string) slog.Handler {
	c := *h
	c.group = strings.TrimPrefix(h.group+"."+name, ".")
	return &c
}
