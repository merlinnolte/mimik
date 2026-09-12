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
)

@Serializable
data class Aufloesung(
    @SerialName("echte_karte") val echteKarte: Int = 0,
    @SerialName("antwort_partner") val antwortPartner: String = "",
    @SerialName("mein_tipp_richtig") val meinTippRichtig: Boolean = false,
    @SerialName("partner_tipp") val partnerTipp: Int = 0,
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
)

@Serializable
data class DranPunkt(val was: String = "", val runde: String = "")

@Serializable
data class Spielzustand(
    val spieler: Spieler? = null,
    /** Die Tags des Spielers – am Spieler, nicht an der Party. */
    val tags: List<String> = emptyList(),
    val party: PartyAus? = null,
    val match: MatchAus? = null,
    val runden: List<RundeAus> = emptyList(),
    val dran: List<DranPunkt> = emptyList(),
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

    /** ab = wie viele Vorschläge dieses Gerät schon gesehen hat. */
    fun tags(ab: Int = 0): TagsAus = hole("/v1/tags?ab=$ab")

    fun tagsSetzen(tags: List<String>): TagsAus = sende(
        "PUT", "/v1/tags",
        buildJsonObject { put("tags", buildJsonArray { tags.forEach { add(it) } }) }.toString(),
    )

    fun zustand(): Spielzustand = hole("/v1/state")

    fun matchAnlegen(): String = ruf("POST", "/v1/matches", "{}")

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
