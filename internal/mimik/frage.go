package mimik

import (
	"sort"
	"strings"
)

// Fragenkern nimmt einer Frage ihren Rahmen und laesst uebrig, wonach sie
// fragt. Zwei Fragen sind dieselbe Frage, wenn ihre Kerne sich decken.
//
// Warum ein eigenes Mass, wo Aehnlichkeit schon da ist: Gemessen an allen
// 10.296 Paaren der 144 Fragen vom 14.09.2026 rangiert das n-Gramm-Mass
// Rahmengleichheit ueber Bedeutungsgleichheit, weil eine Frage kurz ist und
// ueberwiegend aus Rahmen besteht.
//
//	0.48	"Welches Geraeusch magst du, obwohl die meisten es nicht moegen?"
//	    	"Welches Wetter magst du, obwohl die meisten es hassen?"  - verschieden
//	0.38	"Was wuerdest du sagen, wenn niemand beleidigt sein koennte?"
//	    	"Was wuerdest du tun, wenn dir niemand zusehen koennte?"  - verschieden
//	0.44	"Welche Aufgabe im Haushalt schiebst du am laengsten vor dir her?"
//	    	"Welche Aufgabe schiebst du gerade vor dir her?"          - DOPPEL
//
// Das echte Doppel liegt UNTER beiden Fehlalarmen. Ueber die Kerne gerechnet
// dreht sich das Verhaeltnis: 0.60 gegen 0.20 und 0.00.
//
// DIES IST NICHT DAS KOMPLEMENT VON gerippewoerter, und das ist keine
// Nachlaessigkeit. Der Gedanke liegt nahe - Gerippe behaelt die Funktionswoerter,
// hier will man die anderen -, aber gerippewoerter ist absichtlich klein
// gehalten und enthaelt kein einziges Interrogativum, kein "du/dir/dich/dein"
// und keines der Frage-Modalverben ("wuerdest", "willst", "moechtest",
// "kannst"). Genau diese fuenfzehn Woerter SIND der Rahmen einer Frage, und im
// Komplement landen sie alle im Kern. Gegengemessen am selben Bestand:
//
//	Komplement von gerippewoerter	119 Paare >= 0.30, 6 >= 0.50, davon 3 Fehlalarme
//	eigene Liste (unten)        	 12 Paare >= 0.33, 3 >= 0.50, kein Fehlalarm
//
// Wer die beiden Listen zu einer zusammenfasst, blendet dieses Mass, ohne dass
// ein Test rot wird: Der Bau eines Satzes und der Rahmen einer Frage sind zwei
// verschiedene Dinge, die sich nur teilweise ueberschneiden.

