package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/camel/coremesh/pkg/consoleapi/console/v1"
	"github.com/camel/coremesh/pkg/sdk"
)

// fakeHost spielt iam, Catalog und ein Fachmodul AssetsModule.
type fakeHost struct {
	sdk.Host
	t       *testing.T
	zip     []byte
	mu      sync.Mutex
	lastReq sdk.Request
	lastCtx sdk.CallContext
}

func (h *fakeHost) Log(context.Context, sdk.LogLevel, string, map[string]string) error { return nil }

func (h *fakeHost) Handle(ctx context.Context, req sdk.Request) (sdk.Response, error) {
	call := sdk.CallFromContext(ctx)
	h.mu.Lock()
	h.lastReq, h.lastCtx = req, call
	h.mu.Unlock()
	users := map[string]map[string]any{
		"u-admin": {"id": "u-admin", "username": "admin", "tenant_id": "demo"},
		"u-leser": {"id": "u-leser", "username": "leser", "tenant_id": "t-7"},
	}
	switch req.Object + "." + req.Action {
	case "Account.Authenticate":
		in := req.Payload.(map[string]any)
		for _, u := range users {
			if u["username"] == in["username"] && in["password"] == "richtig-123" {
				return sdk.Response{Payload: u}, nil
			}
		}
		return sdk.Response{}, fmt.Errorf("%w: Benutzername oder Passwort falsch", sdk.ErrPermissionDenied)
	case "Account.Me":
		if u, ok := users[call.UserID]; ok {
			return sdk.Response{Payload: u}, nil
		}
		return sdk.Response{}, sdk.ErrNotFound
	case "Account.Granted": // Console.ExtractZip nur für admin
		if call.UserID == "u-admin" {
			return sdk.Response{Payload: map[string]any{"all": true, "company_codes": []any{}}}, nil
		}
		return sdk.Response{Payload: map[string]any{"all": false, "company_codes": []any{}}}, nil
	case "Catalog.GetModule":
		return sdk.Response{Payload: map[string]any{"name": "ledger", "commands": []any{map[string]any{
			"name": "load-coa", "object": "ChartOfAccounts", "action": "load",
			"params": []any{map[string]any{"name": "chart", "required": true}, map[string]any{"name": "file", "file": true}}}}}}, nil
	case "ChartOfAccounts.load":
		return sdk.Response{Payload: map[string]any{"loaded": req.Payload}}, nil
	case "Catalog.GetDefinition":
		return sdk.Response{Payload: map[string]any{"definition": partnerDef}}, nil
	case "AssetsModule.ExportBundle":
		// Wie über das Plugin-Protokoll: []byte kommt als Base64-String an.
		return sdk.Response{Payload: map[string]any{"zip_content": base64.StdEncoding.EncodeToString(h.zip), "theme": req.Payload.(map[string]any)["theme"]}}, nil
	}
	return sdk.Response{}, fmt.Errorf("%w: %s.%s", sdk.ErrUnimplemented, req.Object, req.Action)
}

func startService(t *testing.T, h *fakeHost) consolev1.ConsoleServiceClient {
	t.Helper()
	cfg := settings{tokenTTL: time.Hour, MaxExtractBytes: 1 << 20, MaxExtractFiles: 100}
	svc := newService(h, cfg)
	ln := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(svc.authInterceptor))
	consolev1.RegisterConsoleServiceServer(srv, svc)
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return ln.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return consolev1.NewConsoleServiceClient(conn)
}

func login(t *testing.T, c consolev1.ConsoleServiceClient, user string) context.Context {
	t.Helper()
	resp, err := c.Login(context.Background(), &consolev1.LoginRequest{Username: user, Password: "richtig-123"})
	if err != nil {
		t.Fatal(err)
	}
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+resp.Token)
}

