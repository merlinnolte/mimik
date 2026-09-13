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
| `POST /v1/parties/verlassen` | Party auflösen, für beide Seiten |
| `GET /v1/tags` | 20 Vorschläge + bereits gewählte |
| `PUT /v1/tags` | Auswahl setzen, 422 unter 10 |
| `GET /v1/state` | Kompletter Spielzustand |
| `POST /v1/matches` | Match starten |
| `POST /v1/matches/abbrechen` | Laufendes Match beenden, für beide Seiten |
| `POST /v1/rounds/{id}/answer` | Antwort abgeben |
| `POST /v1/rounds/{id}/guess` | Karte wählen |
| `GET /v1/dossier` | Eigenes Dossier lesen |
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

## Sitzungen beim Anbieter

`MIMIK_HEADERS` kennt einen Platzhalter: **`{zufall}`** wird bei jedem Aufruf
durch eine frische Kennung ersetzt.

```
MIMIK_HEADERS=x-opencode-session: mimik-{zufall}
```

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
