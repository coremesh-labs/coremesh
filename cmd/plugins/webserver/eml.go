package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/htmlindex"
)

// Ansicht einer Mail (.eml, message/rfc822; seit 0.64.7): Statt sie herunterzuladen zeigt
// /file/{object}/{id} Kopf, Text (Zeilenumbrüche erhalten; aus HTML als Text) und die
// Anhänge – jeder ansehen bzw. herunterladen (?part=N), die ganze Mail mit ?download=1.
// Alles als Text in der eigenen Vorlage: kein HTML und kein Skript aus der Mail.

type emlMsg struct {
	From, To, Cc, Date, Subject string
	Text                        string
	Parts                       []emlPart
}

type emlPart struct {
	No          int
	Name, Type  string
	Size        int
	Viewable    bool
	data        []byte
	contentType string
}

var emlWords = &mime.WordDecoder{CharsetReader: func(cs string, in io.Reader) (io.Reader, error) {
	e, err := htmlindex.Get(cs)
	if err != nil {
		return nil, err
	}
	return e.NewDecoder().Reader(in), nil
}}

func emlHeader(h mail.Header, k string) string {
	v := h.Get(k)
	if d, err := emlWords.DecodeHeader(v); err == nil {
		return d
	}
	return v
}

func parseEML(raw []byte) (*emlMsg, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("Mail nicht lesbar: %w", err)
	}
	m := &emlMsg{From: emlHeader(msg.Header, "From"), To: emlHeader(msg.Header, "To"), Cc: emlHeader(msg.Header, "Cc"),
		Subject: emlHeader(msg.Header, "Subject"), Date: msg.Header.Get("Date")}
	if t, err := msg.Header.Date(); err == nil {
		m.Date = t.Local().Format("02.01.2006 15:04")
	}
	var plain, htm string
	walkEML(msg.Header, msg.Body, m, &plain, &htm, 0)
	m.Text = strings.TrimSpace(plain)
	if m.Text == "" && htm != "" {
		m.Text = htmlText(htm)
	}
	return m, nil
}

type headerGetter interface{ Get(string) string }

func walkEML(h headerGetter, body io.Reader, m *emlMsg, plain, htm *string, depth int) {
	if depth > 10 {
		return
	}
	ct := h.Get("Content-Type")
	typ, params, err := mime.ParseMediaType(ct)
	if err != nil || typ == "" {
		typ, params = "text/plain", map[string]string{}
	}
	data := decodeTransfer(h.Get("Content-Transfer-Encoding"), body)
	if strings.HasPrefix(typ, "multipart/") {
		mr := multipart.NewReader(bytes.NewReader(data), params["boundary"])
		for {
			p, err := mr.NextRawPart()
			if err != nil {
				return
			}
			walkEML(p.Header, p, m, plain, htm, depth+1)
		}
	}
	disp, dparams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	name := dparams["filename"]
	if name == "" {
		name = params["name"]
	}
	if d, err := emlWords.DecodeHeader(name); err == nil {
		name = d
	}
	if disp != "attachment" && name == "" {
		switch typ {
		case "text/plain":
			if *plain == "" {
				*plain = charsetText(data, params["charset"])
			}
			return
		case "text/html":
			if *htm == "" {
				*htm = charsetText(data, params["charset"])
			}
			return
		}
	}
	no := len(m.Parts) + 1
	if name == "" {
		name = fmt.Sprintf("Teil %d", no)
		if exts, _ := mime.ExtensionsByType(typ); len(exts) > 0 {
			name += exts[0]
		}
	}
	m.Parts = append(m.Parts, emlPart{No: no, Name: name, Type: typ, Size: len(data), Viewable: inlineTypes[typ], data: data, contentType: typ})
}

func decodeTransfer(enc string, r io.Reader) []byte {
	var src io.Reader = r
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "base64":
		src = base64.NewDecoder(base64.StdEncoding, newlineStripper{r})
	case "quoted-printable":
		src = quotedprintable.NewReader(r)
	}
	b, _ := io.ReadAll(src)
	return b
}

// newlineStripper: Base64 in Mails ist zeilenweise umbrochen.
type newlineStripper struct{ r io.Reader }

func (n newlineStripper) Read(p []byte) (int, error) {
	k, err := n.r.Read(p)
	j := 0
	for _, c := range p[:k] {
		if c != '\r' && c != '\n' && c != ' ' && c != '\t' {
			p[j] = c
			j++
		}
	}
	return j, err
}

func charsetText(b []byte, cs string) string {
	if cs != "" && !strings.EqualFold(cs, "utf-8") && !strings.EqualFold(cs, "us-ascii") {
		if e, err := htmlindex.Get(cs); err == nil {
			if d, err := e.NewDecoder().Bytes(b); err == nil {
				b = d
			}
		}
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

var blankLines = regexp.MustCompile(`\n{3,}`)

// htmlText: Text einer HTML-Mail mit Absätzen (Zeilenumbruch bei br, p, div, li, tr …).
func htmlText(s string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(blankLines.ReplaceAllString(b.String(), "\n\n"))
		case html.TextToken:
			if skip == 0 {
				b.WriteString(strings.Join(strings.Fields(string(z.Text())), " "))
			}
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			n, _ := z.TagName()
			tag := string(n)
			start := z.Token().Type != html.EndTagToken
			switch tag {
			case "script", "style", "head":
				if start {
					skip++
				} else if skip > 0 {
					skip--
				}
			case "br":
				b.WriteString("\n")
			case "p", "div", "tr", "li", "h1", "h2", "h3", "h4", "blockquote", "table":
				b.WriteString("\n")
			case "td", "th":
				b.WriteString(" ")
			}
		}
	}
}

// serveEML: Ansicht bzw. ein Anhang (?part=N) einer Mail.
func (s *server) serveEML(w http.ResponseWriter, r *http.Request, object, id, fileName string, raw []byte) {
	m, err := parseEML(raw)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if ps := r.URL.Query().Get("part"); ps != "" {
		no, _ := strconv.Atoi(ps)
		if no < 1 || no > len(m.Parts) {
			http.NotFound(w, r)
			return
		}
		p := m.Parts[no-1]
		typ, disposition := p.contentType, "inline"
		if r.URL.Query().Get("download") != "" || !inlineTypes[typ] {
			disposition = "attachment"
			if !inlineTypes[typ] {
				typ = "application/octet-stream"
			}
		}
		if typ == "text/plain" {
			typ += "; charset=utf-8"
		}
		h := w.Header()
		h.Set("Content-Type", typ)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Content-Security-Policy", "frame-ancestors 'self'")
		h.Set("Cache-Control", "private, no-store")
		h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": p.Name}))
		_, _ = w.Write(p.data)
		return
	}
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
	w.Header().Set("Cache-Control", "private, no-store")
	s.renderStandalone(w, r, "eml-view", map[string]any{"Mail": m, "URL": fileURL(object, id), "FileName": fileName})
}

// renderStandalone: eigene Seite ohne Layout (z. B. im Viewer-iframe).
func (s *server) renderStandalone(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	data["Theme"] = s.cfg.Theme
	var buf bytes.Buffer
	if err := s.views.fragment(localeFrom(r), &buf, name, data); err != nil {
		s.logError(r, "Template "+name, err)
		http.Error(w, s.T(r, "core.app.render_error"), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}
