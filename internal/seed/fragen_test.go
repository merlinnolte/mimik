package seed_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"mimik/internal/mimik"
	"mimik/internal/seed"
)

// Die Pruefung des Fragenvorrats.
//
// Sie steht hier und nicht bei den Maßen, weil sie den GEGENSTAND huetet: Wer
// fragen.json aendert, bekommt den Fehlschlag in dem Paket, in dem er gerade
// arbeitet. Als externes Testpaket (seed_test), damit internal/seed selbst ein
// reines Datenpaket bleibt und weiter nur embed und encoding/json kennt.

// rubriken sind die erlaubten Werte. Der Wert wird im Go- und Kotlin-Code
// nirgends ausgewertet - er landet nur in rounds.rubrik und bestimmt damit nur,
// welche Mischung ein Match bekommt. Die Liste steht trotzdem hier: Ein
// Tippfehler ("alltagg") waere sonst unbemerkt eine siebte Rubrik mit einer
// Frage darin.
var rubriken = map[string]bool{
	"alltag": true, "haltung": true, "vergangenheit": true, "vorliebe": true,
	"hypothetisch": true, "zukunft": true, "menschen": true, "koerper": true,
}

// MindestVorrat: Ein Mensch verbraucht je Match bis zu 2*RundenProMatch = 12
// Fragen - sechs werden beim Anlegen gezogen, bis zu sechs weitere nachgelegt.
// Zwanzig Matches je Mensch ist die untere Grenze, unter der der Vorrat nicht
// bloss knapp, sondern kaputt ist.
//
// Absichtlich nicht der heutige Bestand (358): Ein Test, der nachspricht, was
// gerade im Verzeichnis liegt, prueft nichts. Aber auch nicht so tief, dass er
// alles durchlaesst - 240 laesst Luft fuer Ruecknahmen und faengt den Fall, dass
// jemand die halbe Datei verliert.
const MindestVorrat = 240

// MaxSpreizung: Die groesste Rubrik darf nicht mehr als dreimal so viele Fragen
// haben wie die kleinste.
//
// Eine Obergrenze statt einer festen Quote: Gezogen wird gleichverteilt ueber
// den ganzen Vorrat, also ist die Rubrikverteilung die Mischung, die ein Match
// zu sehen bekommt. Eine exakte Quote waere eine Regel, die die erste gute
// Frage bricht, die nicht ins Fach passt; eine Spreizung faengt trotzdem den
// Stapel, der eine Rubrik vergisst.
const MaxSpreizung = 3

func texte(f []seed.Frage) []string {
	out := make([]string, len(f))
	for i, x := range f {
		out[i] = x.Text
	}
	return out
}

// falten ist eine Normalform nur fuer diesen Test: kleingeschrieben, alles
// außer Buchstaben und Ziffern zu einem Leerzeichen. Hier lokal und nicht aus
// internal/mimik geholt, damit dafuer nichts exportiert werden muss.
func falten(s string) string {
	var b strings.Builder
	raum := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			raum = false
		case !raum:
			b.WriteRune(' ')
			raum = true
		}
	}
	return strings.TrimSpace(b.String())
}

// Die Kennung ist die Identitaet, an der fragen_vergeben haengt. Ein Doppel
// darin waere schlimmer als ein Doppel im Text: Zwei Fragen teilten sich eine
// id, und wer die eine beantwortet hat, bekaeme die andere nie zu sehen.
var kennungForm = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func TestJedeFrageHatEineEigeneKennung(t *testing.T) {
	gesehen := map[string]string{}
	for _, f := range seed.Fragen() {
		if !kennungForm.MatchString(f.Kennung) {
			t.Errorf("Kennung %q passt nicht auf %s (Frage %q)",
				f.Kennung, kennungForm, f.Text)
		}
		if alt, schon := gesehen[f.Kennung]; schon {
			t.Errorf("Kennung %q zweimal:\n  %s\n  %s", f.Kennung, alt, f.Text)
		}
		gesehen[f.Kennung] = f.Text
	}
}

func TestKeinTextDoppeltSichWoertlich(t *testing.T) {
	gesehen := map[string]int{}
	for i, f := range seed.Fragen() {
		if j, schon := gesehen[f.Text]; schon {
			t.Errorf("Frage %d und %d sind derselbe Text: %q", j, i, f.Text)
		}
		gesehen[f.Text] = i
	}
}

