package app.mimik

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import kotlin.math.roundToInt
import kotlin.random.Random

/** Entspricht ui/components/Panel.svelte: Titel in accent, Versalien, 1px Rahmen. */
@Composable
fun Panel(
    titel: String? = null,
    rechts: String? = null,
    betont: Boolean = false,
    modifier: Modifier = Modifier,
    inhalt: @Composable () -> Unit,
) {
    val p = LokalePalette.current
    val rand = if (betont) p.accent else p.border
    Column(modifier.fillMaxWidth().border(1.dp, rand).background(p.bg)) {
        if (titel != null) {
            Row(
                Modifier.fillMaxWidth().background(p.bgAlt).padding(horizontal = 10.dp, vertical = 3.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    titel.uppercase(), color = p.accent, fontSize = 11.sp,
                    letterSpacing = 1.1.sp, fontWeight = FontWeight.Medium,
                )
                if (rechts != null) Text(rechts, color = p.fgDim, fontSize = 11.sp)
            }
        }
        Column(Modifier.padding(10.dp)) { inhalt() }
    }
}

/** Entspricht button / button.primary aus app.css. */
@Composable
fun Knopf(
    text: String,
    modifier: Modifier = Modifier,
    betont: Boolean = false,
    aktiv: Boolean = true,
    beiKlick: () -> Unit,
) {
    val p = LokalePalette.current
    Button(
        onClick = beiKlick,
        enabled = aktiv,
        shape = RectangleShape,
        border = BorderStroke(1.dp, if (betont) p.accent else p.border),
        colors = ButtonDefaults.buttonColors(
            containerColor = p.bgAlt,
            contentColor = if (betont) p.accent else p.fg,
            disabledContainerColor = p.bgAlt,
            disabledContentColor = p.fgDim,
        ),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 14.dp, vertical = 6.dp),
        modifier = modifier,
    ) { Text(text, fontSize = 13.sp) }
}

@Composable
fun Feld(
    wert: String,
    beiAenderung: (String) -> Unit,
    modifier: Modifier = Modifier,
    hinweis: String = "",
    zeilen: Int = 1,
) {
    val p = LokalePalette.current
    OutlinedTextField(
        value = wert,
        onValueChange = beiAenderung,
        modifier = modifier.fillMaxWidth(),
        singleLine = zeilen == 1,
        minLines = zeilen,
        shape = RectangleShape,
        placeholder = { Text(hinweis, color = p.fgDim, fontSize = 13.sp) },
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = p.bgInset,
            unfocusedContainerColor = p.bgInset,
            focusedBorderColor = p.accent,
            unfocusedBorderColor = p.border,
            cursorColor = p.accent,
            focusedTextColor = p.fg,
            unfocusedTextColor = p.fg,
        ),
    )
}

/**
 * Punktestand als zwei Balken: accent2 gegen accent, im Monospace-Raster.
 *
 * `vonMensch` / `vonMimik` sind der Stand VOR der gerade gezeigten Runde. Der
 * Balken startet dort und läuft auf den neuen Stand zu; die Zahl daneben zählt
 * mit. Ohne diese Angabe steht er sofort richtig – auf dem Basisbildschirm
 * wäre eine Animation nur Zappeln, in der Auflösung ist sie die Nachricht.
 */
@Composable
fun Punktebalken(
    mensch: Int,
    mimik: Int,
    ziel: Int,
    modifier: Modifier = Modifier,
    vonMensch: Int = mensch,
    vonMimik: Int = mimik,
) {
    val p = LokalePalette.current
    Column(modifier.fillMaxWidth()) {
        Balken("MENSCH", mensch, vonMensch, ziel, p.accent2)
        Balken("MIMIK", mimik, vonMimik, ziel, p.accent)
    }
}

