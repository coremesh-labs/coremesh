package database

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/coremesh-labs/coremesh/pkg/sdk"
)

// WriteScope ist das, was ein Plugin zur Laufzeit ändern darf: nur eigene
// Tabellen. Fremde Daten erreicht es über Actions anderer Plugins, nie über SQL.
type WriteScope struct {
	Plugin string // Name des Plugins (für Meldungen)
	Prefix string // Tabellenpräfix, z. B. "contract_billing__" (sdk.TablePrefix)
	Schema string // eigenes DB-Schema bei Schema-Isolation, z. B. "mod_contract_billing"
	// SchemaIsolation: Tabellen liegen im eigenen Schema (PostgreSQL); dort
	// sind auch Namen ohne Präfix eigene Tabellen (search_path = eigenes Schema).
	SchemaIsolation bool
	Write           bool // Freigabe access: write – sonst gar keine Änderungen
}

// CheckStatement prüft eine SQL-Anweisung eines Plugins vor der Ausführung:
//
//   - genau eine Anweisung (kein ";" mit weiterer Anweisung),
//   - nur SELECT, WITH, VALUES, INSERT, UPDATE, DELETE, REPLACE – kein DDL
//     (das macht dbschema), keine Sitzungs- und Transaktionssteuerung (SET,
//     PRAGMA, ATTACH, BEGIN, COMMIT …), kein SELECT … INTO,
//   - jedes Ziel von INSERT, UPDATE, DELETE, REPLACE und MERGE – auch in
//     CTEs und Unterabfragen – ist eine eigene Tabelle.
//
// Die Prüfung arbeitet auf Tokens (Strings, Kommentare und Bezeichner in
// Anführungszeichen werden übersprungen), nicht auf einem vollständigen
// SQL-Parser. Eine harte Grenze auch gegen Funktionen mit Seiteneffekten
// (PostgreSQL) ziehen erst eigene DB-Rollen je Plugin.
func CheckStatement(query string, s WriteScope) error {
	toks, err := tokenize(query)
	if err != nil {
		return deny(s, "%v", err)
	}
	// Höchstens ein abschließendes ";".
	for i, t := range toks {
		if t.kind == tokPunct && t.text == ";" && i != len(toks)-1 {
			return deny(s, "nur eine Anweisung je Aufruf")
		}
	}
	if n := len(toks); n > 0 && toks[n-1].text == ";" {
		toks = toks[:n-1]
	}
	if len(toks) == 0 {
		return deny(s, "leere Anweisung")
	}
	first := toks[0].upper()
	switch first {
	case "SELECT", "WITH", "VALUES", "INSERT", "UPDATE", "DELETE", "REPLACE":
	default:
		return deny(s, "%s ist zur Laufzeit nicht erlaubt (nur SELECT, INSERT, UPDATE, DELETE; Schema über DBSchema.Init)", first)
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind != tokWord {
			continue
		}
		prev, next := word(toks, i-1), word(toks, i+1)
		var target int // Index des Tabellennamens, 0 = keiner
		switch t.upper() {
		case "INSERT", "MERGE":
			j := i + 1
			if word(toks, j) == "OR" { // SQLite: INSERT OR REPLACE INTO
				j += 2
			}
			if word(toks, j) != "INTO" {
				return deny(s, "%s ohne INTO", t.upper())
			}
			target = j + 1
		case "REPLACE":
			if next != "INTO" { // replace(...) als Funktion oder INSERT OR REPLACE
				continue
			}
			if prev == "OR" {
				continue // Teil von INSERT OR REPLACE INTO (schon geprüft)
			}
			target = i + 2
		case "UPDATE":
			if prev == "FOR" || prev == "DO" || prev == "KEY" || prev == "ON" { // FOR UPDATE, DO UPDATE, ON UPDATE
				continue
			}
			target = i + 1
			if word(toks, target) == "OR" { // SQLite: UPDATE OR IGNORE
				target += 2
			}
		case "DELETE":
			if prev == "ON" { // ON DELETE (nur in DDL)
				continue
			}
			if next != "FROM" {
				return deny(s, "DELETE ohne FROM")
			}
			target = i + 2
		case "INTO":
			// INTO gehört zu INSERT/REPLACE/MERGE (oben übersprungen); sonst SELECT … INTO.
			if p := word(toks, i-1); p != "INSERT" && p != "REPLACE" && p != "MERGE" && word(toks, i-3) != "INSERT" {
				return deny(s, "SELECT … INTO legt eine Tabelle an und ist nicht erlaubt")
			}
			continue
		default:
			continue
		}
		if word(toks, target) == "ONLY" { // PostgreSQL: UPDATE ONLY t
			target++
		}
		name, err := tableName(toks, target)
		if err != nil {
			return deny(s, "%s: %v", t.upper(), err)
		}
		if !s.Write {
			return deny(s, "nur lesender Zugriff (%s %s)", t.upper(), name.display)
		}
		if !s.owns(name) {
			return deny(s, "%s %s: ändern darf das Plugin nur eigene Tabellen (%s*)", t.upper(), name.display, s.Prefix)
		}
	}
	return nil
}

func deny(s WriteScope, format string, args ...any) error {
	return fmt.Errorf("%w: Plugin %s: %s", sdk.ErrPermissionDenied, s.Plugin, fmt.Sprintf(format, args...))
}

type qualifiedName struct {
	schema, table string // unquotiert klein geschrieben, quotiert wörtlich
	display       string
}

