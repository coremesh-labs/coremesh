package main

import (
	"strings"
	"testing"
)

func TestParseEML(t *testing.T) {
	raw := "From: =?utf-8?q?Gretchen_M=C3=BCller?= <gretchen@faust.example>\r\nTo: faust@faust.example\r\nSubject: =?iso-8859-1?q?Z=E4hlerstand?=\r\n" +
		"Date: Tue, 03 Nov 2026 10:00:00 +0100\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=XX\r\n\r\n" +
		"--XX\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nZeile eins\r\nZeile zwei =C3=BC\r\n" +
		"--XX\r\nContent-Type: application/pdf; name=\"beleg.pdf\"\r\nContent-Disposition: attachment; filename=\"beleg.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0x\r\nLjQK\r\n--XX--\r\n"
	m, err := parseEML([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.From != "Gretchen Müller <gretchen@faust.example>" || m.Subject != "Zählerstand" {
		t.Fatalf("Kopf: %q %q", m.From, m.Subject)
	}
	if m.Text != "Zeile eins\nZeile zwei ü" {
		t.Fatalf("Text: %q", m.Text)
	}
	if len(m.Parts) != 1 || m.Parts[0].Name != "beleg.pdf" || string(m.Parts[0].data) != "%PDF-1.4\n" || !m.Parts[0].Viewable {
		t.Fatalf("Anhang: %+v", m.Parts)
	}
	h, _ := parseEML([]byte("Subject: x\r\nContent-Type: text/html\r\n\r\n<p>Hallo</p><p>Welt<br>zwei</p><script>x()</script>"))
	if !strings.Contains(h.Text, "Hallo\n") || !strings.Contains(h.Text, "Welt\nzwei") || strings.Contains(h.Text, "x()") {
		t.Fatalf("HTML: %q", h.Text)
	}
}
