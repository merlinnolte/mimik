// Package mimik kapselt alles, was mit dem Sprachmodell zu tun hat: die beiden
// Prompts, den Aufruf und – wichtiger – die Prüfungen danach. Kein Ergebnis des
// Modells erreicht das Spiel ungeprüft.
package mimik

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Schwellen. Sie gelten für das n-Gramm-Maß unten, NICHT für Embeddings.
// Das Dossier nennt für Embeddings 0.72 statt 0.35; beim Umstieg auf ein
// Embedding-Modell gehören die Dossier-Werte hierher.
const (
	SimMaxEcht   = 0.35 // Fälschung darf der echten Antwort nicht näher kommen
	SimEnthalten = 0.75 // ab hier steckt der eine Text im anderen
	// Oberhalb dieser Grenze sind zwei Karten praktisch dieselbe. Anders als
	// die übrigen Schwellen ist diese KEINE Empfehlung, sondern ein Boden:
	// Solche Karten gehen nie hinaus, auch nicht notgedrungen.
	SimUnzumutbar = 0.80
	SimStreuung   = 0.15 // Fälschungen dürfen nicht enger beieinander liegen
	SperrSchwelle = 0.60 // ab hier gilt ein gesperrtes Thema als berührt
)

func normalisieren(s string) string {
	var b strings.Builder
	letzterRaum := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			letzterRaum = false
		case !letzterRaum:
			b.WriteRune(' ')
			letzterRaum = true
		}
	}
	return strings.TrimSpace(b.String())
}

// gramme zerlegt einen Text in n-Gramme über Zeichen (nicht Bytes, damit
// Umlaute nicht zerschnitten werden).
func gramme(s string, n int) map[string]struct{} {
	rs := []rune(normalisieren(s))
	out := map[string]struct{}{}
	for i := 0; i+n <= len(rs); i++ {
		out[string(rs[i:i+n])] = struct{}{}
	}
	return out
}

func schnitt(a, b map[string]struct{}) int {
	n := 0
	for k := range a {
		if _, ok := b[k]; ok {
			n++
		}
	}
	return n
}

// Aehnlichkeit vergleicht zwei Antworten. Beide sind ähnlich lang, deshalb
// symmetrisch (Jaccard über Vierergramme).
func Aehnlichkeit(a, b string) float64 {
	ga, gb := gramme(a, 4), gramme(b, 4)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	s := schnitt(ga, gb)
	return float64(s) / float64(len(ga)+len(gb)-s)
}

// Enthalten ist gerichtet: Wie viel von a steckt in b? Anders als Aehnlichkeit
// bestraft es einen Längenunterschied nicht.
//
// Warum es das braucht: Jaccard teilt durch die Vereinigung. Steht die echte
// Antwort vollständig in einer viel längeren Fälschung, ist die Vereinigung
// groß und die Ähnlichkeit trotzdem klein. Gemessen an einem echten Durchlauf
// am 12.09.2026: "Der Stapel Zei." gegen "Der Stapel Zeitschriften neben dem
// Sofa" ergibt symmetrisch 0.31 – unter der Schwelle, die Karte wäre
// durchgegangen – während die echte Antwort Zeichen für Zeichen in der
// Fälschung steht. Genau der Fall, den "Umkreisen" verhindern soll.
func Enthalten(a, b string) float64 {
	ga := gramme(a, 4)
	if len(ga) == 0 {
		return 0
	}
	return float64(schnitt(ga, gramme(b, 4))) / float64(len(ga))
}

// SperrNaehe vergleicht ein gesperrtes Thema mit einer Antwort. Die Längen sind
// sehr ungleich, deshalb gerichtet: gefragt ist, wie viel vom Thema in der
// Antwort steckt, nicht umgekehrt. Symmetrisch gemessen ginge ein Ein-Wort-Thema
// gegen einen Dreizeiler immer gegen null und die Prüfung liefe ins Leere.
func SperrNaehe(thema, text string) float64 {
	gt := gramme(thema, 3)
	if len(gt) == 0 {
		return 0
	}
	return float64(schnitt(gt, gramme(text, 3))) / float64(len(gt))
}

// Befund ist das Ergebnis der Abstandsprüfung über einen Kartensatz.
type Befund struct {
	MaxZuEcht   float64 // höchste Ähnlichkeit einer Fälschung zur echten Antwort
	MittelPeers float64 // mittlere Ähnlichkeit der Fälschungen untereinander
	NaeheOK     bool
	StreuungOK  bool
	Schuldig    []int // Indizes der Fälschungen, die neu erzeugt werden müssen
	Grund       string
}

func (b Befund) OK() bool { return b.NaeheOK && b.StreuungOK }

