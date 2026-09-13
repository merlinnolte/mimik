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
     * Der eigene Server, voreingestellt. Mit https, weil er hinter einem Reverse
     * Proxy mit TLS steht – und weil die Release-Fassung Klartext ohnehin
     * verweigert. Zum Testen gegen einen lokalen Server überschreibt man das
     * Feld beim Anmelden, etwa mit http://10.0.2.2:8080 im Emulator.
     */
    var server: String
        get() = p.getString("server", "https://mimik.merlinnolte.de").orEmpty()
        set(v) = p.edit().putString("server", v.trim().trimEnd('/')).apply()

    /**
     * Höchste Rundennummer, deren Auflösung dieser Spieler gesehen hat – und
     * das Match, zu dem sie gehört.
     *
     * Ohne das Match war der Merker eine Falle: Rundennummern beginnen in jedem
     * Match wieder bei 1, also lagen nach dem ersten Match alle Auflösungen
     * unter dem alten Höchststand und wurden übersprungen. Zu sehen erst beim
     * zweiten Spiel, und deshalb lange nicht gesehen.
     */
    var gesehenBis: Int
        get() = p.getInt("gesehen_bis", 0)
        set(v) = p.edit().putInt("gesehen_bis", v).apply()

    var gesehenMatch: String
        get() = p.getString("gesehen_match", "").orEmpty()
        set(v) = p.edit().putString("gesehen_match", v).apply()

    var palette: String
        get() = p.getString("palette", "tokyo-night").orEmpty()
        set(v) = p.edit().putString("palette", v).apply()

    /** Das Intro läuft einmal. Aus den Einstellungen lässt es sich erneut starten. */
    var introGesehen: Boolean
        get() = p.getBoolean("intro_gesehen", false)
        set(v) = p.edit().putBoolean("intro_gesehen", v).apply()

    /**
     * Wofür zuletzt eine Benachrichtigung ausging. Ohne diesen Merker meldet der
     * Hintergrunddienst bei jedem Durchlauf dasselbe erneut.
     */
    var letzteMeldung: String
        get() = p.getString("letzte_meldung", "").orEmpty()
        set(v) = p.edit().putString("letzte_meldung", v).apply()

    /** Nach dem Löschen des Kontos darf hier nichts stehen bleiben. */
    fun leeren() = p.edit().clear().apply()
}
