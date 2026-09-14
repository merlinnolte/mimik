package app.mimik

import android.Manifest
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.Text
import androidx.compose.ui.text.style.LineBreak
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

/**
 * Ein Bildschirm ist eine mittig stehende Spalte. Passt der Inhalt, steht er in
 * der Höhe zentriert; passt er nicht, scrollt er. Kein Element klebt oben.
 */
@Composable
fun Huelle(modell: AppModel, inhalt: @Composable ColumnScope.() -> Unit) {
    val p = LokalePalette.current
    // fillMaxSize VOR verticalScroll: Die Spalte ist damit mindestens
    // bildschirmhoch, sodass Arrangement.Center wirklich zentriert – und wächst
    // darüber hinaus, wenn der Inhalt länger ist, statt oben und unten zu
    // klemmen.
    Column(
        Modifier
            .fillMaxSize()
            .background(p.bg)
            .safeDrawingPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 18.dp, vertical = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        Column(
            Modifier.fillMaxWidth().widthIn(max = 460.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            modell.fehler?.let { Panel(titel = "Fehler") { Zeile(it, p.error, 12) } }
            inhalt()
        }
    }
}

@Composable
internal fun Frage(text: String) {
    val p = LokalePalette.current
    Text(
        text,
        color = p.accent,
        fontSize = 16.sp,
        lineHeight = 25.sp,
        textAlign = TextAlign.Center,
        modifier = Modifier.fillMaxWidth().padding(horizontal = 4.dp),
    )
}

@Composable
internal fun Aktionen(inhalt: @Composable () -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) { inhalt() }
}

/**
 * Statt eines Knopfes, der nichts bewirkt: ein Satz, mittig unter dem Inhalt.
 *
 * Auf Wartebildschirmen stand früher "Nachsehen". Der Knopf tat fast nie etwas –
 * er wurde gedrückt, WEIL nichts passierte, und es passierte nichts, weil die
 * andere Seite noch nicht gezogen hatte. Die App fragt jetzt selbst nach; hier
 * steht nur noch, worauf gewartet wird.
 *
 * Die drei laufenden Punkte daneben sind wieder raus. Sie standen rechts, also
 * saß der Satz linksbündig neben einer leeren Spalte, und getaktete Animation
 * auf einem Bildschirm, auf dem man minutenlang wartet, ist Unruhe ohne
 * Aussage: Dass gewartet wird, sagt schon der Satz.
 */
@Composable
internal fun Wartezeile(text: String) {
    val p = LokalePalette.current
    Text(
        text, color = p.fgDim, fontSize = 11.sp, lineHeight = 17.sp,
        textAlign = TextAlign.Center,
        modifier = Modifier.fillMaxWidth(),
    )
}

/**
 * Die freiwillige Stimme zur Frage: eine Zeile, kein Bildschirm.
 *
 * Sie steht dort, wo sowieso gewartet wird – nach dem Abschicken hat der
 * Spieler mindestens fünfzehn Sekunden nichts zu tun. Nichts muss weggeklickt
 * werden, nichts wartet auf eine Antwort, und wer sie überliest, verliert
 * nichts: Die Runde geht von selbst weiter.
 *
 * Deshalb auch so klein und in der gedämpften Farbe. Ein Bedienelement, das
 * etwas anbietet, aber nichts verlangt, darf nicht aussehen wie eines, das
 * etwas verlangt. Erst die getroffene Wahl bekommt die Akzentfarbe – als
 * Quittung, nicht als Aufforderung.
 *
 * Dieselbe Seite noch einmal getippt nimmt die Stimme zurück (urteilNach). Ein
 * eigener Knopf dafür wäre eine dritte Sache in einer Zeile, die eine bleiben
 * soll.
 */
@Composable
internal fun Fragenurteil(modell: AppModel, r: RundeAus) {
    if (!urteilOffen(r)) return
    val p = LokalePalette.current
    val stand = urteilstand(r, modell.urteile)

    @Composable
    fun Seite(text: String, wert: Int) {
        val an = stand == wert
        Text(
            text,
            color = if (an) p.accent else p.fgDim,
            fontSize = 12.sp,
            modifier = Modifier
                .clickable { modell.urteilSenden(r.id, urteilNach(stand, wert)) }
                .padding(horizontal = 10.dp, vertical = 8.dp),
        )
    }

    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            if (stand == 0) "Gute Frage?" else "Notiert.",
            color = p.fgDim, fontSize = 11.sp,
        )
        Seite("ja", 1)
        Text("·", color = p.border, fontSize = 11.sp)
        Seite("nicht so", -1)
    }
}

/**
 * Das Zahnrad liegt über dem Bildschirm, nicht in ihm: Es gehört auf jeden
 * Spielbildschirm, und keiner davon soll deshalb eine Kopfleiste bekommen, die
 * die mittige Anordnung wieder kaputt macht.
 */
