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
	"strconv"
	"strings"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/sicher"
	"mimik/internal/store"
)

// Grenzen für Felder, die ein Spieler frei füllt. Kurz gehalten: Ein Spitzname
// steht in der Benachrichtigung des anderen Geräts, ein Tag in einem Prompt.
// TestCode ist der Einladungscode, der eine Partie gegen einen Testspieler
// eröffnet. Sechs Zeichen kann er nicht sein – dann ließe er sich nicht von
// einem echten Code unterscheiden.
const TestCode = "TEST"

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
		"POST /v1/parties":           s.partyAnlegen,
		"POST /v1/parties/join":      s.partyBeitreten,
		"POST /v1/parties/verlassen": s.partyVerlassen,
		// Partie-adressiert. Die Muster ohne {id} darueber sind spezifischer
		// und gewinnen beim ServeMux von Go 1.22 - "join" und "verlassen"
		// werden also nie fuer eine Partie-ID gehalten.
		"GET /v1/parties/{id}/state":         s.zustand,
		"POST /v1/parties/{id}/verlassen":    s.partyVerlassen,
		"POST /v1/parties/{id}/matches":      s.matchAnlegen,
		"POST /v1/parties/{id}/abbrechen":    s.matchAbbrechen,
		"POST /v1/parties/{id}/gesehen":      s.partieGesehen,
		"GET /v1/lobby":                      s.lobby,
		"GET /v1/spieler":                    s.spielerSuchen,
		"GET /v1/namen/frei":                 s.nameFrei,
		"POST /v1/einladungen":               s.einladen,
		"POST /v1/einladungen/{id}/annehmen": s.einladungAnnehmen,
		"POST /v1/einladungen/{id}/ablehnen": s.einladungAblehnen,
		"DELETE /v1/einladungen/{id}":        s.einladungZurueckziehen,
		"GET /v1/tags":                       s.tagsVorschlagen,
		"PUT /v1/tags":                       s.tagsSetzen,
		"GET /v1/state":                      s.zustand,
		"POST /v1/matches":                   s.matchAnlegen,
		"POST /v1/matches/abbrechen":         s.matchAbbrechen,
		"POST /v1/rounds/{id}/answer":        s.antworten,
		"POST /v1/rounds/{id}/guess":         s.raten,
		"GET /v1/dossier":                    s.dossier,
		"DELETE /v1/dossier":                 s.dossierLoeschen,
		"POST /v1/me/name":                   s.umbenennen,
		"POST /v1/me/delete":                 s.kontoLoeschen,
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
	if !s.Tor.Darf(r, in.Einladung != "") {
		fehler(w, 429, "zu viele anmeldungen von dieser adresse")
		return
	}
	name := sicher.Text(in.Spitzname, MaxSpitzname)
	if name == "" {
		fehler(w, 422, "spitzname darf nicht leer sein")
		return
	}
	p, token, err := s.S.SpielerAnlegen(name)
	if errors.Is(err, store.ErrNameVergeben) {
		// 409 statt stiller Umbenennung: Wer sich anmeldet, sitzt gerade davor
		// und kann einen anderen Namen waehlen. Ihm ungefragt eine Ziffer
		// anzuhaengen hiesse, ihn unter einem Namen spielen zu lassen, den er
		// nicht gewaehlt hat.
		fehler(w, 409, "dieser name ist vergeben")
		return
	}
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 201, map[string]any{"spieler": p, "token": token})
}

// umbenennen verlangt NUR das Token.
//
// Hier standen einmal dieselbe Einladungspruefung und derselbe Zaehler wie beim
// Anmelden. Beides war falsch: Wer ein Token hat, ist bereits eingelassen - die
// App schickt das Geheimnis nach dem Anmelden nie wieder mit, also war
// Umbenennen auf einem geschlossenen Server schlicht unmoeglich. Und ein
// Namenswechsel kostet keinen Modellaufruf, also gibt es auch nichts zu
// deckeln. Schlimmer noch: Er ass vom Anmeldebudget derselben Adresse, sodass
// eine Umbenennung die naechste Anmeldung im Haushalt blockieren konnte.
func (s *Server) umbenennen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Spitzname string `json:"spitzname"`
	}
	lies(r, &in)
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

// partie loest die Partie auf, auf die sich ein Aufruf bezieht.
//
// Steht eine ID im Pfad, gilt sie - mit Mitgliedspruefung. Steht keine da, ist
// der Aufruf von einer aelteren App: Dann gilt die juengste Partie, aber NUR
// solange es genau eine gibt. Bei mehreren wird nicht geraten, sondern
// abgelehnt. Stilles Raten waere hier die schlimmste Wahl: Es sieht aus wie ein
// Bedienfehler, wird deshalb nie gemeldet, und der Zug landet in der falschen
// Partie.
func (s *Server) partie(w http.ResponseWriter, r *http.Request) (game.Party, bool) {
	pid := spieler(r).ID
	if x := r.PathValue("id"); x != "" {
		pa, err := s.S.PartyVon(pid, x)
		if err != nil {
			fehler(w, 404, "party nicht gefunden")
			return game.Party{}, false
		}
		return pa, true
	}
	xs, err := s.S.PartienVon(pid)
	if err != nil {
		fehler(w, 500, err.Error())
		return game.Party{}, false
	}
	switch len(xs) {
	case 0:
		fehler(w, 409, "du bist in keiner party")
		return game.Party{}, false
	case 1:
		return xs[0], true
	default:
		fehler(w, 409, "du bist in mehreren partien – nimm den partie-pfad")
		return game.Party{}, false
	}
}

