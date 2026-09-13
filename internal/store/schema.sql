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
