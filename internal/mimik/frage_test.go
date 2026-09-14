package mimik

import (
	"math"
	"testing"
)

// Die drei Doppel, die am 14.09.2026 im Bestand standen. Erwartet sind die
// gemessenen Werte, nicht "irgendwas ueber der Schwelle": Aendert jemand
// fragerahmen, soll der Test sagen, WIE weit sich das Mass verschoben hat.
func TestFragennaeheFaengtDieDoppel(t *testing.T) {
	faelle := []struct {
		a, b string
		will float64
	}{
		{
			"Was möchtest du in fünf Jahren können, das du heute nicht kannst?",
			"Was willst du in fünf Jahren können, was du heute nicht kannst?",
			1.00,
		},
		{
			"Welche Aufgabe im Haushalt schiebst du am längsten vor dir her?",
			"Welche Aufgabe schiebst du gerade vor dir her?",
			0.60,
		},
		{
			"Welche Jahreszeit passt am wenigsten zu dir?",
			"Welche Jahreszeit passt am besten zu dir und warum?",
			0.50,
		},
	}
	for _, f := range faelle {
		n := Fragennaehe(f.a, f.b)
		if math.Abs(n-f.will) > 0.005 {
			t.Errorf("Fragennaehe(%q, %q) = %.4f, erwartet %.2f", f.a, f.b, n, f.will)
		}
		if n < FrageDoppel {
			t.Errorf("%.4f liegt unter FrageDoppel %.2f - das Doppel geht durch:\n  %s\n  %s",
				n, FrageDoppel, f.a, f.b)
		}
	}
}

// Die Gegenprobe, und der eigentliche Grund fuer ein eigenes Mass: Diese Paare
// liegen beim n-Gramm-Mass bei 0.48 und 0.38 - also HOEHER als das echte Doppel
// oben (0.44). Ueber die Kerne muessen sie unter FrageNachbar bleiben.
func TestFragennaeheOhneFehlalarm(t *testing.T) {
	faelle := []struct {
		a, b string
		will float64
	}{
		{
			"Welches Geräusch magst du, obwohl die meisten es nicht mögen?",
			"Welches Wetter magst du, obwohl die meisten es hassen?",
			0.20,
		},
		{
			"Was würdest du sagen, wenn niemand beleidigt sein könnte?",
			"Was würdest du tun, wenn dir niemand zusehen könnte?",
			0.00,
		},
		{
			"Was würdest du an einem völlig freien Samstag tun, wenn niemand etwas von dir will?",
			"Was würdest du tun, wenn dir niemand zusehen könnte?",
			0.00,
		},
	}
	for _, f := range faelle {
		n := Fragennaehe(f.a, f.b)
		if math.Abs(n-f.will) > 0.005 {
			t.Errorf("Fragennaehe(%q, %q) = %.4f, erwartet %.2f", f.a, f.b, n, f.will)
		}
		if n >= FrageNachbar {
			t.Errorf("Fehlalarm bei %.4f (>= FrageNachbar %.2f):\n  %s\n  %s",
				n, FrageNachbar, f.a, f.b)
		}
	}
}

// Das Umweg-Paar ist der Grund fuer zwei Stufen statt einer Schwelle: Es liegt
// im Warnband, weil es sich lohnt, hinzusehen - aber es sind zwei verschiedene
// Fragen und es darf nicht blockieren.
func TestUmwegBleibtImWarnband(t *testing.T) {
	n := Fragennaehe(
		"Welchen Umweg nimmst du gern in Kauf?",
		"Welchen Umweg nimmst du, um jemandem nicht zu begegnen?")
	if n < FrageNachbar || n >= FrageDoppel {
		t.Errorf("Umweg-Paar liegt bei %.4f, soll zwischen %.2f und %.2f liegen",
			n, FrageNachbar, FrageDoppel)
	}
}

func TestFragennaeheIstSymmetrischUndMitSichSelbstEins(t *testing.T) {
	a := "Was hebst du auf, obwohl es kaputt ist?"
	b := "Welchen Umzug wirst du nicht vergessen?"
	if Fragennaehe(a, b) != Fragennaehe(b, a) {
		t.Error("nicht symmetrisch")
	}
	if n := Fragennaehe(a, a); n != 1 {
		t.Errorf("mit sich selbst %.4f, erwartet 1", n)
	}
}

func TestFragenkernStreichtDenRahmen(t *testing.T) {
	kern := Fragenkern("Was würdest du an einem Tag machen, an dem du unsichtbar wärst?")
	will := []string{"tag", "unsichtbar"}
	if len(kern) != len(will) {
		t.Fatalf("Kern %v, erwartet %v", kern, will)
	}
	for _, w := range will {
		if _, ok := kern[w]; !ok {
			t.Errorf("%q fehlt im Kern %v", w, kern)
		}
	}
}

// Eine Frage, die nur aus Rahmen besteht, hat keinen Kern - und darf dann kein
// Urteil erzeugen, sondern null.
func TestFragenkernOhneInhaltSchweigt(t *testing.T) {
	if n := Fragennaehe("Was machst du?", "Was willst du?"); n != 0 {
		t.Errorf("zwei rahmenlose Fragen ergeben %.4f, erwartet 0", n)
	}
}

// Die Entscheidung gegen ein MinKern: Eine Frage mit EINEM Inhaltswort wird
// gepruefft, nicht uebersprungen. Sonst haetten sich sechs der 144 Fragen jeder
// Pruefung entzogen.
func TestEinWortKernWirdGeprueft(t *testing.T) {
	n := Fragennaehe(
		"Was verzeihst du sofort, was nie?",
		"Wem verzeihst du am schwersten?")
	if n < FrageDoppel {
		t.Errorf("Ein-Wort-Kern gegen seine Variante ergibt %.4f, soll >= %.2f sein",
			n, FrageDoppel)
	}
}

func TestFragendoppelNenntDasGemeinsameUndSortiertFest(t *testing.T) {
	fragen := []string{
		"Welches Buch hast du mehr als einmal gelesen?",
		"Welche Jahreszeit passt am wenigsten zu dir?",
		"Welche Jahreszeit passt am besten zu dir und warum?",
	}
	got := Fragendoppel(fragen, FrageNachbar)
	if len(got) != 1 {
		t.Fatalf("%d Paare, erwartet 1: %+v", len(got), got)
	}
	if got[0].A != 1 || got[0].B != 2 {
		t.Errorf("Paar (%d,%d), erwartet (1,2)", got[0].A, got[0].B)
	}
	if len(got[0].Gemeinsam) != 2 ||
		got[0].Gemeinsam[0] != "jahreszeit" || got[0].Gemeinsam[1] != "passt" {
		t.Errorf("Gemeinsam = %v, erwartet [jahreszeit passt]", got[0].Gemeinsam)
	}
	// Zweimal gerechnet ergibt dieselbe Reihenfolge - Map-Iteration darf nicht
	// durchschlagen.
	for i := 0; i < 20; i++ {
		wieder := Fragendoppel(fragen, FrageNachbar)
		if wieder[0].A != got[0].A || wieder[0].B != got[0].B {
			t.Fatal("Reihenfolge flackert")
		}
	}
}
