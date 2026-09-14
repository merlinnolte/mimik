package app.mimik

import android.content.Context

/**
 * Token, Serveradresse, Palette und ein paar Merker, die nur dieses Gerät
 * angehen. Alles Inhaltliche – Tags, Dossier, Spielstand – liegt auf dem Server.
 */
class Speicher(kontext: Context) {
    private val p = kontext.getSharedPreferences("mimik", Context.MODE_PRIVATE)

    var token: String
        get() = p.getString("token", "").orEmpty()
        set(v) = p.edit().putString("token", v).apply()

    /**
     * Der Server, auf dem gespielt wird.
     *
     * Die Vorgabe kommt aus dem Bau: im Debugbau der eigene Server, in der
     * Freigabefassung nichts. Ein öffentliches APK darf niemanden ungefragt auf
     * einen fremden Server schicken – wer es installiert, trägt seinen eigenen
     * ein. Zum Testen gegen einen lokalen Server tippt man http://10.0.2.2:8080.
     */
    var server: String
        get() = p.getString("server", BuildConfig.VORGABE_SERVER).orEmpty()
        set(v) = p.edit().putString("server", v.trim().trimEnd('/')).apply()

    /** Wann zuletzt nach einer neueren Fassung gefragt wurde. */
    var fassungGesucht: Long
        get() = p.getLong("fassung_gesucht", 0L)
        set(v) = p.edit().putLong("fassung_gesucht", v).apply()

    /** Welche angebotene Fassung weggeklickt wurde. Sie fragt nicht zweimal. */
    var fassungUebergangen: String
        get() = p.getString("fassung_uebergangen", "").orEmpty()
        set(v) = p.edit().putString("fassung_uebergangen", v).apply()

    /** Welche Partie zuletzt offen war. Leer heißt: die Lobby. */
    var offenePartie: String
        get() = p.getString("offene_partie", "").orEmpty()
        set(v) = p.edit().putString("offene_partie", v).apply()

    /**
     * Bis wohin die Auflösungen je Partie gesehen sind, als JSON-Karte
     * partie -> "matchID:nummer".
     *
     * Vorher war das EIN Zahlenpaar für alles. Zweimal ist daran etwas kaputt
     * gegangen: Rundennummern fangen in jedem Match wieder bei 1 an (deshalb
     * die Match-ID), und mit mehreren Partien verschluckte die eine die
     * Auflösungen der anderen (deshalb die Karte).
     */
    var gesehen: Map<String, Gesehen>
        get() = karte("gesehen").mapValues { (_, v) ->
            val t = v.split(":", limit = 2)
            Gesehen(t.getOrElse(0) { "" }, t.getOrElse(1) { "0" }.toIntOrNull() ?: 0)
        }
        set(v) = karteSetzen("gesehen", v.mapValues { (_, g) -> "${g.match}:${g.bis}" })

    /** Bis zu welcher Runde je Partie der Klonblick weggeklickt ist. */
    var klone: Map<String, Gesehen>
        get() = karte("klone").mapValues { (_, v) ->
            val t = v.split(":", limit = 2)
            Gesehen(t.getOrElse(0) { "" }, t.getOrElse(1) { "0" }.toIntOrNull() ?: 0)
        }
        set(v) = karteSetzen("klone", v.mapValues { (_, g) -> "${g.match}:${g.bis}" })

    /** Wofür je Partie zuletzt gemeldet wurde. */
    var gemeldet: Map<String, String>
        get() = karte("gemeldet")
        set(v) = karteSetzen("gemeldet", v)

    /** Feste Nummern für die Benachrichtigungen, je Partie eine. */
    var meldungsNummern: Map<String, String>
        get() = karte("meldungsnummern")
        set(v) = karteSetzen("meldungsnummern", v)

    /**
     * Wann die App zuletzt sichtbar war.
     *
     * Ein Ja/Nein-Merker wäre eine Falle: Stirbt der Prozess, während er auf
     * „ja" steht, kommt nie wieder eine Meldung, und niemand fände je den
     * Grund. Ein Zeitstempel verfällt von selbst.
     */
    var gesehenStempel: Long
        get() = p.getLong("gesehen_stempel", 0L)
        set(v) = p.edit().putLong("gesehen_stempel", v).apply()

    fun imVordergrund(jetzt: Long = System.currentTimeMillis()): Boolean =
        jetzt - gesehenStempel < 60_000

    // Kleine Karten als eine Zeichenkette: Es sind nie mehr als eine Handvoll
    // Partien, und SharedPreferences kann keine verschachtelten Werte. Kaputter
    // Text liefert eine leere Karte, statt zu werfen - schlimmstenfalls sieht
    // man eine Auflösung ein zweites Mal.
    private fun karte(schluessel: String): Map<String, String> =
        p.getString(schluessel, "").orEmpty().split("\u0001").mapNotNull {
            val t = it.split("\u0002", limit = 2)
            if (t.size == 2 && t[0].isNotBlank()) t[0] to t[1] else null
        }.toMap()

    private fun karteSetzen(schluessel: String, v: Map<String, String>) =
        p.edit().putString(
            schluessel,
            v.entries.joinToString("\u0001") { "${it.key}\u0002${it.value}" },
        ).apply()

    var palette: String
        get() = p.getString("palette", "tokyo-night").orEmpty()
        set(v) = p.edit().putString("palette", v).apply()

    /** Das Intro läuft einmal. Aus den Einstellungen lässt es sich erneut starten. */
    var introGesehen: Boolean
        get() = p.getBoolean("intro_gesehen", false)
        set(v) = p.edit().putBoolean("intro_gesehen", v).apply()

    /** Nach dem Löschen des Kontos darf hier nichts stehen bleiben. */
    fun leeren() = p.edit().clear().apply()

    /**
     * Zieht alte Speicherstände auf den heutigen Stand nach. Läuft bei jedem
     * Start, tut aber nur dann etwas, wenn wirklich etwas fehlt.
     *
     * Bisher stand die Serveradresse als Vorgabe im Code und wurde nur dann
     * geschrieben, wenn jemand sie von Hand änderte. Mit der Freigabefassung
     * ist die Vorgabe leer – und wer angemeldet ist, ohne je eine Adresse
     * eingetragen zu haben, stünde ohne diese Zeilen nach dem Update vor einem
     * leeren Feld. Das Token wäre noch da, aber es gäbe niemanden mehr, dem man
     * es zeigt: ausgeloggt, ohne dass es jemand so genannt hätte.
     */
    fun wandern() {
        if (p.getString("server", null) == null && token.isNotBlank()) {
            p.edit().putString("server", ALTE_VORGABE).apply()
        }
    }

    private companion object {
        /** Die Adresse, die bis 0.9 im Code stand. Nur für wandern(). */
        const val ALTE_VORGABE = "https://mimik.merlinnolte.de"
    }
}
