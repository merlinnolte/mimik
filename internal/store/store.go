// Package store hält den gesamten Zustand in einer SQLite-Datei. Bei zwei
// Nutzern ist das die richtige Größe: ein Volume, Sicherung per cp.
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	mrand "math/rand/v2"
	"strings"
	"time"

	"mimik/internal/game"
	"mimik/internal/seed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var (
	ErrNichtGefunden   = errors.New("nicht gefunden")
	ErrCodeUngueltig   = errors.New("einladungscode ungültig oder abgelaufen")
	ErrPartyVoll       = errors.New("party hat bereits zwei mitglieder")
	ErrSchonDrin       = errors.New("spieler ist bereits in einer party")
	ErrSchonMitglied   = errors.New("du bist in dieser party schon dabei")
	ErrSchonPartner    = errors.New("mit dieser person läuft schon eine partie")
	ErrKeinZugriff     = errors.New("keine party von dir")
	ErrNameVergeben    = errors.New("dieser name ist vergeben")
	ErrSchonEingeladen = errors.New("die einladung steht schon")
	ErrSelbst          = errors.New("dich selbst kannst du nicht einladen")
	ErrZuWenigTags     = errors.New("mindestens 10 tags nötig")
)

type Store struct{ db *sql.DB }

func Open(pfad string) (*Store, error) {
	db, err := sql.Open("sqlite", pfad+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite mit einem Schreiber: einfacher als jede Sperrlogik
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{db: db}
	return s, s.fragenSaeen()
}

func (s *Store) Close() error { return s.db.Close() }

// DB gibt die Verbindung heraus – nur für Tests, die etwas nachzählen wollen,
// wofür es keine eigene Methode geben soll.
func (s *Store) DB() *sql.DB { return s.db }

func jetzt() string { return time.Now().UTC().Format(time.RFC3339) }

func id() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) fragenSaeen() error {
	for _, f := range seed.Fragen() {
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO fragen_pool (text, rubrik) VALUES (?, ?)`, f.Text, f.Rubrik); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- Spieler ---

type Player struct {
	ID        string `json:"id"`
	Spitzname string `json:"spitzname"`
}

func hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// SpielerAnlegen erzeugt Spieler und Gerät und gibt das Klartext-Token zurück.
// Gespeichert wird nur der Hash.
func (s *Store) SpielerAnlegen(spitzname string) (Player, string, error) {
	p := Player{ID: id(), Spitzname: strings.TrimSpace(spitzname)}
	if p.Spitzname == "" {
		p.Spitzname = "Spieler"
	}
	token := id() + id()
	tx, err := s.db.Begin()
	if err != nil {
		return Player{}, "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO players (id, spitzname, erstellt_am) VALUES (?,?,?)`,
		p.ID, p.Spitzname, jetzt()); err != nil {
		return Player{}, "", err
	}
	if _, err := tx.Exec(`INSERT INTO devices (id, player_id, token_hash, erstellt_am) VALUES (?,?,?,?)`,
		id(), p.ID, hash(token), jetzt()); err != nil {
		return Player{}, "", err
	}
	// Der Name wird gleich beansprucht. Ein neuer Spieler soll nie in den
	// Dublettenzustand hineingeboren werden: Scheitert der Anspruch, entsteht
	// gar kein Spieler, statt einer, der sich spaeter umbenennen muss.
	if _, err := tx.Exec(
		`INSERT INTO namen (player_id, normal, spitzname, erstellt_am) VALUES (?,?,?,?)`,
		p.ID, NameNormal(p.Spitzname), p.Spitzname, jetzt()); err != nil {
		return Player{}, "", ErrNameVergeben
	}
	return p, token, tx.Commit()
}

func (s *Store) SpielerZuToken(token string) (Player, error) {
	var p Player
	err := s.db.QueryRow(
		`SELECT p.id, p.spitzname FROM players p
		   JOIN devices d ON d.player_id = p.id WHERE d.token_hash = ?`, hash(token)).
		Scan(&p.ID, &p.Spitzname)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNichtGefunden
	}
	return p, err
}

