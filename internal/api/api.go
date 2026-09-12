// Package api ist die HTTP-Schicht. Bewusst grobkörnig: ein Endpunkt liefert
// den kompletten Spielzustand. Bei zwei Nutzern ist feingranulare
// Synchronisation nur Komplexität ohne Gegenwert.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/sicher"
	"mimik/internal/store"
)

// Grenzen für Felder, die ein Spieler frei füllt. Kurz gehalten: Ein Spitzname
// steht in der Benachrichtigung des anderen Geräts, ein Tag in einem Prompt.
const (
	MaxSpitzname = 24
	MaxTag       = 40
	MaxTags      = 60
)

type Server struct {
	S      *store.Store
	M      *mimik.Client
	Worker *Worker
	Tor    Torwache
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/devices", s.geraetAnlegen)

	geschuetzt := map[string]http.HandlerFunc{
		"POST /v1/parties":            s.partyAnlegen,
		"POST /v1/parties/join":       s.partyBeitreten,
		"GET /v1/tags":                s.tagsVorschlagen,
		"PUT /v1/tags":                s.tagsSetzen,
		"GET /v1/state":               s.zustand,
		"POST /v1/matches":            s.matchAnlegen,
		"POST /v1/rounds/{id}/answer": s.antworten,
		"POST /v1/rounds/{id}/guess":  s.raten,
		"GET /v1/dossier":             s.dossier,
		"DELETE /v1/dossier":          s.dossierLoeschen,
		"POST /v1/me/name":            s.umbenennen,
		"POST /v1/me/delete":          s.kontoLoeschen,
	}
	for muster, h := range geschuetzt {
		mux.Handle(muster, s.auth(h))
	}
	return protokoll(mux)
}

// ------------------------------------------------------------- Grundlagen ---

type schluessel int

const spielerSchluessel schluessel = 0

func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			fehler(w, http.StatusUnauthorized, "kein token")
			return
		}
		p, err := s.S.SpielerZuToken(token)
		if err != nil {
			fehler(w, http.StatusUnauthorized, "token unbekannt")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), spielerSchluessel, p)))
	})
}

func spieler(r *http.Request) store.Player {
	p, _ := r.Context().Value(spielerSchluessel).(store.Player)
	return p
}

// protokoll schreibt eine Zeile je Anfrage – und zwar geputzt. Der Pfad kommt
// vom Klienten: Go lässt rohe Steuerzeichen in der Anfragezeile zwar nicht
// durch, prozentkodiert aber schon ("/v1/%1b[2J"), und r.URL.Path ist der
// dekodierte Pfad. Ungeputzt geschrieben steuert er das Terminal des
// Betreibers.
func protokoll(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", sicher.Protokoll(r.Method), sicher.Protokoll(r.URL.Path))
		h.ServeHTTP(w, r)
	})
}

func json_(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fehler(w http.ResponseWriter, code int, text string) {
	json_(w, code, map[string]string{"fehler": text})
}

func lies(r *http.Request, ziel any) bool {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(ziel) == nil
}

// ---------------------------------------------------------------- Spieler ---

func (s *Server) geraetAnlegen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Spitzname string `json:"spitzname"`
		Einladung string `json:"einladung"`
	}
	lies(r, &in)
	if !s.Tor.Passt(in.Einladung) {
		fehler(w, 403, "dieser server verlangt eine einladung")
		return
	}
	if !s.Tor.Darf(r) {
		fehler(w, 429, "zu viele anmeldungen von dieser adresse")
		return
	}
	name := sicher.Text(in.Spitzname, MaxSpitzname)
	if name == "" {
		fehler(w, 422, "spitzname darf nicht leer sein")
		return
	}
	p, token, err := s.S.SpielerAnlegen(name)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 201, map[string]any{"spieler": p, "token": token})
}

func (s *Server) umbenennen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Spitzname string `json:"spitzname"`
		Einladung string `json:"einladung"`
	}
	lies(r, &in)
	if !s.Tor.Passt(in.Einladung) {
		fehler(w, 403, "dieser server verlangt eine einladung")
		return
	}
	if !s.Tor.Darf(r) {
		fehler(w, 429, "zu viele anmeldungen von dieser adresse")
		return
	}
	name := sicher.Text(in.Spitzname, MaxSpitzname)
	if name == "" {
		fehler(w, 422, "spitzname darf nicht leer sein")
		return
	}
	if err := s.S.SpitznameSetzen(spieler(r).ID, name); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	p, _ := s.S.Spieler(spieler(r).ID)
	json_(w, 200, map[string]any{"spieler": p})
}

