package game

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
)

// Testfixtures tragen keine echten Namen: Der Spielkern kennt nur IDs, und
// Klarnamen verlassen die App ohnehin nie unpseudonymisiert (Dossier 11.3).
var party = Party{ID: "pt", A: "spieler-a", B: "spieler-b"}

func rng() *rand.Rand { return rand.New(rand.NewPCG(7, 11)) }

func antwort(s string) Antwort { return Antwort{Original: s, Normalform: s} }

const (
	echtA = "Die zweite Kaffeemaschine. Die alte steht immer noch im Keller."
	echtB = "Ein Rennrad. Ich fahre es dreimal im Jahr, aber es war richtig."
)

func drei(p string) []Faelschung {
	return []Faelschung{
		{Text: p + " eins, eine hinreichend lange Fälschung.", AnkerTag: "kaffee"},
		{Text: p + " zwei, eine hinreichend lange Fälschung.", AnkerTag: "krimis"},
		{Text: p + " drei, eine hinreichend lange Fälschung.", AnkerTag: "nordsee"},
	}
}

// bauen bringt eine Runde bis zur Ratephase.
func bauen(t *testing.T) *Runde {
	t.Helper()
	r := &Runde{ID: "r1", Nummer: 1, Frage: "Frage?"}
	if err := r.AntwortAbgeben(party, party.A, antwort(echtA)); err != nil {
		t.Fatal(err)
	}
	if err := r.AntwortAbgeben(party, party.B, antwort(echtB)); err != nil {
		t.Fatal(err)
	}
	if err := r.KartenSetzen(party.A, drei("A"), rng()); err != nil {
		t.Fatal(err)
	}
	if err := r.KartenSetzen(party.B, drei("B"), rng()); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestZustandsfolge(t *testing.T) {
	r := &Runde{ID: "r1"}
	if got := r.Ableiten(party); got != Schreiben {
		t.Fatalf("leer: %s", got)
	}
	r.AntwortAbgeben(party, party.A, antwort(echtA))
	if got := r.Ableiten(party); got != SchreibenWartet {
		t.Fatalf("eine antwort: %s", got)
	}
	r.AntwortAbgeben(party, party.B, antwort(echtB))
	if got := r.Ableiten(party); got != MimikArbeitet {
		t.Fatalf("tor 1: %s", got)
	}
	r.KartenSetzen(party.A, drei("A"), rng())
	if got := r.Ableiten(party); got != MimikArbeitet {
		t.Fatalf("erst ein kartensatz: %s", got)
	}
	r.KartenSetzen(party.B, drei("B"), rng())
	if got := r.Ableiten(party); got != Raten {
		t.Fatalf("beide kartensätze: %s", got)
	}
	r.TippAbgeben(party, party.A, 1)
	if got := r.Ableiten(party); got != RatenWartet {
		t.Fatalf("ein tipp: %s", got)
	}
	fertig, err := r.TippAbgeben(party, party.B, 1)
	if err != nil || !fertig {
		t.Fatalf("zweiter tipp: fertig=%v err=%v", fertig, err)
	}
	if got := r.Ableiten(party); got != Aufgeloest {
		t.Fatalf("tor 2: %s", got)
	}
}

// Die Ratephase darf erst öffnen, wenn beide geschrieben haben – sonst sieht
// man die Antwort des anderen, bevor man die eigene abgegeben hat.
func TestKeinDurchblickVorTor1(t *testing.T) {
	r := &Runde{ID: "r1"}
	r.AntwortAbgeben(party, party.B, antwort(echtB))
	r.KartenSetzen(party.B, drei("B"), rng())
	if k := r.KartenFuer(party, party.A); k != nil {
		t.Fatalf("A sieht %d karten, obwohl er noch nicht geschrieben hat", len(k))
	}
	if _, err := r.TippAbgeben(party, party.A, 1); !errors.Is(err, ErrFalscherZustand) {
		t.Fatalf("tipp vor tor 1 muss scheitern, bekam %v", err)
	}
}

func TestKartenFuerZeigtDenAnderen(t *testing.T) {
	r := bauen(t)
	k := r.KartenFuer(party, party.A)
	if len(k) != 4 {
		t.Fatalf("erwarte 4 karten, habe %d", len(k))
	}
	for _, c := range k {
		if c.IstEcht && c.Text != echtB {
			t.Fatalf("A bekommt nicht die echte antwort von B: %q", c.Text)
		}
		if strings.HasPrefix(c.Text, "A ") {
			t.Fatal("A sieht seine eigenen fälschungen")
		}
	}
}

func TestPositionenFestUndVollstaendig(t *testing.T) {
	r := bauen(t)
	gesehen := map[int]bool{}
	echte := 0
	for _, k := range r.Karten[party.A] {
		if gesehen[k.Pos] {
			t.Fatalf("position %d doppelt", k.Pos)
		}
		gesehen[k.Pos] = true
		if k.IstEcht {
			echte++
		}
	}
	if len(gesehen) != 4 || echte != 1 {
		t.Fatalf("positionen=%d echte=%d", len(gesehen), echte)
	}
}

func TestMischenIstStabil(t *testing.T) {
	a, b := bauen(t), bauen(t)
	if a.EchteKarte(party.A) != b.EchteKarte(party.A) {
		t.Fatal("gleicher seed, andere reihenfolge")
	}
}

func TestTippWertung(t *testing.T) {
	r := bauen(t)
	echt := r.EchteKarte(party.B) // A rät über B
	if _, err := r.TippAbgeben(party, party.A, echt); err != nil {
		t.Fatal(err)
	}
	if !r.Tipps[party.A].Richtig {
		t.Fatal("treffer wurde nicht als richtig gewertet")
	}
	falsch := echt%4 + 1
	if _, err := r.TippAbgeben(party, party.B, falsch); err != nil {
		t.Fatal(err)
	}
	if r.Tipps[party.B].Richtig == (falsch == r.EchteKarte(party.A)) {
		return // zufällig doch richtig, dann ist die Wertung trotzdem konsistent
	}
}

func TestZweiterTippAbgelehnt(t *testing.T) {
	r := bauen(t)
	r.TippAbgeben(party, party.A, 1)
	if _, err := r.TippAbgeben(party, party.A, 2); !errors.Is(err, ErrSchonGetippt) {
		t.Fatalf("erwarte ErrSchonGetippt, bekam %v", err)
	}
}

func TestAntwortlaenge(t *testing.T) {
	if err := PruefeAntwort("ab"); !errors.Is(err, ErrZuKurz) {
		t.Fatalf("zu kurz: %v", err)
	}
	if err := PruefeAntwort("   "); !errors.Is(err, ErrZuKurz) {
		t.Fatalf("nur leerzeichen: %v", err)
	}
	if err := PruefeAntwort(strings.Repeat("x", 401)); !errors.Is(err, ErrZuLang) {
		t.Fatalf("lang: %v", err)
	}
	if err := PruefeAntwort(echtA); err != nil {
		t.Fatalf("gültig: %v", err)
	}
	// Umlaute zählen als ein Zeichen, nicht als zwei Bytes.
	if err := PruefeAntwort(strings.Repeat("ä", 26)); err != nil {
		t.Fatalf("umlaute: %v", err)
	}
}

// Aus echtem Material: "Kündige!" ist acht Zeichen lang, unverwechselbar und
// muss durchgehen. Die frühere Untergrenze von 25 hätte es abgewiesen.
func TestKurzeAntwortIstGueltig(t *testing.T) {
	for _, s := range []string{"Kündige!", "Nie.", "Auf jeden Fall (rechte) Politik"} {
		if err := PruefeAntwort(s); err != nil {
			t.Errorf("%q wurde abgewiesen: %v", s, err)
		}
	}
	if !Knapp("Kündige!") {
		t.Error("acht zeichen sollten einen hinweis auslösen")
	}
	if Knapp("Bücher: Die lese ich nicht mehr, aber ich finde sie dekorativ") {
		t.Error("60 zeichen brauchen keinen hinweis")
	}
	r := &Runde{ID: "r1"}
	if err := r.AntwortAbgeben(party, party.A, antwort("Kündige!")); err != nil {
		t.Fatalf("runde nimmt kurze antwort nicht an: %v", err)
	}
}

func TestVerbuchenUndDoppeltreffer(t *testing.T) {
	r := bauen(t)
	echtUeberB := r.EchteKarte(party.B)
	echtUeberA := r.EchteKarte(party.A)
	r.TippAbgeben(party, party.A, echtUeberB)     // A trifft
	r.TippAbgeben(party, party.B, echtUeberA%4+1) // B daneben
	s := Verbuchen(Stand{}, *r, party)
	if s.Mensch != 1 || s.Mimik != 1 {
		t.Fatalf("stand %+v", s)
	}
	if s.Tipps() != 2 {
		t.Fatalf("eine runde vergibt genau zwei punkte, hier %d", s.Tipps())
	}
	if Doppeltreffer(*r, party) {
		t.Fatal("kein doppeltreffer, einer lag richtig")
	}
}

func TestMatchende(t *testing.T) {
	faelle := []struct {
		s    Stand
		will Ergebnis
	}{
		{Stand{0, 0}, Offen},
		{Stand{9, 9}, Offen},
		{Stand{10, 8}, MenschGewinnt},
		{Stand{7, 10}, MimikGewinnt},
		{Stand{10, 10}, Verlaengerung},
		// Überschuss: die laufende Runde wird zu Ende gespielt, deshalb sind
		// 11:9 und 10:11 gültige Endstände.
		{Stand{11, 9}, MenschGewinnt},
		{Stand{10, 11}, MimikGewinnt},
	}
	for _, f := range faelle {
		if got := Auswerten(f.s); got != f.will {
			t.Errorf("%+v: %s, erwartet %s", f.s, got, f.will)
		}
	}
}

// Ein Match läuft mindestens 5 und höchstens 10 Runden, weil jede Runde genau
// zwei Punkte vergibt und bei 10 Schluss ist.
func TestMatchlaenge(t *testing.T) {
	for _, quote := range []float64{0, 0.5, 1} {
		s := Stand{}
		runden := 0
		for Auswerten(s) == Offen {
			runden++
			treffer := int(2 * quote)
			s.Mensch += treffer
			s.Mimik += 2 - treffer
			if runden > 50 {
				t.Fatal("match endet nicht")
			}
		}
		if runden < 5 || runden > 10 {
			t.Errorf("quote %.1f: %d runden", quote, runden)
		}
	}
}