@Composable
fun ZahnradEcke(modell: AppModel) {
    if (!modell.zahnradSichtbar) return
    val p = LokalePalette.current
    Box(Modifier.fillMaxSize().safeDrawingPadding(), contentAlignment = Alignment.TopEnd) {
        Box(
            Modifier
                .padding(10.dp)
                .border(1.dp, p.border)
                .background(p.bgAlt)
                .clickable { modell.einstellungen(true) }
                .padding(7.dp),
        ) {
            Icon(
                painterResource(R.drawable.zahnrad), "Einstellungen",
                tint = p.fgDim, modifier = Modifier.size(17.dp),
            )
        }
    }
}

// ------------------------------------------------------------------- Start ---

@Composable
fun StartBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    var name by remember { mutableStateOf("") }
    var url by remember { mutableStateOf(modell.server) }
    var einladung by remember { mutableStateOf("") }
    var schluessel by remember { mutableStateOf("") }
    var umzug by remember { mutableStateOf(false) }
    // Bringt der Bau keine Adresse mit, ist das Feld der erste Schritt und
    // keine Klappe für Entwickler. Im Debugbau bleibt es versteckt: Dort steht
    // eine Vorgabe, und wer einfach spielen will, geht sie nichts an.
    var serverZeigen by remember { mutableStateOf(BuildConfig.VORGABE_SERVER.isBlank()) }
    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Wir kennen uns noch nicht.", schrift = 19)
        Text(
            "MIMIK", color = p.fg, fontSize = 22.sp, fontWeight = FontWeight.Bold,
            letterSpacing = 6.sp, maxLines = 1,
        )
        Zeile("Vier Antworten. Eine ist echt.", p.fgDim, 12)

        if (serverZeigen) {
            Panel(titel = "Server") {
                Zeile("Auf welchem Server spielst du?", p.fgDim, 11)
                Feld(url, { url = it }, hinweis = "mimik.beispiel.de")
                Spacer(Modifier.height(8.dp))
                Zeile(
                    "MIMIK gehört zu keinem Dienst. Der Server ist der, den ihr " +
                        "betreibt oder auf den ihr eingeladen wurdet.",
                    p.fgDim, 11,
                )
            }
        }

        if (!umzug) {
            Panel(titel = "Anmelden") {
                Zeile("Dein Spitzname", p.fgDim, 11)
                Feld(name, { name = it }, hinweis = "Wie heißt du im Spiel?")
                Spacer(Modifier.height(10.dp))
                Zeile("Einladungs-Code", p.fgDim, 11)
                Feld(einladung, { einladung = it }, hinweis = "leer lassen, wenn keiner")
            }
            Aktionen {
                Knopf(
                    "Gerät anmelden", betont = true,
                    aktiv = name.isNotBlank() && url.isNotBlank() && !modell.laden,
                ) {
                    if (modell.serverSetzen(url)) modell.anmelden(name, einladung)
                }
            }
        } else {
            // Der Weg zurück in ein bestehendes Konto. Siehe Umzug.kt: Nach
            // einer Neuinstallation ist der lokale Speicher leer, auf dem
            // Server steht aber alles noch – am Schlüssel.
            Panel(titel = "Gerät umziehen") {
                Zeile("Dein Schlüssel", p.fgDim, 11)
                Feld(schluessel, { schluessel = it }, hinweis = "aus den Einstellungen des alten Geräts", zeilen = 3)
                Spacer(Modifier.height(8.dp))
                Zeile(
                    "Damit bist du wieder derselbe Spieler – mit Party, Punktestand " +
                        "und Dossier. Das alte Gerät bleibt ebenfalls angemeldet.",
                    p.fgDim, 11,
                )
            }
            Aktionen {
                Knopf(
                    "Konto übernehmen", betont = true,
                    aktiv = schluessel.isNotBlank() && url.isNotBlank() && !modell.laden,
                ) {
                    if (modell.serverSetzen(url)) modell.mitSchluessel(schluessel)
                }
            }
        }

        Klickbar(beiKlick = { umzug = !umzug }) {
            Zeile(
                if (umzug) "Ich bin neu hier" else "Ich hatte MIMIK schon",
                p.fgDim, 11,
            )
        }
        if (!serverZeigen) {
            Klickbar(beiKlick = { serverZeigen = true }) {
                Zeile("Anderer Server", p.fgDim, 11)
            }
        }
    }
}

// ------------------------------------------------------------------- Laden ---

@Composable
fun LadeBildschirm(modell: AppModel) {
    Huelle(modell) {
        MimikKopf(Miene.Denkt, "Einen Moment, ich schlage nach.", schrift = 19)
        if (modell.fehler != null) {
            Aktionen { Knopf("Erneut versuchen", betont = true) { modell.aktualisieren() } }
        }
    }
}

// ------------------------------------------------------------------- Intro ---

/**
 * MIMIK stellt sich vor – in ihren Worten, nicht in einer Anleitung. Die Regeln
 * stehen darin, aber aus der Sicht derjenigen, gegen die man sie anwendet.
 *
 * Zeilenumbrüche hängen am vorangehenden Wort, damit das Zerlegen an den
 * Leerzeichen sie mitnimmt: Die Ausgabe läuft Wort für Wort.
 */
