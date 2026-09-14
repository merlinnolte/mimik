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

// rundeBearbeiten baut die Kartensätze, die gebaut werden können.
//
// Und zwar, sobald der jeweilige Mensch geantwortet hat – nicht erst, wenn beide
// fertig sind. Der Satz über A entsteht aus As Antwort und As Dossier; auf B
// wartet er für nichts. Vorher stieg diese Funktion aus, solange die Runde nicht
// auf MIMIK_ARBEITET stand, also bis zur letzten Antwort. Damit lagen beide
// Aufrufe HINTER dem zweiten Menschen, und der sah die volle Wartezeit.
//
// Jetzt läuft der erste Aufruf, während die andere Seite noch nachdenkt – in
// einem Spiel ohne Uhr können das Stunden sein. Wer als Zweiter absendet, wartet
// meist nur noch auf seinen eigenen Satz.
func (w *Worker) rundeBearbeiten(ctx context.Context, rid string) {
	pa, _, err := w.S.PartyVonRunde(rid)
	if err != nil {
		return
	}
	rd, err := w.S.Runde(rid)
	if err != nil {
		return
	}
	if rd.Ableiten(pa) == game.Aufgeloest {
		return
	}

	// Wer geantwortet hat und noch keine vier Karten hat, ist dran.
	offen := make([]game.PlayerID, 0, 2)
	for _, ueber := range []game.PlayerID{pa.A, pa.B} {
		if _, hat := rd.Antworten[ueber]; !hat {
			continue
		}
		if len(rd.Karten[ueber]) != 4 {
			// Zurueckgestellt heisst zurueckgestellt. Ohne diese Zeile ruft
			// eine dauerhaft scheiternde Runde alle 20 Sekunden erneut an.
			if w.S.KartenbauWartet(rid, string(ueber)) {
				continue
			}
			offen = append(offen, ueber)
		}
	}
	if len(offen) == 0 {
		return
	}

	// Die Sätze NEBENEINANDER bauen, nicht nacheinander. Sie hängen nicht
	// voneinander ab; nacheinander dauerte eine Runde so lange wie beide
	// Aufrufe zusammen.
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

	// Neu einlesen: Die Goroutinen haben in die Datenbank geschrieben, die
	// Kopie in rd kennt davon nichts.
	rd, err = w.S.Runde(rid)
	if err != nil {
		return
	}
	neu := rd.Ableiten(pa)

	for i, e := range fehler {
		if e != nil {
			log.Printf("worker: runde %s über %s: %v", rid[:8], offen[i], e)
			w.S.ZustandSetzen(rid, neu, e.Error())
			return // beim nächsten Takt erneut versuchen
		}
	}
	w.S.ZustandSetzen(rid, neu, "")
	if neu == game.Raten {
		log.Printf("worker: runde %s ist zum raten offen", rid[:8])
	}
}

