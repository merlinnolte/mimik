package mimik

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"mimik/internal/game"
	"mimik/internal/sicher"
)

// Grenzen für alles, was das Modell zurückschickt. Eine Antwort des Modells ist
// so wenig vertrauenswürdig wie eine Eingabe des Spielers: Sie entsteht aus
// Spielertext und landet ungefiltert im Protokoll des Betreibers und in der
// Anzeige der App.
const (
	MaxKarte = 400 // dieselbe Grenze wie game.MaxAntwort
	MaxFakt  = 300
	MaxThema = 60
)

// Dossier ist das Material, das MIMIK über einen Spieler bekommt.
type Dossier struct {
	Fakten        []string
	Gesperrt      []string
	AntiBeispiele []string
}

// Ergebnis eines Doppelgaenger-Aufrufs samt Diagnose.
type Ergebnis struct {
	Fakt         string
	Sperre       []string
	Faelschungen []game.Faelschung
	Versuche     int
	Befund       Befund
}

type antwortB struct {
	Fakt      string   `json:"fakt"`
	Sperre    []string `json:"sperre"`
	Antworten []struct {
		Anker string `json:"anker"`
		Text  string `json:"text"`
	} `json:"antworten"`
}

// Normalform glättet eine getippte Antwort. Der zweite Rückgabewert sagt, ob
// der regelbasierte Weg genommen wurde.
//
// Standardmäßig fragt sie das Modell NICHT. Gemessen am 12.09.2026 antwortet
// der Endpunkt nach 15 bis 190 Sekunden – darauf kann niemand warten, der
// gerade auf Absenden getippt hat, und im Versuch lieferte der regelbasierte
// Weg dasselbe Ergebnis. Mit MIMIK_NORMALFORM=modell lässt sich Prompt D
// zuschalten, sobald ein schnellerer Endpunkt zur Verfügung steht.
func (c *Client) Normalform(ctx context.Context, roh string) (string, bool) {
	if !c.NormalformPerModell {
		return ErsatzNormalform(roh), true
	}
	mat := Huelle(roh)
	inhalt, err := c.Chat(ctx, PromptNormalform, mat, 0.1)
	if err == nil {
		var z struct {
			Normalform string `json:"normalform"`
		}
		if LiesJSON(inhalt, &z) == nil {
			n := sicher.Text(z.Normalform, MaxKarte)
			if NormalformPlausibel(roh, n) {
				return n, false
			}
		}
	}
	return ErsatzNormalform(roh), true
}

var (
	reEmoji = regexp.MustCompile(`[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{FE0F}]`)
	reRaum  = regexp.MustCompile(`\s+`)
	reSatz  = regexp.MustCompile(`(?:^|[.!?]\s+)[a-zäöüß]`)
)

// entdoppeln fasst Wiederholungen von Satzzeichen zusammen ("!!!" wird "!").
// Go-Regexp kennt keine Rückverweise, deshalb von Hand.
func entdoppeln(s string) string {
	var b strings.Builder
	var vorher rune
	for _, r := range s {
		if r == vorher && strings.ContainsRune("!?.,", r) {
			continue
		}
		b.WriteRune(r)
		vorher = r
	}
	return b.String()
}

// ErsatzNormalform ist die modellfreie Rückfallebene: grob, aber nie blockierend.
func ErsatzNormalform(roh string) string {
	t := reEmoji.ReplaceAllString(roh, "")
	t = UmlauteHerstellen(t)
	t = entdoppeln(t)
	t = strings.TrimSpace(reRaum.ReplaceAllString(t, " "))
	t = reSatz.ReplaceAllStringFunc(t, strings.ToUpper)
	if t == "" {
		return roh
	}
	if letzte := t[len(t)-1]; letzte != '.' && letzte != '!' && letzte != '?' {
		t += "."
	}
	return t
}