func (s *Server) partyAnlegen(w http.ResponseWriter, r *http.Request) {
	pa, code, err := s.S.PartyAnlegen(spieler(r).ID)
	switch {
	case errors.Is(err, store.ErrSchonDrin):
		fehler(w, 409, "du hast schon genug offene codes")
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
	code := strings.ToUpper(strings.TrimSpace(sicher.Text(in.Code, 12)))

	// TEST setzt einen allein spielbaren Gegner an den Tisch. Zu zweit zu
	// spielen heißt sonst, zu zweit sein zu müssen – wer eine Frage, eine Runde
	// oder den ganzen Ablauf ausprobieren will, bräuchte ein zweites Telefon.
	if code == TestCode {
		pa, err := s.S.TestpartyAnlegen(spieler(r).ID)
		switch {
		case errors.Is(err, store.ErrSchonDrin):
			fehler(w, 409, "du hast schon genug testpartien offen")
		case err != nil:
			fehler(w, 500, err.Error())
		default:
			s.Worker.Anstossen()
			json_(w, 200, map[string]any{"party_id": pa.ID, "test": true})
		}
		return
	}

	pa, err := s.S.PartyBeitreten(code, spieler(r).ID)
	switch {
	case errors.Is(err, store.ErrCodeUngueltig):
		fehler(w, 404, "einladungscode ungültig oder abgelaufen")
	case errors.Is(err, store.ErrPartyVoll):
		fehler(w, 409, "diese party ist voll")
	case errors.Is(err, store.ErrSchonMitglied):
		fehler(w, 409, "in dieser party bist du schon dabei")
	case errors.Is(err, store.ErrSchonPartner):
		fehler(w, 409, "mit dieser person läuft schon eine partie")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 200, map[string]any{"party_id": pa.ID})
	}
}

// partyVerlassen löst die Party auf. Die Rückfrage stellt die App.
func (s *Server) partyVerlassen(w http.ResponseWriter, r *http.Request) {
	pa, ok := s.partie(w, r)
	if !ok {
		return
	}
	err := s.S.PartyVerlassen(spieler(r).ID, pa.ID)
	switch {
	case errors.Is(err, store.ErrNichtGefunden), errors.Is(err, store.ErrKeinZugriff):
		fehler(w, 409, "du bist in keiner party")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 200, map[string]any{"verlassen": true})
	}
}

// ------------------------------------------------------------------- Tags ---

