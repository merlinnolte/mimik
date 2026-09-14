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
// Der Betriebsfall ist zwölf Fakten UND ein Profil - der Test setzte lange kein
// Profil, während der Worker eines füllt, und maß damit an der Wirklichkeit
// vorbei.
//
// Gemessen am 14.09.2026 (Zeichen System + Material): ohne Dossier 7.803, mit
// zwölf Fakten 8.964, mit zwölf Fakten und zwölf Merkmalen 9.945, mit vierzig
// Fakten 11.624. Gegenüber der Messung vom Vormittag 727 Zeichen mehr - das ist
// der Abschnitt, mit dem MIMIK begründet, woraus sie eine Fälschung gebaut hat.
// Er steht im SYSTEMPROMPT, also in dem Teil, den der Prefix-Cache zu einem
// Zehntel abrechnet; auf der Ausgabeseite kostet er rund 130 Token. Am Endpunkt gemessen sind 6.600 Zeichen rund 1.950
// Eingabetoken, also 3,4 Zeichen je Token - ein Kontextüberlauf kann von diesem
// Umfang nicht kommen, jedes brauchbare Modell trägt ein Vielfaches.
//
// Wichtiger als die Gesamtzahl: 6.435 Zeichen davon sind der Systemprompt und
// bei JEDEM Aufruf derselbe Text. Genau das trägt der Prefix-Cache des
// Anbieters (gemessen: 87 Prozent der Eingabe) - und deshalb darf variabler
// Text nie in den Systemprompt wandern.
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
	merkmal := "tagesablauf: steht früh auf und kocht vor der Arbeit Kaffee (wahrscheinlich)"
	for _, f := range []struct {
		fakten, merkmale int
	}{{0, 0}, {12, 0}, {12, 12}, {40, 0}} {
		fakten := make([]string, f.fakten)
		// Gedeckelt wie im Betrieb: MaxThemenImPrompt, nicht beliebig viele.
		themen := make([]string, MaxThemenImPrompt)
		merkmale := make([]string, f.merkmale)
		for i := range fakten {
			fakten[i] = fakt
		}
		for i := range themen {
			themen[i] = "irgendeinthema"
		}
		for i := range merkmale {
			merkmale[i] = merkmal
		}
		c.Faelschungen(t.Context(), "Was ist das Unvernünftigste, das du dir gekauft hast?",
			"Eine zweite Kaffeemühle, aber die erste mahlt zu grob",
			mimik.Dossier{Interessen: []string{"kaffee", "berge", "filme"},
				Fakten: fakten, Profil: merkmale, Gesperrt: themen,
				AntiBeispiele: []string{"Das ist eine spannende Frage!", "Am Ende zählt doch."}})
		t.Logf("%2d Fakten, %2d Merkmale -> %5d Zeichen (~%5d Token)",
			f.fakten, f.merkmale, groesse, groesse*10/34)
		// Zwölf Fakten und ein voller Profilblock sind der Betriebsfall.
		if f.fakten == 12 && f.merkmale == 12 && groesse > 10400 {
			t.Errorf("der Betriebsprompt ist auf %d Zeichen gewachsen", groesse)
		}
	}
}
