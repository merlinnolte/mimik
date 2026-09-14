package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"mimik/internal/seed"
)

func vorrat(t *testing.T, s *Store) map[string]string { // text -> rubrik
	t.Helper()
	rows, err := s.DB().Query(`SELECT text, rubrik FROM fragen_pool`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var t1, r string
		if err := rows.Scan(&t1, &r); err != nil {
			t.Fatal(err)
		}
		out[t1] = r
	}
	return out
}

func idVon(t *testing.T, s *Store, kennung string) int64 {
	t.Helper()
	var fid int64
	if err := s.DB().QueryRow(
		`SELECT frage_id FROM fragen_kennungen WHERE kennung = ?`, kennung).Scan(&fid); err != nil {
		t.Fatalf("kennung %q: %v", kennung, err)
	}
	return fid
}

func frischerStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Der Kern der Sache: Ein Tippfehler laesst sich beheben, OHNE dass die Frage
// eine neue id bekommt. Vorher war genau das unmoeglich - die Identitaet hing
// am Text, und jeder, der die Frage schon beantwortet hatte, war wieder fuer
// sie berechtigt.
func TestKorrekturBehaeltDieID(t *testing.T) {
	s := frischerStore(t)
	fragen := seed.Fragen()
	vorher := idVon(t, s, fragen[0].Kennung)

	fragen[0].Text = fragen[0].Text[:len(fragen[0].Text)-1] + " wirklich?"
	ber, err := s.fragenAbgleichenAus(fragen)
	if err != nil {
		t.Fatal(err)
	}
	if ber.Geaendert != 1 {
		t.Errorf("Geaendert = %d, erwartet 1", ber.Geaendert)
	}
	if nachher := idVon(t, s, fragen[0].Kennung); nachher != vorher {
		t.Errorf("id hat sich geaendert: %d -> %d", vorher, nachher)
	}
	if v := vorrat(t, s); v[fragen[0].Text] == "" {
		t.Error("korrigierter Text steht nicht im Vorrat")
	} else if len(v) != len(fragen) {
		t.Errorf("%d Zeilen im Vorrat, erwartet %d - die alte Fassung ist stehen geblieben",
			len(v), len(fragen))
	}
}

// Eine Frage aus fragen.json entfernen heisst: raus aus dem Vorrat. Ihre
// Kennung bleibt stehen, damit eine Rueckkehr DIESELBE id bekommt.
func TestRuecknahmeUndRueckkehr(t *testing.T) {
	s := frischerStore(t)
	alle := seed.Fragen()
	weg := alle[3]
	fid := idVon(t, s, weg.Kennung)

	gekuerzt := append(append([]seed.Frage{}, alle[:3]...), alle[4:]...)
	ber, err := s.fragenAbgleichenAus(gekuerzt)
	if err != nil {
		t.Fatal(err)
	}
	if ber.Entfernt != 1 {
		t.Errorf("Entfernt = %d, erwartet 1", ber.Entfernt)
	}
	if _, drin := vorrat(t, s)[weg.Text]; drin {
		t.Error("zurueckgezogene Frage steht noch im Vorrat")
	}
	if nach := idVon(t, s, weg.Kennung); nach != fid {
		t.Errorf("Kennung verlor ihre id: %d -> %d", fid, nach)
	}

	if _, err := s.fragenAbgleichenAus(alle); err != nil {
		t.Fatal(err)
	}
	if _, drin := vorrat(t, s)[weg.Text]; !drin {
		t.Error("wiederaufgenommene Frage fehlt im Vorrat")
	}
	if nach := idVon(t, s, weg.Kennung); nach != fid {
		t.Errorf("Wiederaufnahme gab eine neue id: %d -> %d", fid, nach)
	}
}

// Die Bruecke: Eine Datenbank, die aus der Zeit VOR den Kennungen stammt, hat
// den Vorrat schon stehen und schon vergeben. Ueber den Text muss jede Kennung
// ihre bestehende id finden - sonst bekaeme jeder Mensch alle Fragen noch
// einmal.
func TestBrueckeRettetDieAltenIDs(t *testing.T) {
	s := frischerStore(t)
	alle := seed.Fragen()
	alteIDs := map[string]int64{}
	for _, f := range alle {
		alteIDs[f.Text] = idVon(t, s, f.Kennung)
	}
	if _, err := s.DB().Exec(`DELETE FROM fragen_kennungen`); err != nil {
		t.Fatal(err)
	}

	ber, err := s.fragenAbgleichenAus(alle)
	if err != nil {
		t.Fatal(err)
	}
	if ber.Gebrueckt != len(alle) || ber.Neu != 0 {
		t.Errorf("Gebrueckt = %d, Neu = %d, erwartet %d und 0", ber.Gebrueckt, ber.Neu, len(alle))
	}
	for _, f := range alle {
		if neu := idVon(t, s, f.Kennung); neu != alteIDs[f.Text] {
			t.Fatalf("%q bekam eine neue id: %d -> %d", f.Kennung, alteIDs[f.Text], neu)
		}
	}
}

