package mimik

import "testing"

// Der gemeldete Fall vom 14.09.2026, im Bau nachgebildet.
//
// NICHT die echte Antwort, die ihn gemeldet hat: Antworten wirklicher Menschen
// stehen nicht im Repo (siehe SICHERHEIT.md). Nachgebildet ist genau das, was
// den Fall ausmacht - eine Antwort mit zwei kennzeichnenden Gegenstaenden, von
// denen die Begruendung einen nennt, um zu sagen, dass sie ihn NICHT benutzt
// hat. Kim ist erfunden.
const (
	kimAntwort = "Ich würde den ganzen Tag im Liegestuhl sitzen und " +
		"nebenbei ein Hörspiel laufen lassen."
	kimFrage    = "Was würdest du an einem völlig freien Samstag tun, wenn niemand etwas von dir will?"
	kimMaterial = "\n\n[interessen]\nvideospiele, kochen, radfahren"
)

func TestAntwortbezugFaengtDenGemeldetenFall(t *testing.T) {
	texte := []string{"Im Bett bleiben und das neue Spiel anfangen."}
	gruende := []string{
		"Das habe ich geschrieben, weil du Videospiele als Interesse angegeben " +
			"hast, und den Liegestuhl und das Hörspiel habe ich nicht erwähnt.",
	}
	got := Antwortbezug(kimAntwort, kimFrage, kimMaterial, texte, gruende)
	if len(got) != 1 {
		t.Fatalf("Antwortbezug = %v, erwartet [0] - \"Liegestuhl\" steht nur in der echten Antwort", got)
	}
}

// Dieselbe Begruendung ohne den Rueckblick auf die echte Antwort geht durch.
// Das ist der Zweck: Es geht um das Material, auf das sie sich bezieht, nicht um
// das, auf das sie sich nicht bezieht.
func TestAntwortbezugLaesstDasMaterialInRuhe(t *testing.T) {
	texte := []string{"Im Bett bleiben und das neue Spiel anfangen."}
	gruende := []string{
		"Du hast Videospiele als Interesse angegeben – daraus habe ich einen " +
			"Tag gemacht, der gar nicht erst anfängt.",
	}
	if got := Antwortbezug(kimAntwort, kimFrage, kimMaterial, texte, gruende); len(got) != 0 {
		t.Errorf("Fehlalarm: %v", got)
	}
}

// Ein Wort, das in der Frage steht, gehoert nicht der Antwort. "Tag" kommt in
// beiden vor - die Frage ist gemeinsamer Boden, und wer daraus zitiert, zitiert
// nicht die Person.
func TestWortAusDerFrageIstKeinBezug(t *testing.T) {
	texte := []string{"Einkaufen gehen, obwohl nichts fehlt."}
	gruende := []string{"Ein freier Tag ohne Plan, dafür habe ich das genommen."}
	if got := Antwortbezug(kimAntwort, kimFrage, kimMaterial, texte, gruende); len(got) != 0 {
		t.Errorf("Fehlalarm auf ein Wort aus der Frage: %v", got)
	}
}

// Und eines, das im Material steht, auch nicht - sonst waere die Pruefung ein
// Verbot, das Material zu nennen, und genau das soll sie ja fordern.
func TestWortAusDemMaterialIstKeinBezug(t *testing.T) {
	antwort := "Ich koche mir mittags immer dasselbe Gericht."
	texte := []string{"Radfahren, bis die Hände kalt werden."}
	gruende := []string{"Du hast Kochen als Interesse angegeben, das habe ich umgedreht."}
	if got := Antwortbezug(antwort, kimFrage, kimMaterial, texte, gruende); len(got) != 0 {
		t.Errorf("Fehlalarm auf ein Wort aus dem Material: %v", got)
	}
}

// Die Faelschung selbst darf in ihrer eigenen Begruendung stehen - sonst liesse
// sich nicht sagen, was daraus geworden ist.
func TestEigeneFaelschungDarfInIhrerBegruendungStehen(t *testing.T) {
	antwort := "Ein zweites Nudelsieb, das erste war zu grob."
	texte := []string{"Eine Taschenlampe, die im Regal verstaubt."}
	gruende := []string{"Aus der Taschenlampe habe ich etwas gemacht, das nie gebraucht wird."}
	if got := Antwortbezug(antwort, kimFrage, kimMaterial, texte, gruende); len(got) != 0 {
		t.Errorf("Fehlalarm auf die eigene Faelschung: %v", got)
	}
}

// Kurze Woerter bleiben aussen vor: Unter vier Zeichen steht ein Inhaltswort so
// haeufig zufaellig in beiden Texten, dass die Pruefung unbrauchbar wuerde.
func TestKurzeWoerterZaehlenNicht(t *testing.T) {
	antwort := "Mein Rad steht seit dem Winter im Keller."
	texte := []string{"Ein Stapel Zeitschriften neben dem Sofa."}
	gruende := []string{"Das Rad war mir zu naheliegend, also der Stapel."}
	// "rad" hat drei Zeichen und faellt durch das Sieb; gemeldet wird trotzdem,
	// denn "Winter" fehlt - also darf hier NICHTS gemeldet werden.
	if got := Antwortbezug(antwort, kimFrage, kimMaterial, texte, gruende); len(got) != 0 {
		t.Errorf("kurzes Wort hat ausgeloest: %v", got)
	}
}

func TestLeereBegruendungSchweigt(t *testing.T) {
	if got := Antwortbezug(kimAntwort, kimFrage, kimMaterial,
		[]string{"x"}, []string{"  "}); len(got) != 0 {
		t.Errorf("leere Begruendung hat ausgeloest: %v", got)
	}
	if got := Antwortbezug("", kimFrage, kimMaterial,
		[]string{"x"}, []string{"irgendwas"}); len(got) != 0 {
		t.Errorf("ohne Normalform hat ausgeloest: %v", got)
	}
}
