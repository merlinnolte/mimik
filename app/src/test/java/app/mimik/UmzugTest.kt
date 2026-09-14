package app.mimik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Prüfungen für das, was zwischen Gerät und Server steht: die Adresse und der
 * Schlüssel. Beides ist Text, den ein Mensch eintippt – und beides führt bei
 * einem Zeichen zu viel in einen Fehler, der wie ein Netzproblem aussieht.
 */
class UmzugTest {

    @Test
    fun ohneSchemaWirdHttpsAngenommen() {
        assertEquals("https://mimik.beispiel.de", serverNormalform("mimik.beispiel.de"))
    }

    @Test
    fun schraegstrichUndLeerzeichenFallenWeg() {
        assertEquals("https://x.de", serverNormalform("  https://x.de/  "))
    }

    /**
     * Der Testserver im Emulator. Klartext geht durch, wenn jemand ihn tippt –
     * aber nur dann: Ohne Schema bleibt es https, sonst würde ein Vertippen
     * die Verschlüsselung abschalten, ohne dass jemand es gesagt hätte.
     */
    @Test
    fun emulatorAdresseGehtDurch() {
        assertEquals("http://10.0.2.2:8080", serverNormalform("HTTP://10.0.2.2:8080"))
        assertEquals("https://localhost:8080", serverNormalform("localhost:8080"))
    }

    @Test
    fun keinWirtKeineAdresse() {
        assertNull(serverNormalform(""))
        assertNull(serverNormalform("   "))
        assertNull(serverNormalform("hallo"))
        assertNull(serverNormalform("https://"))
    }

    @Test
    fun fremdesSchemaWirdAbgelehnt() {
        assertNull(serverNormalform("ftp://x.de"))
    }

    private val token = "a".repeat(32) + "b".repeat(32)

    @Test
    fun schluesselStehtInAchtergruppen() {
        val gezeigt = schluesselAnzeige(token)
        assertEquals(8, gezeigt.split(" ").size)
        assertEquals(token, schluesselLesen(gezeigt))
    }

    /** Abgeschrieben heißt: mit Leerzeichen, Umbrüchen und Bindestrichen. */
    @Test
    fun abgeschriebenerSchluesselWirdGelesen() {
        assertEquals(token, schluesselLesen(" ${token.chunked(4).joinToString("-\n")} "))
    }

    @Test
    fun kurzesOderKaputtesGiltNicht() {
        assertNull(schluesselLesen(""))
        assertNull(schluesselLesen("abc"))
        assertNull(schluesselLesen(token.dropLast(40)))
        assertNull(schluesselLesen(token.dropLast(1) + "!"))
    }
}
