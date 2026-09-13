PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS players (
  id          TEXT PRIMARY KEY,
  spitzname   TEXT NOT NULL,
  erstellt_am TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS devices (
  id          TEXT PRIMARY KEY,
  player_id   TEXT NOT NULL REFERENCES players(id),
  token_hash  TEXT NOT NULL UNIQUE,
  erstellt_am TEXT NOT NULL
);

-- Testspieler. Eine eigene Tabelle statt einer Spalte in players: CREATE TABLE
-- IF NOT EXISTS legt sie auch in einer bestehenden Datenbank an, ein ALTER
-- TABLE braeuchte eine Wanderung.
CREATE TABLE IF NOT EXISTS bots (
  player_id TEXT PRIMARY KEY REFERENCES players(id)
);

CREATE TABLE IF NOT EXISTS parties (
  id          TEXT PRIMARY KEY,
  code        TEXT UNIQUE,              -- NULL sobald eingelöst
  code_bis    TEXT,
  zustand     TEXT NOT NULL DEFAULT 'AKTIV',
  erstellt_am TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS party_members (
  party_id  TEXT NOT NULL REFERENCES parties(id),
  player_id TEXT NOT NULL REFERENCES players(id),
  seite     TEXT NOT NULL CHECK (seite IN ('A','B')),
  PRIMARY KEY (party_id, player_id),
  UNIQUE (party_id, seite)
);

CREATE TABLE IF NOT EXISTS player_tags (
  player_id TEXT NOT NULL REFERENCES players(id),
  tag       TEXT NOT NULL,
  PRIMARY KEY (player_id, tag)
);

CREATE TABLE IF NOT EXISTS dossier_fakten (
  id          INTEGER PRIMARY KEY,
  player_id   TEXT NOT NULL REFERENCES players(id),
  round_id    TEXT NOT NULL,
  fakt        TEXT NOT NULL,
  erstellt_am TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS gesperrte_themen (
  player_id TEXT NOT NULL REFERENCES players(id),
  thema     TEXT NOT NULL,
  round_id  TEXT NOT NULL,
  PRIMARY KEY (player_id, thema)
);

CREATE TABLE IF NOT EXISTS matches (
  id            TEXT PRIMARY KEY,
  party_id      TEXT NOT NULL REFERENCES parties(id),
  punkte_mensch INTEGER NOT NULL DEFAULT 0,
  punkte_mimik  INTEGER NOT NULL DEFAULT 0,
  ziel          INTEGER NOT NULL DEFAULT 10,
  karten        INTEGER NOT NULL DEFAULT 4,
  ergebnis      TEXT NOT NULL DEFAULT 'OFFEN',
  erstellt_am   TEXT NOT NULL,
  beendet_am    TEXT
);

CREATE TABLE IF NOT EXISTS rounds (
  id           TEXT PRIMARY KEY,
  match_id     TEXT NOT NULL REFERENCES matches(id),
  nummer       INTEGER NOT NULL,
  frage        TEXT NOT NULL,
  rubrik       TEXT NOT NULL,
  zustand      TEXT NOT NULL DEFAULT 'SCHREIBEN',
  fehler       TEXT,                    -- letzter Erzeugungsfehler, für die Anzeige
  geoeffnet_am TEXT NOT NULL,
  aufgeloest_am TEXT,
  UNIQUE (match_id, nummer)
);

CREATE TABLE IF NOT EXISTS answers (
  round_id    TEXT NOT NULL REFERENCES rounds(id),
  player_id   TEXT NOT NULL REFERENCES players(id),
  original    TEXT NOT NULL,
  normalform  TEXT NOT NULL,
  -- Altlast: Der Schalter "nicht ins Dossier" ist entfallen. Die Spalte bleibt,
  -- damit bestehende Datenbanken ohne Migration weiterlaufen; DEFAULT 1 trägt sie.
  ins_dossier INTEGER NOT NULL DEFAULT 1,
  erstellt_am TEXT NOT NULL,
  PRIMARY KEY (round_id, player_id)
);

-- Die vier Karten über einen Spieler, Reihenfolge beim Erzeugen eingefroren.
CREATE TABLE IF NOT EXISTS karten (
  round_id  TEXT NOT NULL REFERENCES rounds(id),
  ueber     TEXT NOT NULL REFERENCES players(id),
  pos       INTEGER NOT NULL,
  text      TEXT NOT NULL,
  ist_echt  INTEGER NOT NULL,
  anker_tag TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (round_id, ueber, pos)
);

CREATE TABLE IF NOT EXISTS guesses (
  round_id    TEXT NOT NULL REFERENCES rounds(id),
  rater_id    TEXT NOT NULL REFERENCES players(id),
  gewaehlt    INTEGER NOT NULL,
  richtig     INTEGER NOT NULL,
  erstellt_am TEXT NOT NULL,
  PRIMARY KEY (round_id, rater_id)
);

CREATE TABLE IF NOT EXISTS fragen_pool (
  id      INTEGER PRIMARY KEY,
  text    TEXT NOT NULL UNIQUE,
  rubrik  TEXT NOT NULL,
  benutzt TEXT                          -- party_id, sobald vergeben
);

CREATE INDEX IF NOT EXISTS idx_rounds_match ON rounds(match_id);
CREATE INDEX IF NOT EXISTS idx_fakten_player ON dossier_fakten(player_id);

-- ---------------------------------------------------------------- Lobby ---

-- Der beanspruchte Name. Eine eigene Tabelle statt eines UNIQUE auf
-- players.spitzname: Das ginge nur ueber einen Tabellenneubau (die Wanderung,
-- die oben ausgeschlossen ist) UND scheiterte an den Dubletten, die in einer
-- bestehenden Datenbank schon stehen. Hier gilt: keine Zeile = kein Anspruch
-- = nicht auffindbar. Wer eine Dublette traegt, spielt weiter, taucht aber in
-- keiner Suche auf, bis er sich einen freien Namen nimmt.
CREATE TABLE IF NOT EXISTS namen (
  player_id   TEXT PRIMARY KEY REFERENCES players(id),
  normal      TEXT NOT NULL UNIQUE,      -- gefaltet, siehe NameNormal
  spitzname   TEXT NOT NULL,             -- Anzeigeform beim Anspruch
  erstellt_am TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS einladungen (
  id             TEXT PRIMARY KEY,
  von_id         TEXT NOT NULL REFERENCES players(id),
  an_id          TEXT NOT NULL REFERENCES players(id),
  zustand        TEXT NOT NULL DEFAULT 'OFFEN',  -- OFFEN|ANGENOMMEN|ABGELEHNT|ZURUECKGEZOGEN
  party_id       TEXT,                           -- gesetzt bei ANGENOMMEN
  erstellt_am    TEXT NOT NULL,
  entschieden_am TEXT
);

-- Eine offene Einladung je Richtung und Paar. Der Teilindex ersetzt jede
-- Pruefung im Go-Code: Ein Doppelklick prallt an der Datenbank ab, nicht an
-- einem SELECT, zwischen dem und dem INSERT eine Luecke liegt.
CREATE UNIQUE INDEX IF NOT EXISTS idx_einladung_offen
  ON einladungen(von_id, an_id) WHERE zustand = 'OFFEN';
CREATE INDEX IF NOT EXISTS idx_einladung_an ON einladungen(an_id, zustand);

-- Bis wann ein Spieler die Aufloesungen einer Partie gesehen hat. Ein
-- Zeitpunkt statt einer Zeile je Runde: Die Lobby fragt "wie viele Runden sind
-- seither aufgeloest worden" und bekommt eine Zahl aus einem COUNT.
CREATE TABLE IF NOT EXISTS gesehen (
  player_id TEXT NOT NULL REFERENCES players(id),
  party_id  TEXT NOT NULL REFERENCES parties(id),
  bis       TEXT NOT NULL,
  PRIMARY KEY (player_id, party_id)
);

-- Welche Frage welcher MENSCH schon hatte. fragen_pool.benutzt traegt eine
-- party_id; sobald jemand in zwei Partien spielt, bekaeme er dieselbe Frage
-- zweimal. benutzt bleibt bedient, ist aber nicht mehr das Kriterium.
CREATE TABLE IF NOT EXISTS fragen_vergeben (
  player_id   TEXT NOT NULL REFERENCES players(id),
  frage_id    INTEGER NOT NULL,
  party_id    TEXT NOT NULL,
  vergeben_am TEXT NOT NULL,
  PRIMARY KEY (player_id, frage_id)
);

-- --------------------------------------------------------------- Profil ---

-- Benannte Merkmale statt freier Fakten: eine Zeile je Merkmal und Spieler,
-- nicht je Beobachtung. Ein Merkmal hat genau einen aktuellen Stand.
CREATE TABLE IF NOT EXISTS profil_merkmale (
  player_id    TEXT NOT NULL REFERENCES players(id),
  merkmal      TEXT NOT NULL,              -- normalisiert: geschwister, geschlecht, alter, ...
  wert         TEXT NOT NULL,
  konfidenz    REAL NOT NULL,              -- 0.0 .. 0.9, nie 1.0
  belege       INTEGER NOT NULL DEFAULT 1, -- Runden, die dafuer sprachen
  wider        INTEGER NOT NULL DEFAULT 0, -- Runden, die dagegen sprachen
  beleg        TEXT NOT NULL DEFAULT '',   -- Beobachtung der letzten Aenderung
  round_id     TEXT NOT NULL DEFAULT '',
  erstellt_am  TEXT NOT NULL,
  geaendert_am TEXT NOT NULL,
  PRIMARY KEY (player_id, merkmal)
);

-- Getrennte Historie, weil "revidiert" ohne das Vorher nicht nachvollziehbar
-- ist - weder fuer den Betreiber noch fuer den Menschen, ueber den es geht.
CREATE TABLE IF NOT EXISTS profil_verlauf (
  id           INTEGER PRIMARY KEY,
  player_id    TEXT NOT NULL REFERENCES players(id),
  merkmal      TEXT NOT NULL,
  round_id     TEXT NOT NULL,
  urteil       TEXT NOT NULL,   -- NEU|BESTAETIGT|REVIDIERT|VERWORFEN|VERFALLEN
  wert_vorher  TEXT NOT NULL DEFAULT '',
  wert_nachher TEXT NOT NULL DEFAULT '',
  konfidenz    REAL NOT NULL,
  beleg        TEXT NOT NULL DEFAULT '',
  erstellt_am  TEXT NOT NULL
);

-- Merker: welche Runde ueber welchen Spieler ausgewertet ist. Ohne ihn wuerde
-- jeder Takt dieselbe Runde erneut reviewen. Der Versuchszaehler steht mit
-- drin, sonst laeuft ein dauerhaft scheiterndes Review ewig weiter.
CREATE TABLE IF NOT EXISTS reviews (
  round_id    TEXT NOT NULL REFERENCES rounds(id),
  ueber       TEXT NOT NULL REFERENCES players(id),
  versuche    INTEGER NOT NULL DEFAULT 0,
  fertig      INTEGER NOT NULL DEFAULT 0,
  fehler      TEXT NOT NULL DEFAULT '',
  erstellt_am TEXT NOT NULL,
  PRIMARY KEY (round_id, ueber)
);

CREATE INDEX IF NOT EXISTS idx_members_player ON party_members(player_id);
CREATE INDEX IF NOT EXISTS idx_matches_party  ON matches(party_id);
CREATE INDEX IF NOT EXISTS idx_answers_player ON answers(player_id);
CREATE INDEX IF NOT EXISTS idx_profil_player  ON profil_merkmale(player_id);
CREATE INDEX IF NOT EXISTS idx_verlauf_player ON profil_verlauf(player_id, merkmal);
