package app.mimik

import android.os.Bundle
import androidx.activity.ComponentActivity
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
    MimikTheme(modell.palette) {
        Box(Modifier.fillMaxSize()) {
            when (modell.bildschirm) {
                Bildschirm.Start -> StartBildschirm(modell)
                Bildschirm.Laden -> LadeBildschirm(modell)
                Bildschirm.Intro -> IntroBildschirm(modell)
                Bildschirm.Tags -> TagsBildschirm(modell)
                Bildschirm.Party -> PartyBildschirm(modell)
                Bildschirm.Basis -> BasisBildschirm(modell)
                Bildschirm.Schreiben -> SchreibenBildschirm(modell)
                Bildschirm.Warten -> WartenBildschirm(modell)
                Bildschirm.Raten -> RatenBildschirm(modell)
                Bildschirm.Getippt -> GetipptBildschirm(modell)
                Bildschirm.Aufloesung -> AufloesungBildschirm(modell)
                Bildschirm.Einstellungen -> EinstellungenBildschirm(modell)
            }
            ZahnradEcke(modell)
        }
    }
}
