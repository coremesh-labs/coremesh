package main

import (
	"html/template"

	"github.com/coremesh-labs/coremesh/pkg/sdk/markdown"
)

// markdownHTML: Markdown der Texte (Abschnitt „Texte“) als HTML – # wird h3
// (unter den Überschriften der Seite); Umfang siehe pkg/sdk/markdown.
func markdownHTML(src string) template.HTML {
	return template.HTML(markdown.HTML(src, markdown.Options{HeadingShift: 2, TableClass: "md-table"}))
}