// Dieselbe Frage mit einem anderen Gedankenstrich oder einem Leerzeichen mehr
// ist fuer fragen_pool.text UNIQUE eine zweite Frage - und wird gezogen.
func TestKeinTextDoppeltSichNachFalten(t *testing.T) {
	gesehen := map[string]string{}
	for _, f := range seed.Fragen() {
		k := falten(f.Text)
		if alt, schon := gesehen[k]; schon {
			t.Errorf("gleich nach Falten:\n  %s\n  %s", alt, f.Text)
		}
		gesehen[k] = f.Text
	}
}

// Der eigentliche Waechter: zwei verschiedene Texte, die dieselbe Frage stellen.
//
// Das hebelt fragen_vergeben aus - zwei Zeilen mit verschiedener ID gelten als
// verschiedene Fragen, und ein Mensch bekommt beide.
func TestKeineFrageStelltDasselbe(t *testing.T) {
	fr := seed.Fragen()
	paare := mimik.Fragendoppel(texte(fr), mimik.FrageNachbar)
	for _, p := range paare {
		zeile := func() string {
			return "  " + fr[p.A].Text + "  [" + fr[p.A].Rubrik + "]\n" +
				"  " + fr[p.B].Text + "  [" + fr[p.B].Rubrik + "]\n" +
				"  gemeinsam: " + strings.Join(p.Gemeinsam, " ")
		}
		if p.Naehe >= mimik.FrageDoppel {
			t.Errorf("Doppel bei %.2f:\n%s", p.Naehe, zeile())
			continue
		}
		t.Logf("Nachbarn bei %.2f:\n%s", p.Naehe, zeile())
	}
}

// Fragen mit einem einzigen Inhaltswort kollidieren mit allem, was dieses Wort
// benutzt. Kein Fehler, aber gut zu wissen, bevor man eine neue Frage schreibt.
func TestDuenneKerneNennen(t *testing.T) {
	for _, f := range seed.Fragen() {
		if k := mimik.Fragenkern(f.Text); len(k) < 2 {
			schl := make([]string, 0, len(k))
			for w := range k {
				schl = append(schl, w)
			}
			sort.Strings(schl)
			t.Logf("Kern zu duenn (%v), kollidiert mit allem darueber: %s",
				schl, f.Text)
		}
	}
}

// Form: gemessen am Bestand vom 14.09.2026 - kuerzeste Frage 30 Zeichen,
// laengste 100, alle enden auf ein Fragezeichen. Die Grenzen lassen zwei
// Zeichen Luft und keine mehr.
//
// Was hier absichtlich NICHT geprueft wird: "spricht mit du". Eine gute Frage
// im Bestand tut es nicht ("Was soll in zehn Jahren noch genauso sein wie
// heute?"). Die Regel gehoert in den Filter fuer neue Vorschlaege, nicht in
// eine Pruefung ueber den Bestand - sonst ist der Test am ersten Tag rot und
// wird nachgegeben statt befolgt.
func TestJedeFrageHatDieFormDesBestands(t *testing.T) {
	for _, f := range seed.Fragen() {
		if !strings.HasSuffix(f.Text, "?") {
			t.Errorf("endet nicht auf ?: %q", f.Text)
		}
		if n := utf8.RuneCountInString(f.Text); n < 28 || n > 105 {
			t.Errorf("%d Zeichen (erlaubt 28-105): %q", n, f.Text)
		}
		if strings.Contains(strings.ToLower(f.Text), "warum") {
			t.Errorf("verlangt eine Begruendung, die MIMIK nachmachen muss "+
				"(MaxKausal = 1): %q", f.Text)
		}
	}
}

func TestVorratTraegtGenugMatches(t *testing.T) {
	if n := len(seed.Fragen()); n < MindestVorrat {
		t.Errorf("%d Fragen, mindestens %d", n, MindestVorrat)
	}
}

func TestRubrikenSindBekanntUndNichtSchief(t *testing.T) {
	zahl := map[string]int{}
	for _, f := range seed.Fragen() {
		if !rubriken[f.Rubrik] {
			t.Errorf("unbekannte Rubrik %q bei %q", f.Rubrik, f.Text)
			continue
		}
		zahl[f.Rubrik]++
	}
	if len(zahl) == 0 {
		t.Fatal("keine Rubrik gezaehlt")
	}
	min, max := 1<<30, 0
	var minName, maxName string
	for r, n := range zahl {
		if n < min {
			min, minName = n, r
		}
		if n > max {
			max, maxName = n, r
		}
	}
	t.Logf("Rubriken: %v", zahl)
	if max > min*MaxSpreizung {
		t.Errorf("%s hat %d Fragen, %s nur %d - mehr als %dx",
			maxName, max, minName, min, MaxSpreizung)
	}
}
