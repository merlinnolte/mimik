package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
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
// die Grenze für ALLE GEMEINSAM – und genau das ist im Betrieb passiert: Eine
// eingeladene Person bekam "zu viele anmeldungen von dieser adresse", weil
// jemand anders die zehn Versuche längst verbraucht hatte.
//
// Weitergereichte Adressen sind trotzdem nichts, was man einfach glaubt: Steht
// der Server nackt im Netz, kann sich jeder Klient ein X-Forwarded-For
// ausdenken und die Grenze damit aushebeln. Deshalb ein ausdrücklicher
// Schalter: Hops sagt, wie viele Proxys DAVOR stehen. Erst dann wird gezählt,
// und zwar von rechts – der letzte Eintrag stammt vom eigenen Proxy, alles
// weiter links kann gefälscht sein.
type Torwache struct {
	Einladung string
	Hops      int

	sperre  sync.Mutex
	zugriff map[string][]time.Time
}

// Grenzen je Adresse und Fenster.
//
// Mit gültiger Einladung darf es mehr sein: Das Geheimnis IST der Riegel, die
// Zählung ist nur der Schutz für einen offenen Server. Ein Haushalt teilt sich
// eine Adresse, und zwei Menschen mit drei Geräten und ein paar Fehlversuchen
// sollen nicht an einer Zahl scheitern, die gegen jemand anderen gedacht war.
const (
	MaxAnmeldungen  = 10
	MaxMitEinladung = 40
	Fenster         = time.Hour
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

// Adresse ermittelt, wer da anfragt.
//
// Ohne Hops: die Gegenstelle, sonst nichts. Mit Hops: der Eintrag, den der
// eigene Proxy geschrieben hat – von rechts gezählt, weil ein Klient die linken
// Einträge selbst mitschicken kann.
func (t *Torwache) Adresse(r *http.Request) string {
	direkt, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		direkt = r.RemoteAddr
	}
	if t.Hops <= 0 {
		return direkt
	}
	kette := r.Header.Values("X-Forwarded-For")
	var teile []string
	for _, k := range kette {
		for _, x := range strings.Split(k, ",") {
			if x = strings.TrimSpace(x); x != "" {
				teile = append(teile, x)
			}
		}
	}
	i := len(teile) - t.Hops
	if i < 0 || i >= len(teile) {
		// Weniger Einträge als angesagte Proxys: Die Kette passt nicht zur
		// Einstellung. Dann lieber die Gegenstelle als eine geratene Adresse.
		return direkt
	}
	return teile[i]
}

// Darf zählt einen Versuch und sagt, ob er noch im Rahmen liegt.
func (t *Torwache) Darf(r *http.Request, eingeladen bool) bool {
	adresse := t.Adresse(r)
	grenze := MaxAnmeldungen
	if eingeladen && !t.Offen() {
		grenze = MaxMitEinladung
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
	if len(t.zugriff[adresse]) >= grenze {
		return false
	}
	t.zugriff[adresse] = append(t.zugriff[adresse], jetzt)
	return true
}
