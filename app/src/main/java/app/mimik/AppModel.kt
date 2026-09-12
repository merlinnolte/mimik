package app.mimik

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

enum class Bildschirm {
    Start, Laden, Intro, Tags, Party, Basis, Schreiben, Warten, Raten, Getippt, Aufloesung,
    Einstellungen,
}

class AppModel(app: Application) : AndroidViewModel(app) {

    private val speicher = Speicher(app)
    private val netz = Netz(speicher.server, speicher.token)

    var palette by mutableStateOf(paletteMit(speicher.palette)); private set
    var server by mutableStateOf(speicher.server)
    var zustand by mutableStateOf<Spielzustand?>(null); private set
    var laden by mutableStateOf(false); private set
    var fehler by mutableStateOf<String?>(null)
    var tagsAuswahl by mutableStateOf<TagsAus?>(null); private set
    var gewaehlteTags by mutableStateOf<Set<String>>(emptySet())
    var vorschau by mutableStateOf<NormalformAus?>(null)
    var letzterTreffer by mutableStateOf<Boolean?>(null)

    /**
     * Die Auflösung der gerade beendeten Runde. Ohne dieses Feld würde sie
     * übersprungen: Sobald beide getippt haben, ist die Runde aufgelöst und die
     * nächste rückt nach – der Moment, auf den man gewartet hat, wäre weg.
     */
    var zeigeAufloesung by mutableStateOf<String?>(null)
        private set
    private var gesehenBis by mutableStateOf(speicher.gesehenBis)

    /** Läuft einmal nach der Anmeldung, danach nur noch auf Wunsch. */
    var introOffen by mutableStateOf(false); private set
    var einstellungenOffen by mutableStateOf(false); private set

    /**
     * Muss Compose-State sein, nicht nur ein Feld im Netz-Objekt: bildschirm
     * kehrt bei fehlendem Token früh zurück und liest dabei zustand gar nicht.
     * Compose abonniert dann nichts und rendert nach dem Anmelden nie neu.
     */
    private var tokenState by mutableStateOf(speicher.token)

    val angemeldet: Boolean get() = tokenState.isNotBlank()

    val spitzname: String get() = zustand?.spieler?.spitzname.orEmpty()

    /**
     * Es gibt immer genau eine Runde, die dran ist. Das Spiel ist eine Schleife:
     * Frage, beide antworten, beide raten, Auflösung, nächste Frage. Mehrere
     * Runden gleichzeitig anzuzeigen hat das Datenmodell gespiegelt, nicht das
     * Spielgefühl.
     */
    val aktuelleRunde: RundeAus?
        get() = zustand?.runden?.firstOrNull { it.zustand != "AUFGELOEST" }

    /**
     * Der Bildschirm ergibt sich aus dem Zustand, nicht aus Navigation.
     *
     * Die Reihenfolge ist die des Onboardings: anmelden, Intro, Tags, Party.
     * Die Tags stehen bewusst vor der Party – was MIMIK über einen weiß, hängt
     * am Spieler und überlebt jede Party.
     */
    val bildschirm: Bildschirm
        get() {
            if (!angemeldet) return Bildschirm.Start
            if (introOffen) return Bildschirm.Intro
            if (einstellungenOffen) return Bildschirm.Einstellungen
            // Erst wenn der Server geantwortet hat, steht fest, wo es weitergeht.
            // Ohne diesen Zwischenschritt blitzt beim Start jedes Mal kurz die
            // Tagauswahl auf, die das Gerät längst hinter sich hat.
            val z = zustand ?: return Bildschirm.Laden
            if (z.tags.size < 10) return Bildschirm.Tags
            val party = z.party ?: return Bildschirm.Party
            if (party.partner == null) return Bildschirm.Party
            if (zeigeAufloesung != null) return Bildschirm.Aufloesung
            val m = z.match ?: return Bildschirm.Basis
            if (m.ergebnis != "OFFEN") return Bildschirm.Basis
            val r = aktuelleRunde ?: return Bildschirm.Basis
            return when {
                r.meineAntwort.isBlank() -> Bildschirm.Schreiben
                r.karten.isEmpty() -> Bildschirm.Warten
                r.meinTipp == null -> Bildschirm.Raten
                else -> Bildschirm.Getippt
            }
        }

    /** Das Zahnrad gehört nicht auf den Anmelde- und nicht auf den Introschirm. */
    val zahnradSichtbar: Boolean
        get() = bildschirm !in
            setOf(Bildschirm.Start, Bildschirm.Laden, Bildschirm.Intro, Bildschirm.Einstellungen)

    fun runde(id: String?): RundeAus? = zustand?.runden?.firstOrNull { it.id == id }

    /**
     * Nach jedem Zustandsabgleich prüfen, ob eine Runde aufgelöst wurde, die
     * dieser Spieler noch nicht gesehen hat. Ohne das bekommt nur die Auflösung
     * zu sehen, wer als Zweiter tippt – wer zuerst tippt, überspringt sie und
     * landet wortlos in der nächsten Frage.
     */
    private fun zustandUebernehmen(z: Spielzustand) {
        zustand = z
        if (zeigeAufloesung != null) return
        val faellig = z.runden
            .filter { it.zustand == "AUFGELOEST" && it.meinTipp != null && it.nummer > gesehenBis }
            .minByOrNull { it.nummer }
        if (faellig != null) zeigeAufloesung = faellig.id
    }

