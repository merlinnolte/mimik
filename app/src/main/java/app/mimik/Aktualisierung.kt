package app.mimik

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.net.HttpURLConnection
import java.net.URL

/**
 * Die App holt ihre eigenen Updates von der Releaseseite auf GitHub.
 *
 * Kein Store dazwischen: MIMIK ist eine App für zwei Leute, die sich kennen,
 * und für die Verteilung über einen Store gibt es hier niemanden. Was es dafür
 * gibt, ist ein Repository mit Releases – und ein Release trägt eine Fassung
 * und ein APK. Mehr braucht ein Update nicht.
 *
 * Die entscheidenden Teile stehen als reine Funktionen hier oben, damit sie in
 * AktualisierungTest ohne Gerät prüfbar sind. Der Rest ist Android: laden,
 * ablegen, dem System hinhalten.
 */

/** Was von einem Release übrig bleibt, wenn es tatsächlich neuer ist. */
data class Fassung(
    val name: String,
    val notiz: String,
    val apk: String,
    val groesse: Long,
)

@Serializable
private data class ReleaseAus(
    @SerialName("tag_name") val tag: String = "",
    val name: String = "",
    val body: String = "",
    val draft: Boolean = false,
    val prerelease: Boolean = false,
    val assets: List<AssetAus> = emptyList(),
)

@Serializable
private data class AssetAus(
    val name: String = "",
    @SerialName("browser_download_url") val url: String = "",
    val size: Long = 0,
)

private val json = Json { ignoreUnknownKeys = true }

/**
 * Vergleicht zwei Fassungsnamen zahlenweise, nicht als Text.
 *
 * Ein Textvergleich hätte "0.10" für älter als "0.9" gehalten – und genau
 * dieser Sprung steht als nächster an. Was keine Zahl ist, trennt: aus "v1.2-rc"
 * werden 1 und 2. Fehlende Stellen zählen als 0, damit "1.2" und "1.2.0"
 * dasselbe sind.
 */
fun istNeuer(vorhanden: String, angeboten: String): Boolean {
    fun teile(x: String) = x.split(Regex("[^0-9]+")).filter { it.isNotBlank() }.map { it.toLong() }
    val a = teile(vorhanden)
    val b = teile(angeboten)
    if (b.isEmpty()) return false
    for (i in 0 until maxOf(a.size, b.size)) {
        val links = a.getOrElse(i) { 0 }
        val rechts = b.getOrElse(i) { 0 }
        if (rechts != links) return rechts > links
    }
    return false
}

/**
 * Liest die Antwort von GitHub und sagt, ob sie ein Update ist.
 *
 * Null heißt in jedem Zweifelsfall: nichts zu tun. Ein Entwurf, eine
 * Vorabfassung, ein Release ohne APK oder eine Fassung, die nicht neuer ist,
 * sind kein Update – und kaputtes JSON auch nicht.
 */
fun fassungAusRelease(text: String, vorhanden: String): Fassung? {
    val r = runCatching { json.decodeFromString<ReleaseAus>(text) }.getOrNull() ?: return null
    if (r.draft || r.prerelease) return null
    val name = r.tag.trim().removePrefix("v").ifBlank { r.name.trim() }
    if (!istNeuer(vorhanden, name)) return null
    val apk = apkAsset(r.assets) ?: return null
    return Fassung(name = name, notiz = r.body.trim(), apk = apk.url, groesse = apk.size)
}

/**
 * Welches Anhängsel das APK ist.
 *
 * Ein Release kann mehrere tragen – künftig vielleicht ein Debug-APK daneben.
 * Deshalb zuerst das mit "release" im Namen, erst danach irgendeines.
 */
private fun apkAsset(assets: List<AssetAus>): AssetAus? {
    val apks = assets.filter { it.name.endsWith(".apk", true) && it.url.isNotBlank() }
    return apks.firstOrNull { it.name.contains("release", true) } ?: apks.firstOrNull()
}

