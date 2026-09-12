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

    var server: String
        get() = p.getString("server", "http://10.0.2.2:8080").orEmpty()
        set(v) = p.edit().putString("server", v.trim().trimEnd('/')).apply()

    /** Höchste Rundennummer, deren Auflösung dieser Spieler gesehen hat. */
    var gesehenBis: Int
        get() = p.getInt("gesehen_bis", 0)
        set(v) = p.edit().putInt("gesehen_bis", v).apply()

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
