package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Einrichtungsdatei: Schritte, die über die Konsole laufen – dieselben Fachobjekte und
// Aktionen wie in der Weboberfläche, nie direkt in die Datenbank.
//
//	title: Testbestand …
//	steps:
//	  - command: ledger:setup-company        # Konsolenbefehl eines Moduls
//	    args: {company: "1000", chart: SKR04}
//	  - handle: BusinessPartner.create       # Aktion eines Objects
//	    unless: "1000|P1"                    # überspringen, wenn <Object>.get mit dieser ID gelingt
//	    data: {…}
//	    as: faust                            # Antwort merken: ${faust.record.id} in späteren Schritten
//
// Werte, die mit ./ oder ../ beginnen, sind Dateien relativ zur Einrichtungsdatei.
type seedFile struct {
	Title string     `yaml:"title"`
	Steps []seedStep `yaml:"steps"`
}

type seedStep struct {
	Title   string         `yaml:"title"`
	Command string         `yaml:"command"`
	Handle  string         `yaml:"handle"`
	Unless  string         `yaml:"unless"`
	ID      string         `yaml:"id"`
	Args    map[string]any `yaml:"args"`
	Data    any            `yaml:"data"`
	As      string         `yaml:"as"`
}

var varRe = regexp.MustCompile(`\$\{([A-Za-z_][\w-]*)((?:\.[\w-]+)*)\}`)

// expand ersetzt ${name.feld…} in allen Texten durch Werte gemerkter Antworten.
func expand(v any, vars map[string]any) (any, error) {
	switch x := v.(type) {
	case string:
		var err error
		out := varRe.ReplaceAllStringFunc(x, func(m string) string {
			g := varRe.FindStringSubmatch(m)
			cur, ok := vars[g[1]]
			for _, k := range strings.Split(strings.TrimPrefix(g[2], "."), ".") {
				if !ok || k == "" {
					break
				}
				mm, isMap := cur.(map[string]any)
				if !isMap {
					ok = false
					break
				}
				cur, ok = mm[k]
			}
			if !ok {
				err = fmt.Errorf("%s ist nicht bekannt", m)
				return m
			}
			return fmt.Sprint(cur)
		})
		return out, err
	case map[string]any:
		r := make(map[string]any, len(x))
		for k, e := range x {
			n, err := expand(e, vars)
			if err != nil {
				return nil, err
			}
			r[k] = n
		}
		return r, nil
	case []any:
		r := make([]any, len(x))
		for i, e := range x {
			n, err := expand(e, vars)
			if err != nil {
				return nil, err
			}
			r[i] = n
		}
		return r, nil
	}
	return v, nil
}

func seed(in *installation, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var sf seedFile
	if err := yaml.Unmarshal(b, &sf); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	base, err := filepath.Abs(filepath.Dir(file)) // die Konsole läuft im Installationsverzeichnis
	if err != nil {
		return err
	}
	logf("Einrichtung: %s (%d Schritte)", sf.Title, len(sf.Steps))
	vars := map[string]any{}
	for i, s := range sf.Steps {
		var err error
		if s.ID, s.Unless, s.Data, s.Args, err = expandStep(s, vars); err != nil {
			return fmt.Errorf("Schritt %d: %w", i+1, err)
		}
		args, label, err := s.argv(base)
		if err != nil {
			return fmt.Errorf("Schritt %d: %w", i+1, err)
		}
		if s.Unless != "" {
			obj, _, _ := strings.Cut(s.Handle, ".")
			if _, err := in.console("handle", obj+".get", "--id="+s.Unless); err == nil {
				continue
			}
		}
		out, err := in.console(args...)
		for try := 1; err != nil && try < 5 && strings.Contains(err.Error(), "database is locked"); try++ {
			time.Sleep(time.Duration(try) * time.Second) // Datenbank kurz belegt (z. B. Sicherung nach dem Start)
			out, err = in.console(args...)
		}
		if err != nil {
			return fmt.Errorf("Schritt %d (%s): %w", i+1, label, err)
		}
		if s.Title != "" {
			logf("  %s: %s", s.Title, message(out))
		}
		if s.As != "" {
			var r any
			if i := strings.Index(string(out), "{"); i >= 0 {
				json.Unmarshal(out[i:], &r)
			}
			vars[s.As] = r
		}
	}
	return nil
}

func expandStep(s seedStep, vars map[string]any) (id, unless string, data any, args map[string]any, err error) {
	var v any
	if v, err = expand(s.ID, vars); err != nil {
		return
	}
	id = v.(string)
	if v, err = expand(s.Unless, vars); err != nil {
		return
	}
	unless = v.(string)
	if data, err = expand(s.Data, vars); err != nil {
		return
	}
	if v, err = expand(map[string]any(s.Args), vars); err != nil {
		return
	}
	args = v.(map[string]any)
	return
}

func (s seedStep) argv(base string) ([]string, string, error) {
	var args []string
	label := s.Command
	switch {
	case s.Command != "" && s.Handle == "":
		args = []string{s.Command}
	case s.Handle != "" && s.Command == "":
		args, label = []string{"handle", s.Handle}, s.Handle
		if s.ID != "" {
			args = append(args, "--id="+s.ID)
		}
		if s.Data != nil {
			b, err := json.Marshal(s.Data)
			if err != nil {
				return nil, "", err
			}
			args = append(args, "--data="+string(b))
		}
	default:
		return nil, "", fmt.Errorf("genau eins von command und handle angeben")
	}
	keys := make([]string, 0, len(s.Args))
	for k := range s.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var v string
		switch x := s.Args[k].(type) {
		case string:
			v = x
			if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
				v = filepath.Join(base, v)
			}
		default:
			b, err := json.Marshal(x)
			if err != nil {
				return nil, "", err
			}
			v = string(b)
		}
		args = append(args, "--"+k+"="+v)
	}
	return args, label, nil
}

// message: Meldung einer Antwort (Feld message), sonst die erste Zeile.
func message(out []byte) string {
	s := strings.TrimSpace(string(out))
	if i := strings.Index(s, "{"); i >= 0 {
		var r struct {
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(s[i:]), &r) == nil && r.Message != "" {
			return r.Message
		}
	}
	if strings.HasPrefix(s, "{") {
		return "ok"
	}
	if l, _, _ := strings.Cut(s, "\n"); len(l) > 160 {
		return l[:160] + " …"
	} else {
		return l
	}
}
