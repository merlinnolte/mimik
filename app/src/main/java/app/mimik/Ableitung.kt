package app.mimik

/**
 * Alles, was sich aus dem Spielzustand ergibt – ohne einen einzigen
 * Android-Import.
 *
 * Warum getrennt: Diese Datei trägt die Entscheidungen (welcher Bildschirm,
 * welche Meldung, welche Zeile in der Lobby), und Entscheidungen gehören
 * geprüft. Ein Test dafür läuft in Millisekunden auf der JVM; dieselbe Frage am
 * Emulator kostet Minuten und beantwortet sie schlechter. `Speicher`, `Melder`
 * und `AppModel` treffen deshalb bewusst keine Entscheidung mehr – sie halten
 * Zustand und rufen hier an.
 */

enum class Bildschirm {
    Start, Laden, NameWaehlen, Intro, Tags, Lobby, Warteraum, Basis,
    Schreiben, Warten, Klone, Raten, Getippt, Aufloesung, Einstellungen,
}

/** Bis wohin ein Spieler die Auflösungen EINER Partie gesehen hat. */
data class Gesehen(val match: String = "", val bis: Int = 0)

/**
 * Was der Bildschirm braucht. Ein Wert, kein Objektgeflecht: So lässt sich
 * jeder Fall in einem Test hinschreiben.
 */
data class Sicht(
    val angemeldet: Boolean = false,
    val lobby: LobbyAus? = null,
    val partie: Spielzustand? = null,
    val offenePartie: String? = null,
    val introOffen: Boolean = false,
    val einstellungenOffen: Boolean = false,
    val gesehen: Map<String, Gesehen> = emptyMap(),
    /** Bis zu welcher Runde je Partie der Klonblick weggeklickt ist. */
    val klone: Map<String, Gesehen> = emptyMap(),
    val balkenLaeuftVoll: String? = null,
)

/**
 * Der Bildschirm ist eine Funktion von (Zustand, offene Partie).
 *
 * Das zweite Argument ist eine **Auswahl über einer Menge**, kein
 * Navigationszustand: Es kann nur Werte annehmen, die in der Lobby vorkommen,
 * und wird bei jedem Lesen dagegen geprüft. Verschwindet die Partie, fällt die
 * Auswahl weg und der Weg endet von selbst in der Lobby. Ausgewählt wird immer
 * über die ID, nie über einen Index – sonst springt die offene Partie weg,
 * sobald der Server die Liste anders sortiert.
 */
fun bildschirmFuer(s: Sicht): Bildschirm {
    if (!s.angemeldet) return Bildschirm.Start
    if (s.introOffen) return Bildschirm.Intro
    val l = s.lobby ?: return Bildschirm.Laden
    // Vor allem anderen: Ohne eindeutigen Namen zeigt jede Einladung und jede
    // Suche auf „eine von mehreren Personen" – das lässt sich nicht anzeigen,
    // nur verhindern.
    if (!l.name.eindeutig) return Bildschirm.NameWaehlen
    if (s.einstellungenOffen) return Bildschirm.Einstellungen
    if (l.tags.size < 10) return Bildschirm.Tags

    val zeile = s.offenePartie?.let { id -> l.partien.firstOrNull { it.partyId == id } }
        ?: return Bildschirm.Lobby
    // Die Partie ist gewählt, ihre Runden sind aber noch nicht da (oder gehören
    // noch zur vorigen Wahl). Warten, statt den falschen Bildschirm zu zeigen.
    val p = s.partie?.takeIf { it.partyId == zeile.partyId } ?: return Bildschirm.Laden
    if (p.party?.partner == null) return Bildschirm.Warteraum
    if (naechsteAufloesung(p, s.gesehen[p.partyId]) != null) return Bildschirm.Aufloesung
    val m = p.match ?: return Bildschirm.Basis
    if (m.ergebnis != "OFFEN") return Bildschirm.Basis
    val r = aktuelleRunde(p) ?: return Bildschirm.Basis
    return when {
        r.meineAntwort.isBlank() -> Bildschirm.Schreiben
        // Der Klonblick kommt VOR dem Raten und vor dem zweiten Warten: Die
        // eigenen vier Karten stehen, sobald MIMIK sie gebaut hat - also lange
        // bevor die andere Seite geantwortet hat. Genau dort ist die Wartezeit,
        // und genau dort gibt es etwas zu sehen.
        klonOffen(p, r, s.klone[p.partyId]) -> Bildschirm.Klone
        r.karten.isEmpty() || s.balkenLaeuftVoll == r.id -> Bildschirm.Warten
        r.meinTipp == null -> Bildschirm.Raten
        else -> Bildschirm.Getippt
    }
}

