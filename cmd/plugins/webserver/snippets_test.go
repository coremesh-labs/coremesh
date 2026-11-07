package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestSnippetAndPreview(t *testing.T) {
	s, h := newTestServer(t, "templates")
	w := do(s, "POST", "/snippet", url.Values{"_snippet_object": {"Snip"}, "_snippet_code": {"ANSCHRIFT"}, "_snippet_key": {"bp/7"}, "subject": {"Gretchen"}}, true)
	if w.Code != 200 || w.Body.String() != "Anschrift bp/7 für Gretchen" {
		t.Fatalf("snippet: %d %q", w.Code, w.Body.String())
	}
	if got := h.last().Payload.(map[string]any)["code"]; got != "ANSCHRIFT" {
		t.Errorf("code %v", got)
	}
	if w := do(s, "POST", "/snippet", url.Values{"_snippet_object": {"snip"}, "_snippet_code": {"X"}}, true); w.Code < 400 {
		t.Errorf("ungültiges Object angenommen: %d", w.Code)
	}
	b := do(s, "POST", "/markdown/preview?field=body", url.Values{"body": {"# Hallo\n\n**Faust** <script>"}}, true).Body.String()
	if !strings.Contains(b, "<h3>Hallo</h3>") || !strings.Contains(b, "<strong>Faust</strong> &lt;script&gt;") {
		t.Errorf("preview: %s", b)
	}
}
