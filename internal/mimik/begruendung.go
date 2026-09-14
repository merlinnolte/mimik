package mimik

import "strings"

// Prueft, ob eine Begruendung ueber die echte Antwort dieser Runde spricht.
//
// Der Fall, gemeldet am 14.09.2026 aus dem Betrieb. Auf eine Antwort, in der
// zwei Gegenstaende vorkamen, schrieb das Modell als Begruendung, es habe diese
// beiden Gegenstaende NICHT erwaehnt und stattdessen etwas anderes genommen.
// Inhaltlich richtig gearbeitet - aber die Begruendung berichtet ueber die
// eigene Arbeit statt ueber die Person, und ausgerechnet ueber das Material, auf
// das sie sich NICHT bezieht.
//
// Die gemeldete Antwort steht hier nicht: Antworten wirklicher Menschen gehoeren
// nicht ins Repo. Der nachgebildete Fall liegt im Test.
//
// Die Ursache stand im Prompt selbst: Er verlangte, "die Formulierung aus
// [echte_antwort_roh], an die du angeknuepft hast" beim Namen zu nennen, und
// verbot vier Zeilen spaeter, auf der echten Antwort aufzubauen. Ein Modell,
// dem man beides sagt, berichtet den Widerspruch. Die Zeile ist raus.
//
// Verraten ist dadurch nie etwas: Die Begruendung sieht nur der Mensch, um
// dessen eigene Antwort es geht (Klonblick und Aufloesung des eigenen Satzes).
// Es ist eine Frage davon, wie es sich liest - deshalb eine weiche Pruefung,
// die einen neuen Versuch anstoesst, und kein Boden.

// MinBezugswort: Kurze Woerter bleiben aussen vor. Unter vier Zeichen ist ein
// Inhaltswort so allgemein, dass es zufaellig in beiden Texten steht. Die
// kennzeichnenden Gegenstaende einer Antwort - ein Moebel, ein Geraet, eine
// Gattung - haben vier Zeichen und mehr; darunter liegen "Rad", "Tag", "Bus".
const MinBezugswort = 4

// Antwortbezug nennt die Begruendungen, die ein Wort aus der echten Antwort
// benutzen, das nirgends sonst herkommen kann.
//
// Gezaehlt wird nur, was die echte Antwort EXKLUSIV hat: Woerter, die auch im
// Material, in der Frage oder in der beschriebenen Faelschung selbst stehen,
// darf die Begruendung nennen - dort kommen sie legitim her. Uebrig bleibt
// genau das, was sie nur aus [echte_antwort_roh] haben kann.
//
// Das Wortsieb ist fragerahmen aus frage.go, und das ist Absicht: Es ist eine
// Liste deutscher Funktionswoerter, die nur ihren ersten Einsatz im Namen
// traegt. Eine zweite Liste fuer denselben Zweck waere ein zweites Vokabular
// fuer dieselbe Sache - genau das, was dieses Projekt nicht haben will.
func Antwortbezug(normalform, frage, material string, texte, gruende []string) []int {
	echt := Fragenkern(normalform)
	if len(echt) == 0 {
		return nil
	}
	for _, erlaubt := range []string{frage, material} {
		for w := range Fragenkern(erlaubt) {
			delete(echt, w)
		}
	}
	var out []int
	for i, grund := range gruende {
		if strings.TrimSpace(grund) == "" {
			continue
		}
		// Die Faelschung, um die es geht, darf in ihrer eigenen Begruendung
		// vorkommen - sonst liesse sich nicht sagen, was daraus geworden ist.
		eigen := map[string]struct{}{}
		if i < len(texte) {
			eigen = Fragenkern(texte[i])
		}
		for w := range Fragenkern(grund) {
			if len([]rune(w)) < MinBezugswort {
				continue
			}
			if _, verraeter := echt[w]; !verraeter {
				continue
			}
			if _, ausEigener := eigen[w]; ausEigener {
				continue
			}
			out = append(out, i)
			break
		}
	}
	return out
}
