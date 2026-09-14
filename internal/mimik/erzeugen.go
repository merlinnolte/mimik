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
	// Interessen sind die Tags des Spielers – alle, ungefiltert.
	//
	// Sie waren einmal "Anker": Drei davon wurden gewürfelt und jeder Fälschung
	// fest zugewiesen. Das ging schief, weil die Auswahl die FRAGE nie ansah –
	// "schlaf" landete bei "Wofür gibst du zu viel Geld aus?" durch reinen
	// Zufall, und das Modell musste eine Verbindung erfinden, die es nicht gibt.
	// Jetzt sucht es sich selbst aus, was zur Frage passt.
	Interessen []string
	Fakten     []string
	// Profil sind vorgeformte Zeilen aus ProfilZeilen - Annahmen ueber den
	// Menschen, nicht seine Saetze. Als []string und nicht als []Merkmal, damit
	// einDurchgang nichts von Konfidenzarithmetik wissen muss.
	Profil        []string
	Gesperrt      []string
	AntiBeispiele []string
}

// Ergebnis eines Aufrufs samt Diagnose.
type Ergebnis struct {
	// Normalform ist die echte Antwort in sauberer Schreibweise. Sie kommt aus
	// demselben Aufruf wie die Fälschungen, damit alle vier Karten dieselbe
	// Hand haben.
	Normalform   string
	Fakt         string
	Sperre       []string
	Faelschungen []game.Faelschung
	Versuche     int
	Befund       Befund
	// Verbrauch aller Durchgaenge, einzeln. Wiederholungen stehen jede fuer
	// sich darin - genau dort steckt das Geld, und eine Summe verwischte es.
	Verbrauch []Verbrauch
}

type antwortB struct {
	Normalform string   `json:"normalform"`
	Fakt       string   `json:"fakt"`
	Sperre     []string `json:"sperre"`
	Antworten  []struct {
		Richtung    string `json:"richtung"`
		Text        string `json:"text"`
		Begruendung string `json:"begruendung"`
	} `json:"antworten"`
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

// BotAntwort laesst den Testspieler antworten. Nur fuer die Testpartie.
func (c *Client) BotAntwort(ctx context.Context, frage string, interessen, schonGesagt []string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "[frage]\n%s", frage)
	if len(interessen) > 0 {
		b.WriteString("\n\n[interessen]\n" + strings.Join(interessen, ", "))
	}
	if len(schonGesagt) > 0 {
		b.WriteString("\n\n[schon gesagt]\n- " + strings.Join(schonGesagt, "\n- "))
	}
	inhalt, _, err := c.Chat(mimikKennung(ctx, "botantwort"), PromptBotAntwort, Huelle(b.String()), 1.0)
	if err != nil {
		return "", err
	}
	var z struct {
		Antwort string `json:"antwort"`
	}
	if err := LiesJSON(inhalt, &z); err != nil {
		return "", err
	}
	t := sicher.Text(z.Antwort, MaxKarte)
	if len([]rune(t)) < game.MinAntwort {
		return "", fmt.Errorf("testspieler antwortet zu kurz: %q", t)
	}
	return t, nil
}

// Faelschungen erzeugt drei Fälschungen und prüft sie. Verstößt ein Satz gegen
// die Themensperre oder das Abstandsfenster, wird neu erzeugt – höchstens
// MaxVersuche mal. Danach gilt der beste Satz: eine schwache Karte ist besser
// als eine Runde, die hängt.
const MaxVersuche = 3

