package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/store"
)

// stubModell ahmt den OpenAI-kompatiblen Endpunkt nach. Es unterscheidet die
// beiden Prompts am System-Text und antwortet deterministisch – damit prüft der
// Test die Mechanik, nicht die Laune eines Modells.
func stubModell(t *testing.T) *httptest.Server {
	t.Helper()
	var n int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roh, _ := io.ReadAll(r.Body)
		var in struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		json.Unmarshal(roh, &in)
		user := in.Messages[1].Content
		n++

		// Die rohe Antwort steht im Material. Das Stubmodell "schreibt sie
		// sauber", indem es den Satzanfang großmacht - genug, um zu prüfen,
		// dass die Normalform aus diesem Aufruf bis auf die Karte durchläuft.
		i := strings.Index(user, "[echte_antwort_roh]\n")
		antwort := user[i+len("[echte_antwort_roh]\n"):]
		antwort = antwort[:strings.Index(antwort, "\n")]

		inhalt, _ := jsonString(map[string]any{
			"normalform": "Sauber: " + antwort,
			"fakt":       fmt.Sprintf("Hat in Runde %d etwas über sich verraten.", n),
			"sperre":     []string{"stubthema"},
			"antworten": []map[string]string{
				{"anker": "a", "text": fmt.Sprintf("Zitronenfalter beobachten, %d Stück.", n)},
				{"anker": "b", "text": fmt.Sprintf("Rathausturm besteigen, ganz oben %d.", n)},
				{"anker": "c", "text": fmt.Sprintf("Werkzeugkisten sortieren bei Nummer %d.", n)},
			},
		})
		json_(w, 200, map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": inhalt}}},
		})
	}))
}

