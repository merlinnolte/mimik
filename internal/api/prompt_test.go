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
// Gemessen am 14.09.2026 (Zeichen System + Material): ohne Dossier 10.366, mit
// zwölf Fakten 11.527, mit zwölf Fakten und zwölf Merkmalen 12.508, mit vierzig
// Fakten 14.187. Am Endpunkt gemessen sind 3,4 Zeichen ein Token.
//
// Der Prompt ist an diesem einen Tag von 6.435 auf 9.500 Zeichen gewachsen -
// und jeder Zuwachs steht für etwas Gemessenes:
//
//	+727  Begründung, woraus eine Fälschung gebaut ist
//	+869  VERLANGT: dass sie die Frage überhaupt beantwortet
//	      (vorher lieferten von 15 Karten mehrere etwas anderes als das Gefragte)
//	+584  Längenfenster (Karten mit 128 Zeichen gegen 48)
//	+900  Stilregeln aus der Lügenforschung: keine Begründung, kein Fazit,
//	      gleicher Bau, nichts Überprüfbares erfinden
//
// Dass das vertretbar ist, hängt an zwei Messungen, nicht an Geschmack. Erstens:
// 87 Prozent der Eingabe kommen aus dem Prefix-Cache, und gewachsen ist genau
// der cachefähige Teil. Zweitens, und das ist das stärkere Argument: Die
// Wiederholungen sind von 2,0 auf 1,2 Versuche je Kartensatz gefallen. Ein
// Wiederholungsversuch ist ein GANZER Aufruf; die zusätzlichen 3.000 Zeichen
// kosten im Cache einen Bruchteil davon. Der längere Prompt ist billiger als
// die Fehler, die er verhindert.
//
// Es bleibt trotzdem eine Grenze: Irgendwann leidet nicht die Rechnung, sondern
// die Aufmerksamkeit des Modells. Wer hier etwas hinzufügt, nimmt besser etwas
// mit - und weist mit einer Messung nach, dass es sich lohnt.
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
		if f.fakten == 12 && f.merkmale == 12 && groesse > 13000 {
			t.Errorf("der Betriebsprompt ist auf %d Zeichen gewachsen", groesse)
		}
	}
}
