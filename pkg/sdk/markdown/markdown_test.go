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

// TestExtras: unterstrichen, durchgestrichen, Zitat, Trennlinie (seit 0.62.0).
func TestExtras(t *testing.T) {
	got := HTML("++Gretchen++ und ~~Mephisto~~\n\n> Verweile doch,\n> du bist so schön\n\n---\n\n- Punkt", Options{})
	for _, want := range []string{"<u>Gretchen</u>", "<s>Mephisto</s>", "<blockquote", "Verweile doch, du bist so schön</blockquote>", "<hr>", "<li>Punkt</li>"} {
		if !strings.Contains(got, want) {
			t.Errorf("fehlt %q in %s", want, got)
		}
	}
	if got := HTML("<b>x</b> > y", Options{}); strings.Contains(got, "<b>") || strings.Contains(got, "blockquote") {
		t.Errorf("HTML durchgelassen bzw. Zitat mitten in der Zeile: %s", got)
	}
}

// TestBreaks: in Mails bleibt jeder Zeilenwechsel ein Umbruch.
func TestBreaks(t *testing.T) {
	if got := HTML("-- \nFaust\nTel. 1", Options{Breaks: true}); !strings.Contains(got, "Faust<br>Tel. 1") {
		t.Errorf("Breaks: %s", got)
	}
	if got := HTML("Faust\nTel. 1", Options{}); !strings.Contains(got, "Faust Tel. 1") {
		t.Errorf("ohne Breaks: %s", got)
	}
}
