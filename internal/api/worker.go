package api

import (
	"context"
	"log"
	"math/rand/v2"
	"time"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/store"
)

// Worker erzeugt die Fälschungen. Er läuft im Hintergrund, weil das je nach
// Endpunkt Sekunden bis Minuten dauert – gemessen 15 s bis über 3 min. Ein
// Spieler wartet darauf nie synchron: die Runde steht so lange auf
// MIMIK_ARBEITET, und der nächste Abruf von /v1/state sieht die Karten.
type Worker struct {
	S      *store.Store
	M      *mimik.Client
	wecker chan struct{}
}

func NeuerWorker(s *store.Store, m *mimik.Client) *Worker {
	return &Worker{S: s, M: m, wecker: make(chan struct{}, 1)}
}

// Anstossen weckt den Worker, ohne zu blockieren.
func (w *Worker) Anstossen() {
	select {
	case w.wecker <- struct{}{}:
	default:
	}
}

func (w *Worker) Laufen(ctx context.Context) {
	takt := time.NewTicker(20 * time.Second)
	defer takt.Stop()
	for {
		w.durchgang(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wecker:
		case <-takt.C:
		}
	}
}

func (w *Worker) durchgang(ctx context.Context) {
	ids, err := w.S.OffeneRunden()
	if err != nil {
		log.Printf("worker: offene runden: %v", err)
		return
	}
	for _, rid := range ids {
		select {
		case <-ctx.Done():
			return
		default:
		}
		w.rundeBearbeiten(ctx, rid)
	}
}

func (w *Worker) rundeBearbeiten(ctx context.Context, rid string) {
	pa, _, err := w.S.PartyVonRunde(rid)
	if err != nil {
		return
	}
	rd, err := w.S.Runde(rid)
	if err != nil {
		return
	}
	if rd.Ableiten(pa) != game.MimikArbeitet {
		return // noch nicht beide geschrieben, oder längst fertig
	}
	for _, ueber := range []game.PlayerID{pa.A, pa.B} {
		if len(rd.Karten[ueber]) == 4 {
			continue
		}
		if err := w.kartenBauen(ctx, rd, pa, ueber); err != nil {
			log.Printf("worker: runde %s über %s: %v", rid[:8], ueber, err)
			w.S.ZustandSetzen(rid, game.MimikArbeitet, err.Error())
			return // beim nächsten Takt erneut versuchen
		}
		rd, _ = w.S.Runde(rid)
	}
	if len(rd.Karten[pa.A]) == 4 && len(rd.Karten[pa.B]) == 4 {
		w.S.ZustandSetzen(rid, game.Raten, "")
		log.Printf("worker: runde %s ist zum raten offen", rid[:8])
	}
}

func (w *Worker) kartenBauen(ctx context.Context, rd game.Runde, pa game.Party, ueber game.PlayerID) error {
	// Das Modell bekommt den rohen Text: Es soll den Stil sehen, bevor es ihn
	// nachmacht. Die saubere Fassung schreibt es selbst und liefert sie zurück.
	roh := rd.Antworten[ueber].Original
	tags, err := w.S.Tags(string(ueber))
	if err != nil {
		return err
	}
	gesperrt, err := w.S.GesperrteThemen(string(ueber))
	if err != nil {
		return err
	}
	anker, gestrichen := mimik.AnkerWaehlen(tags, gesperrt, roh, func(xs []string) {
		rand.Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
	})
	if len(anker) == 0 {
		anker = []string{"alltag", "erinnerung", "vorliebe"} // Notnagel, falls alles verbraucht ist
	}
	fakten, _ := w.S.Fakten(string(ueber), 40)
	verbraucht := make([]string, 0, len(gesperrt))
	for t := range gesperrt {
		verbraucht = append(verbraucht, t)
	}

	ctx, abbruch := context.WithTimeout(ctx, 4*time.Minute)
	defer abbruch()
	erg, err := w.M.Faelschungen(ctx, rd.Frage, roh, anker, mimik.Dossier{
		Fakten:   fakten,
		Gesperrt: verbraucht,
		AntiBeispiele: []string{
			"Das ist eine spannende Frage! Ich würde sagen ...",
			"Am Ende zählt doch, dass man glücklich ist.",
		},
	})
	if err != nil {
		return err
	}

	// Die vom Modell geschriebene Normalform ersetzt die regelbasierte
	// Notfassung, die beim Absenden gespeichert wurde. Erst danach die Karten
	// bauen – die echte Karte IST die Normalform.
	if erg.Normalform != "" && erg.Normalform != rd.Antworten[ueber].Normalform {
		if err := w.S.NormalformSetzen(rd.ID, string(ueber), erg.Normalform); err != nil {
			return err
		}
		a := rd.Antworten[ueber]
		a.Normalform = erg.Normalform
		rd.Antworten[ueber] = a
	}

	// Karten mischen und die Reihenfolge einfrieren.
	if err := rd.KartenSetzen(ueber, erg.Faelschungen, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))); err != nil {
		return err
	}
	if err := w.S.KartenSpeichern(rd.ID, string(ueber), rd.Karten[ueber]); err != nil {
		return err
	}
	// Der Fakt wandert ins Dossier, das Thema auf die Sperrliste.
	w.S.FaktHinzu(string(ueber), rd.ID, erg.Fakt)
	w.S.ThemenSperren(string(ueber), rd.ID, erg.Sperre)
	log.Printf("worker: %s über %s fertig (versuche=%d, abstand=%.2f, gestrichen=%v)",
		rd.ID[:8], ueber, erg.Versuche, erg.Befund.MaxZuEcht, gestrichen)
	return nil
}
