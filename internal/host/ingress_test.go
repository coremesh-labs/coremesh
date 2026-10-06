package host

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/camel/coremesh/internal/config"
	"github.com/camel/coremesh/internal/database"
	"github.com/camel/coremesh/pkg/sdk"
)

type echoTenant struct{}

func (echoTenant) Handle(ctx context.Context, _ sdk.Request) (sdk.Response, error) {
	return sdk.Response{Payload: sdk.CallFromContext(ctx).TenantID}, nil
}

func TestIngressStartsRootRequests(t *testing.T) {
	cfg := &config.Config{
		Host:      config.Host{MaxCallDepth: 8},
		Databases: map[string]config.Database{"main": {Driver: "sqlite", DSN: "file:" + filepath.Join(t.TempDir(), "i.db")}},
	}
	log := slog.New(slog.DiscardHandler)
	db, err := database.Open(context.Background(), cfg.Databases, database.Options{}, log)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, err := New(cfg, db, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.disp.Register("partner", sdk.Manifest{Name: "partner", Capabilities: []sdk.Capability{
		{Object: "Partner", Actions: []string{"list"}},
	}}, echoTenant{}); err != nil {
		t.Fatal(err)
	}

	// HTTP-Anfrage im WebServer: kein laufender Host-Request, Mandant vom Ingress.
	ctx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "http-1", TenantID: "demo"})
	req := sdk.Request{Object: "Partner", Action: "list"}

	web := h.services("webserver", config.Plugin{Ingress: true})
	resp, err := web.Handle(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Payload != "demo" {
		t.Fatalf("Mandant des Ingress: %v", resp.Payload)
	}

	// Ohne ingress: true bleibt es beim bisherigen Schutz.
	other := h.services("hello", config.Plugin{})
	if _, err := other.Handle(ctx, req); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Plugin ohne Ingress: %v", err)
	}

	// Datenbankzugriff außerhalb einer Anfrage: für Ingress erlaubt (eigene
	// Wurzelanfrage je Aufruf), für andere Plugins nicht.
	grants := map[string]config.Grant{"main": {Access: "write"}}
	webDB := h.services("webserver", config.Plugin{Ingress: true, Databases: grants})
	dbCtx := sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "http-2"})
	if _, err := webDB.Exec(dbCtx, "main", `CREATE TABLE webserver__t (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := webDB.Exec(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "http-3"}),
		"main", `INSERT INTO webserver__t (id) VALUES (?)`, "a"); err != nil {
		t.Fatal(err)
	}
	res, err := webDB.Query(sdk.WithCall(context.Background(), sdk.CallContext{RequestID: "http-4"}), "main", `SELECT id FROM webserver__t`)
	if err != nil || len(res.Rows) != 1 {
		t.Fatalf("Query: %v %v", res, err)
	}
	plain := h.services("hello", config.Plugin{Databases: grants})
	if _, err := plain.Query(dbCtx, "main", `SELECT 1`); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Query ohne Ingress außerhalb einer Anfrage: %v", err)
	}
	if _, err := webDB.BeginTx(dbCtx, "main", sql.TxOptions{}); err == nil {
		t.Fatal("Transaktion außerhalb einer Anfrage darf nicht gehen")
	}

	// Host-Routen bleiben auch für Ingress-Plugins gesperrt.
	if _, err := web.Handle(ctx, sdk.Request{Object: sdk.ObjectCatalog, Action: "Register"}); !errors.Is(err, sdk.ErrPermissionDenied) {
		t.Fatalf("Host-Route über Ingress: %v", err)
	}
}
