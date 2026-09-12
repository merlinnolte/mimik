package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"mimik/internal/game"
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
}

type Aufloesung struct {
	EchteKarte      int    `json:"echte_karte"` // im Satz über den Partner
	AntwortPartner  string `json:"antwort_partner"`
	MeinTippRichtig bool   `json:"mein_tipp_richtig"`
	PartnerTipp     int    `json:"partner_tipp"`
	PartnerRichtig  bool   `json:"partner_richtig"`
	Doppeltreffer   bool   `json:"doppeltreffer"`
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
		"runden":  []RundeAus{},
		"dran":    []map[string]string{},
	}

	pa, err := s.S.PartyVon(p.ID)
	if err != nil {
		json_(w, 200, aus)
		return
	}
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
	dran := []map[string]string{}
	for _, rd := range runden {
		z := rd.Ableiten(pa)
		ra := RundeAus{ID: rd.ID, Nummer: rd.Nummer, Frage: rd.Frage, Zustand: z,
			Fehler: s.S.RundenFehler(rd.ID)}
		if a, ok := rd.Antworten[ich]; ok {
			ra.MeineAntwort = a.Normalform
		} else {
			dran = append(dran, map[string]string{"was": "schreiben", "runde": rd.ID})
		}
		aufgedeckt := z == game.Aufgeloest
		ra.Karten = []KarteAus{}
		for _, k := range rd.KartenFuer(pa, ich) {
			ka := KarteAus{Pos: k.Pos, Text: k.Text}
			if aufgedeckt {
				echt := k.IstEcht
				ka.IstEcht = &echt
			}
			ra.Karten = append(ra.Karten, ka)
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
				EchteKarte:      rd.EchteKarte(gegner),
				AntwortPartner:  rd.Antworten[gegner].Normalform,
				MeinTippRichtig: meiner.Richtig,
				PartnerTipp:     partner.Gewaehlt,
				PartnerRichtig:  partner.Richtig,
				Doppeltreffer:   game.Doppeltreffer(rd, pa),
			}
		}
		liste = append(liste, ra)
	}
	aus["runden"] = liste
	aus["dran"] = dran
	json_(w, 200, aus)
}

// ------------------------------------------------------------------- Züge ---

func (s *Server) normalisieren(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	lies(r, &in)
	// Vor der Prüfung putzen, nicht danach: Sonst zählt die Längengrenze
	// Zeichen mit, die anschließend wegfallen, und der Text geht ungeputzt
	// zum Modell.
	in.Text = sicher.Text(in.Text, game.MaxAntwort)
	if err := game.PruefeAntwort(in.Text); err != nil {
		fehler(w, 422, err.Error())
		return
	}
	ctx, abbruch := context.WithTimeout(r.Context(), 25*time.Second)
	defer abbruch()
	n, ersatz := s.M.Normalform(ctx, in.Text)
	json_(w, 200, map[string]any{
		"original": in.Text, "normalform": n,
		"regelbasiert": ersatz, "knapp": game.Knapp(n),
	})
}

func (s *Server) antworten(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	rid := r.PathValue("id")
	var in struct {
		Original   string `json:"original"`
		Normalform string `json:"normalform"`
	}
	lies(r, &in)
	in.Original = sicher.Text(in.Original, game.MaxAntwort)
	in.Normalform = sicher.Text(in.Normalform, game.MaxAntwort)
	if in.Normalform == "" {
		in.Normalform = in.Original
	}
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
	a := game.Antwort{Original: in.Original, Normalform: in.Normalform}
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
