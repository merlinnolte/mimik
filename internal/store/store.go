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
	ErrNichtGefunden = errors.New("nicht gefunden")
	ErrCodeUngueltig = errors.New("einladungscode ungültig oder abgelaufen")
	ErrPartyVoll     = errors.New("party hat bereits zwei mitglieder")
	ErrSchonDrin     = errors.New("spieler ist bereits in einer party")
	ErrZuWenigTags   = errors.New("mindestens 10 tags nötig")
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

func (s *Store) PartyAnlegen(pid string) (game.Party, string, error) {
	if _, err := s.PartyVon(pid); err == nil {
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
	if _, err := s.PartyVon(pid); err == nil {
		return game.Party{}, ErrSchonDrin
	}
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

func (s *Store) PartyVon(pid string) (game.Party, error) {
	var partyID string
	err := s.db.QueryRow(`SELECT party_id FROM party_members WHERE player_id = ?`, pid).Scan(&partyID)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Party{}, ErrNichtGefunden
	}
	if err != nil {
		return game.Party{}, err
	}
	return s.Party(partyID)
}

func (s *Store) PartyCode(partyID string) string {
	var c sql.NullString
	s.db.QueryRow(`SELECT code FROM parties WHERE id = ?`, partyID).Scan(&c)
	return c.String
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
func (s *Store) PartyVerlassen(pid string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var partyID string
	if err := tx.QueryRow(
		`SELECT party_id FROM party_members WHERE player_id = ?`, pid).Scan(&partyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNichtGefunden
		}
		return err
	}
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

func (s *Store) GesperrteThemen(pid string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT thema FROM gesperrte_themen WHERE player_id = ?`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out[t] = true
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
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNichtGefunden
	}
	_, err := s.db.Exec(`UPDATE players SET spitzname = ? WHERE id = ?`, name, pid)
	return err
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

	var partyID string
	err = tx.QueryRow(`SELECT party_id FROM party_members WHERE player_id = ?`, pid).Scan(&partyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if partyID != "" {
		for _, q := range []string{
			`DELETE FROM guesses WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM karten WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM answers WHERE round_id IN
			   (SELECT r.id FROM rounds r JOIN matches m ON m.id = r.match_id WHERE m.party_id = ?)`,
			`DELETE FROM rounds WHERE match_id IN (SELECT id FROM matches WHERE party_id = ?)`,
			`DELETE FROM matches WHERE party_id = ?`,
			`UPDATE fragen_pool SET benutzt = NULL WHERE benutzt = ?`,
			`DELETE FROM party_members WHERE party_id = ?`,
			`DELETE FROM parties WHERE id = ?`,
		} {
			if _, err := tx.Exec(q, partyID); err != nil {
				return err
			}
		}
	}

	for _, q := range []string{
		`DELETE FROM player_tags WHERE player_id = ?`,
		`DELETE FROM dossier_fakten WHERE player_id = ?`,
		`DELETE FROM gesperrte_themen WHERE player_id = ?`,
		`DELETE FROM devices WHERE player_id = ?`,
		`DELETE FROM players WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}
