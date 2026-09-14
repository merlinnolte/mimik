package mimik

import (
	"strings"
	"testing"
)

const echt = "Die zweite Kaffeemaschine. Die alte steht immer noch im Keller, falls die neue ausfällt."

var (
	gestreut = []string{
		"Ein Plattenspieler, obwohl ich genau fünf Platten besitze. Er steht einfach gut da.",
		"Eine Bahncard für ein halbes Jahr, in dem ich fast nie gefahren bin.",
		"Ein Paar Wanderstiefel, die ich seit dem Kauf nicht aus dem Karton geholt habe.",
	}
	umkreisend = []string{
		"Die zweite Kaffeemaschine. Die alte steht noch im Keller, falls die neue kaputtgeht.",
		"Noch eine Kaffeemaschine, die alte habe ich trotzdem behalten.",
		"Eine Kaffeemaschine mehr, die alte steht im Keller.",
	}
	geclustert = []string{
		"Ein Plattenspieler, obwohl ich nur fünf Platten habe.",
		"Ein Plattenregal, obwohl ich kaum Platten besitze.",
		"Noch mehr Schallplatten, obwohl der Spieler kaum läuft.",
	}
)

func TestAbstandsfenster(t *testing.T) {
	faelle := []struct {
		name      string
		fakes     []string
		willOK    bool
		willGrund string
	}{
		{"gestreut", gestreut, true, ""},
		{"umkreisend", umkreisend, false, "Nähe"},
		{"geclustert", geclustert, false, "Streuung"},
	}
	for _, f := range faelle {
		b := Abstandsfenster(echt, f.fakes)
		if b.OK() != f.willOK || b.Grund != f.willGrund {
			t.Errorf("%s: OK=%v grund=%q (max=%.2f peers=%.2f), erwartet OK=%v grund=%q",
				f.name, b.OK(), b.Grund, b.MaxZuEcht, b.MittelPeers, f.willOK, f.willGrund)
		}
		if !f.willOK && len(b.Schuldig) == 0 {
			t.Errorf("%s: verworfen, aber niemand als schuldig benannt", f.name)
		}
	}
}

func TestSperrbruch(t *testing.T) {
	sperre := []string{"rennrad", "fahrrad", "radsport"}
	fakes := []string{
		"Ein Plattenspieler, obwohl ich nur fünf Platten habe.",
		"Ein Fahrrad, das ich nie benutze.", // Bruch
		"Eine Bahncard, die sich nicht gelohnt hat.",
	}
	got := Sperrbruch(fakes, sperre)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("schuldig=%v, erwartet [1]", got)
	}
	if len(Sperrbruch(gestreut, sperre)) != 0 {
		t.Fatal("saubere fälschungen als sperrbruch gemeldet")
	}
}

func TestNormalformPlausibel(t *testing.T) {
	roh := "bereuen tu ich nix, ich trink halt viel kaffe"
	faelle := []struct {
		name string
		aus  string
		will bool
	}{
		{"sauber geglättet", "Bereuen tue ich nichts, ich trinke halt viel Kaffee.", true},
		{"unverändert", roh, true},
		{"leer", "", false},
		{"umgeschrieben und aufgebläht", "Ich bereue diese Anschaffung keineswegs, denn ich bin nun einmal ein Mensch, der ausgesprochen viel Kaffee zu sich nimmt, und deshalb war es richtig.", false},
		{"zusammengestrichen", "Kein Bedauern.", false},
	}
	for _, f := range faelle {
		if got := NormalformPlausibel(roh, f.aus); got != f.will {
			t.Errorf("%s: %v, erwartet %v", f.name, got, f.will)
		}
	}
}

func TestUmlauteZerschneidenKeineGramme(t *testing.T) {
	if Aehnlichkeit("Schöße", "Schöße") < 0.99 {
		t.Fatal("identischer text mit umlaut ist nicht ähnlich zu sich selbst")
	}
	if Aehnlichkeit("völlig anders", strings.Repeat("z", 40)) > 0.05 {
		t.Fatal("unähnliches gilt als ähnlich")
	}
}