/**
 * Steht der Klonblick dieser Runde noch offen?
 *
 * Vier eigene Karten müssen da sein, und die Runde darf noch nicht
 * weggeklickt sein. Der Merker hängt am Match, nicht nur an der Nummer –
 * dieselbe Falle wie bei der Auflösung: Rundennummern fangen in jedem Match
 * wieder bei 1 an.
 */
fun klonOffen(p: Spielzustand, r: RundeAus, g: Gesehen?): Boolean {
    if (r.meineKarten.size != 4) return false
    val bis = if (g != null && g.match == p.match?.id.orEmpty()) g.bis else 0
    return r.nummer > bis
}

fun aktuelleRunde(p: Spielzustand): RundeAus? =
    p.runden.firstOrNull { it.zustand != "AUFGELOEST" }

/**
 * Die älteste aufgelöste Runde, deren Auflösung in DIESER Partie noch
 * offensteht.
 *
 * Der Merker hängt am Match, nicht nur an der Nummer: Rundennummern fangen in
 * jedem Match wieder bei 1 an, und ein Merker ohne Match hielte jede Auflösung
 * des zweiten Matches für gesehen. Und er hängt an der Partie, sonst
 * verschluckt Partie B die Auflösungen von Partie A.
 */
fun naechsteAufloesung(p: Spielzustand, g: Gesehen?): String? {
    val bis = if (g != null && g.match == p.match?.id.orEmpty()) g.bis else 0
    return p.runden
        .filter { it.zustand == "AUFGELOEST" && it.meinTipp != null && it.nummer > bis }
        .minByOrNull { it.nummer }?.id
}

// ------------------------------------------------------------------ Lobby ---

/**
 * Was eine Lobbyzeile verlangt. „Fällig" ist derselbe Begriff wie beim Melden –
 * eine Sache, die eine Handlung erwartet, und nicht bloß etwas Neues.
 */
val LobbyPartie.faellig: Boolean
    get() = dran == "schreiben" || dran == "raten" || dran == "aufgeloest"

/** Fälliges nach oben. Wer sechs Partien hat, soll nicht suchen. */
fun lobbyzeilen(l: LobbyAus): List<LobbyPartie> =
    l.partien.sortedWith(
        compareByDescending<LobbyPartie> { it.faellig }
            .thenBy { it.partner?.spitzname.orEmpty().lowercase() },
    )

fun etikett(dran: String): String = when (dran) {
    "schreiben" -> "ANTWORT FEHLT"
    "raten" -> "RATEN"
    "aufgeloest" -> "AUFLÖSUNG"
    "warten" -> "WARTET"
    "kein_match" -> "BEREIT"
    else -> "OFFENER CODE"
}

// ----------------------------------------------------------- Meldungen ---

data class Meldung(val partie: String, val kennung: String, val titel: String, val text: String)

data class Meldeplan(
    val zeigen: List<Meldung>,
    val loeschen: List<String>,
    val merker: Map<String, String>,
)

/**
 * Gemeldet wird genau zweierlei, und nur das:
 *   1. eine Antwort fehlt noch
 *   2. die Fälschungen liegen, es kann geraten werden
 *
 * Beides steht in `dran` der Lobby; der Server setzt „raten" erst, wenn die vier
 * Karten da sind. Damit fallen alle anderen Anlässe weg – die alte Meldung „Die
 * Party ist vollständig" ebenso wie jede über eine Auflösung. Eine Auflösung ist
 * kein Auftrag; sie wartet.
 *
 * Gegen die Meldung zu einer längst vergangenen Phase hilft der Merker JE
 * PARTIE plus die Löschliste: Steht eine Partie nicht mehr auf „fällig", fällt
 * ihre Meldung aus dem Schacht, statt dort weiter eine Phase zu behaupten, die
 * es nicht mehr gibt.
 */
fun meldeplan(l: LobbyAus, gemeldet: Map<String, String>): Meldeplan {
    val zeigen = mutableListOf<Meldung>()
    val merker = mutableMapOf<String, String>()
    for (p in l.partien) {
        if (p.dran != "schreiben" && p.dran != "raten") continue
        val kennung = "${p.dran}:${p.runde}"
        merker[p.partyId] = kennung
        if (gemeldet[p.partyId] == kennung) continue
        val wer = p.partner?.spitzname?.take(24).orEmpty().ifBlank { "Die andere Seite" }
        val (titel, text) = if (p.dran == "schreiben") {
            "MIMIK wartet" to "$wer · Deine Antwort fehlt noch."
        } else {
            "Vier Karten liegen bereit" to "$wer · Eine davon ist wirklich von ihr."
        }
        zeigen += Meldung(p.partyId, kennung, titel, text)
    }
    return Meldeplan(zeigen, (gemeldet.keys - merker.keys).toList(), merker)
}