// roh ist die Antwort, wie die Person sie getippt hat. Das Modell bekommt sie
// ungeglättet: Es soll den Stil sehen, bevor es ihn nachmacht – und es schreibt
// die saubere Fassung selbst, damit alle vier Karten in derselben Schreibweise
// stehen.
func (c *Client) Faelschungen(ctx context.Context, frage, roh string, d Dossier) (Ergebnis, error) {
	var best Ergebnis
	var letzterFehler error
	// Der Verbrauch wird ueber alle Durchgaenge gesammelt, nicht je Durchgang
	// zurueckgegeben: Ein verworfener Versuch ist bezahlt, und wer die Rechnung
	// liest, will die Wiederholungen sehen.
	var gesamt []Verbrauch
	for versuch := 1; versuch <= MaxVersuche; versuch++ {
		erg, err := c.einDurchgang(ctx, frage, roh, d)
		gesamt = append(gesamt, erg.Verbrauch...)
		erg.Verbrauch = nil
		if err != nil {
			letzterFehler = err
			continue
		}
		erg.Versuche = versuch
		texte := make([]string, len(erg.Faelschungen))
		for i, f := range erg.Faelschungen {
			texte[i] = f.Text
		}
		// Gemessen wird gegen die Normalform, nicht gegen den rohen Text: Die
		// Normalform ist es, die als Karte danebensteht.
		// Erst der Boden: Zwei wortgleiche Karten gehen nie hinaus, auch nicht
		// als bester von drei schlechten Versuchen.
		if i, j, doppelt := Unzumutbar(erg.Normalform, texte); doppelt {
			letzterFehler = fmt.Errorf("karte %d und %d sind praktisch dieselbe", i, j)
			continue
		}
		erg.Befund = Abstandsfenster(erg.Normalform, texte)
		bruch := Sperrbruch(texte, erg.Sperre)
		form := FormPruefen(erg.Normalform, texte)
		if erg.Befund.OK() && len(bruch) == 0 && form.OK() {
			erg.Verbrauch = gesamt
			return erg, nil
		}
		switch {
		case !erg.Befund.OK():
			letzterFehler = fmt.Errorf("abstandsfenster: %s", erg.Befund.Grund)
		case len(bruch) > 0:
			letzterFehler = fmt.Errorf("themensperre verletzt in %v", bruch)
			erg.Befund.Grund = "Sperrbruch"
		default:
			letzterFehler = fmt.Errorf("form: %s", form.Grund())
			erg.Befund.Grund = "Form"
		}
		if best.Fakt == "" || erg.Befund.MaxZuEcht < best.Befund.MaxZuEcht {
			best = erg
		}
	}
	if best.Fakt != "" {
		best.Verbrauch = gesamt
		return best, nil // notgedrungen, aber spielbar
	}
	return Ergebnis{Verbrauch: gesamt},
		fmt.Errorf("keine brauchbaren fälschungen: %w", letzterFehler)
}

