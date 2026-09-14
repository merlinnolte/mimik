package app.mimik

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Prüfungen der Ableitungsschicht – ohne Emulator, ohne Android.
 *
 * Dass das geht, ist der Grund, warum Ableitung.kt überhaupt eine eigene Datei
 * ist: Die Entscheidungen der App stehen dort, und hier stehen die Fälle, in
 * denen sie schon einmal falsch waren.
 */
class AbleitungTest {

    private fun lobby(
        vararg partien: LobbyPartie,
        tags: Int = 10,
        eindeutig: Boolean = true,
    ) = LobbyAus(
        spieler = Spieler("ich", "Merlin"),
        tags = List(tags) { "tag$it" },
        name = NameStand(eindeutig, eindeutig),
        partien = partien.toList(),
    )

    private fun partie(
        id: String = "p1",
        dran: String = "warten",
        partner: String? = "Robin",
    ) = LobbyPartie(
        partyId = id,
        partner = partner?.let { Spieler("g-$id", it) },
        dran = dran,
        runde = "r-$id",
        ziel = 10,
    )

    private fun zustand(
        id: String = "p1",
        partner: String? = "Robin",
        match: MatchAus? = MatchAus("m1", Stand(0, 0), 10, "OFFEN"),
        runden: List<RundeAus> = emptyList(),
    ) = Spielzustand(
        spieler = Spieler("ich", "Merlin"),
        tags = List(10) { "tag$it" },
        partyId = id,
        party = PartyAus(id = id, partner = partner?.let { Spieler("g", it) }),
        match = match,
        runden = runden,
    )

    // ------------------------------------------------------------ Kaskade ---

    @Test fun `ohne token der start`() {
        assertEquals(Bildschirm.Start, bildschirmFuer(Sicht()))
    }

    @Test fun `ohne lobby wird geladen`() {
        assertEquals(Bildschirm.Laden, bildschirmFuer(Sicht(angemeldet = true)))
    }

    // Der doppelte Name kommt vor allem anderen: Er macht jede Einladung
    // mehrdeutig, und das lässt sich nicht anzeigen, nur verhindern.
    @Test fun `doppelter name schlaegt tags und offene partie`() {
        val s = Sicht(
            angemeldet = true,
            lobby = lobby(partie(), tags = 0, eindeutig = false),
            partie = zustand(),
            offenePartie = "p1",
        )
        assertEquals(Bildschirm.NameWaehlen, bildschirmFuer(s))
    }

    @Test fun `unter zehn tags erst die auswahl`() {
        val s = Sicht(angemeldet = true, lobby = lobby(partie(), tags = 3))
        assertEquals(Bildschirm.Tags, bildschirmFuer(s))
    }

    @Test fun `ohne offene partie die lobby`() {
        assertEquals(
            Bildschirm.Lobby,
            bildschirmFuer(Sicht(angemeldet = true, lobby = lobby(partie()))),
        )
    }

    // Eine Auswahl, die es nicht mehr gibt, ist keine Auswahl. Das ist der
    // einzige erwünschte Sprung: Wird die Partie aufgelöst, während sie offen
    // ist, landet man von selbst in der Übersicht.
    @Test fun `verschwundene partie faellt in die lobby zurueck`() {
        val s = Sicht(
            angemeldet = true, lobby = lobby(partie("p2")),
            partie = zustand("p1"), offenePartie = "p1",
        )
        assertEquals(Bildschirm.Lobby, bildschirmFuer(s))
    }

    @Test fun `partie gewaehlt aber noch nicht geladen`() {
        val s = Sicht(angemeldet = true, lobby = lobby(partie()), offenePartie = "p1")
        assertEquals(Bildschirm.Laden, bildschirmFuer(s))
    }

    @Test fun `ohne partner der warteraum`() {
        val s = Sicht(
            angemeldet = true, lobby = lobby(partie(partner = null)),
            partie = zustand(partner = null), offenePartie = "p1",
        )
        assertEquals(Bildschirm.Warteraum, bildschirmFuer(s))
    }

