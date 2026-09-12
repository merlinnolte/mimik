// Package game enthält die Spielregeln von MIMIK: den Zustand einer
// Runde, die Wertung und das Matchende. Bewusst ohne Datenbank, ohne HTTP und
// ohne Modellzugriff – alles hier ist eine reine Funktion der Spieldaten und
// damit vollständig testbar.
package game

// PlayerID ist die Kennung eines Spielers innerhalb einer Party.
type PlayerID string

// Zustand einer Runde. Wird nie direkt gesetzt, sondern aus dem Inhalt der
// Runde abgeleitet (siehe Ableiten). Gespeichert wird er nur, damit man danach
// filtern kann.
type Zustand string

const (
	Schreiben       Zustand = "SCHREIBEN"
	SchreibenWartet Zustand = "SCHREIBEN_WARTET"
	MimikArbeitet   Zustand = "MIMIK_ARBEITET"
	Raten           Zustand = "RATEN"
	RatenWartet     Zustand = "RATEN_WARTET"
	Aufgeloest      Zustand = "AUFGELOEST"
)

// Antwort ist das, was ein Spieler selbst geschrieben hat.
//
// Es gab hier einmal ein Feld InsDossier, mit dem man eine Antwort vom Lernen
// ausnehmen konnte. Es ist entfallen: Das Spiel lebt davon, dass MIMIK
// dazulernt, und ein Schalter, der das abstellt, war ein Angebot, gegen die
// eigene Spielidee zu spielen.
type Antwort struct {
	Original   string // so getippt, nur für die Chronik
	Normalform string // so angezeigt
}

// Karte ist eine der vier Auswahlmöglichkeiten. Die Reihenfolge steht ab dem
// ersten Öffnen des Ratebildschirms fest; ohne das könnte man durch mehrfaches
// Neuladen auf die echte Karte schließen.
type Karte struct {
	Pos      int // 1..4, Anzeigereihenfolge
	Text     string
	IstEcht  bool
	AnkerTag string // nur bei Fälschungen gesetzt
}

// Tipp ist die Wahl eines Spielers über den jeweils anderen.
type Tipp struct {
	Gewaehlt int // Pos der gewählten Karte
	Richtig  bool
}

// Runde bündelt alles, was zu einer Frage gehört.
//
// Richtungen, die man leicht verwechselt:
//   - Antworten[p] ist die echte Antwort von p.
//   - Karten[p] sind die vier Karten ÜBER p – sie werden dem anderen gezeigt.
//   - Tipps[p] ist der Tipp, den p abgegeben hat, also über den anderen.
type Runde struct {
	ID        string
	MatchID   string
	Nummer    int
	Frage     string
	Antworten map[PlayerID]Antwort
	Karten    map[PlayerID][]Karte
	Tipps     map[PlayerID]Tipp
}

// Party hält die beiden Spieler in fester Reihenfolge.
type Party struct {
	ID string
	A  PlayerID
	B  PlayerID
}

// Gegner liefert den jeweils anderen Spieler.
func (p Party) Gegner(x PlayerID) PlayerID {
	if x == p.A {
		return p.B
	}
	return p.A
}

// Mitglied sagt, ob ein Spieler zu dieser Party gehört.
func (p Party) Mitglied(x PlayerID) bool { return x == p.A || x == p.B }

func (r Runde) hatAntwort(p PlayerID) bool { _, ok := r.Antworten[p]; return ok }
func (r Runde) hatKarten(p PlayerID) bool  { return len(r.Karten[p]) == 4 }
func (r Runde) hatTipp(p PlayerID) bool    { _, ok := r.Tipps[p]; return ok }

// Ableiten bestimmt den Zustand allein aus dem Inhalt der Runde. Es gibt keine
// Übergänge, die man vergessen könnte – der Zustand ist immer konsistent mit
// dem, was gespeichert ist.
func (r Runde) Ableiten(p Party) Zustand {
	na, nb := r.hatAntwort(p.A), r.hatAntwort(p.B)
	if !na && !nb {
		return Schreiben
	}
	if na != nb {
		return SchreibenWartet
	}
	// Beide haben geschrieben: Tor 1. Jetzt baut MIMIK die Fälschungen.
	if !r.hatKarten(p.A) || !r.hatKarten(p.B) {
		return MimikArbeitet
	}
	ga, gb := r.hatTipp(p.A), r.hatTipp(p.B)
	if !ga && !gb {
		return Raten
	}
	if ga != gb {
		return RatenWartet
	}
	// Tor 2: die Runde endet mit dem zweiten Tipp.
	return Aufgeloest
}

// EchteKarte liefert die Position der echten Karte im Kartensatz über p.
func (r Runde) EchteKarte(p PlayerID) int {
	for _, k := range r.Karten[p] {
		if k.IstEcht {
			return k.Pos
		}
	}
	return 0
}
