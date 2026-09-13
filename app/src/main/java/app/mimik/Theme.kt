package app.mimik

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ProvidableCompositionLocal
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.LineBreak
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * Die vier Paletten aus src/ui/theme/themes.css des DungeonMaster-Projekts,
 * unverändert übernommen. Rollennamen bleiben gleich, damit man zwischen
 * Web-Projekt und App nicht umdenken muss.
 */
data class Palette(
    val id: String,
    val name: String,
    val bg: Color,
    val bgAlt: Color,
    val bgInset: Color,
    val fg: Color,
    val fgDim: Color,
    val accent: Color,
    val accent2: Color,
    val border: Color,
    val success: Color,
    val warn: Color,
    val error: Color,
    val selection: Color,
)

private fun farbe(hex: String) = Color(android.graphics.Color.parseColor(hex))

val PALETTEN = listOf(
    Palette(
        "tokyo-night", "Tokyo Night",
        farbe("#1a1b26"), farbe("#24283b"), farbe("#16161e"), farbe("#c0caf5"), farbe("#565f89"),
        farbe("#7aa2f7"), farbe("#bb9af7"), farbe("#414868"),
        farbe("#9ece6a"), farbe("#e0af68"), farbe("#f7768e"), farbe("#33467c"),
    ),
    Palette(
        "gruvbox", "Gruvbox",
        farbe("#282828"), farbe("#3c3836"), farbe("#1d2021"), farbe("#ebdbb2"), farbe("#928374"),
        farbe("#83a598"), farbe("#d3869b"), farbe("#504945"),
        farbe("#b8bb26"), farbe("#fabd2f"), farbe("#fb4934"), farbe("#504945"),
    ),
    Palette(
        "nord", "Nord",
        farbe("#2e3440"), farbe("#3b4252"), farbe("#272c36"), farbe("#d8dee9"), farbe("#616e88"),
        farbe("#88c0d0"), farbe("#b48ead"), farbe("#434c5e"),
        farbe("#a3be8c"), farbe("#ebcb8b"), farbe("#bf616a"), farbe("#434c5e"),
    ),
    Palette(
        "catppuccin", "Catppuccin",
        farbe("#1e1e2e"), farbe("#313244"), farbe("#181825"), farbe("#cdd6f4"), farbe("#6c7086"),
        farbe("#89b4fa"), farbe("#cba6f7"), farbe("#45475a"),
        farbe("#a6e3a1"), farbe("#f9e2af"), farbe("#f38ba8"), farbe("#45475a"),
    ),
)

fun paletteMit(id: String): Palette = PALETTEN.firstOrNull { it.id == id } ?: PALETTEN[0]

val LokalePalette: ProvidableCompositionLocal<Palette> = compositionLocalOf { PALETTEN[0] }

/**
 * JetBrains Mono, gebündelt – wie im DungeonMaster-Projekt. Die System-Monospace
 * von Android reicht nicht: Ihr fehlen die Block- und Rahmenzeichen, sie fällt
 * dafür auf eine andere Schrift mit anderen Metriken zurück, und MIMIKs Gesicht
 * zerfällt. Erst mit einer Schrift, die U+2500–U+259F vollständig abdeckt, sitzt
 * die Zeichenmatrix im Raster.
 *
 * 14sp, Zeilenhöhe 1.5 – dieselben Werte wie app.css. Auch Antworttexte laufen
 * im Monospace: sonst fiele eine Karte schon durch den Satz aus der Reihe.
 */
val MonoSchrift = FontFamily(
    Font(R.font.jetbrains_mono_regular, FontWeight.Normal),
    Font(R.font.jetbrains_mono_bold, FontWeight.Bold),
)

private val mono = MonoSchrift

/**
 * Ausgeglichener Umbruch statt "gierig".
 *
 * Der gierige Umbruch fuellt die erste Zeile bis zum Rand und laesst den Rest
 * als Stummel stehen. Bei zweizeiligen Saetzen - und aus denen besteht diese
 * App fast nur - sieht das aus wie ein Fehler. Balanced verteilt gleichmaessig.
 *
 * Es gilt je Absatz: Ein hartes \n teilt den Text, und beide Haelften werden
 * getrennt ausgeglichen. Deshalb sind die beiden handgesetzten Umbrueche in
 * Screens.kt weg - sie standen da, um genau diesen Mangel von Hand zu
 * umgehen, und wuerden ihn jetzt wieder herstellen.
 */
private val ausgeglichen = LineBreak.Paragraph.copy(strategy = LineBreak.Strategy.Balanced)

private val typo = Typography(
    bodyLarge = TextStyle(
        fontFamily = mono, fontSize = 14.sp, lineHeight = 21.sp, lineBreak = ausgeglichen,
    ),
    bodyMedium = TextStyle(
        fontFamily = mono, fontSize = 13.sp, lineHeight = 20.sp, lineBreak = ausgeglichen,
    ),
    bodySmall = TextStyle(
        fontFamily = mono, fontSize = 11.sp, lineHeight = 16.sp, lineBreak = ausgeglichen,
    ),
    titleMedium = TextStyle(
        fontFamily = mono, fontSize = 15.sp, fontWeight = FontWeight.Bold,
        lineBreak = ausgeglichen,
    ),
    labelSmall = TextStyle(
        fontFamily = mono, fontSize = 11.sp, fontWeight = FontWeight.Normal, letterSpacing = 1.2.sp,
    ),
    // Material3 zieht die Knopfbeschriftung aus labelLarge. Ohne diese Zeile
    // läuft sie in Roboto und fällt aus dem Monospace-Raster – der einzige
    // Proportionalsatz, der sonst in der App übrig bleibt.
    labelLarge = TextStyle(
        fontFamily = mono, fontSize = 13.sp, fontWeight = FontWeight.Medium, letterSpacing = 0.4.sp,
    ),
    titleLarge = TextStyle(
        fontFamily = mono, fontSize = 20.sp, fontWeight = FontWeight.Bold,
        lineBreak = ausgeglichen,
    ),
    headlineSmall = TextStyle(
        fontFamily = mono, fontSize = 18.sp, fontWeight = FontWeight.Bold,
        lineBreak = ausgeglichen,
    ),
)

@Composable
fun MimikTheme(palette: Palette, inhalt: @Composable () -> Unit) {
    @Suppress("UNUSED_EXPRESSION") isSystemInDarkTheme() // Paletten sind alle dunkel, bewusst
    CompositionLocalProvider(LokalePalette provides palette) {
        MaterialTheme(
            colorScheme = darkColorScheme(
                primary = palette.accent,
                onPrimary = palette.bg,
                secondary = palette.accent2,
                background = palette.bg,
                onBackground = palette.fg,
                surface = palette.bgAlt,
                onSurface = palette.fg,
                error = palette.error,
                outline = palette.border,
            ),
            typography = typo,
            content = inhalt,
        )
    }
}
