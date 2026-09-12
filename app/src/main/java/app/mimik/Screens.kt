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
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
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
private fun Frage(text: String) {
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
private fun Aktionen(inhalt: @Composable () -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) { inhalt() }
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
    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Wir kennen uns noch nicht.", schrift = 19)
        Text(
            "MIMIK", color = p.fg, fontSize = 22.sp, fontWeight = FontWeight.Bold,
            letterSpacing = 6.sp,
        )
        Zeile("Vier Antworten. Eine ist echt.", p.fgDim, 12)
        Panel(titel = "Anmelden") {
            Zeile("Serveradresse", p.fgDim, 11)
            Feld(url, { url = it }, hinweis = "http://…:8080")
            Spacer(Modifier.height(10.dp))
            Zeile("Dein Spitzname", p.fgDim, 11)
            Feld(name, { name = it }, hinweis = "Wie heißt du im Spiel?")
            Spacer(Modifier.height(10.dp))
            Zeile("Einladung, falls der Server eine verlangt", p.fgDim, 11)
            Feld(einladung, { einladung = it }, hinweis = "leer lassen, wenn keine")
        }
        Aktionen {
            Knopf("Gerät anmelden", betont = true, aktiv = name.isNotBlank() && !modell.laden) {
                modell.serverSetzen(url)
                modell.anmelden(name, einladung)
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

    LaunchedEffect(bis) {
        if (fertig) return@LaunchedEffect
        delay(pause(INTRO_WORTE[bis]))
        bis += 1
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

// ------------------------------------------------------------------- Party ---

@Composable
fun PartyBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    var code by remember { mutableStateOf("") }
    val party = modell.zustand?.party
    Huelle(modell) {
        if (party == null) {
            MimikKopf(Miene.Bereit, "Zu zweit. Einer gründet, einer tritt bei.")
            Aktionen {
                Knopf("Party gründen", betont = true, aktiv = !modell.laden) { modell.partyAnlegen() }
            }
            Zeile("oder", p.fgDim, 11)
            Panel(titel = "Einladungscode") {
                Feld(code, { code = it.uppercase() }, hinweis = "ABC123")
            }
            Aktionen {
                Knopf("Beitreten", aktiv = code.length >= 6 && !modell.laden) {
                    modell.partyBeitreten(code)
                }
            }
        } else {
            MimikKopf(Miene.Denkt, "Ich warte auf die zweite Person.")
            Zeile("Gib diesen Code weiter", p.fgDim, 11)
            Text(party.code.ifBlank { "—" }, color = p.accent2, fontSize = 38.sp, letterSpacing = 8.sp)
            Aktionen { Knopf("Nachsehen", betont = true, aktiv = !modell.laden) { modell.aktualisieren() } }
        }
    }
}

// -------------------------------------------------------------------- Tags ---

@Composable
fun TagsBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    LaunchedEffect(Unit) { if (modell.tagsAuswahl == null) modell.tagsLaden() }
    val vorrat = modell.tagsAuswahl
    val gewaehlt = modell.gewaehlteTags
    Huelle(modell) {
        MimikKopf(Miene.Bereit, "Woran soll ich mich bei dir festhalten?")
        Text(
            "${gewaehlt.size} / 10", color = if (gewaehlt.size >= 10) p.accent2 else p.fgDim,
            fontSize = 15.sp,
        )
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            val alle = ((vorrat?.gewaehlt ?: emptyList()) + (vorrat?.vorschlaege ?: emptyList())).distinct()
            alle.chunked(2).forEach { paar ->
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
            Knopf("Weitere", aktiv = !modell.laden) { modell.tagsLaden() }
            Knopf("Speichern", betont = true, aktiv = gewaehlt.size >= 10 && !modell.laden) {
                modell.tagsSpeichern()
            }
        }
    }
}

// ------------------------------------------------- Kein Match / Matchende ---

@Composable
fun BasisBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val m = modell.zustand?.match
    val vorbei = m != null && m.ergebnis != "OFFEN"
    val gewonnen = m?.ergebnis == "MENSCH"
    Huelle(modell) {
        if (vorbei) {
            MimikKopf(
                if (gewonnen) Miene.Getroffen else Miene.Triumph,
                if (gewonnen) "Ihr habt mich durchschaut." else "Ich kenne euch besser, als ihr denkt.",
                haltend = true, schrift = 19,
            )
            Text(
                if (gewonnen) "IHR GEWINNT" else "MIMIK GEWINNT",
                color = if (gewonnen) p.accent2 else p.accent, fontSize = 19.sp, letterSpacing = 2.sp,
            )
            Punktebalken(m!!.stand.mensch, m.stand.mimik, m.ziel)
            Aktionen { Knopf("Revanche", betont = true, aktiv = !modell.laden) { modell.matchStarten() } }
        } else {
            MimikKopf(Miene.Bereit, "Bereit, wenn ihr es seid.")
            Aktionen { Knopf("Match starten", betont = true, aktiv = !modell.laden) { modell.matchStarten() } }
        }
    }
}

// --------------------------------------------------------------- Schreiben ---

@Composable
fun SchreibenBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val r = modell.aktuelleRunde ?: return
    var text by remember(r.id) { mutableStateOf("") }
    val vorschau = modell.vorschau

    Huelle(modell) {
        if (vorschau == null) {
            MimikKopf(Miene.Bereit, "Runde ${r.nummer} · beantwortet das mal ehrlich.")
            Frage(r.frage)
            Feld(text, { text = it }, hinweis = "Ein bis drei Sätze reichen", zeilen = 4)
            Aktionen {
                Knopf("Weiter", betont = true, aktiv = text.trim().length >= 4 && !modell.laden) {
                    modell.vorschauHolen(r.id, text.trim())
                }
            }
        } else {
            MimikKopf(Miene.Denkt, "So zeige ich deine Antwort.")
            Frage(r.frage)
            Panel(titel = "Deine Antwort", betont = true) {
                Text(vorschau.normalform, color = p.fg, fontSize = 14.sp, lineHeight = 22.sp)
            }
            Zeile("Rechtschreibung wird vereinheitlicht, deine Worte bleiben.", p.fgDim, 11)
            Aktionen {
                Knopf("Ändern") { modell.vorschau = null }
                Knopf("Passt", betont = true, aktiv = !modell.laden) {
                    modell.antwortSenden(r.id, vorschau.original, vorschau.normalform)
                }
            }
        }
    }
}

