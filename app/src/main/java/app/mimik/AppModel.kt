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

class AppModel(app: Application) : AndroidViewModel(app) {

    // wandern() vor allem anderen: Netz wird gleich mit speicher.server
    // gebaut, und genau diese Adresse zieht die Wanderung gerade nach.
    private val speicher = Speicher(app).also { it.wandern() }
    private val netz = Netz(speicher.server, speicher.token)

    var palette by mutableStateOf(paletteMit(speicher.palette)); private set
    var server by mutableStateOf(speicher.server)
    /** Die Übersicht: alle Partien, Einladungen, der eigene Namensstand. */
    var lobby by mutableStateOf<LobbyAus?>(null); private set

    /** Der Zustand der GERADE OFFENEN Partie. Null heißt: noch nicht geladen. */
    var zustand by mutableStateOf<Spielzustand?>(null); private set

    /**
     * Welche Partie offen ist – eine Auswahl, keine Navigation. Der Bildschirm
     * ergibt sich weiterhin allein aus dem Zustand; dieses Feld sagt nur, aus
     * welcher der Partien. Es liegt im Speicher, damit ein Prozesstod niemanden
     * mitten im Schreiben in die Lobby wirft.
     */
    var offenePartie by mutableStateOf(speicher.offenePartie.ifBlank { null })
        private set

    /** Trefferliste der Namenssuche. */
    var treffer by mutableStateOf<List<Spieler>>(emptyList()); private set
    var sichtbar by mutableStateOf(true); private set
    var laden by mutableStateOf(false); private set
    var fehler by mutableStateOf<String?>(null)
    /**
     * Die Tagauswahl besteht aus drei Teilen, und sie getrennt zu halten ist
     * der Grund, warum "Weitere" jetzt funktioniert.
     *
     * gespeicherteTags  was der Server kennt
     * vorschlaege       was dieses Gerät bisher angeboten bekommen hat, wachsend
     * gewaehlteTags     was der Finger angetippt hat, noch nicht abgeschickt
     *
     * Vorher gab es nur eine Antwort vom Server, und jedes Nachladen setzte die
     * Auswahl auf den Stand des Servers zurück – im Onboarding also auf leer.
     */
    var gespeicherteTags by mutableStateOf<List<String>>(emptyList()); private set
    var vorschlaege by mutableStateOf<List<String>>(emptyList()); private set
    var mehrTags by mutableStateOf(false); private set
    var tagsGeladen by mutableStateOf(false); private set
    var gewaehlteTags by mutableStateOf<Set<String>>(emptySet())

    /** Was auf dem Bildschirm steht: Gespeichertes zuerst, dann Vorschläge. */
    val sichtbareTags: List<String>
        get() = (gespeicherteTags + vorschlaege).distinct()
    var letzterTreffer by mutableStateOf<Boolean?>(null)

    /**
     * Bis wohin die Auflösungen je Partie gesehen sind.
     *
     * Kein gesetztes "zeige jetzt die Auflösung" mehr: Was zu sehen ist, leitet
     * naechsteAufloesung daraus ab. Ein gesetzter Bildschirm war bei einer
     * Partie noch zu halten; bei mehreren wäre er ein zweiter Wahrheitsstand
     * neben dem Spielzustand.
     */
    private var gesehen by mutableStateOf(speicher.gesehen)
    private var klone by mutableStateOf(speicher.klone)

    /**
     * Die eben abgegebenen Stimmen zu Fragen, je Runden-ID – nur für die Zeit
     * zwischen dem Tippen und dem nächsten Abgleich.
     *
     * Absichtlich NICHT im Speicher: Die Wahrheit steht auf dem Server und
     * kommt mit RundeAus.meinUrteil zurück. Hier liegt nur die Ungeduld.
     */
    var urteile by mutableStateOf(emptyMap<String, Int>()); private set

    /**
     * Die Runde, deren Fortschrittsbalken noch volläuft. Die Karten sind schon
     * da – der Bildschirm bleibt trotzdem einen Moment stehen, damit der Balken
     * nicht mitten im Lauf verschwindet.
     */
    var balkenLaeuftVoll by mutableStateOf<String?>(null)
        private set

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

