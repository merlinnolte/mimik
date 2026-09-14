package app.mimik

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.add
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import java.io.BufferedReader
import java.net.HttpURLConnection
import java.net.URL

// --- Was der Server schickt -------------------------------------------------

@Serializable
data class Spieler(val id: String = "", val spitzname: String = "")

@Serializable
data class PartyAus(
    val id: String = "",
    val code: String = "",
    val tags: List<String> = emptyList(),
    val partner: Spieler? = null,
)

@Serializable
data class Stand(val mensch: Int = 0, val mimik: Int = 0)

@Serializable
data class MatchAus(
    val id: String = "",
    val stand: Stand = Stand(),
    val ziel: Int = 10,
    val ergebnis: String = "OFFEN",
)

@Serializable
data class KarteAus(
    val pos: Int = 0,
    val text: String = "",
    @SerialName("ist_echt") val istEcht: Boolean? = null,
    /** Woraus MIMIK die Fälschung gebaut hat – nur im eigenen Satz besetzt. */
    val begruendung: String = "",
)

@Serializable
data class Aufloesung(
    @SerialName("echte_karte") val echteKarte: Int = 0,
    @SerialName("antwort_partner") val antwortPartner: String = "",
    @SerialName("mein_tipp_richtig") val meinTippRichtig: Boolean = false,
    @SerialName("partner_tipp") val partnerTipp: Int = 0,
    @SerialName("partner_tipp_text") val partnerTippText: String = "",
    @SerialName("meine_echte") val meineEchte: String = "",
    @SerialName("partner_richtig") val partnerRichtig: Boolean = false,
    val doppeltreffer: Boolean = false,
)

@Serializable
data class RundeAus(
    val id: String = "",
    val nummer: Int = 0,
    val frage: String = "",
    val zustand: String = "",
    @SerialName("meine_antwort") val meineAntwort: String = "",
    val karten: List<KarteAus> = emptyList(),
    @SerialName("mein_tipp") val meinTipp: Int? = null,
    @SerialName("mein_treffer") val meinTreffer: Boolean? = null,
    val aufloesung: Aufloesung? = null,
    val fehler: String = "",
    @SerialName("wartet_seit") val wartetSeit: Int = 0,
    /** Der Satz über MICH: eigene Antwort plus die drei Klone. */
    @SerialName("meine_karten") val meineKarten: List<KarteAus> = emptyList(),
)

@Serializable
data class DranPunkt(val was: String = "", val runde: String = "")

/**
 * Der Zustand EINER Partie. Seit es mehrere gibt, steht die Partie-ID mit drin:
 * Die App prüft damit, ob die Antwort noch zu der Partie gehört, die gerade
 * offen ist – sonst zeigte ein spät eintreffender Abgleich die Runde der
 * falschen Partie.
 */
@Serializable
data class Spielzustand(
    val spieler: Spieler? = null,
    /** Die Tags des Spielers – am Spieler, nicht an der Party. */
    val tags: List<String> = emptyList(),
    @SerialName("party_id") val partyId: String = "",
    val name: NameStand = NameStand(),
    val party: PartyAus? = null,
    val match: MatchAus? = null,
    val runden: List<RundeAus> = emptyList(),
    val dran: List<DranPunkt> = emptyList(),
)

/** Ob der eigene Spitzname eindeutig ist. Nur dann ist man auffindbar. */
@Serializable
data class NameStand(val eindeutig: Boolean = true, val beansprucht: Boolean = true)

/** Eine Zeile der Lobby: eine Partie und was dort ansteht. */
@Serializable
data class LobbyPartie(
    @SerialName("party_id") val partyId: String = "",
    val partner: Spieler? = null,
    val testpartie: Boolean = false,
    val code: String = "",
    val stand: Stand = Stand(),
    val ziel: Int = 10,
    val ergebnis: String = "",
    /** schreiben | raten | warten | aufgeloest | kein_match | kein_partner */
    val dran: String = "",
    val runde: String = "",
    val ungesehen: Int = 0,
)

@Serializable
data class EinladungAus(
    val id: String = "",
    val zustand: String = "",
    val gegenueber: Spieler = Spieler(),
    @SerialName("erstellt_am") val erstelltAm: String = "",
)

@Serializable
data class Einladungen(
    val eingehend: List<EinladungAus> = emptyList(),
    val ausgehend: List<EinladungAus> = emptyList(),
)

@Serializable
data class LobbyAus(
    val spieler: Spieler? = null,
    val tags: List<String> = emptyList(),
    val name: NameStand = NameStand(),
    val partien: List<LobbyPartie> = emptyList(),
    val einladungen: Einladungen = Einladungen(),
)

@Serializable
data class TrefferAus(val treffer: List<Spieler> = emptyList())

@Serializable
data class MerkmalAus(
    val merkmal: String = "",
    val wert: String = "",
    val stand: String = "",
    val belege: Int = 0,
    val wider: Int = 0,
    val beleg: String = "",
)

@Serializable
data class GeraetAus(val spieler: Spieler = Spieler(), val token: String = "")

@Serializable
data class PartyNeu(@SerialName("party_id") val partyId: String = "", val code: String = "")

@Serializable
data class TagsAus(
    val vorschlaege: List<String> = emptyList(),
    val gewaehlt: List<String> = emptyList(),
    val mindestens: Int = 10,
    /** Ob nach diesen Vorschlägen noch weitere im Vorrat liegen. */
    val mehr: Boolean = false,
)

@Serializable
data class TippAus(
    val richtig: Boolean = false,
    @SerialName("runde_beendet") val rundeBeendet: Boolean = false,
    val stand: Stand = Stand(),
    val ergebnis: String = "OFFEN",
)

