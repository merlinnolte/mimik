package mimik

import (
	"fmt"
	"sort"
	"strings"
)

// Grenzen des Profils.
const (
	MaxMerkmalName = 40
	MaxMerkmalWert = 160
	MaxBeleg       = 300
	MaxMerkmale    = 12 // je Spieler
	MaxJeReview    = 3  // je Runde
	// MaxJeBuendel gilt fuer ein Sammelreview ueber mehrere Runden. Vier statt
	// drei: Drei Runden auf einmal geben mehr her als eine, aber nicht dreimal
	// so viel - was wirklich traegt, zeigt sich in wenigen Merkmalen.
	MaxJeBuendel   = 4
	ProfilSchwelle = 0.4 // ab hier geht ein Merkmal in den Faelschungsprompt
	ProfilVerfall  = 0.15
	ProfilKappe    = 0.9 // nie 1.0: Das hier ist eine Einschaetzung
	ProfilStart    = 0.45
	ProfilSchritt  = 0.2
)

// KernMerkmale fallen nie aus dem Profil, auch wenn es ueberlaeuft. Sie sind
// die Achsen, an denen sich eine Antwort am staerksten aufhaengt.
var KernMerkmale = map[string]bool{"geschwister": true, "geschlecht": true, "alter": true}

// Merkmal ist ein Stand, kein Ereignis: ein Wert je Spieler und Schluessel.
type Merkmal struct {
	Merkmal   string
	Wert      string
	Konfidenz float64
	Belege    int
	Wider     int
	Beleg     string
	RundeID   string
}

// Urteil ist, was das Modell zu einem Merkmal sagt.
type Urteil struct {
	Beobachtung string  `json:"beobachtung"`
	Merkmal     string  `json:"merkmal"`
	Wert        string  `json:"wert"`
	Urteil      string  `json:"urteil"`
	Konfidenz   float64 `json:"konfidenz"`
}

// Aenderung ist, was der Store schreiben soll: der neue Stand plus die Zeile
// fuer den Verlauf.
type Aenderung struct {
	Merkmal    Merkmal
	Urteil     string // NEU|BESTAETIGT|REVIDIERT|VERWORFEN|VERFALLEN
	WertVorher string
	Loeschen   bool
}

// MerkmalSchluessel normalisiert den Namen.
//
// Ohne das fuehrt ein Modell ueber zwanzig Runden drei Schreibweisen desselben
// Merkmals nebeneinander ("Geschwister?", "hat Geschwister", "anzahl
// geschwister") - und keine davon widerspricht je einer anderen.
func MerkmalSchluessel(roh string) string {
	k := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(roh))), " ")
	k = strings.Trim(k, "?:.!-")
	switch {
	case strings.Contains(k, "geschwister"), strings.Contains(k, "bruder"),
		strings.Contains(k, "schwester"):
		return "geschwister"
	case strings.Contains(k, "geschlecht"):
		return "geschlecht"
	case strings.HasPrefix(k, "alter"), strings.Contains(k, "lebensalter"),
		strings.Contains(k, "altersspanne"):
		return "alter"
	}
	if len([]rune(k)) > MaxMerkmalName {
		k = string([]rune(k)[:MaxMerkmalName])
	}
	return k
}

// deckel begrenzt die Konfidenz nach der Zahl der Belege.
//
// Eine einzelne Runde kann ein Merkmal nie sicher machen, egal was das Modell
// vorschlaegt: Vier Saetze sind kein Beweis. Erst mehrere Runden, die
// unabhaengig dasselbe nahelegen, rechtfertigen mehr.
func deckel(belege int) float64 {
	switch {
	case belege <= 1:
		return ProfilStart
	case belege == 2:
		return 0.7
	default:
		return ProfilKappe
	}
}

