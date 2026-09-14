package mimik

import "testing"

// Wer begruendet, konstruiert: Eine erfundene Erinnerung traegt ihre Herleitung
// mit, eine echte nicht. Eine Begruendung unter drei Karten ist erlaubt - auch
// Menschen begruenden gelegentlich, und drei Karten ohne einen einzigen
// Nebensatz sind als Satz genauso auffaellig.
func TestKausalbruch(t *testing.T) {
	behauptend := []string{
		"Vor dem Staubsauger, ich bin immer weggerannt.",
		"Vor dem Nachbarshund, der hat mich einmal angesprungen.",
		"Vor dem dunklen Keller.",
	}
	if b := Kausalbruch(behauptend); len(b) != 0 {
		t.Fatalf("behauptende karten abgewiesen: %v", b)
	}
	eine := append([]string{}, behauptend...)
	eine[0] = "Vor dem Staubsauger, weil das Geräusch mich erschreckt hat."
	if b := Kausalbruch(eine); len(b) != 0 {
		t.Fatalf("eine begründung ist erlaubt, abgewiesen: %v", b)
	}
	zwei := append([]string{}, eine...)
	zwei[1] = "Vor dem Hund, deshalb bin ich außen herum gelaufen."
	if b := Kausalbruch(zwei); len(b) != 2 {
		t.Fatalf("zwei begründungen gingen durch: %v", b)
	}
	// Kein Treffer mitten im Wort: "Weiler", "Denner", "umzu".
	for _, x := range []string{"Im Weiler bei meiner Oma.", "Beim Denner einkaufen."} {
		if Kausal(x) {
			t.Fatalf("falscher treffer in %q", x)
		}
	}
}

// Was nur EINE der vier Karten hat, verraet sie - und die anderen drei mit.
func TestSatzbaubruch(t *testing.T) {
	echt := "Mit meinem Bruder ums Aufräumen der Garage."
	ok := []string{
		"Mit meiner Mitbewohnerin wegen des Küchenregals.",
		"Um den Sitzplatz im Zug.",
		"Mit meiner Mutter beim Ausmisten.",
	}
	if b := Satzbaubruch(echt, ok); len(b) != 0 {
		t.Fatalf("gleicher bau abgewiesen: %v", b)
	}
	// Drei Sätze gegen einen, und vier Kommas gegen keins.
	fremd := []string{
		"Mit meiner Mutter. Sie hat ausgemistet. Ich war nicht gefragt.",
		"Ach, um so einen Blödsinn, den Waschlappen, wer ihn aufhängt, wer nicht.",
	}
	if b := Satzbaubruch(echt, fremd); len(b) != 2 {
		t.Fatalf("fremder bau ging durch: %v", b)
	}
}

// Nur bewertende Schlussfiguren, keine Partikeln: "eben" und "halt" sind das
// Gegenteil eines Verdachts, naemlich gesprochene Sprache.
func TestFloskelbruch(t *testing.T) {
	if b := Floskelbruch([]string{"Ist eben so, ich räume halt trotzdem auf."}); len(b) != 0 {
		t.Fatalf("partikeln als floskel gewertet: %v", b)
	}
	if b := Floskelbruch([]string{"War teuer, aber am Ende zählt, dass es Freude macht."}); len(b) != 1 {
		t.Fatal("das fazit am schluss ging durch")
	}
}

// Stilbruch zaehlt zusammen und nennt den auffaelligsten Grund - die Zahl
// entscheidet nach drei Versuchen, welcher Satz notgedrungen hinausgeht.
func TestStilbruchZaehltUndBenennt(t *testing.T) {
	echt := "Nicht mit vollem Mund reden, das nervt wirklich."
	gut := []string{
		"Erst aufessen, dann aufstehen.",
		"Hände waschen, bevor es Essen gibt.",
		"Wir mussten immer fragen, bevor wir vom Tisch aufstanden.",
	}
	if n, grund := Stilbruch(echt, gut); n != 0 || grund != "" {
		t.Fatalf("brauchbare karten: %d verstöße, grund %q", n, grund)
	}
	zulang := []string{
		"Beim Kochen die Töpfe nicht bis zum Rand füllen, das habe ich früher " +
			"für Schikane gehalten und inzwischen zweimal eine Herdplatte geputzt.",
	}
	n, grund := Stilbruch(echt, zulang)
	if n == 0 || grund != "Länge" {
		t.Fatalf("%d verstöße, grund %q - erwartet Länge", n, grund)
	}
}