func jsonString(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

type klient struct {
	t     *testing.T
	basis string
	token string
}

func (k *klient) ruf(methode, pfad string, koerper any, ziel any) int {
	k.t.Helper()
	var leib io.Reader
	if koerper != nil {
		b, _ := json.Marshal(koerper)
		leib = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(methode, k.basis+pfad, leib)
	if k.token != "" {
		req.Header.Set("Authorization", "Bearer "+k.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		k.t.Fatalf("%s %s: %v", methode, pfad, err)
	}
	defer res.Body.Close()
	if ziel != nil {
		json.NewDecoder(res.Body).Decode(ziel)
	}
	return res.StatusCode
}

func aufbauen(t *testing.T) (*httptest.Server, *store.Store, *Worker) {
	return bauen(t, "")
}

// bauen stellt Server, Ablage und Worker gegen das Stubmodell. einladung setzt
// das Geheimnis der Torwache; leer heißt offen, wie im Standardfall.
func bauen(t *testing.T, einladung string) (*httptest.Server, *store.Store, *Worker) {
	t.Helper()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	modell := stubModell(t)
	t.Cleanup(modell.Close)

	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	m := mimik.NeuAusUmgebung()
	m.BaseURL, m.APIKey, m.Model, m.JSONMode = modell.URL, "test", "stub", false
	w := NeuerWorker(s, m)
	srv := httptest.NewServer((&Server{
		S: s, M: m, Worker: w, Tor: Torwache{Einladung: einladung},
	}).Routes())
	t.Cleanup(srv.Close)
	return srv, s, w
}

func zehnTags() []string {
	return []string{"kaffee", "bücher", "filme", "musik", "berge",
		"garten", "zugfahren", "kochen", "wandern", "podcasts"}
}

// TestGanzesMatch spielt ein vollständiges Match durch: Geräte, Party, Tags,
// Runden, Auflösung, Matchende. Der Stub liefert immer brauchbare Fälschungen,
// geprüft wird also die Mechanik drumherum.
func TestGanzesMatch(t *testing.T) {
	srv, s, w := aufbauen(t)
	ctx := context.Background()

	// Zwei Geräte anmelden.
	a := &klient{t: t, basis: srv.URL}
	b := &klient{t: t, basis: srv.URL}
	for _, k := range []*klient{a, b} {
		var out struct {
			Token string `json:"token"`
		}
		if code := k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "X"}, &out); code != 201 {
			t.Fatalf("devices: %d", code)
		}
		k.token = out.Token
	}

	// Party gründen und beitreten.
	var pa struct {
		Code string `json:"code"`
	}
	if code := a.ruf("POST", "/v1/parties", nil, &pa); code != 201 {
		t.Fatalf("parties: %d", code)
	}
	if code := b.ruf("POST", "/v1/parties/join", map[string]string{"code": pa.Code}, nil); code != 200 {
		t.Fatalf("join: %d", code)
	}
	// Code ist einmal einlösbar.
	c := &klient{t: t, basis: srv.URL}
	var out3 struct{ Token string }
	c.ruf("POST", "/v1/devices", map[string]string{"spitzname": "Z"}, &out3)
	c.token = out3.Token
	if code := c.ruf("POST", "/v1/parties/join", map[string]string{"code": pa.Code}, nil); code == 200 {
		t.Fatal("derselbe code liess sich zweimal einloesen")
	}

	// Ohne Tags kein Match.
	if code := a.ruf("POST", "/v1/matches", nil, nil); code != 409 {
		t.Fatalf("match ohne tags: %d, erwartet 409", code)
	}
	for _, k := range []*klient{a, b} {
		if code := k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()[:9]}, nil); code != 422 {
			t.Fatalf("9 tags: %d, erwartet 422", code)
		}
		if code := k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil); code != 200 {
			t.Fatalf("10 tags: %d", code)
		}
	}
	if code := a.ruf("POST", "/v1/matches", nil, nil); code != 201 {
		t.Fatalf("match: %d", code)
	}

	// Das Match durchspielen.
	runden := 0
	for {
		runden++
		if runden > 30 {
			t.Fatal("match endet nicht")
		}
		var st struct {
			Match struct {
				ID       string        `json:"id"`
				Stand    game.Stand    `json:"stand"`
				Ergebnis game.Ergebnis `json:"ergebnis"`
			} `json:"match"`
			Runden []RundeAus `json:"runden"`
		}
		a.ruf("GET", "/v1/state", nil, &st)
		if st.Match.ID == "" {
			t.Fatal("/v1/state liefert kein match mehr - der client saehe keinen endstand")
		}
		if st.Match.Ergebnis != game.Offen {
			if st.Match.Stand.Mensch < 10 && st.Match.Stand.Mimik < 10 {
				t.Fatalf("match beendet bei %+v", st.Match.Stand)
			}
			t.Logf("Match endet %d:%d nach %d Runden als %s",
				st.Match.Stand.Mensch, st.Match.Stand.Mimik, runden, st.Match.Ergebnis)
			break
		}

		// Die erste Runde suchen, die noch nicht aufgelöst ist.
		var rid string
		for _, r := range st.Runden {
			if r.Zustand != game.Aufgeloest {
				rid = r.ID
				break
			}
		}
		if rid == "" {
			t.Fatal("keine offene runde, match aber nicht beendet")
		}

		// Beide schreiben.
		for i, k := range []*klient{a, b} {
			text := fmt.Sprintf("Meine ganz eigene Antwort Nummer %d in Runde %d.", i, runden)
			k.ruf("POST", "/v1/rounds/"+rid+"/answer",
				map[string]any{"original": text, "normalform": text}, nil)
		}
		// Vor der Ratephase darf niemand Karten sehen.
		a.ruf("GET", "/v1/state", nil, &st)
		for _, r := range st.Runden {
			if r.ID == rid && len(r.Karten) > 0 {
				t.Fatal("karten sichtbar, bevor MIMIK fertig ist")
			}
		}

		w.durchgang(ctx) // der Worker erzeugt die Karten

		// Beide raten.
		for _, k := range []*klient{a, b} {
			var s2 struct {
				Runden []RundeAus `json:"runden"`
			}
			k.ruf("GET", "/v1/state", nil, &s2)
			for _, r := range s2.Runden {
				if r.ID != rid {
					continue
				}
				if len(r.Karten) != 4 {
					t.Fatalf("%d karten statt 4", len(r.Karten))
				}
				for _, ka := range r.Karten {
					if ka.IstEcht != nil {
						t.Fatal("ist_echt wird vor der Aufloesung ausgeliefert")
					}
				}
				k.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
			}
		}
	}

	// Die Party hat jetzt Fakten im Dossier.
	pAlle, _ := s.PartyVon(mussSpieler(t, s, a.token).ID)
	f, _ := s.Fakten(string(pAlle.A), 100)
	if len(f) == 0 {
		t.Fatal("kein einziger Fakt im Dossier gelandet")
	}
	t.Logf("Dossier von Spieler A: %d Fakten", len(f))
}

