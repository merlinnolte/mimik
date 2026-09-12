// Package sicher putzt Text, der von außen kommt: von den Spielern und vom
// Modell. Beide sind für dieses Programm dieselbe Sorte Quelle – Material, nie
// eine Anweisung.
//
// Drei Wege führen aus einer Zeichenkette heraus in etwas, das nicht mehr Text
// ist:
//
//  1. In das Terminal des Betreibers. Eine Antwort mit ESC [ 2 J löscht dem
//     Betreiber den Bildschirm, eine mit ESC ] 8 legt einen anklickbaren Link
//     ins Protokoll. Beides ist kein hypothetischer Angriff, sondern das, was
//     jede Protokollzeile mit Fremdtext von sich aus möglich macht.
//  2. In die Anzeige der App. U+202E dreht die Leserichtung um: Auf dem Schirm
//     stünde dann etwas anderes als in der Datenbank. In einem Spiel, in dem
//     man vier Texte gegeneinander liest, wäre das kein Schönheitsfehler.
//  3. Über die Länge. Ein Feld ohne Grenze ist eine Einladung, den Speicher,
//     das Protokoll oder die Rechnung beim Modellanbieter zu füllen.
//
// Was hier durchgeht, ist druckbarer Text. Mehr braucht das Spiel nicht.
package sicher

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Text macht aus beliebigen Bytes eine einzeilige, druckbare Zeichenkette von
// höchstens max Zeichen (nicht Bytes – gezählt werden Runen).
func Text(roh string, max int) string {
	if !utf8.ValidString(roh) {
		roh = strings.ToValidUTF8(roh, "")
	}
	roh = ohneFolgen(roh)

	var b strings.Builder
	b.Grow(len(roh))
	letzteLeer := false
	for _, r := range roh {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			// Zeilenumbrüche im Kartentext würden das Raster sprengen; im
			// Protokoll würden sie eine Zeile vortäuschen, die es nie gab.
			r = ' '
		case unwillkommen(r):
			continue
		}
		if r == ' ' {
			if letzteLeer {
				continue
			}
			letzteLeer = true
		} else {
			letzteLeer = false
		}
		b.WriteRune(r)
	}
	return kuerzen(strings.TrimSpace(b.String()), max)
}

// Protokoll bereitet Fremdtext für eine Protokollzeile auf. Enger als Text:
// Hier zählt nur, dass nichts das Terminal erreicht, was es steuern könnte.
func Protokoll(roh string) string {
	return Text(roh, 200)
}

// unwillkommen sind Steuerzeichen, Formatzeichen und alles, wofür es keine
// Darstellung gibt. Ausdrücklich mitgemeint: die Zeichen für Leserichtung
// (U+202A–U+202E, U+2066–U+2069) und die unsichtbaren Breitenlosen, die beide
// unter unicode.Cf fallen.
func unwillkommen(r rune) bool {
	switch {
	case r == utf8.RuneError:
		return true
	case r < 0x20 || (r >= 0x7f && r <= 0x9f):
		return true
	case unicode.Is(unicode.Cf, r): // Format: Bidi, Zero-Width, BOM
		return true
	case unicode.Is(unicode.Co, r): // Private Use
		return true
	case unicode.Is(unicode.Cs, r): // Surrogate
		return true
	}
	return false
}

// ohneFolgen entfernt ANSI-Folgen, bevor die einzelnen Steuerzeichen fallen.
// Ohne diesen Schritt bliebe von "ESC[31m" das lesbare "[31m" übrig – harmlos,
// aber Müll mitten im Text.
func ohneFolgen(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != 0x1b {
			b.WriteRune(rs[i])
			continue
		}
		i++
		if i >= len(rs) {
			break
		}
		switch rs[i] {
		case '[': // CSI: bis zum ersten Zeichen aus @ bis ~
			for i++; i < len(rs); i++ {
				if rs[i] >= '@' && rs[i] <= '~' {
					break
				}
			}
		case ']': // OSC: bis BEL oder ESC \ – hier stecken die Hyperlinks
			for i++; i < len(rs); i++ {
				if rs[i] == 0x07 {
					break
				}
				if rs[i] == 0x1b && i+1 < len(rs) && rs[i+1] == '\\' {
					i++
					break
				}
			}
		default: // zweizeichige Folge, das eine Zeichen ist schon verbraucht
		}
	}
	return b.String()
}

// kuerzen schneidet an einer Runengrenze und hängt ein Auslassungszeichen an,
// damit ein abgeschnittener Text auch als solcher zu erkennen ist.
func kuerzen(s string, max int) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return strings.TrimSpace(string(rs[:max-1])) + "…"
}