func (s *Store) Spieler(pid string) (Player, error) {
	var p Player
	err := s.db.QueryRow(`SELECT id, spitzname FROM players WHERE id = ?`, pid).Scan(&p.ID, &p.Spitzname)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNichtGefunden
	}
	return p, err
}

// ------------------------------------------------------------------ Party ---

func code6() string {
	const zeichen = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // ohne I, O, 0, 1
	b := make([]byte, 6)
	rand.Read(b)
	out := make([]byte, 6)
	for i := range b {
		out[i] = zeichen[int(b[i])%len(zeichen)]
	}
	return string(out)
}

// MaxOffeneCodes deckelt, wie viele unerloeste Einladungscodes ein Spieler
// gleichzeitig herumliegen haben darf. Ohne Deckel legt ein Daumen auf dem
// Knopf beliebig viele halbe Partien an, die nie jemand betritt.
const MaxOffeneCodes = 5

func (s *Store) PartyAnlegen(pid string) (game.Party, string, error) {
	var offen int
	s.db.QueryRow(
		`SELECT COUNT(*) FROM party_members pm JOIN parties pa ON pa.id = pm.party_id
		  WHERE pm.player_id = ? AND pa.code IS NOT NULL`, pid).Scan(&offen)
	if offen >= MaxOffeneCodes {
		return game.Party{}, "", ErrSchonDrin
	}
	c := code6()
	pa := game.Party{ID: id(), A: game.PlayerID(pid)}
	tx, err := s.db.Begin()
	if err != nil {
		return pa, "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO parties (id, code, code_bis, erstellt_am) VALUES (?,?,?,?)`,
		pa.ID, c, time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339), jetzt()); err != nil {
		return pa, "", err
	}
	if _, err := tx.Exec(`INSERT INTO party_members (party_id, player_id, seite) VALUES (?,?,'A')`,
		pa.ID, pid); err != nil {
		return pa, "", err
	}
	return pa, c, tx.Commit()
}

func (s *Store) PartyBeitreten(c, pid string) (game.Party, error) {
	var partyID, bis string
	err := s.db.QueryRow(`SELECT id, code_bis FROM parties WHERE code = ?`, strings.ToUpper(strings.TrimSpace(c))).
		Scan(&partyID, &bis)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Party{}, ErrCodeUngueltig
	}
	if err != nil {
		return game.Party{}, err
	}
	if t, e := time.Parse(time.RFC3339, bis); e == nil && time.Now().UTC().After(t) {
		return game.Party{}, ErrCodeUngueltig
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM party_members WHERE party_id = ?`, partyID).Scan(&n)
	if n >= 2 {
		return game.Party{}, ErrPartyVoll
	}
	// Dass man der eigenen Partie nicht beitreten kann, verhinderte bisher die
	// Wache "schon in einer Party". Ohne sie waere der eigene Code der kuerzeste
	// Weg, gegen sich selbst zu spielen.
	var schon int
	s.db.QueryRow(`SELECT COUNT(*) FROM party_members WHERE party_id = ? AND player_id = ?`,
		partyID, pid).Scan(&schon)
	if schon > 0 {
		return game.Party{}, ErrSchonMitglied
	}
	// Und zweimal dieselbe Person heisst nicht zwei Partien, sondern zwei
	// gleich aussehende Zeilen in der Lobby.
	pa, err := s.Party(partyID)
	if err != nil {
		return game.Party{}, err
	}
	for _, x := range []game.PlayerID{pa.A, pa.B} {
		if x == "" {
			continue
		}
		if partner, err := s.SchonPartner(string(x), pid); err == nil && partner {
			return game.Party{}, ErrSchonPartner
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return game.Party{}, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO party_members (party_id, player_id, seite) VALUES (?,?,'B')`,
		partyID, pid); err != nil {
		return game.Party{}, err
	}
	// Code ist einmal einlösbar.
	if _, err := tx.Exec(`UPDATE parties SET code = NULL WHERE id = ?`, partyID); err != nil {
		return game.Party{}, err
	}
	if err := tx.Commit(); err != nil {
		return game.Party{}, err
	}
	// Eine offene Einladung zwischen denselben zwei Menschen waere ab jetzt
	// eine Schaltflaeche, deren Annahme nur noch ErrSchonPartner liefert.
	for _, x := range []game.PlayerID{pa.A, pa.B} {
		if x != "" {
			s.EinladungenSchliessen(string(x), pid)
		}
	}
	return s.Party(partyID)
}

