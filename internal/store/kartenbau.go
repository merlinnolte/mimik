package store

import (
	"database/sql"
	"errors"
	"math"
	"time"
)

const (
	// MaxKartenversuche: Danach nur noch stuendlich. Kein harter Stopp - der
	// parkte die Runde fuer immer und blockierte das Match, und das ist fuer
	// den Menschen davor schlimmer als eine schwache Karte.
	MaxKartenversuche = 8
	// KartenbauGrundwarte ist die erste Wartezeit; sie verdoppelt sich je
	// Versuch bis hoechstens einer Stunde. Die ersten Versuche kommen damit
	// fast unveraendert schnell.
	KartenbauGrundwarte = 20 * time.Second
	KartenbauMaxwarte   = time.Hour
)

// KartenbauBeanspruchen zaehlt einen Versuch und setzt den naechsten Zeitpunkt.
//
// Vor dem Aufruf, nicht danach: Ein Absturz mitten im Modellaufruf kostet damit
// einen Versuch statt einer Endlosschleife - dasselbe Argument wie bei
// ReviewBeanspruchen.
func (s *Store) KartenbauBeanspruchen(rid, ueber string) error {
	var versuche int
	err := s.db.QueryRow(
		`INSERT INTO kartenbau (round_id, ueber, versuche, erstellt_am) VALUES (?,?,1,?)
		 ON CONFLICT(round_id, ueber) DO UPDATE SET versuche = versuche + 1
		 RETURNING versuche`, rid, ueber, jetzt()).Scan(&versuche)
	if err != nil {
		return err
	}
	warte := KartenbauGrundwarte * time.Duration(math.Pow(2, float64(versuche-1)))
	if warte > KartenbauMaxwarte || versuche >= MaxKartenversuche {
		warte = KartenbauMaxwarte
	}
	_, err = s.db.Exec(
		`UPDATE kartenbau SET naechster_versuch_am = ? WHERE round_id = ? AND ueber = ?`,
		time.Now().UTC().Add(warte).Format(time.RFC3339), rid, ueber)
	return err
}

// KartenbauFertig raeumt den Zaehler ab. Die Karten stehen, die Zeile hat ihren
// Zweck erfuellt.
func (s *Store) KartenbauFertig(rid, ueber string) error {
	_, err := s.db.Exec(`DELETE FROM kartenbau WHERE round_id = ? AND ueber = ?`, rid, ueber)
	return err
}

func (s *Store) KartenbauGescheitert(rid, ueber, meldung string) error {
	_, err := s.db.Exec(
		`UPDATE kartenbau SET fehler = ? WHERE round_id = ? AND ueber = ?`, meldung, rid, ueber)
	return err
}

// KartenbauWartet sagt, ob dieser Kartensatz gerade zurueckgestellt ist.
func (s *Store) KartenbauWartet(rid, ueber string) bool {
	var bis string
	err := s.db.QueryRow(
		`SELECT naechster_versuch_am FROM kartenbau WHERE round_id = ? AND ueber = ?`,
		rid, ueber).Scan(&bis)
	if errors.Is(err, sql.ErrNoRows) || err != nil || bis == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, bis)
	return err == nil && time.Now().UTC().Before(t)
}
