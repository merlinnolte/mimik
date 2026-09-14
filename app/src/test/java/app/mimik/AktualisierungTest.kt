package app.mimik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Prüfungen der Updatelogik – ohne Gerät, ohne Netz.
 *
 * Geprüft wird das, was still falsch sein kann: der Vergleich zweier Fassungen
 * und die Frage, was aus einer Antwort von GitHub überhaupt ein Update macht.
 * Das Laden und das Installieren stehen bewusst daneben; sie sind Android.
 */
class AktualisierungTest {

    /**
     * Der Fall, der diesen Vergleich überhaupt nötig macht. Als Text wäre
     * "0.10" kleiner als "0.9" – und genau dieser Sprung steht an.
     */
    @Test
    fun zehnIstGroesserAlsNeun() {
        assertTrue(istNeuer("0.9", "0.10"))
        assertFalse(istNeuer("0.10", "0.9"))
    }

    @Test
    fun gleichIstKeinUpdate() {
        assertFalse(istNeuer("0.10", "0.10"))
        assertFalse(istNeuer("1.2", "1.2.0"))
    }

    @Test
    fun fehlendeStellenZaehlenAlsNull() {
        assertTrue(istNeuer("1.2", "1.2.1"))
        assertFalse(istNeuer("1.2.1", "1.2"))
    }

    @Test
    fun ohneZahlKeinUpdate() {
        assertFalse(istNeuer("0.9", ""))
        assertFalse(istNeuer("0.9", "irgendwas"))
    }

    private fun release(
        tag: String = "v0.11",
        draft: Boolean = false,
        vorab: Boolean = false,
        assets: String = """[{"name":"mimik-0.11-release.apk","browser_download_url":"https://x/a.apk","size":42}]""",
    ) = """
        {"tag_name":"$tag","name":"MIMIK $tag","body":"Zwei Dinge.\nUnd noch eins.",
         "draft":$draft,"prerelease":$vorab,"assets":$assets}
    """.trimIndent()

    @Test
    fun neuesReleaseMitApk() {
        val f = fassungAusRelease(release(), "0.10")
        assertEquals("0.11", f?.name)
        assertEquals("https://x/a.apk", f?.apk)
        assertEquals(42L, f?.groesse)
        assertTrue(f!!.notiz.startsWith("Zwei Dinge."))
    }

    /** Das "v" am Tag ist Gewohnheit, nicht Teil der Fassung. */
    @Test
    fun tagOhneVWirdAuchGelesen() {
        assertEquals("0.11", fassungAusRelease(release(tag = "0.11"), "0.10")?.name)
    }

    @Test
    fun entwurfUndVorabsindKeinUpdate() {
        assertNull(fassungAusRelease(release(draft = true), "0.10"))
        assertNull(fassungAusRelease(release(vorab = true), "0.10"))
    }

    @Test
    fun aeltereOderGleicheFassungIstKeinUpdate() {
        assertNull(fassungAusRelease(release(tag = "v0.9"), "0.10"))
        assertNull(fassungAusRelease(release(tag = "v0.10"), "0.10"))
    }

    /** Ein Release ohne APK ist eine Ankündigung, kein Update. */
    @Test
    fun ohneApkKeinUpdate() {
        assertNull(fassungAusRelease(release(assets = "[]"), "0.10"))
        assertNull(
            fassungAusRelease(
                release(assets = """[{"name":"quellen.zip","browser_download_url":"https://x/q.zip","size":1}]"""),
                "0.10",
            ),
        )
    }

    /** Liegen mehrere APKs bei, gilt das Freigabe-APK. */
    @Test
    fun freigabeApkSchlaegtDebugApk() {
        val zwei = """[
            {"name":"mimik-0.11-debug.apk","browser_download_url":"https://x/d.apk","size":1},
            {"name":"mimik-0.11-release.apk","browser_download_url":"https://x/r.apk","size":2}
        ]"""
        assertEquals("https://x/r.apk", fassungAusRelease(release(assets = zwei), "0.10")?.apk)
    }

    /** Kaputte Antwort heißt: nichts zu tun, nicht: Absturz. */
    @Test
    fun unsinnFuehrtZuNichts() {
        assertNull(fassungAusRelease("", "0.10"))
        assertNull(fassungAusRelease("<html>404</html>", "0.10"))
    }
}