private const val INTRO =
    "Hallo.\n\n" +
        "Ich bin MIMIK.\n\n" +
        "Ab jetzt lese ich mit. Jede Frage, die ihr beantwortet, " +
        "beantworte ich dreimal mit – in eurer Sprache, mit euren Macken, über euch.\n\n" +
        "Vier Karten. Drei sind von mir. Ihr müsst sagen, welche wirklich " +
        "von der anderen Seite kam.\n\n" +
        "Trefft ihr, bekommt ihr einen Punkt. Trefft ihr nicht, bekomme ich ihn. " +
        "Bei zehn ist Schluss.\n\n" +
        "Ich sage euch das so offen, weil es nichts ändert: Mit jeder Runde lerne " +
        "ich euch besser. Und irgendwann klingt ihr wie ich.\n\n" +
        "Fangen wir an."

private val INTRO_WORTE = INTRO.split(" ")

/** Satzenden atmen, Absätze atmen länger. Sonst prasselt der Text nur. */
private fun pause(wort: String): Long = when {
    wort.contains("\n\n") -> 620
    wort.endsWith(".") || wort.endsWith(":") -> 300
    wort.endsWith(",") || wort.endsWith("–") -> 170
    else -> 95
}

@Composable
fun IntroBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val kontext = LocalContext.current
    var bis by remember { mutableIntStateOf(0) }
    val fertig = bis >= INTRO_WORTE.size

    val frage = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        modell.introBeenden()
    }

    // Den Index EINMAL am Anfang lesen und danach nur noch ihn benutzen.
    //
    // "bis" ist Snapshot-State: Ein Zugriff darauf im Coroutinenrumpf liest den
    // Wert von jetzt, nicht den der Komposition. "fertig" dagegen ist ein
    // gewöhnliches val und steht auf dem Stand der Komposition. Zwischen dem
    // Start der Coroutine und ihrer ersten Zeile passt ein Tipp – dann war
    // fertig noch false, bis aber schon INTRO_WORTE.size, und der Zugriff lief
    // über das Ende der Liste. Auf dem Gerät gefunden: IndexOutOfBounds,
    // Absturz beim Überspringen, zeitabhängig und deshalb nicht immer.
    LaunchedEffect(bis) {
        val i = bis
        if (i >= INTRO_WORTE.size) return@LaunchedEffect
        delay(pause(INTRO_WORTE[i]))
        // Nur weiterrücken, wenn in der Zwischenzeit niemand übersprungen hat.
        if (bis == i) bis = i + 1
    }

    Box(
        Modifier.fillMaxSize().background(p.bg)
            // Antippen überspringt. Wer das Intro kennt, soll nicht warten müssen.
            .clickable(enabled = !fertig) { bis = INTRO_WORTE.size },
    ) {
        Huelle(modell) {
            MimikGesicht(
                if (fertig) Miene.Bereit else Miene.Denkt,
                schrift = 17, rahmen = false,
            )
            Text(
                INTRO_WORTE.take(bis).joinToString(" ") + if (fertig) "" else " ▋",
                color = p.fg, fontSize = 14.sp, lineHeight = 23.sp,
                // Hier ausdrücklich der einfache Umbruch: Der Absatz wächst
                // Wort für Wort, und ein ausgeglichener Umbruch verteilte bei
                // jedem Wort neu – der schon gelesene Teil spränge im Takt der
                // Schreibmaschine.
                style = LocalTextStyle.current.copy(lineBreak = LineBreak.Simple),
                modifier = Modifier.fillMaxWidth(),
            )
            if (fertig) {
                val brauchtErlaubnis = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                    !Melder.darfMelden(kontext)
                if (brauchtErlaubnis) {
                    Panel(titel = "Noch eins") {
                        Zeile(
                            "Ich stupse dich an, sobald die andere Seite gezogen hat. " +
                                "Ohne Benachrichtigungen musst du selbst nachsehen – " +
                                "spielen kannst du trotzdem.",
                            p.fgDim, 12,
                        )
                    }
                    Aktionen {
                        Knopf("Später") { modell.introBeenden() }
                        Knopf("Einverstanden", betont = true) {
                            frage.launch(Manifest.permission.POST_NOTIFICATIONS)
                        }
                    }
                } else {
                    Aktionen { Knopf("Los geht's", betont = true) { modell.introBeenden() } }
                }
            } else {
                Zeile("tippen zum Überspringen", p.fgDim, 11)
            }
        }
    }
}

// -------------------------------------------------------------------- Tags ---

@Composable
fun TagsBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    LaunchedEffect(Unit) { if (!modell.tagsGeladen) modell.tagsLaden() }
    val gewaehlt = modell.gewaehlteTags
    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Wähle Themen aus, die dich interessieren.")
        Text(
            "${gewaehlt.size} / 10", color = if (gewaehlt.size >= 10) p.accent2 else p.fgDim,
            fontSize = 15.sp,
        )
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            modell.sichtbareTags.chunked(2).forEach { paar ->
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    paar.forEach { t ->
                        val an = t in gewaehlt
                        Box(
                            Modifier.weight(1f)
                                .border(1.dp, if (an) p.accent2 else p.border)
                                .background(if (an) p.bgAlt else p.bg)
                                .clickable {
                                    modell.gewaehlteTags = if (an) gewaehlt - t else gewaehlt + t
                                }
                                .padding(horizontal = 10.dp, vertical = 9.dp),
                        ) {
                            Text(
                                (if (an) "✓ " else "  ") + t,
                                color = if (an) p.accent2 else p.fg, fontSize = 12.sp,
                            )
                        }
                    }
                    if (paar.size == 1) Spacer(Modifier.weight(1f))
                }
            }
        }
        Aktionen {
            Knopf("Weitere", aktiv = modell.mehrTags && !modell.laden) { modell.tagsNachladen() }
            Knopf("Speichern", betont = true, aktiv = gewaehlt.size >= 10 && !modell.laden) {
                modell.tagsSpeichern()
            }
        }
        if (!modell.mehrTags && modell.tagsGeladen) {
            Zeile("Das war der ganze Vorrat.", p.fgDim, 11)
        }
    }
}

