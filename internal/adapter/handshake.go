// Package adapter übersetzt zwischen den öffentlichen SDK-Typen (pkg/sdk) und
// dem internen Wire-Protokoll (internal/api/plugin/v1, gRPC über HashiCorp
// go-plugin). Plugin-Entwickler sehen davon nichts; sie nutzen pkg/sdk/plugin.
package adapter

import (
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// PluginName ist der Schlüssel, unter dem Host und Plugin das Plugin registrieren.
const PluginName = "plugin"

// Handshake muss bei Host und Plugin identisch sein. Eine Änderung von
// ProtocolVersion macht ältere Plugins bewusst inkompatibel.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "COREMESH_PLUGIN",
	MagicCookieValue: "coremesh-7c2f9a41-handshake-v1",
}

// ServerPlugins liefert die Registrierung für goplugin.ServeConfig (Plugin-Seite).
func ServerPlugins(impl sdk.Plugin) goplugin.PluginSet {
	return goplugin.PluginSet{PluginName: &GRPCPlugin{Impl: impl}}
}

// ClientPlugins liefert die Registrierung für goplugin.ClientConfig (Host-Seite).
// Nach Dispense(PluginName) ist das Ergebnis ein sdk.Plugin.
func ClientPlugins() goplugin.PluginSet {
	return goplugin.PluginSet{PluginName: &GRPCPlugin{}}
}
