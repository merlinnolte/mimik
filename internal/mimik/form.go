package mimik

import (
	"strings"
	"unicode"
)

// Formmangel prüft, wie ein Kartentext geschrieben ist – nicht, was er sagt.
//
// Alle vier Karten einer Runde stehen nebeneinander. Fällt eine durch die Form
// aus der Reihe, ist sie erkannt, bevor jemand ihren Inhalt gelesen hat. Genau
// das soll die gemeinsame Erzeugung verhindern; hier wird nachgehalten, ob das
// Modell sich daran gehalten hat.
//
// Geprüft wird nur, was ohne Wörterbuch objektiv falsch ist. Ob ein Substantiv
// mitten im Satz großgeschrieben gehört, weiß keine Regel – "essen" und "Essen"
// sind dasselbe Wort in verschiedenen Rollen. Dafür gibt es keinen Ersatz für
// das Modell, und deshalb schreibt es hier alle vier Texte selbst.
//
// Rückgabe: leerer String heißt in Ordnung, sonst der Grund.
func Formmangel(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return "leer"
	}
	rs := []rune(t)

	// Kleiner Satzanfang. Ziffern und Anführungszeichen sind erlaubt – "3 Tage
	// am Stück" ist kein Fehler.
	if unicode.IsLower(rs[0]) {
		return "beginnt klein"
	}

	// Kein Satzzeichen am Ende. Ein Gedankenstrich zählt nicht: Die echte
	// Antwort darf abbrechen, aber dann tun es die Fälschungen auch – das
	// entscheidet das Modell, nicht diese Prüfung. Hier geht es nur darum, dass
	// überhaupt etwas dasteht.
	if !strings.ContainsRune(".!?…", rs[len(rs)-1]) {
		return "ohne Satzzeichen am Ende"
	}

	// Verdoppelte Satzzeichen. "!!!" ist eines der deutlichsten Zeichen dafür,
	// dass jemand getippt statt geschrieben hat.
	var vorher rune
	for _, r := range rs {
		if r == vorher && strings.ContainsRune("!?.,", r) {
			return "verdoppelte Satzzeichen"
		}
		vorher = r
	}

	if reEmoji.MatchString(t) {
		return "Emoji"
	}
	return ""
}

// FormBefund sammelt die Mängel über einen ganzen Kartensatz. Der Index -1
// steht für die echte Karte, 0 bis 2 für die Fälschungen.
type FormBefund struct {
	Mangel map[int]string
}

func (f FormBefund) OK() bool { return len(f.Mangel) == 0 }

func (f FormBefund) Grund() string {
	if f.OK() {
		return ""
	}
	teile := make([]string, 0, len(f.Mangel))
	for i, m := range f.Mangel {
		wer := "Fälschung " + string(rune('1'+i))
		if i < 0 {
			wer = "echte Karte"
		}
		teile = append(teile, wer+": "+m)
	}
	return strings.Join(teile, ", ")
}

// Schuldig nennt die Fälschungen, die neu geschrieben werden müssen. Ein Mangel
// an der echten Karte steht nicht darin – die schreibt man nicht neu, die
// korrigiert man.
func (f FormBefund) Schuldig() []int {
	var xs []int
	for i := range f.Mangel {
		if i >= 0 {
			xs = append(xs, i)
		}
	}
	return xs
}

func FormPruefen(normalform string, faelschungen []string) FormBefund {
	b := FormBefund{Mangel: map[int]string{}}
	if m := Formmangel(normalform); m != "" {
		b.Mangel[-1] = m
	}
	for i, f := range faelschungen {
		if m := Formmangel(f); m != "" {
			b.Mangel[i] = m
		}
	}
	return b
}
