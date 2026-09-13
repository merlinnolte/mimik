package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mimik/internal/mimik"
)

// Der Prompt darf nicht unbemerkt wachsen.
//
// Gemessen am 13.09.2026: ohne Dossier 5.578 Zeichen, mit zwölf Fakten 7.155,
// mit vierzig 10.711 – also rund 3.600 Token im schlimmsten Fall. Das ist der
// Grund, warum ein gemeldeter Kontextüberlauf NICHT vom Umfang kommen kann:
// Jedes brauchbare Modell trägt ein Vielfaches davon. Der Test hält die Zahl
// fest, damit das so bleibt, wenn jemand dem Prompt etwas hinzufügt.
func TestPromptbleibtklein(t *testing.T) {
	var groesse int
	modell := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roh, _ := io.ReadAll(r.Body)
		var in struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		json.Unmarshal(roh, &in)
		groesse = len(in.Messages[0].Content) + len(in.Messages[1].Content)
		json_(w, 200, map[string]any{"choices": []map[string]any{
			{"message": map[string]string{"content": `{"normalform":"x","fakt":"f","sperre":[],` +
				`"antworten":[{"anker":"a","text":"Eins."},{"anker":"b","text":"Zwei."},{"anker":"c","text":"Drei."}]}`}}}})
	}))
	defer modell.Close()

	c := mimik.NeuAusUmgebung()
	c.BaseURL, c.APIKey, c.Model, c.JSONMode = modell.URL, "t", "stub", false

	fakt := "Besitzt ein Rennrad, fährt es etwa dreimal im Jahr und empfindet den Kauf nicht als Fehler."
	for _, n := range []int{0, 12, 40} {
		fakten := make([]string, n)
		themen := make([]string, n*2)
		for i := range fakten {
			fakten[i] = fakt
		}
		for i := range themen {
			themen[i] = "irgendeinthema"
		}
		c.Faelschungen(t.Context(), "Was ist das Unvernünftigste, das du dir gekauft hast?",
			"Noch eine kleine Spielkonsole, aber ich habe viel Spaß damit",
			[]string{"kaffee", "berge", "filme"},
			mimik.Dossier{Fakten: fakten, Gesperrt: themen,
				AntiBeispiele: []string{"Das ist eine spannende Frage!", "Am Ende zählt doch."}})
		t.Logf("%2d Fakten -> %5d Zeichen (~%5d Token)", n, groesse, groesse/3)
		// Zwölf Fakten sind der Betriebsfall (siehe worker.go).
		if n == 12 && groesse > 9000 {
			t.Errorf("der Prompt ist auf %d Zeichen gewachsen", groesse)
		}
	}
}
