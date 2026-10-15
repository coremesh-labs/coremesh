package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coremesh-labs/coremesh/pkg/sdk/crud"
)

// providerRow ist ein KI-Dienst mit Schlüssel.
type providerRow struct {
	Code, Name, BaseURL, AuthMethod, AuthName, Model, KeyEnv, Headers, Fallback string
	FilePart                                                                    string // AUTO, IMAGE_URL, FILE
	Structured                                                                  bool
	Timeout                                                                     time.Duration
	key                                                                         string
}

// loadProvider: Dienst (code; leer = erster aktiver) mit Schlüssel aus der
// Umgebungsvariablen bzw. der eigenen Tabelle.
func (m *Module) loadProvider(ctx context.Context, code string) (*providerRow, error) {
	q := `SELECT code, name, base_url, auth_method, auth_name, model, api_key_env, extra_headers, plain_json, timeout_seconds, fallback_provider, file_part FROM ai__provider WHERE is_active = ?`
	args := []any{true}
	if code != "" {
		q += ` AND code = ?`
		args = append(args, code)
	}
	res, err := m.db.Query(ctx, q+` ORDER BY code`, args...)
	if err != nil {
		return nil, err
	}
	if len(res.Rows) == 0 {
		if code != "" {
			return nil, crud.Invalid("KI-Dienst %s gibt es nicht oder er ist inaktiv", code)
		}
		return nil, crud.Invalid("Kein aktiver KI-Dienst (KI → Einstellungen → KI-Dienste)")
	}
	r := res.Rows[0]
	p := &providerRow{Code: crud.Str(r[0]), Name: crud.Str(r[1]), BaseURL: strings.TrimRight(crud.Str(r[2]), "/"), AuthMethod: crud.Str(r[3]),
		AuthName: crud.Str(r[4]), Model: crud.Str(r[5]), KeyEnv: crud.Str(r[6]), Headers: crud.Str(r[7]), Structured: !crud.AsBool(r[8]),
		Timeout: time.Duration(toInt(r[9])) * time.Second, Fallback: crud.Str(r[10]), FilePart: firstOf(crud.Str(r[11]), filePartAuto)}
	if p.Timeout <= 0 {
		p.Timeout = 90 * time.Second
	}
	if p.KeyEnv != "" {
		p.key = m.getenv(p.KeyEnv)
	}
	if p.key == "" {
		p.key = m.text(ctx, `SELECT api_key FROM ai__provider_secret WHERE provider = ?`, p.Code)
	}
	if p.key == "" && p.AuthMethod != "NONE" {
		return nil, crud.Invalid("KI-Dienst %s: kein Schlüssel (gespeichert oder Umgebungsvariable %s)", p.Code, firstOf(p.KeyEnv, "–"))
	}
	return p, nil
}

