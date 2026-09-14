package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"mimik/internal/game"
)

// RundenProMatch ist die erste Ladung. Reicht sie nicht bis zum Ziel, werden
// weitere nachgelegt (siehe RundeNachlegen).
const RundenProMatch = 6

type Match struct {
	ID       string        `json:"id"`
	PartyID  string        `json:"party_id"`
	Stand    game.Stand    `json:"stand"`
	Ziel     int           `json:"ziel"`
	Karten   int           `json:"karten"`
	Ergebnis game.Ergebnis `json:"ergebnis"`
}

func (s *Store) MatchAnlegen(partyID string) (Match, error) {
	m := Match{ID: id(), PartyID: partyID, Ziel: game.Ziel, Karten: 4, Ergebnis: game.Offen}
	tx, err := s.db.Begin()
	if err != nil {
		return m, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO matches (id, party_id, ziel, karten, erstellt_am) VALUES (?,?,?,?,?)`,
		m.ID, partyID, m.Ziel, m.Karten, jetzt()); err != nil {
		return m, err
	}
	mit, err := mitgliederTx(tx, partyID)
	if err != nil {
		return m, err
	}
	for i := 1; i <= RundenProMatch; i++ {
		if err := rundeAnlegenTx(tx, m.ID, partyID, mit, i); err != nil {
			return m, err
		}
	}
	return m, tx.Commit()
}

// rundeAnlegenTx zieht eine Frage, die noch KEINER DER BEIDEN hatte.
//
// Vorher haing das an fragen_pool.benutzt, und dort steht eine party_id.
// Solange jeder nur eine Partie hatte, war das dasselbe; sobald jemand zwei
// spielt, bekaeme er dieselbe Frage ein zweites Mal - und die zweite Antwort
// waere die erste, nur schlechter.
func rundeAnlegenTx(tx *sql.Tx, matchID, partyID string, mitglieder []string, nummer int) error {
	var fid int
	var text, rubrik string
	a, b := "", ""
	if len(mitglieder) > 0 {
		a = mitglieder[0]
	}
	if len(mitglieder) > 1 {
		b = mitglieder[1]
	}
	err := tx.QueryRow(
		`SELECT id, text, rubrik FROM fragen_pool
		  WHERE id NOT IN (SELECT frage_id FROM fragen_vergeben WHERE player_id IN (?,?))
		  ORDER BY RANDOM() LIMIT 1`, a, b).Scan(&fid, &text, &rubrik)

	// Zweite Stufe: eine, die WENIGSTENS EINER der beiden noch nicht hatte.
	//
	// Ohne diese Stufe fiel der Rueckweg sofort auf "irgendeine" - und eine
	// zufaellig gezogene Frage hatten unter Umstaenden BEIDE schon. Wenn eine
	// Wiederholung unvermeidlich ist, soll sie wenigstens nur einen von zwei
	// Menschen treffen.
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRow(
			`SELECT p.id, p.text, p.rubrik FROM fragen_pool p
			  ORDER BY (SELECT COUNT(*) FROM fragen_vergeben v
			             WHERE v.frage_id = p.id AND v.player_id IN (?,?)), RANDOM()
			  LIMIT 1`, a, b).Scan(&fid, &text, &rubrik)
	}
	if err != nil {
		return fmt.Errorf("fragenvorrat leer: %w", err)
	}
	// Und laut sagen, wenn es knapp wird. Der Vorrat ist endlich: 144 Fragen
	// reichen fuer rund zwanzig Matches, danach sieht jemand eine zweite Mal -
	// und das faellt zuerst dem Spieler auf, nicht dem Betreiber.
	for _, x := range mitglieder {
		if x == "" {
			continue
		}
		var offen int
		tx.QueryRow(
			`SELECT COUNT(*) FROM fragen_pool
			  WHERE id NOT IN (SELECT frage_id FROM fragen_vergeben WHERE player_id = ?)`,
			x).Scan(&offen)
		if offen <= 15 {
			log.Printf("fragenvorrat: fuer %s sind nur noch %d fragen offen", x[:8], offen)
		}
	}
	if _, err := tx.Exec(`UPDATE fragen_pool SET benutzt = ? WHERE id = ?`, partyID, fid); err != nil {
		return err
	}
	for _, x := range mitglieder {
		if x == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO fragen_vergeben (player_id, frage_id, party_id, vergeben_am)
			 VALUES (?,?,?,?)`, x, fid, partyID, jetzt()); err != nil {
			return err
		}
	}
	_, err = tx.Exec(
		`INSERT INTO rounds (id, match_id, nummer, frage, rubrik, geoeffnet_am) VALUES (?,?,?,?,?,?)`,
		id(), matchID, nummer, text, rubrik, jetzt())
	return err
}

