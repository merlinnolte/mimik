package api

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
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
	// Erst die Testspieler ziehen lassen, dann die Karten bauen: Sonst stünde
	// eine Testpartie nach jedem Zug eine Taktlänge still.
	w.botzuege(ctx)

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

// botzuege spielt die Zuege des Testspielers: antworten und raten.
//
// Er ist ein ganz normaler Spieler – dieselben Endpunkte waeren es auch, wenn er
// ein Telefon haette. Deshalb laeuft alles ueber dieselben Regeln in
// internal/game; hier steht nur, WANN er dran ist.
func (w *Worker) botzuege(ctx context.Context) {
	ids, err := w.S.BotRunden()
	if err != nil {
		log.Printf("worker: botrunden: %v", err)
		return
	}
	for _, rid := range ids {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := w.botzug(ctx, rid); err != nil {
			log.Printf("worker: testspieler in runde %s: %v", rid[:8], err)
		}
	}
}

func (w *Worker) botzug(ctx context.Context, rid string) error {
	pa, m, err := w.S.PartyVonRunde(rid)
	if err != nil {
		return err
	}
	var bot game.PlayerID
	for _, x := range []game.PlayerID{pa.A, pa.B} {
		if w.S.IstBot(string(x)) {
			bot = x
		}
	}
	if bot == "" {
		return nil
	}
	rd, err := w.S.Runde(rid)
	if err != nil {
		return err
	}

	// Antworten.
	if _, hat := rd.Antworten[bot]; !hat {
		fakten, _ := w.S.Fakten(string(bot), 12)
		tags, _ := w.S.Tags(string(bot))
		ctx, abbruch := context.WithTimeout(ctx, 4*time.Minute)
		defer abbruch()
		text, err := w.M.BotAntwort(ctx, rd.Frage, tags, fakten)
		if err != nil {
			return err
		}
		a := game.Antwort{Original: text, Normalform: mimik.ErsatzNormalform(text)}
		if err := rd.AntwortAbgeben(pa, bot, a); err != nil {
			return err
		}
		if err := w.S.AntwortSpeichern(rid, string(bot), a); err != nil {
			return err
		}
		w.S.ZustandSetzen(rid, rd.Ableiten(pa), "")
		log.Printf("worker: testspieler antwortet in %s: %q", rid[:8], text)
		return nil
	}

	// Raten. Bewusst gewuerfelt und nicht vom Modell: Geprueft werden soll der
	// Ablauf fuer den Menschen davor, nicht wie gut ein Modell raet – und jeder
	// Modellaufruf kostet hier eine weitere Minute.
	if _, hat := rd.Tipps[bot]; !hat && len(rd.KartenFuer(pa, bot)) == 4 {
		pos := 1 + rand.IntN(4)
		if _, err := rd.TippAbgeben(pa, bot, pos); err != nil {
			return err
		}
		if _, _, err := w.S.TippSpeichern(m, pa, rd, string(bot)); err != nil {
			return err
		}
		w.S.ZustandSetzen(rid, rd.Ableiten(pa), "")
		log.Printf("worker: testspieler tippt in %s auf %d", rid[:8], pos)
	}
	return nil
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
	// Die beiden Kartensätze NEBENEINANDER bauen, nicht nacheinander.
	//
	// Sie hängen nicht voneinander ab: Der Satz über A entsteht aus As Antwort
	// und As Dossier, der über B aus Bs. Nacheinander dauerte eine Runde so
	// lange wie beide Aufrufe zusammen – bei gemessenen 15 bis 190 Sekunden je
	// Aufruf also bis zu sechs Minuten, und mit Wiederholungen mehr. Parallel
	// dauert sie so lange wie der langsamere von beiden.
	offen := make([]game.PlayerID, 0, 2)
	for _, ueber := range []game.PlayerID{pa.A, pa.B} {
		if len(rd.Karten[ueber]) != 4 {
			offen = append(offen, ueber)
		}
	}

	var wg sync.WaitGroup
	fehler := make([]error, len(offen))
	for i, ueber := range offen {
		wg.Add(1)
		go func(i int, ueber game.PlayerID) {
			defer wg.Done()
			fehler[i] = w.kartenBauen(ctx, rid, pa, ueber)
		}(i, ueber)
	}
	wg.Wait()

	for i, err := range fehler {
		if err != nil {
			log.Printf("worker: runde %s über %s: %v", rid[:8], offen[i], err)
			w.S.ZustandSetzen(rid, game.MimikArbeitet, err.Error())
			return // beim nächsten Takt erneut versuchen
		}
	}

	// Neu einlesen: Beide Goroutinen haben in die Datenbank geschrieben, die
	// Kopie in rd kennt davon nichts.
	rd, err = w.S.Runde(rid)
	if err != nil {
		return
	}
	if len(rd.Karten[pa.A]) == 4 && len(rd.Karten[pa.B]) == 4 {
		w.S.ZustandSetzen(rid, game.Raten, "")
		log.Printf("worker: runde %s ist zum raten offen", rid[:8])
	}
}

// kartenBauen lädt die Runde selbst, statt eine mitgereichte Kopie zu benutzen.
//
// game.Runde trägt Maps (Karten, Antworten). Zwei Goroutinen mit derselben
// Struktur schrieben in dieselben Maps – nebenläufige Map-Schreibzugriffe sind
// in Go kein Fehlerwert, sondern ein Absturz des ganzen Prozesses. Jede
// Goroutine bekommt deshalb ihre eigene Runde.
func (w *Worker) kartenBauen(ctx context.Context, rid string, pa game.Party, ueber game.PlayerID) error {
	rd, err := w.S.Runde(rid)
	if err != nil {
		return err
	}
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
	// Zwölf statt vierzig Fakten. Das Dossier wächst mit jeder Runde, und jeder
	// Fakt geht in jeden Prompt – bei vierzig Fakten zu je bis zu 300 Zeichen
	// wären das zwölftausend Zeichen, die mit jeder Runde länger brauchen und
	// irgendwann den Kontext sprengen. Die jüngsten zwölf sagen über einen
	// Menschen genug; was älter ist, steckt ohnehin in den gesperrten Themen.
	fakten, _ := w.S.Fakten(string(ueber), 12)
	verbraucht := make([]string, 0, len(gesperrt))
	for t := range gesperrt {
		verbraucht = append(verbraucht, t)
	}

	ctx, abbruch := context.WithTimeout(ctx, 4*time.Minute)
	defer abbruch()
	erg, err := w.M.Faelschungen(ctx, rd.Frage, roh, mimik.Dossier{
		Interessen: tags,
		Fakten:     fakten,
		Gesperrt:   verbraucht,
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
	richtungen := make([]string, 0, 3)
	for _, f := range erg.Faelschungen {
		richtungen = append(richtungen, f.Richtung)
	}
	log.Printf("worker: %s über %s fertig (versuche=%d, abstand=%.2f, richtungen=%v)",
		rd.ID[:8], ueber, erg.Versuche, erg.Befund.MaxZuEcht, richtungen)
	return nil
}
