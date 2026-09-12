package mimik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client spricht einen OpenAI-kompatiblen Endpunkt an – OpenCode, DeepSeek
// direkt oder ein lokaler Server. Dieselben Umgebungsvariablen wie harness.py.
type Client struct {
	BaseURL  string
	APIKey   string
	Model    string
	Headers  map[string]string
	JSONMode bool
	HTTP     *http.Client
}

func NeuAusUmgebung() *Client {
	c := &Client{
		BaseURL:  getenv("MIMIK_BASE_URL", "https://api.deepseek.com"),
		APIKey:   os.Getenv("MIMIK_API_KEY"),
		Model:    getenv("MIMIK_MODEL", "deepseek-v4-flash"),
		Headers:  ParseHeaders(os.Getenv("MIMIK_HEADERS")),
		JSONMode: os.Getenv("MIMIK_JSON_MODE") != "0",

		HTTP: &http.Client{Timeout: 300 * time.Second},
	}
	return c
}

func getenv(k, standard string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return standard
}

// ParseHeaders liest "Name: Wert" je Zeile – dasselbe Format wie parseHeaders()
// in state/settings.ts des DungeonMaster-Projekts.
func ParseHeaders(raw string) map[string]string {
	out := map[string]string{}
	for _, zeile := range strings.Split(raw, "\n") {
		i := strings.Index(zeile, ":")
		if i <= 0 {
			continue
		}
		name := strings.TrimSpace(zeile[:i])
		wert := strings.TrimSpace(zeile[i+1:])
		if name != "" && wert != "" {
			out[name] = wert
		}
	}
	return out
}

type chatAntwort struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat schickt einen Aufruf und gibt den rohen Inhalt zurück.
func (c *Client) Chat(ctx context.Context, system, user string, temperatur float64) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("MIMIK_API_KEY ist nicht gesetzt")
	}
	payload := map[string]any{
		"model":       c.Model,
		"temperature": temperatur,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	if c.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("kein kontakt zu %s: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	roh, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("modell antwortet %d: %s", res.StatusCode, kurz(string(roh)))
	}
	var a chatAntwort
	if err := json.Unmarshal(roh, &a); err != nil || len(a.Choices) == 0 {
		return "", fmt.Errorf("antwort unlesbar: %s", kurz(string(roh)))
	}
	return a.Choices[0].Message.Content, nil
}

// LiesJSON ist tolerant: es nimmt auch JSON, das in einem Codeblock oder in
// Fließtext steckt. Nicht jeder Endpunkt hält sich an response_format.
func LiesJSON(inhalt string, ziel any) error {
	s := strings.TrimSpace(inhalt)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	if err := json.Unmarshal([]byte(s), ziel); err == nil {
		return nil
	}
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		if err := json.Unmarshal([]byte(s[i:j+1]), ziel); err == nil {
			return nil
		}
	}
	return fmt.Errorf("keine lesbare JSON-antwort: %s", kurz(inhalt))
}

func kurz(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

func replaceAll(s, alt, neu string) string { return strings.ReplaceAll(s, alt, neu) }
