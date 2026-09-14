package app.mimik

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay

/**
 * Die Übersicht. Sie ist der neue Mittelpunkt der App: Ein Spieler kann in
 * mehreren Partien gleichzeitig sein, und aus jeder führt ein Weg hierher
 * zurück.
 */
@Composable
fun LobbyBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val zeilen = modell.zeilen
    val ein = modell.einladungen.eingehend
    val aus = modell.einladungen.ausgehend

    Huelle(modell) {
        MimikKopf(
            Miene.Bereit,
            when {
                zeilen.any { it.faellig } -> "Bei dir liegt etwas."
                zeilen.isEmpty() -> "Noch niemand. Lade jemanden ein."
                else -> "Nichts liegt an. Ich warte mit."
            },
        )

        Panel(titel = "Partien", rechts = if (zeilen.isEmpty()) null else "${zeilen.size}") {
            if (zeilen.isEmpty()) {
                Zeile("Such jemanden, oder tritt mit einem Code bei.", p.fgDim, 12)
            } else {
                Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    zeilen.forEach { z ->
                        PartieZeile(z) { modell.partieOeffnen(z.partyId) }
                    }
                }
            }
        }

        if (ein.isNotEmpty() || aus.isNotEmpty()) {
            Panel(titel = "Einladungen", betont = ein.isNotEmpty()) {
                Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    ein.forEach { e ->
                        Column(Modifier.fillMaxWidth()) {
                            Zeile("${e.gegenueber.spitzname} will gegen dich spielen.", p.fg, 12)
                            Spacer(Modifier.height(8.dp))
                            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                Knopf("Annehmen", betont = true, aktiv = !modell.laden) {
                                    modell.einladungAnnehmen(e.id)
                                }
                                Knopf("Ablehnen", aktiv = !modell.laden) {
                                    modell.einladungAblehnen(e.id)
                                }
                            }
                        }
                    }
                    aus.forEach { e ->
                        Row(
                            Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Zeile("An ${e.gegenueber.spitzname} · offen", p.fgDim, 12)
                            Knopf("Zurückziehen", aktiv = !modell.laden) {
                                modell.einladungZurueckziehen(e.id)
                            }
                        }
                    }
                }
            }
        }

        Suchfeld(modell)

        Panel(titel = "Mit Code") {
            MitCode(modell)
        }
    }
}

/**
 * Eine Partie in der Übersicht.
 *
 * Der Punktebalken ist derselbe wie im Spiel – absichtlich: Wer sechs Partien
 * hat, soll sie an der Form wiedererkennen, die er im Spiel sieht, statt an
 * einer zweiten, nur für die Liste erfundenen. Ohne `von`-Werte, sonst liefe
 * bei jedem Abgleich die ganze Liste los.
 */
@Composable
private fun PartieZeile(z: LobbyPartie, beiKlick: () -> Unit) {
    val p = LokalePalette.current
    val farbe = when (z.dran) {
        "schreiben", "raten" -> p.accent
        "aufgeloest" -> p.accent2
        else -> p.fgDim
    }
    Klickbar(
        Modifier.fillMaxWidth()
            .border(1.dp, if (z.faellig) farbe else p.border)
            .background(if (z.faellig) p.bgAlt else p.bg),
        beiKlick = beiKlick,
    ) {
        Column(Modifier.fillMaxWidth().padding(10.dp)) {
            Row(
                Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    z.partner?.spitzname ?: "noch niemand",
                    color = p.fg, fontSize = 14.sp, maxLines = 1,
                )
                Text(
                    etikett(z.dran), color = farbe, fontSize = 10.sp,
                    letterSpacing = 1.1.sp, maxLines = 1,
                )
            }
            if (z.partner != null) {
                Spacer(Modifier.height(8.dp))
                Punktebalken(z.stand.mensch, z.stand.mimik, z.ziel)
            }
        }
    }
}

