package app.mimik

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.Text
import androidx.compose.ui.text.style.LineBreak
import kotlinx.coroutines.delay

/**
 * MIMIK als Zeichenmatrix. Kein Bild, keine Animationsbibliothek: Das Gesicht
 * ist ein Bauplan aus vier beweglichen Teilen, pro Bild wird nur der Text
 * getauscht – dieselbe Struktur wie in der Figurenstudie.
 */
enum class Miene { Bereit, Denkt, Triumph, Getroffen, Neutral }

private data class Bild(
    val antenne: Char,
    val stiel: Char,
    val auge: Char,
    val mitte: String,
    val mund: String,
    val dauer: Long,
    val hell: Boolean = false,
    val blass: Boolean = false,
    val antenneX: Int = 5,
)

private const val MITTE_NASE = "    ·    "
private const val MITTE_TRAENE = " ˙       "
private const val MITTE_LEER = "         "
private const val MUND_NEUTRAL = "─────"

private fun zeile(z: Char, x: Int) = buildString { repeat(11) { append(if (it == x) z else ' ') } }

private fun male(b: Bild) = buildString {
    appendLine(zeile(b.antenne, b.antenneX))
    appendLine(zeile(b.stiel, 5))
    appendLine("╔═════════╗")
    appendLine("║ ${b.auge}     ${b.auge} ║")
    appendLine("║${b.mitte}║")
    appendLine("║  ${b.mund}  ║")
    append("╚══╤═══╤══╝")
}

private const val MUND_SCHMAL = " ─── "
private const val MUND_SCHIEF = "──╯  "
private const val MUND_SCHIEF2 = "  ╰──"
private const val MUND_SCHMUNZELN = "╲───╱"
private const val MUND_SPITZ = "  ·  "

/**
 * Leerlauf: atmen, blinzeln, und hin und wieder ein Zug um den Mund.
 *
 * Vorher bewegten sich nur Antenne und Augen, und die Figur wirkte im Warten
 * wie abgeschaltet. Jetzt läuft ein zweiter, längerer Takt mit: ein Zucken auf
 * einer Seite, ein kurzes Schmunzeln, dann wieder gerade. Wichtig ist die
 * Ungleichzeitigkeit – blinzeln und schmunzeln fallen nie zusammen, sonst
 * sieht es nach einer Schleife aus statt nach jemandem, der wartet.
 */
private val LEERLAUF = listOf(
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 900),
    Bild('∘', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 500),
    Bild('·', '│', '▄', MITTE_NASE, MUND_SCHMAL, 420),
    Bild('∘', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 500),
    // Blinzeln.
    Bild('◦', '│', '▂', MITTE_NASE, MUND_NEUTRAL, 70),
    Bild('◦', '│', '▁', MITTE_NASE, MUND_NEUTRAL, 80),
    Bild('◦', '│', '▂', MITTE_NASE, MUND_NEUTRAL, 70),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 1200),
    // Ein Zucken, erst die eine Seite, dann die andere.
    Bild('∘', '│', '▄', MITTE_NASE, MUND_SCHIEF, 260),
    Bild('∘', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 700),
    Bild('·', '│', '▖', MITTE_NASE, MUND_NEUTRAL, 380),
    Bild('·', '│', '▗', MITTE_NASE, MUND_SCHIEF2, 380),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 900),
    // Kurz schelmisch - der einzige Moment, in dem sie zeigt, dass ihr das
    // hier Spass macht. Danach sofort wieder gerade.
    Bild('◦', '│', '▀', MITTE_NASE, MUND_SCHMUNZELN, 420),
    Bild('●', '│', '▀', MITTE_NASE, MUND_SCHMUNZELN, 240, hell = true),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_SCHMAL, 300),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 1100),
    // Nochmal blinzeln, diesmal an anderer Stelle im Takt.
    Bild('∘', '│', '▂', MITTE_NASE, MUND_NEUTRAL, 70),
    Bild('∘', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 800),
    Bild('·', '│', '▄', MITTE_NASE, MUND_SPITZ, 300),
    Bild('∘', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 700),
)

// 20 Bilder à 110 ms gehen glatt auf: 5 Antennenumdrehungen, 4 Mundläufe,
// 5 Blicke. Ohne das ruckelt der Übergang an der Schleifennaht.
private val DENKT = List(20) { i ->
    val spin = listOf('│', '╱', '─', '╲')[i % 4]
    val blick = listOf('▖', '▄', '▗', '▄')[i % 4]
    val punkte = listOf("●····", "·●···", "··●··", "···●·", "····●")[i % 5]
    Bild(spin, '│', blick, MITTE_LEER, punkte, 110)
}

private val TRIUMPH = listOf(
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 90),
    Bild('◦', '│', '▄', MITTE_NASE, "╲───╱", 90),
    Bild('●', '│', '▀', MITTE_NASE, "╲___╱", 700),
    Bild('●', '│', '▀', MITTE_NASE, "╲___╱", 120, hell = true),
    Bild('●', '│', '▀', MITTE_NASE, "╲___╱", 500),
    Bild('◦', '│', '▄', MITTE_NASE, "╲───╱", 110),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 110),
)

private val GETROFFEN = listOf(
    Bild('◦', '│', '█', MITTE_LEER, MUND_NEUTRAL, 90),
    Bild('·', '╵', '▄', MITTE_LEER, "╱───╲", 90, blass = true, antenneX = 4),
    Bild('·', '╵', '▄', MITTE_TRAENE, "╱‾‾‾╲", 800, blass = true, antenneX = 4),
    Bild('·', '╵', '▄', MITTE_LEER, "╱‾‾‾╲", 200, blass = true, antenneX = 4),
    Bild('·', '╵', '▄', MITTE_TRAENE, "╱‾‾‾╲", 400, blass = true, antenneX = 4),
    Bild('◦', '│', '▄', MITTE_LEER, "╱───╲", 110, blass = true),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 110),
)