// fragerahmen sind die Woerter, die in jeder Frage stehen koennen und nichts
// darueber sagen, wonach gefragt wird: Interrogativa, Pronomen, Hilfs- und
// Modalverben, Praepositionen, Allerweltsadverbien.
//
// Bewusst grosszuegig, anders als gerippewoerter: Jedes Rahmenwort, das stehen
// bleibt, verwaessert den Kern und drueckt die Naehe zweier gleicher Fragen nach
// unten. Ein Wort zu viel in der Liste kostet nur dort etwas, wo es der
// Gegenstand einer Frage ist - und "welches", "wuerdest", "gerade" ist das nie.
//
// Diese Liste und die Schwellen unten sind EIN Ding und gehoeren zusammen
// nachgemessen. Belegt am 14.09.2026: Der Versuch, "lang" aufzunehmen (wegen
// "eine Woche lang" gegen "einen Tag lang"), schrumpfte den Kern von "Was
// wuerdest du tun, wenn du ein Jahr lang nicht arbeiten muesstest?" auf zwei
// Woerter - und liess es gegen "Was tust du, wenn du eigentlich arbeiten
// solltest?" von 0.33 auf 0.50 steigen, also aus dem Warnband in die Sperre.
// Zwei verschiedene Fragen, ein Fehlalarm, ein Wort. Wieder raus.
var fragerahmen = map[string]bool{
	// Interrogativa
	"was": true, "wer": true, "wen": true, "wem": true, "wessen": true,
	"wie": true, "wo": true, "woher": true, "wohin": true, "wann": true,
	"warum": true, "wieso": true, "weshalb": true, "wofuer": true, "wofür": true,
	"wobei": true, "worauf": true, "worueber": true, "worüber": true,
	"wovor": true, "wovon": true, "womit": true, "wodurch": true,
	"welche": true, "welcher": true, "welches": true, "welchen": true, "welchem": true,
	// Pronomen und Artikel
	"du": true, "dir": true, "dich": true, "dein": true, "deine": true,
	"deinem": true, "deiner": true, "deinen": true, "deines": true,
	"ich": true, "mir": true, "mich": true, "mein": true, "meine": true,
	"man": true, "es": true, "sich": true,
	"der": true, "die": true, "das": true, "den": true, "dem": true, "des": true,
	"ein": true, "eine": true, "einen": true, "einem": true, "einer": true, "eines": true,
	"kein": true, "keine": true, "keinen": true,
	// Bindewoerter
	"und": true, "oder": true, "aber": true, "wenn": true, "obwohl": true,
	"dass": true, "als": true, "ob": true, "damit": true, "weil": true,
	"denn": true, "doch": true, "noch": true, "schon": true, "dann": true, "so": true,
	// Hilfs- und Modalverben in allen Formen, die in Fragen vorkommen
	"ist": true, "sind": true, "bist": true, "war": true, "warst": true,
	"waere": true, "wäre": true, "waerst": true, "wärst": true,
	"hast": true, "habe": true, "hat": true, "haben": true, "hatte": true,
	"hattest": true, "haetten": true, "hätten": true,
	"wirst": true, "werden": true, "wird": true,
	"wuerde": true, "würde": true, "wuerdest": true, "würdest": true,
	"kannst": true, "kann": true, "koennte": true, "könnte": true,
	"koenntest": true, "könntest": true, "konnte": true,
	"willst": true, "will": true, "wollte": true,
	"moechtest": true, "möchtest": true, "moechte": true, "möchte": true,
	"magst": true, "mag": true, "musst": true, "muss": true,
	"muesstest": true, "müsstest": true,
	"sollst": true, "soll": true, "sollte": true, "solltest": true,
	"tust": true, "tun": true, "tut": true, "getan": true,
	"machst": true, "machen": true, "macht": true, "gemacht": true,
	"gibst": true, "gibt": true, "geben": true, "haeltst": true, "hältst": true,
	// Praepositionen
	"bei": true, "mit": true, "ohne": true, "von": true, "vom": true,
	"zu": true, "zum": true, "zur": true, "in": true, "im": true,
	"an": true, "am": true, "auf": true, "aus": true, "fuer": true, "für": true,
	"ueber": true, "über": true, "um": true, "nach": true, "vor": true,
	"seit": true, "gegen": true, "bis": true, "durch": true,
	// Allerweltsadverbien
	"gern": true, "gerne": true, "nie": true, "immer": true, "nicht": true,
	"mehr": true, "sehr": true, "ganz": true, "auch": true, "nur": true,
	"eigentlich": true, "wirklich": true, "selbst": true, "einmal": true,
	"etwas": true, "jemand": true, "niemand": true, "zuletzt": true,
	"heute": true, "gerade": true, "sofort": true, "zuerst": true, "wieder": true,
}

// Es gibt hier absichtlich KEIN Gegenstueck zu MinGerippe.
//
// Bei Gerippe schweigt die Pruefung unter drei Woertern, weil ein zu kurzer Satz
// keinen Bau hat, den man abschreiben koennte. Hier ist es umgekehrt: Ein Kern
// aus einem einzigen Wort heisst, dass die Frage sehr allgemein ist ("Was
// verzeihst du sofort, was nie?" -> {verzeihst}), und dann IST die Uebereinstim-
// mung in diesem einen Wort die Uebereinstimmung. Eine Ausnahme waere ein Loch:
// sechs der 144 Fragen haetten sich damit jeder Pruefung entzogen.
//
// Nachgemessen: Ohne Ausnahme kommt genau ein Paar hinzu, und es ist ein echtes
// ("Was tust du, wenn du eigentlich arbeiten solltest?" gegen "Was wuerdest du
// tun, wenn du ein Jahr lang nicht arbeiten muesstest?", 0.33). Nichts
// ueberschreitet dadurch FrageDoppel.

