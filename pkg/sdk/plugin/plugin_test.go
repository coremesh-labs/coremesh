package plugin

import (
	"context"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/coremesh-lab/coremesh/internal/adapter"
	"github.com/coremesh-lab/coremesh/pkg/sdk"
)

type echoPlugin struct{}

func (echoPlugin) Manifest(context.Context) (sdk.Manifest, error) {
	return sdk.Manifest{Name: "echo", Version: "1.0.0",
		Capabilities: []sdk.Capability{{Object: "Echo", Actions: []string{"say"}}}}, nil
}
func (echoPlugin) Configure(context.Context, sdk.Config) error { return nil }
func (echoPlugin) Handle(_ context.Context, req sdk.Request) (sdk.Response, error) {
	return sdk.Response{Payload: req.Payload}, nil
}

// TestDebugReattach startet ein Plugin im Debug-Modus und hängt sich wie der
// Host über die JSON-Reattach-Info daran an.
func TestDebugReattach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	infoCh := make(chan adapter.ReattachInfo, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveDebug(ctx, echoPlugin{}, func(name string, info adapter.ReattachInfo) {
			if name != "echo" {
				t.Errorf("Name: %q", name)
			}
			infoCh <- info
		})
	}()

	cfg, err := (<-infoCh).Config()
	if err != nil {
		t.Fatal(err)
	}
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  adapter.Handshake,
		Plugins:          adapter.ClientPlugins(),
		Reattach:         cfg,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})
	rpc, err := client.Client()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := rpc.Dispense(adapter.PluginName)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := raw.(sdk.Plugin).Handle(ctx, sdk.Request{Object: "Echo", Action: "say", Payload: "hallo"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Payload != "hallo" {
		t.Fatalf("Payload: %v", resp.Payload)
	}

	client.Kill()
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