    val spitzname: String
        get() = lobby?.spieler?.spitzname ?: zustand?.spieler?.spitzname.orEmpty()

    private val sicht: Sicht
        get() = Sicht(
            angemeldet = angemeldet,
            lobby = lobby,
            partie = zustand,
            offenePartie = offenePartie,
            introOffen = introOffen,
            einstellungenOffen = einstellungenOffen,
            gesehen = gesehen,
            klone = klone,
            balkenLaeuftVoll = balkenLaeuftVoll,
        )

    /** Der Bildschirm steht in Ableitung.kt – dort ist er prüfbar. */
    val bildschirm: Bildschirm get() = bildschirmFuer(sicht)

    val aktuelleRunde: RundeAus?
        get() = zustand?.let { aktuelleRunde(it) }

    /** Die Auflösung, die gerade ansteht – abgeleitet, nicht gesetzt. */
    val zeigeAufloesung: String?
        get() = zustand?.let { naechsteAufloesung(it, gesehen[it.partyId]) }

    val zeilen: List<LobbyPartie> get() = lobby?.let { lobbyzeilen(it) } ?: emptyList()
    val einladungen: Einladungen get() = lobby?.einladungen ?: Einladungen()

    fun einladungOffenMit(pid: String): Boolean =
        einladungen.ausgehend.any { it.gegenueber.id == pid }

    /** Steckt der Spieler in einer vollständigen Party? */
    val inParty: Boolean
        get() = zustand?.party?.partner != null

    val partnerName: String
        get() = zustand?.party?.partner?.spitzname.orEmpty()

    /** Läuft gerade ein Spiel, das man abbrechen könnte? */
    val matchLaeuft: Boolean
        get() = zustand?.match?.ergebnis == "OFFEN"

    /**
     * Wie oft nachgefragt wird – nach dem Bildschirm, nicht nach der Zahl der
     * Partien: Die Lobby kommt in einem Abruf, sechs Partien kosten so viel wie
     * eine. Drei Sekunden nur dort, wo etwas in Sekunden passiert; im
     * Hintergrund gar nicht, und auf Schreiben und Raten auch nicht – dort ist
     * der Mensch dran, und ein Abgleich währenddessen kann nur stören.
     */
    private val takt: Long
        get() = when {
            !sichtbar || !angemeldet -> 0L
            bildschirm == Bildschirm.Warten -> 3_000L
            bildschirm == Bildschirm.Getippt -> 5_000L
            bildschirm == Bildschirm.Warteraum -> 5_000L
            bildschirm == Bildschirm.Lobby -> 12_000L
            // Auf dem Klonblick wird gelesen, nicht gewartet.
            bildschirm == Bildschirm.Klone -> 0L
            bildschirm == Bildschirm.Laden -> 3_000L
            else -> 0L
        }

    /** Das Zahnrad gehört nicht auf den Anmelde- und nicht auf den Introschirm. */
    val zahnradSichtbar: Boolean
        get() = bildschirm !in setOf(
            Bildschirm.Start, Bildschirm.Laden, Bildschirm.Intro,
            Bildschirm.Einstellungen, Bildschirm.NameWaehlen,
        )

    /** Der Rückweg in die Übersicht – überall dort, wo eine Partie offen ist. */
    val zurueckSichtbar: Boolean
        get() = offenePartie != null && bildschirm !in setOf(
            Bildschirm.Start, Bildschirm.Intro, Bildschirm.Einstellungen, Bildschirm.NameWaehlen,
        )

    fun runde(id: String?): RundeAus? = zustand?.runden?.firstOrNull { it.id == id }

