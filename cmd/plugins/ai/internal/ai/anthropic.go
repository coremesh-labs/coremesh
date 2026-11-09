package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
	"github.com/coremesh-labs/coremesh/pkg/sdk/metamodel"
)

// Schnittstelle eines KI-Dienstes (seit 0.5.0): OpenAI-kompatibel (/chat/completions)
// oder die Messages API von Anthropic (Claude). Die Anfrage entsteht immer im Format
// von /chat/completions; für Anthropic übersetzt anthropicOnce sie – so bleiben
// Wiederholung, Ausweich-Dienst und Protokoll für alle Dienste gleich.
const (
	apiOpenAI    = "OPENAI"
	apiAnthropic = "ANTHROPIC"
)

var apiKindOptions = []metamodel.Option{
	{Value: apiOpenAI, Label: "OpenAI-kompatibel (z. B. Gemini, OpenAI)"},
	{Value: apiAnthropic, Label: "Anthropic Messages API (Claude)"},
}

// anthropicMaxTokens: Obergrenze der Antwort (Kopf und viele Positionen passen hinein).
const anthropicMaxTokens = 16000

// anthropicOnce: eine Anfrage an die Messages API. Liefert wie chatOnce Antwort,
// HTTP-Status (0 = nicht erreichbar) und Fehler.
func (m *Module) anthropicOnce(ctx context.Context, p *providerRow, req chatRequest) (*chatAnswer, int, error) {
	params, err := anthropicParams(req)
	if err != nil {
		return nil, 0, err
	}
	opts := []option.RequestOption{
		// nur die Einstellung des Dienstes – keine ANTHROPIC_*-Umgebungsvariablen oder Profile des Servers
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(p.key), // Authorization: Bearer
		// Die SDK hängt v1/messages selbst an; …/v1 aus der Einstellung wird weggelassen.
		option.WithBaseURL(strings.TrimSuffix(p.BaseURL, "/v1") + "/"),
		option.WithHTTPClient(m.http),
		option.WithMaxRetries(0), // Wiederholung macht chatRetry
		option.WithRequestTimeout(p.Timeout),
	}
	hs, _ := parseHeaders(p.Headers)
	for k, v := range hs {
		opts = append(opts, option.WithHeader(k, v))
	}
	client := anthropic.NewClient(opts...)
	msg, err := client.Messages.New(ctx, params)
	// Lehnt der Dienst das JSON-Schema ab (HTTP 400), einmal ohne – die leere Struktur steht ohnehin im Auftrag.
	if status, _ := anthropicError(p, err); status == 400 && params.OutputConfig.Format.Schema != nil {
		m.log.WarnContext(ctx, "KI-Dienst lehnt JSON-Schema ab, ohne Schema wiederholt", "dienst", p.Code, "err", redact(err.Error(), p.key))
		params.OutputConfig = anthropic.OutputConfigParam{}
		msg, err = client.Messages.New(ctx, params)
	}
	if err != nil {
		status, e := anthropicError(p, err)
		return &chatAnswer{Raw: rawOf(err)}, status, e
	}
	a := &chatAnswer{Raw: msg.RawJSON(), Model: string(msg.Model), Tokens: msg.Usage.InputTokens + msg.Usage.OutputTokens}
	var b strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	a.Text = b.String()
	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return a, 200, fmt.Errorf("KI-Dienst %s: Anfrage abgelehnt (%s)", p.Code, firstOf(msg.StopDetails.Explanation, string(msg.StopDetails.Category), "ohne Begründung"))
	case anthropic.StopReasonMaxTokens:
		return a, 200, fmt.Errorf("KI-Dienst %s: Antwort abgeschnitten (mehr als %d Tokens)", p.Code, anthropicMaxTokens)
	}
	return a, 200, nil
}

// anthropicError: HTTP-Status und Meldung (ohne Schlüssel) eines Fehlers der SDK.
func anthropicError(p *providerRow, err error) (int, error) {
	if err == nil {
		return 200, nil
	}
	var apierr *anthropic.Error
	if errors.As(err, &apierr) {
		return apierr.StatusCode, fmt.Errorf("KI-Dienst %s: HTTP %d – %s", p.Code, apierr.StatusCode, redact(errorText([]byte(apierr.RawJSON())), p.key))
	}
	return 0, fmt.Errorf("KI-Dienst %s nicht erreichbar: %v", p.Code, redact(err.Error(), p.key))
}

func rawOf(err error) string {
	var apierr *anthropic.Error
	if errors.As(err, &apierr) {
		return apierr.RawJSON()
	}
	return ""
}

