package mimik

import "testing"

func merkmal(k, wert string, konf float64, belege int) Merkmal {
	return Merkmal{Merkmal: k, Wert: wert, Konfidenz: konf, Belege: belege}
}

// Ein einzelner Beleg macht nichts sicher, egal was das Modell vorschlaegt.
// Vier Saetze sind kein Beweis.
func TestEinBelegBleibtVermutung(t *testing.T) {
	a := ProfilVerrechnen(nil, []Urteil{{
		Beobachtung: "sagt, ihre Schwester habe den Tisch gebaut",
		Merkmal:     "Geschwister?", Wert: "hat eine Schwester",
		Urteil: "NEU", Konfidenz: 0.95,
	}}, "r1")
	if len(a) != 1 {
		t.Fatalf("%d änderungen statt 1", len(a))
	}
	if a[0].Merkmal.Merkmal != "geschwister" {
		t.Fatalf("schlüssel %q nicht normalisiert", a[0].Merkmal.Merkmal)
	}
	if a[0].Merkmal.Konfidenz > ProfilStart {
		t.Fatalf("konfidenz %.2f über dem deckel %.2f", a[0].Merkmal.Konfidenz, ProfilStart)
	}
}

// Zustimmung steigt in kleinen Schritten, sonst genügen zwei gefällige Runden
// für eine Gewissheit.
func TestBestaetigenSteigtLangsam(t *testing.T) {
	alt := []Merkmal{merkmal("alter", "Mitte dreißig", 0.4, 1)}
	a := ProfilVerrechnen(alt, []Urteil{{
		Merkmal: "alter", Wert: "Mitte dreißig", Urteil: "BESTAETIGT", Konfidenz: 1.0,
	}}, "r2")
	if len(a) != 1 {
		t.Fatalf("%d änderungen", len(a))
	}
	if got := a[0].Merkmal.Konfidenz; got > 0.4+ProfilSchritt+0.001 {
		t.Fatalf("konfidenz springt von 0.40 auf %.2f", got)
	}
	if a[0].Merkmal.Belege != 2 {
		t.Fatalf("%d belege statt 2", a[0].Merkmal.Belege)
	}
	// Und niemals 1.0.
	hoch := ProfilVerrechnen([]Merkmal{merkmal("alter", "x", 0.9, 5)},
		[]Urteil{{Merkmal: "alter", Wert: "x", Urteil: "BESTAETIGT", Konfidenz: 1.0}}, "r3")
	if hoch[0].Merkmal.Konfidenz > ProfilKappe {
		t.Fatalf("konfidenz %.2f über der kappe", hoch[0].Merkmal.Konfidenz)
	}
}

// Widerspruch ist teurer als Zustimmung: Ein falsches Merkmal steht sonst in
// jeder kommenden Fälschung.
func TestWiderspruchHalbiert(t *testing.T) {
	alt := []Merkmal{merkmal("wohnform", "wohnt allein", 0.8, 3)}
	// Schwacher Gegenvorschlag: Der alte Wert bleibt, aber die Sicherheit fällt.
	a := ProfilVerrechnen(alt, []Urteil{{
		Merkmal: "wohnform", Wert: "wohnt zu zweit", Urteil: "REVIDIERT", Konfidenz: 0.3,
	}}, "r2")
	if a[0].Merkmal.Wert != "wohnt allein" {
		t.Fatalf("schwacher gegenvorschlag hat den wert übernommen: %q", a[0].Merkmal.Wert)
	}
	if got := a[0].Merkmal.Konfidenz; got > 0.41 || got < 0.39 {
		t.Fatalf("konfidenz %.2f statt der halbierten 0.40", got)
	}
	// Starker Gegenvorschlag: Der neue Wert setzt sich durch, aber nur als
	// frische Vermutung.
	b := ProfilVerrechnen(alt, []Urteil{{
		Merkmal: "wohnform", Wert: "wohnt zu zweit", Urteil: "REVIDIERT", Konfidenz: 0.7,
	}}, "r2")
	if b[0].Merkmal.Wert != "wohnt zu zweit" {
		t.Fatal("starker gegenvorschlag setzt sich nicht durch")
	}
	if b[0].Merkmal.Konfidenz > ProfilStart {
		t.Fatalf("der neue wert startet mit %.2f", b[0].Merkmal.Konfidenz)
	}
}

// Was unter die Verfallsgrenze fällt, verschwindet - mit Verlaufseintrag.
func TestMerkmalVerfaellt(t *testing.T) {
	alt := []Merkmal{merkmal("haustier", "hat eine Katze", 0.2, 1)}
	a := ProfilVerrechnen(alt, []Urteil{{
		Merkmal: "haustier", Wert: "hat eine Katze", Urteil: "REVIDIERT", Konfidenz: 0.05,
	}}, "r2")
	var verfallen bool
	for _, x := range a {
		if x.Urteil == "VERFALLEN" && x.Loeschen {
			verfallen = true
		}
	}
	if !verfallen {
		t.Fatalf("nichts verfallen: %+v", a)
	}
}

// Bei Überlauf fliegt das schwächste - aber nie eine der drei Achsen.
func TestUeberlaufSchontKernmerkmale(t *testing.T) {
	var alt []Merkmal
	alt = append(alt, merkmal("geschwister", "eine Schwester", 0.2, 1))
	alt = append(alt, merkmal("geschlecht", "weiblich", 0.25, 1))
	alt = append(alt, merkmal("alter", "Mitte dreißig", 0.2, 1))
	for i := 0; i < MaxMerkmale; i++ {
		alt = append(alt, merkmal(string(rune('a'+i))+"ding", "x", 0.5, 2))
	}
	a := ProfilVerrechnen(alt, nil, "r9")
	for _, x := range a {
		if x.Loeschen && KernMerkmale[x.Merkmal.Merkmal] {
			t.Fatalf("kernmerkmal %q wurde weggeworfen", x.Merkmal.Merkmal)
		}
	}
	if len(a) == 0 {
		t.Fatal("der überlauf wurde gar nicht abgeräumt")
	}
}

// Zwei Urteile zum selben Merkmal in einer Runde sind ein Beleg, nicht zwei.
func TestZweiUrteileEineRunde(t *testing.T) {
	alt := []Merkmal{merkmal("alter", "Mitte dreißig", 0.4, 1)}
	a := ProfilVerrechnen(alt, []Urteil{
		{Merkmal: "alter", Wert: "Mitte dreißig", Urteil: "BESTAETIGT", Konfidenz: 0.6},
		{Merkmal: "Alter", Wert: "Mitte dreißig", Urteil: "BESTAETIGT", Konfidenz: 0.9},
	}, "r2")
	if len(a) != 1 || a[0].Merkmal.Belege != 2 {
		t.Fatalf("%d änderungen, %+v", len(a), a)
	}
}

// In den Fälschungsprompt geht nur, was über der Schwelle liegt - und als Wort.
func TestProfilZeilen(t *testing.T) {
	ms := []Merkmal{
		merkmal("alter", "Mitte dreißig", 0.8, 3),
		merkmal("haustier", "hat eine Katze", 0.2, 1),
	}
	z := ProfilZeilen(ms, ProfilSchwelle)
	if len(z) != 1 {
		t.Fatalf("%v statt genau einer zeile", z)
	}
	if got := z[0]; got != "alter: Mitte dreißig (ziemlich sicher)" {
		t.Fatalf("zeile %q", got)
	}
}
