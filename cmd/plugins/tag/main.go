// Command tag ist das Plugin für die Module TagManagement (internal/tagmanagement,
// pkg/sdk/tagservice) und Texte (internal/textmanagement, seit 0.5.0,
// pkg/sdk/textservice). Die beiden Module haben fachlich nichts gemeinsam.
// Beschreibung: README.md.
package main

import (
	"github.com/coremesh-labs/coremesh/pkg/sdk/module"
	"github.com/coremesh-labs/coremesh/pkg/sdk/plugin"

	"github.com/coremesh-labs/coremesh/cmd/plugins/tag/internal/tagmanagement"
	"github.com/coremesh-labs/coremesh/cmd/plugins/tag/internal/textmanagement"
)

const version = "0.5.4"

func main() {
	plugin.Main(module.NewPlugin(
		module.Info{Name: "tag", Version: version, Description: "TagManagement (Tags, Tag Sets, Regeln) und Texte für beliebige Objects"},
		tagmanagement.New(), textmanagement.New(),
	))
}