// mitgliederTx liest die beiden Spieler einer Partie in der laufenden
// Transaktion - rundeAnlegenTx braucht sie, um Fragen je Mensch zu vergeben.
func mitgliederTx(tx *sql.Tx, partyID string) ([]string, error) {
	rows, err := tx.Query(`SELECT player_id FROM party_members WHERE party_id = ?`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// RundeNachlegen hängt eine weitere Runde an ein laufendes Match.
func (s *Store) RundeNachlegen(m Match) error {
	var max int
	s.db.QueryRow(`SELECT COALESCE(MAX(nummer),0) FROM rounds WHERE match_id = ?`, m.ID).Scan(&max)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	mit, err := mitgliederTx(tx, m.PartyID)
	if err != nil {
		return err
	}
	if err := rundeAnlegenTx(tx, m.ID, m.PartyID, mit, max+1); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Match(mid string) (Match, error) {
	var m Match
	err := s.db.QueryRow(
		`SELECT id, party_id, punkte_mensch, punkte_mimik, ziel, karten, ergebnis FROM matches WHERE id = ?`, mid).
		Scan(&m.ID, &m.PartyID, &m.Stand.Mensch, &m.Stand.Mimik, &m.Ziel, &m.Karten, &m.Ergebnis)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNichtGefunden
	}
	return m, err
}

// LetztesMatch liefert das neueste Match einer Party, auch ein entschiedenes.
// Der Zustandsendpunkt braucht das: Nach dem Matchende will der Client den
// Endstand anzeigen, nicht plötzlich gar kein Match mehr sehen.
func (s *Store) LetztesMatch(partyID string) (Match, error) {
	var mid string
	err := s.db.QueryRow(
		`SELECT id FROM matches WHERE party_id = ? ORDER BY erstellt_am DESC, rowid DESC LIMIT 1`,
		partyID).Scan(&mid)
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNichtGefunden
	}
	if err != nil {
		return Match{}, err
	}
	return s.Match(mid)
}

// AktivesMatch liefert das laufende Match einer Party, falls es eins gibt.
func (s *Store) AktivesMatch(partyID string) (Match, error) {
	var mid string
	err := s.db.QueryRow(
		`SELECT id FROM matches WHERE party_id = ? AND ergebnis = 'OFFEN' ORDER BY erstellt_am DESC LIMIT 1`,
		partyID).Scan(&mid)
	if errors.Is(err, sql.ErrNoRows) {
		return Match{}, ErrNichtGefunden
	}
	if err != nil {
		return Match{}, err
	}
	return s.Match(mid)
}

// ------------------------------------------------------------------ Runde ---

// Runde lädt eine Runde samt Antworten, Karten und Tipps.
// MatchAbbrechen beendet ein laufendes Match für BEIDE Seiten.
//
// Es gehört beiden, also endet es auch für beide – die andere Seite sieht beim
// nächsten Abgleich, dass es vorbei ist, statt weiter auf eine Antwort zu
// warten, die nie kommt. Offene Runden werden auf AUFGELOEST gesetzt, sonst
// zöge der Worker sie weiter durch und riefe das Modell für ein Spiel, das
// niemand mehr spielt.
func (s *Store) MatchAbbrechen(mid string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`UPDATE matches SET ergebnis = ?, beendet_am = ? WHERE id = ? AND ergebnis = ?`,
		game.Abgebrochen, jetzt(), mid, game.Offen); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE rounds SET zustand = ?, aufgeloest_am = ? WHERE match_id = ? AND zustand != ?`,
		game.Aufgeloest, jetzt(), mid, game.Aufgeloest); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Runde(rid string) (game.Runde, error) {
	var r game.Runde
	err := s.db.QueryRow(`SELECT id, match_id, nummer, frage FROM rounds WHERE id = ?`, rid).
		Scan(&r.ID, &r.MatchID, &r.Nummer, &r.Frage)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNichtGefunden
	}
	if err != nil {
		return r, err
	}
	r.Antworten = map[game.PlayerID]game.Antwort{}
	r.Karten = map[game.PlayerID][]game.Karte{}
	r.Tipps = map[game.PlayerID]game.Tipp{}

	rows, err := s.db.Query(`SELECT player_id, original, normalform FROM answers WHERE round_id = ?`, rid)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var pid string
		var a game.Antwort
		if err := rows.Scan(&pid, &a.Original, &a.Normalform); err != nil {
			rows.Close()
			return r, err
		}
		r.Antworten[game.PlayerID(pid)] = a
	}
	rows.Close()

	rows, err = s.db.Query(
		`SELECT k.ueber, k.pos, k.text, k.ist_echt, k.anker_tag, COALESCE(g.grund,'')
		   FROM karten k
		   LEFT JOIN karten_gruende g
		          ON g.round_id = k.round_id AND g.ueber = k.ueber AND g.pos = k.pos
		  WHERE k.round_id = ? ORDER BY k.ueber, k.pos`, rid)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var ueber string
		var k game.Karte
		if err := rows.Scan(&ueber, &k.Pos, &k.Text, &k.IstEcht, &k.Richtung, &k.Begruendung); err != nil {
			rows.Close()
			return r, err
		}
		p := game.PlayerID(ueber)
		r.Karten[p] = append(r.Karten[p], k)
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT rater_id, gewaehlt, richtig FROM guesses WHERE round_id = ?`, rid)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var t game.Tipp
		if err := rows.Scan(&pid, &t.Gewaehlt, &t.Richtig); err != nil {
			return r, err
		}
		r.Tipps[game.PlayerID(pid)] = t
	}
	return r, rows.Err()
}