/**
 * Unentschieden: einer hat sie erwischt, einen hat sie erwischt. Kein Triumph,
 * kein Schmerz – ein Achselzucken, das sie nicht zugibt. Deshalb nur die
 * Augenbrauen, ein kurzer Blick zur Seite und ein gerader Mund.
 */
private val NEUTRAL = listOf(
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 90),
    Bild('◦', '│', '▀', MITTE_NASE, MUND_SCHMAL, 200),
    Bild('◦', '│', '▖', MITTE_NASE, MUND_NEUTRAL, 700),
    Bild('∘', '│', '▗', MITTE_NASE, MUND_NEUTRAL, 500),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_NEUTRAL, 400),
    Bild('◦', '│', '▄', MITTE_NASE, MUND_SCHMAL, 150),
)

private fun bilder(m: Miene) = when (m) {
    Miene.Bereit -> LEERLAUF
    Miene.Denkt -> DENKT
    Miene.Triumph -> TRIUMPH
    Miene.Getroffen -> GETROFFEN
    Miene.Neutral -> NEUTRAL
}

/**
 * Triumph und Getroffen sind Einmalfiguren: hinein, kurz stehen, zurück in den
 * Leerlauf. Die Figur bleibt nie in einer Miene hängen – sonst wirkt sie
 * eingefroren statt lebendig.
 */
private fun einmalig(m: Miene) =
    m == Miene.Triumph || m == Miene.Getroffen || m == Miene.Neutral

@Composable
fun MimikGesicht(
    miene: Miene,
    modifier: Modifier = Modifier,
    schrift: Int = 11,
    haltend: Boolean = false,
    rahmen: Boolean = true,
) {
    val p = LokalePalette.current
    var aktuell by remember(miene) { mutableIntStateOf(0) }
    var laeuft by remember(miene) { mutableIntStateOf(0) } // 0 = Miene, 1 = zurück im Leerlauf
    var steht by remember(miene) { mutableStateOf(false) }

    val satz = if (laeuft == 0) bilder(miene) else LEERLAUF
    val bild = satz[aktuell.coerceIn(0, satz.lastIndex)]

    // Das längste Bild einer Miene ist ihr Standbild – bei Triumph die 700 ms,
    // bei Getroffen die 800 ms. Darauf hält "haltend" an.
    val standbild = remember(satz) { satz.indices.maxByOrNull { satz[it].dauer } ?: 0 }

    LaunchedEffect(miene, laeuft, aktuell, steht) {
        if (steht) return@LaunchedEffect
        // Auf einem Ergebnisbildschirm bleibt die Miene stehen: Dort ist sie eine
        // Aussage über den Ausgang, keine flüchtige Reaktion. Im Spiel kehrt sie
        // dagegen in den Leerlauf zurück.
        if (haltend && einmalig(miene) && aktuell == standbild) {
            steht = true
            return@LaunchedEffect
        }
        delay(bild.dauer)
        val naechste = aktuell + 1
        if (naechste >= satz.size) {
            aktuell = 0
            if (laeuft == 0 && einmalig(miene)) laeuft = 1
        } else {
            aktuell = naechste
        }
    }

    val ziel = when {
        bild.hell -> p.warn
        bild.blass -> p.fgDim
        else -> p.accent
    }
    val farbe by animateColorAsState(ziel, tween(90, easing = LinearEasing), label = "mimikFarbe")

    // Das Gesicht bricht NICHT um. Die App setzt sonst ausgeglichen (Theme.kt),
    // und jeder Umbruch zerlegt hier die Zeichenmatrix in Fragmente - derselbe
    // Grund, aus dem die Schrift mitgeliefert wird.
    Text(
        text = male(bild),
        color = farbe,
        fontFamily = MonoSchrift,
        fontSize = schrift.sp,
        lineHeight = (schrift * 1.18f).sp,
        textAlign = TextAlign.Start,
        softWrap = false,
        style = LocalTextStyle.current.copy(lineBreak = LineBreak.Simple),
        modifier = if (rahmen) {
            modifier.background(p.bgInset).border(1.dp, p.border)
                .padding(horizontal = 10.dp, vertical = 8.dp)
        } else {
            modifier
        },
    )
}

/**
 * MIMIK als Kopf der Seite: groß, ohne Rahmen, horizontal zentriert, darunter
 * eine Zeile. So liest sich der Bildschirm, als frage man sie, was ansteht.
 */
@Composable
fun MimikKopf(
    miene: Miene,
    zeile: String,
    modifier: Modifier = Modifier,
    haltend: Boolean = false,
    schrift: Int = 17,
) {
    val p = LokalePalette.current
    Column(
        modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        MimikGesicht(miene, schrift = schrift, haltend = haltend, rahmen = false)
        Spacer(Modifier.height(10.dp))
        Text(
            zeile, color = p.fgDim, fontSize = 12.sp, lineHeight = 18.sp,
            textAlign = TextAlign.Center, modifier = Modifier.fillMaxWidth(),
        )
    }
}

/** Der Zustand ist reine Ableitung aus dem Spielstand, keine eigene Laune. */
fun mieneFuer(zustand: String, letzterTreffer: Boolean?): Miene = when {
    zustand == "MIMIK_ARBEITET" -> Miene.Denkt
    letzterTreffer == true -> Miene.Getroffen
    letzterTreffer == false -> Miene.Triumph
    else -> Miene.Bereit
}

@Suppress("unused")
private val unbenutzt: Color = Color.Transparent