// Ein zweiter Lauf darf nichts tun. Der Abgleich laeuft bei JEDEM Serverstart.
func TestAbgleichIstFolgenlosBeimZweitenMal(t *testing.T) {
	s := frischerStore(t)
	ber, err := s.fragenAbgleichen()
	if err != nil {
		t.Fatal(err)
	}
	if ber.Neu != 0 || ber.Gebrueckt != 0 || ber.Geaendert != 0 || ber.Entfernt != 0 {
		t.Errorf("zweiter Lauf hat gearbeitet: %+v", ber)
	}
	if n := len(vorrat(t, s)); n != ber.Gesamt {
		t.Errorf("%d Zeilen, erwartet %d", n, ber.Gesamt)
	}
}

// Zwei Fragen tauschen ihre Texte. Ein UPDATE je Zeile wuerde hier an
// text UNIQUE scheitern, weil der Zieltext noch bei der anderen Zeile steht.
func TestTextTauschGehtDurch(t *testing.T) {
	s := frischerStore(t)
	fragen := seed.Fragen()
	fragen[0].Text, fragen[1].Text = fragen[1].Text, fragen[0].Text
	if _, err := s.fragenAbgleichenAus(fragen); err != nil {
		t.Fatalf("Tausch abgelehnt: %v", err)
	}
	var txt string
	if err := s.DB().QueryRow(`SELECT text FROM fragen_pool WHERE id = ?`,
		idVon(t, s, fragen[0].Kennung)).Scan(&txt); err != nil {
		t.Fatal(err)
	}
	if txt != fragen[0].Text {
		t.Errorf("Text nicht getauscht: %q", txt)
	}
}

// Eine neue Frage bekommt eine id, die keiner bestehenden Kennung gehoert.
// Nach dem DELETE faengt SQLites rowid wieder bei 1 an - wer sich darauf
// verlaesst, verteilt ids doppelt.
func TestNeueFrageBekommtEineFreieID(t *testing.T) {
	s := frischerStore(t)
	fragen := append(seed.Fragen(), seed.Frage{
		Kennung: "probe-neue-frage",
		Text:    "Was hast du heute zum ersten Mal bemerkt, obwohl es lange da war?",
		Rubrik:  "alltag",
	})
	ber, err := s.fragenAbgleichenAus(fragen)
	if err != nil {
		t.Fatal(err)
	}
	if ber.Neu != 1 {
		t.Errorf("Neu = %d, erwartet 1", ber.Neu)
	}
	neu := idVon(t, s, "probe-neue-frage")
	for _, f := range seed.Fragen() {
		if idVon(t, s, f.Kennung) == neu {
			t.Fatalf("id %d doppelt vergeben (%q)", neu, f.Kennung)
		}
	}
	if n := len(vorrat(t, s)); n != len(fragen) {
		t.Errorf("%d Zeilen, erwartet %d", n, len(fragen))
	}
}