// FrageDoppel und FrageNachbar: zwei Stufen, nicht eine Schwelle.
//
// Kalibriert an allen 10.296 Paaren der 144 Fragen vom 14.09.2026, also am
// Bestand VOR den Ruecknahmen:
//
//	>= 0.50    3 Paare  - alle drei echte Doppel, null Fehlalarme
//	0.40-0.50  1 Paar   - "Umweg in Kauf" gegen "Umweg, um jemandem nicht zu
//	                      begegnen": zwei verschiedene Fragen
//	0.33-0.40  8 Paare  - ein echtes Doppel ("gesehen haben"), sonst
//	                      Haeufungen um ein geteiltes Wort ("kind", "tag")
//	0.25-0.30 17 Paare  - Rauschen
//
// Deshalb blockiert 0.50 und 0.33 warnt nur. Ein einziges Tor waere entweder
// nutzlos (bei 0.50 gehen echte Nachbarn durch) oder laestig (bei 0.33 fallen
// vier gute Fragen).
//
// Nach den acht Ruecknahmen (136 Fragen): 0 Paare ueber 0.50, 7 im Warnband.
// Nach dem Auffuellen auf 358 (63.903 Paare): 0 ueber 0.50, 28 im Warnband.
const (
	FrageDoppel  = 0.50
	FrageNachbar = 0.33
)

// Fragenkern liefert die Inhaltswoerter einer Frage.
//
// Woerter unter drei Zeichen fallen mit: Was davon nicht schon im Rahmen steht,
// ist Rest der Normalisierung und nie der Gegenstand einer Frage.
//
// Wortweise und nicht ueber n-Gramme: Der Unterschied zwischen zwei Fragen liegt
// in ganzen Woertern, nicht in Zeichenfolgen.
func Fragenkern(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range strings.Fields(normalisieren(text)) {
		if fragerahmen[w] || len([]rune(w)) <= 2 {
			continue
		}
		out[w] = struct{}{}
	}
	return out
}

// Fragennaehe ist Jaccard ueber die Kerne. Symmetrisch, weil zwei Fragen
// aehnlich lang sind - und ueber Woertern, nicht ueber n-Grammen, weil der
// Unterschied zwischen zwei Fragen in ganzen Woertern liegt und nicht in
// Zeichenfolgen.
//
// Null, sobald eine Seite gar keinen Kern hat - das ist keine Frage mehr,
// sondern ein Satz aus Rahmenwoertern.
func Fragennaehe(a, b string) float64 {
	ka, kb := Fragenkern(a), Fragenkern(b)
	if len(ka) == 0 || len(kb) == 0 {
		return 0
	}
	s := schnitt(ka, kb)
	return float64(s) / float64(len(ka)+len(kb)-s)
}

// Fragenpaar benennt zwei Fragen ueber ihre Stellung in der uebergebenen Liste.
// Indizes statt Texte, damit derselbe Rueckgabewert dem Test (der Text UND
// Rubrik zeigen will) und dem Werkzeug am Rand reicht.
type Fragenpaar struct {
	A, B      int
	Naehe     float64
	Gemeinsam []string // die Woerter, die beide Kerne teilen, alphabetisch
}

// Fragendoppel nennt alle Paare ab der uebergebenen Schwelle, das naechste
// zuerst.
//
// Quadratisch und absichtlich: 360 Fragen sind 64.620 Paare, und das rechnet ein
// Test in Millisekunden. Eine Abkuerzung ueber einen Index waere schneller und
// koennte ein Paar verschweigen.
//
// Die Reihenfolge ist vollstaendig festgelegt (Naehe, dann A, dann B): Sonst
// flackert die Ausgabe des Werkzeugs zwischen zwei Laeufen und ein Diff darueber
// ist wertlos.
func Fragendoppel(fragen []string, ab float64) []Fragenpaar {
	kerne := make([]map[string]struct{}, len(fragen))
	for i, f := range fragen {
		kerne[i] = Fragenkern(f)
	}
	var out []Fragenpaar
	for i := range fragen {
		if len(kerne[i]) == 0 {
			continue
		}
		for j := i + 1; j < len(fragen); j++ {
			if len(kerne[j]) == 0 {
				continue
			}
			var gemeinsam []string
			for w := range kerne[i] {
				if _, ok := kerne[j][w]; ok {
					gemeinsam = append(gemeinsam, w)
				}
			}
			s := len(gemeinsam)
			n := float64(s) / float64(len(kerne[i])+len(kerne[j])-s)
			if n < ab {
				continue
			}
			sort.Strings(gemeinsam)
			out = append(out, Fragenpaar{A: i, B: j, Naehe: n, Gemeinsam: gemeinsam})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Naehe != out[j].Naehe {
			return out[i].Naehe > out[j].Naehe
		}
		if out[i].A != out[j].A {
			return out[i].A < out[j].A
		}
		return out[i].B < out[j].B
	})
	return out
}