// Der Fall, der die Pruefung ausgeloest hat: Am 14.09.2026 kam gegen "Eine
// zweite Kaffeemuehle, die erste mahlt zu grob" die Faelschung "Eine zweite
// Fahrkartenhuelle, die erste ist noch voellig in Ordnung". Die n-Gramm-
// Aehnlichkeit lag bei 0.22 - weit unter jeder Schwelle, weil die
// Inhaltswoerter verschieden sind. Uebernommen war der BAU.
func TestGerippebruchFaengtDieAbwandlung(t *testing.T) {
	echt := "Eine zweite Kaffeemühle, die erste mahlt zu grob."
	abwandlung := "Eine zweite Fahrkartenhülle, die erste ist noch völlig in Ordnung."
	if s := Aehnlichkeit(echt, abwandlung); s >= SimMaxEcht {
		t.Fatalf("der fall braucht die neue prüfung nicht, sim=%.2f", s)
	}
	if b := Gerippebruch(echt, []string{abwandlung}); len(b) != 1 {
		t.Fatalf("die abwandlung ging durch (gerippe %v gegen %v)",
			Gerippe(echt), Gerippe(abwandlung))
	}
}

// Und der wichtigere Teil: Karten, die am 14.09.2026 tatsaechlich erzeugt
// wurden und in Ordnung waren, duerfen NICHT anschlagen. Eine Pruefung, die
// unschuldige Karten trifft, kostet einen ganzen Modellaufruf je Fehlalarm.
func TestGerippebruchOhneFehlalarm(t *testing.T) {
	faelle := []struct {
		echt  string
		fakes []string
	}{
		{
			"Mit meinem Bruder ums Aufräumen der Garage.",
			[]string{
				"Mit meiner Mutter, sie ruft immer sonntags an und ich hab nicht abgenommen.",
				"Beim Kartenspielen, jemand hat gemogelt und ich habe es laut gesagt.",
				"Mit meinem Nachbarn über den Balkon, es ging um Ranken an der Trennwand.",
			},
		},
		{
			"Vor dem Staubsauger, ich bin immer weggerannt.",
			[]string{
				"Vor der Kinderärztin, im Wartezimmer war es immer so still.",
				"Vor Gewittern, ich habe die Decke über den Kopf gezogen.",
				"Vor dem Keller, ich habe mich nie alleine runtergetraut.",
			},
		},
		{
			"Nicht mit vollem Mund reden, das nervt wirklich.",
			[]string{
				"Beim Essen bleibt das Handy weg.",
				"Nichts stehen lassen, immer gleich abwaschen.",
				"Erst zuhören, dann antworten.",
			},
		},
		{
			"Den kleinen Schirm.",
			[]string{"Die Kaffeetasse vom Vortag.", "Ein Kartenspiel, für die Zugfahrt."},
		},
	}
	for _, f := range faelle {
		if b := Gerippebruch(f.echt, f.fakes); len(b) != 0 {
			t.Errorf("fehlalarm bei %q: %v (gerippe echt %v, fake %v)",
				f.echt, b, Gerippe(f.echt), Gerippe(f.fakes[b[0]]))
		}
	}
}

// Unter drei Gerippewoertern sagt die Pruefung nichts: "Kaffee." hat keinen
// Bau, den man kopieren koennte.
func TestGerippeZuKurzSchweigt(t *testing.T) {
	if b := Gerippebruch("Fenster auf.", []string{"Kaffee.", "Aufs Handy."}); len(b) != 0 {
		t.Fatalf("kurze karten angeschlagen: %v", b)
	}
}