@Composable
private fun Suchfeld(modell: AppModel) {
    val p = LokalePalette.current
    var suche by remember { mutableStateOf("") }
    Panel(titel = "Jemanden einladen") {
        Feld(suche, { suche = it; modell.suchen(it) }, hinweis = "Spitzname")
        if (suche.trim().length >= 2) {
            Spacer(Modifier.height(10.dp))
            if (modell.treffer.isEmpty()) {
                Zeile("Niemand mit diesem Namen.", p.fgDim, 11)
            }
            Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                modell.treffer.forEach { t ->
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(t.spitzname, color = p.fg, fontSize = 13.sp, maxLines = 1)
                        // Wen man schon eingeladen hat, lädt man nicht zweimal ein.
                        val schon = modell.einladungOffenMit(t.id)
                        Knopf(
                            if (schon) "eingeladen" else "Einladen",
                            betont = !schon, aktiv = !schon && !modell.laden,
                        ) { modell.einladen(t.id) }
                    }
                }
            }
        } else {
            Spacer(Modifier.height(8.dp))
            Zeile("Ab zwei Zeichen suche ich.", p.fgDim, 11)
        }
    }
}

@Composable
private fun MitCode(modell: AppModel) {
    val p = LokalePalette.current
    var code by remember { mutableStateOf("") }
    var offen by remember { mutableStateOf(false) }
    if (!offen) {
        Zeile("Gründen und den Code weitergeben – oder einen eingeben.", p.fgDim, 11)
        Spacer(Modifier.height(10.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Knopf("Partie gründen", aktiv = !modell.laden) { modell.partyAnlegen() }
            Knopf("Code eingeben") { offen = true }
        }
        Spacer(Modifier.height(8.dp))
        Zeile("Code TEST spielt gegen einen Testspieler.", p.fgDim, 11)
    } else {
        Feld(code, { code = it.uppercase() }, hinweis = "ABC123")
        Spacer(Modifier.height(10.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Knopf("Doch nicht") { offen = false; code = "" }
            // Vier statt sechs, damit auch TEST durchgeht.
            Knopf("Beitreten", betont = true, aktiv = code.trim().length >= 4 && !modell.laden) {
                modell.partyBeitreten(code)
            }
        }
    }
}

/** Die gegründete Partie, solange niemand beigetreten ist. */
@Composable
fun WarteraumBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val party = modell.zustand?.party ?: return
    Huelle(modell) {
        MimikKopf(Miene.Denkt, "Ich warte auf die zweite Person.")
        Zeile("Gib diesen Code weiter", p.fgDim, 11)
        Text(
            party.code.ifBlank { "—" }, color = p.accent2, fontSize = 38.sp,
            letterSpacing = 8.sp, maxLines = 1,
        )
        Wartezeile("Sobald sie beitritt, geht es hier von selbst weiter.")
        Aktionen {
            Knopf("Abbrechen", aktiv = !modell.laden) { modell.partyVerlassen() }
        }
    }
}

/**
 * Kommt vor allem anderen, sobald der Server sagt, der Name sei nicht mehr
 * eindeutig.
 *
 * Warum überhaupt: Spitznamen sind suchbar geworden. Tragen zwei Menschen
 * denselben, zeigt jede Einladung auf „eine von beiden" – und das lässt sich
 * nicht anzeigen, nur verhindern. Spielen darf man trotzdem weiter; man ist nur
 * nicht auffindbar, bis der Name steht.
 */
@Composable
fun NameBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    var name by remember { mutableStateOf("") }
    Huelle(modell) {
        MimikKopf(Miene.Denkt, "Deinen Namen trägt inzwischen noch jemand.", schrift = 19)
        Panel(titel = "Neuer Name", betont = true) {
            Zeile(
                "Ich brauche einen Namen, der dich eindeutig macht – sonst landen " +
                    "Einladungen bei der falschen Person. Solange du keinen hast, " +
                    "findet dich niemand über die Suche.",
                p.fgDim, 12,
            )
            Spacer(Modifier.height(10.dp))
            Feld(name, { name = it }, hinweis = modell.spitzname)
        }
        Aktionen {
            Knopf("Übernehmen", betont = true, aktiv = name.trim().length >= 2 && !modell.laden) {
                modell.umbenennen(name)
            }
        }
    }
}

/**
 * Der Rückweg in die Übersicht, oben links – wie das Zahnrad eine Überlagerung
 * und kein Teil des Bildschirms: Sonst bräuchte jeder Spielbildschirm eine
 * Kopfleiste, und die mittige Anordnung wäre hin.
 */