func (c *Client) einDurchgang(ctx context.Context, frage, roh string, d Dossier) (Ergebnis, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "[frage]\n%s\n\n[echte_antwort_roh]\n%s", frage, roh)
	if len(d.Interessen) > 0 {
		b.WriteString("\n\n[interessen]\n" + strings.Join(d.Interessen, ", "))
	}
	if len(d.Fakten) > 0 {
		b.WriteString("\n\n[dossier · fakten]\n- " + strings.Join(d.Fakten, "\n- "))
	}
	if len(d.Profil) > 0 {
		b.WriteString("\n\n[dossier · profil]\n- " + strings.Join(d.Profil, "\n- "))
	}
	if len(d.Gesperrt) > 0 {
		b.WriteString("\n\n[dossier · verbrauchte themen]\n" + strings.Join(d.Gesperrt, ", "))
	}
	if len(d.AntiBeispiele) > 0 {
		b.WriteString("\n\n[anti-beispiele]\n- " + strings.Join(d.AntiBeispiele, "\n- "))
	}

	inhalt, verbrauch, err := c.Chat(mimikKennung(ctx, "faelschungen"), PromptFaelschungen, Huelle(b.String()), 1.0)
	// Der Verbrauch reist auch bei Fehlern mit: Ein Aufruf, der nichts
	// Brauchbares liefert, ist trotzdem bezahlt - und genau der soll sichtbar
	// sein.
	if err != nil {
		return Ergebnis{Verbrauch: []Verbrauch{verbrauch}}, err
	}
	var a antwortB
	if err := LiesJSON(inhalt, &a); err != nil {
		return Ergebnis{Verbrauch: []Verbrauch{verbrauch}}, err
	}
	if len(a.Antworten) < 3 {
		return Ergebnis{Verbrauch: []Verbrauch{verbrauch}},
			fmt.Errorf("nur %d statt 3 antworten", len(a.Antworten))
	}
	// Einziger Ort, an dem Modellausgabe das Programm betritt – hier wird sie
	// geputzt, danach fasst sie niemand mehr an.
	// Die Normalform darf glätten, aber nicht umschreiben. Hält sie sich nicht
	// daran, gilt die regelbasierte Fassung – lieber eine Karte mit kleinem
	// Substantiv als eine, die etwas anderes sagt als die Person.
	norm := sicher.Text(a.Normalform, MaxKarte)
	if !NormalformPlausibel(roh, norm) {
		norm = roh
	}
	// Alle vier Texte laufen durch dieselbe mechanische Glättung: großer
	// Satzanfang, ein Satzzeichen am Ende, keine Mehrfachzeichen, keine Emoji.
	// Das ist billiger als ein neuer Aufruf und behebt genau die Mängel, die
	// eine Regel beheben KANN. Was sie nicht kann - Substantive mitten im Satz -
	// hat das Modell schon erledigt.
	erg := Ergebnis{
		Normalform: ErsatzNormalform(norm),
		Fakt:       sicher.Text(a.Fakt, MaxFakt),
		Verbrauch:  []Verbrauch{verbrauch},
	}
	for _, t := range a.Sperre {
		if t = sicher.Text(t, MaxThema); t != "" {
			erg.Sperre = append(erg.Sperre, t)
		}
	}
	for i := 0; i < 3; i++ {
		t := sicher.Text(a.Antworten[i].Text, MaxKarte)
		if t == "" {
			return Ergebnis{Verbrauch: []Verbrauch{verbrauch}},
				fmt.Errorf("leere fälschung an stelle %d", i+1)
		}
		t = ErsatzNormalform(t)
		erg.Faelschungen = append(erg.Faelschungen, game.Faelschung{
			Text:        t,
			Richtung:    sicher.Text(a.Antworten[i].Richtung, MaxThema),
			Begruendung: sicher.Text(a.Antworten[i].Begruendung, MaxBeleg),
		})
	}
	return erg, nil
}

// mimikKennung haengt den Zweck an den Kontext, damit die Rechnung eines
// Aufrufs sagen kann, wofuer er war.
func mimikKennung(ctx context.Context, zweck string) context.Context {
	alt := kennung(ctx)
	return MitKennung(ctx, zweck, alt.RundeID)
}

// ---------------------------------------------------------------- Review ---

// Reviewrunde ist eine gespielte Runde, wie das Review sie zu sehen bekommt.
// Die Antwort des Partners steht bewusst nicht darin: Das Profil, das dieser
// Spieler spaeter selbst lesen kann, darf nichts ueber den anderen enthalten.
type Reviewrunde struct {
	RundeID  string
	Frage    string
	Antwort  string // Normalform des Spielers
	Karten   []game.Karte
	Gewaehlt int // Position, auf die das Gegenueber getippt hat
	Richtig  bool
}

// Reviewmaterial ist ein Buendel: MEHRERE Runden desselben Spielers in einem
// Aufruf.
//
// Vorher lief ein Aufruf je Runde, und der Systemprompt von 3.385 Zeichen ging
// jedes Mal mit. Drei Runden zusammen kosten einen Systemprompt statt drei -
// und das Modell sieht mehr Belege auf einmal, was der Sache eher hilft als
// schadet. Der Preis steht in ProfilVerrechnen: Ein Buendel zaehlt als EIN
// Beleg, das Profil festigt sich also langsamer. Das ist die richtige Richtung.
type Reviewmaterial struct {
	Runden []Reviewrunde
	Profil []Merkmal
}

