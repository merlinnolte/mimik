package game

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"
)

// Grenzen für eine Spielerantwort.
//
// MinAntwort stand zuerst bei 25, mit der Begründung, kurze Antworten seien
// trivial zu fälschen. Die erste Sammlung echter Antworten hat das widerlegt:
// "Kündige!" hat acht Zeichen, ist unverwechselbar und wäre abgewiesen worden.
// Kurz ist nicht beliebig – kurz ist eine Form, die MIMIK nachbauen muss. Die
// harte Grenze schützt nur noch vor dem leeren Feld; alles darüber ist eine
// Gestaltungsfrage und gehört in einen Hinweis, nicht in eine Sperre.
const (
	MinAntwort = 4
	MaxAntwort = 400
)

var (
	ErrFalscherZustand = errors.New("in diesem Zustand nicht erlaubt")
	ErrKeinMitglied    = errors.New("spieler gehört nicht zu dieser party")
	ErrZuKurz          = fmt.Errorf("antwort kürzer als %d zeichen", MinAntwort)
	ErrZuLang          = fmt.Errorf("antwort länger als %d zeichen", MaxAntwort)
	ErrSchonGetippt    = errors.New("dieser spieler hat bereits getippt")
	ErrKartePos        = errors.New("karte gibt es nicht")
)

// Faelschung ist eine von MIMIK erzeugte Antwort samt ihrem Anker.
type Faelschung struct {
	Text     string
	AnkerTag string
}

// PruefeAntwort prüft eine Spielerantwort gegen die Längengrenzen.
func PruefeAntwort(text string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	switch {
	case n < MinAntwort:
		return ErrZuKurz
	case n > MaxAntwort:
		return ErrZuLang
	}
	return nil
}

// AntwortAbgeben trägt die echte Antwort eines Spielers ein. Erlaubt nur,
// solange die Runde in der Schreibphase steht – danach hätte der Spieler die
// Karten des anderen schon gesehen.
func (r *Runde) AntwortAbgeben(p Party, spieler PlayerID, a Antwort) error {
	if !p.Mitglied(spieler) {
		return ErrKeinMitglied
	}
	if z := r.Ableiten(p); z != Schreiben && z != SchreibenWartet {
		return fmt.Errorf("%w: %s", ErrFalscherZustand, z)
	}
	if r.hatAntwort(spieler) {
		return fmt.Errorf("%w: bereits geantwortet", ErrFalscherZustand)
	}
	if err := PruefeAntwort(a.Normalform); err != nil {
		return err
	}
	if r.Antworten == nil {
		r.Antworten = map[PlayerID]Antwort{}
	}
	r.Antworten[spieler] = a
	return nil
}

// KartenSetzen legt die vier Karten über einen Spieler fest und mischt sie
// einmalig. Die Reihenfolge wird damit eingefroren: ein Neuladen des
// Ratebildschirms mischt nicht neu, sonst ließe sich die echte Karte durch
// Positionsvergleich erschließen.
func (r *Runde) KartenSetzen(spieler PlayerID, faelschungen []Faelschung, rng *rand.Rand) error {
	a, ok := r.Antworten[spieler]
	if !ok {
		return fmt.Errorf("%w: ohne echte antwort keine karten", ErrFalscherZustand)
	}
	if len(faelschungen) != 3 {
		return fmt.Errorf("brauche genau 3 fälschungen, habe %d", len(faelschungen))
	}
	karten := []Karte{{Text: a.Normalform, IstEcht: true}}
	for _, f := range faelschungen {
		karten = append(karten, Karte{Text: f.Text, AnkerTag: f.AnkerTag})
	}
	rng.Shuffle(len(karten), func(i, j int) { karten[i], karten[j] = karten[j], karten[i] })
	for i := range karten {
		karten[i].Pos = i + 1
	}
	if r.Karten == nil {
		r.Karten = map[PlayerID][]Karte{}
	}
	r.Karten[spieler] = karten
	return nil
}

// TippAbgeben verbucht die Wahl eines Spielers über den jeweils anderen und
// sagt, ob die Runde damit endet.
func (r *Runde) TippAbgeben(p Party, rater PlayerID, pos int) (fertig bool, err error) {
	if !p.Mitglied(rater) {
		return false, ErrKeinMitglied
	}
	if z := r.Ableiten(p); z != Raten && z != RatenWartet {
		return false, fmt.Errorf("%w: %s", ErrFalscherZustand, z)
	}
	if r.hatTipp(rater) {
		return false, ErrSchonGetippt
	}
	ziel := p.Gegner(rater) // geraten wird über den anderen
	if pos < 1 || pos > len(r.Karten[ziel]) {
		return false, ErrKartePos
	}
	if r.Tipps == nil {
		r.Tipps = map[PlayerID]Tipp{}
	}
	r.Tipps[rater] = Tipp{Gewaehlt: pos, Richtig: pos == r.EchteKarte(ziel)}
	return r.Ableiten(p) == Aufgeloest, nil
}

// KartenFuer liefert die Karten, die einem Spieler vorgelegt werden – also die
// über seinen Gegenüber. Solange die Runde nicht in der Ratephase ist, gibt es
// nichts zu sehen.
func (r Runde) KartenFuer(p Party, betrachter PlayerID) []Karte {
	switch r.Ableiten(p) {
	case Raten, RatenWartet, Aufgeloest:
	default:
		return nil
	}
	return r.Karten[p.Gegner(betrachter)]
}
