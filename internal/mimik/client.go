package mimik

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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

	// Denken schaltet die Denkspur des Modells ein.
	//
	// Gemessen am 14.09.2026 an deepseek-v4.1-flash mit dem echten Prompt:
	// MIT Denkspur 2.019 Ausgabetoken im Schnitt, davon rund 1.800 reines
	// Nachdenken - OHNE 227. Die Qualitaet war in sechs Faellen nicht
	// schlechter: beide 6/6 im Abstandsfenster, beide 24/24 in der Form, Naehe
	// 0.06 gegen 0.07. Ausgabetoken sind die teure Seite der Rechnung; das
	// Denken kostete also rund neunzig Prozent davon fuer nichts.
	//
	// Der Schalter bleibt, weil "nicht schlechter in sechs Faellen" kein Beweis
	// ist: MIMIK_DENKEN=1 holt die Denkspur zurueck.
	Denken bool

	// MaxAusgabe deckelt die Ausgabe. 0 heisst: kein Deckel.
	//
	// Vorsicht, an der Messung gelernt: Ein Deckel WIRKT auf die Denkspur. Mit
	// eingeschaltetem Denken und max_tokens=600 kamen 600 Token Nachdenken und
	// KEIN Inhalt zurueck - voll bezahlt, Runde kaputt. Deshalb gilt der Deckel
	// nur, wenn das Denken aus ist.
	MaxAusgabe int

	// SitzungAufrufe: nach so vielen Aufrufen dreht sich {sitzung}.
	SitzungAufrufe int

	sperre   sync.Mutex
	sitzung  string
	seitDreh int
}

func NeuAusUmgebung() *Client {
	c := &Client{
		BaseURL:  getenv("MIMIK_BASE_URL", "https://api.deepseek.com"),
		APIKey:   os.Getenv("MIMIK_API_KEY"),
		Model:    getenv("MIMIK_MODEL", "deepseek-v4-flash"),
		Headers:  ParseHeaders(os.Getenv("MIMIK_HEADERS")),
		JSONMode: os.Getenv("MIMIK_JSON_MODE") != "0",
		Denken:   os.Getenv("MIMIK_DENKEN") == "1",

		HTTP: &http.Client{Timeout: 300 * time.Second},
	}
	// Vier Karten a MaxKarte, ein Fakt, eine Sperre - aus den eigenen Grenzen
	// gerechnet, nicht geraten. Wer die Denkspur einschaltet, bekommt keinen
	// Deckel: Der wuerde dort das Denken abschneiden statt die Ausgabe.
	c.MaxAusgabe = zahl("MIMIK_MAX_AUSGABE", 1200)
	if c.Denken {
		c.MaxAusgabe = 0
	}
	c.SitzungAufrufe = zahl("MIMIK_SITZUNG_AUFRUFE", 50)
	return c
}

