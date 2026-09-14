package api

import (
	"errors"
	"net/http"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/sicher"
	"mimik/internal/store"
)

// KarteAus ist die Sicht des Spielers auf eine Karte. IstEcht und AnkerTag
// fehlen absichtlich, solange die Runde läuft – sie werden erst in der
// Auflösung nachgereicht.
type KarteAus struct {
	Pos     int    `json:"pos"`
	Text    string `json:"text"`
	IstEcht *bool  `json:"ist_echt,omitempty"`
	// Begruendung steht nur im EIGENEN Satz: Sie zitiert das Material des
	// Menschen, ueber den die Karte ist.
	Begruendung string `json:"begruendung,omitempty"`
}

type RundeAus struct {
	ID           string       `json:"id"`
	Nummer       int          `json:"nummer"`
	Frage        string       `json:"frage"`
	Zustand      game.Zustand `json:"zustand"`
	MeineAntwort string       `json:"meine_antwort,omitempty"`
	Karten       []KarteAus   `json:"karten,omitempty"`
	MeinTipp     *int         `json:"mein_tipp,omitempty"`
	MeinTreffer  *bool        `json:"mein_treffer,omitempty"`
	Aufloesung   *Aufloesung  `json:"aufloesung,omitempty"`
	Fehler       string       `json:"fehler,omitempty"`
	// MeineKarten ist der Satz UEBER MICH - die eigene Antwort und die drei
	// Faelschungen, die MIMIK daraus gebaut hat, mit ihren Begruendungen.
	//
	// Das verraet nichts: Wer seine eigene Antwort geschrieben hat, weiss
	// ohnehin, welche der vier Karten sie ist. Geraten wird ueber die ANDERE
	// Seite, und dieser Satz steht in Karten. Umgekehrt ist es der einzige Weg,
	// MIMIK bei der Arbeit zuzusehen - und das ist der Reiz.
	MeineKarten []KarteAus `json:"meine_karten,omitempty"`
	// Sekunden, seit BEIDE geantwortet haben - der Beginn von MIMIKs Arbeit.
	// Nur gesetzt, solange sie arbeitet; die App zeichnet daraus den
	// Fortschrittsbalken und findet ihn nach einem Neustart wieder.
	WartetSeit int `json:"wartet_seit,omitempty"`
}

type Aufloesung struct {
	EchteKarte      int    `json:"echte_karte"` // im Satz über den Partner
	AntwortPartner  string `json:"antwort_partner"`
	MeinTippRichtig bool   `json:"mein_tipp_richtig"`
	PartnerTipp     int    `json:"partner_tipp"`
	// Der Text der Karte, die die andere Seite fuer meine echte Antwort hielt -
	// aus dem Satz ueber MICH. Der spannendste Teil der Auflösung: Wofuer hat
	// sie mich gehalten? Erst nach dem zweiten Tipp, also ist nichts verraten.
	PartnerTippText string `json:"partner_tipp_text,omitempty"`
	// Woraus MIMIK die Faelschung gebaut hat, auf die die andere Seite
	// hereingefallen ist.
	//
	// NUR fuer den Satz ueber MICH. Die Begruendungen der Karten ueber das
	// Gegenueber zitieren DESSEN Dossier - Faktzen, die es dem Spiel erzaehlt
	// hat, unter Umstaenden in einer ganz anderen Partie. Die gehen hier
	// niemals hinaus.
	PartnerTippGrund string `json:"partner_tipp_grund,omitempty"`
	MeineEchte       string `json:"meine_echte,omitempty"`
	PartnerRichtig   bool   `json:"partner_richtig"`
	Doppeltreffer    bool   `json:"doppeltreffer"`
}

// nichtNil sorgt dafür, dass leere Listen als [] und nicht als null hinausgehen.
// Go marshalt ein nil-Slice zu null, und ein Klient mit strenger Typisierung
// bricht daran ab – hier gefunden, als die App sich weigerte, "tags":null zu lesen.
func nichtNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// kartentext sucht eine Karte nach Position. Fehlt sie, bleibt das Feld leer -
// die App zeigt den Abschnitt dann gar nicht, statt eine Luecke zu malen.
func kartentext(ks []game.Karte, pos int) string {
	for _, k := range ks {
		if k.Pos == pos {
			return k.Text
		}
	}
	return ""
}

// kartengrund liefert die Begruendung einer Karte nach Position.
func kartengrund(ks []game.Karte, pos int) string {
	for _, k := range ks {
		if k.Pos == pos && !k.IstEcht {
			return k.Begruendung
		}
	}
	return ""
}