// ------------------------------------------------- Kein Match / Matchende ---

@Composable
fun BasisBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val m = modell.zustand?.match
    val vorbei = m != null && m.ergebnis != "OFFEN"
    var verlassenFragen by remember { mutableStateOf(false) }
    val gewonnen = m?.ergebnis == "MENSCH"
    val abgebrochen = m?.ergebnis == "ABGEBROCHEN"
    Huelle(modell) {
        if (vorbei) {
            MimikKopf(
                when {
                    abgebrochen -> Miene.Bereit
                    gewonnen -> Miene.Getroffen
                    else -> Miene.Triumph
                },
                when {
                    abgebrochen -> "Abgebrochen. Fangen wir neu an, wenn ihr wollt."
                    gewonnen -> "Ihr habt mich durchschaut."
                    else -> "Ich kenne euch besser, als ihr denkt."
                },
                haltend = !abgebrochen, schrift = 19,
            )
            Text(
                when {
                    abgebrochen -> "SPIEL BEENDET"
                    gewonnen -> "IHR GEWINNT"
                    else -> "MIMIK GEWINNT"
                },
                color = when {
                    abgebrochen -> p.fgDim
                    gewonnen -> p.accent2
                    else -> p.accent
                },
                fontSize = 19.sp, letterSpacing = 2.sp,
            )
            Punktebalken(m!!.stand.mensch, m.stand.mimik, m.ziel)
            // Nach einem Spiel stehen zwei Wege offen, und beide gehören hierhin:
            // noch einmal gegeneinander, oder auseinander. Wer aufhören will,
            // musste bisher ins Zahnrad – an der Stelle, an der die Frage
            // tatsächlich aufkommt, stand sie nicht.
            Aktionen {
                Knopf("Revanche", betont = true, aktiv = !modell.laden) { modell.matchStarten() }
                Knopf("Party verlassen", aktiv = !modell.laden) { verlassenFragen = true }
            }
            if (verlassenFragen) {
                Panel(titel = "Sicher?", betont = true) {
                    Zeile(
                        "Die Party wird für euch beide aufgelöst. Dein Dossier und deine " +
                            "Tags bleiben – danach kannst du eine neue Party gründen oder " +
                            "einer beitreten.",
                        p.warn, 12,
                    )
                    Spacer(Modifier.height(10.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Knopf("Doch nicht") { verlassenFragen = false }
                        Knopf("Verlassen", aktiv = !modell.laden) {
                            verlassenFragen = false
                            modell.partyVerlassen()
                        }
                    }
                }
            }
        } else {
            MimikKopf(Miene.Bereit, "Bereit, wenn ihr es seid.")
            Aktionen { Knopf("Match starten", betont = true, aktiv = !modell.laden) { modell.matchStarten() } }
        }
    }
}

// --------------------------------------------------------------- Schreiben ---

@Composable
fun SchreibenBildschirm(modell: AppModel) {
    val r = modell.aktuelleRunde ?: return
    var text by remember(r.id) { mutableStateOf("") }
    val laenge = text.trim().length

    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Runde ${r.nummer} · beantwortet das mal ehrlich.")
        Frage(r.frage)
        Feld(text, { text = it }, hinweis = "Ein bis drei Sätze reichen", zeilen = 4)
        Aktionen {
            Knopf("Absenden", betont = true, aktiv = laenge >= 4 && !modell.laden) {
                modell.antwortSenden(r.id, text.trim())
            }
        }
    }
}

// ------------------------------------------------------------------ Warten ---

