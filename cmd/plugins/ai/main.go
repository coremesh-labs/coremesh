// Command ai ist das Plugin der KI-Anbindung (Modul ai, internal/ai): Anfragen
// an OpenAI-kompatible Dienste (z. B. Google Gemini) mit Protokoll; Objekt-
// Anfragen füllen Strukturen aus dem Metamodell (Geschäftspartner, Rechnung …).
package main

import (
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/plugin"

	"github.com/coremesh-labs/coremesh/cmd/plugins/ai/internal/ai"
)

const version = "0.4.2"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "ai", Version: version, Description: "KI: Anfragen an OpenAI-kompatible Dienste, Objekte aus Texten füllen"},
		ai.New(),
	))
}
