# MIMIK · Server

Ein Go-Binary, eine SQLite-Datei, ein Container. Kein Push-Dienst, kein
Embedding-Modell, kein Fragengenerator – der MVP-Schnitt aus §12 des Dossiers.

## Starten

```bash
cp .env.beispiel .env   # MIMIK_API_KEY eintragen
docker compose up -d --build
```

Ohne Docker:

```bash
MIMIK_API_KEY=... MIMIK_BASE_URL=https://opencode.ai/zen/go/v1 MIMIK_HEADERS="x-opencode-session: mimik" go run ./cmd/server
```

| Variable | Standard | Bedeutung |
|---|---|---|
| `MIMIK_DB` | `mimik.db` | SQLite-Datei |
| `MIMIK_PORT` | `127.0.0.1:8080` | Worauf der Container im Wirtssystem hört. Für das eigene WLAN z. B. `8082` |
| `MIMIK_ADDR` | `:8080` | Adresse |
| `MIMIK_BASE_URL` | `https://api.deepseek.com` | OpenAI-kompatibler Endpunkt |
| `MIMIK_API_KEY` | – | Pflicht, sonst bleiben Runden auf `MIMIK_ARBEITET` |
| `MIMIK_MODEL` | `deepseek-v4-flash` | |
| `MIMIK_HEADERS` | – | `Name: Wert` je Zeile. `{zufall}` im Wert wird je Aufruf frisch ersetzt – siehe unten |
| `MIMIK_EINLADUNG` | – | Geheimnis fürs Anmelden. Leer = offener Server, siehe [SICHERHEIT.md](SICHERHEIT.md) |

## Endpunkte

Alles außer `POST /v1/devices` braucht `Authorization: Bearer <token>`.

| Endpunkt | Zweck |
|---|---|
| `POST /v1/devices` | Gerät anmelden, liefert Token |
| `POST /v1/parties` | Party gründen, liefert Einladungscode |
| `POST /v1/parties/join` | Beitreten (Code einmal einlösbar). Code `TEST` öffnet eine Partie gegen einen Testspieler |
| `POST /v1/parties/verlassen` | Party auflösen (Altpfad, siehe unten) |
| `GET /v1/lobby` | Alle Partien des Spielers mit dem, was dort ansteht, plus Einladungen |
| `GET /v1/spieler?q=` | Namenssuche, ab zwei Zeichen, nur eindeutige Namen |
| `GET /v1/namen/frei?name=` | Ist dieser Name zu haben? |
| `POST /v1/einladungen` | Jemanden einladen |
| `POST /v1/einladungen/{id}/annehmen` | Annehmen – hier entsteht die Partie |
| `POST /v1/einladungen/{id}/ablehnen` | Ablehnen |
| `DELETE /v1/einladungen/{id}` | Eigene Einladung zurückziehen |
| `GET /v1/parties/{id}/state` | Zustand EINER Partie |
| `POST /v1/parties/{id}/verlassen` | Diese Party auflösen, für beide Seiten |
| `POST /v1/parties/{id}/matches` | Match in dieser Partie starten |
| `POST /v1/parties/{id}/abbrechen` | Laufendes Match dieser Partie beenden |
| `POST /v1/parties/{id}/gesehen` | Auflösungen dieser Partie als gesehen merken |
| `GET /v1/tags` | 20 Vorschläge + bereits gewählte |
| `PUT /v1/tags` | Auswahl setzen, 422 unter 10 |
| `GET /v1/state` | Spielzustand (Altpfad, siehe unten) |
| `POST /v1/matches` | Match starten |
| `POST /v1/matches/abbrechen` | Laufendes Match beenden, für beide Seiten |
| `POST /v1/rounds/{id}/answer` | Antwort abgeben |
| `POST /v1/rounds/{id}/guess` | Karte wählen |
| `GET /v1/dossier` | Eigenes Dossier lesen – Fakten, verbrauchte Themen, Tags, Profil |
| `DELETE /v1/dossier` | Dossier löschen, Tags bleiben |
| `POST /v1/me/name` | Umbenennen, Dossier bleibt |
| `POST /v1/me/delete` | Konto, Tags, Dossier und Party löschen |

`POST` statt `PATCH`/`DELETE` bei `/v1/me/…`: Die Bestätigung gehört in den
Rumpf, und `HttpURLConnection` auf Android gibt einem `DELETE` keinen Rumpf und
kennt `PATCH` gar nicht.