func zahl(k string, standard int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v >= 0 {
		return v
	}
	return standard
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
	// Was der Anbieter ueber den Aufruf zurueckmeldet. Ohne diese Zahlen ist
	// jede Kostenaussage geraten - die Protokollzeile zaehlt Zeichen, und ob
	// ein Zeichen ein Token ist oder ein Drittel, entscheidet der Tokenisierer.
	// DeepSeek meldet die Cachefelder flach, OpenAI-kompatible Weiterleitungen
	// unter prompt_tokens_details; beides lesen, fehlende Felder bleiben 0.
	Usage struct {
		PromptTokens          int `json:"prompt_tokens"`
		CompletionTokens      int `json:"completion_tokens"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
		PromptTokensDetails   struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

// Verbrauch ist die Rechnung eines Aufrufs.
type Verbrauch struct {
	Zweck        string // faelschungen | review | botantwort
	RundeID      string
	Eingabe      int
	Ausgabe      int
	Denkspur     int
	CacheTreffer int
	Zeichen      int
	Sekunden     float64
}

type kennungSchluessel struct{}

// MitKennung haengt Zweck und Runde an den Kontext.
//
// Ueber den Kontext und nicht als Parameter, weil der Klient nichts von Runden
// und Matches wissen soll - er spricht einen Endpunkt an, sonst nichts. Den
// Kontext gibt ihm ohnehin jeder Aufrufer mit.
func MitKennung(ctx context.Context, zweck, rundeID string) context.Context {
	return context.WithValue(ctx, kennungSchluessel{}, Verbrauch{Zweck: zweck, RundeID: rundeID})
}

func kennung(ctx context.Context) Verbrauch {
	if v, ok := ctx.Value(kennungSchluessel{}).(Verbrauch); ok {
		return v
	}
	return Verbrauch{Zweck: "unbekannt"}
}

// Chat schickt einen Aufruf und gibt den rohen Inhalt samt Rechnung zurück.
func (c *Client) Chat(ctx context.Context, system, user string, temperatur float64) (string, Verbrauch, error) {
	v := kennung(ctx)
	v.Zeichen = len(system) + len(user)
	if c.APIKey == "" {
		return "", v, fmt.Errorf("MIMIK_API_KEY ist nicht gesetzt")
	}
	payload := map[string]any{
		"model":       c.Model,
		"temperature": temperatur,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	if !c.Denken {
		// Beide Schreibweisen: reasoning_effort ist die OpenAI-kompatible,
		// thinking die von Anthropic und einigen Weiterleitungen. Ein Endpunkt,
		// der eine davon nicht kennt, ignoriert sie.
		payload["reasoning_effort"] = "none"
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	if c.MaxAusgabe > 0 {
		payload["max_tokens"] = c.MaxAusgabe
	}
	if c.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", v, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	sitzung := c.sitzungskennung()
	for k, wert := range c.Headers {
		req.Header.Set(k, ersetzeKennungen(wert, sitzung))
	}

	begonnen := time.Now()
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", v, fmt.Errorf("kein kontakt zu %s: %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	roh, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", v, fmt.Errorf("modell antwortet %d: %s", res.StatusCode, kurz(string(roh)))
	}
	var a chatAntwort
	if err := json.Unmarshal(roh, &a); err != nil || len(a.Choices) == 0 {
		return "", v, fmt.Errorf("antwort unlesbar: %s", kurz(string(roh)))
	}
	inhalt := a.Choices[0].Message.Content
	u := a.Usage
	v.Eingabe, v.Ausgabe = u.PromptTokens, u.CompletionTokens
	v.Denkspur = u.CompletionTokensDetails.ReasoningTokens
	v.CacheTreffer = u.PromptCacheHitTokens
	if v.CacheTreffer == 0 {
		v.CacheTreffer = u.PromptTokensDetails.CachedTokens
	}
	v.Sekunden = time.Since(begonnen).Seconds()

	// Zeichen UND Token, nebeneinander: Das Verhaeltnis ist die einzige
	// Moeglichkeit, eine Schaetzung zu kalibrieren - und wenn die Token
	// wachsen, waehrend die Zeichen gleich bleiben, sammelt der Anbieter
	// Verlauf an. Genau das ist der Kontextueberlauf, im Anflug und in Zahlen.
	log.Printf("modell: %s zeichen=%d ein=%d (cache %d) aus=%d (denk %d) %.1fs",
		v.Zweck, v.Zeichen, v.Eingabe, v.CacheTreffer, v.Ausgabe, v.Denkspur, v.Sekunden)
	// Ein Token ist nie kuerzer als ein Zeichen. Mehr Token als Zeichen kann
	// deshalb nur fremder Kontext sein.
	if v.Eingabe > v.Zeichen && v.Zeichen > 0 {
		log.Printf("WARNUNG: %d Eingabetoken auf %d Zeichen - der Anbieter haelt Verlauf. "+
			"MIMIK_SITZUNG_AUFRUFE=1 setzen.", v.Eingabe, v.Zeichen)
	}
	return inhalt, v, nil
}

// sitzungskennung liefert den Wert fuer {sitzung} und dreht ihn in Bloecken.
//
// Gemessen am 14.09.2026 an opencode.ai/zen: Der Anbieter gewaehrt seinen
// Prefix-Cache NUR innerhalb derselben Sitzung - mit frischer Kennung 0 von
// 1.953 Eingabetoken aus dem Cache, mit fester 1.664 von 1.920, also 87
// Prozent. Fuenf Aufrufe hintereinander in einer Sitzung liessen prompt_tokens
// flach bei ~1.920: Der Anbieter sammelt KEINEN Verlauf an, die Sitzung ist
// allein ein Cacheschluessel. Das ist die Messung, die {zufall} bei jedem
// Aufruf ueberfluessig macht - und der Block ist die Vorsicht, falls sich das
// je aendert: Er begrenzt den Schaden auf seine Laenge, und die Warnung oben
// faellt vorher auf.
func (c *Client) sitzungskennung() string {
	c.sperre.Lock()
	defer c.sperre.Unlock()
	if c.sitzung == "" || (c.SitzungAufrufe > 0 && c.seitDreh >= c.SitzungAufrufe) {
		c.sitzung = zufallshex()
		c.seitDreh = 0
	}
	c.seitDreh++
	return c.sitzung
}

// ersetzeKennungen tauscht {sitzung} und {zufall} in einem Kopfzeilenwert.
//
// {zufall} ist bei jedem Aufruf neu, {sitzung} bleibt fuer einen Block von
// Aufrufen gleich. Fuer OpenCode ("x-opencode-session") gehoert {sitzung}
// hinein: Nur innerhalb einer Sitzung gewaehrt der Anbieter seinen
// Prefix-Cache, und 87 Prozent der Eingabe sind bei jedem Aufruf derselbe
// Systemprompt.
func ersetzeKennungen(v, sitzung string) string {
	if strings.Contains(v, "{sitzung}") {
		v = strings.ReplaceAll(v, "{sitzung}", sitzung)
	}
	if strings.Contains(v, "{zufall}") {
		v = strings.ReplaceAll(v, "{zufall}", zufallshex())
	}
	return v
}

func zufallshex() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
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
