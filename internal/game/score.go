package game

// Ziel ist die Punktzahl, bei der ein Match entschieden wird. Beide Seiten
// speisen sich aus denselben Ereignissen: jeder Tipp vergibt genau einen Punkt.
const Ziel = 10

// Stand ist der Punktestand eines Matches.
type Stand struct {
	Mensch int `json:"mensch"`
	Mimik  int `json:"mimik"`
}

// Tipps ist die Zahl der bisher abgegebenen Tipps. Weil jeder Tipp genau einen
// Punkt vergibt, ist das immer die Summe beider Zähler.
func (s Stand) Tipps() int { return s.Mensch + s.Mimik }

// Ergebnis eines Matches nach Abschluss einer Runde.
type Ergebnis string

const (
	Offen         Ergebnis = "OFFEN"
	MenschGewinnt Ergebnis = "MENSCH"
	MimikGewinnt  Ergebnis = "MIMIK"
	Verlaengerung Ergebnis = "VERLAENGERUNG"
	// Abgebrochen kommt nie aus Auswerten, sondern nur, wenn ein Mensch das
	// Spiel beendet. Es zählt als beendet: Das Match taucht nicht mehr als
	// laufend auf, der Punktestand bleibt zum Nachsehen stehen.
	Abgebrochen Ergebnis = "ABGEBROCHEN"
)

// Auswerten entscheidet über das Matchende. Es wird ausschließlich NACH dem
// Abschluss einer Runde aufgerufen, nie nach einem einzelnen Tipp – die
// laufende Runde wird immer zu Ende gespielt, auch wenn MIMIK mit dem ersten
// Tipp bereits das Ziel erreicht.
func Auswerten(s Stand) Ergebnis {
	if s.Mensch < Ziel && s.Mimik < Ziel {
		return Offen
	}
	switch {
	case s.Mensch > s.Mimik:
		return MenschGewinnt
	case s.Mimik > s.Mensch:
		return MimikGewinnt
	default:
		// Beide gleichauf am Ziel: eine weitere Runde entscheidet.
		return Verlaengerung
	}
}

// Verbuchen zählt die Punkte einer abgeschlossenen Runde auf den Stand.
// Eine Runde vergibt immer genau zwei Punkte, einen je Tipp.
func Verbuchen(s Stand, r Runde, p Party) Stand {
	for _, spieler := range []PlayerID{p.A, p.B} {
		t, ok := r.Tipps[spieler]
		if !ok {
			continue
		}
		if t.Richtig {
			s.Mensch++
		} else {
			s.Mimik++
		}
	}
	return s
}

// Doppeltreffer sagt, ob MIMIK in dieser Runde beide getäuscht hat.
func Doppeltreffer(r Runde, p Party) bool {
	ta, oka := r.Tipps[p.A]
	tb, okb := r.Tipps[p.B]
	return oka && okb && !ta.Richtig && !tb.Richtig
}