`GET /v1/state` liefert `tags` auf oberster Ebene, nicht unter `party`: Das
Onboarding läuft, bevor es eine Party gibt, und MIMIKs Wissen über einen Spieler
überlebt jede Party.

`GET /v1/state` liefert `ist_echt` **nie**, bevor die Runde aufgelöst ist. Ein
End-to-End-Test hält das fest.

### Mehrere Partien, und was aus den alten Routen wurde

Ein Spieler kann in beliebig vielen Partien gleichzeitig sein. Das Schema konnte
das immer – `party_members` hat den Schlüssel `(party_id, player_id)`; die
Annahme „einer, eine" stand allein in `store.PartyVon(pid)` und drei Wächtern.

Die partielosen Altrouten (`/v1/state`, `/v1/parties/verlassen`, `/v1/matches`,
`/v1/matches/abbrechen`) bleiben stehen und bedienen die einzige Partie. Gibt es
**mehr als eine**, antworten sie **409 „du bist in mehreren partien – nimm den
partie-pfad"**. Kein stilles Raten: Das wäre die Sorte Fehler, die niemand
meldet, weil sie nach einem Bedienfehler aussieht, und der Zug landete in der
falschen Partie.

`GET /v1/lobby` trägt bewusst **keine Runden**. Es kommt mit zwei Abfragen aus,
egal wie viele Partien jemand hat; `RundenVonMatch` macht drei Abfragen je
Runde, das wären bei zehn Partien 240 für einen Blick auf die Übersicht. Den
Zustand je Partie leitet trotzdem `game.Runde.Ableiten` ab – aus einer Attrappe
mit Platzhaltern. Eine zweite Fassung derselben Logik in SQL wäre die Sorte
Doppelung, die auseinanderläuft, ohne dass ein Test es merkt.

### Eindeutige Namen

Spitznamen sind eindeutig, weil sie suchbar sind. Der Anspruch steht in einer
**eigenen Tabelle** `namen` und nicht als `UNIQUE` auf `players.spitzname`: Das
bräuchte einen Tabellenneubau – die Wanderung, die dieses Projekt vermeidet –
und scheiterte an den Dubletten, die in einer bestehenden Datenbank schon
stehen.

Daraus folgt die ganze Regel: **Keine Zeile in `namen` = kein Anspruch = nicht
auffindbar.** Wer einen belegten Namen trägt, spielt weiter, antwortet weiter,
löst Codes ein und lädt selbst ein – er taucht nur in keiner Suche auf, bis er
sich einen freien Namen nimmt. `"name": {"eindeutig": false}` im Zustand sagt der
App, dass sie danach fragen soll. Der Server sperrt dafür nichts: Eine ganze
Schnittstelle zu verriegeln, um eine Textänderung zu erzwingen, wäre eine
Geiselnahme.

