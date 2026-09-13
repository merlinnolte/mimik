package store

import (
	"path/filepath"
	"testing"
	"time"
)

// Zwei Spieler sollen verschiedene erste Seiten sehen, derselbe Spieler aber
// immer dieselbe - sonst zeigte "Weitere" Begriffe doppelt oder gar nicht.
func TestVorschlaegeMischenStabil(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a1, _, _ := s.TagVorschlaege("spieler-a", 0, 20)
	a2, _, _ := s.TagVorschlaege("spieler-a", 0, 20)
	b1, _, _ := s.TagVorschlaege("spieler-b", 0, 20)

	for i := range a1 {
		if a1[i] != a2[i] {
			t.Fatalf("derselbe Spieler bekommt zwei Reihenfolgen: %q vs %q", a1[i], a2[i])
		}
	}
	gleich := 0
	for i := range a1 {
		if a1[i] == b1[i] {
			gleich++
		}
	}
	if gleich > 3 {
		t.Fatalf("zwei Spieler sehen %d von 20 Begriffen an derselben Stelle", gleich)
	}

	// Und die zweite Seite überschneidet sich nicht mit der ersten.
	a3, _, _ := s.TagVorschlaege("spieler-a", 20, 20)
	erste := map[string]bool{}
	for _, x := range a1 {
		erste[x] = true
	}
	for _, x := range a3 {
		if erste[x] {
			t.Fatalf("%q steht auf Seite 1 und Seite 2", x)
		}
	}
}

// Der Fortschrittsbalken in der App haengt daran, dass der Server sagt, wie
// lange MIMIK schon arbeitet. Ohne Antwort darf das 0 sein und nicht raten.
func TestArbeitSeit(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := s.ArbeitSeit("gibt-es-nicht"); got != 0 {
		t.Fatalf("ohne Antwort %d Sekunden statt 0", got)
	}
	// Eltern zuerst: answers haengt an rounds und players.
	for _, q := range []string{
		`INSERT INTO players (id, spitzname, erstellt_am) VALUES ('p1','Kim','2026-01-01T00:00:00Z')`,
		`INSERT INTO players (id, spitzname, erstellt_am) VALUES ('p2','Robin','2026-01-01T00:00:00Z')`,
		`INSERT INTO parties (id, code, code_bis, erstellt_am) VALUES ('pa1',NULL,NULL,'2026-01-01T00:00:00Z')`,
		`INSERT INTO matches (id, party_id, erstellt_am) VALUES ('m1','pa1','2026-01-01T00:00:00Z')`,
		`INSERT INTO rounds (id, match_id, nummer, frage, rubrik, geoeffnet_am)
		 VALUES ('r1','m1',1,'Frage?','a','2026-01-01T00:00:00Z')`,
	} {
		if _, err := s.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB().Exec(
		`INSERT INTO answers (round_id, player_id, original, normalform, erstellt_am)
		 VALUES (?,?,?,?,?)`, "r1", "p1", "roh", "glatt",
		time.Now().UTC().Add(-30*time.Second).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	// Eine Antwort allein ist noch keine Arbeit: MIMIK faengt erst an, wenn
	// beide geschrieben haben.
	if got := s.ArbeitSeit("r1"); got != 0 {
		t.Fatalf("mit nur einer Antwort %d Sekunden statt 0", got)
	}
	if _, err := s.DB().Exec(
		`INSERT INTO answers (round_id, player_id, original, normalform, erstellt_am)
		 VALUES (?,?,?,?,?)`, "r1", "p2", "roh", "glatt",
		time.Now().UTC().Add(-5*time.Second).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	// Gezaehlt wird ab der spaeteren Antwort, also ab 5 s - nicht ab 30.
	if got := s.ArbeitSeit("r1"); got < 4 || got > 15 {
		t.Fatalf("die spaetere Antwort ist 5 s alt, gemeldet werden %d", got)
	}
}