// ------------------------------------------------------------------ Warten ---

@Composable
fun WartenBildschirm(modell: AppModel) {
    val r = modell.aktuelleRunde ?: return
    val arbeitet = r.zustand == "MIMIK_ARBEITET"
    Huelle(modell) {
        MimikKopf(
            if (arbeitet) Miene.Denkt else Miene.Bereit,
            if (arbeitet) "Ich baue gerade drei Fälschungen.\nDas dauert einen Moment."
            else "Deine Antwort steht. Jetzt ist die andere Seite dran.",
            schrift = 19,
        )
        Frage(r.frage)
        Panel(titel = "Deine Antwort") {
            Text(
                r.meineAntwort, color = LokalePalette.current.fg,
                fontSize = 14.sp, lineHeight = 22.sp,
            )
        }
        if (r.fehler.isNotBlank()) Zeile(r.fehler, LokalePalette.current.warn, 11)
        Aktionen { Knopf("Nachsehen", betont = true, aktiv = !modell.laden) { modell.aktualisieren() } }
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
    Huelle(modell) {
        MimikKopf(Miene.Denkt, "Tipp steht.\nDie Auflösung kommt, sobald auch sie getippt hat.", schrift = 19)
        Frage(r.frage)
        Panel(titel = "Deine Wahl", betont = true) {
            Row {
                Text("[${r.meinTipp}] ", color = p.accent2, fontSize = 14.sp, fontWeight = FontWeight.Bold)
                Text(meine?.text.orEmpty(), color = p.fg, fontSize = 14.sp, lineHeight = 22.sp)
            }
        }
        Aktionen { Knopf("Nachsehen", betont = true, aktiv = !modell.laden) { modell.aktualisieren() } }
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
    val m = modell.zustand?.match
    val echt = r.karten.firstOrNull { it.istEcht == true }
    val meine = r.karten.firstOrNull { it.pos == r.meinTipp }

    Huelle(modell) {
        MimikKopf(
            if (richtig) Miene.Getroffen else Miene.Triumph,
            if (richtig) "Erwischt. Punkt für euch." else "Das war ich. Punkt für mich.",
            haltend = true, schrift = 19,
        )
        if (m != null) Punktebalken(m.stand.mensch, m.stand.mimik, m.ziel)
        Frage(r.frage)
        // Nur zeigen, worum es geht: bei einem Treffer die echte Karte, sonst
        // die falsch gewählte neben der echten. Alle vier noch einmal
        // durchzugehen heißt, den Moment im Rauschen zu ertränken.
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            if (!richtig && meine != null) {
                Aufloesungskarte(meine.text, "DEIN TIPP · VON MIR", betont = false)
            }
            if (echt != null) {
                Aufloesungskarte(
                    echt.text,
                    if (richtig) "ECHT · DEIN TIPP" else "ECHT",
                    betont = true,
                )
            }
        }
        if (a != null) {
            Zeile(
                if (a.doppeltreffer) "Ihr lagt beide daneben – Doppeltreffer für MIMIK."
                else if (a.partnerRichtig) "Dein Gegenüber hat dich erkannt."
                else "Dein Gegenüber ist auf MIMIK hereingefallen.",
                if (a.doppeltreffer) p.warn else p.fgDim, 12,
            )
        }
        Aktionen { Knopf("Nächste Frage", betont = true, aktiv = !modell.laden) { modell.weiter() } }
    }
}

// ------------------------------------------------------------ Einstellungen ---

@Composable
fun EinstellungenBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    var name by remember { mutableStateOf(modell.spitzname) }
    var loeschStufe by remember { mutableIntStateOf(0) } // 0 = zu, 1 = Dossier, 2 = alles
    var bestaetigung by remember { mutableStateOf("") }

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

        Panel(titel = "Intro") {
            Zeile("MIMIKs Ansage noch einmal von vorn.", p.fgDim, 11)
            Spacer(Modifier.height(8.dp))
            Knopf("Erneut ansehen") { modell.introZeigen() }
        }

        Panel(titel = "Daten auf dem Server") {
            when (loeschStufe) {
                1 -> {
                    Zeile(
                        "Alle Fakten, die ich aus deinen Antworten gezogen habe, werden " +
                            "gelöscht. Deine Tags bleiben, sie sind eine Einstellung. " +
                            "Der laufende Punktestand bleibt auch.",
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
