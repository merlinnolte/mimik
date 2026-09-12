package store

import (
	"path/filepath"
	"testing"
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