// Loescht ein Spieler sein Konto, darf der Partner, der bleibt, seine Fragen
// NICHT zurueckbekommen. Er hat sie beantwortet; sie noch einmal zu stellen
// waere dieselbe Antwort, nur schlechter.
func TestKontoloeschenGibtDemPartnerNichtsZurueck(t *testing.T) {
	s := frischerStore(t)
	a, _, err := s.SpielerAnlegen("Anna")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := s.SpielerAnlegen("Bert")
	if err != nil {
		t.Fatal(err)
	}
	pa, kode, err := s.PartyAnlegen(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PartyBeitreten(kode, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MatchAnlegen(pa.ID); err != nil {
		t.Fatal(err)
	}

	var vorher int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fragen_vergeben WHERE player_id = ?`, b.ID).Scan(&vorher); err != nil {
		t.Fatal(err)
	}
	if vorher == 0 {
		t.Fatal("B hat keine Fragen bekommen - der Test prueft nichts")
	}
	if err := s.AllesLoeschen(a.ID); err != nil {
		t.Fatal(err)
	}
	var nachher int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fragen_vergeben WHERE player_id = ?`, b.ID).Scan(&nachher); err != nil {
		t.Fatal(err)
	}
	if nachher != vorher {
		t.Errorf("B hatte %d vergebene Fragen, nach dem Kontoloeschen von A nur noch %d",
			vorher, nachher)
	}
}

// ------------------------------------------------------ Stimmen und Gewicht ---

func TestFragengewichtIstAsymmetrischUndGedeckelt(t *testing.T) {
	faelle := []struct {
		mag, magNicht, will int
		warum               string
	}{
		{0, 0, GewichtNormal, "unbewertet ist die Mitte"},
		{1, 0, 9, "ein Zuspruch: anderthalbmal so oft"},
		{0, 1, 2, "eine Ablehnung: dreimal seltener"},
		{1, 1, 5, "Uneinigkeit landet fast wieder in der Mitte"},
		{0, 2, GewichtMin, "zwei Ablehnungen stossen an den Boden"},
		{0, 9, GewichtMin, "und fallen nicht darunter - eine Frage verschwindet nie ganz"},
		{9, 0, GewichtMax, "Zuspruch stoesst an den Deckel - der Vorrat lebt von Vielfalt"},
	}
	for _, f := range faelle {
		if g := Fragengewicht(f.mag, f.magNicht); g != f.will {
			t.Errorf("Fragengewicht(%d, %d) = %d, erwartet %d (%s)",
				f.mag, f.magNicht, g, f.will, f.warum)
		}
	}
	// Die Asymmetrie ist Absicht und keine Laune: Eine schlechte Frage
	// verbrennt eine Runde fuer zwei Menschen, eine gute ist nur etwas besser
	// als der Durchschnitt.
	if GewichtNormal-Fragengewicht(0, 1) <= Fragengewicht(1, 0)-GewichtNormal {
		t.Error("eine Ablehnung muss schwerer wiegen als ein Zuspruch")
	}
}

// Das Los kommt als Zahl herein, also ist die Auswahl vollstaendig nachrechenbar.
func TestWaehleGewichtetTrifftJedesLos(t *testing.T) {
	g := []int{1, 6, 3}
	will := []int{0, 1, 1, 1, 1, 1, 1, 2, 2, 2}
	if n := gewichtssumme(g); n != len(will) {
		t.Fatalf("Summe %d, erwartet %d", n, len(will))
	}
	for los, w := range will {
		if i := waehleGewichtet(g, los); i != w {
			t.Errorf("Los %d trifft %d, erwartet %d", los, i, w)
		}
	}
	// Ein Los ausserhalb der Summe darf nicht in den Graben laufen.
	if i := waehleGewichtet(g, 999); i != 2 {
		t.Errorf("Los ueber der Summe trifft %d, erwartet die letzte (2)", i)
	}
}

func TestUrteilKommtAnUndLaesstSichZuruecknehmen(t *testing.T) {
	s := frischerStore(t)
	a, _, _ := s.SpielerAnlegen("Anna")
	b, _, _ := s.SpielerAnlegen("Bert")
	pa, kode, err := s.PartyAnlegen(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PartyBeitreten(kode, b.ID); err != nil {
		t.Fatal(err)
	}
	m, err := s.MatchAnlegen(pa.ID)
	if err != nil {
		t.Fatal(err)
	}
	runden, err := s.RundenVonMatch(m.ID)
	if err != nil || len(runden) == 0 {
		t.Fatal(err)
	}
	rid := runden[0].ID

	if err := s.FrageUrteilen(a.ID, rid, 1); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.UrteileVonMatch(a.ID, m.ID); u[rid] != 1 {
		t.Errorf("eigene Stimme = %d, erwartet 1", u[rid])
	}
	// Die Stimme des einen ist nicht die des anderen.
	if u, _ := s.UrteileVonMatch(b.ID, m.ID); len(u) != 0 {
		t.Errorf("B sieht die Stimme von A: %v", u)
	}
	fid, err := s.FrageIDVonRunde(rid)
	if err != nil {
		t.Fatal(err)
	}
	if mag, nicht, _ := s.Urteilsstand(fid); mag != 1 || nicht != 0 {
		t.Errorf("Stand %d/%d, erwartet 1/0", mag, nicht)
	}

	// Umentscheiden ersetzt, es haeuft nicht.
	if err := s.FrageUrteilen(a.ID, rid, -1); err != nil {
		t.Fatal(err)
	}
	if mag, nicht, _ := s.Urteilsstand(fid); mag != 0 || nicht != 1 {
		t.Errorf("nach dem Umentscheiden %d/%d, erwartet 0/1", mag, nicht)
	}

	// Und zurueckziehen geht auch.
	if err := s.FrageUrteilen(a.ID, rid, 0); err != nil {
		t.Fatal(err)
	}
	if mag, nicht, _ := s.Urteilsstand(fid); mag != 0 || nicht != 0 {
		t.Errorf("nach dem Zuruecknehmen %d/%d, erwartet 0/0", mag, nicht)
	}
	if u, _ := s.UrteileVonMatch(a.ID, m.ID); len(u) != 0 {
		t.Errorf("zurueckgenommene Stimme steht noch da: %v", u)
	}
}

// Eine Frage, die nicht mehr im Vorrat steht, laesst sich nicht bewerten - und
// das ist kein Fehler des Spielers, sondern ein sauberer, benannter Rueckweg.
func TestUrteilZuVerschwundenerFrageMeldetSichSo(t *testing.T) {
	s := frischerStore(t)
	a, _, _ := s.SpielerAnlegen("Anna")
	b, _, _ := s.SpielerAnlegen("Bert")
	pa, kode, _ := s.PartyAnlegen(a.ID)
	s.PartyBeitreten(kode, b.ID)
	m, _ := s.MatchAnlegen(pa.ID)
	runden, _ := s.RundenVonMatch(m.ID)
	rid := runden[0].ID

	if _, err := s.DB().Exec(`DELETE FROM fragen_pool`); err != nil {
		t.Fatal(err)
	}
	err := s.FrageUrteilen(a.ID, rid, 1)
	if !errors.Is(err, ErrFrageNichtImVorrat) {
		t.Errorf("Fehler = %v, erwartet ErrFrageNichtImVorrat", err)
	}
}

// Der eigentliche Zweck: Eine abgelehnte Frage kommt seltener, eine
// zugesprochene haeufiger. Gemessen an einem kleinen Vorrat und vielen Zuegen.
func TestAbgelehnteFrageKommtSeltener(t *testing.T) {
	s := frischerStore(t)
	// Ein Vorrat aus genau drei Fragen, damit die Zahlen aussagen und nicht
	// rauschen.
	if _, err := s.DB().Exec(`DELETE FROM fragen_pool`); err != nil {
		t.Fatal(err)
	}
	for i, txt := range []string{"A?", "B?", "C?"} {
		if _, err := s.DB().Exec(
			`INSERT INTO fragen_pool (id, text, rubrik) VALUES (?,?,?)`,
			i+1, txt, "alltag"); err != nil {
			t.Fatal(err)
		}
	}
	// Ein Dritter hat B abgelehnt und C zugesprochen.
	dritter, _, _ := s.SpielerAnlegen("Cleo")
	for fid, u := range map[int]int{2: -1, 3: 1} {
		if _, err := s.DB().Exec(
			`INSERT INTO fragen_urteile (player_id, frage_id, urteil, erstellt_am)
			 VALUES (?,?,?,?)`, dritter.ID, fid, u, "jetzt"); err != nil {
			t.Fatal(err)
		}
	}

	zahl := map[string]int{}
	for i := 0; i < 240; i++ {
		a, _, _ := s.SpielerAnlegen(fmt.Sprintf("spieler-a-%d", i))
		b, _, _ := s.SpielerAnlegen(fmt.Sprintf("spieler-b-%d", i))
		pa, kode, err := s.PartyAnlegen(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.PartyBeitreten(kode, b.ID); err != nil {
			t.Fatal(err)
		}
		// Nur die ERSTE Runde zaehlt: Danach ist die gezogene Frage vergeben
		// und der Vorrat der beiden ein anderer.
		m, err := s.MatchAnlegen(pa.ID)
		if err != nil {
			t.Fatal(err)
		}
		runden, _ := s.RundenVonMatch(m.ID)
		zahl[runden[0].Frage]++
	}
	t.Logf("Ziehungen: %v (Gewichte A=%d B=%d C=%d)",
		zahl, Fragengewicht(0, 0), Fragengewicht(0, 1), Fragengewicht(1, 0))
	if zahl["B?"] >= zahl["A?"] {
		t.Errorf("abgelehnte Frage kam %dx, unbewertete %dx", zahl["B?"], zahl["A?"])
	}
	if zahl["C?"] <= zahl["B?"] {
		t.Errorf("zugesprochene Frage kam %dx, abgelehnte %dx", zahl["C?"], zahl["B?"])
	}
}