// anthropicParams übersetzt die Anfrage: system-Nachrichten in system, Teile
// (Text, image_url, file mit data:-URL) in Blöcke – PDF als document, Bilder
// als image, Dateien vor dem Text; response_format in output_config.format.
// Eine Temperatur entfällt (die aktuellen Modelle lehnen sie ab).
func anthropicParams(req chatRequest) (anthropic.MessageNewParams, error) {
	params := anthropic.MessageNewParams{Model: anthropic.Model(req.Model), MaxTokens: anthropicMaxTokens}
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok && strings.TrimSpace(s) != "" {
				params.System = append(params.System, anthropic.TextBlockParam{Text: s})
			}
			continue
		}
		var files, texts []anthropic.ContentBlockParamUnion
		switch c := msg.Content.(type) {
		case string:
			texts = append(texts, anthropic.NewTextBlock(c))
		case []any:
			for _, part := range c {
				pm, _ := part.(map[string]any)
				switch pm["type"] {
				case "text":
					texts = append(texts, anthropic.NewTextBlock(crud.Str(pm["text"])))
				case "image_url":
					u, _ := pm["image_url"].(map[string]any)
					blk, err := anthropicFile(crud.Str(u["url"]))
					if err != nil {
						return params, err
					}
					files = append(files, blk)
				case "file":
					f, _ := pm["file"].(map[string]any)
					blk, err := anthropicFile(crud.Str(f["file_data"]))
					if err != nil {
						return params, err
					}
					files = append(files, blk)
				default:
					return params, crud.Invalid("Nachrichtenteil %q für Anthropic nicht übersetzbar", pm["type"])
				}
			}
		}
		blocks := append(files, texts...)
		if msg.Role == "assistant" {
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(blocks...))
		} else {
			params.Messages = append(params.Messages, anthropic.NewUserMessage(blocks...))
		}
	}
	if rf, ok := req.ResponseFormat.(map[string]any); ok {
		if js, ok := rf["json_schema"].(map[string]any); ok {
			if s, ok := js["schema"].(map[string]any); ok {
				params.OutputConfig.Format = anthropic.JSONOutputFormatParam{Schema: strictSchema(s).(map[string]any)}
			}
		}
	}
	return params, nil
}

// anthropicFile: data:-URL als Block – PDF als document, Bilder als image.
func anthropicFile(dataURL string) (anthropic.ContentBlockParamUnion, error) {
	head, data, ok := strings.Cut(strings.TrimPrefix(dataURL, "data:"), ",")
	ctype, isB64 := strings.CutSuffix(head, ";base64")
	if !ok || !isB64 || !strings.HasPrefix(dataURL, "data:") {
		return anthropic.ContentBlockParamUnion{}, crud.Invalid("Datei für Anthropic: data:-URL mit Base64 erwartet")
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return anthropic.ContentBlockParamUnion{}, crud.Invalid("Datei für Anthropic: nicht Base64-kodiert")
	}
	switch {
	case ctype == "application/pdf":
		return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: data}), nil
	case slices.Contains([]string{"image/jpeg", "image/png", "image/gif", "image/webp"}, ctype):
		return anthropic.NewImageBlockBase64(ctype, data), nil
	}
	return anthropic.ContentBlockParamUnion{}, crud.Invalid("Dateiart %s kann Anthropic nicht lesen (PDF, JPEG, PNG, GIF, WebP)", ctype)
}

// strictSchema: JSON-Schema in der Form, die die strukturierte Ausgabe von
// Anthropic verlangt – jedes Objekt mit additionalProperties: false und allen
// Eigenschaften als Pflicht; damit leere Angaben möglich bleiben, Zahlen auch
// null und Auswahlwerte auch "" (wie in der leeren Struktur des Auftrags).
func strictSchema(v any) any {
	s, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := map[string]any{}
	for k, x := range s {
		out[k] = x
	}
	if props, ok := s["properties"].(map[string]any); ok {
		np := map[string]any{}
		keys := make([]string, 0, len(props))
		for k, x := range props {
			np[k] = strictSchema(x)
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out["properties"], out["required"], out["additionalProperties"] = np, keys, false
	}
	if items, ok := s["items"]; ok {
		out["items"] = strictSchema(items)
	}
	if enum, ok := s["enum"].([]string); ok && !slices.Contains(enum, "") {
		out["enum"] = append(slices.Clone(enum), "")
	}
	if s["type"] == "number" {
		delete(out, "type")
		out["anyOf"] = []any{map[string]any{"type": "number"}, map[string]any{"type": "null"}}
	}
	return out
}
