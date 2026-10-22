// Command script ist das Plugin für Skripte und Berichte (Modul script,
// internal/script): JavaScript (goja) mit Zugriff auf die Objects aller
// Module – ausführbar über eine Maske, als Event-Handler oder Hook-Abonnent –
// und Berichte aus Objekten oder Skripten (Spalten, Gruppen, Summen, Vorlage).
// Beschreibung: README.md.
package main

import (
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/plugin"

	"github.com/coremesh-labs/coremesh/cmd/plugins/script/internal/script"
)

const version = "0.2.3"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "script", Version: version, Description: "Skripte (JavaScript) und Berichte über die Objects aller Module"},
		script.New(),
	))
}
