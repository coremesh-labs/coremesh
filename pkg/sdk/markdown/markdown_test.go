package markdown

import (
	"strings"
	"testing"
)

func TestHTML(t *testing.T) {
	got := HTML("# Brief\n\nSehr geehrter Herr *Faust*,  \nsiehe [Vorgang](https://example.org/a?b=1&c=2).\n\n| A | B |\n|---|--:|\n| x | 1 |", Options{})
	for _, want := range []string{"<h1>Brief</h1>", "<em>Faust</em>,<br>siehe", `<a href="https://example.org/a?b=1&amp;c=2">Vorgang</a>`,
		`<table style="border-collapse:collapse">`, `<td style="border:1px solid #ccc;padding:2px 6px;text-align:right">1</td>`} {
		if !strings.Contains(got, want) {
			t.Errorf("fehlt %q in\n%s", want, got)
		}
	}
	if strings.Contains(HTML("[x](javascript:alert(1))", Options{}), "<a") {
		t.Error("javascript-Link")
	}
}
