package main

import (
	"strings"
	"testing"
)

// TestMarkdownHTML: Überschriften, Listen, Hervorhebungen, Tabellen; HTML maskiert.
func TestMarkdownHTML(t *testing.T) {
	src := "# Ergebnis\n\nDie **Abrechnung** ist *geprüft*.\nZweite Zeile  \numgebrochen\n\n" +
		"- Heizung\n- Wasser\n  mit Folgezeile\n\n1. erstens\n2. zweitens\n\na) Punkt a\nb) Punkt b\n\n" +
		"| Position | Betrag |\n|---|--:|\n| Hausgeld | 1.200,00 |\n\n<script>alert(1)</script>"
	got := string(markdownHTML(src))
	for _, want := range []string{
		"<h3>Ergebnis</h3>",
		"<p>Die <strong>Abrechnung</strong> ist <em>geprüft</em>. Zweite Zeile<br>umgebrochen</p>",
		"<ul><li>Heizung</li><li>Wasser mit Folgezeile</li></ul>",
		"<ol><li>erstens</li><li>zweitens</li></ol>",
		`<ol type="a"><li>Punkt a</li><li>Punkt b</li></ol>`,
		`<thead><tr><th>Position</th><th style="text-align:right">Betrag</th></tr></thead>`,
		`<td style="text-align:right">1.200,00</td>`,
		"&lt;script&gt;alert(1)&lt;/script&gt;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fehlt %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script>") {
		t.Fatal("HTML nicht maskiert")
	}
}

// TestMarkdownAlign: ohne Angabe wie in PDFs – erste Spalte links, die übrigen rechts.
func TestMarkdownAlign(t *testing.T) {
	got := string(markdownHTML("| A | B | C |\n|---|:--|:-:|\n| a | b | c |\n\nText\n\n| x | y |"))
	for _, want := range []string{`<th>A</th><th>B</th><th style="text-align:center">C</th>`, `<td>x</td><td style="text-align:right">y</td>`} {
		if !strings.Contains(got, want) {
			t.Errorf("fehlt %q in\n%s", want, got)
		}
	}
}