// ProfilVerrechnen ist die einzige Stelle, an der ein Modellurteil zu Konfidenz
// wird.
//
// Die Arithmetik steht hier und nicht im Prompt, weil sie sonst selbst aus dem
// Modell kaeme - und ein Modell, das seine eigene Sicherheit bewertet, spinnt
// sich in jede Annahme hinein, die es einmal gefasst hat. Zustimmung steigt in
// kleinen Schritten, Widerspruch halbiert: Ein falsches Merkmal ist teurer als
// ein fehlendes, also soll es schneller fallen, als es steigt.
func ProfilVerrechnen(alt []Merkmal, urteile []Urteil, rundeID string) []Aenderung {
	stand := map[string]Merkmal{}
	for _, m := range alt {
		stand[m.Merkmal] = m
	}
	var out []Aenderung
	gesehen := map[string]bool{}

	for _, u := range urteile {
		if len(out) >= MaxJeBuendel {
			break
		}
		k := MerkmalSchluessel(u.Merkmal)
		// Zwei Urteile zum selben Merkmal aus EINEM Aufruf sind ein Beleg,
		// nicht zwei - sie stammen aus demselben Blick auf dasselbe Material.
		// Seit ein Aufruf drei Runden auf einmal auswertet, heisst das: Das
		// Profil festigt sich langsamer. Gewollt: Ein Profil, das langsam
		// sicher wird, ist besser als eines, das sich schnell einspinnt.
		if k == "" || gesehen[k] {
			continue
		}
		gesehen[k] = true
		vor, da := stand[k]
		vorschlag := u.Konfidenz
		if vorschlag < 0 {
			vorschlag = 0
		}
		if vorschlag > ProfilKappe {
			vorschlag = ProfilKappe
		}

		switch strings.ToUpper(strings.TrimSpace(u.Urteil)) {
		case "NEU":
			if da || u.Wert == "" {
				continue
			}
			m := Merkmal{
				Merkmal: k, Wert: u.Wert, Belege: 1, Beleg: u.Beobachtung, RundeID: rundeID,
				Konfidenz: min(vorschlag, deckel(1)),
			}
			out = append(out, Aenderung{Merkmal: m, Urteil: "NEU"})

		case "BESTAETIGT":
			if !da {
				continue
			}
			m := vor
			m.Belege++
			m.Beleg, m.RundeID = u.Beobachtung, rundeID
			if u.Wert != "" {
				m.Wert = u.Wert
			}
			hoch := vorschlag
			if hoch > vor.Konfidenz+ProfilSchritt {
				hoch = vor.Konfidenz + ProfilSchritt
			}
			if hoch < vor.Konfidenz {
				hoch = vor.Konfidenz
			}
			m.Konfidenz = min(hoch, deckel(m.Belege))
			out = append(out, Aenderung{Merkmal: m, Urteil: "BESTAETIGT", WertVorher: vor.Wert})

		case "REVIDIERT":
			if !da || u.Wert == "" {
				continue
			}
			halb := vor.Konfidenz / 2
			m := vor
			m.Wider++
			m.Beleg, m.RundeID = u.Beobachtung, rundeID
			if vorschlag > halb {
				// Der neue Wert setzt sich nur durch, wenn er den halbierten
				// alten schlaegt. Zwei Runden, die einander widersprechen,
				// landen sonst im Flackern zwischen zwei Werten.
				m.Wert = u.Wert
				m.Belege = 1
				m.Konfidenz = min(vorschlag, deckel(1))
			} else {
				m.Konfidenz = halb
			}
			out = append(out, Aenderung{Merkmal: m, Urteil: "REVIDIERT", WertVorher: vor.Wert})

		case "VERWORFEN":
			if !da {
				continue
			}
			out = append(out, Aenderung{
				Merkmal: vor, Urteil: "VERWORFEN", WertVorher: vor.Wert, Loeschen: true,
			})
		}
	}

	// Verfall und Ueberlauf auf dem Stand NACH den Urteilen.
	neu := map[string]Merkmal{}
	for k, m := range stand {
		neu[k] = m
	}
	for _, a := range out {
		if a.Loeschen {
			delete(neu, a.Merkmal.Merkmal)
		} else {
			neu[a.Merkmal.Merkmal] = a.Merkmal
		}
	}
	for k, m := range neu {
		if m.Konfidenz < ProfilVerfall {
			delete(neu, k)
			out = append(out, Aenderung{
				Merkmal: m, Urteil: "VERFALLEN", WertVorher: m.Wert, Loeschen: true,
			})
		}
	}
	if len(neu) > MaxMerkmale {
		var schwach []Merkmal
		for _, m := range neu {
			if !KernMerkmale[m.Merkmal] {
				schwach = append(schwach, m)
			}
		}
		sort.Slice(schwach, func(i, j int) bool { return schwach[i].Konfidenz < schwach[j].Konfidenz })
		for i := 0; i < len(schwach) && len(neu) > MaxMerkmale; i++ {
			delete(neu, schwach[i].Merkmal)
			out = append(out, Aenderung{
				Merkmal: schwach[i], Urteil: "VERFALLEN", WertVorher: schwach[i].Wert, Loeschen: true,
			})
		}
	}
	return out
}