object Aktualisierung {

    /** Wie oft von selbst nachgefragt wird. Ein Update hat keine Eile. */
    const val ABSTAND = 6 * 60 * 60 * 1000L

    private const val ZEITGRENZE = 15_000

    /**
     * Fragt GitHub nach dem neuesten Release. Wirft nicht: Ein Update, das
     * nicht zu erreichen ist, darf nichts stören – der Spielbetrieb hängt
     * nicht daran.
     */
    fun suchen(vorhanden: String, quelle: String = BuildConfig.QUELLE): Fassung? = runCatching {
        val verb = URL("https://api.github.com/repos/$quelle/releases/latest")
            .openConnection() as HttpURLConnection
        verb.connectTimeout = ZEITGRENZE
        verb.readTimeout = ZEITGRENZE
        verb.setRequestProperty("Accept", "application/vnd.github+json")
        val text = if (verb.responseCode in 200..299) {
            verb.inputStream.bufferedReader().use { it.readText() }
        } else {
            ""
        }
        verb.disconnect()
        fassungAusRelease(text, vorhanden)
    }.getOrNull()

    /** Wohin geladene APKs kommen: in den Cache, denn nach dem Einspielen sind
     *  sie Müll, und das System darf sie jederzeit wegräumen. */
    private fun ablage(kontext: Context): File =
        File(kontext.cacheDir, "fassungen").apply { mkdirs() }

    /**
     * Lädt das APK und meldet den Fortschritt in Prozent.
     *
     * Vorher wird die Ablage geleert: Zwei halbe Downloads nebeneinander
     * nützen niemandem, und ein abgebrochener bliebe sonst für immer liegen.
     */
    fun herunterladen(kontext: Context, f: Fassung, beiFortschritt: (Int) -> Unit): File {
        val ordner = ablage(kontext)
        ordner.listFiles()?.forEach { it.delete() }
        val ziel = File(ordner, "mimik-${f.name}.apk")
        val verb = URL(f.apk).openConnection() as HttpURLConnection
        verb.connectTimeout = ZEITGRENZE
        verb.readTimeout = 60_000
        verb.instanceFollowRedirects = true
        try {
            if (verb.responseCode !in 200..299) throw NetzFehler(verb.responseCode, "Download")
            val ganz = if (f.groesse > 0) f.groesse else verb.contentLengthLong
            var bisher = 0L
            verb.inputStream.use { ein ->
                ziel.outputStream().use { aus ->
                    val puffer = ByteArray(64 * 1024)
                    while (true) {
                        val n = ein.read(puffer)
                        if (n <= 0) break
                        aus.write(puffer, 0, n)
                        bisher += n
                        if (ganz > 0) beiFortschritt(((bisher * 100) / ganz).toInt().coerceIn(0, 100))
                    }
                }
            }
        } finally {
            verb.disconnect()
        }
        return ziel
    }

    /**
     * Ob das System diese App überhaupt installieren lässt. Seit Android 8
     * hängt „unbekannte Quellen" an der einzelnen App, nicht mehr am Gerät.
     */
    fun darfInstallieren(kontext: Context): Boolean =
        kontext.packageManager.canRequestPackageInstalls()

    /** Führt zur Systemseite, auf der genau diese Erlaubnis liegt. */
    fun erlaubnisHolen(kontext: Context) {
        kontext.startActivity(
            Intent(
                Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                Uri.parse("package:" + kontext.packageName),
            ).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
        )
    }

    /**
     * Übergibt das APK dem Installer des Systems. Den letzten Schritt tut der
     * Mensch – eine App, die sich selbst still austauscht, gibt es hier nicht.
     */
    fun installieren(kontext: Context, datei: File) {
        val adresse = FileProvider.getUriForFile(kontext, kontext.packageName + ".fassungen", datei)
        kontext.startActivity(
            Intent(Intent.ACTION_VIEW)
                .setDataAndType(adresse, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK),
        )
    }
}
