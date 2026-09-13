package store

import (
	"database/sql"
	"errors"

	"mimik/internal/game"
)

// Einladung ist eine Anfrage von einem Spieler an einen anderen.
//
// Warum nicht sofort eine Partie: Namen sind suchbar, also koennte sonst jeder
// jeden ungefragt in Partien ziehen. Eine Einladung ist die kleinste Form von
// "ja, mit dir" - und sie laesst sich ablehnen, ohne dass etwas entsteht.
type Einladung struct {
	ID         string `json:"id"`
	VonID      string `json:"-"`
	AnID       string `json:"-"`
	Zustand    string `json:"zustand"`
	PartyID    string `json:"party_id,omitempty"`
	ErstelltAm string `json:"erstellt_am"`
	Gegenueber Player `json:"gegenueber"`
}

// EinladungAnlegen. Doppelte offene Einladungen faengt der Teilindex ab, nicht
// eine Pruefung hier - zwischen einem SELECT und einem INSERT liegt eine
// Luecke, und ein Doppeltipp findet sie zuverlaessig.
func (s *Store) EinladungAnlegen(von, an string) (Einladung, error) {
	if von == an {
		return Einladung{}, ErrSelbst
	}
	if _, err := s.Spieler(an); err != nil {
		return Einladung{}, ErrNichtGefunden
	}
	partner, err := s.SchonPartner(von, an)
	if err != nil {
		return Einladung{}, err
	}
	if partner {
		return Einladung{}, ErrSchonPartner
	}
	e := Einladung{ID: id(), VonID: von, AnID: an, Zustand: "OFFEN", ErstelltAm: jetzt()}
	_, err = s.db.Exec(
		`INSERT INTO einladungen (id, von_id, an_id, zustand, erstellt_am) VALUES (?,?,?,'OFFEN',?)`,
		e.ID, von, an, e.ErstelltAm)
	if err != nil {
		return Einladung{}, ErrSchonEingeladen
	}
	return e, nil
}