func (s *Store) RundenVonMatch(mid string) ([]game.Runde, error) {
	rows, err := s.db.Query(`SELECT id FROM rounds WHERE match_id = ? ORDER BY nummer`, mid)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, x)
	}
	rows.Close()
	out := make([]game.Runde, 0, len(ids))
	for _, x := range ids {
		r, err := s.Runde(x)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) AntwortSpeichern(rid, pid string, a game.Antwort) error {
	_, err := s.db.Exec(
		`INSERT INTO answers (round_id, player_id, original, normalform, erstellt_am)
		 VALUES (?,?,?,?,?)`, rid, pid, a.Original, a.Normalform, jetzt())
	return err
}

// ArbeitSeit sagt, wie viele Sekunden MIMIK an dieser Runde arbeitet: gezaehlt
// ab der SPAETEREN der beiden Antworten.
//
// Vorher zaehlte es ab der eigenen. Wer zuerst schrieb und dann eine halbe
// Stunde auf die andere Seite wartete, sah den Fortschrittsbalken bei seinem
// ersten Blick schon voll - die Zeit war ja wirklich vergangen, nur nicht mit
// Arbeit. Gearbeitet wird erst, wenn beide geschrieben haben (siehe
// Runde.Ableiten), und genau das ist der Nullpunkt. Weniger als zwei
// Antworten: 0, es laeuft noch nichts.
func (s *Store) ArbeitSeit(rid string) int {
	var anzahl int
	var spaeteste string
	if err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(erstellt_am),'') FROM answers WHERE round_id=?`,
		rid).Scan(&anzahl, &spaeteste); err != nil {
		return 0
	}
	if anzahl < 2 {
		return 0
	}
	t, err := time.Parse(time.RFC3339, spaeteste)
	if err != nil {
		return 0
	}
	d := int(time.Since(t).Seconds())
	if d < 0 {
		return 0
	}
	return d
}

// NormalformSetzen trägt die vom Modell geschriebene Fassung nach. Beim
// Absenden stand hier die regelbasierte Notfassung – sie hält den
// Wartebildschirm gefüllt, bis der Worker durch ist.
func (s *Store) NormalformSetzen(rid, pid, text string) error {
	_, err := s.db.Exec(
		`UPDATE answers SET normalform = ? WHERE round_id = ? AND player_id = ?`,
		text, rid, pid)
	return err
}

func (s *Store) KartenSpeichern(rid, ueber string, karten []game.Karte) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM karten WHERE round_id = ? AND ueber = ?`, rid, ueber); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM karten_gruende WHERE round_id = ? AND ueber = ?`, rid, ueber); err != nil {
		return err
	}
	for _, k := range karten {
		echt := 0
		if k.IstEcht {
			echt = 1
		}
		if _, err := tx.Exec(
			// Spalte heißt weiter anker_tag: Sie trägt jetzt die Richtung, die sich
			// MIMIK selbst gesucht hat. Umbenennen hieße migrieren, und der Name
			// steht nur in der Datenbank, nirgends im Code.
			`INSERT INTO karten (round_id, ueber, pos, text, ist_echt, anker_tag) VALUES (?,?,?,?,?,?)`,
			rid, ueber, k.Pos, k.Text, echt, k.Richtung); err != nil {
			return err
		}
		if k.Begruendung == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO karten_gruende (round_id, ueber, pos, grund) VALUES (?,?,?,?)`,
			rid, ueber, k.Pos, k.Begruendung); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ZustandSetzen(rid string, z game.Zustand, fehler string) error {
	var f any
	if fehler != "" {
		f = fehler
	}
	auf := any(nil)
	if z == game.Aufgeloest {
		auf = jetzt()
	}
	_, err := s.db.Exec(
		`UPDATE rounds SET zustand = ?, fehler = ?, aufgeloest_am = COALESCE(?, aufgeloest_am) WHERE id = ?`,
		string(z), f, auf, rid)
	return err
}