    private fun imHintergrund(arbeit: suspend () -> Unit) {
        viewModelScope.launch {
            laden = true
            fehler = null
            try {
                withContext(Dispatchers.IO) { arbeit() }
            } catch (e: NetzFehler) {
                fehler = e.text
            } catch (e: Exception) {
                fehler = e.message ?: "unbekannter Fehler"
            } finally {
                laden = false
            }
        }
    }

    fun paletteWaehlen(id: String) {
        speicher.palette = id
        palette = paletteMit(id)
    }

    fun serverSetzen(url: String) {
        server = url
        speicher.server = url
        netz.setze(speicher.server, speicher.token)
    }

    fun anmelden(spitzname: String, einladung: String = "") = imHintergrund {
        val aus = netz.geraetAnlegen(spitzname, einladung.trim())
        speicher.token = aus.token
        netz.setze(speicher.server, aus.token)
        val z = netz.zustand()
        withContext(Dispatchers.Main) {
            tokenState = aus.token
            introOffen = !speicher.introGesehen
            zustandUebernehmen(z)
            Melder.planen(getApplication())
        }
    }

    fun introBeenden() {
        speicher.introGesehen = true
        introOffen = false
    }

    fun introZeigen() {
        einstellungenOffen = false
        introOffen = true
    }

    fun einstellungen(offen: Boolean) {
        fehler = null
        einstellungenOffen = offen
    }

    fun aktualisieren() = imHintergrund {
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    /** Solange MIMIK arbeitet, alle zehn Sekunden nachfragen. */
    fun beobachten() {
        viewModelScope.launch {
            while (true) {
                delay(10_000)
                val arbeitet = zustand?.runden?.any { it.zustand == "MIMIK_ARBEITET" } == true
                if (angemeldet && arbeitet) {
                    runCatching { withContext(Dispatchers.IO) { netz.zustand() } }
                        .onSuccess { zustandUebernehmen(it) }
                }
            }
        }
    }

    fun partyAnlegen() = imHintergrund {
        netz.partyAnlegen()
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    fun partyBeitreten(code: String) = imHintergrund {
        netz.partyBeitreten(code.trim().uppercase())
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    fun tagsLaden() = imHintergrund {
        val t = netz.tags()
        tagsAuswahl = t
        gewaehlteTags = t.gewaehlt.toSet()
    }

    fun tagsSpeichern() = imHintergrund {
        netz.tagsSetzen(gewaehlteTags.toList())
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    fun matchStarten() = imHintergrund {
        netz.matchAnlegen()
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    fun vorschauHolen(runde: String, text: String) = imHintergrund {
        vorschau = netz.normalform(runde, text)
    }

    fun antwortSenden(runde: String, original: String, normalform: String) = imHintergrund {
        netz.antworten(runde, original, normalform)
        val z = netz.zustand()
        withContext(Dispatchers.Main) {
            vorschau = null
            zustandUebernehmen(z)
        }
    }

    fun tippSenden(runde: String, pos: Int) = imHintergrund {
        val aus = netz.raten(runde, pos)
        val z = netz.zustand()
        withContext(Dispatchers.Main) {
            letzterTreffer = aus.richtig
            zustandUebernehmen(z)
        }
    }

    /** Auflösung wegklicken und in die nächste Runde gehen. */
    fun weiter() {
        runde(zeigeAufloesung)?.let {
            gesehenBis = maxOf(gesehenBis, it.nummer)
            speicher.gesehenBis = gesehenBis
        }
        zeigeAufloesung = null
        vorschau = null
        aktualisieren()
    }

    // ------------------------------------------------------------ Konto ---

    fun umbenennen(neu: String) = imHintergrund {
        netz.umbenennen(neu.trim())
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    /** MIMIK vergisst, was sie gelernt hat. Die Tags bleiben – sie sind eine
     *  Einstellung, kein Gelerntes. */
    fun dossierLoeschen() = imHintergrund {
        netz.dossierLoeschen()
        val z = netz.zustand()
        withContext(Dispatchers.Main) { zustandUebernehmen(z) }
    }

    /** Danach ist auf dem Server nichts mehr von diesem Spieler übrig, und die
     *  App steht wieder am Anfang. */
    fun kontoLoeschen(bestaetigung: String) = imHintergrund {
        netz.kontoLoeschen(bestaetigung.trim())
        withContext(Dispatchers.Main) {
            Melder.abbestellen(getApplication())
            val adresse = speicher.server
            speicher.leeren()
            speicher.server = adresse
            netz.setze(adresse, "")
            tokenState = ""
            zustand = null
            tagsAuswahl = null
            gewaehlteTags = emptySet()
            zeigeAufloesung = null
            gesehenBis = 0
            einstellungenOffen = false
        }
    }
}