// SchonPartner sagt, ob zwischen zwei Spielern schon eine Partie laeuft.
// Zweimal dieselbe Person in der Lobby waere keine zweite Partie, sondern eine
// Verwechslungsgefahr.
func (s *Store) SchonPartner(a, b string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM party_members x
		   JOIN party_members y ON y.party_id = x.party_id
		  WHERE x.player_id = ? AND y.player_id = ?`, a, b).Scan(&n)
	return n > 0, err
}

// EinladungenFuer liefert, was offen ist - eingehend und ausgehend getrennt,
// weil sich daraus verschiedene Knoepfe ergeben.
func (s *Store) EinladungenFuer(pid string) (ein []Einladung, aus []Einladung, err error) {
	rows, err := s.db.Query(
		`SELECT e.id, e.von_id, e.an_id, e.zustand, e.erstellt_am, p.id, p.spitzname
		   FROM einladungen e
		   JOIN players p ON p.id = CASE WHEN e.von_id = ? THEN e.an_id ELSE e.von_id END
		  WHERE e.zustand = 'OFFEN' AND (e.von_id = ? OR e.an_id = ?)
		  ORDER BY e.erstellt_am DESC`, pid, pid, pid)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Einladung
		if err := rows.Scan(&e.ID, &e.VonID, &e.AnID, &e.Zustand, &e.ErstelltAm,
			&e.Gegenueber.ID, &e.Gegenueber.Spitzname); err != nil {
			return nil, nil, err
		}
		if e.AnID == pid {
			ein = append(ein, e)
		} else {
			aus = append(aus, e)
		}
	}
	return ein, aus, rows.Err()
}

// EinladungAnnehmen erzeugt die Partie.
//
// Der Zustandswechsel steht im WHERE, nicht in einem SELECT davor: Die Zahl der
// geaenderten Zeilen entscheidet, ob dieser Aufruf die Annahme war. Sonst
// gewinnen zwei gleichzeitige Annahmen beide und es entstuenden zwei Partien.
func (s *Store) EinladungAnnehmen(eid, pid string) (game.Party, error) {
	var von, an string
	err := s.db.QueryRow(
		`SELECT von_id, an_id FROM einladungen WHERE id = ? AND an_id = ? AND zustand = 'OFFEN'`,
		eid, pid).Scan(&von, &an)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Party{}, ErrNichtGefunden
	}
	if err != nil {
		return game.Party{}, err
	}
	if partner, err := s.SchonPartner(von, an); err != nil {
		return game.Party{}, err
	} else if partner {
		return game.Party{}, ErrSchonPartner
	}

	tx, err := s.db.Begin()
	if err != nil {
		return game.Party{}, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`UPDATE einladungen SET zustand = 'ANGENOMMEN', entschieden_am = ?
		  WHERE id = ? AND an_id = ? AND zustand = 'OFFEN'`, jetzt(), eid, pid)
	if err != nil {
		return game.Party{}, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return game.Party{}, ErrNichtGefunden
	}

	pa := game.Party{ID: id(), A: game.PlayerID(von), B: game.PlayerID(an)}
	// Ohne Code: Eine Partie aus einer Einladung braucht keinen zweiten Weg
	// hinein, sie ist schon vollstaendig.
	if _, err := tx.Exec(
		`INSERT INTO parties (id, code, code_bis, erstellt_am) VALUES (?,NULL,NULL,?)`,
		pa.ID, jetzt()); err != nil {
		return game.Party{}, err
	}
	for seite, x := range map[string]string{"A": von, "B": an} {
		if _, err := tx.Exec(
			`INSERT INTO party_members (party_id, player_id, seite) VALUES (?,?,?)`,
			pa.ID, x, seite); err != nil {
			return game.Party{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE einladungen SET party_id = ? WHERE id = ?`, pa.ID, eid); err != nil {
		return game.Party{}, err
	}
	// Die Gegeneinladung mit schliessen. Haben sich beide gleichzeitig
	// eingeladen, laege sonst in der Lobby eine Einladung, deren Annahme an
	// ErrSchonPartner scheitert - eine Schaltflaeche, die nur Fehler liefert.
	if _, err := tx.Exec(
		`UPDATE einladungen SET zustand = 'ANGENOMMEN', party_id = ?, entschieden_am = ?
		  WHERE von_id = ? AND an_id = ? AND zustand = 'OFFEN'`,
		pa.ID, jetzt(), an, von); err != nil {
		return game.Party{}, err
	}
	return pa, tx.Commit()
}

func (s *Store) EinladungAblehnen(eid, pid string) error {
	return s.einladungSchliessen(eid, pid, "an_id", "ABGELEHNT")
}

func (s *Store) EinladungZurueckziehen(eid, pid string) error {
	return s.einladungSchliessen(eid, pid, "von_id", "ZURUECKGEZOGEN")
}

func (s *Store) einladungSchliessen(eid, pid, spalte, zustand string) error {
	res, err := s.db.Exec(
		`UPDATE einladungen SET zustand = ?, entschieden_am = ?
		  WHERE id = ? AND `+spalte+` = ? AND zustand = 'OFFEN'`,
		zustand, jetzt(), eid, pid)
	if err != nil {
		return err
	}
	// Nicht gefunden statt "kein Zugriff": Die Einladungs-ID soll kein Orakel
	// darueber sein, dass es sie ueberhaupt gibt.
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNichtGefunden
	}
	return nil
}

// EinladungenSchliessen raeumt die offenen Einladungen zwischen zwei Spielern
// weg - aufgerufen, wenn auf anderem Weg eine Partie zwischen ihnen entsteht.
func (s *Store) EinladungenSchliessen(a, b string) error {
	_, err := s.db.Exec(
		`UPDATE einladungen SET zustand = 'ANGENOMMEN', entschieden_am = ?
		  WHERE zustand = 'OFFEN'
		    AND ((von_id = ? AND an_id = ?) OR (von_id = ? AND an_id = ?))`,
		jetzt(), a, b, b, a)
	return err
}