func mussSpieler(t *testing.T, s *store.Store, token string) store.Player {
	t.Helper()
	p, err := s.SpielerZuToken(token)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Ein nil-Slice wird in Go zu null, und ein Klient mit strenger Typisierung
// bricht daran ab. Der Zustandsendpunkt darf deshalb nirgends null für eine
// Liste liefern – gefunden, als die App sich an "tags":null verschluckt hat.
func TestKeineNullListen(t *testing.T) {
	srv, _, _ := aufbauen(t)
	k := &klient{t: t, basis: srv.URL}
	var o struct {
		Token string `json:"token"`
	}
	k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "X"}, &o)
	k.token = o.Token
	k.ruf("POST", "/v1/parties", nil, nil)

	for _, pfad := range []string{"/v1/state", "/v1/tags", "/v1/dossier"} {
		var roh map[string]any
		k.ruf("GET", pfad, nil, &roh)
		pruefeKeinNull(t, pfad, roh)
	}
}

func pruefeKeinNull(t *testing.T, pfad string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for schluessel, wert := range x {
			if wert == nil {
				// Einzelwerte dürfen null sein (partner, match), Listen nicht.
				continue
			}
			pruefeKeinNull(t, pfad+"."+schluessel, wert)
		}
	case []any:
		for _, e := range x {
			pruefeKeinNull(t, pfad, e)
		}
	}
}

// Gezielt: die Felder, an denen die App hängengeblieben ist.
func TestListenfelderSindArrays(t *testing.T) {
	srv, _, _ := aufbauen(t)
	k := &klient{t: t, basis: srv.URL}
	var o struct {
		Token string `json:"token"`
	}
	k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "X"}, &o)
	k.token = o.Token
	k.ruf("POST", "/v1/parties", nil, nil)

	var st map[string]any
	k.ruf("GET", "/v1/state", nil, &st)
	party, _ := st["party"].(map[string]any)
	if party == nil {
		t.Fatal("keine party im zustand")
	}
	if _, ok := party["tags"].([]any); !ok {
		t.Fatalf("party.tags ist %#v, erwartet []", party["tags"])
	}
	for _, feld := range []string{"runden", "dran"} {
		if _, ok := st[feld].([]any); !ok {
			t.Fatalf("%s ist %#v, erwartet []", feld, st[feld])
		}
	}
}