    /**
     * Übernimmt den Zustand der offenen Partie.
     *
     * Seiteneffekt ist hier nur noch eines: das kurze Halten des
     * Wartebildschirms, damit der Fortschrittsbalken sichtbar vollläuft. Das
     * ist Zeitsteuerung, kein Zustand. Was angezeigt wird, leitet
     * bildschirmFuer ab.
     */
    private fun zustandUebernehmen(z: Spielzustand) {
        val vorher = zustand
        val alt = vorher?.runden?.firstOrNull { it.id == aktuelleRunde?.id }
        zustand = z
        val jetzt = aktuelleRunde
        if (vorher?.partyId == z.partyId && jetzt != null && alt != null &&
            alt.karten.isEmpty() && jetzt.karten.isNotEmpty() && jetzt.meinTipp == null
        ) {
            balkenLaeuftVoll = jetzt.id
            viewModelScope.launch {
                delay(900)
                balkenLaeuftVoll = null
            }
        }
    }

    /**
     * Übernimmt die Lobby und kürzt dabei den Meldemerker.
     *
     * Kürzen, weil eine erledigte Phase sonst als "schon gemeldet" stehen
     * bleibt und die nächste Meldung derselben Art verschluckt – das war der
     * Fall nach jedem Zug, den man im Vordergrund gemacht hat. Nicht
     * erweitern, weil Zuschauen keine Meldung ist: Wer die App öffnet, die
     * offene Runde sieht und wieder weggeht, soll später trotzdem angestupst
     * werden.
     */
    private fun lobbyUebernehmen(l: LobbyAus) {
        lobby = l
        val faellig = l.partien.filter { it.dran == "schreiben" || it.dran == "raten" }
            .map { it.partyId }.toSet()
        val alt = speicher.gemeldet
        val erledigt = alt.keys - faellig
        if (erledigt.isEmpty()) return
        erledigt.forEach { Melder.wegnehmen(getApplication(), it) }
        speicher.gemeldet = alt.filterKeys { it in faellig }
    }


