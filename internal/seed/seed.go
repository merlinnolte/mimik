// Package seed haelt den Vorrat an Fragen und Tags, eingebettet, damit der
// Server ohne Netz startet.
//
// fragen.json ist die Wahrheit ueber den Fragenvorrat, nicht bloss sein
// Startwert: store.fragenAbgleichen richtet die Tabelle bei jedem Start nach
// dieser Datei - einsaeen, korrigieren, zuruecknehmen.
//
// Reines Datenpaket - es kennt nur embed und encoding/json. Die Pruefungen
// darueber stehen in fragen_test.go (externes Testpaket, damit dieses Paket
// nichts hinzulernt) und das Mass dahinter in internal/mimik/frage.go.
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
	// Kennung ist die fachliche Identitaet einer Frage - ausdruecklich NICHT
	// ihr Text.
	//
	// Warum: fragen_vergeben merkt sich, welche frage_id ein Mensch hatte.
	// Haengt die ID am Text, dann bekommt eine Frage nach der Korrektur eines
	// Tippfehlers eine neue ID - und jeder, der sie schon beantwortet hat, ist
	// wieder fuer sie berechtigt. Ein Tippfehler waere damit unbehebbar.
	//
	// Eine Kennung ist deshalb dauerhaft. Sie umbenennen heisst, die Frage
	// durch eine neue zu ersetzen; den Text darunter zu aendern kostet nichts.
	Kennung string `json:"kennung"`
	Text    string `json:"text"`
	Rubrik  string `json:"rubrik"`
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
