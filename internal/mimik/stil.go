package mimik

import (
	"regexp"
	"strings"
)

// Stilpruefungen: drei Merkmale, an denen eine Faelschung erkannt wird, bevor
// jemand ueber ihren Inhalt nachdenkt.
//
// Sie kommen aus der Lügenforschung, nicht aus dem Bauchgefuehl - und sie
// stehen HIER und nicht nur im Prompt, weil das an einem Tag dreimal nicht
// gereicht hat (Frage beantworten, Laenge, Materialtreue). Eine Regel im
// Prompt ist eine Bitte; eine Pruefung ist eine Bedingung.
//
// Alle drei sind weich wie Sperrbruch: Sie loesen einen neuen Versuch aus,
// verhindern aber nicht, dass am Ende der beste von drei Saetzen hinausgeht.
// Eine stilistisch schwache Karte ist eine schwache Runde, keine kaputte.

// MaxKausal ist, wie viele der drei Faelschungen einen Kausalsatz tragen
// duerfen.
//
// Warum ueberhaupt: Der stabilste Befund der verbalen Luegenforschung (Reality
// Monitoring, Johnson & Raye; aufgearbeitet bei Vrij) ist, dass wahre
// Schilderungen reicher an wahrnehmungsnahen Details sind und erfundene mehr
// "cognitive operations" enthalten - Einordnungen, Herleitungen, Begruendungen.
// Eine erfundene Erinnerung traegt ihre Konstruktion mit. "Vor dem Staubsauger,
// ich bin immer weggerannt" behauptet; "..., weil das Geraeusch mich
// erschreckt hat" erklaert sich.
//
// Eine erlaubt, nicht null: Auch Menschen begruenden gelegentlich, und drei
// Karten ohne einen einzigen Nebensatz sind als Satz genauso auffaellig.
const MaxKausal = 1

var reKausal = regexp.MustCompile(
	`(?i)(^|[\s,;–-])(weil|damit|deshalb|darum|daher|obwohl|sodass|so dass|denn|um zu|weshalb)($|[\s,.;:!?])`)

// Kausal sagt, ob ein Text sich begruendet.
func Kausal(text string) bool { return reKausal.MatchString(text) }

// Kausalbruch nennt die Faelschungen, die zusammen zu viele Begruendungen
// tragen. Gemeldet werden ALLE begruendenden, nicht nur die ueberzaehligen:
// Welche davon umgeschrieben wird, entscheidet das Modell besser als eine
// Reihenfolge im Code.
func Kausalbruch(faelschungen []string) []int {
	var mit []int
	for i, f := range faelschungen {
		if Kausal(f) {
			mit = append(mit, i)
		}
	}
	if len(mit) <= MaxKausal {
		return nil
	}
	return mit
}

// ---------------------------------------------------------------- Bau ---

// Satzbau ist der aeussere Bau eines Textes - was man sieht, bevor man liest.
type Satzbau struct {
	Saetze       int
	Kommas       int
	Schlusszeich string
}

var reSatzende = regexp.MustCompile(`[.!?…]+`)

func BauVon(text string) Satzbau {
	t := strings.TrimSpace(text)
	b := Satzbau{
		Saetze: len(reSatzende.FindAllString(t, -1)),
		Kommas: strings.Count(t, ","),
	}
	if b.Saetze == 0 {
		b.Saetze = 1 // ein Satzfragment ist ein Satz
	}
	if t != "" {
		if letzte := []rune(t)[len([]rune(t))-1]; strings.ContainsRune(".!?…", letzte) {
			b.Schlusszeich = string(letzte)
		}
	}
	return b
}

// Satzbaubruch nennt die Faelschungen, deren Bau aus dem Rahmen der Normalform
// faellt.
//
// Wofuer: Vier Karten liegen nebeneinander, und der Mensch davor vergleicht.
// Jede Eigenschaft, die genau EINE Karte hat, ist ein Signal - ob gut oder
// schlecht ist dabei gleichgueltig. Ein Nebensatz mehr, zwei Kommas mehr, ein
// Fragezeichen, wo die anderen einen Punkt haben: erkannt, ohne ein Wort
// gelesen zu haben.
//
// Toleranz: ein Satz und zwei Kommas. Enger waere Willkuer - Deutsch laesst
// dieselbe Aussage mit und ohne Komma zu -, weiter waere wirkungslos.
func Satzbaubruch(normalform string, faelschungen []string) []int {
	n := BauVon(normalform)
	var out []int
	for i, f := range faelschungen {
		b := BauVon(f)
		zuviel := b.Saetze-n.Saetze > 1 || n.Saetze-b.Saetze > 1 ||
			b.Kommas-n.Kommas > 2
		if zuviel {
			out = append(out, i)
		}
	}
	return out
}

// ------------------------------------------------------------ Abschluss ---

// abschluss sind Wendungen, mit denen ein Text sich selbst einordnet.
//
// Die schwaechste der drei Pruefungen, und deshalb eine kurze Liste: Nur
// mehrwortige, bewertende Schlussfiguren stehen darin. Einzelne Partikeln
// ("eben", "halt", "schon") gehoeren NICHT hierher - die sind das Gegenteil,
// naemlich echte gesprochene Sprache.
//
// Wofuer: Die Metaanalyse von DePaulo u. a. (2003) findet, dass erfundene
// Schilderungen weniger gewoehnliche Unvollkommenheiten enthalten und runder
// erzaehlt sind. Ein Fazit am Schluss ist die haeufigste Form dieser Rundung -
// ein Mensch hoert mitten drin auf.
var abschluss = []string{
	"am ende zählt", "am ende ist es", "letztlich ist es", "im grunde ist es",
	"das war es wert", "so ist das leben", "aber so ist das", "man lernt daraus",
	"das hat mir gezeigt", "seitdem weiß ich", "im nachhinein betrachtet",
	"und das ist auch gut so", "aber das gehört wohl dazu", "so lernt man",
	"hauptsache", "aber egal", "was soll man machen",
}

// Floskelbruch nennt die Faelschungen, die sich selbst einordnen.
func Floskelbruch(faelschungen []string) []int {
	var out []int
	for i, f := range faelschungen {
		k := strings.ToLower(f)
		for _, w := range abschluss {
			if strings.Contains(k, w) {
				out = append(out, i)
				break
			}
		}
	}
	return out
}

// Stilbruch fasst die drei zusammen: die Zahl der Verstoesse und ein Grund.
//
// Eine Zahl statt vier: Sie entscheidet nach drei gescheiterten Versuchen,
// welcher Satz notgedrungen hinausgeht - und dafuer braucht es ein Mass, keine
// Tabelle.
func Stilbruch(normalform string, faelschungen []string) (int, string) {
	kausal := Kausalbruch(faelschungen)
	bau := Satzbaubruch(normalform, faelschungen)
	floskel := Floskelbruch(faelschungen)
	lang := Laengenbruch(normalform, faelschungen)
	n := len(kausal) + len(bau) + len(floskel) + len(lang)
	switch {
	case len(lang) > 0:
		return n, "Länge"
	case len(kausal) > 0:
		return n, "Begründung"
	case len(bau) > 0:
		return n, "Bau"
	case len(floskel) > 0:
		return n, "Abschluss"
	}
	return 0, ""
}
