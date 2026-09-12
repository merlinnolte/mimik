package sicher

import "testing"

func TestText(t *testing.T) {
	faelle := []struct{ name, ein, aus string }{
		{"bildschirm löschen", "harmlos\x1b[2Jweg", "harmlosweg"},
		{"farbe", "\x1b[31mrot\x1b[0m", "rot"},
		{"terminal-hyperlink", "vor\x1b]8;;http://boese.example\x07klick\x1b]8;;\x07nach",
			"vorklicknach"},
		{"leserichtung umkehren", "Kaffee‮eeffaK", "KaffeeeeffaK"},
		{"unsichtbare marke", "Kaf​fee", "Kaffee"},
		{"zeilenumbruch wird leerzeichen", "eine\nzweite", "eine zweite"},
		{"rücktaste", "abc\x08def", "abcdef"},
		{"umlaute bleiben", "Küche, groß – schön!", "Küche, groß – schön!"},
		{"emoji bleibt", "läuft 🙂", "läuft 🙂"},
	}
	for _, f := range faelle {
		if got := Text(f.ein, 400); got != f.aus {
			t.Errorf("%s: %q -> %q, erwartet %q", f.name, f.ein, got, f.aus)
		}
	}
}

func TestLaengeInRunen(t *testing.T) {
	// Zehn Umlaute sind zwanzig Bytes; gezählt werden muss trotzdem in Zeichen,
	// sonst kappt die Grenze mitten in einer Rune.
	if got := Text("ääääääääää", 5); got != "ääää…" {
		t.Errorf("gekürzt zu %q", got)
	}
	if got := Text("kurz", 100); got != "kurz" {
		t.Errorf("unnötig angefasst: %q", got)
	}
}

func TestProtokollzeileBleibtEinzeilig(t *testing.T) {
	ein := "/v1/rounds/\x1b[2K\rGET /admin gefälschte zeile"
	got := Protokoll(ein)
	for _, r := range got {
		if r < 0x20 {
			t.Fatalf("steuerzeichen überlebt: %q", got)
		}
	}
}
