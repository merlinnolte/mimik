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
        WorkManager.getInstance(kontext).enqueueUniquePeriodicWork(
            AUFTRAG, ExistingPeriodicWorkPolicy.KEEP, auftrag,
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

    fun melden(kontext: Context, titel: String, text: String) {
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
            .setContentTitle(titel)
            .setContentText(text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(text))
            .setContentIntent(oeffnen)
            .setAutoCancel(true)
            .build()
        runCatching { NotificationManagerCompat.from(kontext).notify(1, meldung) }
    }
}

/**
 * Ein Durchlauf: Zustand holen, prüfen, ob etwas ansteht, und nur melden, wenn
 * es etwas anderes ist als beim letzten Mal. Ohne diesen Vergleich meldet jeder
 * Durchlauf dieselbe offene Runde erneut.
 */
class MelderArbeit(kontext: Context, parameter: WorkerParameters) :
    CoroutineWorker(kontext, parameter) {

    override suspend fun doWork(): Result {
        val speicher = Speicher(applicationContext)
        if (speicher.token.isBlank()) return Result.success()

        val z = runCatching { Netz(speicher.server, speicher.token).zustand() }
            .getOrElse { return Result.retry() }

        val (kennung, titel, text) = ansage(z) ?: run {
            speicher.letzteMeldung = ""
            return Result.success()
        }
        if (kennung == speicher.letzteMeldung) return Result.success()
        speicher.letzteMeldung = kennung
        Melder.melden(applicationContext, titel, text)
        return Result.success()
    }

    /**
     * Was ansteht, entscheidet der Server: `dran` ist genau die Liste dessen,
     * was dieser Spieler tun muss. Steht dort etwas, hat die andere Seite
     * gezogen.
     */
    private fun ansage(z: Spielzustand): Triple<String, String, String>? {
        val partner = z.party?.partner?.spitzname?.take(24).orEmpty().ifBlank { "Die andere Seite" }
        val offen = z.dran.firstOrNull()
        if (offen != null) {
            val nummer = z.runden.firstOrNull { it.id == offen.runde }?.nummer ?: 0
            return when (offen.was) {
                "schreiben" -> Triple(
                    "schreiben:${offen.runde}", "MIMIK wartet",
                    "Runde $nummer steht offen. Deine Antwort fehlt noch.",
                )
                "raten" -> Triple(
                    "raten:${offen.runde}", "Vier Karten liegen bereit",
                    "Runde $nummer: Eine davon hat $partner wirklich geschrieben.",
                )
                else -> null
            }
        }
        // Vor dem ersten Match gibt es nichts zu tun – außer dem Moment, in dem
        // die Party endlich zu zweit ist.
        val p = z.party
        if (p?.partner != null && z.match == null) {
            return Triple(
                "party:${p.id}", "Die Party ist vollständig",
                "$partner ist beigetreten. Ihr könnt anfangen.",
            )
        }
        return null
    }
}
