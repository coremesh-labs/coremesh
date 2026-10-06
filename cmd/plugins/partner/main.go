// Command partner ist das Plugin für das fachliche Modul Geschäftspartner
// (internal/businesspartner). main.go verdrahtet nur: Plugin-Identität und
// Module. Manifest, Routing, DBSchema.Init, Catalog.Describe und der
// Lebenszyklus kommen aus pkg/sdk/module. Beschreibung: README.md.
package main

import (
	"github.com/camel/coremesh/pkg/sdk/module"
	"github.com/camel/coremesh/pkg/sdk/plugin"

	"github.com/camel/coremesh/cmd/plugins/partner/internal/businesspartner"
)

const version = "0.3.0"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "partner", Version: version, Description: "Geschäftspartner (SAP-BP-Modell)"},
		businesspartner.New(),
	))
}
