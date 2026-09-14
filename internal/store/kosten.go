package store

import (
	"math"
	"time"

	"mimik/internal/mimik"
)

// AufrufBuchen schreibt die Rechnung eines Modellaufrufs mit.
//
// Wofuer: Die Frage "was kostet eigentlich ein Match" war bis zum 14.09.2026
// nicht beantwortbar - der Klient las das usage-Objekt gar nicht. Jetzt steht
// sie in einer Tabelle, und die Summe ist eine Abfrage statt einer Schaetzung.
func (s *Store) AufrufBuchen(v mimik.Verbrauch) error {
	_, err := s.db.Exec(
		`INSERT INTO aufrufe
		   (round_id, zweck, eingabe, ausgabe, denkspur, cache_treffer, zeichen,
		    sekunden, erstellt_am)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		v.RundeID, v.Zweck, v.Eingabe, v.Ausgabe, v.Denkspur, v.CacheTreffer,
		v.Zeichen, v.Sekunden, jetzt())
	return err
}

// AufrufeBuchen bucht mehrere - etwa die Wiederholungen eines Kartenbaus.
func (s *Store) AufrufeBuchen(vs []mimik.Verbrauch) {
	for _, v := range vs {
		// Ein Fehler beim Buchen darf keine Runde kosten: Die Rechnung ist
		// Beiwerk, das Spiel ist die Sache.
		s.AufrufBuchen(v)
	}
}

// Kostenstand ist die Summe ueber einen Zeitraum, fuer den Blick von aussen.
type Kostenstand struct {
	Aufrufe      int     `json:"aufrufe"`
	Eingabe      int     `json:"eingabe"`
	Ausgabe      int     `json:"ausgabe"`
	Denkspur     int     `json:"denkspur"`
	CacheTreffer int     `json:"cache_treffer"`
	Cachequote   float64 `json:"cachequote"`
}

func (s *Store) Kosten(seit time.Time) (Kostenstand, error) {
	var k Kostenstand
	err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(eingabe),0), COALESCE(SUM(ausgabe),0),
		        COALESCE(SUM(denkspur),0), COALESCE(SUM(cache_treffer),0)
		   FROM aufrufe WHERE erstellt_am >= ?`,
		seit.UTC().Format(time.RFC3339)).
		Scan(&k.Aufrufe, &k.Eingabe, &k.Ausgabe, &k.Denkspur, &k.CacheTreffer)
	if err != nil {
		return k, err
	}
	if k.Eingabe > 0 {
		k.Cachequote = math.Round(100*float64(k.CacheTreffer)/float64(k.Eingabe)) / 100
	}
	return k, nil
}