// Stufe gibt der Konfidenz ein Wort.
//
// Weder der Mensch im Dossier-Bildschirm noch das Modell im Prompt faengt mit
// "0.45" etwas an: Die Zahl behauptet eine Genauigkeit, die diese Schaetzung
// nicht hat.
func Stufe(k float64) string {
	switch {
	case k >= 0.7:
		return "ziemlich sicher"
	case k >= ProfilSchwelle:
		return "wahrscheinlich"
	default:
		return "vermutung"
	}
}

// ProfilZeilen formt das Profil fuer einen Prompt-Block.
func ProfilZeilen(ms []Merkmal, abKonfidenz float64) []string {
	var out []string
	for _, m := range ms {
		if m.Konfidenz < abKonfidenz {
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s (%s)", m.Merkmal, m.Wert, Stufe(m.Konfidenz)))
	}
	sort.Strings(out)
	return out
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ThemenFuerPrompt waehlt aus, welche verbrauchten Themen mitfahren.
//
// Warum nicht einfach die juengsten: Der Block wirkt allein ueber den Prompt -
// Sperrbruch prueft nur die Sperre der LAUFENDEN Runde nach. Ein Thema, das zur
// Frage nicht passt, kann auch nicht versehentlich wiederholt werden; es kostet
// nur Tokens. Gewaehlt wird deshalb nach Naehe zur FRAGE, plus die juengsten
// wenigen: Das zuletzt verbrauchte Thema ist das, das dem Modell am naechsten
// liegt.
//
// Wofuer ueberhaupt ein Deckel: Ein bis drei Themen kommen je Runde dazu, und
// das Dossier haengt am Spieler, nicht am Match. Nach zehn Matches waeren das
// rund 200 Themen in JEDEM Aufruf - und der Teil ist nicht cachebar.
func ThemenFuerPrompt(juengsteZuerst []string, frage string, n int) []string {
	if len(juengsteZuerst) <= n {
		return juengsteZuerst
	}
	const jung = 6
	drin := map[string]bool{}
	out := make([]string, 0, n)
	nimm := func(t string) {
		if t == "" || drin[t] || len(out) >= n {
			return
		}
		drin[t] = true
		out = append(out, t)
	}
	for i := 0; i < len(juengsteZuerst) && i < jung; i++ {
		nimm(juengsteZuerst[i])
	}
	// Der Rest nach Naehe zur Frage. Bei Gleichstand gewinnt das juengere:
	// sort.SliceStable auf einer Liste, die schon juengste-zuerst steht.
	rest := make([]string, 0, len(juengsteZuerst))
	for _, t := range juengsteZuerst[minInt(jung, len(juengsteZuerst)):] {
		rest = append(rest, t)
	}
	sort.SliceStable(rest, func(i, j int) bool {
		return SperrNaehe(rest[i], frage) > SperrNaehe(rest[j], frage)
	})
	for _, t := range rest {
		nimm(t)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
