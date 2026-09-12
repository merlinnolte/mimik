package mimik

import (
	"strings"
	"unicode"
)

// Wer ohne Umlauttastatur oder aus Gewohnheit tippt, schreibt "hoer" statt
// "hör". Das ist Rechtschreibung, nicht Stimme – gehört also in die Normalform.
//
// Eine allgemeine Regel oe→ö gibt es dabei nicht, und zwar aus einem harten
// Grund: Sie zerstört mehr, als sie repariert. Aus "Poesie" würde "Pösie", aus
// "Michael" "Michäl", aus "Abenteuer" "Abenteür", aus "aktuell" "aktüll". Für
// jede dieser Stellen bräuchte man ein Wörterbuch, um zu entscheiden – und ein
// falsch aufgelöster Name ist schlimmer als eine fehlende Umlautpunktierung.
//
// Deshalb eine Positivliste: Nur was hier steht, wird ersetzt. Das kann
// per Konstruktion keine falschen Treffer erzeugen, und die Liste wächst,
// wenn euch etwas fehlt.
var umlautWoerter = map[string]string{
	// für / über
	"fuer": "für", "fuers": "fürs", "dafuer": "dafür", "wofuer": "wofür",
	"ueber": "über", "ueberall": "überall", "ueberhaupt": "überhaupt",
	"uebrigens": "übrigens", "uebung": "übung", "uebrig": "übrig",
	// können / möchten / müssen / dürfen / würden
	"koennen": "können", "koennte": "könnte", "koennten": "könnten", "koennt": "könnt",
	"moechte": "möchte", "moechten": "möchten", "moechtest": "möchtest",
	"muessen": "müssen", "muesste": "müsste", "muessten": "müssten", "muesst": "müsst",
	"duerfen": "dürfen", "duerfte": "dürfte", "wuerde": "würde", "wuerden": "würden",
	"waere": "wäre", "waeren": "wären", "waerst": "wärst",
	"haette": "hätte", "haetten": "hätten", "haettest": "hättest",
	// hören / fühlen / schön
	"hoer": "hör", "hoere": "höre", "hoeren": "hören", "hoert": "hört",
	"gehoert": "gehört", "aufhoeren": "aufhören",
	"fuehle": "fühle", "fuehlen": "fühlen", "fuehlt": "fühlt", "gefuehl": "gefühl",
	"schoen": "schön", "schoene": "schöne", "schoener": "schöner", "schoenste": "schönste",
	// Zeit und Häufigkeit
	"spaet": "spät", "spaeter": "später", "naechste": "nächste", "naechsten": "nächsten",
	"taeglich": "täglich", "jaehrlich": "jährlich", "haeufig": "häufig", "oefter": "öfter",
	"waehrend": "während", "frueh": "früh", "frueher": "früher", "maerz": "märz",
	// Alltag
	"zurueck": "zurück", "natuerlich": "natürlich", "ungefaehr": "ungefähr",
	"moeglich": "möglich", "unmoeglich": "unmöglich", "noetig": "nötig",
	"aehnlich": "ähnlich", "aendern": "ändern", "geaendert": "geändert",
	"aerger": "ärger", "aergert": "ärgert", "aergerlich": "ärgerlich",
	"erklaeren": "erklären", "erzaehlen": "erzählen", "erzaehlt": "erzählt",
	"waehlen": "wählen", "aufraeumen": "aufräumen", "raeumen": "räumen",
	"traeumen": "träumen", "laeuft": "läuft", "zufaellig": "zufällig",
	"verrueckt": "verrückt", "gluecklich": "glücklich", "glueck": "glück",
	"muede": "müde", "bloed": "blöd", "boese": "böse", "loesung": "lösung",
	"stueck": "stück", "buecher": "bücher", "tuer": "tür", "tueren": "türen",
	"kueche": "küche", "kuehl": "kühl", "kuehlschrank": "kühlschrank",
	"tschuess": "tschüss", "oel": "öl", "roemisch": "römisch",
	// Orte
	"koeln": "köln", "muenchen": "münchen", "duesseldorf": "düsseldorf",
	"nuernberg": "nürnberg", "osterreich": "österreich", "oesterreich": "österreich",
	// ss → ß, ebenfalls nur wo eindeutig
	"strasse": "straße", "strassen": "straßen",
	"gross": "groß", "grosse": "große", "grossen": "großen", "grosser": "großer",
	"grosses": "großes", "groesse": "größe", "groesser": "größer",
	"groesste": "größte", "heisst": "heißt", "weiss": "weiß", "fuss": "fuß",
	"fuesse": "füße", "spass": "spaß", "massnahme": "maßnahme",
	"draussen": "draußen", "aussen": "außen",
	"schliessen": "schließen", "schliesst": "schließt", "heissen": "heißen",
	"geniessen": "genießen", "geniesse": "genieße", "weisst": "weißt",
	"gruesse": "grüße", "suess": "süß", "suesse": "süße",
}

// UmlauteHerstellen löst bekannte Umschriften auf und behält dabei die
// Groß-/Kleinschreibung des Originals: "Hoer" wird "Hör", "HOER" wird "HÖR".
func UmlauteHerstellen(text string) string {
	var b strings.Builder
	var wort strings.Builder

	spuelen := func() {
		if wort.Len() == 0 {
			return
		}
		w := wort.String()
		if ersatz, ok := umlautWoerter[strings.ToLower(w)]; ok {
			b.WriteString(uebernehmeSchreibung(w, ersatz))
		} else {
			b.WriteString(w)
		}
		wort.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) {
			wort.WriteRune(r)
			continue
		}
		spuelen()
		b.WriteRune(r)
	}
	spuelen()
	return b.String()
}

// uebernehmeSchreibung überträgt das Muster des Originals auf den Ersatz.
func uebernehmeSchreibung(original, ersatz string) string {
	rs := []rune(original)
	if len(rs) == 0 {
		return ersatz
	}
	alleGross := true
	for _, r := range rs {
		if unicode.IsLower(r) {
			alleGross = false
			break
		}
	}
	// "HOER" nur dann als Versalien behandeln, wenn es mehr als ein Zeichen hat –
	// sonst würde ein einzelnes großes Wort am Satzanfang mitgerissen.
	if alleGross && len(rs) > 1 {
		return strings.ToUpper(ersatz)
	}
	if unicode.IsUpper(rs[0]) {
		er := []rune(ersatz)
		er[0] = unicode.ToUpper(er[0])
		return string(er)
	}
	return ersatz
}
