package app.mimik

/**
 * Umzug: dasselbe Konto auf einer Neuinstallation.
 *
 * Nötig geworden durch die Freigabefassung. Die bisher verteilten APKs waren
 * Debug-Bauten, signiert mit dem Standardschlüssel, den jeder Entwicklerrechner
 * hat; die Freigabefassung trägt einen eigenen. Android nimmt ein Update nur bei
 * gleicher Signatur an – der Wechsel geht also nur über Deinstallieren und neu
 * Installieren, und dabei ist der lokale Speicher weg.
 *
 * Weg ist damit aber nur das Gerät, nicht das Konto: Auf dem Server hängt alles
 * am Token, und das ist eine Zeichenkette. Wer sie vor dem Deinstallieren
 * mitnimmt, ist danach derselbe Spieler – mit Party, Punktestand und Dossier.
 *
 * Alles hier ist reine Textarbeit und steht deshalb in UmzugTest.
 */

/**
 * Bringt eine eingetippte Serveradresse in die Form, in der sie gespeichert
 * wird. Null heißt: damit lässt sich nichts anfangen.
 *
 * Ohne Schema wird https angenommen – niemand tippt es, und die
 * Freigabefassung verweigert Klartext ohnehin. Der Schrägstrich am Ende fliegt,
 * weil jeder Pfad im Netz-Klienten mit einem beginnt und zwei daraus einen
 * Fehlschlag machen.
 */
fun serverNormalform(eingabe: String): String? {
    var x = eingabe.trim()
    if (x.isBlank()) return null
    if (!x.contains("://")) x = "https://$x"
    val teile = x.split("://", limit = 2)
    val schema = teile[0].lowercase()
    if (schema != "http" && schema != "https") return null
    val rest = teile[1].trimEnd('/')
    // Ein Wirt ohne Punkt und ohne Doppelpunkt ist kein Wirt, sondern ein
    // Tippfehler. "localhost:8080" und "10.0.2.2:8080" gehen damit durch.
    val wirt = rest.substringBefore('/')
    if (wirt.isBlank() || (!wirt.contains('.') && !wirt.contains(':'))) return null
    return "$schema://$rest"
}

/**
 * Das Token in Achtergruppen, damit man es abschreiben oder vorlesen kann.
 * 64 Zeichen am Stück liest niemand ohne sich zu verzählen.
 */
fun schluesselAnzeige(token: String): String =
    token.chunked(8).joinToString(" ")

/**
 * Liest einen abgeschriebenen oder eingefügten Schlüssel zurück. Leerzeichen,
 * Zeilenumbrüche und Bindestriche fallen weg – wer ihn aus einer Notiz holt,
 * bringt sie mit. Null heißt: das ist kein Schlüssel.
 */
fun schluesselLesen(eingabe: String): String? {
    val x = eingabe.filterNot { it.isWhitespace() || it == '-' }
    if (x.length !in 32..128) return null
    if (!x.all { it.isLetterOrDigit() }) return null
    return x
}
