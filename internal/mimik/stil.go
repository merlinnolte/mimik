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
	gerippe := Gerippebruch(normalform, faelschungen)
	n := len(kausal) + len(bau) + len(floskel) + len(lang) + len(gerippe)
	switch {
	case len(gerippe) > 0:
		return n, "Abwandlung"
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

// ---------------------------------------------------------------- Gerippe ---

// gerippewoerter sind die Wörter, die ein Satz braucht und die nichts über
// seinen Inhalt sagen: Artikel, Pronomen, Praepositionen, Ordnungszahlen,
// Haeufigkeitswoerter. Uebrig bleibt der BAU eines Satzes ohne seinen
// Gegenstand.
//
// Bewusst klein gehalten: Je mehr Woerter darin stehen, desto aehnlicher werden
// sich alle deutschen Saetze - und die Pruefung darunter waere nicht mehr zu
// gebrauchen.
var gerippewoerter = map[string]bool{
	"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true,
	"ein": true, "eine": true, "einen": true, "einem": true, "einer": true,
	"mein": true, "meine": true, "meinen": true, "meinem": true, "meiner": true,
	"ich": true, "mir": true, "mich": true, "man": true, "es": true,
	"mit": true, "ohne": true, "vor": true, "nach": true, "bei": true, "beim": true,
	"in": true, "im": true, "an": true, "am": true, "auf": true, "aus": true,
	"zu": true, "zum": true, "zur": true, "von": true, "vom": true, "um": true, "ums": true,
	"und": true, "aber": true, "oder": true, "dann": true, "noch": true, "schon": true,
	"immer": true, "nie": true, "wieder": true, "nur": true, "auch": true, "so": true,
	"erste": true, "ersten": true, "zweite": true, "zweiten": true, "dritte": true,
	"ist": true, "war": true, "habe": true, "hab": true, "hatte": true, "bin": true,
}

var reWort = regexp.MustCompile(`[\p{L}]+`)

// Gerippe ist der Satzbau ohne Gegenstand: nur die Wörter aus
// gerippewoerter, in ihrer Reihenfolge.
func Gerippe(text string) []string {
	var out []string
	for _, w := range reWort.FindAllString(strings.ToLower(text), -1) {
		if gerippewoerter[w] {
			out = append(out, w)
		}
	}
	return out
}

// MinGerippe: Unter drei Gerippewoertern sagt die Pruefung nichts. "Kaffee."
// hat keinen Bau, den man kopieren koennte.
const MinGerippe = 3

// MinGleicherAnfang: So viele Gerippewoerter am Stueck, und der Satz faengt
// erkennbar genauso an.
//
// Vier, nicht drei: Mit drei schlaegt "Vor dem Staubsauger, ich bin immer
// weggerannt" gegen "Vor dem Keller, ich habe mich nie runtergetraut" an
// ("vor dem ich"), und das ist keine Abwandlung, sondern gewoehnliches Deutsch.
// Gemessen an den Karten vom 14.09.2026: bei vier kein einziger Fehlalarm, der
// Zielfall ("Eine zweite X, die erste ...") faellt durch.
const MinGleicherAnfang = 4

// EnthaltenGerippe faengt den Fall, in dem der Bau nicht am Anfang, sondern
// mitten im Satz uebernommen ist.
const EnthaltenGerippe = 0.8

// Gerippebruch nennt die Faelschungen, die das Satzgerippe der echten Antwort
// uebernehmen.
//
// Wofuer: "Eine zweite Kaffeemuehle, die erste mahlt zu grob" gegen "Eine
// zweite Fahrkartenhuelle, die erste ist noch voellig in Ordnung" - gemessen am
// 14.09.2026 im Betrieb. Die n-Gramm-Aehnlichkeit lag bei 0.22, also weit unter
// jeder Schwelle, weil die INHALTSWOERTER verschieden sind. Uebernommen ist
// aber der Bau, und der ist das Verraeterische: Wer die echte Antwort abwandelt,
// erzeugt eine zweite richtige Karte, und der Tipp wird zum Muenzwurf.
//
// Der Prompt verbietet das ("kein Nachbarfall, keine Abwandlung") - zum vierten
// Mal an einem Tag hat eine Regel im Prompt allein nicht gehalten.
func Gerippebruch(normalform string, faelschungen []string) []int {
	echt := Gerippe(normalform)
	if len(echt) < MinGerippe {
		return nil
	}
	e := strings.Join(echt, " ")
	var out []int
	for i, f := range faelschungen {
		g := Gerippe(f)
		if len(g) < MinGerippe {
			continue
		}
		gleich := 0
		for gleich < len(echt) && gleich < len(g) && echt[gleich] == g[gleich] {
			gleich++
		}
		if gleich >= MinGleicherAnfang ||
			Enthalten(e, strings.Join(g, " ")) >= EnthaltenGerippe {
			out = append(out, i)
		}
	}
	return out
}