// Abstandsfenster prüft beide Fehlerarten auf einmal:
//
//	Umkreisen – eine Fälschung liegt zu nah an der echten Antwort. Dann gäbe es
//	            zwei richtige Karten und der Tipp wird zur Münze.
//	Ausreißer – die Fälschungen liegen enger beieinander als zur echten. Dann
//	            erkennt man die echte, ohne die Person zu kennen.
func Abstandsfenster(echt string, faelschungen []string) Befund {
	b := Befund{NaeheOK: true, StreuungOK: true}
	for i, f := range faelschungen {
		s := Aehnlichkeit(echt, f)
		if s > b.MaxZuEcht {
			b.MaxZuEcht = s
		}
		// Zwei Maße, weil sie verschiedene Fehler sehen: Jaccard findet zwei
		// ähnlich lange Texte, die dasselbe sagen; die gerichtete Enthaltung
		// findet den Text, der im anderen steckt. In beide Richtungen geprüft,
		// denn beide Seiten können die kürzere sein.
		if s > SimMaxEcht || Enthalten(echt, f) > SimEnthalten || Enthalten(f, echt) > SimEnthalten {
			b.NaeheOK = false
			b.Schuldig = append(b.Schuldig, i)
		}
	}
	if !b.NaeheOK {
		b.Grund = "Nähe"
		return b
	}
	paare := 0
	for i := range faelschungen {
		for j := i + 1; j < len(faelschungen); j++ {
			b.MittelPeers += Aehnlichkeit(faelschungen[i], faelschungen[j])
			paare++
		}
	}
	if paare > 0 {
		b.MittelPeers /= float64(paare)
	}
	if b.MittelPeers-b.MaxZuEcht > SimStreuung {
		b.StreuungOK = false
		b.Grund = "Streuung"
		// Die engste der drei fliegt: die mit der höchsten Summe zu den anderen.
		besteSumme, besterIdx := -1.0, 0
		for i := range faelschungen {
			summe := 0.0
			for j := range faelschungen {
				if i != j {
					summe += Aehnlichkeit(faelschungen[i], faelschungen[j])
				}
			}
			if summe > besteSumme {
				besteSumme, besterIdx = summe, i
			}
		}
		b.Schuldig = []int{besterIdx}
	}
	return b
}

// Unzumutbar sagt, ob zwei der vier Karten praktisch dieselbe sind – die echte
// gegen eine Fälschung oder zwei Fälschungen untereinander.
//
// Das ist der Boden unter der Rückfallebene. Abstandsfenster und Sperrbruch
// dürfen scheitern; dann geht notgedrungen der beste Satz hinaus, denn eine
// schwache Karte ist besser als eine Runde, die hängt. Zwei wortgleiche Karten
// sind aber keine schwache Runde, sondern eine kaputte: Der Tipp wird zur
// Münze, und wer es merkt, hat das Spiel durchschaut.
//
// Am 13.09.2026 im Emulator aufgetreten – die echte Antwort stand zweimal im
// Kartensatz, weil drei Versuche nichts Besseres brachten und die
// Rückfallebene alles durchließ.
func Unzumutbar(normalform string, faelschungen []string) (int, int, bool) {
	alle := append([]string{normalform}, faelschungen...)
	for i := range alle {
		for j := i + 1; j < len(alle); j++ {
			if Aehnlichkeit(alle[i], alle[j]) >= SimUnzumutbar ||
				Enthalten(alle[i], alle[j]) >= 0.90 || Enthalten(alle[j], alle[i]) >= 0.90 {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// Sperrbruch findet Fälschungen, die ein gesperrtes Thema doch berühren.
// Das ist die billige, modellfreie Hälfte der Nähe-Prüfung: MIMIK benennt das
// Thema der echten Antwort selbst, und hier wird nachgehalten, dass sie sich
// daran hält.
func Sperrbruch(faelschungen []string, sperre []string) []int {
	var schuldig []int
	for i, f := range faelschungen {
		for _, thema := range sperre {
			if SperrNaehe(thema, f) >= SperrSchwelle {
				schuldig = append(schuldig, i)
				break
			}
		}
	}
	return schuldig
}

// NormalformPlausibel prüft die Normalform aus dem Modellaufruf. Sie darf
// Rechtschreibung glätten, aber nicht umschreiben: Länge höchstens zehn Prozent
// daneben, Satzzahl höchstens um eins verschoben.
func NormalformPlausibel(original, normalform string) bool {
	if strings.TrimSpace(normalform) == "" {
		return false
	}
	o, n := utf8.RuneCountInString(original), utf8.RuneCountInString(normalform)
	if float64(abs(o-n)) > 0.10*float64(o)+12 {
		return false
	}
	return abs(saetze(original)-saetze(normalform)) <= 1
}

func saetze(s string) int {
	n := 0
	for _, r := range s {
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			n++
		}
	}
	if n == 0 && strings.TrimSpace(s) != "" {
		return 1
	}
	return n
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