// Reviewergebnis ist, was das Modell zurueckgibt.
type Reviewergebnis struct {
	Gewaehlt  string   `json:"gewaehlt"`
	Verworfen string   `json:"verworfen"`
	Urteile   []Urteil `json:"merkmale"`
}

// Review wertet ein Buendel gespielter Runden aus.
func (c *Client) Review(ctx context.Context, m Reviewmaterial) (Reviewergebnis, Verbrauch, error) {
	if len(m.Runden) == 0 {
		return Reviewergebnis{}, Verbrauch{}, fmt.Errorf("review ohne runden")
	}
	var b strings.Builder
	for i, r := range m.Runden {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "\n[runde %d · frage]\n%s", i+1, r.Frage)
		fmt.Fprintf(&b, "\n\n[runde %d · echte_antwort]\n%s", i+1, r.Antwort)
		fmt.Fprintf(&b, "\n\n[runde %d · karten]", i+1)
		for _, k := range r.Karten {
			marke := ""
			// Die Marken stehen an der Karte und nicht in einem eigenen Feld:
			// Ein Modell, das die Zuordnung aus zwei getrennten Listen
			// rekonstruieren muss, dreht sie gelegentlich um.
			if k.IstEcht {
				marke += " (echt)"
			}
			if k.Pos == r.Gewaehlt {
				marke += " (gewählt)"
			}
			fmt.Fprintf(&b, "\n%d %s%s", k.Pos, k.Text, marke)
		}
		fmt.Fprintf(&b, "\n\n[runde %d · ergebnis]\nDas Gegenüber hat auf %d getippt und lag %s.",
			i+1, r.Gewaehlt, map[bool]string{true: "richtig", false: "falsch"}[r.Richtig])
	}
	if zeilen := ProfilZeilen(m.Profil, 0); len(zeilen) > 0 {
		b.WriteString("\n\n[profil]\n- " + strings.Join(zeilen, "\n- "))
	}

	inhalt, verbrauch, err := c.Chat(
		mimikKennung(ctx, "review"), PromptReview, Huelle(strings.TrimSpace(b.String())), 0.4)
	if err != nil {
		return Reviewergebnis{}, verbrauch, err
	}
	var erg Reviewergebnis
	if err := LiesJSON(inhalt, &erg); err != nil {
		return Reviewergebnis{}, verbrauch, err
	}
	erg.Gewaehlt = sicher.Text(erg.Gewaehlt, MaxBeleg)
	erg.Verworfen = sicher.Text(erg.Verworfen, MaxBeleg)
	sauber := make([]Urteil, 0, len(erg.Urteile))
	for _, u := range erg.Urteile {
		u.Merkmal = MerkmalSchluessel(sicher.Text(u.Merkmal, MaxMerkmalName))
		u.Wert = sicher.Text(u.Wert, MaxMerkmalWert)
		u.Beobachtung = sicher.Text(u.Beobachtung, MaxBeleg)
		switch strings.ToUpper(strings.TrimSpace(u.Urteil)) {
		case "NEU", "BESTAETIGT", "REVIDIERT", "VERWORFEN":
			u.Urteil = strings.ToUpper(strings.TrimSpace(u.Urteil))
		default:
			// Ein Urteil, das keins ist, ist kein Grund, die ganze Runde
			// wegzuwerfen - aber es wird auch nicht geraten.
			continue
		}
		if u.Merkmal == "" {
			continue
		}
		sauber = append(sauber, u)
		if len(sauber) >= MaxJeBuendel {
			break
		}
	}
	erg.Urteile = sauber
	return erg, verbrauch, nil
}
