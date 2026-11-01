package main

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// Markdown der Texte (Abschnitt „Texte“): ein kleiner, sicherer Teil – alles wird
// erst maskiert, eigenes HTML kommt so nie auf die Seite. Derselbe Teil wie bei
// den PDF-Vorlagen (mdtex im Plugin template):
//
//	# Titel, ## Abschnitt, ### Unterabschnitt   Überschriften
//	Leerzeile                                   neuer Absatz; zwei Leerzeichen oder \ am Zeilenende: Zeilenumbruch
//	**fett**, *kursiv*
//	- Punkt  bzw.  * Punkt                      Aufzählung
//	1. Punkt                                    nummerierte Liste
//	a) Punkt                                    Liste a), b), …
//	| Spalte | Spalte |                         Tabelle; |---|--:| Linie unter dem Kopf,
//	                                            :-- links, --: rechts, :-: mittig
//
// Folgezeilen eines Listenpunkts (eingerückt oder ohne neues Zeichen) gehören zu ihm.

var (
	mdHeading = regexp.MustCompile(`^\s*(#{1,4})\s+(.*?)\s*#*\s*$`)
	mdBullet  = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)
	mdNumber  = regexp.MustCompile(`^\s*(\d+)[.)]\s+(.*)$`)
	mdAlpha   = regexp.MustCompile(`^\s*([a-z])\)\s+(.*)$`)
	mdInline  = regexp.MustCompile(`\*\*(.+?)\*\*|\*([^*\s][^*]*?)\*`)
	mdRule    = regexp.MustCompile(`^:?-+:?$`)
)

// markdownHTML: Text als HTML (maskiert, nur die Strukturen oben).
func markdownHTML(src string) template.HTML {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	var para []string
	list, item := "", []string(nil) // offene Liste: ul, ol, ola
	var items [][]string
	var table []string
	flushPara := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + mdLines(para) + "</p>\n")
			para = nil
		}
	}
	flushList := func() {
		if item != nil {
			items = append(items, item)
			item = nil
		}
		if list == "" {
			return
		}
		tag, attr := list, ""
		if list == "ola" {
			tag, attr = "ol", ` type="a"`
		}
		b.WriteString("<" + tag + attr + ">")
		for _, it := range items {
			b.WriteString("<li>" + mdLines(it) + "</li>")
		}
		b.WriteString("</" + tag + ">\n")
		list, items = "", nil
	}
	flushTable := func() {
		if len(table) > 0 {
			b.WriteString(mdTableHTML(table))
			table = nil
		}
	}
	flush := func() { flushPara(); flushList(); flushTable() }
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trim := strings.TrimSpace(line)
		switch {
		case trim == "":
			flushPara()
			flushList()
			// Tabellenzeilen über Leerzeilen hinweg bleiben eine Tabelle
		case strings.HasPrefix(trim, "|"):
			flushPara()
			flushList()
			table = append(table, trim)
		case mdHeading.MatchString(line):
			flush()
			m := mdHeading.FindStringSubmatch(line)
			lvl := len(m[1]) + 2 // # → h3 (unter den Überschriften der Seite)
			if lvl > 6 {
				lvl = 6
			}
			tag := "h" + string(rune('0'+lvl))
			b.WriteString("<" + tag + ">" + mdInlineHTML(m[2]) + "</" + tag + ">\n")
		case mdBullet.MatchString(line) || mdNumber.MatchString(line) || mdAlpha.MatchString(line):
			flushPara()
			flushTable()
			kind, text := "ul", ""
			if m := mdBullet.FindStringSubmatch(line); m != nil {
				text = m[1]
			} else if m := mdNumber.FindStringSubmatch(line); m != nil {
				kind, text = "ol", m[2]
			} else {
				kind, text = "ola", mdAlpha.FindStringSubmatch(line)[2]
			}
			if list != kind {
				flushList()
				list = kind
			}
			if item != nil {
				items = append(items, item)
			}
			item = []string{text}
		default:
			flushTable()
			if list != "" {
				item = append(item, raw) // Folgezeile des Listenpunkts
				continue
			}
			para = append(para, raw)
		}
	}
	flush()
	return template.HTML(b.String())
}

// mdLines: Zeilen eines Absatzes bzw. Listenpunkts; Zeilenumbruch bei zwei
// Leerzeichen oder \ am Ende, sonst Leerzeichen.
func mdLines(ls []string) string {
	var b strings.Builder
	for i, l := range ls {
		brk := strings.HasSuffix(l, "  ") || strings.HasSuffix(strings.TrimRight(l, " "), `\`)
		l = strings.TrimSuffix(strings.TrimSpace(l), `\`)
		b.WriteString(mdInlineHTML(strings.TrimSpace(l)))
		if i < len(ls)-1 {
			if brk {
				b.WriteString("<br>")
			} else {
				b.WriteString(" ")
			}
		}
	}
	return b.String()
}

// mdInlineHTML: maskieren, dann **fett** und *kursiv*.
func mdInlineHTML(s string) string {
	esc := html.EscapeString(s)
	return mdInline.ReplaceAllStringFunc(esc, func(m string) string {
		p := mdInline.FindStringSubmatch(m)
		if p[1] != "" {
			return "<strong>" + p[1] + "</strong>"
		}
		return "<em>" + p[2] + "</em>"
	})
}

// mdTableHTML: Tabelle; eine Linie |---| trennt den Kopf und legt die Ausrichtung fest.
func mdTableHTML(rows []string) string {
	cells := func(r string) []string {
		r = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(r), "|"), "|")
		cs := strings.Split(r, "|")
		for i := range cs {
			cs[i] = strings.TrimSpace(cs[i])
		}
		return cs
	}
	isRule := func(cs []string) bool {
		for _, c := range cs {
			if !mdRule.MatchString(c) {
				return false
			}
		}
		return len(cs) > 0
	}
	var head []string
	var body [][]string
	var align []string
	for _, r := range rows {
		cs := cells(r)
		if isRule(cs) {
			align = make([]string, len(cs))
			for i, c := range cs {
				switch {
				case strings.HasPrefix(c, ":") && strings.HasSuffix(c, ":"):
					align[i] = "center"
				case strings.HasSuffix(c, ":"):
					align[i] = "right"
				default:
					align[i] = "left"
				}
			}
			if head == nil && len(body) == 1 {
				head, body = body[0], nil
			}
			continue
		}
		body = append(body, cs)
	}
	style := func(i int) string {
		if i < len(align) && align[i] != "left" {
			return ` style="text-align:` + align[i] + `"`
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(`<table class="md-table">`)
	if head != nil {
		b.WriteString("<thead><tr>")
		for i, c := range head {
			b.WriteString("<th" + style(i) + ">" + mdInlineHTML(c) + "</th>")
		}
		b.WriteString("</tr></thead>")
	}
	b.WriteString("<tbody>")
	for _, r := range body {
		b.WriteString("<tr>")
		for i, c := range r {
			b.WriteString("<td" + style(i) + ">" + mdInlineHTML(c) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>\n")
	return b.String()
}
