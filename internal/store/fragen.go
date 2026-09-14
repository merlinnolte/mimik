package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Freiwillige Stimmen zu Fragen, und was sie am Vorrat bewirken.
//
// Der Gedanke: Eine Frage, die niemandem etwas abverlangt, kostet eine ganze
// Runde - fuer zwei Menschen. Eine gute Frage ist dagegen nur wenig besser als
// eine mittlere. Daraus folgt alles Weitere, vor allem die Asymmetrie unten.

// Gewichte fuer das Ziehen.
//
// GewichtNormal ist eine unbewertete Frage, und es ist keine 1: Eine Stimme
// soll verschieben koennen, ohne dass eine einzelne Stimme alles entscheidet.
// Mit 6 als Mitte ist eine abgelehnte Frage sechsmal seltener und eine
// zugesprochene anderthalbmal haeufiger - deutlich, aber nicht endgueltig.
//
// AblehnungWiegt > ZuspruchWiegt, weil der Schaden ungleich verteilt ist: Eine
// schlechte Frage verbrennt eine Runde fuer zwei Menschen, eine gute ist nur
// etwas besser als der Durchschnitt.
//
// GewichtMin ist 1 und nicht 0. Bei zwei Nutzern waere eine Null das Recht
// eines einzelnen Daumens, eine Frage fuer alle zu loeschen - das ist zu viel
// Macht fuer eine Geste, die man auch aus Laune macht. Sechsmal seltener
// reicht, und der Vorrat schrumpft nicht heimlich unter das, was
// internal/seed/fragen_test.go garantiert.
//
// GewichtMax deckelt die andere Seite: Ohne Deckel verdraengte eine Frage, die
// drei Leuten gefiel, alles andere - und die Aufgabe des Vorrats ist Vielfalt.
const (
	GewichtNormal  = 6
	GewichtMin     = 1
	GewichtMax     = 18
	ZuspruchWiegt  = 3
	AblehnungWiegt = 4
)

// Fragengewicht sagt, wie viele Lose eine Frage in die Ziehung legt.
func Fragengewicht(mag, magNicht int) int {
	g := GewichtNormal + ZuspruchWiegt*mag - AblehnungWiegt*magNicht
	if g < GewichtMin {
		return GewichtMin
	}
	if g > GewichtMax {
		return GewichtMax
	}
	return g
}

// gewichtssumme und waehleGewichtet trennen das Wuerfeln vom Waehlen: Das Los
// kommt als Zahl herein, damit die Auswahl ohne Zufallsquelle pruefbar ist.
// Los 0 trifft die erste Frage, summe-1 die letzte.
func gewichtssumme(gewichte []int) int {
	n := 0
	for _, g := range gewichte {
		n += g
	}
	return n
}

func waehleGewichtet(gewichte []int, los int) int {
	for i, g := range gewichte {
		los -= g
		if los < 0 {
			return i
		}
	}
	return len(gewichte) - 1 // nur bei los >= summe erreichbar
}

// ------------------------------------------------------------- Stimmen ---

var ErrFrageNichtImVorrat = errors.New("frage steht nicht mehr im vorrat")

// frageIDVonRunde findet die Frage, die in einer Runde gestellt wurde.
//
// Ueber den TEXT und nicht ueber eine Spalte in rounds: rounds traegt den
// Fragetext als Kopie (damit eine laufende Runde gegen jeden Poolwandel immun
// ist), eine frage_id hat die Tabelle nicht, und eine Spalte nachzuruesten
// hiesse ALTER TABLE. fragen_pool.text ist UNIQUE, der Weg ist also eindeutig.
//
// Er scheitert genau in einem Fall: Die Frage hat den Vorrat verlassen oder ihr
// Text wurde korrigiert, seit die Runde angelegt wurde. Dann gibt es nichts zu
// gewichten, und die Stimme faellt weg - richtig so, aber es gehoert ins
// Protokoll und nicht in eine Fehlermeldung an den Spieler.
func frageIDVonRunde(q abfrager, roundID string) (int64, error) {
	var fid int64
	err := q.QueryRow(
		`SELECT p.id FROM rounds r JOIN fragen_pool p ON p.text = r.frage
		  WHERE r.id = ?`, roundID).Scan(&fid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrFrageNichtImVorrat
	}
	return fid, err
}

type abfrager interface {
	QueryRow(string, ...any) *sql.Row
}

// FrageUrteilen speichert die Stimme eines Spielers zu der Frage einer Runde.
// urteil 0 zieht eine abgegebene Stimme zurueck.
func (s *Store) FrageUrteilen(pid, roundID string, urteil int) error {
	fid, err := frageIDVonRunde(s.db, roundID)
	if err != nil {
		return err
	}
	if urteil == 0 {
		_, err := s.db.Exec(
			`DELETE FROM fragen_urteile WHERE player_id = ? AND frage_id = ?`, pid, fid)
		return err
	}
	if urteil > 0 {
		urteil = 1
	} else {
		urteil = -1
	}
	_, err = s.db.Exec(
		`INSERT INTO fragen_urteile (player_id, frage_id, urteil, erstellt_am)
		 VALUES (?,?,?,?)
		 ON CONFLICT(player_id, frage_id) DO UPDATE SET
		   urteil = excluded.urteil, erstellt_am = excluded.erstellt_am`,
		pid, fid, urteil, jetzt())
	return err
}

// UrteileVonMatch liefert die eigenen Stimmen zu allen Fragen eines Matches,
// nach Runden-ID.
//
// Eine Abfrage fuer das ganze Match und nicht eine je Runde: zustand() laeuft
// alle drei Sekunden und geht dabei ueber sechs bis zwoelf Runden.
func (s *Store) UrteileVonMatch(pid, matchID string) (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT r.id, u.urteil
		   FROM rounds r
		   JOIN fragen_pool p ON p.text = r.frage
		   JOIN fragen_urteile u ON u.frage_id = p.id AND u.player_id = ?
		  WHERE r.match_id = ?`, pid, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aus := map[string]int{}
	for rows.Next() {
		var rid string
		var u int
		if err := rows.Scan(&rid, &u); err != nil {
			return nil, err
		}
		aus[rid] = u
	}
	return aus, rows.Err()
}

// Urteilsstand zaehlt die Stimmen zu einer Frage - fuer Tests und fuer die
// Protokollzeile beim Ziehen.
func (s *Store) Urteilsstand(frageID int64) (mag, magNicht int, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(SUM(urteil > 0), 0), COALESCE(SUM(urteil < 0), 0)
		   FROM fragen_urteile WHERE frage_id = ?`, frageID).Scan(&mag, &magNicht)
	return
}

// FrageIDVonRunde ist frageIDVonRunde fuer Tests und Werkzeuge.
func (s *Store) FrageIDVonRunde(roundID string) (int64, error) {
	fid, err := frageIDVonRunde(s.db, roundID)
	if err != nil {
		return 0, fmt.Errorf("runde %s: %w", roundID, err)
	}
	return fid, nil
}