Testspieler nehmen still die nächste Nummer („Kim (Testbot) 2"). Bei einem
Menschen wäre das falsch – ihm ungefragt den Namen zu ändern geht nicht –, bei
einem Bot ist es richtig: Er ist niemandem gegenüber sein Name.

### Fragen hängen am Menschen

`fragen_pool.benutzt` trägt eine `party_id`. Solange jeder nur eine Partie hatte,
war das dasselbe; sobald jemand zwei spielt, bekäme er dieselbe Frage ein
zweites Mal – und die zweite Antwort wäre die erste, nur schlechter. Die neue
Wahrheit steht in `fragen_vergeben`, je Spieler.

## Das Profil

Nach jeder aufgelösten Runde wertet ein **zweiter Modellaufruf** aus, welche
Karte das Gegenüber gewählt hat und was die echte Antwort über den Menschen
sagt. Daraus wächst ein Profil aus benannten Merkmalen – Geschwister,
Geschlecht, Alter, und was das Material sonst hergibt – mit Konfidenz und
Belegen.

**Die Arithmetik steht in Go, nicht im Prompt** (`internal/mimik/profil.go`).
Das ist die einzige Verteidigung gegen das Einspinnen, die nicht selbst aus dem
Modell kommt:

- Ein Beleg deckelt bei 0.45, zwei bei 0.70, ab drei bei 0.90. Nie 1.0 – das
  hier ist eine Einschätzung durch vier Sätze.
- Zustimmung steigt um höchstens 0.20, **Widerspruch halbiert**. Ein falsches
  Merkmal ist teurer als ein fehlendes, also soll es schneller fallen, als es
  steigt.
- Unter 0.15 verfällt es; höchstens zwölf Merkmale, die drei Achsen sind vom
  Überlauf ausgenommen.
- In den Fälschungsprompt geht nur, was über 0.40 liegt – und als Wort, nicht
  als Zahl. Mit „0.45" fängt kein Modell etwas an.

**Eigene Worker-Schleife** (`LernenLaufen`, Takt 60 s): Ein Review darf den
Kartenbau nicht aufhalten – auf den wartet ein Mensch, auf ein Review niemand.

**Was ausgeschlossen bleibt:** Runden ohne zwei Tipps (ein abgebrochenes Match
wird aufgelöst, ohne dass jemand geraten hat), Runden mit Bot-Beteiligung (der
Testspieler würfelt seinen Tipp – ein gewürfelter Tipp ist ein Scheinbeleg) und
Reviews über Testspieler selbst. Das nimmt der Testpartie sämtliche Kosten ab.

**Ein Review über X sieht nur Material über X.** Nie die Antwort des Partners –
sonst wäre das Profil, das X selbst lesen darf, ein Fenster in die Antworten des
anderen. Ein Test im Stubmodell prüft das mechanisch mit. Das Profil steht
ausschließlich in `GET /v1/dossier`, nie in `/v1/state`, und fällt mit
`DELETE /v1/dossier` und der Kontolöschung. Ins Betriebsprotokoll gehen nur
Merkmalsschlüssel, nie Werte.

## Sitzungen beim Anbieter

`MIMIK_HEADERS` kennt einen Platzhalter: **`{zufall}`** wird bei jedem Aufruf
durch eine frische Kennung ersetzt.

```
MIMIK_HEADERS="x-opencode-session: mimik-{zufall}"
```

Die Anführungszeichen sind kein Zierrat: Ohne sie lässt sich die `.env` nicht mit
`set -a && . ./.env` in eine Shell laden – Bash liest die Zuweisung bis zum
Leerzeichen und hält den Rest für einen Befehl. `docker compose` kommt mit beidem
zurecht und streift die Anführungszeichen ab.

Das ist kein Zierrat. Steht dort ein fester Wert, laufen alle Aufrufe unter
derselben Sitzung. Legt der Anbieter das so aus, dass er den Verlauf einer
Sitzung mitführt, wächst der mit jeder Runde – die Antworten werden langsamer
und irgendwann meldet er einen Kontextüberlauf, obwohl der eigene Prompt klein
geblieben ist. Das Spiel braucht keine Sitzung: Jeder Aufruf trägt sein
gesamtes Material selbst.

## Wie lange MIMIK braucht

Gemessen gegen OpenCode Go am 12.09.2026: **15 s bis über 5 min je Aufruf**, mit
Wiederholungen. Eine Runde braucht zwei Aufrufe – einen je Spieler –, und die
laufen **nebeneinander**: Eine Runde dauert so lange wie der langsamere von
beiden, nicht wie beide zusammen. Deshalb wartet niemand synchron – die Runde steht auf
`MIMIK_ARBEITET`, der Worker versucht es alle 20 Sekunden erneut, und der
nächste `GET /v1/state` sieht die Karten. Bei Runden, die über Tage laufen,
fällt das nicht auf.

Aus demselben Grund gibt es keine Vorschau mehr vor dem Absenden. Die saubere
Fassung der echten Antwort entsteht in **demselben** Aufruf wie die Fälschungen,
also im Worker: vier Texte, eine Hand, eine Rechtschreibung. Bis der Worker
durch ist, steht in der Datenbank eine regelbasierte Notfassung, damit der
Wartebildschirm nicht leer ist – die App sagt dazu, dass sie noch geglättet wird.

## Tests

```bash
go test ./internal/... -cover
```

`TestGanzesMatch` spielt ein vollständiges Match gegen ein nachgebautes Modell –
Geräte, Party, Tags, Runden, Auflösung, Matchende – ohne einen einzigen echten
API-Aufruf. `opsec_test.go` hält die Zusagen aus [SICHERHEIT.md](SICHERHEIT.md)
fest, `TestKonto` und `TestPartyMitLoeschen` das Löschen.
