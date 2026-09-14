package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"mimik/internal/game"
)

func offen(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func spieler(t *testing.T, s *Store, name string) Player {
	t.Helper()
	p, _, err := s.SpielerAnlegen(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return p
}

// Ein Name gehoert einmal. Der zweite Anmeldeversuch darf keinen halben
// Spieler hinterlassen - sonst stuende jemand in players, den es nie gab.
func TestNameNurEinmal(t *testing.T) {
	s := offen(t)
	spieler(t, s, "Robin")
	if _, _, err := s.SpielerAnlegen("robin"); err == nil {
		t.Fatal("zwei spieler namens robin")
	}
	var n int
	s.DB().QueryRow(`SELECT COUNT(*) FROM players`).Scan(&n)
	if n != 1 {
		t.Fatalf("%d spieler in der tabelle statt 1", n)
	}
}

// Bestandsdubletten: Die Datenbank kann sie schon enthalten, weil Namen frueher
// frei waren. Sie duerfen weiterspielen, nur nicht auffindbar sein.
func TestDubletteSpieltWeiterIstAberUnsichtbar(t *testing.T) {
	s := offen(t)
	a := spieler(t, s, "Kim")
	// Roh eingefuegt, ohne Namensanspruch - genau der Zustand einer alten
	// Datenbank.
	if _, err := s.DB().Exec(
		`INSERT INTO players (id, spitzname, erstellt_am) VALUES ('alt','Kim','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatal(err)
	}
	if s.NameGeklaert("alt") {
		t.Fatal("die dublette gilt als geklaert")
	}
	if !s.NameGeklaert(a.ID) {
		t.Fatal("der ordentlich angelegte spieler gilt als ungeklaert")
	}
	// Gesucht wird ueber die Namenstabelle: Die Dublette taucht nicht auf.
	treffer, err := s.SpielerSuchen("kim", "wer-anders")
	if err != nil {
		t.Fatal(err)
	}
	if len(treffer) != 1 || treffer[0].ID != a.ID {
		t.Fatalf("suche liefert %d treffer statt genau des angemeldeten", len(treffer))
	}
	// Und sie kommt aus dem Zustand heraus, indem sie einen freien Namen nimmt.
	if err := s.NameBeanspruchen("alt", "Kim zwei"); err != nil {
		t.Fatal(err)
	}
	if !s.NameGeklaert("alt") {
		t.Fatal("nach dem anspruch immer noch ungeklaert")
	}
}

// Die Suche darf kein Verzeichnis sein.
func TestSucheIstKeinVerzeichnis(t *testing.T) {
	s := offen(t)
	spieler(t, s, "Robin")
	spieler(t, s, "Rosa")
	for _, q := range []string{"%", "_", "r", " "} {
		if tr, _ := s.SpielerSuchen(q, ""); len(tr) != 0 {
			t.Fatalf("%q liefert %d treffer", q, len(tr))
		}
	}
	if tr, _ := s.SpielerSuchen("ro", ""); len(tr) != 2 {
		t.Fatalf("ro liefert %d treffer statt 2", len(tr))
	}
}

// Testspieler halten niemanden auf und tauchen in keiner Suche auf.
func TestTestspielerBekommenNummernUndBleibenUnsichtbar(t *testing.T) {
	s := offen(t)
	p := spieler(t, s, "Merlin")
	namen := map[string]bool{}
	for i := 0; i < MaxTestpartien; i++ {
		pa, err := s.TestpartyAnlegen(p.ID)
		if err != nil {
			t.Fatalf("testpartie %d: %v", i+1, err)
		}
		bot, _ := s.Spieler(string(pa.B))
		if namen[bot.Spitzname] {
			t.Fatalf("zweimal derselbe botname: %q", bot.Spitzname)
		}
		namen[bot.Spitzname] = true
	}
	if _, err := s.TestpartyAnlegen(p.ID); err == nil {
		t.Fatal("der deckel fuer testpartien greift nicht")
	}
	if tr, _ := s.SpielerSuchen("testbot", p.ID); len(tr) != 0 {
		t.Fatalf("testspieler sind auffindbar: %d treffer", len(tr))
	}
}

// Der ganze Weg einer Einladung, und die Faelle, die danebengehen koennen.
func TestEinladung(t *testing.T) {
	s := offen(t)
	a := spieler(t, s, "Anna")
	b := spieler(t, s, "Bo")

	if _, err := s.EinladungAnlegen(a.ID, a.ID); err == nil {
		t.Fatal("man kann sich selbst einladen")
	}
	e, err := s.EinladungAnlegen(a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EinladungAnlegen(a.ID, b.ID); err == nil {
		t.Fatal("dieselbe einladung zweimal")
	}
	ein, aus, err := s.EinladungenFuer(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ein) != 1 || len(aus) != 0 || ein[0].Gegenueber.ID != a.ID {
		t.Fatalf("b sieht %d eingehende, %d ausgehende", len(ein), len(aus))
	}
	// Gegeneinladung: Wenn sich beide gleichzeitig einladen, darf am Ende
	// trotzdem nur eine Partie stehen.
	if _, err := s.EinladungAnlegen(b.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	pa, err := s.EinladungAnnehmen(e.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if xs, _ := s.PartienVon(a.ID); len(xs) != 1 || xs[0].ID != pa.ID {
		t.Fatalf("a hat %d partien", len(xs))
	}
	ein, aus, _ = s.EinladungenFuer(a.ID)
	if len(ein)+len(aus) != 0 {
		t.Fatalf("nach der annahme stehen noch %d einladungen offen", len(ein)+len(aus))
	}
	if _, err := s.EinladungAnnehmen(e.ID, b.ID); err == nil {
		t.Fatal("dieselbe einladung zweimal angenommen")
	}
	if _, err := s.EinladungAnlegen(a.ID, b.ID); err == nil {
		t.Fatal("einladung trotz laufender partie")
	}
}

// Mehrere Partien nebeneinander: Verlassen trifft genau eine, Loeschen alle.
func TestMehrerePartien(t *testing.T) {
	s := offen(t)
	ich := spieler(t, s, "Merlin")
	var partien []string
	for _, name := range []string{"Robin", "Alex", "Sam"} {
		g := spieler(t, s, name)
		e, err := s.EinladungAnlegen(ich.ID, g.ID)
		if err != nil {
			t.Fatal(err)
		}
		pa, err := s.EinladungAnnehmen(e.ID, g.ID)
		if err != nil {
			t.Fatal(err)
		}
		partien = append(partien, pa.ID)
	}
	if xs, _ := s.PartienVon(ich.ID); len(xs) != 3 {
		t.Fatalf("%d partien statt 3", len(xs))
	}
	if err := s.PartyVerlassen(ich.ID, partien[0]); err != nil {
		t.Fatal(err)
	}
	if xs, _ := s.PartienVon(ich.ID); len(xs) != 2 {
		t.Fatalf("nach dem verlassen %d partien statt 2", len(xs))
	}
	// Eine fremde Partie ist nicht zu verlassen und nicht zu lesen.
	fremd := spieler(t, s, "Fremd")
	if _, err := s.PartyVon(fremd.ID, partien[1]); err == nil {
		t.Fatal("fremde partie ohne mitgliedschaft lesbar")
	}
	// Und Loeschen raeumt alles ab. Vorher lief das bei mehreren Partien in
	// einen Fremdschluesselfehler auf DELETE FROM players.
	if err := s.AllesLoeschen(ich.ID); err != nil {
		t.Fatalf("alles loeschen: %v", err)
	}
	if _, err := s.Spieler(ich.ID); err == nil {
		t.Fatal("der spieler steht noch")
	}
	var n int
	s.DB().QueryRow(`SELECT COUNT(*) FROM party_members WHERE player_id = ?`, ich.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d mitgliedschaften uebrig", n)
	}
	// Die Gegenueber behalten ihr Konto.
	if tr, _ := s.SpielerSuchen("robin", ""); len(tr) != 1 {
		t.Fatal("das gegenueber wurde mitgeloescht")
	}
}

// Die Lobby sagt je Partie, was ansteht.
func TestLobbySagtWasAnsteht(t *testing.T) {
	s := offen(t)
	ich := spieler(t, s, "Merlin")
	g := spieler(t, s, "Robin")
	e, _ := s.EinladungAnlegen(ich.ID, g.ID)
	pa, err := s.EinladungAnnehmen(e.ID, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []string{ich.ID, g.ID} {
		if err := s.TagsSetzen(x, TestbotTags); err != nil {
			t.Fatal(err)
		}
	}

	l, err := s.Lobby(ich.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 1 || l[0].Dran != "kein_match" || l[0].Partner == nil {
		t.Fatalf("ohne match: %+v", l)
	}

	m, err := s.MatchAnlegen(pa.ID)
	if err != nil {
		t.Fatal(err)
	}
	l, _ = s.Lobby(ich.ID)
	if l[0].Dran != "schreiben" || l[0].Runde == "" {
		t.Fatalf("mit offenem match: %+v", l[0])
	}

	rd, err := s.RundenVonMatch(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AntwortSpeichern(rd[0].ID, ich.ID, game.Antwort{Original: "roh", Normalform: "Roh."}); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Lobby(ich.ID)
	if l[0].Dran != "warten" {
		t.Fatalf("nach der eigenen antwort: %q", l[0].Dran)
	}

	// Eine zweite Partie mit offenem Code hat weder Partner noch Match.
	if _, _, err := s.PartyAnlegen(ich.ID); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Lobby(ich.ID)
	if len(l) != 2 {
		t.Fatalf("%d zeilen statt 2", len(l))
	}
	var offenerCode bool
	for _, x := range l {
		if x.Dran == "kein_partner" && x.Code != "" {
			offenerCode = true
		}
	}
	if !offenerCode {
		t.Fatalf("die gegruendete partie fehlt oder hat keinen code: %+v", l)
	}
}

// Der Kartenbau hatte keinen Versuchszaehler: Eine dauerhaft scheiternde Runde
// rief alle 20 Sekunden erneut an, bis zu dreimal je Takt, unbegrenzt. Eine
// Nacht davon kostet mehr als hundert Matches.
func TestKartenbauWirdZurueckgestellt(t *testing.T) {
	s := offen(t)
	// kartenbau haengt an rounds und players - also erst die Eltern.
	p := spieler(t, s, "Karten")
	for _, q := range []string{
		`INSERT INTO parties (id, code, code_bis, erstellt_am) VALUES ('pa1',NULL,NULL,'2026-01-01T00:00:00Z')`,
		`INSERT INTO matches (id, party_id, erstellt_am) VALUES ('m1','pa1','2026-01-01T00:00:00Z')`,
		`INSERT INTO rounds (id, match_id, nummer, frage, rubrik, geoeffnet_am)
		 VALUES ('r1','m1',1,'Frage?','a','2026-01-01T00:00:00Z')`,
	} {
		if _, err := s.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if s.KartenbauWartet("r1", p.ID) {
		t.Fatal("ohne versuch schon zurueckgestellt")
	}
	if err := s.KartenbauBeanspruchen("r1", p.ID); err != nil {
		t.Fatal(err)
	}
	// Nach dem ersten Versuch gilt eine Wartezeit - sonst waere der Zaehler
	// wirkungslos.
	if !s.KartenbauWartet("r1", p.ID) {
		t.Fatal("nach dem ersten versuch keine wartezeit")
	}
	var versuche int
	s.DB().QueryRow(`SELECT versuche FROM kartenbau WHERE round_id='r1'`).Scan(&versuche)
	if versuche != 1 {
		t.Fatalf("%d versuche statt 1", versuche)
	}
	// Die Wartezeit verdoppelt sich und kommt nie ueber eine Stunde.
	for i := 0; i < MaxKartenversuche+3; i++ {
		s.KartenbauBeanspruchen("r1", p.ID)
	}
	var bis string
	s.DB().QueryRow(`SELECT naechster_versuch_am FROM kartenbau WHERE round_id='r1'`).Scan(&bis)
	tt, err := time.Parse(time.RFC3339, bis)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(tt); d > KartenbauMaxwarte+time.Minute {
		t.Fatalf("wartezeit %s ueber der obergrenze", d)
	}
	// Und mit den Karten ist der Zaehler weg.
	if err := s.KartenbauFertig("r1", p.ID); err != nil {
		t.Fatal(err)
	}
	if s.KartenbauWartet("r1", p.ID) {
		t.Fatal("der zaehler steht noch, obwohl die karten fertig sind")
	}
}

// Verbrauchte Themen kommen juengste-zuerst und deterministisch - vorher war es
// eine Map, und Go durchlaeuft die in zufaelliger Reihenfolge.
func TestGesperrteThemenSindGeordnetUndGedeckelt(t *testing.T) {
	s := offen(t)
	p := spieler(t, s, "Themen")
	for i := 0; i < 20; i++ {
		if err := s.ThemenSperren(p.ID, "r1", []string{fmt.Sprintf("thema%02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	erste, err := s.GesperrteThemen(p.ID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(erste) != 5 || erste[0] != "thema19" {
		t.Fatalf("%v", erste)
	}
	for i := 0; i < 5; i++ {
		wieder, _ := s.GesperrteThemen(p.ID, 5)
		for j := range erste {
			if wieder[j] != erste[j] {
				t.Fatalf("reihenfolge wechselt: %v gegen %v", erste, wieder)
			}
		}
	}
	alle, _ := s.GesperrteThemen(p.ID, 0)
	if len(alle) != 20 {
		t.Fatalf("ohne grenze %d statt 20", len(alle))
	}
}