    /**
     * Nach jedem Zug: die Lobby und - falls eine Partie offen ist - deren
     * Zustand. Zwei Abrufe, weil die Lobby keine Runden traegt: Sie soll auch
     * bei zehn Partien eine Abfrage bleiben.
     */
    private suspend fun nachladen() {
        val l = netz.lobby()
        val z = offenePartie?.let { netz.zustand(it) }
        withContext(Dispatchers.Main) {
            lobbyUebernehmen(l)
            if (z != null) zustandUebernehmen(z)
        }
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
                fehler = "Keine Verbindung zum Server."
            } finally {
                laden = false
            }
        }
    }

    fun paletteWaehlen(id: String) {
        speicher.palette = id
        palette = paletteMit(id)
    }

    /**
     * Nimmt eine eingetippte Adresse an, wenn sie eine ist. Der Rückgabewert
     * sagt es: Anmelden gegen eine kaputte Adresse läuft sonst in einen
     * Verbindungsfehler, und der erklärt niemandem, dass der Tippfehler drei
     * Felder weiter oben steht.
     */
    fun serverSetzen(eingabe: String): Boolean {
        val adresse = serverNormalform(eingabe)
        if (adresse == null) {
            fehler = "Das ist keine Serveradresse."
            return false
        }
        server = adresse
        speicher.server = adresse
        netz.setze(adresse, speicher.token)
        return true
    }

    fun anmelden(spitzname: String, einladung: String = "") = imHintergrund {
        val aus = netz.geraetAnlegen(spitzname, einladung.trim())
        speicher.token = aus.token
        netz.setze(speicher.server, aus.token)
        val l = netz.lobby()
        withContext(Dispatchers.Main) {
            tokenState = aus.token
            introOffen = !speicher.introGesehen
            lobbyUebernehmen(l)
            Melder.planen(getApplication())
        }
    }

    /**
     * Anmelden mit einem Schlüssel statt mit einem Namen – derselbe Spieler auf
     * einem neuen Gerät oder nach einer Neuinstallation. Siehe Umzug.kt.
     *
     * Die Lobby ist die Probe: Sie kommt nur, wenn der Schlüssel gilt. Erst
     * danach wird er gespeichert – ein falscher darf den vorhandenen nicht
     * überschreiben.
     */
    fun mitSchluessel(eingabe: String) = imHintergrund {
        val s = schluesselLesen(eingabe)
            ?: throw NetzFehler(0, "Das sieht nicht nach einem Schlüssel aus.")
        val vorher = speicher.token
        netz.setze(speicher.server, s)
        val l = try {
            netz.lobby()
        } catch (e: NetzFehler) {
            netz.setze(speicher.server, vorher)
            throw if (e.code == 401) {
                NetzFehler(401, "Dieser Schlüssel gehört zu keinem Konto auf diesem Server.")
            } else {
                e
            }
        } catch (e: Exception) {
            netz.setze(speicher.server, vorher)
            throw e
        }
        speicher.token = s
        // Wer umzieht, hat das Intro gesehen. Es noch einmal vorzuspielen
        // wäre die falsche Begrüßung für jemanden, der schon mitspielt.
        speicher.introGesehen = true
        withContext(Dispatchers.Main) {
            tokenState = s
            lobbyUebernehmen(l)
            Melder.planen(getApplication())
        }
    }

    // ------------------------------------------------------- Aktualisierung ---

    /** Was in diesem APK steckt. Steht in den Einstellungen. */
    val fassung: String get() = BuildConfig.VERSION_NAME

    /** Der eigene Schlüssel, für den Umzug auf ein anderes Gerät. */
    val schluessel: String get() = speicher.token

    /** Die neuere Fassung, falls es eine gibt. */
    var angeboten by mutableStateOf<Fassung?>(null); private set
    var fassungSucht by mutableStateOf(false); private set

    /** Prozent, solange geladen wird. Null heißt: es lädt nichts. */
    var fassungLaedt by mutableStateOf<Int?>(null); private set

    /** Was zur Suche zu sagen ist – absichtlich nicht `fehler`: Ein Update, das
     *  nicht erreichbar ist, ist kein Spielfehler und gehört nicht überall hin. */
    var fassungHinweis by mutableStateOf<String?>(null)

    /**
     * Sucht nach einer neueren Fassung. `vonSelbst` ist der Griff beim Start:
     * Er hält den Abstand ein und schweigt, wenn nichts da ist oder die
     * angebotene Fassung schon weggeklickt wurde. Auf Knopfdruck gilt beides
     * nicht – wer fragt, will eine Antwort.
     */
    fun fassungSuchen(vonSelbst: Boolean = false) {
        if (fassungSucht || fassungLaedt != null) return
        val jetzt = System.currentTimeMillis()
        if (vonSelbst && jetzt - speicher.fassungGesucht < Aktualisierung.ABSTAND) return
        viewModelScope.launch {
            fassungSucht = true
            if (!vonSelbst) fassungHinweis = null
            val f = withContext(Dispatchers.IO) { Aktualisierung.suchen(fassung) }
            speicher.fassungGesucht = jetzt
            fassungSucht = false
            when {
                f == null -> if (!vonSelbst) fassungHinweis = "Keine neuere Fassung."
                vonSelbst && f.name == speicher.fassungUebergangen -> Unit
                else -> angeboten = f
            }
        }
    }

    /** „Später" – dieselbe Fassung fragt nicht noch einmal von selbst. */
    fun fassungUebergehen() {
        speicher.fassungUebergangen = angeboten?.name.orEmpty()
        angeboten = null
    }

    /**
     * Lädt das APK und hält es dem System hin. Den letzten Schritt tut der
     * Mensch: MIMIK tauscht sich nicht still selbst aus.
     */
    fun fassungHolen() {
        val f = angeboten ?: return
        if (fassungLaedt != null) return
        val kontext = getApplication<Application>()
        if (!Aktualisierung.darfInstallieren(kontext)) {
            fassungHinweis = "Android fragt jetzt, ob MIMIK Apps installieren darf."
            Aktualisierung.erlaubnisHolen(kontext)
            return
        }
        viewModelScope.launch {
            fassungLaedt = 0
            fassungHinweis = null
            try {
                val datei = withContext(Dispatchers.IO) {
                    Aktualisierung.herunterladen(kontext, f) { prozent ->
                        // Nur bei echter Aenderung zurueck auf den Hauptfaden:
                        // Compose-Zustand gehoert dorthin, und 100 Spruenge sind
                        // genug fuer einen Balken.
                        if (prozent != fassungLaedt) {
                            viewModelScope.launch { fassungLaedt = prozent }
                        }
                    }
                }
                Aktualisierung.installieren(kontext, datei)
            } catch (e: Exception) {
                fassungHinweis = "Das Update ließ sich nicht laden."
            } finally {
                fassungLaedt = null
            }
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

    fun aktualisieren() = imHintergrund { nachladen() }

    /**
     * Fragt von selbst nach, solange der Mensch wartet.
     *
     * Bewusst OHNE imHintergrund: Das setzte laden=true, machte bei jedem Takt
     * die Knöpfe grau und löschte eine angezeigte Fehlermeldung. Ein Abgleich
     * im Hintergrund darf im Vordergrund nicht sichtbar sein.
     *
     * Der Takt hängt am Bildschirm (siehe takt) und hält im Hintergrund ganz
     * an. Vorher lief er weiter, solange die Activity lebte – alle drei
     * Sekunden, auch wenn niemand hinsah. Der Zeitstempel hier ist zugleich der
     * Herzschlag, an dem der Melder erkennt, dass die App sichtbar ist.
     */
    fun beobachten() {
        viewModelScope.launch {
            var fehlschlaege = 0
            while (true) {
                val t = takt
                if (t == 0L) {
                    delay(2_000)
                    continue
                }
                speicher.gesehenStempel = System.currentTimeMillis()
                delay(t * (1L shl minOf(fehlschlaege, 3)))
                if (laden) continue
                runCatching {
                    withContext(Dispatchers.IO) {
                        val l = netz.lobby()
                        val z = offenePartie?.let { netz.zustand(it) }
                        l to z
                    }
                }.onSuccess { (l, z) ->
                    fehlschlaege = 0
                    // Und die Meldung wieder weg. Sie wird hier gesetzt, also
                    // muss sie hier auch verschwinden: Ein kurzer Aussetzer im
                    // WLAN hinterliess sonst eine Fehlerzeile, die stehen
                    // blieb, obwohl laengst wieder alles lief.
                    fehler = null
                    lobbyUebernehmen(l)
                    if (z != null) zustandUebernehmen(z)
                }.onFailure { e ->
                    fehlschlaege++
                    // Der Takt schweigt sonst. Das ist richtig, solange nur
                    // die Verbindung hakt - aber wer auf dem Ladebildschirm
                    // festsitzt, soll erfahren, warum. Ab dem dritten
                    // Fehlschlag steht es da.
                    if (fehlschlaege >= 5 && fehler == null) {
                        // Ein Java-Stapelsatz ("Unable to resolve host …") sagt
                        // niemandem, was zu tun ist. Vom Server kommt eine
                        // Begruendung, vom Netz nur ein Aussetzer - und der
                        // heisst hier auch so.
                        fehler = (e as? NetzFehler)?.text
                            ?: "Keine Verbindung. Ich versuche es weiter."
                    }
                    // Die offene Partie gibt es nicht mehr (verlassen, vom
                    // Gegenueber aufgeloest): zurueck in die Uebersicht, statt
                    // ewig eine Partie zu laden, die weg ist.
                    if ((e as? NetzFehler)?.code == 404) zurueckZurLobby()
                }
            }
        }
    }

    fun sichtbarkeit(an: Boolean) {
        sichtbar = an
        speicher.gesehenStempel = if (an) System.currentTimeMillis() else 0L
    }

    // ------------------------------------------------------------ Lobby ---

    fun partieOeffnen(id: String) {
        fehler = null
        offenePartie = id
        speicher.offenePartie = id
        zustand = null
        imHintergrund {
            val z = netz.zustand(id)
            withContext(Dispatchers.Main) { zustandUebernehmen(z) }
        }
    }

    fun zurueckZurLobby() {
        fehler = null
        offenePartie = null
        speicher.offenePartie = ""
        zustand = null
        aktualisieren()
    }

    fun suchen(q: String) {
        if (q.trim().length < 2) {
            treffer = emptyList()
            return
        }
        viewModelScope.launch {
            runCatching { withContext(Dispatchers.IO) { netz.spielerSuchen(q.trim()) } }
                .onSuccess { treffer = it.treffer }
        }
    }

    fun einladen(pid: String) = imHintergrund {
        netz.einladen(pid)
        val l = netz.lobby()
        withContext(Dispatchers.Main) { lobbyUebernehmen(l) }
    }

    fun einladungAnnehmen(id: String) = imHintergrund {
        netz.einladungAnnehmen(id)
        val l = netz.lobby()
        withContext(Dispatchers.Main) { lobbyUebernehmen(l) }
    }

    fun einladungAblehnen(id: String) = imHintergrund {
        netz.einladungAblehnen(id)
        val l = netz.lobby()
        withContext(Dispatchers.Main) { lobbyUebernehmen(l) }
    }

    fun einladungZurueckziehen(id: String) = imHintergrund {
        netz.einladungZurueckziehen(id)
        val l = netz.lobby()
        withContext(Dispatchers.Main) { lobbyUebernehmen(l) }
    }

    fun partyAnlegen() = imHintergrund {
        val neu = netz.partyAnlegen()
        val l = netz.lobby()
        val z = netz.zustand(neu.partyId)
        withContext(Dispatchers.Main) {
            lobbyUebernehmen(l)
            // Direkt in den Warteraum: Wer gründet, will den Code sehen.
            offenePartie = neu.partyId
            speicher.offenePartie = neu.partyId
            zustandUebernehmen(z)
        }
    }

    fun partyBeitreten(code: String) = imHintergrund {
        val neu = netz.partyBeitreten(code.trim().uppercase())
        val l = netz.lobby()
        val z = netz.zustand(neu.partyId)
        withContext(Dispatchers.Main) {
            lobbyUebernehmen(l)
            offenePartie = neu.partyId
            speicher.offenePartie = neu.partyId
            zustandUebernehmen(z)
        }
    }

    /** Erster Griff: Stand vom Server holen, Auswahl daraus übernehmen. */
    fun tagsLaden() = imHintergrund {
        val t = netz.tags(0)
        withContext(Dispatchers.Main) {
            gespeicherteTags = t.gewaehlt
            vorschlaege = t.vorschlaege
            gewaehlteTags = t.gewaehlt.toSet()
            mehrTags = t.mehr
            tagsGeladen = true
        }
    }

    /**
     * "Weitere": hängt an, statt zu ersetzen – und fasst die Auswahl nicht an.
     * Das Antippen von zehn Begriffen soll nicht dadurch verfallen, dass man
     * noch einmal in den Vorrat greift.
     */
    fun tagsNachladen() = imHintergrund {
        val t = netz.tags(vorschlaege.size)
        withContext(Dispatchers.Main) {
            vorschlaege = (vorschlaege + t.vorschlaege).distinct()
            mehrTags = t.mehr
        }
    }

    fun tagsSpeichern() = imHintergrund {
        netz.tagsSetzen(gewaehlteTags.toList())
        val l = netz.lobby()
        withContext(Dispatchers.Main) {
            gespeicherteTags = gewaehlteTags.toList().sorted()
            lobbyUebernehmen(l)
        }
    }

    /** Löst die Party auf – für beide. Konto, Tags und Dossier bleiben. */
    /** Löst die Party auf – für beide. Konto, Tags und Dossier bleiben. */
    fun partyVerlassen() = imHintergrund {
        val p = offenePartie ?: return@imHintergrund
        netz.partyVerlassen(p)
        val l = netz.lobby()
        withContext(Dispatchers.Main) {
            einstellungenOffen = false
            offenePartie = null
            speicher.offenePartie = ""
            zustand = null
            lobbyUebernehmen(l)
        }
    }

    fun matchAbbrechen() = imHintergrund {
        val p = offenePartie ?: return@imHintergrund
        netz.matchAbbrechen(p)
        withContext(Dispatchers.Main) { einstellungenOffen = false }
        nachladen()
    }

    fun matchStarten() = imHintergrund {
        val p = offenePartie ?: return@imHintergrund
        netz.matchAnlegen(p)
        nachladen()
    }

    fun antwortSenden(runde: String, original: String) = imHintergrund {
        netz.antworten(runde, original)
        nachladen()
    }

    /**
     * Die freiwillige Stimme zur Frage – und der einzige Aufruf der App, der
     * bewusst NICHT durch imHintergrund läuft.
     *
     * Grund: imHintergrund setzt `laden` (der ganze Bildschirm wird blass und
     * Knöpfe gehen aus) und zeigt bei einem Fehlschlag ein rotes Band. Beides
     * wäre hier falsch. Wer aus Freundlichkeit einen Daumen setzt, soll dafür
     * nicht den Bildschirm blockieren und schon gar keine Fehlermeldung
     * bekommen. Die Wahl steht sofort lokal, der Server erfährt sie danach, und
     * wenn das schiefgeht, bleibt der Daumen stehen und niemand erwähnt es.
     *
     * Der Preis ist ausdrücklich: Eine Stimme kann verloren gehen, ohne dass es
     * jemand merkt. Bei freiwilligem Feedback auf eine Frage, die derselbe
     * Mensch nie wieder sieht, ist das der richtige Tausch.
     */
    fun urteilSenden(runde: String, urteil: Int) {
        urteile = urteile + (runde to urteil)
        viewModelScope.launch {
            try {
                withContext(Dispatchers.IO) { netz.urteilen(runde, urteil) }
            } catch (e: Exception) {
                // Absichtlich still. Siehe oben.
            }
        }
    }

    fun tippSenden(runde: String, pos: Int) = imHintergrund {
        val aus = netz.raten(runde, pos)
        withContext(Dispatchers.Main) { letzterTreffer = aus.richtig }
        nachladen()
    }

    /**
     * Den Klonblick wegklicken und ins Raten gehen.
     *
     * Wie bei der Auflösung wird nichts "zugemacht", sondern der Merker
     * weitergeschoben - was zu sehen ist, ergibt sich danach von selbst.
     */
    fun klonWeiter() {
        val z = zustand ?: return
        val r = aktuelleRunde ?: return
        klone = klone + (z.partyId to Gesehen(z.match?.id.orEmpty(), r.nummer))
        speicher.klone = klone
    }

    /**
     * Auflösung wegklicken. Es wird nichts "zugemacht", sondern der Merker
     * dieser Partie weitergeschoben - was zu sehen ist, ergibt sich danach von
     * selbst.
     */
    fun weiter() {
        val z = zustand ?: return
        val r = z.runden.firstOrNull { it.id == zeigeAufloesung } ?: return
        gesehen = gesehen + (z.partyId to Gesehen(z.match?.id.orEmpty(), r.nummer))
        speicher.gesehen = gesehen
        aktualisieren()
    }

    // ------------------------------------------------------------ Konto ---

    fun umbenennen(neu: String) = imHintergrund {
        netz.umbenennen(neu.trim())
        nachladen()
    }

    /**
     * Was MIMIK über einen notiert hat – auf Wunsch, nicht im Hintergrund
     * geladen: Es ändert sich nur nach einer Runde, und wer es nie öffnet,
     * soll dafür auch keine Abfrage bezahlen.
     */
    var dossier by mutableStateOf<DossierAus?>(null); private set

    fun dossierLaden() = imHintergrund {
        val d = netz.dossier()
        withContext(Dispatchers.Main) { dossier = d }
    }

    /** MIMIK vergisst, was sie gelernt hat. Die Tags bleiben – sie sind eine
     *  Einstellung, kein Gelerntes. */
    fun dossierLoeschen() = imHintergrund {
        netz.dossierLoeschen()
        withContext(Dispatchers.Main) { dossier = null }
        nachladen()
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
            lobby = null
            offenePartie = null
            gespeicherteTags = emptyList()
            vorschlaege = emptyList()
            gewaehlteTags = emptySet()
            tagsGeladen = false
            gesehen = emptyMap()
            klone = emptyMap()
            einstellungenOffen = false
        }
    }
}