func (s WriteScope) owns(n qualifiedName) bool {
	switch n.schema {
	case "":
		return strings.HasPrefix(n.table, s.Prefix) || s.SchemaIsolation
	case "main", "public":
		return !s.SchemaIsolation && strings.HasPrefix(n.table, s.Prefix)
	default:
		return s.SchemaIsolation && n.schema == s.Schema
	}
}

// tableName liest [schema.]tabelle ab toks[i].
func tableName(toks []token, i int) (qualifiedName, error) {
	part := func(j int) (string, bool) {
		if j >= len(toks) {
			return "", false
		}
		switch t := toks[j]; t.kind {
		case tokWord:
			return strings.ToLower(t.text), true
		case tokQuoted:
			return t.text, true
		}
		return "", false
	}
	a, ok := part(i)
	if !ok {
		return qualifiedName{}, fmt.Errorf("Tabellenname fehlt")
	}
	if i+2 < len(toks) && toks[i+1].kind == tokPunct && toks[i+1].text == "." {
		b, ok := part(i + 2)
		if !ok {
			return qualifiedName{}, fmt.Errorf("Tabellenname nach %s. fehlt", a)
		}
		return qualifiedName{schema: a, table: b, display: a + "." + b}, nil
	}
	return qualifiedName{table: a, display: a}, nil
}

// --- Tokenizer ------------------------------------------------------------------------

type tokKind int

const (
	tokWord   tokKind = iota // Schlüsselwort oder unquotierter Bezeichner
	tokQuoted                // "…", `…`, […]
	tokString                // '…', E'…', $tag$…$tag$
	tokNumber
	tokPunct
	tokParam // ?, $1
)

type token struct {
	kind tokKind
	text string
}

func (t token) upper() string { return strings.ToUpper(t.text) }

// word liefert toks[i] groß geschrieben, wenn es ein Wort ist, sonst "".
func word(toks []token, i int) string {
	if i < 0 || i >= len(toks) || toks[i].kind != tokWord {
		return ""
	}
	return toks[i].upper()
}

func tokenize(q string) ([]token, error) {
	var out []token
	r := []rune(q)
	n := len(r)
	for i := 0; i < n; {
		c := r[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '-' && i+1 < n && r[i+1] == '-':
			for i < n && r[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && r[i+1] == '*':
			depth, j := 1, i+2
			for j < n && depth > 0 {
				switch {
				case r[j] == '/' && j+1 < n && r[j+1] == '*':
					depth, j = depth+1, j+2
				case r[j] == '*' && j+1 < n && r[j+1] == '/':
					depth, j = depth-1, j+2
				default:
					j++
				}
			}
			if depth > 0 {
				return nil, fmt.Errorf("Kommentar nicht geschlossen")
			}
			i = j
		case c == '\'':
			j, err := closeQuote(r, i+1, '\'', false)
			if err != nil {
				return nil, err
			}
			out, i = append(out, token{tokString, string(r[i:j])}), j
		case (c == 'E' || c == 'e') && i+1 < n && r[i+1] == '\'':
			j, err := closeQuote(r, i+2, '\'', true)
			if err != nil {
				return nil, err
			}
			out, i = append(out, token{tokString, string(r[i:j])}), j
		case c == '"' || c == '`':
			j, err := closeQuote(r, i+1, c, false)
			if err != nil {
				return nil, err
			}
			ident := strings.ReplaceAll(string(r[i+1:j-1]), string([]rune{c, c}), string(c))
			out, i = append(out, token{tokQuoted, ident}), j
		case c == '[':
			j := i + 1
			for j < n && r[j] != ']' {
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("Bezeichner [ nicht geschlossen")
			}
			out, i = append(out, token{tokQuoted, string(r[i+1 : j])}), j+1
		case c == '$':
			// $1 (Platzhalter) oder $tag$…$tag$ (PostgreSQL)
			j := i + 1
			for j < n && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
				j++
			}
			if j < n && r[j] == '$' {
				tag := r[i : j+1]
				stop := -1
				for k := j + 1; k+len(tag) <= n; k++ {
					if string(r[k:k+len(tag)]) == string(tag) {
						stop = k + len(tag)
						break
					}
				}
				if stop < 0 {
					return nil, fmt.Errorf("Zeichenkette %s nicht geschlossen", string(tag))
				}
				out, i = append(out, token{tokString, string(r[i:stop])}), stop
				continue
			}
			out, i = append(out, token{tokParam, string(r[i:j])}), j
		case c == '?':
			out, i = append(out, token{tokParam, "?"}), i+1
		case unicode.IsLetter(c) || c == '_':
			j := i + 1
			for j < n && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_' || r[j] == '$') {
				j++
			}
			out, i = append(out, token{tokWord, string(r[i:j])}), j
		case unicode.IsDigit(c):
			j := i + 1
			for j < n && (unicode.IsDigit(r[j]) || r[j] == '.' || r[j] == 'e' || r[j] == 'E') {
				j++
			}
			out, i = append(out, token{tokNumber, string(r[i:j])}), j
		default:
			out, i = append(out, token{tokPunct, string(c)}), i+1
		}
	}
	return out, nil
}

// closeQuote sucht das Ende eines Literals ab start; verdoppelte Quotes bleiben
// darin, mit backslash (E'…') auch \'. Liefert den Index nach dem Ende.
func closeQuote(r []rune, start int, q rune, backslash bool) (int, error) {
	for j := start; j < len(r); j++ {
		switch {
		case backslash && r[j] == '\\':
			j++
		case r[j] == q:
			if j+1 < len(r) && r[j+1] == q {
				j++
				continue
			}
			return j + 1, nil
		}
	}
	return 0, fmt.Errorf("%c nicht geschlossen", q)
}
