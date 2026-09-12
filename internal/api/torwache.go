package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"sync"
	"time"
)

// Torwache schützt den einzigen Endpunkt ohne Token: POST /v1/devices.
//
// Warum überhaupt: Der Server steht im Netz, und wer sich ein Gerät anlegen
// kann, kann sich auch eine Party mit sich selbst bauen und ein Match starten.
// Jede Runde darin sind zwei Modellaufrufe auf Rechnung des Betreibers. Für ein
// Spiel für zwei Personen ist ein offenes Anmelden deshalb keine Freundlichkeit,
// sondern eine Rechnung, die jemand anders schreibt.
//
// Zwei Riegel, beide klein:
//
//   - Ein Einladungsgeheimnis aus MIMIK_EINLADUNG. Ist es gesetzt, muss jede
//     Anmeldung es mitschicken. Ist es leer, bleibt der Server offen – dann
//     warnt er beim Start.
//   - Eine Obergrenze je Adresse, damit auch ein offener Server nicht in einer
//     Minute hundert Konten bekommt.
//
// Hinter einem Reverse Proxy ist RemoteAddr die Adresse des Proxys. Dann zählt
// die Grenze für alle gemeinsam; das ist gewollt konservativ, denn eine
// weitergereichte Adresse kann sich der Klient selbst ausdenken.
type Torwache struct {
	Einladung string

	sperre  sync.Mutex
	zugriff map[string][]time.Time
}

// MaxAnmeldungen je Adresse und Fenster. Zwei Personen, zwei Geräte, ein paar
// Fehlversuche – zehn pro Stunde sind großzügig und trotzdem eine Grenze.
const (
	MaxAnmeldungen = 10
	Fenster        = time.Hour
)

func (t *Torwache) Offen() bool { return t.Einladung == "" }

// Passt vergleicht in konstanter Zeit. Ein Vergleich mit == verrät über die
// Laufzeit, wie viele Zeichen stimmen.
func (t *Torwache) Passt(gegeben string) bool {
	if t.Offen() {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(gegeben), []byte(t.Einladung)) == 1
}

// Darf zählt einen Versuch und sagt, ob er noch im Rahmen liegt.
func (t *Torwache) Darf(r *http.Request) bool {
	adresse, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		adresse = r.RemoteAddr
	}
	jetzt := time.Now()

	t.sperre.Lock()
	defer t.sperre.Unlock()
	if t.zugriff == nil {
		t.zugriff = map[string][]time.Time{}
	}
	// Beim Aufräumen alle Adressen mitnehmen, nicht nur die eigene: Sonst
	// wächst die Karte mit jeder Adresse, die einmal vorbeikam.
	for a, zs := range t.zugriff {
		frisch := zs[:0]
		for _, z := range zs {
			if jetzt.Sub(z) < Fenster {
				frisch = append(frisch, z)
			}
		}
		if len(frisch) == 0 {
			delete(t.zugriff, a)
		} else {
			t.zugriff[a] = frisch
		}
	}
	if len(t.zugriff[adresse]) >= MaxAnmeldungen {
		return false
	}
	t.zugriff[adresse] = append(t.zugriff[adresse], jetzt)
	return true
}