func (w *Worker) kartenBauen(ctx context.Context, rid string, pa game.Party, ueber game.PlayerID) error {
	rd, err := w.S.Runde(rid)
	if err != nil {
		return err
	}
	if err := w.S.KartenbauBeanspruchen(rid, string(ueber)); err != nil {
		return err
	}
	// Das Modell bekommt den rohen Text: Es soll den Stil sehen, bevor es ihn
	// nachmacht. Die saubere Fassung schreibt es selbst und liefert sie zurück.
	roh := rd.Antworten[ueber].Original
	tags, err := w.S.Tags(string(ueber))
	if err != nil {
		return err
	}
	// Grosszuegig laden, dann auswaehlen: Welche mitfahren, entscheidet
	// ThemenFuerPrompt nach Naehe zur Frage.
	gesperrt, err := w.S.GesperrteThemen(string(ueber), 120)
	if err != nil {
		return err
	}
	// Zwölf statt vierzig Fakten. Das Dossier wächst mit jeder Runde, und jeder
	// Fakt geht in jeden Prompt – bei vierzig Fakten zu je bis zu 300 Zeichen
	// wären das zwölftausend Zeichen, die mit jeder Runde länger brauchen und
	// irgendwann den Kontext sprengen. Die jüngsten zwölf sagen über einen
	// Menschen genug; was älter ist, steckt ohnehin in den gesperrten Themen.
	fakten, _ := w.S.Fakten(string(ueber), 12)
	// Nur, was mehr als eine beilaeufige Vermutung ist. Ein Merkmal mit 0.2
	// Konfidenz in den Prompt zu geben heisst, drei Faelschungen auf einen
	// Muenzwurf zu bauen.
	merkmale, _ := w.S.ProfilFuerModell(string(ueber))
	profil := mimik.ProfilZeilen(merkmale, mimik.ProfilSchwelle)
	verbraucht := mimik.ThemenFuerPrompt(gesperrt, rd.Frage, MaxThemenImPrompt)

	ctx, abbruch := context.WithTimeout(ctx, 4*time.Minute)
	defer abbruch()
	ctx = mimik.MitKennung(ctx, "faelschungen", rid)
	erg, err := w.M.Faelschungen(ctx, rd.Frage, roh, mimik.Dossier{
		Interessen: tags,
		Fakten:     fakten,
		Profil:     profil,
		Gesperrt:   verbraucht,
		AntiBeispiele: []string{
			"Das ist eine spannende Frage! Ich würde sagen ...",
			"Am Ende zählt doch, dass man glücklich ist.",
		},
	})
	// Gebucht wird IMMER, auch wenn der Aufruf nichts Brauchbares lieferte:
	// bezahlt ist er trotzdem, und genau diese Aufrufe will man sehen.
	w.S.AufrufeBuchen(erg.Verbrauch)
	if err != nil {
		w.S.KartenbauGescheitert(rid, string(ueber), err.Error())
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
	// Streuung und Grund gehoeren mit ins Protokoll: Steigt die Zahl der
	// Versuche oder sinkt die Streuung, war eine Aenderung am Prompt schlecht -
	// und ohne diese Zahlen merkt es niemand.
	log.Printf("worker: %s über %s fertig (versuche=%d, abstand=%.2f, streuung=%.2f, richtungen=%v)",
		rd.ID[:8], ueber, erg.Versuche, erg.Befund.MaxZuEcht,
		erg.Befund.MittelPeers-erg.Befund.MaxZuEcht, richtungen)
	return w.S.KartenbauFertig(rid, string(ueber))
}

// ----------------------------------------------------------------- Lernen ---

// MaxThemenImPrompt deckelt den Block der verbrauchten Themen.
const MaxThemenImPrompt = 16

// BuendelJeTakt deckelt, wie viele Buendel ein Takt abarbeitet.
const BuendelJeTakt = 3

// Nacheinander, nicht nebenlaeufig: Zwei Reviews ueber DENSELBEN Spieler
// wuerden sonst gleichzeitig dasselbe Merkmal verrechnen, und ein Beleg zaehlte
// doppelt. Nebenlaeufigkeit spart hier ohnehin nichts - auf ein Review wartet
// niemand.

// LernenLaufen wertet aufgeloeste Runden aus.
//
// Eine eigene Schleife statt eines Schritts in durchgang(): Ein Review kann
// Minuten dauern und hielte dort den Kartenbau der naechsten Runde auf - und
// auf den wartet ein Mensch. Auf ein Review wartet keiner, deshalb auch der
// langsamere Takt.
func (w *Worker) LernenLaufen(ctx context.Context) {
	takt := time.NewTicker(60 * time.Second)
	defer takt.Stop()
	kosten := time.NewTicker(time.Hour)
	defer kosten.Stop()
	for {
		w.Reviews(ctx)
		select {
		case <-ctx.Done():
			return
		case <-kosten.C:
			w.kostenzeile()
		case <-takt.C:
		}
	}
}

// kostenzeile schreibt einmal je Stunde die Rechnung ins Protokoll.
//
// Wofuer: Die Tabelle "aufrufe" hat alles, aber im Container liegt kein
// sqlite3, und wer die Rechnung nur nach dem Monatsende sieht, sieht sie zu
// spaet. Eine Zeile je Stunde genuegt, um zu merken, dass etwas aus dem Ruder
// laeuft - und die Cachequote sagt, ob der Prefix-Cache noch traegt.
func (w *Worker) kostenzeile() {
	k, err := w.S.Kosten(time.Now().Add(-time.Hour))
	if err != nil || k.Aufrufe == 0 {
		return
	}
	log.Printf("kosten (1h): %d aufrufe, %d token ein (%.0f%% aus dem cache), %d aus (%d denkspur)",
		k.Aufrufe, k.Eingabe, k.Cachequote*100, k.Ausgabe, k.Denkspur)
}

// Reviews arbeitet ein Buendel ab. Oeffentlich, damit ein Test es direkt
// anstossen kann, statt auf den Takt zu warten.
func (w *Worker) Reviews(ctx context.Context) {
	// Mehrere Buendel je Takt, aber gedeckelt: Bei zwei Spielern haette ein
	// einziges Buendel je Takt den zweiten eine Minute warten lassen - und der
	// Deckel ist die Rechnung, die daran haengt.
	for i := 0; i < BuendelJeTakt; i++ {
		buendel, err := w.S.ReviewBuendel()
		if err != nil {
			log.Printf("worker: offene reviews: %v", err)
			return
		}
		if len(buendel) == 0 {
			return
		}
		if err := w.review(ctx, buendel); err != nil {
			log.Printf("worker: review %s (+%d): %v",
				kurz(buendel[0].RundeID), len(buendel)-1, err)
			for _, a := range buendel {
				w.S.ReviewGescheitert(a.RundeID, a.Ueber, err.Error())
			}
			return
		}
	}
}

// review wertet mehrere Runden EINES Spielers in einem Aufruf aus.
func (w *Worker) review(ctx context.Context, buendel []store.Reviewauftrag) error {
	ueberID := buendel[0].Ueber
	ueber := game.PlayerID(ueberID)

	var runden []mimik.Reviewrunde
	var genommen []store.Reviewauftrag
	for _, a := range buendel {
		if a.Ueber != ueberID {
			// Ein Buendel gehoert einem Spieler. Zwei zu mischen heisst,
			// Merkmale dem falschen Menschen zuzuschreiben - und ein falsch
			// zugeordnetes Merkmal verdirbt zwei Profile auf einmal.
			continue
		}
		darf, err := w.S.ReviewBeanspruchen(a.RundeID, a.Ueber)
		if err != nil {
			return err
		}
		if !darf {
			continue
		}
		pa, _, err := w.S.PartyVonRunde(a.RundeID)
		if err != nil {
			return err
		}
		rd, err := w.S.Runde(a.RundeID)
		if err != nil {
			return err
		}
		tipp, ok := rd.Tipps[pa.Gegner(ueber)]
		if !ok {
			continue
		}
		runden = append(runden, mimik.Reviewrunde{
			RundeID:  a.RundeID,
			Frage:    rd.Frage,
			Antwort:  rd.Antworten[ueber].Normalform,
			Karten:   rd.Karten[ueber],
			Gewaehlt: tipp.Gewaehlt,
			Richtig:  tipp.Richtig,
		})
		genommen = append(genommen, a)
	}
	if len(runden) == 0 {
		return nil
	}

	merkmale, err := w.S.ProfilFuerModell(ueberID)
	if err != nil {
		return err
	}

	// Kuerzer als die vier Minuten des Kartenbaus: Der Prompt ist deutlich
	// kleiner, und ein haengendes Review kostet nur Rechenzeit.
	ctx, abbruch := context.WithTimeout(ctx, 3*time.Minute)
	defer abbruch()
	ctx = mimik.MitKennung(ctx, "review", runden[0].RundeID)
	erg, verbrauch, err := w.M.Review(ctx, mimik.Reviewmaterial{
		Runden: runden,
		Profil: merkmale,
	})
	w.S.AufrufBuchen(verbrauch)
	if err != nil {
		return err
	}
	// Der Verlauf haengt an der juengsten Runde des Buendels - sie ist die, aus
	// der die Beobachtung am ehesten stammt.
	juengste := runden[len(runden)-1].RundeID
	aend := mimik.ProfilVerrechnen(merkmale, erg.Urteile, juengste)
	if err := w.S.ProfilAnwenden(ueberID, juengste, aend); err != nil {
		return err
	}
	// Nur die Schluessel ins Protokoll, nie die Werte: Das Betriebsprotokoll
	// braucht die Vermutung ueber das Alter eines Menschen nicht.
	namen := make([]string, 0, len(aend))
	for _, x := range aend {
		namen = append(namen, x.Merkmal.Merkmal+"/"+x.Urteil)
	}
	log.Printf("worker: review über %s, %d runden: %v", kurz(ueberID), len(runden), namen)
	for _, a := range genommen {
		if err := w.S.ReviewFertig(a.RundeID, a.Ueber); err != nil {
			return err
		}
	}
	return nil
}

func kurz(x string) string {
	if len(x) > 8 {
		return x[:8]
	}
	return x
}