@Serializable
data class DossierAus(
    val fakten: List<String> = emptyList(),
    @SerialName("verbrauchte_themen") val verbrauchteThemen: List<String> = emptyList(),
    val tags: List<String> = emptyList(),
    val profil: List<MerkmalAus> = emptyList(),
)

@Serializable
private data class FehlerAus(val fehler: String = "")

class NetzFehler(val code: Int, val text: String) : Exception("$code: $text")

/**
 * Sehr kleiner HTTP-Klient auf HttpURLConnection. Bewusst ohne Bibliothek: Die
 * Schnittstelle hat zwölf Endpunkte, alle mit demselben Muster.
 */
class Netz(private var basis: String, private var token: String) {

    private val json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    fun setze(basis: String, token: String) {
        this.basis = basis.trimEnd('/')
        this.token = token
    }

    fun hatToken() = token.isNotBlank()

    private fun ruf(methode: String, pfad: String, koerper: String?): String {
        val verb = URL(basis + pfad).openConnection() as HttpURLConnection
        verb.requestMethod = methode
        verb.connectTimeout = 10_000
        verb.readTimeout = 30_000
        verb.setRequestProperty("Accept", "application/json")
        if (token.isNotBlank()) verb.setRequestProperty("Authorization", "Bearer $token")
        if (koerper != null) {
            verb.doOutput = true
            verb.setRequestProperty("Content-Type", "application/json; charset=utf-8")
            verb.outputStream.use { it.write(koerper.toByteArray()) }
        }
        val code = verb.responseCode
        val strom = if (code in 200..299) verb.inputStream else verb.errorStream
        val text = strom?.bufferedReader()?.use(BufferedReader::readText).orEmpty()
        verb.disconnect()
        if (code !in 200..299) {
            val grund = runCatching { json.decodeFromString<FehlerAus>(text).fehler }
                .getOrNull().orEmpty().ifBlank { text.take(200) }
            throw NetzFehler(code, grund)
        }
        return text
    }

    /**
     * Suchbegriffe gehoeren in der Adresse kodiert. Ein Leerzeichen oder ein
     * Umlaut im Namen wuerde die Anfrage sonst zerlegen - und ein "&" waere ein
     * zweiter Parameter.
     */
    private fun enkodiere(x: String): String =
        java.net.URLEncoder.encode(x, "UTF-8").replace("+", "%20")

    private inline fun <reified T> hole(pfad: String): T =
        json.decodeFromString(ruf("GET", pfad, null))

    private inline fun <reified T> sende(methode: String, pfad: String, koerper: String? = null): T =
        json.decodeFromString(ruf(methode, pfad, koerper))

    /** Die Einladung ist nur nötig, wenn der Server MIMIK_EINLADUNG gesetzt hat. */
    fun geraetAnlegen(spitzname: String, einladung: String): GeraetAus =
        sende(
            "POST", "/v1/devices",
            buildJsonObject {
                put("spitzname", spitzname)
                put("einladung", einladung)
            }.toString(),
        )

    fun partyAnlegen(): PartyNeu = sende("POST", "/v1/parties", "{}")

    fun partyBeitreten(code: String): PartyNeu =
        sende("POST", "/v1/parties/join", buildJsonObject { put("code", code) }.toString())

    fun partyVerlassen(partie: String): String =
        ruf("POST", "/v1/parties/$partie/verlassen", "{}")

    fun lobby(): LobbyAus = hole("/v1/lobby")

    fun spielerSuchen(q: String): TrefferAus = hole("/v1/spieler?q=" + enkodiere(q))

    fun einladen(an: String): String =
        ruf("POST", "/v1/einladungen", buildJsonObject { put("an", an) }.toString())

    fun einladungAnnehmen(id: String): String =
        ruf("POST", "/v1/einladungen/$id/annehmen", "{}")

    fun einladungAblehnen(id: String): String =
        ruf("POST", "/v1/einladungen/$id/ablehnen", "{}")

    fun einladungZurueckziehen(id: String): String = ruf("DELETE", "/v1/einladungen/$id", null)

    /** ab = wie viele Vorschläge dieses Gerät schon gesehen hat. */

    fun tags(ab: Int = 0): TagsAus = hole("/v1/tags?ab=$ab")

    fun tagsSetzen(tags: List<String>): TagsAus = sende(
        "PUT", "/v1/tags",
        buildJsonObject { put("tags", buildJsonArray { tags.forEach { add(it) } }) }.toString(),
    )

    fun zustand(partie: String): Spielzustand = hole("/v1/parties/$partie/state")

    fun matchAnlegen(partie: String): String = ruf("POST", "/v1/parties/$partie/matches", "{}")

    fun matchAbbrechen(partie: String): String = ruf("POST", "/v1/parties/$partie/abbrechen", "{}")

    /**
     * Nur der rohe Text. Die saubere Fassung schreibt MIMIK, zusammen mit den
     * Fälschungen – damit alle vier Karten dieselbe Schreibweise haben.
     */
    fun antworten(runde: String, original: String): String =
        ruf(
            "POST", "/v1/rounds/$runde/answer",
            buildJsonObject { put("original", original) }.toString(),
        )

    fun raten(runde: String, pos: Int): TippAus =
        sende("POST", "/v1/rounds/$runde/guess", buildJsonObject { put("pos", pos) }.toString())

    fun dossier(): DossierAus = hole("/v1/dossier")

    fun dossierLoeschen(): String = ruf("DELETE", "/v1/dossier", null)

    fun umbenennen(spitzname: String): String =
        ruf("POST", "/v1/me/name", buildJsonObject { put("spitzname", spitzname) }.toString())

    fun kontoLoeschen(bestaetigung: String): String =
        ruf(
            "POST", "/v1/me/delete",
            buildJsonObject { put("bestaetigung", bestaetigung) }.toString(),
        )
}
