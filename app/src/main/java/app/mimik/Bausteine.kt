package app.mimik

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

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

/** Punktestand als zwei Balken: accent2 gegen accent, im Monospace-Raster. */
@Composable
fun Punktebalken(mensch: Int, mimik: Int, ziel: Int, modifier: Modifier = Modifier) {
    val p = LokalePalette.current
    Column(modifier.fillMaxWidth()) {
        Balken("MENSCH", mensch, ziel, p.accent2)
        Balken("MIMIK", mimik, ziel, p.accent)
    }
}

@Composable
private fun Balken(name: String, wert: Int, ziel: Int, farbe: Color) {
    val p = LokalePalette.current
    Row(
        Modifier.fillMaxWidth().padding(vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(name, color = p.fgDim, fontSize = 10.sp, letterSpacing = 1.1.sp, modifier = Modifier.width(64.dp))
        Box(
            Modifier.weight(1f).height(10.dp).background(p.bgAlt).border(1.dp, p.border),
        ) {
            Box(
                Modifier.fillMaxWidth(if (ziel > 0) (wert.toFloat() / ziel).coerceIn(0f, 1f) else 0f)
                    .height(10.dp).background(farbe),
            )
        }
        Text(
            wert.toString(), color = farbe, fontSize = 13.sp,
            modifier = Modifier.width(30.dp).padding(start = 8.dp),
        )
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