@Composable
fun WartenBildschirm(modell: AppModel) {
    val r = modell.aktuelleRunde ?: return
    val arbeitet = r.zustand == "MIMIK_ARBEITET"
    // Die Karten sind da, der Balken läuft aber noch voll. Der Bildschirm
    // bleibt so lange stehen; das Gesicht darf schon aufhören zu denken.
    val fertig = modell.balkenLaeuftVoll == r.id
    // Wer wartet, wartet auf einen Menschen mit Namen. „Sie" zwingt den Leser,
    // aus dem Zusammenhang zu erschließen, wer gemeint ist - der Spitzname sagt
    // es sofort. Klein geschrieben der Notbehelf, weil er mitten im Satz steht.
    val sie = modell.partnerName.ifBlank { "die andere Seite" }
    Huelle(modell) {
        MimikKopf(
            if (arbeitet && !fertig) Miene.Denkt else Miene.Bereit,
            if (fertig) "Fertig. Vier Karten, eine ist echt."
            else if (arbeitet) "Ich baue gerade drei Fälschungen. Das dauert einen Moment."
            else "Deine Antwort steht. Jetzt ist $sie dran.",
            schrift = 19,
        )
        Frage(r.frage)
        Fragenurteil(modell, r)
        // Solange MIMIK arbeitet, steht hier die regelbasierte Notfassung. Die
        // endgültige schreibt sie selbst, zusammen mit den Fälschungen – der
        // Titel sagt das, statt den Wechsel klammheimlich passieren zu lassen.
        Panel(titel = if (arbeitet) "Deine Antwort · wird noch geglättet" else "Deine Antwort") {
            Text(
                r.meineAntwort, color = LokalePalette.current.fg,
                fontSize = 14.sp, lineHeight = 22.sp,
            )
        }
        if (r.fehler.isNotBlank()) Zeile(r.fehler, LokalePalette.current.warn, 11)
        if (arbeitet || fertig) {
            Fortschritt(r.wartetSeit, fertig)
            // Die Zeile bleibt stehen, auch wenn sie den Text wechselt: Fiele
            // sie für die knappe Sekunde des Volllaufens weg, ruckte der ganze
            // mittig gesetzte Bildschirm nach unten.
            Wartezeile(
                if (fertig) "Gleich geht es weiter."
                else "Es geht von selbst weiter, du musst nicht warten.",
            )
        } else {
            Wartezeile("Es geht von selbst weiter, sobald $sie geantwortet hat.")
        }
    }
}

// ------------------------------------------------------------------- Raten ---

@Composable
fun RatenBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val r = modell.aktuelleRunde ?: return
    var gewaehlt by remember(r.id) { mutableIntStateOf(0) }

    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Eine davon ist echt. Drei sind von mir.")
        Frage(r.frage)
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            r.karten.forEach { k ->
                val an = k.pos == gewaehlt
                Box(
                    Modifier.fillMaxWidth()
                        .border(1.dp, if (an) p.accent2 else p.border)
                        .background(if (an) p.bgAlt else p.bg)
                        .clickable { gewaehlt = k.pos }
                        .padding(12.dp),
                ) {
                    Row {
                        Text(
                            "[${k.pos}] ", color = if (an) p.accent2 else p.fgDim,
                            fontSize = 14.sp, fontWeight = FontWeight.Bold,
                        )
                        Text(k.text, color = p.fg, fontSize = 14.sp, lineHeight = 22.sp)
                    }
                }
            }
        }
        Aktionen {
            Knopf("Tipp abgeben", betont = true, aktiv = gewaehlt > 0 && !modell.laden) {
                modell.tippSenden(r.id, gewaehlt)
            }
        }
    }
}

// --------------------------------------------------------------- Getippt ---

@Composable
fun GetipptBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val r = modell.aktuelleRunde ?: return
    val meine = r.karten.firstOrNull { it.pos == r.meinTipp }
    val sie = modell.partnerName.ifBlank { "die andere Seite" }
    Huelle(modell) {
        MimikKopf(Miene.Denkt, "Tipp steht. Die Auflösung kommt, sobald auch $sie getippt hat.", schrift = 19)
        Frage(r.frage)
        Panel(titel = "Deine Wahl", betont = true) {
            Row {
                Text("[${r.meinTipp}] ", color = p.accent2, fontSize = 14.sp, fontWeight = FontWeight.Bold)
                Text(meine?.text.orEmpty(), color = p.fg, fontSize = 14.sp, lineHeight = 22.sp)
            }
        }
        Wartezeile("Es geht von selbst weiter, sobald $sie getippt hat.")
    }
}

// -------------------------------------------------------------- Auflösung ---

/** Eine Karte in der Auflösung: Text, darunter das Etikett. */
@Composable
private fun Aufloesungskarte(text: String, etikett: String, betont: Boolean) {
    val p = LokalePalette.current
    val farbe = if (betont) p.accent2 else p.accent
    Box(
        Modifier.fillMaxWidth()
            .border(1.dp, if (betont) p.accent2 else p.border)
            .background(if (betont) p.bgAlt else p.bg)
            .padding(12.dp),
    ) {
        Column {
            Text(
                text, color = if (betont) p.fg else p.fgDim,
                fontSize = 14.sp, lineHeight = 22.sp,
            )
            Spacer(Modifier.height(5.dp))
            Text(etikett, color = farbe, fontSize = 10.sp, letterSpacing = 1.1.sp)
        }
    }
}

