// Package plugin startet ein CoreMesh-Plugin – im Normalbetrieb oder zum
// Debuggen. Allen Funktionen wird eine Referenz auf die Plugin-Struktur
// übergeben, die sdk.Plugin implementiert:
//
//	func main() { plugin.Main(&myPlugin{}) }
//
// Das gRPC-Wire-Protokoll bleibt dabei vollständig in internal/.
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/coremesh-labs/coremesh/internal/adapter"
	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// DebugEnv aktiviert in Main den Debug-Modus, alternativ zum Argument -debug.
const DebugEnv = "COREMESH_PLUGIN_DEBUG"

// Main ist der empfohlene Einstiegspunkt: Mit dem Argument -debug bzw. --debug
// oder COREMESH_PLUGIN_DEBUG=1 läuft das Plugin im Debug-Modus (Debug),
// sonst im Normalbetrieb (Serve).
func Main(p sdk.Plugin) {
	if !debugRequested() {
		Serve(p)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Debug(ctx, p); err != nil {
		fmt.Fprintln(os.Stderr, "plugin:", err)
		os.Exit(1)
	}
}

// Serve startet das Plugin im Normalbetrieb: Der Host startet den Prozess,
// Serve blockiert, bis der Host ihn beendet. Direkt aufgerufen bricht das
// Plugin mit einem Hinweis ab.
func Serve(p sdk.Plugin) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: adapter.Handshake,
		Plugins:         adapter.ServerPlugins(p),
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
	shutdown(p)
}

// ShutdownTimeout begrenzt sdk.Shutdowner.Shutdown. Der Host wartet nach dem
// Beenden-Signal nur kurz (go-plugin: ca. 2 s), bevor er den Prozess beendet.
const ShutdownTimeout = 1500 * time.Millisecond

// shutdown ruft den optionalen Shutdown-Hook auf, nachdem der Host den
// gRPC-Server beendet hat (kein Handle mehr aktiv).
func shutdown(p sdk.Plugin) {
	s, ok := p.(sdk.Shutdowner)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "plugin: Shutdown:", err)
	}
}

// Debug startet das Plugin eigenständig – z. B. aus der IDE oder unter Delve,
// damit Breakpoints greifen. Es gibt aus, wie der Host gestartet werden muss,
// um sich an diesen Prozess anzuhängen (COREMESH_REATTACH_PLUGINS), und
// blockiert, bis ctx beendet wird.
func Debug(ctx context.Context, p sdk.Plugin) error {
	return serveDebug(ctx, p, func(name string, info adapter.ReattachInfo) {
		printReattach(os.Stdout, name, info)
	})
}

func serveDebug(ctx context.Context, p sdk.Plugin, ready func(name string, info adapter.ReattachInfo)) error {
	m, err := p.Manifest(ctx)
	if err != nil {
		return fmt.Errorf("Manifest: %w", err)
	}
	if m.Name == "" {
		return errors.New("Manifest.Name ist leer")
	}

	reattachCh := make(chan *goplugin.ReattachConfig, 1)
	closeCh := make(chan struct{})
	go goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: adapter.Handshake,
		Plugins:         adapter.ServerPlugins(p),
		GRPCServer:      goplugin.DefaultGRPCServer,
		Logger:          hclog.New(&hclog.LoggerOptions{Name: m.Name, Level: hclog.Info, Output: os.Stderr}),
		Test: &goplugin.ServeTestConfig{
			Context:          ctx,
			ReattachConfigCh: reattachCh,
			CloseCh:          closeCh,
		},
	})

	select {
	case cfg := <-reattachCh:
		ready(m.Name, adapter.NewReattachInfo(cfg))
	case <-closeCh:
		return errors.New("Plugin-Server wurde vor dem Start beendet")
	case <-time.After(10 * time.Second):
		return errors.New("Plugin-Server ist nicht rechtzeitig gestartet")
	}
	<-closeCh
	shutdown(p)
	return nil
}

func printReattach(w io.Writer, name string, info adapter.ReattachInfo) {
	b, _ := json.Marshal(map[string]adapter.ReattachInfo{name: info})
	fmt.Fprintf(w, "CoreMesh-Plugin %q läuft im Debug-Modus (PID %d).\n", name, info.Pid)
	fmt.Fprintf(w, "Host in einem zweiten Terminal mit dieser Umgebungsvariable starten:\n\n")
	fmt.Fprintf(w, "  PowerShell: $env:%s='%s'\n", adapter.ReattachEnv, b)
	fmt.Fprintf(w, "  bash:       export %s='%s'\n\n", adapter.ReattachEnv, b)
	fmt.Fprintf(w, "Beenden mit Strg+C.\n")
}

func debugRequested() bool {
	if v := os.Getenv(DebugEnv); v == "1" || v == "true" {
		return true
	}
	return slices.ContainsFunc(os.Args[1:], func(a string) bool { return a == "-debug" || a == "--debug" })
}
