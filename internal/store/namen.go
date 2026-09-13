package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// MinSuche ist die kuerzeste Anfrage, die die Suche beantwortet.
//
// Eine Suche nach einem Zeichen ist keine Suche, sondern ein Verzeichnis: Sie
// liefert praktisch alle Namen des Servers. Zwei Zeichen sind immer noch wenig,
// aber sie verlangen wenigstens, dass man den Anfang eines Namens kennt.
const MinSuche = 2

// MaxTreffer deckelt, was eine Anfrage herausgibt.
const MaxTreffer = 20

// NameNormal faltet einen Namen fuer den Vergleich.
//
// Klein und getrimmt, Mehrfachleerzeichen zusammengezogen - "Robin", "robin"
// und "  Robin " sind derselbe Anspruch. Umlaute werden ausdruecklich NICHT auf
// ue/oe/ae gezogen: "Jürgen" und "Juergen" sind zwei Menschen, nicht zwei
// Schreibweisen desselben. Wer beide zusammenwirft, nimmt einem von ihnen
// seinen Namen weg.
func NameNormal(name string) string {
	f := strings.Fields(strings.ToLower(strings.TrimSpace(name)))
	return strings.Join(f, " ")
}

// NameFrei sagt, ob ein Name zu haben ist. "ausser" ist der eigene Spieler –
// der eigene Name ist fuer einen selbst immer frei.
func (s *Store) NameFrei(name, ausser string) (bool, error) {
	n := NameNormal(name)
	if n == "" {
		return false, nil
	}
	var wer string
	err := s.db.QueryRow(`SELECT player_id FROM namen WHERE normal = ?`, n).Scan(&wer)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return wer == ausser, nil
}

// NameBeanspruchen traegt den Namen ein und setzt den Spitznamen mit.
//
// Beides in einer Transaktion, weil players.spitzname und namen.spitzname sonst
// auseinanderlaufen - und der Spitzname ist das, was ueberall angezeigt wird,
// waehrend namen.normal das ist, wonach gesucht wird.
func (s *Store) NameBeanspruchen(pid, name string) error {
	name = strings.TrimSpace(name)
	n := NameNormal(name)
	if n == "" {
		return ErrNichtGefunden
	}
	frei, err := s.NameFrei(name, pid)
	if err != nil {
		return err
	}
	if !frei {
		return ErrNameVergeben
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO namen (player_id, normal, spitzname, erstellt_am) VALUES (?,?,?,?)
		 ON CONFLICT(player_id) DO UPDATE SET normal = excluded.normal,
		                                      spitzname = excluded.spitzname`,
		pid, n, name, jetzt()); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE players SET spitzname = ? WHERE id = ?`, name, pid); err != nil {
		return err
	}
	return tx.Commit()
}

// NameGeklaert sagt, ob dieser Spieler einen Namen beansprucht hat.
//
// Wer keinen Anspruch hat, spielt trotzdem weiter - er ist nur nicht
// auffindbar. Das ist die ganze Regel fuer Bestandsdubletten: Sie werden nicht
// umbenannt, nicht ausgesperrt und nicht angehalten; sie tauchen in keiner
// Suche auf, bis sie sich einen freien Namen nehmen.
func (s *Store) NameGeklaert(pid string) bool {
	var x string
	err := s.db.QueryRow(`SELECT normal FROM namen WHERE player_id = ?`, pid).Scan(&x)
	return err == nil
}

// SpielerSuchen findet Menschen nach Namen. Testspieler nie, sich selbst nie,
// Namenlose nie.
func (s *Store) SpielerSuchen(q, ausser string) ([]Player, error) {
	n := NameNormal(q)
	if len([]rune(n)) < MinSuche {
		return nil, nil
	}
	// % und _ sind in LIKE Platzhalter. Ohne dieses Entschaerfen waere die
	// Anfrage "%" eine vollstaendige Mitgliederliste.
	e := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(n)
	rows, err := s.db.Query(
		`SELECT p.id, p.spitzname FROM namen n
		   JOIN players p ON p.id = n.player_id
		  WHERE n.normal LIKE ? ESCAPE '\'
		    AND n.player_id != ?
		    AND NOT EXISTS (SELECT 1 FROM bots b WHERE b.player_id = n.player_id)
		  ORDER BY (n.normal LIKE ? ESCAPE '\') DESC, n.normal
		  LIMIT ?`,
		"%"+e+"%", ausser, e+"%", MaxTreffer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.Spitzname); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// botName sucht einen freien Namen fuer einen Testspieler.
//
// Der Testspieler verhandelt nicht ueber seinen Namen, er nimmt die naechste
// Nummer: "Kim (Testbot)", "Kim (Testbot) 2", und so weiter. Bei einem Menschen
// waere das falsch - ihm ohne Rueckfrage den Namen zu aendern geht nicht -, bei
// einem Bot ist es richtig, denn er ist niemandem gegenueber sein Name.
func (s *Store) botName(stamm string) string {
	for i := 1; i < 1000; i++ {
		kandidat := stamm
		if i > 1 {
			kandidat = fmt.Sprintf("%s %d", stamm, i)
		}
		if frei, err := s.NameFrei(kandidat, ""); err == nil && frei {
			return kandidat
		}
	}
	return stamm + " " + id()[:6]
}