func (s *Store) RundenFehler(rid string) string {
	var f sql.NullString
	s.db.QueryRow(`SELECT fehler FROM rounds WHERE id = ?`, rid).Scan(&f)
	return f.String
}

// TippSpeichern schreibt den Tipp und, falls die Runde damit endet, verbucht es
// die zwei Punkte und prüft das Matchende – alles in einer Transaktion.
func (s *Store) TippSpeichern(m Match, pa game.Party, r game.Runde, rater string) (Match, bool, error) {
	t := r.Tipps[game.PlayerID(rater)]
	richtig := 0
	if t.Richtig {
		richtig = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return m, false, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO guesses (round_id, rater_id, gewaehlt, richtig, erstellt_am) VALUES (?,?,?,?,?)`,
		r.ID, rater, t.Gewaehlt, richtig, jetzt()); err != nil {
		return m, false, err
	}
	fertig := r.Ableiten(pa) == game.Aufgeloest
	if fertig {
		m.Stand = game.Verbuchen(m.Stand, r, pa)
		m.Ergebnis = game.Auswerten(m.Stand)
		var beendet any
		if m.Ergebnis != game.Offen && m.Ergebnis != game.Verlaengerung {
			beendet = jetzt()
		}
		if _, err := tx.Exec(
			`UPDATE matches SET punkte_mensch=?, punkte_mimik=?, ergebnis=?, beendet_am=? WHERE id=?`,
			m.Stand.Mensch, m.Stand.Mimik, string(m.Ergebnis), beendet, m.ID); err != nil {
			return m, false, err
		}
		if _, err := tx.Exec(
			`UPDATE rounds SET zustand=?, aufgeloest_am=? WHERE id=?`,
			string(game.Aufgeloest), jetzt(), r.ID); err != nil {
			return m, false, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE rounds SET zustand=? WHERE id=?`, string(game.RatenWartet), r.ID); err != nil {
			return m, false, err
		}
	}
	return m, fertig, tx.Commit()
}

// OffeneRunden liefert Runden, die auf MIMIK warten – für den Worker.
func (s *Store) OffeneRunden() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id
		  WHERE r.zustand IN ('SCHREIBEN','SCHREIBEN_WARTET','MIMIK_ARBEITET') AND m.ergebnis = 'OFFEN'
		  ORDER BY r.geoeffnet_am`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) PartyVonRunde(rid string) (game.Party, Match, error) {
	var mid, partyID string
	err := s.db.QueryRow(
		`SELECT m.id, m.party_id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE r.id = ?`, rid).
		Scan(&mid, &partyID)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Party{}, Match{}, ErrNichtGefunden
	}
	if err != nil {
		return game.Party{}, Match{}, err
	}
	pa, err := s.Party(partyID)
	if err != nil {
		return pa, Match{}, err
	}
	m, err := s.Match(mid)
	return pa, m, err
}

var _ = time.Now
