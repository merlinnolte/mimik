package store

import (
	"mimik/internal/mimik"
)

// MaxReviewversuche: Danach gibt der Worker eine Runde auf. Ohne Deckel liefe
// ein dauerhaft scheiterndes Review jede Minute erneut.
const MaxReviewversuche = 3

// Merkmal ist die Sicht der App auf ein Profilmerkmal. Die Konfidenz geht als
// Wort hinaus ("Stand"), nicht als Zahl - eine Zahl behauptet eine Genauigkeit,
// die diese Schaetzung nicht hat.
type Merkmal struct {
	Merkmal   string  `json:"merkmal"`
	Wert      string  `json:"wert"`
	Stand     string  `json:"stand"`
	Belege    int     `json:"belege"`
	Wider     int     `json:"wider"`
	Beleg     string  `json:"beleg,omitempty"`
	Konfidenz float64 `json:"-"`
}

// Profil laedt die Merkmale eines Spielers, das sicherste zuerst.
func (s *Store) Profil(pid string) ([]Merkmal, error) {
	rows, err := s.db.Query(
		`SELECT merkmal, wert, konfidenz, belege, wider, beleg
		   FROM profil_merkmale WHERE player_id = ?
		  ORDER BY konfidenz DESC, merkmal`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Merkmal
	for rows.Next() {
		var m Merkmal
		if err := rows.Scan(&m.Merkmal, &m.Wert, &m.Konfidenz, &m.Belege, &m.Wider, &m.Beleg); err != nil {
			return nil, err
		}
		m.Stand = mimik.Stufe(m.Konfidenz)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ProfilFuerModell liefert dieselben Merkmale in der Form, mit der die
// Arithmetik in internal/mimik rechnet.
func (s *Store) ProfilFuerModell(pid string) ([]mimik.Merkmal, error) {
	ms, err := s.Profil(pid)
	if err != nil {
		return nil, err
	}
	out := make([]mimik.Merkmal, 0, len(ms))
	for _, m := range ms {
		out = append(out, mimik.Merkmal{
			Merkmal: m.Merkmal, Wert: m.Wert, Konfidenz: m.Konfidenz,
			Belege: m.Belege, Wider: m.Wider, Beleg: m.Beleg,
		})
	}
	return out, nil
}

// ProfilAnwenden schreibt Stand und Verlauf in einer Transaktion.
func (s *Store) ProfilAnwenden(pid, rundeID string, as []mimik.Aenderung) error {
	if len(as) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, a := range as {
		m := a.Merkmal
		if a.Loeschen {
			if _, err := tx.Exec(
				`DELETE FROM profil_merkmale WHERE player_id = ? AND merkmal = ?`,
				pid, m.Merkmal); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(
				`INSERT INTO profil_merkmale
				   (player_id, merkmal, wert, konfidenz, belege, wider, beleg, round_id,
				    erstellt_am, geaendert_am)
				 VALUES (?,?,?,?,?,?,?,?,?,?)
				 ON CONFLICT(player_id, merkmal) DO UPDATE SET
				   wert = excluded.wert, konfidenz = excluded.konfidenz,
				   belege = excluded.belege, wider = excluded.wider,
				   beleg = excluded.beleg, round_id = excluded.round_id,
				   geaendert_am = excluded.geaendert_am`,
				pid, m.Merkmal, m.Wert, m.Konfidenz, m.Belege, m.Wider, m.Beleg, rundeID,
				jetzt(), jetzt()); err != nil {
				return err
			}
		}
		wertNachher := ""
		if !a.Loeschen {
			wertNachher = m.Wert
		}
		if _, err := tx.Exec(
			`INSERT INTO profil_verlauf
			   (player_id, merkmal, round_id, urteil, wert_vorher, wert_nachher, konfidenz,
			    beleg, erstellt_am)
			 VALUES (?,?,?,?,?,?,?,?,?)`,
			pid, m.Merkmal, rundeID, a.Urteil, a.WertVorher, wertNachher, m.Konfidenz,
			m.Beleg, jetzt()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Verlaufseintrag ist eine Zeile der Historie.
type Verlaufseintrag struct {
	Merkmal     string `json:"merkmal"`
	Urteil      string `json:"urteil"`
	WertVorher  string `json:"wert_vorher,omitempty"`
	WertNachher string `json:"wert_nachher,omitempty"`
	ErstelltAm  string `json:"erstellt_am"`
}

func (s *Store) ProfilVerlauf(pid string, grenze int) ([]Verlaufseintrag, error) {
	rows, err := s.db.Query(
		`SELECT merkmal, urteil, wert_vorher, wert_nachher, erstellt_am
		   FROM profil_verlauf WHERE player_id = ? ORDER BY id DESC LIMIT ?`, pid, grenze)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Verlaufseintrag
	for rows.Next() {
		var v Verlaufseintrag
		if err := rows.Scan(&v.Merkmal, &v.Urteil, &v.WertVorher, &v.WertNachher, &v.ErstelltAm); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Reviewauftrag: diese Runde, ueber diesen Spieler.
type Reviewauftrag struct {
	RundeID string
	Ueber   string
}

// OffeneReviews findet Runden, aus denen sich etwas lernen laesst.
//
// Ausgeschlossen, jeweils mit Grund:
//   - Runden ohne zwei Tipps. MatchAbbrechen setzt ganze Matches auf
//     AUFGELOEST, ohne dass jemand geraten hat - daraus ist nichts zu lernen.
//   - Runden, in denen ein Testspieler geraten hat. Der wuerfelt seinen Tipp;
//     ein gewuerfelter Tipp ist ein Scheinbeleg, und ein Scheinbeleg im Profil
//     ist schlimmer als kein Beleg.
//   - Reviews ueber Testspieler selbst. Ueber sie lernt niemand etwas.
func (s *Store) OffeneReviews(grenze int) ([]Reviewauftrag, error) {
	rows, err := s.db.Query(
		`SELECT r.id, a.player_id
		   FROM rounds r
		   JOIN answers a ON a.round_id = r.id
		   JOIN matches m ON m.id = r.match_id
		   LEFT JOIN reviews rv ON rv.round_id = r.id AND rv.ueber = a.player_id
		  WHERE r.zustand = 'AUFGELOEST' AND r.aufgeloest_am IS NOT NULL
		    AND (SELECT COUNT(*) FROM guesses g WHERE g.round_id = r.id) = 2
		    AND (SELECT COUNT(*) FROM karten k
		          WHERE k.round_id = r.id AND k.ueber = a.player_id) = 4
		    AND NOT EXISTS (SELECT 1 FROM bots b
		                      JOIN party_members pm ON pm.player_id = b.player_id
		                     WHERE pm.party_id = m.party_id)
		    AND (rv.round_id IS NULL OR (rv.fertig = 0 AND rv.versuche < ?))
		  ORDER BY r.aufgeloest_am
		  LIMIT ?`, MaxReviewversuche, grenze)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reviewauftrag
	for rows.Next() {
		var a Reviewauftrag
		if err := rows.Scan(&a.RundeID, &a.Ueber); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ReviewBeanspruchen zaehlt den Versuch gleich mit hoch.
//
// Ein Absturz mitten im Modellaufruf kostet damit einen Versuch statt einer
// ewigen Schleife - der bessere Tausch gegenueber einem Zustand "LAEUFT", den
// niemand aufraeumt.
func (s *Store) ReviewBeanspruchen(rid, ueber string) (bool, error) {
	var versuche int
	err := s.db.QueryRow(
		`INSERT INTO reviews (round_id, ueber, versuche, erstellt_am) VALUES (?,?,1,?)
		 ON CONFLICT(round_id, ueber) DO UPDATE SET versuche = versuche + 1
		 RETURNING versuche`, rid, ueber, jetzt()).Scan(&versuche)
	if err != nil {
		return false, err
	}
	return versuche <= MaxReviewversuche, nil
}

func (s *Store) ReviewFertig(rid, ueber string) error {
	_, err := s.db.Exec(
		`UPDATE reviews SET fertig = 1, fehler = '' WHERE round_id = ? AND ueber = ?`, rid, ueber)
	return err
}

func (s *Store) ReviewGescheitert(rid, ueber, meldung string) error {
	_, err := s.db.Exec(
		`UPDATE reviews SET fehler = ? WHERE round_id = ? AND ueber = ?`, meldung, rid, ueber)
	return err
}