// kontoLoeschen verlangt den eigenen Spitznamen als Bestätigung. Bei etwas
// Unwiderruflichem ist ein Ja-Knopf zu wenig – man soll den Namen tippen
// müssen, den man löscht.
//
// POST statt DELETE, obwohl DELETE die passendere Vokabel wäre: Die Bestätigung
// gehört in den Rumpf, und HttpURLConnection auf Android verweigert einem DELETE
// den Rumpf.
func (s *Server) kontoLoeschen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Bestaetigung string `json:"bestaetigung"`
	}
	lies(r, &in)
	p := spieler(r)
	if !strings.EqualFold(strings.TrimSpace(in.Bestaetigung), strings.TrimSpace(p.Spitzname)) {
		fehler(w, 422, "zur bestätigung den eigenen spitznamen eintippen")
		return
	}
	if err := s.S.AllesLoeschen(p.ID); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 200, map[string]any{"geloescht": true})
}

// ------------------------------------------------------------------ Party ---

func (s *Server) partyAnlegen(w http.ResponseWriter, r *http.Request) {
	pa, code, err := s.S.PartyAnlegen(spieler(r).ID)
	switch {
	case errors.Is(err, store.ErrSchonDrin):
		fehler(w, 409, "du bist bereits in einer party")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 201, map[string]any{"party_id": pa.ID, "code": code})
	}
}

func (s *Server) partyBeitreten(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	lies(r, &in)
	pa, err := s.S.PartyBeitreten(sicher.Text(in.Code, 12), spieler(r).ID)
	switch {
	case errors.Is(err, store.ErrCodeUngueltig):
		fehler(w, 404, "einladungscode ungültig oder abgelaufen")
	case errors.Is(err, store.ErrPartyVoll):
		fehler(w, 409, "diese party ist voll")
	case errors.Is(err, store.ErrSchonDrin):
		fehler(w, 409, "du bist bereits in einer party")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 200, map[string]any{"party_id": pa.ID})
	}
}

// ------------------------------------------------------------------- Tags ---

func (s *Server) tagsVorschlagen(w http.ResponseWriter, r *http.Request) {
	v, err := s.S.TagVorschlaege(spieler(r).ID, 20)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	gewaehlt, _ := s.S.Tags(spieler(r).ID)
	json_(w, 200, map[string]any{
		"vorschlaege": nichtNil(v), "gewaehlt": nichtNil(gewaehlt), "mindestens": 10,
	})
}

func (s *Server) tagsSetzen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Tags []string `json:"tags"`
	}
	lies(r, &in)
	if len(in.Tags) > MaxTags {
		in.Tags = in.Tags[:MaxTags]
	}
	// Tags wandern in einen Prompt und auf Knöpfe. Beides verträgt keinen Text,
	// der sich als etwas anderes ausgibt.
	sauber := make([]string, 0, len(in.Tags))
	for _, t := range in.Tags {
		if t = sicher.Text(t, MaxTag); t != "" {
			sauber = append(sauber, t)
		}
	}
	err := s.S.TagsSetzen(spieler(r).ID, sauber)
	switch {
	case errors.Is(err, store.ErrZuWenigTags):
		fehler(w, 422, "mindestens 10 tags nötig")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		t, _ := s.S.Tags(spieler(r).ID)
		json_(w, 200, map[string]any{"tags": nichtNil(t)})
	}
}

// ----------------------------------------------------------------- Dossier ---

func (s *Server) dossier(w http.ResponseWriter, r *http.Request) {
	f, _ := s.S.Fakten(spieler(r).ID, 200)
	g, _ := s.S.GesperrteThemen(spieler(r).ID)
	themen := make([]string, 0, len(g))
	for t := range g {
		themen = append(themen, t)
	}
	tags, _ := s.S.Tags(spieler(r).ID)
	json_(w, 200, map[string]any{
		"fakten": nichtNil(f), "verbrauchte_themen": nichtNil(themen), "tags": nichtNil(tags),
	})
}

func (s *Server) dossierLoeschen(w http.ResponseWriter, r *http.Request) {
	if err := s.S.DossierLoeschen(spieler(r).ID); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 200, map[string]any{"geloescht": true})
}

// ------------------------------------------------------------------ Match ---

func (s *Server) matchAnlegen(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	pa, err := s.S.PartyVon(p.ID)
	if err != nil {
		fehler(w, 409, "du bist in keiner party")
		return
	}
	if pa.A == "" || pa.B == "" {
		fehler(w, 409, "die party braucht zwei mitglieder")
		return
	}
	for _, x := range []game.PlayerID{pa.A, pa.B} {
		t, _ := s.S.Tags(string(x))
		if len(t) < 10 {
			fehler(w, 409, "beide spieler brauchen mindestens 10 tags")
			return
		}
	}
	if m, err := s.S.AktivesMatch(pa.ID); err == nil {
		json_(w, 200, map[string]any{"match_id": m.ID, "bereits_offen": true})
		return
	}
	m, err := s.S.MatchAnlegen(pa.ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 201, map[string]any{"match_id": m.ID})
}
