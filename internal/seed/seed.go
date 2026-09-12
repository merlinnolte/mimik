// Package seed hält den Startvorrat an Fragen und Tags. Im MVP ersetzt er die
// Prompts A und E: 60 Fragen reichen für mehrere Matches, 68 Tags für die
// Kalibrierung. Beides liegt im Binary, damit der Server ohne Netz startet.
package seed

import (
	_ "embed"
	"encoding/json"
)

//go:embed fragen.json
var fragenRoh []byte

//go:embed tags.json
var tagsRoh []byte

type Frage struct {
	Text   string `json:"text"`
	Rubrik string `json:"rubrik"`
}

func Fragen() []Frage {
	var f []Frage
	if err := json.Unmarshal(fragenRoh, &f); err != nil {
		panic("seed: fragen.json unlesbar: " + err.Error())
	}
	return f
}

func Tags() []string {
	var t []string
	if err := json.Unmarshal(tagsRoh, &t); err != nil {
		panic("seed: tags.json unlesbar: " + err.Error())
	}
	return t
}
