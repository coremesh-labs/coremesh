// Command hello ist ein Beispiel-Plugin. Es importiert bewusst nur pkg/sdk/...,
// genau wie ein externes Plugin-Modul es tun würde – nie internal/.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/camel/coremesh/pkg/sdk"
	"github.com/camel/coremesh/pkg/sdk/metamodel"
	"github.com/camel/coremesh/pkg/sdk/plugin"
)

const version = "0.1.0"

type hello struct {
	greeting string
}

var _ sdk.Plugin = (*hello)(nil)

func (h *hello) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{
		Name:    "hello",
		Version: version,
		Capabilities: []sdk.Capability{
			{Object: "Greeting", Actions: []string{"say", "list", "export"}, Description: "Begrüßt eine Person"},
			// Reservierte Lebenszyklus-Capability: Der Host ruft Init einmal pro Version auf.
			{Object: sdk.ObjectDBSchema, Actions: []string{sdk.ActionInit}},
			{Object: sdk.ObjectCatalog, Actions: []string{sdk.ActionDescribe}},
		},
	}, nil
}

func (h *hello) Configure(_ context.Context, cfg sdk.Config) error {
	h.greeting = "Hallo"
	if g, ok := cfg.Settings["greeting"].(string); ok && g != "" {
		h.greeting = g
	}
	return nil
}

func (h *hello) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	switch {
	case req.Object == sdk.ObjectDBSchema && req.Action == sdk.ActionInit:
		return h.initSchema(req)
	case req.Object == sdk.ObjectCatalog && req.Action == sdk.ActionDescribe:
		return sdk.Response{Payload: metamodel.DescribeResponse{Objects: []metamodel.ObjectDefinition{greetingDef}}}, nil
	case req.Object == "Greeting" && req.Action == "say":
		return h.say(ctx, req)
	case req.Object == "Greeting" && req.Action == "list":
		return h.list(ctx)
	case req.Object == "Greeting" && req.Action == "export":
		return h.export(ctx)
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

// schemaHCL ist das vollständige Soll-Schema dieses Moduls (Atlas-HCL).
// Alle Namen beginnen mit sdk.TablePrefix("hello") == "hello__".
const schemaHCL = `
schema "main" {}

table "hello__greeting_log" {
  schema = schema.main
  column "id" {
    type = text
  }
  column "tenant_id" {
    type = text
    null = true
  }
  column "name" {
    type = text
  }
  column "created_at" {
    type = text
  }
  primary_key {
    columns = [column.id]
  }
  index "hello__greeting_log_name" {
    columns = [column.name]
  }
}
`

// initSchema liefert Modulname und Soll-Schema. Den Abgleich (Diff/Apply)
// macht der Host mit Atlas – isoliert auf hello__* und nur einmal pro Version.
func (h *hello) initSchema(req sdk.Request) (sdk.Response, error) {
	var in sdk.SchemaInitRequest
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: sdk.SchemaInitResponse{Module: in.Module, Schema: schemaHCL}}, nil
}

// sayInput akzeptiert {"name": …} direkt oder – wie vom WebServer – als
// custom-Payload {"data": {"name": …}}.
type sayInput struct {
	Name string `json:"name"`
	Data struct {
		Name string `json:"name"`
	} `json:"data"`
}

// list liefert die letzten Begrüßungen (Kind list im Metamodell).
func (h *hello) list(ctx context.Context) (sdk.Response, error) {
	res, err := sdk.HostFrom(ctx).Query(ctx, "main",
		`SELECT id, name, tenant_id, created_at FROM hello__greeting_log ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return sdk.Response{}, err
	}
	items := make([]any, len(res.Rows))
	for i, r := range res.Rows {
		items[i] = map[string]any{"id": r[0], "name": r[1], "tenant_id": r[2], "created_at": r[3]}
	}
	return sdk.Response{Payload: map[string]any{"items": items}}, nil
}

func (h *hello) say(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	var in sayInput
	if err := sdk.Decode(req.Payload, &in); err != nil {
		return sdk.Response{}, err
	}
	if in.Name == "" {
		in.Name = in.Data.Name
	}
	if in.Name == "" {
		return sdk.Response{}, fmt.Errorf("%w: name fehlt", sdk.ErrInvalidArgument)
	}

	call := sdk.CallFromContext(ctx)
	host := sdk.HostFrom(ctx)
	if _, err := host.Exec(ctx, "main",
		`INSERT INTO hello__greeting_log (id, tenant_id, name, created_at) VALUES (?, ?, ?, ?)`,
		newID(), call.TenantID, in.Name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return sdk.Response{}, err
	}
	res, err := host.Query(ctx, "main", `SELECT COUNT(*) FROM hello__greeting_log WHERE name = ?`, in.Name)
	if err != nil {
		return sdk.Response{}, err
	}
	_ = host.Log(ctx, sdk.LogInfo, "greeting", map[string]string{"name": in.Name})

	return sdk.Response{Payload: map[string]any{
		"message": fmt.Sprintf("%s, %s!", h.greeting, in.Name),
		"tenant":  call.TenantID,
		"greeted": res.Rows[0][0],
	}}, nil
}

func main() {
	plugin.Main(&hello{})
}

// newID erzeugt eine zufällige ID – portabel für SQLite und PostgreSQL
// (INTEGER PRIMARY KEY zählt nur in SQLite automatisch hoch).
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// greetingDef ist das Metamodell des Objects Greeting für Catalog und WebServer.
var greetingDef = metamodel.ObjectDefinition{
	Name:  "Greeting",
	Title: "Begrüßung",
	Icon:  "icon-hand-wave",
	Fields: []metamodel.FieldDefinition{
		{Key: "name", Label: "Name", Type: metamodel.TypeText, Required: true, Listable: true, Editable: true},
		{Key: "tenant_id", Label: "Mandant", Type: metamodel.TypeText, Listable: true},
		{Key: "created_at", Label: "Zeitpunkt", Type: metamodel.TypeText, Listable: true},
	},
	Actions: []metamodel.ActionConfig{
		{Name: "list", Kind: metamodel.KindList, Label: "Übersicht"},
		{Name: "say", Kind: metamodel.KindCustom, Label: "Begrüßen"},
	},
}

// export liefert alle Begrüßungen als ZIP (greetings.csv + LIESMICH.txt).
// Ein []byte unter "zip_content" entpackt das Console-Plugin auf Wunsch in
// ein Verzeichnis (console --target-dir …).
func (h *hello) export(ctx context.Context) (sdk.Response, error) {
	res, err := sdk.HostFrom(ctx).Query(ctx, "main",
		`SELECT id, name, tenant_id, created_at FROM hello__greeting_log ORDER BY created_at`)
	if err != nil {
		return sdk.Response{}, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("export/greetings.csv")
	cw := csv.NewWriter(f)
	cw.Write([]string{"id", "name", "tenant_id", "created_at"})
	for _, r := range res.Rows {
		cw.Write([]string{fmt.Sprint(r[0]), fmt.Sprint(r[1]), fmt.Sprint(r[2]), fmt.Sprint(r[3])})
	}
	cw.Flush()
	readme, _ := zw.Create("export/LIESMICH.txt")
	fmt.Fprintf(readme, "Export des Moduls hello, %d Begrüßungen.\n", len(res.Rows))
	if err := zw.Close(); err != nil {
		return sdk.Response{}, err
	}
	return sdk.Response{Payload: map[string]any{
		"zip_content": buf.Bytes(),
		"greetings":   len(res.Rows),
	}}, nil
}
