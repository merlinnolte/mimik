package app.mimik

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import java.util.concurrent.TimeUnit

/**
 * Benachrichtigungen ohne Push-Dienst: Die App fragt im Hintergrund selbst nach.
 *
 * Kein Firebase, kein Google-Konto, keine dritte Partei zwischen dem Server und
 * den zwei Telefonen – der Server bleibt das einzige, was von außen erreichbar
 * sein muss. Der Preis ist die Latenz: WorkManager lässt einen wiederkehrenden
 * Auftrag höchstens alle 15 Minuten laufen, und im Dösen des Geräts auch
 * seltener. Für ein Spiel, dessen ganze Idee Zeitunabhängigkeit ist, ist das
 * der richtige Tausch.
 */
object Melder {

    private const val AUFTRAG = "mimik-nachsehen"
    private const val KANAL = "zuege"

    fun planen(kontext: Context) {
        val auftrag = PeriodicWorkRequestBuilder<MelderArbeit>(15, TimeUnit.MINUTES)
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .build()
        // UPDATE statt KEEP: Mit KEEP behielt ein Geraet, das schon spielt, fuer
        // immer den Auftrag, mit dem es einmal angefangen hat - eine Aenderung
        // am Takt oder an den Bedingungen erreichte genau die Geraete nie, auf
        // die es ankommt.
        WorkManager.getInstance(kontext).enqueueUniquePeriodicWork(
            AUFTRAG, ExistingPeriodicWorkPolicy.UPDATE, auftrag,
        )
    }

    fun abbestellen(kontext: Context) {
        WorkManager.getInstance(kontext).cancelUniqueWork(AUFTRAG)
        NotificationManagerCompat.from(kontext).cancelAll()
    }

    fun darfMelden(kontext: Context): Boolean =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(kontext, Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED

    fun kanalAnlegen(kontext: Context) {
        val kanal = NotificationChannel(
            KANAL, "Züge", NotificationManager.IMPORTANCE_DEFAULT,
        ).apply { description = "Wenn die andere Seite gezogen hat." }
        kontext.getSystemService(NotificationManager::class.java).createNotificationChannel(kanal)
    }

    /**
     * Eine feste Nummer ging, solange es eine Partie gab. Bei mehreren wuerde
     * jede Meldung die vorige ueberschreiben: Man saehe immer nur die zuletzt
     * eingetroffene und wuesste nicht, dass noch eine zweite aussteht.
     */
    private fun nummer(kontext: Context, partie: String): Int {
        val s = Speicher(kontext)
        val karte = s.meldungsNummern
        karte[partie]?.toIntOrNull()?.let { return it }
        val neu = (karte.values.mapNotNull { it.toIntOrNull() }.maxOrNull() ?: 100) + 1
        s.meldungsNummern = karte + (partie to neu.toString())
        return neu
    }

    fun wegnehmen(kontext: Context, partie: String) {
        runCatching { NotificationManagerCompat.from(kontext).cancel(nummer(kontext, partie)) }
    }

    fun melden(kontext: Context, m: Meldung) {
        if (!darfMelden(kontext)) return
        kanalAnlegen(kontext)
        val oeffnen = PendingIntent.getActivity(
            kontext, 0,
            Intent(kontext, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val meldung = NotificationCompat.Builder(kontext, KANAL)
            .setSmallIcon(R.drawable.mimik_melder)
            .setContentTitle(m.titel)
            .setContentText(m.text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(m.text))
            .setContentIntent(oeffnen)
            .setGroup(KANAL)
            .setAutoCancel(true)
            .build()
        runCatching {
            NotificationManagerCompat.from(kontext).notify(nummer(kontext, m.partie), meldung)
        }
    }
}

/**
 * Ein Durchlauf: nachsehen, ob etwas ansteht - und nur dann melden.
 *
 * Zwei Dinge waren hier kaputt, beide im Betrieb aufgefallen:
 *
 * 1. Es gab keine Vordergrundpruefung. Wer gerade auf dem Bildschirm sass,
 *    bekam trotzdem einen Zettel.
 * 2. Meldungen bezogen sich auf eine Phase, die schon vorbei war. Zwei
 *    Ursachen: Der Abruf darf zehn Sekunden auf die Verbindung warten, und in
 *    dieser Zeit kann der Zug laengst gemacht sein - deshalb wird VOR und NACH
 *    dem Abruf geprueft. Und eine einmal gestellte Meldung blieb im Schacht
 *    stehen, weil setAutoCancel nur beim Antippen abraeumt - deshalb die
 *    Loeschliste aus dem Meldeplan.
 */
class MelderArbeit(kontext: Context, parameter: WorkerParameters) :
    CoroutineWorker(kontext, parameter) {

    override suspend fun doWork(): Result {
        val speicher = Speicher(applicationContext)
        if (speicher.token.isBlank()) return Result.success()
        // Wer gerade hinschaut, braucht keinen Zettel.
        if (speicher.imVordergrund()) return Result.success()

        val l = runCatching { Netz(speicher.server, speicher.token).lobby() }
            .getOrElse { return Result.retry() }
        if (speicher.imVordergrund()) return Result.success()

        val plan = meldeplan(l, speicher.gemeldet)
        plan.loeschen.forEach { Melder.wegnehmen(applicationContext, it) }
        plan.zeigen.forEach { Melder.melden(applicationContext, it) }
        speicher.gemeldet = plan.merker
        return Result.success()
    }
}