// Faelschungen erzeugt drei Fälschungen und prüft sie. Verstößt ein Satz gegen
// die Themensperre oder das Abstandsfenster, wird neu erzeugt – höchstens
// MaxVersuche mal. Danach gilt der beste Satz: eine schwache Karte ist besser
// als eine Runde, die hängt.
const MaxVersuche = 3

func (c *Client) Faelschungen(ctx context.Context, frage, echt string, anker []string, d Dossier) (Ergebnis, error) {
	var best Ergebnis
	var letzterFehler error
	for versuch := 1; versuch <= MaxVersuche; versuch++ {
		erg, err := c.einDurchgang(ctx, frage, echt, anker, d)
		if err != nil {
			letzterFehler = err
			continue
		}
		erg.Versuche = versuch
		texte := make([]string, len(erg.Faelschungen))
		for i, f := range erg.Faelschungen {
			texte[i] = f.Text
		}
		erg.Befund = Abstandsfenster(echt, texte)
		bruch := Sperrbruch(texte, erg.Sperre)
		if erg.Befund.OK() && len(bruch) == 0 {
			return erg, nil
		}
		if !erg.Befund.OK() {
			letzterFehler = fmt.Errorf("abstandsfenster: %s", erg.Befund.Grund)
		} else {
			letzterFehler = fmt.Errorf("themensperre verletzt in %v", bruch)
			erg.Befund.Grund = "Sperrbruch"
		}
		if best.Fakt == "" || erg.Befund.MaxZuEcht < best.Befund.MaxZuEcht {
			best = erg
		}
	}
	if best.Fakt != "" {
		return best, nil // notgedrungen, aber spielbar
	}
	return Ergebnis{}, fmt.Errorf("keine brauchbaren fälschungen: %w", letzterFehler)
}

func (c *Client) einDurchgang(ctx context.Context, frage, echt string, anker []string, d Dossier) (Ergebnis, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "[frage]\n%s\n\n[echte_antwort]\n%s\n\n[anker]\n", frage, echt)
	for i, a := range anker {
		fmt.Fprintf(&b, "%d: %s   ", i+1, a)
	}
	if len(d.Fakten) > 0 {
		b.WriteString("\n\n[dossier · fakten]\n- " + strings.Join(d.Fakten, "\n- "))
	}
	if len(d.Gesperrt) > 0 {
		b.WriteString("\n\n[dossier · verbrauchte themen]\n" + strings.Join(d.Gesperrt, ", "))
	}
	if len(d.AntiBeispiele) > 0 {
		b.WriteString("\n\n[anti-beispiele]\n- " + strings.Join(d.AntiBeispiele, "\n- "))
	}

	inhalt, err := c.Chat(ctx, PromptFaelschungen, Huelle(b.String()), 1.0)
	if err != nil {
		return Ergebnis{}, err
	}
	var a antwortB
	if err := LiesJSON(inhalt, &a); err != nil {
		return Ergebnis{}, err
	}
	if len(a.Antworten) < 3 {
		return Ergebnis{}, fmt.Errorf("nur %d statt 3 antworten", len(a.Antworten))
	}
	// Einziger Ort, an dem Modellausgabe das Programm betritt – hier wird sie
	// geputzt, danach fasst sie niemand mehr an.
	erg := Ergebnis{Fakt: sicher.Text(a.Fakt, MaxFakt)}
	for _, t := range a.Sperre {
		if t = sicher.Text(t, MaxThema); t != "" {
			erg.Sperre = append(erg.Sperre, t)
		}
	}
	for i := 0; i < 3; i++ {
		t := sicher.Text(a.Antworten[i].Text, MaxKarte)
		if t == "" {
			return Ergebnis{}, fmt.Errorf("leere fälschung an stelle %d", i+1)
		}
		ank := sicher.Text(a.Antworten[i].Anker, MaxThema)
		if ank == "" && i < len(anker) {
			ank = anker[i]
		}
		erg.Faelschungen = append(erg.Faelschungen, game.Faelschung{Text: t, AnkerTag: ank})
	}
	return erg, nil
}