// tagsVorschlagen blättert durch den Vorrat. "ab" sagt, wie viele Vorschläge
// der Klient schon gesehen hat; "mehr" sagt ihm, ob sich ein weiterer Griff
// lohnt.
func (s *Server) tagsVorschlagen(w http.ResponseWriter, r *http.Request) {
	ab, _ := strconv.Atoi(r.URL.Query().Get("ab"))
	v, mehr, err := s.S.TagVorschlaege(spieler(r).ID, ab, 20)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	gewaehlt, _ := s.S.Tags(spieler(r).ID)
	json_(w, 200, map[string]any{
		"vorschlaege": nichtNil(v), "gewaehlt": nichtNil(gewaehlt),
		"mindestens": 10, "mehr": mehr,
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
	// Das Profil steht nur hier - nie in /v1/state. Es ist eine Einschaetzung
	// ueber einen Menschen, und die geht niemanden ausser ihn selbst etwas an,
	// schon gar nicht sein Gegenueber.
	profil, _ := s.S.Profil(spieler(r).ID)
	verlauf, _ := s.S.ProfilVerlauf(spieler(r).ID, 40)
	if profil == nil {
		profil = []store.Merkmal{}
	}
	if verlauf == nil {
		verlauf = []store.Verlaufseintrag{}
	}
	json_(w, 200, map[string]any{
		"fakten": nichtNil(f), "verbrauchte_themen": nichtNil(themen), "tags": nichtNil(tags),
		"profil": profil, "profil_verlauf": verlauf,
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

// matchAbbrechen beendet das laufende Match. Die Rückfrage stellt die App; der
// Server verlangt hier nichts weiter, weil beide Seiten ohnehin dasselbe Match
// besitzen und jede es beenden darf.
func (s *Server) matchAbbrechen(w http.ResponseWriter, r *http.Request) {
	pa, ok := s.partie(w, r)
	if !ok {
		return
	}
	m, err := s.S.AktivesMatch(pa.ID)
	if err != nil {
		fehler(w, 409, "es läuft gerade kein spiel")
		return
	}
	if err := s.S.MatchAbbrechen(m.ID); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 200, map[string]any{"abgebrochen": true})
}

func (s *Server) matchAnlegen(w http.ResponseWriter, r *http.Request) {
	pa, ok := s.partie(w, r)
	if !ok {
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

// ------------------------------------------------------------------ Lobby ---

func (s *Server) lobby(w http.ResponseWriter, r *http.Request) {
	p := spieler(r)
	partien, err := s.S.Lobby(p.ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	ein, aus, err := s.S.EinladungenFuer(p.ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	tags, _ := s.S.Tags(p.ID)
	json_(w, 200, map[string]any{
		"spieler": p,
		"tags":    nichtNil(tags),
		"name":    s.nameStand(p.ID),
		"partien": partien,
		"einladungen": map[string]any{
			"eingehend": nichtNilE(ein),
			"ausgehend": nichtNilE(aus),
		},
	})
}

// nameStand sagt, ob dieser Spieler seinen Namen beansprucht hat.
//
// Steht "beansprucht" auf false, traegt jemand anders denselben Namen - der
// Spieler kann weiterspielen, ist aber in keiner Suche zu finden, bis er sich
// einen freien nimmt. Die Aufforderung dazu stellt die App aus diesem Feld;
// der Server sperrt dafuer nichts. Ein Server, der eine ganze Schnittstelle
// verriegelt, um eine Textaenderung zu erzwingen, nimmt Geiseln.
func (s *Server) nameStand(pid string) map[string]any {
	geklaert := s.S.NameGeklaert(pid)
	return map[string]any{"eindeutig": geklaert, "beansprucht": geklaert}
}

func nichtNilE(xs []store.Einladung) []store.Einladung {
	if xs == nil {
		return []store.Einladung{}
	}
	return xs
}

func (s *Server) partieGesehen(w http.ResponseWriter, r *http.Request) {
	pa, ok := s.partie(w, r)
	if !ok {
		return
	}
	if err := s.S.PartieGesehen(spieler(r).ID, pa.ID); err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 200, map[string]any{"gesehen": true})
}

// ---------------------------------------------------------- Namen · Suche ---

func (s *Server) spielerSuchen(w http.ResponseWriter, r *http.Request) {
	q := sicher.Text(r.URL.Query().Get("q"), MaxSpitzname)
	treffer, err := s.S.SpielerSuchen(q, spieler(r).ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	if treffer == nil {
		treffer = []store.Player{}
	}
	json_(w, 200, map[string]any{"treffer": treffer})
}

func (s *Server) nameFrei(w http.ResponseWriter, r *http.Request) {
	name := sicher.Text(r.URL.Query().Get("name"), MaxSpitzname)
	frei, err := s.S.NameFrei(name, spieler(r).ID)
	if err != nil {
		fehler(w, 500, err.Error())
		return
	}
	json_(w, 200, map[string]any{"frei": frei})
}

// ------------------------------------------------------------ Einladungen ---

func (s *Server) einladen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		An string `json:"an"`
	}
	lies(r, &in)
	e, err := s.S.EinladungAnlegen(spieler(r).ID, sicher.Text(in.An, 64))
	switch {
	case errors.Is(err, store.ErrSelbst):
		fehler(w, 422, "dich selbst kannst du nicht einladen")
	case errors.Is(err, store.ErrNichtGefunden):
		fehler(w, 404, "diese person gibt es nicht")
	case errors.Is(err, store.ErrSchonPartner):
		fehler(w, 409, "mit dieser person läuft schon eine partie")
	case errors.Is(err, store.ErrSchonEingeladen):
		fehler(w, 409, "die einladung steht schon")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 201, map[string]any{"einladung": e})
	}
}

func (s *Server) einladungAnnehmen(w http.ResponseWriter, r *http.Request) {
	pa, err := s.S.EinladungAnnehmen(r.PathValue("id"), spieler(r).ID)
	switch {
	case errors.Is(err, store.ErrNichtGefunden):
		fehler(w, 404, "diese einladung steht nicht mehr offen")
	case errors.Is(err, store.ErrSchonPartner):
		fehler(w, 409, "mit dieser person läuft schon eine partie")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 200, map[string]any{"party_id": pa.ID})
	}
}

func (s *Server) einladungAblehnen(w http.ResponseWriter, r *http.Request) {
	s.einladungSchliessen(w, s.S.EinladungAblehnen(r.PathValue("id"), spieler(r).ID))
}

func (s *Server) einladungZurueckziehen(w http.ResponseWriter, r *http.Request) {
	s.einladungSchliessen(w, s.S.EinladungZurueckziehen(r.PathValue("id"), spieler(r).ID))
}

func (s *Server) einladungSchliessen(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNichtGefunden):
		fehler(w, 404, "diese einladung steht nicht mehr offen")
	case err != nil:
		fehler(w, 500, err.Error())
	default:
		json_(w, 200, map[string]any{"geschlossen": true})
	}
}
