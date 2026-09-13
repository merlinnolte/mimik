package app.mimik

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.viewmodel.compose.viewModel

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Melder.kanalAnlegen(this)
        setContent { App() }
    }

    // Der Zeitstempel ist die ganze Vordergrunderkennung: Der Melder fragt ihn,
    // bevor er jemandem einen Zettel schreibt. Ein Ja/Nein-Merker waere eine
    // Falle - stirbt der Prozess auf "ja", kaeme nie wieder eine Meldung.
    override fun onResume() {
        super.onResume()
        Speicher(this).gesehenStempel = System.currentTimeMillis()
    }

    override fun onStop() {
        super.onStop()
        Speicher(this).gesehenStempel = 0L
    }
}

@Composable
private fun App() {
    val modell: AppModel = viewModel()
    val kontext = LocalContext.current
    LaunchedEffect(Unit) {
        if (modell.angemeldet) {
            modell.aktualisieren()
            // Auch für Geräte, die sich vor dieser Fassung angemeldet haben:
            // enqueueUniquePeriodicWork mit KEEP legt nichts doppelt an.
            Melder.planen(kontext)
        }
        modell.beobachten()
    }
    // Der Systemzurueck fuehrt dorthin, wo man herkam: aus den Einstellungen in
    // den Bildschirm, aus einer Partie in die Uebersicht.
    BackHandler(enabled = modell.einstellungenOffen || modell.offenePartie != null) {
        if (modell.einstellungenOffen) modell.einstellungen(false) else modell.zurueckZurLobby()
    }
    // Sichtbarkeit fuer den Takt: Im Hintergrund fragt die App gar nicht nach.
    androidx.compose.runtime.DisposableEffect(Unit) {
        modell.sichtbarkeit(true)
        onDispose { modell.sichtbarkeit(false) }
    }
    MimikTheme(modell.palette) {
        Box(Modifier.fillMaxSize()) {
            when (modell.bildschirm) {
                Bildschirm.Start -> StartBildschirm(modell)
                Bildschirm.Laden -> LadeBildschirm(modell)
                Bildschirm.NameWaehlen -> NameBildschirm(modell)
                Bildschirm.Intro -> IntroBildschirm(modell)
                Bildschirm.Tags -> TagsBildschirm(modell)
                Bildschirm.Lobby -> LobbyBildschirm(modell)
                Bildschirm.Warteraum -> WarteraumBildschirm(modell)
                Bildschirm.Basis -> BasisBildschirm(modell)
                Bildschirm.Schreiben -> SchreibenBildschirm(modell)
                Bildschirm.Warten -> WartenBildschirm(modell)
                Bildschirm.Raten -> RatenBildschirm(modell)
                Bildschirm.Getippt -> GetipptBildschirm(modell)
                Bildschirm.Aufloesung -> AufloesungBildschirm(modell)
                Bildschirm.Einstellungen -> EinstellungenBildschirm(modell)
            }
            ZahnradEcke(modell)
            ZurueckEcke(modell)
        }
    }
}
