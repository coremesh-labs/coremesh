// Package markdown: ein kleiner, sicherer Teil von Markdown als HTML – alles wird
// erst maskiert, eigenes HTML kommt so nie in die Ausgabe. Derselbe Teil wie bei
// den PDF-Vorlagen (mdtex im Plugin template):
//
//	# Titel, ## Abschnitt, ### Unterabschnitt   Überschriften
//	Leerzeile                                   neuer Absatz; zwei Leerzeichen oder \ am Zeilenende: Zeilenumbruch
//	**fett**, *kursiv*, [Text](https://…)
//	++unterstrichen++, ~~durchgestrichen~~      (seit 0.62.0; nur HTML – mdtex der PDF-Vorlagen zeigt sie als Text)
//	> Zitat                                     Zitat (zusammenhängende Zeilen)
//	---                                         Trennlinie (drei oder mehr -)
//	- Punkt  bzw.  * Punkt                      Aufzählung
//	1. Punkt                                    nummerierte Liste
//	a) Punkt                                    Liste a), b), …
//	| Spalte | Spalte |                         Tabelle; |---|--:| Linie unter dem Kopf,
//	                                            :-- links, --: rechts, :-: mittig
//
// Folgezeilen eines Listenpunkts (eingerückt oder ohne neues Zeichen) gehören zu ihm.
package markdown

import (
	"html"
	"regexp"
	"strings"
)

var (
	mdHeading = regexp.MustCompile(`^\s*(#{1,4})\s+(.*?)\s*#*\s*$`)
	mdBullet  = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)
	mdNumber  = regexp.MustCompile(`^\s*(\d+)[.)]\s+(.*)$`)
	mdAlpha   = regexp.MustCompile(`^\s*([a-z])\)\s+(.*)$`)
	mdInline  = regexp.MustCompile(`\*\*(.+?)\*\*|\*([^*\s][^*]*?)\*`)
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\(((?:https?://|mailto:)[^\s)]+)\)`)
	mdRule    = regexp.MustCompile(`^:?-+:?$`)
	mdExtra   = regexp.MustCompile(`\+\+(.+?)\+\+|~~(.+?)~~`)
	mdHR      = regexp.MustCompile(`^\s*(-\s*){3,}$|^\s*(\*\s*){3,}$`)
)

// Options: Darstellung.
type Options struct {
	// HeadingShift: # wird zu h(1+HeadingShift), z. B. 2 unter den Überschriften einer Seite.
	HeadingShift int
	// TableClass: CSS-Klasse der Tabellen (leer = Rahmenlinien als Stil, z. B. für Mails).
	TableClass string
	// Breaks (seit 0.62.1): jeder Zeilenwechsel im Absatz bleibt ein Umbruch (Mails,
	// Signaturen) – sonst nur bei zwei Leerzeichen oder \ am Zeilenende.
	Breaks bool
}

// HTML: Text als HTML (maskiert, nur die Strukturen oben).
func HTML(src string, o Options) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	var para []string
	list, item := "", []string(nil) // offene Liste: ul, ol, ola
	var items [][]string
	var table []string
	var quote []string
	flushPara := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + mdLines(para, o.Breaks) + "</p>\n")
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
			b.WriteString("<li>" + mdLines(it, o.Breaks) + "</li>")
		}
		b.WriteString("</" + tag + ">\n")
		list, items = "", nil
	}
	flushTable := func() {
		if len(table) > 0 {
			b.WriteString(mdTableHTML(table, o))
			table = nil
		}
	}
	flushQuote := func() {
		if len(quote) > 0 {
			b.WriteString(`<blockquote style="margin:.5em 0;padding-left:.8em;border-left:3px solid #ccc;color:#555">` + mdLines(quote, o.Breaks) + "</blockquote>\n")
			quote = nil
		}
	}
	flush := func() { flushPara(); flushList(); flushTable(); flushQuote() }
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trim := strings.TrimSpace(line)
		switch {
		case trim == "":
			flushPara()
			flushList()
			flushQuote()
			// Tabellenzeilen über Leerzeilen hinweg bleiben eine Tabelle
		case mdHR.MatchString(line):
			flush()
			b.WriteString("<hr>\n")
		case strings.HasPrefix(trim, ">"):
			flushPara()
			flushList()
			flushTable()
			quote = append(quote, strings.TrimPrefix(strings.TrimPrefix(trim, ">"), " "))
		case strings.HasPrefix(trim, "|"):
			flushPara()
			flushList()
			table = append(table, trim)
		case mdHeading.MatchString(line):
			flush()
			m := mdHeading.FindStringSubmatch(line)
			lvl := len(m[1]) + o.HeadingShift
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
	return b.String()
}

// mdLines: Zeilen eines Absatzes bzw. Listenpunkts; Zeilenumbruch bei zwei
// Leerzeichen oder \ am Ende, sonst Leerzeichen.
func mdLines(ls []string, breaks bool) string {
	var b strings.Builder
	for i, l := range ls {
		brk := breaks || strings.HasSuffix(l, "  ") || strings.HasSuffix(strings.TrimRight(l, " "), `\`)
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

// mdInlineHTML: maskieren, dann [Text](https://…), ++unterstrichen++, ~~durchgestrichen~~,
// **fett** und *kursiv*.
func mdInlineHTML(s string) string {
	esc := html.EscapeString(s)
	esc = mdLink.ReplaceAllString(esc, `<a href="$2">$1</a>`)
	esc = mdExtra.ReplaceAllStringFunc(esc, func(m string) string {
		p := mdExtra.FindStringSubmatch(m)
		if p[1] != "" {
			return "<u>" + p[1] + "</u>"
		}
		return "<s>" + p[2] + "</s>"
	})
	return mdInline.ReplaceAllStringFunc(esc, func(m string) string {
		p := mdInline.FindStringSubmatch(m)
		if p[1] != "" {
			return "<strong>" + p[1] + "</strong>"
		}
		return "<em>" + p[2] + "</em>"
	})
}

// mdTableHTML: Tabelle; eine Linie |---| trennt den Kopf und legt die Ausrichtung fest.
func mdTableHTML(rows []string, o Options) string {
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
				case strings.HasPrefix(c, ":"):
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
	// ohne Angabe wie in PDFs (mdtex): erste Spalte links, die übrigen rechts
	style := func(i int) string {
		a := ""
		if i < len(align) {
			a = align[i]
		}
		if a == "" && i > 0 {
			a = "right"
		}
		if a == "" || a == "left" {
			return ""
		}
		return ` style="text-align:` + a + `"`
	}
	cellStyle := func(i int) string {
		st := style(i)
		if o.TableClass != "" {
			return st
		}
		pad := "border:1px solid #ccc;padding:2px 6px;"
		if st == "" {
			return ` style="` + pad + `"`
		}
		return strings.Replace(st, `style="`, `style="`+pad, 1)
	}
	var b strings.Builder
	if o.TableClass != "" {
		b.WriteString(`<table class="` + html.EscapeString(o.TableClass) + `">`)
	} else {
		b.WriteString(`<table style="border-collapse:collapse">`)
	}
	if head != nil {
		b.WriteString("<thead><tr>")
		for i, c := range head {
			b.WriteString("<th" + cellStyle(i) + ">" + mdInlineHTML(c) + "</th>")
		}
		b.WriteString("</tr></thead>")
	}
	b.WriteString("<tbody>")
	for _, r := range body {
		b.WriteString("<tr>")
		for i, c := range r {
			b.WriteString("<td" + cellStyle(i) + ">" + mdInlineHTML(c) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>\n")
	return b.String()
}
