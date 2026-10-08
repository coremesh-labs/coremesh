// Command document ist das Plugin der Dokumentverweise (Modul documents,
// internal/documents). Andere Plugins und der WebServer nutzen es nur über
// pkg/sdk/docservice (Object Documents) – ein Dokumentenmanagementsystem kann
// es über ein Adapter-Plugin mit derselben Schnittstelle ersetzen.
package main

import (
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/plugin"

	"github.com/coremesh-labs/coremesh/cmd/plugins/document/internal/documents"
)

const version = "0.1.0"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "document", Version: version, Description: "Dokumentverweise für beliebige Objects"},
		documents.New(),
	))
}