// chatRequest: Nachrichten im Format von /chat/completions.
type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    *float64      `json:"temperature,omitempty"`
	ResponseFormat any           `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Text oder Teile (Text, Bild)
}

type chatAnswer struct {
	Text   string
	Raw    string
	Tokens int64
	Model  string
}

// retryWaits: Pausen vor den Wiederholungen bei Überlast (HTTP 429, 5xx).
var retryWaits = []time.Duration{3 * time.Second, 8 * time.Second}

// chat sendet die Anfrage an den Dienst; bei Überlast (429, 500, 502, 503, 504) bis zu
// zweimal erneut.
func (m *Module) chat(ctx context.Context, p *providerRow, req chatRequest) (*chatAnswer, error) {
	a, _, err := m.chatRetry(ctx, p, req)
	return a, err
}

// disturbed: Störung des Dienstes (Überlast, Serverfehler, nicht erreichbar) – kein Fehler der Anfrage.
func disturbed(status int, err error) bool {
	return err != nil && (status == 0 || status == 408 || status == 429 || status >= 500)
}

// chatFallback: wie chat; ist der Dienst gestört, springt sein Ausweich-Dienst ein
// (mit dessen Modell; Kette höchstens drei Dienste). Liefert den Dienst, der geantwortet hat.
func (m *Module) chatFallback(ctx context.Context, p *providerRow, req chatRequest) (*chatAnswer, *providerRow, []string, error) {
	var notes []string
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		seen[p.Code] = true
		a, status, err := m.chatRetry(ctx, p, req)
		if !disturbed(status, err) || p.Fallback == "" || seen[p.Fallback] {
			return a, p, notes, err
		}
		fb, ferr := m.loadProvider(ctx, p.Fallback)
		if ferr != nil {
			return a, p, notes, err
		}
		notes = append(notes, fmt.Sprintf("%s gestört (%v) – Ausweich-Dienst %s", p.Code, err, fb.Code))
		m.log.WarnContext(ctx, "KI-Dienst gestört, Ausweich-Dienst", "dienst", p.Code, "ausweich", fb.Code, "err", err.Error())
		req.Model = fb.Model
		p = fb
	}
	return nil, p, notes, fmt.Errorf("KI-Dienste gestört")
}

func (m *Module) chatRetry(ctx context.Context, p *providerRow, req chatRequest) (*chatAnswer, int, error) {
	for i := 0; ; i++ {
		a, status, err := m.chatOnce(ctx, p, req)
		retry := status == 429 || status == 500 || status == 502 || status == 503 || status == 504
		if err == nil || !retry || i >= len(retryWaits) {
			return a, status, err
		}
		select {
		case <-ctx.Done():
			return a, status, err
		case <-time.After(retryWaits[i]):
		}
	}
}

func (m *Module) chatOnce(ctx context.Context, p *providerRow, req chatRequest) (*chatAnswer, int, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	u := p.BaseURL + "/chat/completions"
	if p.AuthMethod == "QUERY" {
		u += "?" + url.Values{p.AuthName: {p.key}}.Encode()
	}
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	hr.Header.Set("Content-Type", "application/json")
	switch p.AuthMethod {
	case "BEARER":
		hr.Header.Set("Authorization", "Bearer "+p.key)
	case "HEADER":
		hr.Header.Set(p.AuthName, p.key)
	}
	hs, _ := parseHeaders(p.Headers)
	for k, v := range hs {
		hr.Header.Set(k, v)
	}
	resp, err := m.http.Do(hr)
	if err != nil {
		return nil, 0, fmt.Errorf("KI-Dienst %s nicht erreichbar: %v", p.Code, redact(err.Error(), p.key))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		return &chatAnswer{Raw: string(raw)}, resp.StatusCode, fmt.Errorf("KI-Dienst %s: HTTP %d – %s", p.Code, resp.StatusCode, redact(errorText(raw), p.key))
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Total int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return &chatAnswer{Raw: string(raw)}, resp.StatusCode, fmt.Errorf("KI-Dienst %s: Antwort ohne Ergebnis", p.Code)
	}
	a := &chatAnswer{Raw: string(raw), Tokens: out.Usage.Total, Model: out.Model}
	switch c := out.Choices[0].Message.Content.(type) {
	case string:
		a.Text = c
	case []any: // Teile
		var b strings.Builder
		for _, part := range c {
			if pm, ok := part.(map[string]any); ok {
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		a.Text = b.String()
	}
	return a, resp.StatusCode, nil
}

// errorText: Meldung aus einer Fehlerantwort (OpenAI- und Google-Format).
func errorText(raw []byte) string {
	var one struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &one) == nil && one.Error.Message != "" {
		return one.Error.Message
	}
	var list []struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &list) == nil && len(list) > 0 && list[0].Error.Message != "" {
		return list[0].Error.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300] + " …"
	}
	return s
}

// redact entfernt den Schlüssel aus Meldungen.
func redact(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "***")
}

func parseTemperature(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	if err != nil || f < 0 || f > 2 {
		return 0, crud.Invalid("Temperatur %q: Zahl von 0 bis 2", s)
	}
	return f, nil
}

// extractJSON: JSON aus der Antwort (auch in ```json … ``` oder mit Text davor).
func extractJSON(s string) (map[string]any, error) {
	t := strings.TrimSpace(s)
	if i := strings.Index(t, "```"); i >= 0 {
		t = t[i+3:]
		t = strings.TrimPrefix(t, "json")
		if j := strings.Index(t, "```"); j >= 0 {
			t = t[:j]
		}
	}
	if i, j := strings.Index(t, "{"), strings.LastIndex(t, "}"); i >= 0 && j > i {
		t = t[i : j+1]
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(t), &out); err != nil {
		return nil, fmt.Errorf("Antwort ist kein JSON-Objekt: %v", err)
	}
	return out, nil
}
