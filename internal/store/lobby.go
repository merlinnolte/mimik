package store

import (
	"database/sql"

	"mimik/internal/game"
)

// LobbyPartie ist eine Zeile der Uebersicht.
type LobbyPartie struct {
	PartyID    string        `json:"party_id"`
	Partner    *Player       `json:"partner"`
	Testpartie bool          `json:"testpartie"`
	Code       string        `json:"code,omitempty"`
	MatchID    string        `json:"match_id,omitempty"`
	Stand      game.Stand    `json:"stand"`
	Ziel       int           `json:"ziel"`
	Ergebnis   game.Ergebnis `json:"ergebnis,omitempty"`
	Dran       string        `json:"dran"` // schreiben|raten|warten|aufgeloest|kein_match|kein_partner
	Runde      string        `json:"runde,omitempty"`
	Ungesehen  int           `json:"ungesehen"`
}

// Lobby laedt alle Partien eines Spielers mit dem, was dort ansteht.
//
// Bewusst OHNE RundenVonMatch: das macht drei Abfragen je Runde, bei zehn
// Partien mit acht Runden also 240 fuer einen Blick auf die Uebersicht. Hier
// sind es zwei - eine Zeile je Partie, und eine Zeile je offener Runde und
// Seite mit EXISTS-Spalten statt der Inhalte.
//
// Den Zustand leitet trotzdem game.Runde.Ableiten ab, aus einer Attrappe mit
// Platzhaltern. Eine zweite Fassung derselben Logik in SQL waere die Sorte
// Doppelung, die auseinanderlaeuft, ohne dass ein Test es merkt.
func (s *Store) Lobby(pid string) ([]LobbyPartie, error) {
	rows, err := s.db.Query(
		`SELECT pm.party_id, pa.code,
		        g.player_id, p.spitzname,
		        EXISTS(SELECT 1 FROM bots b WHERE b.player_id = g.player_id),
		        m.id, m.punkte_mensch, m.punkte_mimik, m.ziel, m.ergebnis,
		        (SELECT COUNT(*) FROM rounds r2
		           JOIN matches m2 ON m2.id = r2.match_id
		          WHERE m2.party_id = pm.party_id AND r2.aufgeloest_am IS NOT NULL
		            AND r2.aufgeloest_am > COALESCE(
		                  (SELECT bis FROM gesehen
		                    WHERE player_id = pm.player_id AND party_id = pm.party_id), ''))
		   FROM party_members pm
		   JOIN parties pa ON pa.id = pm.party_id
		   LEFT JOIN party_members g ON g.party_id = pm.party_id AND g.player_id != pm.player_id
		   LEFT JOIN players p ON p.id = g.player_id
		   LEFT JOIN matches m ON m.id = (SELECT id FROM matches m3
		                                   WHERE m3.party_id = pm.party_id
		                                   ORDER BY erstellt_am DESC, rowid DESC LIMIT 1)
		  WHERE pm.player_id = ?
		  ORDER BY pa.erstellt_am DESC, pa.rowid DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LobbyPartie{}
	stelle := map[string]int{}
	for rows.Next() {
		var (
			l                   LobbyPartie
			code, gid, gname    sql.NullString
			mid, ergebnis       sql.NullString
			mensch, mimik, ziel sql.NullInt64
			bot                 sql.NullBool
		)
		if err := rows.Scan(&l.PartyID, &code, &gid, &gname, &bot,
			&mid, &mensch, &mimik, &ziel, &ergebnis, &l.Ungesehen); err != nil {
			return nil, err
		}
		l.Code = code.String
		if gid.Valid {
			l.Partner = &Player{ID: gid.String, Spitzname: gname.String}
			l.Testpartie = bot.Bool
		}
		l.MatchID = mid.String
		l.Stand = game.Stand{Mensch: int(mensch.Int64), Mimik: int(mimik.Int64)}
		l.Ziel = int(ziel.Int64)
		if l.Ziel == 0 {
			l.Ziel = game.Ziel
		}
		l.Ergebnis = game.Ergebnis(ergebnis.String)
		switch {
		case l.Partner == nil:
			l.Dran = "kein_partner"
		case !mid.Valid || l.Ergebnis != game.Offen:
			l.Dran = "kein_match"
		default:
			l.Dran = "warten"
		}
		stelle[l.PartyID] = len(out)
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	if err := s.lobbyRunden(pid, out, stelle); err != nil {
		return nil, err
	}
	for i := range out {
		// Eine ungesehene Auflösung schlaegt das Warten: Sie ist der Moment,
		// auf den gewartet wurde, und sie geht verloren, sobald die naechste
		// Runde nachrueckt.
		if out[i].Dran == "warten" && out[i].Ungesehen > 0 {
			out[i].Dran = "aufgeloest"
		}
	}
	return out, nil
}

// lobbyRunden traegt Dran und Runde nach - eine Abfrage ueber alle Partien.
func (s *Store) lobbyRunden(pid string, out []LobbyPartie, stelle map[string]int) error {
	rows, err := s.db.Query(
		`SELECT r.id, m.party_id, r.nummer, mem.player_id,
		        EXISTS(SELECT 1 FROM answers a WHERE a.round_id = r.id AND a.player_id = mem.player_id),
		        (SELECT COUNT(*) FROM karten k WHERE k.round_id = r.id AND k.ueber = mem.player_id),
		        EXISTS(SELECT 1 FROM guesses gu WHERE gu.round_id = r.id AND gu.rater_id = mem.player_id)
		   FROM rounds r
		   JOIN matches m ON m.id = r.match_id
		   JOIN party_members mem ON mem.party_id = m.party_id
		  WHERE m.ergebnis = 'OFFEN' AND r.zustand != 'AUFGELOEST'
		    AND m.party_id IN (SELECT party_id FROM party_members WHERE player_id = ?)
		  ORDER BY m.party_id, r.nummer`, pid)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Je Partie nur die kleinste offene Rundennummer - das ist die, die dran
	// ist. Die spaeteren sind vorgelegt und noch nicht gespielt.
	type stand struct {
		rid    string
		nummer int
		rd     game.Runde
		pa     game.Party
	}
	offen := map[string]*stand{}
	for rows.Next() {
		var (
			rid, party, mem string
			nummer, karten  int
			hatAntwort      bool
			hatTipp         bool
		)
		if err := rows.Scan(&rid, &party, &nummer, &mem, &hatAntwort, &karten, &hatTipp); err != nil {
			return err
		}
		st, da := offen[party]
		if !da || nummer < st.nummer {
			st = &stand{rid: rid, nummer: nummer, rd: game.Runde{
				ID:        rid,
				Antworten: map[game.PlayerID]game.Antwort{},
				Karten:    map[game.PlayerID][]game.Karte{},
				Tipps:     map[game.PlayerID]game.Tipp{},
			}}
			offen[party] = st
		}
		if st.nummer != nummer {
			continue
		}
		x := game.PlayerID(mem)
		if st.pa.A == "" {
			st.pa.A = x
		} else if st.pa.B == "" && st.pa.A != x {
			st.pa.B = x
		}
		if hatAntwort {
			st.rd.Antworten[x] = game.Antwort{Normalform: "·"}
		}
		if karten > 0 {
			st.rd.Karten[x] = make([]game.Karte, karten)
		}
		if hatTipp {
			st.rd.Tipps[x] = game.Tipp{}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	ich := game.PlayerID(pid)
	for party, st := range offen {
		i, da := stelle[party]
		if !da || st.pa.B == "" {
			continue
		}
		_, hatAntwort := st.rd.Antworten[ich]
		_, hatTipp := st.rd.Tipps[ich]
		out[i].Runde = st.rid
		switch st.rd.Ableiten(st.pa) {
		case game.Schreiben, game.SchreibenWartet:
			if hatAntwort {
				out[i].Dran = "warten"
			} else {
				out[i].Dran = "schreiben"
			}
		case game.MimikArbeitet:
			out[i].Dran = "warten"
		case game.Raten, game.RatenWartet:
			if hatTipp {
				out[i].Dran = "warten"
			} else {
				out[i].Dran = "raten"
			}
		default:
			out[i].Dran = "warten"
		}
	}
	return nil
}

// PartieGesehen merkt sich, dass dieser Spieler die Aufloesungen dieser Partie
// bis jetzt gesehen hat.
func (s *Store) PartieGesehen(pid, partyID string) error {
	_, err := s.db.Exec(
		`INSERT INTO gesehen (player_id, party_id, bis) VALUES (?,?,?)
		 ON CONFLICT(player_id, party_id) DO UPDATE SET bis = excluded.bis`,
		pid, partyID, jetzt())
	return err
}