@Composable
fun AufloesungBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val r = modell.runde(modell.zeigeAufloesung) ?: return
    val a = r.aufloesung
    val richtig = a?.meinTippRichtig == true
    val partnerRichtig = a?.partnerRichtig == true
    val m = modell.zustand?.match
    val echt = r.karten.firstOrNull { it.istEcht == true }
    val meine = r.karten.firstOrNull { it.pos == r.meinTipp }
    val sie = modell.partnerName.ifBlank { "Die andere Seite" }

    Huelle(modell) {
        // Vier Ausgänge, vier Mienen. Der Punktestand allein sagt nicht, was
        // passiert ist: Zwei Treffer und zwei Fehlgriffe stehen beide 2:0
        // beziehungsweise 0:2 - aber MIMIK hat dabei einmal verloren und einmal
        // gewonnen, und genau das zeigt das Gesicht.
        MimikKopf(
            when {
                richtig && partnerRichtig -> Miene.Getroffen
                !richtig && !partnerRichtig -> Miene.Triumph
                else -> Miene.Neutral
            },
            when {
                richtig && partnerRichtig -> "$sie und du – beide. Das tut weh."
                richtig && !partnerRichtig -> "Du hast mich erwischt. $sie nicht."
                !richtig && partnerRichtig -> "$sie hat mich erwischt. Dich habe ich."
                else -> "Euch beide. Doppelt."
            },
            haltend = true, schrift = 19,
        )
        if (m != null) {
            // Eine Runde vergibt immer genau zwei Punkte, einen je Tipp. Also
            // steht der Stand von vorher fest, ohne dass der Server ihn
            // mitschicken muss: was diese Runde gebracht hat, wieder abgezogen.
            val fuerMensch = (if (richtig) 1 else 0) + (if (partnerRichtig) 1 else 0)
            Punktebalken(
                m.stand.mensch, m.stand.mimik, m.ziel,
                vonMensch = (m.stand.mensch - fuerMensch).coerceAtLeast(0),
                vonMimik = (m.stand.mimik - (2 - fuerMensch)).coerceAtLeast(0),
            )
        }
        Frage(r.frage)

        // Erster Block: der Satz über den Partner – was du geraten hast.
        Zeile("Über ${sie}", p.fgDim, 11)
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            if (!richtig && meine != null) {
                Aufloesungskarte(meine.text, "DEIN TIPP · VON MIR", betont = false)
            }
            if (echt != null) {
                Aufloesungskarte(
                    echt.text,
                    if (richtig) "ECHT · DEIN TIPP" else "ECHT · VON ${sie.uppercase()}",
                    betont = true,
                )
            }
        }

        // Zweiter Block: der Satz über dich – wofür die andere Seite dich
        // gehalten hat. Das ist der eigentlich spannende Teil und stand bisher
        // nur als Nebensatz da: Nicht wie gut du rätst, sondern wie gut sie
        // dich kennt.
        if (a != null && a.partnerTippText.isNotBlank()) {
            Spacer(Modifier.height(4.dp))
            Zeile("Über dich", p.fgDim, 11)
            Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                if (!partnerRichtig) {
                    Aufloesungskarte(
                        a.partnerTippText,
                        "DAFÜR HIELT ${sie.uppercase()} DICH · VON MIR",
                        betont = false,
                    )
                }
                if (a.meineEchte.isNotBlank()) {
                    Aufloesungskarte(
                        a.meineEchte,
                        if (partnerRichtig) "DAS WARST DU · ${sie.uppercase()} HAT ES ERKANNT"
                        else "DAS WARST DU",
                        betont = true,
                    )
                }
            }
        }

        Aktionen { Knopf("Weiter", betont = true, aktiv = !modell.laden) { modell.weiter() } }
    }
}

// ------------------------------------------------------------ Einstellungen ---

