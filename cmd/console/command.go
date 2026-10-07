package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	consolev1 "github.com/coremesh-labs/coremesh/pkg/consoleapi/console/v1"
)

// Konsolenbefehle der Module: console <modul>:<befehl> --param=wert …
//
// Das Console-Plugin löst den Befehl über den Catalog zu Object.Action auf und
// prüft die Parameter. Die CLI liest vorher die Befehlsdefinition, um
// Datei-Parameter (CommandParam.File) lokal einzulesen: *.json wird geparst,
// *.csv zu Zeilen (Kopfzeile = Spaltennamen, Trenner , oder ;), sonst Text.

// globalFlags sind Optionen der CLI selbst; alle anderen --name=wert hinter einem
// Befehl sind Parameter des Befehls.
var globalFlags = map[string]bool{"addr": true, "tls-ca": true, "user": true, "out": true, "timeout": true}

// commandArgs trennt die Argumente hinter <modul>:<befehl>: CLI-Optionen bleiben
// für flag, Befehlsparameter (--name=wert, --name wert, --schalter) gehen in p.
func commandArgs(args []string, p params) []string {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if globalFlags[name] {
			rest = append(rest, a)
			if !hasValue && i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		if !hasValue {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				value = args[i]
			} else {
				value = "true"
			}
		}
		_ = p.Set(name + "=" + value)
	}
	return rest
}

type commandParam struct {
	Name, Description string
	Required, File    bool
}

type commandDef struct {
	Name, Object, Action, Description string
	Params                            []commandParam
}

func runCommand(ctx context.Context, c consolev1.ConsoleServiceClient, o options) error {
	module, name, _ := strings.Cut(o.command, ":")
	def, err := lookupCommand(ctx, c, module, name)
	if err != nil {
		return err
	}
	if def != nil {
		for _, p := range def.Params {
			path, ok := o.params[p.Name].(string)
			if !p.File || !ok {
				continue
			}
			v, err := readParamFile(path)
			if err != nil {
				return fmt.Errorf("--%s: %w", p.Name, err)
			}
			o.params[p.Name] = v
		}
	}
	params, err := structpb.NewStruct(o.params)
	if err != nil {
		return err
	}
	resp, err := c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: o.command, Parameters: params})
	if err != nil {
		return err
	}
	if resp.Message != "" {
		fmt.Fprintln(os.Stderr, resp.Message)
	}
	body, err := json.MarshalIndent(resp.Payload.AsInterface(), "", "  ")
	if err != nil {
		return err
	}
	return output(o.out, append(body, '\n'), "Antwort")
}

// lookupCommand liest die Befehle des Moduls (Catalog.GetModule). nil ohne
// Fehler: Hilfe bzw. unbekannter Befehl – das beantwortet das Console-Plugin.
func lookupCommand(ctx context.Context, c consolev1.ConsoleServiceClient, module, name string) (*commandDef, error) {
	p, _ := structpb.NewStruct(map[string]any{"module": module})
	resp, err := c.Execute(ctx, &consolev1.ExecuteRequest{TargetObject: "Catalog", TargetAction: "GetModule", Parameters: p})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp.Payload.AsInterface())
	var mi struct {
		Commands []commandDef `json:"commands"`
	}
	if err := json.Unmarshal(raw, &mi); err != nil {
		return nil, err
	}
	for i := range mi.Commands {
		if mi.Commands[i].Name == name {
			return &mi.Commands[i], nil
		}
	}
	return nil, nil
}

// readParamFile liest eine lokale Datei für einen Datei-Parameter.
func readParamFile(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return v, nil
	case ".csv":
		return readCSV(b)
	}
	return string(b), nil
}

// readCSV: erste Zeile = Spaltennamen; Trenner ; wenn die Kopfzeile ihn enthält, sonst ,.
func readCSV(b []byte) ([]any, error) {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) // BOM aus Excel
	r := csv.NewReader(bytes.NewReader(b))
	if head, _, _ := bytes.Cut(b, []byte("\n")); bytes.Contains(head, []byte(";")) {
		r.Comma = ';'
	}
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []any{}, nil
	}
	out := make([]any, 0, len(rows)-1)
	for _, row := range rows[1:] {
		m := map[string]any{}
		for i, col := range rows[0] {
			if i < len(row) {
				m[strings.TrimSpace(col)] = strings.TrimSpace(row[i])
			}
		}
		out = append(out, m)
	}
	return out, nil
}