func (s *Store) Party(partyID string) (game.Party, error) {
	pa := game.Party{ID: partyID}
	rows, err := s.db.Query(`SELECT player_id, seite FROM party_members WHERE party_id = ?`, partyID)
	if err != nil {
		return pa, err
	}
	defer rows.Close()
	gefunden := false
	for rows.Next() {
		var pid, seite string
		if err := rows.Scan(&pid, &seite); err != nil {
			return pa, err
		}
		gefunden = true
		if seite == "A" {
			pa.A = game.PlayerID(pid)
		} else {
			pa.B = game.PlayerID(pid)
		}
	}
	if !gefunden {
		return pa, ErrNichtGefunden
	}
	return pa, rows.Err()
}

// PartienVon liefert alle Partien eines Spielers, die juengste zuerst.
//
// Hier stand einmal PartyVon(pid) und lieferte GENAU EINE - eine Signatur, die
// behauptete, ein Spieler habe hoechstens eine Partie. Die Behauptung stand
// nirgends im Schema (party_members hat den Schluessel (party_id, player_id)),
// sondern allein in dieser Zeile und in drei Waechtern, die sie durchsetzten.
func (s *Store) PartienVon(pid string) ([]game.Party, error) {
	rows, err := s.db.Query(
		`SELECT pm.party_id FROM party_members pm
		   JOIN parties pa ON pa.id = pm.party_id
		  WHERE pm.player_id = ?
		  ORDER BY pa.erstellt_am DESC, pa.rowid DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		ids = append(ids, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]game.Party, 0, len(ids))
	for _, x := range ids {
		pa, err := s.Party(x)
		if err != nil {
			return nil, err
		}
		out = append(out, pa)
	}
	return out, nil
}

// PartyVon laedt eine Partie und prueft die Mitgliedschaft mit. Jeder Endpunkt
// mit einer Partie-ID im Pfad geht hier durch: Ohne die Pruefung waere die ID
// aus dem Pfad eine Einladung, in fremde Partien zu sehen.
func (s *Store) PartyVon(pid, partyID string) (game.Party, error) {
	pa, err := s.Party(partyID)
	if err != nil {
		return game.Party{}, err
	}
	if !pa.Mitglied(game.PlayerID(pid)) {
		return game.Party{}, ErrKeinZugriff
	}
	return pa, nil
}

// ZuletztePartie ist die juengste Partie eines Spielers - nur fuer die alten,
// partielosen Routen. Wer in mehreren Partien ist, bekommt dort einen Fehler
// statt einer geratenen Partie; das entscheidet die API, nicht der Store.
func (s *Store) ZuletztePartie(pid string) (game.Party, error) {
	xs, err := s.PartienVon(pid)
	if err != nil {
		return game.Party{}, err
	}
	if len(xs) == 0 {
		return game.Party{}, ErrNichtGefunden
	}
	return xs[0], nil
}

func (s *Store) PartyCode(partyID string) string {
	var c sql.NullString
	s.db.QueryRow(`SELECT code FROM parties WHERE id = ?`, partyID).Scan(&c)
	return c.String
}

// TestbotTags ist das Profil des Testspielers – dieselben zehn wie in
// beispiele-kim.json, damit Werkbank und Testpartie denselben Menschen meinen.
var TestbotTags = []string{
	"kaffee", "kochen", "zugfahren", "nachrichten", "handarbeit",
	"kartenspiele", "einkaufen", "kindheit", "wohnen", "pflanzen",
}

// TestpartyAnlegen setzt den Spieler in eine Party mit einem Testspieler.
//
// Wofuer: Zu zweit zu spielen heisst, zu zweit zu sein. Wer allein etwas
// ausprobieren will – eine Frage, eine Runde, den ganzen Ablauf – braucht sonst
// ein zweites Telefon und eine zweite Person. Der Testspieler ist ein ganz
// normaler Spieler mit Tags und Dossier; der einzige Unterschied steht in der
// Tabelle bots, und der Worker sieht dort nach, ob er fuer ihn ziehen muss.
// MaxTestpartien deckelt die Testpartien je Spieler. Jede erzeugt einen
// eigenen Spieler und einen Bot, der in jeder Runde antwortet - also laufende
// Modellaufrufe auf Rechnung des Betreibers. Ohne die gefallene Wache "schon in
// einer Party" waere das ein offener Hahn.
const MaxTestpartien = 3

func (s *Store) TestpartyAnlegen(pid string) (game.Party, error) {
	var n int
	s.db.QueryRow(
		`SELECT COUNT(*) FROM party_members pm
		   JOIN party_members g ON g.party_id = pm.party_id AND g.player_id != pm.player_id
		   JOIN bots b ON b.player_id = g.player_id
		  WHERE pm.player_id = ?`, pid).Scan(&n)
	if n >= MaxTestpartien {
		return game.Party{}, ErrSchonDrin
	}
	bot, _, err := s.SpielerAnlegen(s.botName("Kim (Testbot)"))
	if err != nil {
		return game.Party{}, err
	}
	pa := game.Party{ID: id(), A: game.PlayerID(pid), B: game.PlayerID(bot.ID)}

	tx, err := s.db.Begin()
	if err != nil {
		return pa, err
	}
	defer tx.Rollback()
	// Ohne Code: Eine Testpartie laedt niemand ein.
	if _, err := tx.Exec(
		`INSERT INTO parties (id, code, code_bis, erstellt_am) VALUES (?,NULL,NULL,?)`,
		pa.ID, jetzt()); err != nil {
		return pa, err
	}
	for seite, x := range map[string]string{"A": pid, "B": bot.ID} {
		if _, err := tx.Exec(
			`INSERT INTO party_members (party_id, player_id, seite) VALUES (?,?,?)`,
			pa.ID, x, seite); err != nil {
			return pa, err
		}
	}
	if _, err := tx.Exec(`INSERT INTO bots (player_id) VALUES (?)`, bot.ID); err != nil {
		return pa, err
	}
	for _, t := range TestbotTags {
		if _, err := tx.Exec(
			`INSERT INTO player_tags (player_id, tag) VALUES (?,?)`, bot.ID, t); err != nil {
			return pa, err
		}
	}
	return pa, tx.Commit()
}

// IstBot sagt, ob fuer diesen Spieler der Worker ziehen muss.
func (s *Store) IstBot(pid string) bool {
	var x string
	return s.db.QueryRow(`SELECT player_id FROM bots WHERE player_id = ?`, pid).Scan(&x) == nil
}

// BotRunden liefert je laufendem Match mit Testspieler GENAU die Runde, die
// gerade dran ist – die mit der kleinsten Nummer, die noch nicht aufgelöst ist.
//
// Ein Match legt seine Runden im Voraus an. Ohne diese Einschränkung beantwortet
// der Testspieler sie alle auf einmal: Modellaufrufe für Runden, die bei einem
// Spiel bis zehn Punkten vielleicht nie gespielt werden, und ein Dossier, das in
// der falschen Reihenfolge wächst.
func (s *Store) BotRunden() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT r.id FROM rounds r
		   JOIN matches m ON m.id = r.match_id
		   JOIN party_members pm ON pm.party_id = m.party_id
		   JOIN bots b ON b.player_id = pm.player_id
		  WHERE m.ergebnis = 'OFFEN' AND r.zustand != 'AUFGELOEST'
		    AND r.nummer = (SELECT MIN(r2.nummer) FROM rounds r2
		                     WHERE r2.match_id = m.id AND r2.zustand != 'AUFGELOEST')
		  ORDER BY r.geoeffnet_am`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// PartyVerlassen loest die Party auf – für beide Seiten.
//
// Eine Party ist zu zweit oder gar nicht; eine Party mit einem Mitglied wäre ein
// Wartezimmer ohne Tür. Wer geht, beendet sie also ganz. Ein laufendes Match
// wird dabei abgebrochen, sonst bliebe es für immer offen stehen.
//
// Was den Spielern gehört, bleibt: Konto, Tags und Dossier. Genau dafür hängt
// das Dossier am Spieler und nicht an der Party – MIMIK vergisst nichts, nur
// weil ihr in neuer Aufstellung antretet.
//
// Die vergebenen Fragen bleiben vergeben. Sie in den Vorrat zurückzulegen hieße,
// denselben Menschen dieselben Fragen noch einmal zu stellen.
func (s *Store) PartyVerlassen(pid, partyID string) error {
	// Welche Partie, steht jetzt im Aufruf. Vorher zog sich diese Funktion
	// selbst eine party_id ueber den Spieler - bei mehreren Partien haette sie
	// stillschweigend irgendeine davon aufgeloest.
	if _, err := s.PartyVon(pid, partyID); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE matches SET ergebnis = ?, beendet_am = ? WHERE party_id = ? AND ergebnis = ?`,
		game.Abgebrochen, jetzt(), partyID, game.Offen); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE rounds SET zustand = ?, aufgeloest_am = ?
		  WHERE match_id IN (SELECT id FROM matches WHERE party_id = ?) AND zustand != ?`,
		game.Aufgeloest, jetzt(), partyID, game.Aufgeloest); err != nil {
		return err
	}
	// Nur die Mitgliedschaft fällt, die Party-Zeile bleibt stehen.
	//
	// Ein DELETE auf parties scheitert am Fremdschlüssel von matches – und das
	// zu Recht: Die Chronik der gespielten Matches hängt daran. Ohne Mitglieder
	// findet PartyVon niemanden mehr, damit ist die Party für beide Seiten weg;
	// was gespielt wurde, bleibt trotzdem nachlesbar.
	if _, err := tx.Exec(
		`UPDATE parties SET zustand = 'BEENDET', code = NULL WHERE id = ?`, partyID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM party_members WHERE party_id = ?`, partyID); err != nil {
		return err
	}
	return tx.Commit()
}

// ------------------------------------------------------------------- Tags ---

func (s *Store) Tags(pid string) ([]string, error) {
	rows, err := s.db.Query(`SELECT tag FROM player_tags WHERE player_id = ? ORDER BY tag`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) TagsSetzen(pid string, tags []string) error {
	sauber := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			sauber[t] = true
		}
	}
	if len(sauber) < 10 {
		return ErrZuWenigTags
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM player_tags WHERE player_id = ?`, pid); err != nil {
		return err
	}
	for t := range sauber {
		if _, err := tx.Exec(`INSERT INTO player_tags (player_id, tag) VALUES (?,?)`, pid, t); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TagVorschlaege liefert bis zu n Begriffe aus dem Vorrat, beginnend bei ab.
// Bereits gespeicherte fallen heraus. Der zweite Rückgabewert sagt, ob danach
// noch etwas kommt.
//
// Das "ab" ist der Grund, warum es diese Fassung gibt: Vorher lieferte die
// Funktion immer die ersten n und filterte nur nach dem, was schon GESPEICHERT
// war. Im Onboarding ist aber noch nichts gespeichert – der Knopf "Weitere"
// holte also jedes Mal dieselben zwanzig.
func (s *Store) TagVorschlaege(pid string, ab, n int) ([]string, bool, error) {
	gewaehlt, err := s.Tags(pid)
	if err != nil {
		return nil, false, err
	}
	hat := map[string]bool{}
	for _, t := range gewaehlt {
		hat[t] = true
	}
	frei := make([]string, 0, len(seed.Tags()))
	for _, t := range seed.Tags() {
		if !hat[t] {
			frei = append(frei, t)
		}
	}
	// Je Spieler eine eigene, aber stabile Reihenfolge.
	//
	// Der Vorrat ist alphabetisch sortiert. Ohne Mischen bekäme jeder Mensch
	// dieselben zwanzig Begriffe von "aberglaube" bis "backen" zu sehen und
	// würde daraus wählen – dreihundert weitere lägen unerreicht dahinter.
	// Gemischt wird aus der Spieler-ID heraus, nicht aus dem Zufall: Sonst
	// verschöbe sich die Reihenfolge zwischen zwei Seiten und "Weitere" zeigte
	// Begriffe doppelt oder gar nicht.
	h := fnv.New64a()
	h.Write([]byte(pid))
	misch := mrand.New(mrand.NewPCG(h.Sum64(), 0x9E3779B97F4A7C15))
	misch.Shuffle(len(frei), func(i, j int) { frei[i], frei[j] = frei[j], frei[i] })

	if ab < 0 {
		ab = 0
	}
	if ab > len(frei) {
		ab = len(frei)
	}
	ende := ab + n
	if ende > len(frei) {
		ende = len(frei)
	}
	return frei[ab:ende], ende < len(frei), nil
}

// ---------------------------------------------------------------- Dossier ---

func (s *Store) FaktHinzu(pid, roundID, fakt string) error {
	if strings.TrimSpace(fakt) == "" {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO dossier_fakten (player_id, round_id, fakt, erstellt_am) VALUES (?,?,?,?)`,
		pid, roundID, fakt, jetzt())
	return err
}

func (s *Store) Fakten(pid string, grenze int) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT fakt FROM dossier_fakten WHERE player_id = ? ORDER BY id DESC LIMIT ?`, pid, grenze)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append([]string{f}, out...) // älteste zuerst
	}
	return out, rows.Err()
}

func (s *Store) ThemenSperren(pid, roundID string, themen []string) error {
	for _, t := range themen {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO gesperrte_themen (player_id, thema, round_id) VALUES (?,?,?)`,
			pid, t, roundID); err != nil {
			return err
		}
	}
	return nil
}

// GesperrteThemen liefert die verbrauchten Themen, die juengsten zuerst.
//
// Vorher gab das eine map[string]bool zurueck, und der Worker baute daraus per
// "for t := range" eine Liste. Das war ein stiller Fehler: Go durchlaeuft eine
// Map in zufaelliger Reihenfolge, also stand der Block bei jedem Aufruf anders
// im Prompt. Das verrauscht jeden Vergleich zweier Prompts - und es verhindert,
// dass ein Prefix-Cache je etwas davon tragen kann.
//
// grenze <= 0 heisst: alle. Die Liste waechst sonst unbegrenzt - ein bis drei
// Themen je Runde, und das Dossier haengt am Spieler, nicht am Match.
func (s *Store) GesperrteThemen(pid string, grenze int) ([]string, error) {
	q := `SELECT thema FROM gesperrte_themen WHERE player_id = ? ORDER BY rowid DESC`
	args := []any{pid}
	if grenze > 0 {
		q += ` LIMIT ?`
		args = append(args, grenze)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DossierLoeschen(pid string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM dossier_fakten WHERE player_id = ?`,
		`DELETE FROM gesperrte_themen WHERE player_id = ?`,
		// Das Profil ist Dossier: Wer vergessen will, was MIMIK ueber ihn
		// notiert hat, meint die Vermutungen zuerst.
		`DELETE FROM profil_merkmale WHERE player_id = ?`,
		`DELETE FROM profil_verlauf WHERE player_id = ?`,
	} {
		if _, err := tx.Exec(q, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ------------------------------------------------------------------- Konto ---

// SpitznameSetzen ändert nur den Anzeigenamen. Der Spieler behält seine ID und
// damit sein Dossier: MIMIK weiß weiterhin, was sie weiß.
func (s *Store) SpitznameSetzen(pid, name string) error {
	// Nur noch ueber den Anspruch. Ein direktes UPDATE auf players.spitzname
	// liesse die Anzeigeform und den gesuchten Namen auseinanderlaufen - und
	// waere ausserdem der Weg, einen vergebenen Namen doch zu bekommen.
	return s.NameBeanspruchen(pid, name)
}

// AllesLoeschen entfernt diesen Spieler restlos: Konto, Geräte, Tags, Dossier
// und die Party mit ihrer gesamten Chronik.
//
// Die Party geht mit, weil sie geteilt ist – ein Match ohne zweite Seite wäre
// eine Ruine. Was dem Gegenüber allein gehört, bleibt dagegen stehen: sein
// Konto, seine Tags, sein Dossier. Fremde Daten löscht man nicht für jemanden
// mit.
//
// Die vergebenen Fragen fallen in den Pool zurück, sonst wären sie für immer
// verbraucht.
func (s *Store) AllesLoeschen(pid string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// ALLE Partien, nicht die erstbeste. Vorher zog sich die Funktion genau
	// eine party_id und lief bei zweien in einen Fremdschluesselfehler auf
	// DELETE FROM players - die Reste der zweiten Partie hielten den Spieler
	// fest, und das Konto liess sich nicht mehr loeschen.
	rows, err := tx.Query(`SELECT party_id FROM party_members WHERE player_id = ?`, pid)
	if err != nil {
		return err
	}
	var partien []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			rows.Close()
			return err
		}
		partien = append(partien, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, partyID := range partien {
		// Die Testspieler dieser Partie mit aufsammeln: Sie gehoeren zu nichts
		// sonst, und ohne diesen Schritt bleibt nach jeder Testpartie ein
		// Spieler mit Tags stehen, den niemand mehr erreicht.
		var bots []string
		brows, err := tx.Query(
			`SELECT b.player_id FROM bots b
			   JOIN party_members pm ON pm.player_id = b.player_id
			  WHERE pm.party_id = ?`, partyID)
		if err != nil {
			return err
		}
		for brows.Next() {
			var x string
			if err := brows.Scan(&x); err != nil {
				brows.Close()
				return err
			}
			bots = append(bots, x)
		}
		brows.Close()

		for _, q := range []string{
			`DELETE FROM reviews WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM guesses WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM karten_gruende WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM karten WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM answers WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM rounds WHERE match_id IN (SELECT id FROM matches WHERE party_id = ?)`,
			`DELETE FROM matches WHERE party_id = ?`,
			`UPDATE fragen_pool SET benutzt = NULL WHERE benutzt = ?`,
			`DELETE FROM gesehen WHERE party_id = ?`,
			`DELETE FROM fragen_vergeben WHERE party_id = ?`,
			`DELETE FROM party_members WHERE party_id = ?`,
			`DELETE FROM parties WHERE id = ?`,
		} {
			if _, err := tx.Exec(q, partyID); err != nil {
				return err
			}
		}
		for _, b := range bots {
			if err := spielerreste(tx, b); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM bots WHERE player_id = ?`, b); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM players WHERE id = ?`, b); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(
		`DELETE FROM einladungen WHERE von_id = ? OR an_id = ?`, pid, pid); err != nil {
		return err
	}
	if err := spielerreste(tx, pid); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM players WHERE id = ?`, pid); err != nil {
		return err
	}
	return tx.Commit()
}

// spielerreste loescht alles, was an einem Spieler haengt, aber nicht an einer
// Partie - fuer Menschen und Testspieler dasselbe.
func spielerreste(tx *sql.Tx, pid string) error {
	for _, q := range []string{
		`DELETE FROM player_tags WHERE player_id = ?`,
		`DELETE FROM dossier_fakten WHERE player_id = ?`,
		`DELETE FROM gesperrte_themen WHERE player_id = ?`,
		`DELETE FROM profil_merkmale WHERE player_id = ?`,
		`DELETE FROM profil_verlauf WHERE player_id = ?`,
		`DELETE FROM fragen_vergeben WHERE player_id = ?`,
		`DELETE FROM gesehen WHERE player_id = ?`,
		`DELETE FROM namen WHERE player_id = ?`,
		`DELETE FROM devices WHERE player_id = ?`,
	} {
		if _, err := tx.Exec(q, pid); err != nil {
			return err
		}
	}
	return nil
}