@Composable
private fun Balken(name: String, wert: Int, von: Int, ziel: Int, farbe: Color) {
    val p = LokalePalette.current
    // Der Bildschirm wird mit dem NEUEN Stand frisch aufgebaut. Eine Animation,
    // die beim Kompositionswert beginnt, hätte also nichts zu tun. Deshalb
    // startet der Zustand beim alten Wert, und der Effekt schiebt ihn danach
    // auf den neuen – erst dadurch gibt es eine Strecke zu laufen.
    var bis by remember { mutableFloatStateOf(von.toFloat()) }
    LaunchedEffect(wert) { bis = wert.toFloat() }
    val jetzt by animateFloatAsState(
        targetValue = bis,
        animationSpec = tween(durationMillis = 900, easing = FastOutSlowInEasing),
        label = name,
    )
    Row(
        Modifier.fillMaxWidth().padding(vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(name, color = p.fgDim, fontSize = 10.sp, letterSpacing = 1.1.sp, modifier = Modifier.width(64.dp))
        Box(
            Modifier.weight(1f).height(10.dp).background(p.bgAlt).border(1.dp, p.border),
        ) {
            Box(
                Modifier.fillMaxWidth(if (ziel > 0) (jetzt / ziel).coerceIn(0f, 1f) else 0f)
                    .height(10.dp).background(farbe),
            )
        }
        Text(
            jetzt.roundToInt().toString(), color = farbe, fontSize = 13.sp,
            modifier = Modifier.width(30.dp).padding(start = 8.dp),
        )
    }
}

/**
 * Fortschritt beim Fälschen: fünfzehn Sekunden bis voll, in unregelmäßigen
 * Sprüngen.
 *
 * Ehrlich messen lässt sich hier nichts. Ein Aufruf liegt im Median bei gut
 * zehn Sekunden, kann aber das Doppelte brauchen, wenn die Abstandsprüfung eine
 * Fassung zurückweist – eine echte Restzeit wäre geraten. Also zeigt der Balken
 * eine Erwartung: Er läuft in ungleichen Schritten und ungleichen Abständen auf
 * die fünfzehn Sekunden zu, wie etwas, das arbeitet, und nicht wie eine Uhr.
 * Darunter stehen die tatsächlich verstrichenen Sekunden – wer nachrechnen
 * will, wird nicht belogen.
 *
 * `seit` sind Sekunden seit dem Absenden, vom Server. Nach einem Neustart der
 * App steht der Balken deshalb da, wo er hingehört, statt wieder bei null.
 *
 * `fertig` heißt: Die Karten sind da. Dann geht er zügig auf voll, und der
 * Bildschirm hält so lange – ein Balken, der mitten im Lauf verschwindet,
 * lässt den Moment unfertig aussehen.
 */
@Composable
fun Fortschritt(seit: Int, fertig: Boolean, modifier: Modifier = Modifier) {
    val p = LokalePalette.current
    val start = System.currentTimeMillis() - seit * 1000L
    var anteil by remember { mutableFloatStateOf(0f) }
    var sekunden by remember { mutableIntStateOf(seit) }

    LaunchedEffect(fertig) {
        if (fertig) {
            anteil = 1f
            return@LaunchedEffect
        }
        while (true) {
            val ms = System.currentTimeMillis() - start
            sekunden = (ms / 1000L).toInt()
            // Der gleichmäßige Lauf auf 15 s, versetzt um einen Zufallsschlag.
            // Das Maximum nimmt der Versatz zurück, damit der Balken nicht
            // rückwärts geht: gezeigt wird immer der höchste bisherige Stand.
            val gerade = (ms / 15000f).coerceIn(0f, 1f)
            val versetzt = gerade + Random.nextFloat() * 0.09f - 0.03f
            anteil = maxOf(anteil, versetzt.coerceIn(0f, 1f))
            if (anteil >= 1f) {
                // Voll, aber noch nichts da: nur noch die Sekunden zählen.
                delay(1000)
            } else {
                delay(Random.nextLong(120, 800))
            }
        }
    }

    val gezeigt by animateFloatAsState(
        targetValue = anteil,
        animationSpec = tween(durationMillis = if (fertig) 320 else 260, easing = LinearEasing),
        label = "fortschritt",
    )
    Column(modifier.fillMaxWidth()) {
        Box(Modifier.fillMaxWidth().height(10.dp).background(p.bgAlt).border(1.dp, p.border)) {
            Box(Modifier.fillMaxWidth(gezeigt.coerceIn(0f, 1f)).height(10.dp).background(p.accent))
        }
        Spacer(Modifier.height(5.dp))
        // Feste Breite für die Prozentzahl: Sie steht in der Mitte, und ohne
        // die Breite rückte die ganze Zeile bei jedem Sprung zur Seite.
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
            Text(
                "%3d %%".format((gezeigt * 100).roundToInt()),
                color = p.accent, fontSize = 11.sp,
                textAlign = TextAlign.End, modifier = Modifier.width(42.dp),
            )
            Text(
                if (fertig) " · fertig" else " · MIMIK schreibt · %d s".format(sekunden),
                color = p.fgDim, fontSize = 11.sp,
            )
        }
    }
}

@Composable
fun Zeile(text: String, farbe: Color? = null, groesse: Int = 13) {
    val p = LokalePalette.current
    Text(text, color = farbe ?: p.fg, fontSize = groesse.sp, lineHeight = (groesse * 1.5).sp)
}

@Composable
fun Klickbar(modifier: Modifier = Modifier, beiKlick: () -> Unit, inhalt: @Composable () -> Unit) {
    Box(modifier.clickable(onClick = beiKlick)) { inhalt() }
}