@Composable
fun ZurueckEcke(modell: AppModel) {
    if (!modell.zurueckSichtbar) return
    val p = LokalePalette.current
    Box(Modifier.fillMaxSize().safeDrawingPadding(), contentAlignment = Alignment.TopStart) {
        Box(
            Modifier
                .padding(10.dp)
                .border(1.dp, p.border)
                .background(p.bgAlt),
        ) {
            Klickbar(beiKlick = { modell.zurueckZurLobby() }) {
                Text(
                    "‹ ÜBERSICHT", color = p.fgDim, fontSize = 10.sp, letterSpacing = 1.1.sp,
                    maxLines = 1, modifier = Modifier.padding(horizontal = 9.dp, vertical = 8.dp),
                )
            }
        }
    }
}

/**
 * Der Klonblick: die eigene Antwort, und dahinter die drei Fälschungen, die
 * MIMIK daraus gebaut hat – mit dem Satz, woraus sie sie gebaut hat.
 *
 * Warum das niemandem etwas verrät: Geraten wird über die ANDERE Seite. Wer
 * seine eigene Antwort getippt hat, weiß ohnehin, welche der vier Karten sie
 * ist. Umgekehrt ist es der einzige Moment, in dem man MIMIK bei der Arbeit
 * zusieht – und er liegt genau dort, wo bisher nur gewartet wurde: Der eigene
 * Kartensatz steht, sobald MIMIK ihn gebaut hat, lange bevor die andere Seite
 * geantwortet hat.
 *
 * Die Klone erscheinen einer nach dem anderen. Nicht als Zierde: Sie treten
 * hinter der echten Antwort an, und das Nacheinander ist das, was aus vier
 * Textblöcken ein Klonen macht.
 */
@Composable
fun KloneBildschirm(modell: AppModel) {
    val p = LokalePalette.current
    val r = modell.aktuelleRunde ?: return
    val echt = r.meineKarten.firstOrNull { it.istEcht == true }
    val klone = r.meineKarten.filter { it.istEcht != true }

    var bis by remember(r.id) { mutableStateOf(0) }
    val fertig = bis >= klone.size
    LaunchedEffect(r.id, bis) {
        if (bis >= klone.size) return@LaunchedEffect
        // Der erste Klon braucht länger als die folgenden: Erst soll die eigene
        // Antwort einen Moment allein dastehen.
        delay(if (bis == 0) 900 else 650)
        bis += 1
    }

    Huelle(modell) {
        MimikKopf(
            if (fertig) Miene.Triumph else Miene.Denkt,
            if (fertig) "Drei davon bin ich. Findest du dich wieder?"
            else "Ich schreibe dich nach.",
            schrift = 19,
        )
        Frage(r.frage)

        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            Klonkarte(echt?.text.orEmpty(), "DU", "", betont = true)
            klone.take(bis).forEach { k ->
                Klonkarte(k.text, "MIMIK", k.begruendung, betont = false)
            }
            if (!fertig) {
                // Ein Platzhalter, der nicht springt: Ohne ihn wächst die Spalte
                // mit jedem Klon, und weil die Hülle mittig anordnet, rutscht
                // der ganze Bildschirm bei jedem Schritt nach oben.
                Box(Modifier.fillMaxWidth().height(52.dp)) {
                    Text(
                        "▍", color = p.accent, fontSize = 14.sp,
                        fontFamily = MonoSchrift,
                        modifier = Modifier.padding(start = 12.dp, top = 12.dp),
                    )
                }
            }
        }

        if (fertig) {
            Aktionen {
                Knopf("Weiter", betont = true, aktiv = !modell.laden) { modell.klonWeiter() }
            }
        } else {
            Zeile("Ich bin noch nicht fertig.", p.fgDim, 11)
        }
    }
}

@Composable
private fun Klonkarte(text: String, etikett: String, grund: String, betont: Boolean) {
    val p = LokalePalette.current
    val farbe = if (betont) p.accent2 else p.accent
    Box(
        Modifier.fillMaxWidth()
            .border(1.dp, if (betont) p.accent2 else p.border)
            .background(if (betont) p.bgAlt else p.bg)
            .padding(12.dp),
    ) {
        Column {
            Text(etikett, color = farbe, fontSize = 10.sp, letterSpacing = 1.1.sp, maxLines = 1)
            Spacer(Modifier.height(6.dp))
            Text(text, color = p.fg, fontSize = 14.sp, lineHeight = 22.sp)
            if (grund.isNotBlank()) {
                Spacer(Modifier.height(8.dp))
                Text(grund, color = p.fgDim, fontSize = 11.sp, lineHeight = 17.sp)
            }
        }
    }
}