func (s *Server) zustand(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	// Listenfelder immer belegen, auch auf den frühen Rückwegen: Ein fehlendes
	// Feld ist für einen streng typisierten Klienten dasselbe Problem wie null.
	// Tags hängen am Spieler, nicht an der Party: Das Onboarding läuft, bevor
	// es eine Party gibt, und MIMIKs Wissen überlebt jede Party.
	tags, _ := s.S.Tags(p.ID)
	aus := map[string]any{
		"spieler": p,
		"tags":    nichtNil(tags),
		"name":    s.nameStand(p.ID),
		"runden":  []RundeAus{},
		"dran":    []map[string]string{},
	}

	// Mit Partie-ID im Pfad genau diese, ohne Pfad die einzige. Wer in
	// mehreren Partien ist und den alten Weg nimmt, bekommt hier KEINE
	// geratene Partie, sondern eine leere Antwort - die App merkt daran, dass
	// sie ueber die Lobby gehen muss.
	var pa game.Party
	if x := r.PathValue("id"); x != "" {
		var err error
		pa, err = s.S.PartyVon(p.ID, x)
		if err != nil {
			fehler(w, 404, "party nicht gefunden")
			return
		}
	} else {
		xs, err := s.S.PartienVon(p.ID)
		if err != nil || len(xs) != 1 {
			json_(w, 200, aus)
			return
		}
		pa = xs[0]
	}
	// Die Partie-ID GLEICH HIER, nicht erst am Ende: Darunter liegen zwei
	// vorzeitige Rueckwege (kein Match, keine Runden). Eine frische Partie hat
	// noch kein Match, ging also ohne party_id hinaus - und die App, die ihre
	// geoeffnete Partie ueber genau dieses Feld wiedererkennt, blieb im
	// Ladebildschirm stehen.
	aus["party_id"] = pa.ID
	party := map[string]any{"id": pa.ID, "tags": nichtNil(tags), "code": s.S.PartyCode(pa.ID)}
	gegner := pa.Gegner(game.PlayerID(p.ID))
	if gegner != "" {
		if g, err := s.S.Spieler(string(gegner)); err == nil {
			party["partner"] = g
		}
	}
	aus["party"] = party

	m, err := s.S.LetztesMatch(pa.ID)
	if err != nil {
		json_(w, 200, aus)
		return
	}
	aus["match"] = map[string]any{
		"id": m.ID, "stand": m.Stand, "ziel": m.Ziel, "ergebnis": m.Ergebnis,
	}

	runden, err := s.S.RundenVonMatch(m.ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	ich := game.PlayerID(p.ID)
	liste := []RundeAus{}
	aufgeloeste := false
	dran := []map[string]string{}
	for _, rd := range runden {
		z := rd.Ableiten(pa)
		ra := RundeAus{ID: rd.ID, Nummer: rd.Nummer, Frage: rd.Frage, Zustand: z,
			Fehler: s.S.RundenFehler(rd.ID)}
		if a, ok := rd.Antworten[ich]; ok {
			ra.MeineAntwort = a.Normalform
			if z == game.MimikArbeitet {
				ra.WartetSeit = s.S.ArbeitSeit(rd.ID)
			}
		} else {
			dran = append(dran, map[string]string{"was": "schreiben", "runde": rd.ID})
		}
		aufgedeckt := z == game.Aufgeloest
		if aufgedeckt {
			aufgeloeste = true
		}
		ra.Karten = []KarteAus{}
		for _, k := range rd.KartenFuer(pa, ich) {
			ka := KarteAus{Pos: k.Pos, Text: k.Text}
			if aufgedeckt {
				echt := k.IstEcht
				ka.IstEcht = &echt
			}
			ra.Karten = append(ra.Karten, ka)
		}
		// Der eigene Satz, sobald er steht. Vor der Auflösung traegt er die
		// Begruendungen; danach braucht sie niemand mehr, und die Auflösung
		// zeigt ohnehin die eine, auf die das Gegenueber hereingefallen ist.
		if len(rd.Karten[ich]) == 4 {
			for _, k := range rd.Karten[ich] {
				echt := k.IstEcht
				ra.MeineKarten = append(ra.MeineKarten, KarteAus{
					Pos: k.Pos, Text: k.Text, IstEcht: &echt, Begruendung: k.Begruendung,
				})
			}
		}
		if t, ok := rd.Tipps[ich]; ok {
			g, tr := t.Gewaehlt, t.Richtig
			ra.MeinTipp, ra.MeinTreffer = &g, &tr
		} else if z == game.Raten || z == game.RatenWartet {
			dran = append(dran, map[string]string{"was": "raten", "runde": rd.ID})
		}
		if aufgedeckt {
			meiner, partner := rd.Tipps[ich], rd.Tipps[gegner]
			ra.Aufloesung = &Aufloesung{
				EchteKarte:       rd.EchteKarte(gegner),
				AntwortPartner:   rd.Antworten[gegner].Normalform,
				MeinTippRichtig:  meiner.Richtig,
				PartnerTipp:      partner.Gewaehlt,
				PartnerTippText:  kartentext(rd.Karten[ich], partner.Gewaehlt),
				PartnerTippGrund: kartengrund(rd.Karten[ich], partner.Gewaehlt),
				MeineEchte:       rd.Antworten[ich].Normalform,
				PartnerRichtig:   partner.Richtig,
				Doppeltreffer:    game.Doppeltreffer(rd, pa),
			}
		}
		liste = append(liste, ra)
	}
	aus["runden"] = liste
	aus["dran"] = dran
	// Wer den Zustand einer Partie abruft, hat ihre Auflösungen vor Augen.
	// Damit faellt der Zaehler in der Lobby genau dann, wenn der Mensch
	// hingeschaut hat - und nicht schon, wenn die Runde aufgeloest wurde.
	if aufgeloeste {
		s.S.PartieGesehen(p.ID, pa.ID)
	}
	json_(w, 200, aus)
}

// ------------------------------------------------------------------- Züge ---

func (s *Server) antworten(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	rid := r.PathValue("id")
	var in struct {
		Original string `json:"original"`
	}
	lies(r, &in)
	// Der Klient schickt nur noch den rohen Text. Die saubere Fassung entsteht
	// im Worker, zusammen mit den Fälschungen – bis dahin steht die
	// regelbasierte Notfassung da, damit der Wartebildschirm nicht leer ist.
	in.Original = sicher.Text(in.Original, game.MaxAntwort)
	pa, _, err := s.S.PartyVonRunde(rid)
	if err != nil || !pa.Mitglied(game.PlayerID(p.ID)) {
		fehler(w, 404, "runde nicht gefunden")
		return
	}
	rd, err := s.S.Runde(rid)
	if err != nil {
		fehler(w, 404, "runde nicht gefunden")
		return
	}
	a := game.Antwort{Original: in.Original, Normalform: mimik.ErsatzNormalform(in.Original)}
	if err := rd.AntwortAbgeben(pa, game.PlayerID(p.ID), a); err != nil {
		code := 409
		if errors.Is(err, game.ErrZuKurz) || errors.Is(err, game.ErrZuLang) {
			code = 422
		}
		fehler(w, code, err.Error())
		return
	}
	if err := s.S.AntwortSpeichern(rid, p.ID, a); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	neu := rd.Ableiten(pa)
	s.S.ZustandSetzen(rid, neu, "")
	if neu == game.MimikArbeitet {
		s.Worker.Anstossen() // MIMIK darf loslegen
	}
	json_(w, 200, map[string]any{"zustand": neu})
}

func (s *Server) raten(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	rid := r.PathValue("id")
	var in struct {
		Pos int `json:"pos"`
	}
	lies(r, &in)
	pa, m, err := s.S.PartyVonRunde(rid)
	if err != nil || !pa.Mitglied(game.PlayerID(p.ID)) {
		fehler(w, 404, "runde nicht gefunden")
		return
	}
	rd, err := s.S.Runde(rid)
	if err != nil {
		fehler(w, 404, "runde nicht gefunden")
		return
	}
	fertig, err := rd.TippAbgeben(pa, game.PlayerID(p.ID), in.Pos)
	if err != nil {
		code := 409
		if errors.Is(err, game.ErrKartePos) {
			code = 422
		}
		fehler(w, code, err.Error())
		return
	}
	m, _, err = s.S.TippSpeichern(m, pa, rd, p.ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	// Reicht der Vorrat an Runden nicht bis zum Ziel, eine nachlegen.
	if m.Ergebnis == game.Offen || m.Ergebnis == game.Verlaengerung {
		s.rundenNachlegen(m, pa)
	}
	json_(w, 200, map[string]any{
		"richtig":       rd.Tipps[game.PlayerID(p.ID)].Richtig,
		"runde_beendet": fertig, "stand": m.Stand, "ergebnis": m.Ergebnis,
	})
}

// rundenNachlegen sorgt dafür, dass immer mindestens zwei unbeantwortete Runden
// bereitliegen – sonst steht das Match, obwohl das Ziel noch nicht erreicht ist.
func (s *Server) rundenNachlegen(m store.Match, pa game.Party) {
	runden, err := s.S.RundenVonMatch(m.ID)
	if err != nil {
		return
	}
	offen := 0
	for _, rd := range runden {
		if rd.Ableiten(pa) != game.Aufgeloest {
			offen++
		}
	}
	for i := offen; i < 2; i++ {
		if err := s.S.RundeNachlegen(m); err != nil {
			return
		}
	}
}