    // Der Klonblick kommt vor dem Raten - und nur einmal je Runde.
    @Test fun `klonblick kommt vor dem raten und nur einmal`() {
        val meine = List(4) { KarteAus(it + 1, "karte", istEcht = it == 0, begruendung = "weil") }
        val karten = List(4) { KarteAus(it + 1, "ueber die andere seite") }
        val r = RundeAus(
            id = "r1", nummer = 1, zustand = "RATEN", meineAntwort = "steht",
            karten = karten, meineKarten = meine,
        )
        val s = Sicht(
            angemeldet = true, lobby = lobby(partie()),
            partie = zustand(runden = listOf(r)), offenePartie = "p1",
        )
        assertEquals(Bildschirm.Klone, bildschirmFuer(s))
        // Weggeklickt: ab jetzt wird geraten.
        val weg = s.copy(klone = mapOf("p1" to Gesehen("m1", 1)))
        assertEquals(Bildschirm.Raten, bildschirmFuer(weg))
        // Ein neues Match faengt bei Runde 1 an - der Merker darf nicht
        // vorgreifen. Dieselbe Falle wie bei der Aufloesung.
        val neuesMatch = s.copy(
            partie = zustand(match = MatchAus("m2", Stand(0, 0), 10, "OFFEN"), runden = listOf(r)),
            klone = mapOf("p1" to Gesehen("m1", 6)),
        )
        assertEquals(Bildschirm.Klone, bildschirmFuer(neuesMatch))
    }

    // Solange der eigene Satz nicht vollstaendig ist, gibt es nichts zu sehen.
    @Test fun `ohne vier eigene karten kein klonblick`() {
        val r = RundeAus(
            id = "r1", nummer = 1, zustand = "MIMIK_ARBEITET", meineAntwort = "steht",
            meineKarten = List(2) { KarteAus(it + 1, "halb") },
        )
        val s = Sicht(
            angemeldet = true, lobby = lobby(partie()),
            partie = zustand(runden = listOf(r)), offenePartie = "p1",
        )
        assertEquals(Bildschirm.Warten, bildschirmFuer(s))
    }

    @Test fun `die vier ausgaenge einer runde`() {
        fun mit(r: RundeAus) = bildschirmFuer(
            Sicht(
                angemeldet = true, lobby = lobby(partie()),
                partie = zustand(runden = listOf(r)), offenePartie = "p1",
            ),
        )
        val leer = RundeAus(id = "r1", nummer = 1, zustand = "SCHREIBEN")
        assertEquals(Bildschirm.Schreiben, mit(leer))
        assertEquals(Bildschirm.Warten, mit(leer.copy(meineAntwort = "steht")))
        val karten = List(4) { KarteAus(it + 1, "karte") }
        assertEquals(Bildschirm.Raten, mit(leer.copy(meineAntwort = "steht", karten = karten)))
        assertEquals(
            Bildschirm.Getippt,
            mit(leer.copy(meineAntwort = "steht", karten = karten, meinTipp = 2)),
        )
    }

    /**
     * Der Kernfall: Der Bildschirm hängt an der ID, nicht an einer Position.
     *
     * Kommen die Partien umsortiert zurück oder kommt eine fällige dazu, darf
     * sich an dem, was gerade offen ist, nichts ändern. Sonst springt die App
     * einem beim Schreiben unter den Fingern weg.
     */
    @Test fun `umsortierte lobby aendert den bildschirm nicht`() {
        val runde = RundeAus(id = "r1", nummer = 1, zustand = "SCHREIBEN")
        val vorher = Sicht(
            angemeldet = true,
            lobby = lobby(partie("p1"), partie("p2")),
            partie = zustand("p1", runden = listOf(runde)),
            offenePartie = "p1",
        )
        val nachher = vorher.copy(
            lobby = lobby(partie("p3", dran = "raten"), partie("p2"), partie("p1")),
        )
        assertEquals(Bildschirm.Schreiben, bildschirmFuer(vorher))
        assertEquals(bildschirmFuer(vorher), bildschirmFuer(nachher))
    }

    // ---------------------------------------------------------- Auflösung ---

    private fun aufgeloest(nummer: Int) = RundeAus(
        id = "r$nummer", nummer = nummer, zustand = "AUFGELOEST", meinTipp = 1,
    )

    @Test fun `die aelteste ungesehene zuerst`() {
        val z = zustand(runden = listOf(aufgeloest(1), aufgeloest(2)))
        assertEquals("r1", naechsteAufloesung(z, null))
        assertEquals("r2", naechsteAufloesung(z, Gesehen("m1", 1)))
        assertNull(naechsteAufloesung(z, Gesehen("m1", 2)))
    }