@Composable
fun EinstellungenBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    var name by remember { mutableStateOf(modell.spitzname) }
    var loeschStufe by remember { mutableIntStateOf(0) } // 0 = zu, 1 = Dossier, 2 = alles
    var spielBeenden by remember { mutableStateOf(false) }
    var partyVerlassen by remember { mutableStateOf(false) }
    var bestaetigung by remember { mutableStateOf("") }
    var schluesselZeigen by remember { mutableStateOf(false) }
    val ablage = LocalClipboardManager.current

    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Was möchtest du ändern?")

        Panel(titel = "Spitzname") {
            Feld(name, { name = it }, hinweis = modell.spitzname)
            Spacer(Modifier.height(8.dp))
            Zeile("Der Name ändert sich, ich behalte trotzdem alles.", p.fgDim, 11)
            Spacer(Modifier.height(8.dp))
            Knopf(
                "Umbenennen",
                aktiv = name.isNotBlank() && name != modell.spitzname && !modell.laden,
            ) { modell.umbenennen(name) }
        }

        Panel(titel = "Farbschema") {
            PalettenWahl(modell)
        }

        if (modell.matchLaeuft) {
            Panel(titel = "Laufendes Spiel") {
                if (!spielBeenden) {
                    Zeile("Punktestand und Chronik bleiben stehen.", p.fgDim, 11)
                    Spacer(Modifier.height(8.dp))
                    Knopf("Spiel beenden") { spielBeenden = true }
                } else {
                    Zeile(
                        "Das Spiel endet für euch beide – auch mitten in einer Runde, " +
                            "auch wenn die andere Seite gerade schreibt. Danach könnt ihr " +
                            "ein neues starten.",
                        p.warn, 12,
                    )
                    Spacer(Modifier.height(10.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Knopf("Doch nicht") { spielBeenden = false }
                        Knopf("Beenden", aktiv = !modell.laden) {
                            spielBeenden = false
                            modell.matchAbbrechen()
                        }
                    }
                }
            }
        }

        if (modell.inParty) {
            Panel(titel = "Party") {
                if (!partyVerlassen) {
                    Zeile("Ihr spielt zu zweit mit ${modell.partnerName}.", p.fgDim, 11)
                    Spacer(Modifier.height(8.dp))
                    Knopf("Party verlassen") { partyVerlassen = true }
                } else {
                    Zeile(
                        "Die Party wird für euch beide aufgelöst, ein laufendes Spiel " +
                            "endet dabei. Dein Dossier, deine Tags und dein Konto bleiben — " +
                            "MIMIK vergisst nichts, nur weil ihr neu antretet. Danach kannst " +
                            "du eine neue Party gründen oder einer beitreten.",
                        p.warn, 12,
                    )
                    Spacer(Modifier.height(10.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Knopf("Doch nicht") { partyVerlassen = false }
                        Knopf("Verlassen", aktiv = !modell.laden) {
                            partyVerlassen = false
                            modell.partyVerlassen()
                        }
                    }
                }
            }
        }

        Panel(titel = "Intro") {
            Zeile("MIMIKs Ansage noch einmal von vorn.", p.fgDim, 11)
            Spacer(Modifier.height(8.dp))
            Knopf("Erneut ansehen") { modell.introZeigen() }
        }

        Fassungsfeld(modell)

        // Der Schlüssel ist das Konto. Er steht hier, weil es sonst keinen Weg
        // zurück gäbe: Wechselt die Signatur oder geht das Telefon verloren, ist
        // der lokale Speicher weg und auf dem Server hängt alles an dieser
        // Zeichenkette. Siehe Umzug.kt.
        Panel(titel = "Dieses Gerät") {
            if (!schluesselZeigen) {
                Zeile(
                    "Dein Schlüssel bringt dieses Konto auf ein anderes Gerät – " +
                        "oder auf eine Neuinstallation.",
                    p.fgDim, 11,
                )
                Spacer(Modifier.height(8.dp))
                Knopf("Schlüssel zeigen") { schluesselZeigen = true }
            } else {
                Zeile(
                    "Wer ihn hat, ist du. Gib ihn niemandem, den du nicht selbst bist.",
                    p.warn, 11,
                )
                Spacer(Modifier.height(8.dp))
                Zeile(schluesselAnzeige(modell.schluessel), p.fg, 12)
                Spacer(Modifier.height(10.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Knopf("Kopieren") {
                        ablage.setText(AnnotatedString(modell.schluessel))
                    }
                    Knopf("Verbergen") { schluesselZeigen = false }
                }
            }
        }

        // Was MIMIK über einen weiß, steht bisher nur im Prompt. Wer sich
        // anschauen will, was da über ihn notiert ist, soll das können - die
        // Fakten sind aus den eigenen Antworten gezogen, es ist sein Material.
        modell.dossier?.let { d ->
            // Ausdrücklich getrennt vom Dossier: Fakten sind Zitate aus den
            // eigenen Antworten, Merkmale sind Schlüsse daraus. Das eine ist
            // belegt, das andere geraten – und wer das nicht auseinanderhält,
            // liest eine Vermutung als Tatsache über sich selbst.
            Panel(
                titel = "Was ich über dich vermute",
                rechts = if (d.profil.isEmpty()) null else "${d.profil.size}",
            ) {
                if (d.profil.isEmpty()) {
                    Zeile("Noch nichts. Ich schaue nach jeder Runde nach.", p.fgDim, 12)
                } else {
                    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        d.profil.forEach { m ->
                            Column(Modifier.fillMaxWidth()) {
                                Zeile("${m.merkmal}: ${m.wert}", p.fg, 12)
                                Zeile(
                                    m.stand + (if (m.beleg.isBlank()) "" else " · ${m.beleg}"),
                                    p.fgDim, 11,
                                )
                            }
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    Zeile(
                        "Das sind Vermutungen, keine Tatsachen – sie dürfen falsch sein. " +
                            "„Mein Dossier löschen“ nimmt sie mit.",
                        p.fgDim, 11,
                    )
                }
            }
        }

        Panel(titel = "Mein Dossier", rechts = modell.dossier?.let { "${it.fakten.size} Fakten" }) {
            val d = modell.dossier
            if (d == null) {
                Zeile("Was ich aus deinen Antworten mitgeschrieben habe.", p.fgDim, 11)
                Spacer(Modifier.height(8.dp))
                Knopf("Ansehen", aktiv = !modell.laden) { modell.dossierLaden() }
            } else {
                if (d.fakten.isEmpty()) {
                    Zeile("Noch nichts. Das ändert sich nach der ersten Runde.", p.fgDim, 12)
                } else {
                    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        d.fakten.forEach { f -> Zeile("· $f", p.fg, 12) }
                    }
                }
                if (d.verbrauchteThemen.isNotEmpty()) {
                    Spacer(Modifier.height(12.dp))
                    Zeile("Verbrauchte Themen", p.accent, 11)
                    Spacer(Modifier.height(4.dp))
                    // Einmal benutzt, nicht noch einmal: Diese Themen nimmt
                    // MIMIK für Fälschungen nicht mehr her.
                    Zeile(d.verbrauchteThemen.sorted().joinToString(", "), p.fgDim, 12)
                }
                if (d.tags.isNotEmpty()) {
                    Spacer(Modifier.height(12.dp))
                    Zeile("Deine Themen", p.accent, 11)
                    Spacer(Modifier.height(4.dp))
                    Zeile(d.tags.sorted().joinToString(", "), p.fgDim, 12)
                }
                Spacer(Modifier.height(10.dp))
                Knopf("Neu laden", aktiv = !modell.laden) { modell.dossierLaden() }
            }
        }

        Panel(titel = "Daten auf dem Server") {
            when (loeschStufe) {
                1 -> {
                    Zeile(
                        "Alle Fakten und alle Vermutungen, die ich aus deinen Antworten " +
                            "gezogen habe, werden gelöscht. Deine Tags bleiben, sie sind " +
                            "eine Einstellung. Der laufende Punktestand bleibt auch.",
                        p.warn, 12,
                    )
                    Spacer(Modifier.height(10.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Knopf("Abbrechen") { loeschStufe = 0 }
                        Knopf("Löschen", aktiv = !modell.laden) {
                            modell.dossierLoeschen()
                            loeschStufe = 0
                        }
                    }
                }
                2 -> {
                    Zeile(
                        "Konto, Tags, Dossier und die gemeinsame Party mit allen Runden " +
                            "werden gelöscht. Die Party gehört euch beiden, sie geht also mit. " +
                            "Was deinem Gegenüber allein gehört, bleibt. " +
                            "Das lässt sich nicht rückgängig machen.",
                        p.error, 12,
                    )
                    Spacer(Modifier.height(10.dp))
                    Zeile("Tippe zur Bestätigung „${modell.spitzname}“", p.fgDim, 11)
                    Feld(bestaetigung, { bestaetigung = it }, hinweis = modell.spitzname)
                    Spacer(Modifier.height(10.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        Knopf("Abbrechen") { loeschStufe = 0; bestaetigung = "" }
                        Knopf(
                            "Alles löschen",
                            aktiv = bestaetigung.trim().equals(modell.spitzname, true) && !modell.laden,
                        ) { modell.kontoLoeschen(bestaetigung) }
                    }
                }
                else -> {
                    Zeile("Zwei verschiedene Dinge – das kleinere zuerst.", p.fgDim, 11)
                    Spacer(Modifier.height(10.dp))
                    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                        Knopf("Mein Dossier löschen") { loeschStufe = 1 }
                        Knopf("Alles löschen") { loeschStufe = 2 }
                    }
                }
            }
        }

        Aktionen { Knopf("Zurück", betont = true) { modell.einstellungen(false) } }
    }
}

/**
 * Welche Fassung läuft und ob es eine neuere gibt.
 *
 * MIMIK wird nicht über einen Store verteilt, also fragt die App selbst bei den
 * Releases nach. Installieren tut sie nichts still: Geladen wird auf Knopfdruck,
 * und den Installer führt das System.
 */
@Composable
private fun Fassungsfeld(modell: AppModel) {
    val p = LokalePalette.current
    val neu = modell.angeboten
    val laedt = modell.fassungLaedt
    Panel(titel = "Fassung", rechts = modell.fassung, betont = neu != null) {
        when {
            laedt != null -> {
                Zeile("Lade Fassung ${neu?.name.orEmpty()} … $laedt %", p.accent, 12)
                Spacer(Modifier.height(8.dp))
                Zeile(
                    "Danach fragt Android, ob es installieren darf. Deine Anmeldung " +
                        "bleibt dabei stehen.",
                    p.fgDim, 11,
                )
            }
            neu != null -> {
                Zeile("Fassung ${neu.name} ist da.", p.accent, 13)
                if (neu.notiz.isNotBlank()) {
                    Spacer(Modifier.height(6.dp))
                    Zeile(neu.notiz.lineSequence().take(6).joinToString("\n"), p.fgDim, 11)
                }
                Spacer(Modifier.height(10.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Knopf("Jetzt laden", betont = true) { modell.fassungHolen() }
                    Knopf("Später") { modell.fassungUebergehen() }
                }
            }
            else -> {
                Zeile("MIMIK ${modell.fassung}. Updates kommen von GitHub.", p.fgDim, 11)
                Spacer(Modifier.height(8.dp))
                Knopf("Nach Updates suchen", aktiv = !modell.fassungSucht) {
                    modell.fassungSuchen()
                }
            }
        }
        modell.fassungHinweis?.let {
            Spacer(Modifier.height(8.dp))
            Zeile(it, p.fgDim, 11)
        }
    }
}

@Composable
private fun PalettenWahl(modell: AppModel) {
    val p = LokalePalette.current
    Row(horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.fillMaxWidth()) {
        PALETTEN.forEach { pal ->
            val an = pal.id == modell.palette.id
            Box(
                Modifier.weight(1f)
                    .border(1.dp, if (an) p.accent2 else p.border)
                    .background(p.bgAlt)
                    .clickable { modell.paletteWaehlen(pal.id) }
                    .padding(vertical = 7.dp),
                contentAlignment = Alignment.Center,
            ) { Text(pal.id.take(5), fontSize = 10.sp, color = if (an) p.accent2 else p.fgDim) }
        }
    }
}