func enthaelt(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

func TestErsatzNormalform(t *testing.T) {
	faelle := []struct{ ein, aus string }{
		{"bereuen tu ich nix!!! ich trink halt viel kaffe",
			"Bereuen tu ich nix! Ich trink halt viel kaffe."},
		{"die zweite kaffeemaschine 😄", "Die zweite kaffeemaschine."},
		{"Schon in Ordnung.", "Schon in Ordnung."},
		{"Geh raus!", "Geh raus!"},
	}
	for _, f := range faelle {
		if got := ErsatzNormalform(f.ein); got != f.aus {
			t.Errorf("%q\n  bekam    %q\n  erwartet %q", f.ein, got, f.aus)
		}
	}
}

// Die Formprüfung hält fest, was ohne Wörterbuch objektiv falsch ist. Sie
// ersetzt das Modell nicht - Substantive mitten im Satz kann sie nicht beurteilen
// -, aber sie fängt die deutlichen Brüche.
func TestFormmangel(t *testing.T) {
	schlecht := []struct{ text, grund string }{
		{"kleingeschrieben, aber sonst in Ordnung.", "beginnt klein"},
		{"Ohne Punkt am Ende", "ohne Satzzeichen am Ende"},
		{"Das nervt total!!!", "verdoppelte Satzzeichen"},
		{"Ein schöner Tag 😄.", "Emoji"},
		{"   ", "leer"},
	}
	for _, f := range schlecht {
		if got := Formmangel(f.text); got != f.grund {
			t.Errorf("%q -> %q, erwartet %q", f.text, got, f.grund)
		}
	}
	gut := []string{
		"Ein selbstgemachtes Kochbuch, handgeschrieben.",
		"Geh raus!",
		"Wirklich? Ich weiß es nicht.",
		"3 Tage am Stück gewandert.",
		"Der Stapel Zeitschriften neben dem Sofa …",
	}
	for _, t2 := range gut {
		if got := Formmangel(t2); got != "" {
			t.Errorf("%q abgelehnt: %s", t2, got)
		}
	}
}

// Ein Satz, in dem nur EINE Karte aus der Form fällt, ist der Fall, um den es
// geht: Sie ist erkannt, bevor jemand ihren Inhalt gelesen hat.
func TestFormPruefenFindetDieEineKarte(t *testing.T) {
	b := FormPruefen("Ein selbstgemachtes Kochbuch, handgeschrieben.", []string{
		"Eine Postkarte aus Lissabon, ohne Anlass.",
		"der stapel zeitschriften neben dem sofa.",
		"Ein Fenster in der Küche, irgendeins.",
	})
	if b.OK() {
		t.Fatal("die kleingeschriebene Karte ist durchgegangen")
	}
	if s := b.Schuldig(); len(s) != 1 || s[0] != 1 {
		t.Fatalf("falsche Karte beschuldigt: %v", s)
	}
}

// Und die echte Karte: Ein Mangel dort wird gemeldet, aber sie steht nicht auf
// der Liste der neu zu schreibenden - die schreibt man nicht neu.
func TestFormPruefenMeldetDieEchteKarte(t *testing.T) {
	b := FormPruefen("ein selbstgemachtes kochbuch", []string{
		"Eine Postkarte aus Lissabon, ohne Anlass.",
		"Der Stapel Zeitschriften neben dem Sofa.",
		"Ein Fenster in der Küche, irgendeins.",
	})
	if b.OK() {
		t.Fatal("die echte Karte ging ungeprüft durch")
	}
	if len(b.Schuldig()) != 0 {
		t.Fatalf("die echte Karte soll nicht neu geschrieben werden: %v", b.Schuldig())
	}
}

func TestUmlauteHerstellen(t *testing.T) {
	gut := []struct{ ein, aus string }{
		{"hoer auf zu snoozen", "hör auf zu snoozen"},
		{"Hoer auf damit", "Hör auf damit"},
		{"HOER AUF", "HÖR AUF"},
		{"das ist fuer dich", "das ist für dich"},
		{"ueber kurz oder lang", "über kurz oder lang"},
		{"waere schoen gewesen", "wäre schön gewesen"},
		{"macht grossen spass", "macht großen spaß"},
		{"auf der strasse", "auf der straße"},
		{"ich haette gerne tschuess gesagt", "ich hätte gerne tschüss gesagt"},
	}
	for _, f := range gut {
		if got := UmlauteHerstellen(f.ein); got != f.aus {
			t.Errorf("%q\n  bekam    %q\n  erwartet %q", f.ein, got, f.aus)
		}
	}
}

// Der eigentliche Test: Wörter, die eine allgemeine Regel oe→ö zerstören würde.
// Sie dürfen die Positivliste unter keinen Umständen berühren.
func TestUmlauteZerstoertNichts(t *testing.T) {
	unangetastet := []string{
		"Poesie", "Michael", "Israel", "Abenteuer", "Steuer", "aktuell", "Duell",
		"Zoe", "Aloe", "Koeffizient", "Boeing", "Joel", "Samuel", "Manuel",
		"queer", "Souvenir", "Museum", "Individuum", "Vakuum", "Jubilaeum",
		"dass", "Fluss", "Kuss", "Schloss", "Ross", "Bass", "Gasse", "Masse",
	}
	for _, w := range unangetastet {
		if got := UmlauteHerstellen(w); got != w {
			t.Errorf("%q wurde zu %q verändert", w, got)
		}
	}
	satz := "Michael liest Poesie im Museum, das Abenteuer war aktuell ein Duell."
	if got := UmlauteHerstellen(satz); got != satz {
		t.Errorf("ganzer Satz verändert:\n  %q", got)
	}
}

func TestNormalformMitUmlauten(t *testing.T) {
	got := ErsatzNormalform("hoer auf zu snoozen!!! mach ich selber nie")
	will := "Hör auf zu snoozen! Mach ich selber nie."
	if got != will {
		t.Fatalf("bekam %q, erwartet %q", got, will)
	}
}

// TestUmkreisenDurchEnthaltung hält den Fall fest, der beim Durchspielen am
// 12.09.2026 durchgerutscht ist: Die echte Antwort steckt vollständig in einer
// viel längeren Fälschung. Symmetrisch gemessen sind das 0.31 und damit
// unauffällig – trotzdem hätte die Karte dieselbe Antwort zweimal gezeigt.
func TestUmkreisenDurchEnthaltung(t *testing.T) {
	echt := "Der Stapel Zei."
	umkreisend := "Der Stapel Zeitschriften neben dem Sofa"

	if s := Aehnlichkeit(echt, umkreisend); s > SimMaxEcht {
		t.Fatalf("der Fall ist nicht mehr der beschriebene: symmetrisch %.2f", s)
	}
	b := Abstandsfenster(echt, []string{
		umkreisend,
		"Kartenspiele lernen, alleine, den ganzen Nachmittag",
		"Regenwuermer in der Jackentasche, frag nicht",
	})
	if b.NaeheOK {
		t.Fatal("eine Fälschung, welche die echte Antwort enthält, ist durchgegangen")
	}
	if len(b.Schuldig) != 1 || b.Schuldig[0] != 0 {
		t.Fatalf("falsche Karte beschuldigt: %v", b.Schuldig)
	}
}

// Und die Gegenrichtung: eine Fälschung, die kürzer ist als die echte Antwort
// und darin steckt.
func TestUmkreisenAndersherum(t *testing.T) {
	echt := "Eine zweite Kaffeemuehle, aber die erste mahlt zu grob"
	b := Abstandsfenster(echt, []string{
		"Ein Fenster in der Kueche, irgendeins",
		"Eine zweite Kaffeemuehle",
		"Regenwuermer in der Jackentasche, frag nicht",
	})
	if b.NaeheOK {
		t.Fatal("eine Fälschung, die in der echten Antwort steckt, ist durchgegangen")
	}
}

// Gegenprobe: gestreute Fälschungen dürfen nicht plötzlich an der neuen
// Prüfung hängenbleiben, auch wenn sie sehr unterschiedlich lang sind.
func TestKurzeAntwortBleibtDurchlaessig(t *testing.T) {
	b := Abstandsfenster("Geh raus!", []string{
		"Kochen, richtig kochen, nicht nur aufwaermen",
		"Der Stapel Zeitschriften neben dem Sofa",
		"Regenwuermer gesammelt und eingesteckt",
	})
	if !b.OK() {
		t.Fatalf("harmloser Satz abgelehnt: %s (max %.2f)", b.Grund, b.MaxZuEcht)
	}
}

// TestZweiGleicheKartenGehenNieHinaus: der Boden unter der Rückfallebene.
func TestZweiGleicheKartenGehenNieHinaus(t *testing.T) {
	echt := "Der Stapel Zeitschriften neben dem Sofa."
	// Genau der Fall aus dem Emulator: eine Fälschung ist die echte Antwort.
	if _, _, doppelt := Unzumutbar(echt, []string{
		"Eine zweite Kaffeemuehle, die erste mahlt zu grob.",
		echt,
		"Abends noch Nachrichten lesen, jedes Mal.",
	}); !doppelt {
		t.Fatal("die echte Antwort als Fälschung ist durchgegangen")
	}
	// Auch zwei gleiche Fälschungen untereinander.
	if _, _, doppelt := Unzumutbar(echt, []string{
		"Abends noch Nachrichten lesen, jedes Mal.",
		"Abends noch Nachrichten lesen, jedes Mal.",
		"Zu spät ins Bett, zu früh raus.",
	}); !doppelt {
		t.Fatal("zwei gleiche Fälschungen sind durchgegangen")
	}
	// Ein normaler Satz darf nicht hängenbleiben.
	if _, _, doppelt := Unzumutbar(echt, []string{
		"Eine zweite Kaffeemuehle, die erste mahlt zu grob.",
		"Abends noch Nachrichten lesen, jedes Mal.",
		"Zu spät ins Bett, zu früh raus.",
	}); doppelt {
		t.Fatal("ein harmloser Kartensatz wurde abgelehnt")
	}
}

// Alle vier Karten stehen nebeneinander: Eine, die dreimal so lang ist wie die
// anderen, ist an der Laenge erkannt, bevor jemand ein Wort davon liest.
func TestLaengenbruch(t *testing.T) {
	echt := "Nicht mit vollem Mund reden, das nervt wirklich." // 47 Zeichen
	min, max := Laengenfenster(echt)
	if min > 20 || max < 80 || max > 110 {
		t.Fatalf("fenster [%d..%d] fuer 47 zeichen ist unplausibel", min, max)
	}
	ok := []string{
		"Wenn alle gleichzeitig reden, versteht man niemanden mehr.",
		"Hände waschen, bevor es Essen gibt.",
		"Erst aufessen, dann aufstehen, das gilt bei mir noch.",
	}
	if b := Laengenbruch(echt, ok); len(b) != 0 {
		t.Fatalf("brauchbare laengen abgewiesen: %v", b)
	}
	// Die Faelschung, die im Betrieb aufgefallen ist: 168 Zeichen gegen 47.
	zulang := "Dass man beim Kochen die Töpfe nicht bis zum Rand füllt, habe ich " +
		"früher für Schikane gehalten und inzwischen zweimal eine Herdplatte " +
		"geputzt, die das Gegenteil beweist."
	if b := Laengenbruch(echt, []string{zulang}); len(b) != 1 {
		t.Fatalf("die zu lange karte ging durch (%d zeichen)", len([]rune(zulang)))
	}
	// Und ein Wortfetzen gegen einen ganzen Satz faellt nach unten heraus.
	if b := Laengenbruch(echt, []string{"Ja."}); len(b) != 1 {
		t.Fatal("die viel zu kurze karte ging durch")
	}
	// Bei einer sehr kurzen echten Antwort darf niemand an der Haelfte
	// scheitern - dort traegt der absolute Spielraum.
	kurz := "Geh raus!"
	if b := Laengenbruch(kurz, []string{"Mach die Tür zu.", "Lies ein Buch."}); len(b) != 0 {
		t.Fatalf("bei neun zeichen ist das fenster zu eng: %v", b)
	}
}