func TestLoginRequired(t *testing.T) {
	c := startService(t, &fakeHost{t: t})
	_, err := c.SampleFile(context.Background(), &consolev1.SampleFileRequest{TargetObject: "BusinessPartner"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ohne Token: %v", err)
	}
	if _, err := c.Login(context.Background(), &consolev1.LoginRequest{Username: "admin", Password: "falsch"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("falsches Passwort: %v", err)
	}
	ctx := login(t, c, "admin")
	if _, err := c.Logout(ctx, &consolev1.LogoutRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SampleFile(ctx, &consolev1.SampleFileRequest{TargetObject: "BusinessPartner"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("nach Logout: %v", err)
	}
}

func TestSampleFileRPC(t *testing.T) {
	h := &fakeHost{t: t}
	c := startService(t, h)
	ctx := login(t, c, "leser")
	resp, err := c.SampleFile(ctx, &consolev1.SampleFileRequest{TargetObject: "BusinessPartner"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Format != "yaml" || resp.Filename != "business_partner_sample.yaml" || !strings.Contains(string(resp.Content), "# Pflichtfelder") {
		t.Fatalf("SampleFile: %s %s", resp.Format, resp.Filename)
	}
	// Der Catalog wird als angemeldeter Benutzer gefragt.
	if h.lastCtx.UserID != "u-leser" || h.lastCtx.TenantID != "t-7" || h.lastCtx.Metadata["ingress"] != "console" {
		t.Fatalf("CallContext: %+v", h.lastCtx)
	}
}

func TestExecuteWithZip(t *testing.T) {
	h := &fakeHost{t: t, zip: makeZip(t, zentry{name: "static/app.css", body: "body{}"}, zentry{name: "index.html", body: "<h1/>"})}
	c := startService(t, h)
	dir := filepath.Join(t.TempDir(), "www")
	params, _ := structpb.NewStruct(map[string]any{"theme": "dark"})

	// leser: Action erlaubt (fake), aber kein Console.ExtractZip.
	_, err := c.Execute(login(t, c, "leser"), &consolev1.ExecuteRequest{TargetObject: "AssetsModule", TargetAction: "ExportBundle",
		Parameters: params, TargetDirectory: dir})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ohne Console.ExtractZip: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("trotz fehlender Berechtigung entpackt")
	}

	// admin: target_directory als Parameter – wird nicht ans Modul weitergereicht.
	params, _ = structpb.NewStruct(map[string]any{"theme": "dark", "target_directory": dir})
	resp, err := c.Execute(login(t, c, "admin"), &consolev1.ExecuteRequest{TargetObject: "AssetsModule", TargetAction: "ExportBundle", Parameters: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Extract.GetFiles() != 2 || !strings.Contains(resp.Message, "2 Dateien") {
		t.Fatalf("Ergebnis: %+v %q", resp.Extract, resp.Message)
	}
	if _, ok := h.lastReq.Payload.(map[string]any)["target_directory"]; ok && h.lastReq.Object == "AssetsModule" {
		t.Fatal("target_directory an das Modul weitergegeben")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "static", "app.css")); string(b) != "body{}" {
		t.Fatalf("Datei: %q", b)
	}
	if p := resp.Payload.GetStructValue().AsMap(); p["theme"] != "dark" || p["zip_content"] != nil {
		t.Fatalf("Payload ohne zip_content erwartet: %v", p)
	}

	// Systemverzeichnis → PermissionDenied.
	bad := "/etc/coremesh"
	if os.PathSeparator == '\\' {
		bad = `C:\Windows\CoreMesh`
	}
	_, err = c.Execute(login(t, c, "admin"), &consolev1.ExecuteRequest{TargetObject: "AssetsModule", TargetAction: "ExportBundle", TargetDirectory: bad})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Systemverzeichnis: %v", err)
	}
}

// TestModuleCommand: <modul>:<befehl> wird über den Catalog zu Object.Action aufgelöst.
func TestModuleCommand(t *testing.T) {
	h := &fakeHost{t: t}
	c := startService(t, h)
	ctx := login(t, c, "admin")
	p, _ := structpb.NewStruct(map[string]any{"chart": "SKR04"})
	resp, err := c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: "ledger:load-coa", Parameters: p})
	if err != nil {
		t.Fatal(err)
	}
	if h.lastReq.Object != "ChartOfAccounts" || h.lastReq.Action != "load" || h.lastReq.Payload.(map[string]any)["chart"] != "SKR04" {
		t.Fatalf("Aufruf: %+v", h.lastReq)
	}
	if !strings.Contains(resp.Payload.String(), "SKR04") {
		t.Fatalf("Antwort: %v", resp.Payload)
	}
	// Liste der Befehle.
	resp, err = c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: "ledger:help"})
	if err != nil || !strings.Contains(resp.Payload.String(), "load-coa") {
		t.Fatalf("help: %v %v", resp, err)
	}
	for name, tc := range map[string]struct {
		target string
		params map[string]any
		code   codes.Code
	}{
		"unbekannter Befehl":    {"ledger:load-xyz", map[string]any{}, codes.NotFound},
		"Pflichtparameter":      {"ledger:load-coa", map[string]any{}, codes.InvalidArgument},
		"unbekannter Parameter": {"ledger:load-coa", map[string]any{"chart": "SKR04", "kontenplan": "x"}, codes.InvalidArgument},
	} {
		p, _ := structpb.NewStruct(tc.params)
		if _, err := c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: tc.target, Parameters: p}); status.Code(err) != tc.code {
			t.Errorf("%s: %v", name, err)
		}
	}
}
