// Command host ist der CoreMesh-Host.
//
// Startsequenz:
//  1. Konfiguration: alle *.yaml aus -config alphanumerisch einlesen und mergen
//  2. Datenbank: SQL-Pools öffnen und prüfen
//  3. Core-Plugins im Host-Prozess starten, dann DBSchema.Activate ausführen
//  4. Domain-Plugins auflösen (lokal oder Download), starten und registrieren
//
// Danach läuft der Host bis Strg+C. Mit -call führt er stattdessen genau einen
// Aufruf aus und beendet sich, z. B.:
//
//	host -config configs -call Greeting.say -payload '{"name":"Christof"}'
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib" // SQL-Treiber "pgx" (PostgreSQL)
	_ "modernc.org/sqlite"             // SQL-Treiber "sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/coreplugins/catalog"
	"github.com/camel/coremesh/internal/coreplugins/dbschema"
	"github.com/camel/coremesh/internal/coreplugins/event"
	"github.com/camel/coremesh/internal/coreplugins/iam"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/internal/host"
	"github.com/camel/coremesh/internal/resolver"
	"github.com/camel/coremesh/pkg/sdk"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "host:", err)
		os.Exit(1)
	}
}

func run() error {
	configDir := flag.String("config", "configs", "Verzeichnis(se) mit den *.yaml-Konfigurationsdateien, kommagetrennt (z. B. configs,../coremesh-erp/configs)")
	call := flag.String("call", "", "nach dem Start einen Aufruf <Object>.<action> ausführen und beenden")
	payload := flag.String("payload", "null", "JSON-Payload für -call")
	tenant := flag.String("tenant", "", "Mandant für -call")
	user := flag.String("user", "", "Benutzer für -call")
	debug := flag.Bool("v", false, "Debug-Logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Konfiguration
	cfg, files, err := config.LoadDirs(strings.Split(*configDir, ",")...)
	if err != nil {
		return err
	}
	log.Info("1/4 Konfiguration geladen", "dateien", files, "plugins", len(cfg.Plugins))

	// 2. Datenbank-Pools
	db, err := database.Open(ctx, cfg.Databases, database.Options{
		TxTimeout:     cfg.Host.TxTimeout,
		MaxTxPerOwner: cfg.Host.MaxTxPerPlugin,
	}, log.With("component", "database"))
	if err != nil {
		return err
	}
	defer db.Close()
	log.Info("2/4 Datenbank-Pools geöffnet", "datenbanken", db.Names())

	h, err := host.New(cfg, db, log)
	if err != nil {
		return err
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), cfg.Host.ShutdownTimeout)
		defer cancel()
		h.Shutdown(sctx)
	}()

	// 3. Core-Plugins (im Host-Prozess) und Schema-Aktivierung
	if err := h.StartInternal(ctx, map[string]host.InternalFactory{
		dbschema.Name: func(d host.InternalDeps) sdk.Plugin { return dbschema.New(d.DB) },
		catalog.Name:  func(d host.InternalDeps) sdk.Plugin { return catalog.New(d.Catalog) },
		event.Name:    func(d host.InternalDeps) sdk.Plugin { return event.New(d.Catalog) },
		iam.Name:      func(d host.InternalDeps) sdk.Plugin { return iam.New(d.DB) },
	}); err != nil {
		return err
	}
	resp, err := h.ActivateSchema(ctx)
	if err != nil {
		return fmt.Errorf("DBSchema.Activate: %w", err)
	}
	log.Info("3/4 Core-Plugins gestartet, Schema aktiviert", "ergebnis", resp.Payload)

	// 4. Domain-Plugins
	res := resolver.New(cfg.Host.PluginDir, cfg.Resolver.Download, log.With("component", "resolver"), cfg.Host.ExtraPluginDirs...)
	if err := h.StartExternal(ctx, res); err != nil {
		return err
	}
	var routes []string
	for _, r := range h.Routes() {
		routes = append(routes, fmt.Sprintf("%s.%s→%s", r.Object, r.Action, r.Plugin))
	}
	log.Info("4/4 Domain-Plugins gestartet", "routen", routes)

	if *call != "" {
		return doCall(ctx, h, *call, *payload, sdk.CallContext{TenantID: *tenant, UserID: *user})
	}
	log.Info("Host läuft – beenden mit Strg+C")
	<-ctx.Done()
	log.Info("Host wird beendet")
	return nil
}

func doCall(ctx context.Context, h *host.Host, target, payload string, call sdk.CallContext) error {
	object, action, ok := strings.Cut(target, ".")
	if !ok {
		return errors.New("-call erwartet <Object>.<action>")
	}
	var p any
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return fmt.Errorf("-payload: %w", err)
	}
	resp, err := h.Handle(sdk.WithCall(ctx, call), sdk.Request{Object: object, Action: action, Payload: p})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"payload": resp.Payload, "metadata": resp.Metadata})
}
