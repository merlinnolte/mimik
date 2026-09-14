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
	"sync/atomic"
	"testing"
	"time"

	"mimik/internal/game"
	"mimik/internal/mimik"
	"mimik/internal/seed"
	"mimik/internal/store"
)

// stubModell ahmt den OpenAI-kompatiblen Endpunkt nach. Es unterscheidet die
// beiden Prompts am System-Text und antwortet deterministisch – damit prüft der
// Test die Mechanik, nicht die Laune eines Modells.
// Die beiden Antworten aus TestReviewBautProfil. Stehen sie je zusammen in
// einem Reviewmaterial, ist die Trennung zwischen den Seiten kaputt.
const (
	geheimEineSeite      = "schwester hat den tisch gebaut"
	geheimDerAndereSeite = "zwetschgenkuchen im august"
)

func stubModell(t *testing.T) *httptest.Server {
	t.Helper()
	// Atomar, weil der Worker beide Kartensätze einer Runde nebenläufig baut
	// und dieses Stubmodell damit aus zwei Goroutinen gleichzeitig gerufen wird.
	var n atomic.Int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roh, _ := io.ReadAll(r.Body)
		var in struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		json.Unmarshal(roh, &in)
		system, user := in.Messages[0].Content, in.Messages[1].Content
		lauf := n.Add(1)

		// Der Testspieler benutzt einen eigenen, viel kleineren Prompt.
		if strings.Contains(system, "Du bist ein Mensch mit den unten genannten") {
			json_(w, 200, map[string]any{"choices": []map[string]any{{"message": map[string]string{
				"content": fmt.Sprintf(`{"antwort":"Der Zug um sieben, Runde %d."}`, lauf)}}}})
			return
		}

		// Das Review laeuft nach der Aufloesung und hat einen eigenen Prompt.
		// Das Material darueber wird hier gleich mitgeprueft: Es darf NICHTS
		// vom Partner enthalten - das Profil, das ein Mensch selbst lesen kann,
		// waere sonst ein Fenster in die Antworten des anderen.
		if strings.Contains(system, "Runden sind vorbei.") {
			// Jede Seite hat ihr eigenes Review. Im Material darf immer nur
			// EINE der beiden echten Antworten stehen - stuenden beide darin,
			// waere das Profil, das ein Mensch selbst lesen kann, ein Fenster
			// in die Antworten des anderen.
			if strings.Contains(user, geheimDerAndereSeite) && strings.Contains(user, geheimEineSeite) {
				t.Errorf("das review sieht beide antworten:\n%s", user)
			}
			json_(w, 200, map[string]any{"choices": []map[string]any{{"message": map[string]string{
				"content": `{"gewaehlt":"Die zweite, sie war am knappsten.",` +
					`"verworfen":"Zu rund, zu erklaerend.",` +
					`"merkmale":[{"beobachtung":"spricht von ihrer Schwester",` +
					`"merkmal":"Geschwister?","wert":"hat eine Schwester",` +
					`"urteil":"NEU","konfidenz":0.9}]}`}}}})
			return
		}

		// Die rohe Antwort steht im Material. Das Stubmodell "schreibt sie
		// sauber", indem es den Satzanfang großmacht - genug, um zu prüfen,
		// dass die Normalform aus diesem Aufruf bis auf die Karte durchläuft.
		i := strings.Index(user, "[echte_antwort_roh]\n")
		antwort := user[i+len("[echte_antwort_roh]\n"):]
		antwort = antwort[:strings.Index(antwort, "\n")]

		inhalt, _ := jsonString(map[string]any{
			"normalform": "Sauber: " + antwort,
			"fakt":       fmt.Sprintf("Hat in Runde %d etwas über sich verraten.", lauf),
			"sperre":     []string{"stubthema"},
			"antworten": []map[string]string{
				{"richtung": "a", "text": fmt.Sprintf("Zitronenfalter beobachten, %d Stück.", lauf),
					"begruendung": "grund-" + antwort},
				{"richtung": "b", "text": fmt.Sprintf("Rathausturm besteigen, ganz oben %d.", lauf),
					"begruendung": "grund-" + antwort},
				{"richtung": "c", "text": fmt.Sprintf("Werkzeugkisten sortieren bei Nummer %d.", lauf),
					"begruendung": "grund-" + antwort},
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

// rohtext holt eine Antwort als Text - fuer Pruefungen, die nicht nach einem
// Feld suchen, sondern nach einer Zeichenkette, die NIRGENDS stehen darf.
func (k *klient) rohtext(methode, pfad string) string {
	k.t.Helper()
	req, _ := http.NewRequest(methode, k.basis+pfad, nil)
	if k.token != "" {
		req.Header.Set("Authorization", "Bearer "+k.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		k.t.Fatalf("%s %s: %v", methode, pfad, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
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
	// Verschiedene Namen: Spitznamen sind eindeutig, und zwei Spieler "X"
	// kommen gar nicht mehr zustande.
	for i, k := range []*klient{a, b} {
		var out struct {
			Token string `json:"token"`
		}
		name := fmt.Sprintf("Spieler%d", i+1)
		if code := k.ruf("POST", "/v1/devices", map[string]string{"spitzname": name}, &out); code != 201 {
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
	pAlle, _ := s.ZuletztePartie(mussSpieler(t, s, a.token).ID)
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
	if xs, _ := s.PartienVon(idB); len(xs) != 0 {
		t.Fatalf("%d partien stehen noch, obwohl eine seite gelöscht wurde", len(xs))
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

// TestTagsWeitere hält fest, was der Knopf "Weitere" im Onboarding tun muss.
//
// Vorher tat er nichts Sinnvolles: TagVorschlaege filterte nur nach GESPEICHERTEN
// Tags, und im Onboarding ist noch nichts gespeichert – also kamen jedes Mal
// dieselben zwanzig. Auf dem Gerät gefunden.
func TestTagsWeitere(t *testing.T) {
	srv, _, _ := aufbauen(t)
	k := &klient{t: t, basis: srv.URL}
	var an struct {
		Token string `json:"token"`
	}
	k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "A"}, &an)
	k.token = an.Token

	type aus struct {
		Vorschlaege []string `json:"vorschlaege"`
		Gewaehlt    []string `json:"gewaehlt"`
		Mehr        bool     `json:"mehr"`
	}

	var erste, zweite aus
	k.ruf("GET", "/v1/tags", nil, &erste)
	k.ruf("GET", "/v1/tags?ab=20", nil, &zweite)

	if len(erste.Vorschlaege) != 20 {
		t.Fatalf("erste Seite hat %d Vorschläge", len(erste.Vorschlaege))
	}
	if !erste.Mehr {
		t.Fatal("nach der ersten Seite soll es weitergehen")
	}
	if len(zweite.Vorschlaege) == 0 {
		t.Fatal("die zweite Seite ist leer")
	}
	// Der eigentliche Fehler: zweimal dasselbe.
	schnitt := map[string]bool{}
	for _, x := range erste.Vorschlaege {
		schnitt[x] = true
	}
	for _, x := range zweite.Vorschlaege {
		if schnitt[x] {
			t.Fatalf("%q steht auf beiden Seiten", x)
		}
	}

	// Bis ans Ende blättern: Der Vorrat muss sich erschöpfen, sonst dreht der
	// Knopf sich im Kreis.
	gesehen, ab, runden := map[string]bool{}, 0, 0
	for {
		runden++
		if runden > 20 {
			t.Fatal("der Vorrat hört nicht auf")
		}
		var seite aus
		k.ruf("GET", fmt.Sprintf("/v1/tags?ab=%d", ab), nil, &seite)
		for _, x := range seite.Vorschlaege {
			if gesehen[x] {
				t.Fatalf("%q kam zweimal", x)
			}
			gesehen[x] = true
		}
		ab += len(seite.Vorschlaege)
		if !seite.Mehr {
			break
		}
	}
	// Gegen den echten Vorrat prüfen, nicht gegen eine Zahl im Test: Sonst
	// schlägt dieser Test jedes Mal fehl, wenn jemand Begriffe hinzufügt.
	if len(gesehen) != len(seed.Tags()) {
		t.Fatalf("%d von %d Begriffen erreichbar", len(gesehen), len(seed.Tags()))
	}

	// Gespeicherte Tags dürfen nicht noch einmal vorgeschlagen werden.
	k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
	var nachher aus
	k.ruf("GET", "/v1/tags", nil, &nachher)
	for _, x := range nachher.Vorschlaege {
		for _, g := range zehnTags() {
			if x == g {
				t.Fatalf("%q ist gewählt und wird trotzdem vorgeschlagen", x)
			}
		}
	}
	if len(nachher.Gewaehlt) != 10 {
		t.Fatalf("gewaehlt hat %d Einträge", len(nachher.Gewaehlt))
	}
}

// TestMatchAbbrechen: Das Match gehört beiden, also endet es für beide – auch
// mitten in einer Runde, in der die andere Seite noch schreibt.
func TestMatchAbbrechen(t *testing.T) {
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

	// A schreibt, B nicht – die Runde steht also mitten im Zug.
	var st struct {
		Runden []RundeAus `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &st)
	rid := st.Runden[0].ID
	a.ruf("POST", "/v1/rounds/"+rid+"/answer", map[string]string{"original": "irgendwas"}, nil)

	if code := b.ruf("POST", "/v1/matches/abbrechen", nil, nil); code != 200 {
		t.Fatalf("abbrechen: %d", code)
	}

	// Beide Seiten sehen dasselbe Ergebnis.
	for name, k := range map[string]*klient{"A": a, "B": b} {
		var z struct {
			Match struct {
				Ergebnis string `json:"ergebnis"`
			} `json:"match"`
		}
		k.ruf("GET", "/v1/state", nil, &z)
		if z.Match.Ergebnis != "ABGEBROCHEN" {
			t.Fatalf("%s sieht %q", name, z.Match.Ergebnis)
		}
	}

	// Der Worker lässt die Finger davon: Kein Modellaufruf für ein Spiel, das
	// niemand mehr spielt.
	offen, _ := s.OffeneRunden()
	if len(offen) != 0 {
		t.Fatalf("worker sieht noch %d offene runden", len(offen))
	}
	w.durchgang(context.Background())

	// Und ein neues Match lässt sich starten.
	if code := a.ruf("POST", "/v1/matches", nil, nil); code != 201 {
		t.Fatalf("neues match nach abbruch: %d", code)
	}
	// Ohne laufendes Match gibt es nichts abzubrechen – aber jetzt läuft ja eins.
	if code := a.ruf("POST", "/v1/matches/abbrechen", nil, nil); code != 200 {
		t.Fatalf("zweiter abbruch: %d", code)
	}
	if code := a.ruf("POST", "/v1/matches/abbrechen", nil, nil); code != 409 {
		t.Fatalf("abbruch ohne laufendes spiel: %d, erwartet 409", code)
	}
}

// TestPartyVerlassen: der Weg zurück auf den Party-Bildschirm. Vorher gab es
// keinen – wer einmal zu zweit war, kam nur über "Alles löschen" wieder heraus,
// und das nahm das Dossier mit.
func TestPartyVerlassen(t *testing.T) {
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

	if code := a.ruf("POST", "/v1/parties/verlassen", nil, nil); code != 200 {
		t.Fatalf("verlassen: %d", code)
	}

	// Beide sind draußen und sehen den Party-Bildschirm wieder (party == null).
	for name, k := range map[string]*klient{"A": a, "B": b} {
		var z struct {
			Party *struct{} `json:"party"`
			Tags  []string  `json:"tags"`
		}
		k.ruf("GET", "/v1/state", nil, &z)
		if z.Party != nil {
			t.Fatalf("%s steckt noch in einer Party", name)
		}
		if len(z.Tags) != 10 {
			t.Fatalf("%s hat seine Tags verloren", name)
		}
	}

	// Was den Spielern gehört, ist geblieben.
	for name, k := range map[string]*klient{"A": a, "B": b} {
		var d struct {
			Fakten []string `json:"fakten"`
		}
		if code := k.ruf("GET", "/v1/dossier", nil, &d); code != 200 {
			t.Fatalf("%s: dossier weg (%d)", name, code)
		}
		if len(d.Fakten) != 1 {
			t.Fatalf("%s: %d Fakten statt 1", name, len(d.Fakten))
		}
	}

	// Das laufende Match ist beendet, nicht verwaist.
	if _, err := s.AktivesMatch("egal"); err == nil {
		t.Fatal("es gibt noch ein aktives match")
	}

	// Und eine neue Party lässt sich gründen.
	var neu struct {
		Code string `json:"code"`
	}
	if code := a.ruf("POST", "/v1/parties", nil, &neu); code != 201 {
		t.Fatalf("neue party: %d", code)
	}
	if code := b.ruf("POST", "/v1/parties/join", map[string]string{"code": neu.Code}, nil); code != 200 {
		t.Fatalf("erneut beitreten: %d", code)
	}
	if code := a.ruf("POST", "/v1/matches", nil, nil); code != 201 {
		t.Fatalf("match in der neuen party: %d", code)
	}

	// Ohne Party gibt es nichts zu verlassen.
	c := &klient{t: t, basis: srv.URL}
	var an struct {
		Token string `json:"token"`
	}
	c.ruf("POST", "/v1/devices", map[string]string{"spitzname": "C"}, &an)
	c.token = an.Token
	if code := c.ruf("POST", "/v1/parties/verlassen", nil, nil); code != 409 {
		t.Fatalf("verlassen ohne party: %d, erwartet 409", code)
	}
}

// TestTestpartie: allein spielen. Der Testspieler antwortet und tippt, der
// Mensch merkt keinen Unterschied zu einem zweiten Telefon.
func TestTestpartie(t *testing.T) {
	srv, s, w := aufbauen(t)
	ctx := context.Background()

	k := &klient{t: t, basis: srv.URL}
	var an struct {
		Token string `json:"token"`
	}
	k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "Merlin"}, &an)
	k.token = an.Token
	k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)

	var bei struct {
		Test bool `json:"test"`
	}
	if code := k.ruf("POST", "/v1/parties/join", map[string]string{"code": "test"}, &bei); code != 200 {
		t.Fatalf("TEST beitreten: %d", code)
	}
	if !bei.Test {
		t.Fatal("der server sagt nicht, dass es eine testpartie ist")
	}

	// Die Party ist sofort vollständig – kein Warten auf jemanden.
	var z struct {
		Party struct {
			Partner *struct {
				Spitzname string `json:"spitzname"`
			} `json:"partner"`
		} `json:"party"`
	}
	k.ruf("GET", "/v1/state", nil, &z)
	if z.Party.Partner == nil {
		t.Fatal("die testpartie hat keinen partner")
	}
	t.Logf("Gegenüber: %s", z.Party.Partner.Spitzname)

	if code := k.ruf("POST", "/v1/matches", nil, nil); code != 201 {
		t.Fatalf("match: %d", code)
	}

	// Eine ganze Runde, ohne zweites Gerät.
	var st struct {
		Runden []RundeAus `json:"runden"`
	}
	k.ruf("GET", "/v1/state", nil, &st)
	rid := st.Runden[0].ID
	k.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": "der stapel zeitschriften neben dem sofa"}, nil)

	// Erster Takt: der Testspieler antwortet. Zweiter: MIMIK baut die Karten.
	w.durchgang(ctx)

	// Und zwar NUR in der Runde, die dran ist. Ein Match legt seine Runden im
	// Voraus an; ohne diese Einschränkung beantwortete der Testspieler alle auf
	// einmal – Modellaufrufe für Runden, die vielleicht nie gespielt werden.
	var antworten int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM answers a JOIN bots b ON b.player_id = a.player_id`).
		Scan(&antworten); err != nil {
		t.Fatal(err)
	}
	if antworten != 1 {
		t.Fatalf("der Testspieler hat %d Runden auf einmal beantwortet", antworten)
	}

	w.durchgang(ctx)

	k.ruf("GET", "/v1/state", nil, &st)
	if len(st.Runden[0].Karten) != 4 {
		t.Fatalf("nach zwei Takten %d Karten, Zustand %q, Fehler %q",
			len(st.Runden[0].Karten), st.Runden[0].Zustand, st.Runden[0].Fehler)
	}

	// Der Mensch tippt, der Testspieler zieht nach, die Runde löst auf.
	k.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
	w.durchgang(ctx)
	k.ruf("GET", "/v1/state", nil, &st)
	if st.Runden[0].Zustand != game.Aufgeloest {
		t.Fatalf("runde steht auf %q statt AUFGELOEST", st.Runden[0].Zustand)
	}
	a := st.Runden[0].Aufloesung
	if a == nil {
		t.Fatal("keine auflösung")
	}
	// Der spannendste Teil der Auflösung: Wofuer hat mich die andere Seite
	// gehalten? Ohne Text steht in der App eine leere Karte.
	if a.PartnerTippText == "" {
		t.Fatal("die auflösung sagt nicht, welche karte die andere seite fuer echt hielt")
	}
	if a.MeineEchte == "" {
		t.Fatal("die auflösung nennt die eigene echte antwort nicht")
	}

	_ = s
	t.Logf("Runde aufgelöst, Tipp des Menschen richtig: %v", a.MeinTippRichtig)
}

// TestKartenLaufenVor: Der Satz über A entsteht, sobald A geantwortet hat –
// nicht erst, wenn auch B fertig ist. Sonst lägen beide Modellaufrufe hinter
// dem zweiten Menschen, und der säße die volle Wartezeit ab.
func TestKartenLaufenVor(t *testing.T) {
	srv, s, w := aufbauen(t)
	ctx := context.Background()

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
	b, _ := machen("B")
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

	// Nur A antwortet.
	a.ruf("POST", "/v1/rounds/"+rid+"/answer", map[string]string{"original": "Der Stapel neben dem Sofa"}, nil)
	w.durchgang(ctx)

	// As Satz steht schon, obwohl B noch nicht getippt hat.
	rd, err := s.Runde(rid)
	if err != nil {
		t.Fatal(err)
	}
	party, _, _ := s.PartyVonRunde(rid)
	if len(rd.Karten[game.PlayerID(idA)]) != 4 {
		t.Fatalf("As Karten stehen noch nicht: %d", len(rd.Karten[game.PlayerID(idA)]))
	}
	if rd.Ableiten(party) != game.SchreibenWartet {
		t.Fatalf("Zustand ist %q, erwartet SCHREIBEN_WARTET", rd.Ableiten(party))
	}

	// Und niemand sieht sie: KartenFuer gibt vor dem Raten nichts heraus.
	b.ruf("GET", "/v1/state", nil, &st)
	if len(st.Runden[0].Karten) != 0 {
		t.Fatalf("B sieht %d Karten, bevor er geantwortet hat", len(st.Runden[0].Karten))
	}

	// Jetzt B – danach fehlt nur noch SEIN Satz.
	b.ruf("POST", "/v1/rounds/"+rid+"/answer", map[string]string{"original": "Eine Postkarte ohne Anlass"}, nil)
	w.durchgang(ctx)

	a.ruf("GET", "/v1/state", nil, &st)
	if st.Runden[0].Zustand != game.Raten {
		t.Fatalf("Zustand ist %q statt RATEN", st.Runden[0].Zustand)
	}
	if len(st.Runden[0].Karten) != 4 {
		t.Fatalf("%d Karten", len(st.Runden[0].Karten))
	}
}

// TestReviewBautProfil: Nach einer aufgeloesten Runde entsteht ein Profil, die
// Konfidenz ist gedeckelt, und ein zweiter Durchlauf reviewt dieselbe Runde
// nicht noch einmal.
func TestReviewBautProfil(t *testing.T) {
	srv, _, w := aufbauen(t)
	ctx := context.Background()

	a := &klient{t: t, basis: srv.URL}
	b := &klient{t: t, basis: srv.URL}
	for i, k := range []*klient{a, b} {
		var out struct {
			Token string `json:"token"`
		}
		k.ruf("POST", "/v1/devices", map[string]string{
			"spitzname": fmt.Sprintf("Review%d", i)}, &out)
		k.token = out.Token
		k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
	}
	var neu struct {
		Code string `json:"code"`
	}
	a.ruf("POST", "/v1/parties", nil, &neu)
	b.ruf("POST", "/v1/parties/join", map[string]string{"code": neu.Code}, nil)
	a.ruf("POST", "/v1/matches", nil, nil)

	var st struct {
		Runden []RundeAus `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &st)
	rid := st.Runden[0].ID
	a.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": "meine schwester hat den tisch gebaut"}, nil)
	b.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": geheimDerAndereSeite}, nil)
	w.durchgang(ctx)
	a.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
	b.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)

	// Ein Buendel wartet normalerweise auf drei Runden. Fuer diesen Test zaehlt
	// die einzelne, also die Schonfrist auf null.
	store.ReviewSchonfrist = 0
	t.Cleanup(func() { store.ReviewSchonfrist = 15 * time.Minute })
	w.Reviews(ctx)

	var d struct {
		Profil []struct {
			Merkmal string `json:"merkmal"`
			Wert    string `json:"wert"`
			Stand   string `json:"stand"`
			Belege  int    `json:"belege"`
		} `json:"profil"`
		Verlauf []map[string]string `json:"profil_verlauf"`
	}
	a.ruf("GET", "/v1/dossier", nil, &d)
	if len(d.Profil) != 1 {
		t.Fatalf("%d merkmale im profil statt 1: %+v", len(d.Profil), d.Profil)
	}
	if d.Profil[0].Merkmal != "geschwister" {
		t.Fatalf("schlüssel %q nicht normalisiert", d.Profil[0].Merkmal)
	}
	// Das Stubmodell schlägt 0.9 vor. Ein einzelner Beleg darf daraus keine
	// Gewissheit machen - die Arithmetik steht in Go, nicht im Prompt.
	if d.Profil[0].Stand != "wahrscheinlich" {
		t.Fatalf("stand %q trotz nur eines belegs", d.Profil[0].Stand)
	}
	if len(d.Verlauf) == 0 {
		t.Fatal("kein verlaufseintrag")
	}

	// Das Profil ist privat: Die Gegenseite sieht davon nichts.
	roh := b.rohtext("GET", "/v1/state")
	if strings.Contains(roh, "geschwister") || strings.Contains(roh, "Schwester") {
		t.Fatalf("das profil steht im zustand der gegenseite:\n%s", roh)
	}

	// Zweiter Durchlauf: Die Runde ist abgehakt und wird nicht erneut geprüft.
	vorher := len(d.Verlauf)
	w.Reviews(ctx)
	a.ruf("GET", "/v1/dossier", nil, &d)
	if len(d.Verlauf) != vorher {
		t.Fatalf("die runde wurde ein zweites mal reviewt: %d statt %d einträge",
			len(d.Verlauf), vorher)
	}
}

// TestFrischePartieTraegtIhreID: Eine gegruendete Partie hat noch kein Match.
// Der Zustand steigt dann vorzeitig aus - und muss trotzdem sagen, um welche
// Partie es geht. Ohne das Feld erkennt die App ihre geoeffnete Partie nicht
// wieder und bleibt im Ladebildschirm stehen. Genau so gesehen, in 0.8.
func TestFrischePartieTraegtIhreID(t *testing.T) {
	srv, _, _ := aufbauen(t)
	k := &klient{t: t, basis: srv.URL}
	var out struct {
		Token string `json:"token"`
	}
	k.ruf("POST", "/v1/devices", map[string]string{"spitzname": "Frisch"}, &out)
	k.token = out.Token

	var neu struct {
		PartyID string `json:"party_id"`
	}
	if code := k.ruf("POST", "/v1/parties", nil, &neu); code != 201 {
		t.Fatalf("party: %d", code)
	}

	var st struct {
		PartyID string `json:"party_id"`
		Party   struct {
			Code    string    `json:"code"`
			Partner *struct{} `json:"partner"`
		} `json:"party"`
	}
	k.ruf("GET", "/v1/parties/"+neu.PartyID+"/state", nil, &st)
	if st.PartyID != neu.PartyID {
		t.Fatalf("party_id fehlt oder ist falsch: %q statt %q", st.PartyID, neu.PartyID)
	}
	if st.Party.Code == "" {
		t.Fatal("der einladungscode fehlt im zustand")
	}
	// Und dasselbe ueber den Altpfad, solange es nur diese eine Partie gibt.
	st.PartyID = ""
	k.ruf("GET", "/v1/state", nil, &st)
	if st.PartyID != neu.PartyID {
		t.Fatalf("altpfad ohne party_id: %q", st.PartyID)
	}
}

// TestReviewBuendeltDreiRunden: Der Systemprompt des Reviews ist 3.385 Zeichen
// und ging vorher je Runde einmal hinaus. Drei Runden zusammen kosten EINEN
// Aufruf - das ist der Sparhebel, und dieser Test haelt ihn fest.
func TestReviewBuendeltDreiRunden(t *testing.T) {
	srv, s, w := aufbauen(t)
	ctx := context.Background()

	a := &klient{t: t, basis: srv.URL}
	b := &klient{t: t, basis: srv.URL}
	for i, k := range []*klient{a, b} {
		var out struct {
			Token string `json:"token"`
		}
		k.ruf("POST", "/v1/devices", map[string]string{
			"spitzname": fmt.Sprintf("Buendel%d", i)}, &out)
		k.token = out.Token
		k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
	}
	var neu struct {
		Code string `json:"code"`
	}
	a.ruf("POST", "/v1/parties", nil, &neu)
	b.ruf("POST", "/v1/parties/join", map[string]string{"code": neu.Code}, nil)
	a.ruf("POST", "/v1/matches", nil, nil)

	// Drei Runden bis zur Auflösung spielen.
	for i := 0; i < 3; i++ {
		var st struct {
			Runden []RundeAus `json:"runden"`
		}
		a.ruf("GET", "/v1/state", nil, &st)
		var rid string
		for _, r := range st.Runden {
			if r.Zustand != game.Aufgeloest {
				rid = r.ID
				break
			}
		}
		if rid == "" {
			t.Fatalf("runde %d: keine offene runde", i+1)
		}
		a.ruf("POST", "/v1/rounds/"+rid+"/answer",
			map[string]string{"original": fmt.Sprintf("antwort a in runde %d", i+1)}, nil)
		b.ruf("POST", "/v1/rounds/"+rid+"/answer",
			map[string]string{"original": fmt.Sprintf("antwort b in runde %d", i+1)}, nil)
		w.durchgang(ctx)
		a.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
		b.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 2}, nil)
	}

	// Sechs offene Reviews: drei Runden mal zwei Spieler. Vorher waren das
	// sechs Modellaufrufe, jetzt zwei - einer je Spieler.
	vorher := reviewaufrufe(t, s)
	w.Reviews(ctx)
	if got := reviewaufrufe(t, s) - vorher; got != 2 {
		t.Fatalf("%d reviewaufrufe fuer sechs runden statt 2", got)
	}
	var fertig int
	s.DB().QueryRow(`SELECT COUNT(*) FROM reviews WHERE fertig = 1`).Scan(&fertig)
	if fertig != 6 {
		t.Fatalf("%d runden als fertig vermerkt statt 6", fertig)
	}
	// Und jedes Buendel gehoerte genau einem Spieler: Zwei zu mischen heisst,
	// ein Merkmal dem falschen Menschen zuzuschreiben.
	var gemischt int
	s.DB().QueryRow(`SELECT COUNT(*) FROM profil_verlauf`).Scan(&gemischt)
	if gemischt == 0 {
		t.Fatal("kein verlaufseintrag - das buendel hat nichts gelernt")
	}
}

// reviewaufrufe zaehlt, was die Buchhaltung ueber Reviews sagt. Nebenbei der
// Nachweis, dass die Tabelle ueberhaupt gefuellt wird.
func reviewaufrufe(t *testing.T, s *store.Store) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM aufrufe WHERE zweck = 'review'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestBegruendungNurUeberMichSelbst: MIMIK sagt, woraus sie eine Faelschung
// gebaut hat - aber nur dem Menschen, ueber den die Karte ist.
//
// Die Begruendungen der Karten UEBER DAS GEGENUEBER zitieren dessen Dossier:
// Fakten, die es dem Spiel erzaehlt hat, unter Umstaenden in einer ganz anderen
// Partie. Sie duerfen den Ratebildschirm nie erreichen.
func TestBegruendungNurUeberMichSelbst(t *testing.T) {
	srv, _, w := aufbauen(t)
	ctx := context.Background()

	a := &klient{t: t, basis: srv.URL}
	b := &klient{t: t, basis: srv.URL}
	for i, k := range []*klient{a, b} {
		var out struct {
			Token string `json:"token"`
		}
		k.ruf("POST", "/v1/devices", map[string]string{
			"spitzname": fmt.Sprintf("Grund%d", i)}, &out)
		k.token = out.Token
		k.ruf("PUT", "/v1/tags", map[string]any{"tags": zehnTags()}, nil)
	}
	var neu struct {
		Code string `json:"code"`
	}
	a.ruf("POST", "/v1/parties", nil, &neu)
	b.ruf("POST", "/v1/parties/join", map[string]string{"code": neu.Code}, nil)
	a.ruf("POST", "/v1/matches", nil, nil)

	var st struct {
		Runden []RundeAus `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &st)
	rid := st.Runden[0].ID
	a.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": geheimEineSeite}, nil)
	b.ruf("POST", "/v1/rounds/"+rid+"/answer",
		map[string]string{"original": geheimDerAndereSeite}, nil)
	w.durchgang(ctx)

	// Die Trennlinie: A sieht die Begruendungen zu SEINEM eigenen Satz - das
	// ist der Klonblick, und es ist sein Material. Die Begruendungen zum Satz
	// ueber B sieht A nie: Die zitieren BS Dossier.
	roh := a.rohtext("GET", "/v1/state")
	if !strings.Contains(roh, "grund-"+geheimEineSeite) {
		t.Fatalf("der eigene klonsatz kommt ohne begruendung:\n%s", roh)
	}
	if strings.Contains(roh, "grund-"+geheimDerAndereSeite) {
		t.Fatalf("die begruendung ueber die gegenseite ist bei a gelandet:\n%s", roh)
	}
	// Und der Ratesatz selbst traegt keine: Dort stehen die Karten ueber B.
	var vorm struct {
		Runden []struct {
			Karten []struct {
				Begruendung string `json:"begruendung"`
			} `json:"karten"`
		} `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &vorm)
	for _, k := range vorm.Runden[0].Karten {
		if k.Begruendung != "" {
			t.Fatalf("eine karte ueber die gegenseite traegt eine begruendung: %q", k.Begruendung)
		}
	}

	// Der Partner faellt auf eine Faelschung herein, a liegt richtig.
	var meine struct {
		Runden []RundeAus `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &meine)
	var echt int
	for _, k := range meine.Runden[0].Karten {
		_ = k
	}
	a.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
	b.ruf("POST", "/v1/rounds/"+rid+"/guess", map[string]int{"pos": 1}, nil)
	_ = echt

	// Nach der Auflösung: a erfaehrt die Begruendung der Karte, auf die b
	// hereingefallen ist - und die ist aus AS eigenem Material gebaut.
	var auf struct {
		Runden []struct {
			Aufloesung *struct {
				PartnerRichtig   bool   `json:"partner_richtig"`
				PartnerTippText  string `json:"partner_tipp_text"`
				PartnerTippGrund string `json:"partner_tipp_grund"`
			} `json:"aufloesung"`
		} `json:"runden"`
	}
	a.ruf("GET", "/v1/state", nil, &auf)
	al := auf.Runden[0].Aufloesung
	if al == nil {
		t.Fatal("keine auflösung")
	}
	if !al.PartnerRichtig {
		if al.PartnerTippGrund == "" {
			t.Fatal("die begruendung der karte, auf die der partner hereinfiel, fehlt")
		}
		if !strings.Contains(al.PartnerTippGrund, geheimEineSeite) {
			t.Fatalf("die begruendung stammt nicht aus dem eigenen material: %q",
				al.PartnerTippGrund)
		}
	}
	// Und die Begruendungen ueber B tauchen bei A weiterhin nicht auf.
	roh = a.rohtext("GET", "/v1/state")
	if strings.Contains(roh, "grund-"+geheimDerAndereSeite) {
		t.Fatalf("die begruendung ueber die gegenseite ist bei a gelandet:\n%s", roh)
	}
}
