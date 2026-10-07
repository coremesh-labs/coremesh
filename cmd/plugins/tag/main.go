// Command tag ist das Plugin für das Modul TagManagement
// (internal/tagmanagement). Andere Plugins nutzen es über pkg/sdk/tagservice.
// Beschreibung: README.md.
package main

import (
	"github.com/coremesh-lab/coremesh/pkg/sdk/module"
	"github.com/coremesh-lab/coremesh/pkg/sdk/plugin"

	"github.com/coremesh-lab/coremesh/cmd/plugins/tag/internal/tagmanagement"
)

const version = "0.2.0"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "tag", Version: version, Description: "TagManagement: Tags, Tag Sets und Regeln für beliebige Objects"},
		tagmanagement.New(),
	))
}