// TestKonto deckt ab, was das Onboarding und die Einstellungen brauchen: Tags
// liegen am Spieler und stehen im Zustand, bevor es eine Party gibt; Umbenennen
// lässt das Dossier stehen; Alles-Löschen räumt wirklich auf.
func TestKonto(t *testing.T) {
	srv, s, _ := aufbauen(t)

	a := &klient{t: t, basis: srv.URL}
	var an struct {
		Token   string `json:"token"`
		Spieler struct {
			ID string `json:"id"`
		} `json:"spieler"`
	}
	a.ruf("POST", "/v1/devices", map[string]string{"spitzname": "Merlin"}, &an)
	a.token = an.Token

	// Ohne Party: Tags müssen trotzdem gehen und im Zustand auftauchen, sonst
	// könnte die App das Onboarding nicht vor die Party ziehen.
	if code := a.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil); code != 200 {
		t.Fatalf("tags ohne party: %d", code)
	}
	var z1 struct {
		Tags  []string  `json:"tags"`
		Party *struct{} `json:"party"`
	}
	a.ruf("GET", "/v1/state", nil, &z1)
	if len(z1.Tags) != 10 {
		t.Fatalf("state ohne party liefert %d tags, erwartet 10", len(z1.Tags))
	}
	if z1.Party != nil {
		t.Fatal("state erfindet eine party")
	}

	// Ein Fakt, damit es beim Löschen etwas zu löschen gibt.
	s.FaktHinzu(an.Spieler.ID, "runde-1", "Trinkt Kaffee schwarz.")

	// Umbenennen lässt das Dossier stehen: Es hängt an der ID, nicht am Namen.
	if code := a.ruf("POST", "/v1/me/name", map[string]string{"spitzname": "Merl"}, nil); code != 200 {
		t.Fatalf("umbenennen: %d", code)
	}
	var d struct {
		Fakten []string `json:"fakten"`
		Tags   []string `json:"tags"`
	}
	a.ruf("GET", "/v1/dossier", nil, &d)
	if len(d.Fakten) != 1 || len(d.Tags) != 10 {
		t.Fatalf("nach umbenennen: %d fakten, %d tags", len(d.Fakten), len(d.Tags))
	}

	// Dossier löschen nimmt die Fakten, aber nicht die Tags.
	if code := a.ruf("DELETE", "/v1/dossier", nil, nil); code != 200 {
		t.Fatalf("dossier löschen: %d", code)
	}
	a.ruf("GET", "/v1/dossier", nil, &d)
	if len(d.Fakten) != 0 {
		t.Fatalf("fakten überleben das löschen: %v", d.Fakten)
	}
	if len(d.Tags) != 10 {
		t.Fatalf("tags sind mitgelöscht worden, obwohl sie eine einstellung sind")
	}

	// Alles löschen verlangt den eigenen (neuen) Spitznamen.
	if code := a.ruf("POST", "/v1/me/delete", map[string]string{"bestaetigung": "Merlin"}, nil); code != 422 {
		t.Fatalf("alter name als bestätigung: %d, erwartet 422", code)
	}
	if code := a.ruf("POST", "/v1/me/delete", map[string]string{"bestaetigung": "Merl"}, nil); code != 200 {
		t.Fatalf("alles löschen: %d", code)
	}
	if code := a.ruf("GET", "/v1/state", nil, nil); code != 401 {
		t.Fatalf("token gilt nach dem löschen weiter: %d", code)
	}
	if _, err := s.Spieler(an.Spieler.ID); err == nil {
		t.Fatal("spieler steht nach dem löschen noch in der datenbank")
	}
}

// TestPartyMitLoeschen prüft, dass die geteilte Party mitgeht – und dass das,
// was dem Gegenüber allein gehört, stehen bleibt.
func TestPartyMitLoeschen(t *testing.T) {
	srv, s, _ := aufbauen(t)

	machen := func(name string) (*klient, string) {
		k := &klient{t: t, basis: srv.URL}
		var an struct {
			Token   string `json:"token"`
			Spieler struct {
				ID string `json:"id"`
			} `json:"spieler"`
		}
		k.ruf("POST", "/v1/devices", map[string]string{"spitzname": name}, &an)
		k.token = an.Token
		k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
		return k, an.Spieler.ID
	}
	a, idA := machen("A")
	b, idB := machen("B")

	var pa struct {
		Code string `json:"code"`
	}
	a.ruf("POST", "/v1/parties", nil, &pa)
	b.ruf("POST", "/v1/parties/join", map[string]string{"code": pa.Code}, nil)
	a.ruf("POST", "/v1/matches", nil, nil)
	s.FaktHinzu(idA, "r", "A mag Berge.")
	s.FaktHinzu(idB, "r", "B mag Meer.")

	if code := a.ruf("POST", "/v1/me/delete", map[string]string{"bestaetigung": "A"}, nil); code != 200 {
		t.Fatalf("löschen: %d", code)
	}

	// B ist noch da, mit Tags und Dossier – aber ohne Party.
	var d struct {
		Fakten []string `json:"fakten"`
		Tags   []string `json:"tags"`
	}
	if code := b.ruf("GET", "/v1/dossier", nil, &d); code != 200 {
		t.Fatalf("b ist mitgelöscht worden: %d", code)
	}
	if len(d.Fakten) != 1 || len(d.Tags) != 10 {
		t.Fatalf("b verliert eigene daten: %d fakten, %d tags", len(d.Fakten), len(d.Tags))
	}
	if _, err := s.PartyVon(idB); err == nil {
		t.Fatal("die party steht noch, obwohl eine seite gelöscht wurde")
	}
}