    // Rundennummern fangen in jedem Match wieder bei 1 an. Ein Merker ohne
    // Match-ID hielte jede Auflösung des zweiten Matches für gesehen - genau
    // dieser Fehler war im Betrieb zu sehen.
    @Test fun `neues match setzt den merker zurueck`() {
        val z = zustand(
            match = MatchAus("m2", Stand(0, 0), 10, "OFFEN"),
            runden = listOf(aufgeloest(1)),
        )
        assertEquals("r1", naechsteAufloesung(z, Gesehen("m1", 6)))
    }

    // Und je Partie getrennt, sonst verschluckt die eine die andere.
    @Test fun `partien stoeren einander nicht`() {
        val a = zustand("p1", runden = listOf(aufgeloest(1)))
        val b = zustand("p2", runden = listOf(aufgeloest(1)))
        val gesehen = mapOf("p1" to Gesehen("m1", 1))
        assertNull(naechsteAufloesung(a, gesehen["p1"]))
        assertEquals("r1", naechsteAufloesung(b, gesehen["p2"]))
    }

    @Test fun `ohne eigenen tipp keine aufloesung`() {
        val z = zustand(runden = listOf(aufgeloest(1).copy(meinTipp = null)))
        assertNull(naechsteAufloesung(z, null))
    }

    // ------------------------------------------------------------- Lobby ---

    @Test fun `faelliges steht oben`() {
        val l = lobby(
            partie("p1", dran = "warten", partner = "Zoe"),
            partie("p2", dran = "raten", partner = "Alex"),
            partie("p3", dran = "warten", partner = "Bo"),
        )
        val z = lobbyzeilen(l)
        assertEquals("p2", z.first().partyId)
        assertEquals(listOf("Bo", "Zoe"), z.drop(1).map { it.partner?.spitzname })
    }

    @Test fun `aufloesung gilt als faellig`() {
        assertTrue(partie(dran = "aufgeloest").faellig)
        assertTrue(!partie(dran = "warten").faellig)
    }

    // --------------------------------------------------------- Meldungen ---

    @Test fun `nur schreiben und raten werden gemeldet`() {
        val l = lobby(
            partie("p1", dran = "schreiben"),
            partie("p2", dran = "raten"),
            partie("p3", dran = "warten"),
            partie("p4", dran = "aufgeloest"),
            partie("p5", dran = "kein_match"),
        )
        val plan = meldeplan(l, emptyMap())
        assertEquals(listOf("p1", "p2"), plan.zeigen.map { it.partie })
    }

    @Test fun `dasselbe wird nicht zweimal gemeldet`() {
        val l = lobby(partie("p1", dran = "schreiben"))
        val erst = meldeplan(l, emptyMap())
        assertEquals(1, erst.zeigen.size)
        val zweit = meldeplan(l, erst.merker)
        assertEquals(0, zweit.zeigen.size)
    }

    // Der Phasenwechsel in derselben Runde ist eine neue Nachricht: Erst fehlte
    // die Antwort, jetzt liegen die Karten.
    @Test fun `phasenwechsel meldet erneut`() {
        val erst = meldeplan(lobby(partie("p1", dran = "schreiben")), emptyMap())
        val zweit = meldeplan(lobby(partie("p1", dran = "raten")), erst.merker)
        assertEquals(1, zweit.zeigen.size)
    }

    /**
     * Der Test gegen die Meldung zu einer vergangenen Phase: Ist der Zug
     * gemacht, kommt die Partie auf die Löschliste - die alte Meldung fällt aus
     * dem Schacht, statt dort eine Phase zu behaupten, die es nicht mehr gibt.
     */
    @Test fun `erledigter zug raeumt die meldung weg`() {
        val erst = meldeplan(lobby(partie("p1", dran = "schreiben")), emptyMap())
        val danach = meldeplan(lobby(partie("p1", dran = "warten")), erst.merker)
        assertEquals(0, danach.zeigen.size)
        assertEquals(listOf("p1"), danach.loeschen)
        assertTrue(danach.merker.isEmpty())
    }

    // Die alte Meldung "Die Party ist vollständig" ist entfallen: Sie war kein
    // Auftrag, und gemeldet wird nur noch, was eine Handlung verlangt.
    @Test fun `vollstaendige party ohne match meldet nichts`() {
        val plan = meldeplan(lobby(partie("p1", dran = "kein_match")), emptyMap())
        assertTrue(plan.zeigen.isEmpty())
    }
}