// TestNormalformKommtVomModell hält den Weg fest, den die saubere Fassung
// nimmt: Beim Absenden steht die regelbasierte Notfassung in der Datenbank,
// nach dem Worker die vom Modell geschriebene – und genau die ist die echte
// Karte. Liefe das auseinander, stünde auf der Karte etwas anderes als in der
// Chronik.
func TestNormalformKommtVomModell(t *testing.T) {
	srv, s, w := aufbauen(t)

	machen := func(name string) *klient {
		k := &klient{t: t, basis: srv.URL}
		var an struct {
			Token string `json:"token"`
		}
		k.ruf("POST", "/v1/devices", map[string]string{"spitzname": name}, &an)
		k.token = an.Token
		k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
		return k
	}
	a, b := machen("A"), machen("B")
	var pa struct {
		Code string `json:"code"`
	}
	a.ruf("POST", "/v1/parties", nil, &pa)
	b.ruf("POST", "/v1/parties/join", map[string]string{"code": pa.Code}, nil)
	a.ruf("POST", "/v1/matches", nil, nil)

	var st struct {
		Runden []RundeAus `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &st)
	rid := st.Runden[0].ID

	// Klein getippt, ohne Punkt – genau das, was eine Regel nicht reparieren
	// kann, weil sie kein Substantiv erkennt.
	roh := "ein selbstgemachtes kochbuch"
	if code := a.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": roh}, nil); code != 200 && code != 201 {
		t.Fatalf("antwort: %d", code)
	}

	// Vor dem Worker: die regelbasierte Notfassung, damit der Wartebildschirm
	// nicht leer ist.
	a.ruf("GET", "/v1/state", nil, &st)
	if got := st.Runden[0].MeineAntwort; got != "Ein selbstgemachtes kochbuch." {
		t.Fatalf("Notfassung ist %q", got)
	}

	b.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": "eine postkarte aus lissabon"}, nil)
	w.durchgang(context.Background())

	// Nach dem Worker: die Fassung aus dem Aufruf, in dem auch die Fälschungen
	// entstanden sind.
	a.ruf("GET", "/v1/state", nil, &st)
	if got := st.Runden[0].MeineAntwort; got != "Sauber: ein selbstgemachtes kochbuch." {
		t.Fatalf("nach dem Worker steht da %q", got)
	}

	// Und dieselbe Fassung steht auf der Karte, die das Gegenüber sieht.
	var stB struct {
		Runden []RundeAus `json:"runden"`
	}
	b.ruf("GET", "/v1/state", nil, &stB)
	gefunden := false
	for _, k := range stB.Runden[0].Karten {
		if k.Text == "Sauber: ein selbstgemachtes kochbuch." {
			gefunden = true
		}
	}
	if !gefunden {
		t.Fatalf("die Karte trägt nicht die Normalform: %+v", stB.Runden[0].Karten)
	}

	// Alle vier Karten müssen die Formprüfung bestehen - sonst fällt eine schon
	// durch ihre Schreibweise auf.
	for _, k := range stB.Runden[0].Karten {
		if m := mimik.Formmangel(k.Text); m != "" {
			t.Errorf("Karte %d (%q): %s", k.Pos, k.Text, m)
		}
	}
	_ = s
}
