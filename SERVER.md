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

## Dass die Fälschung die Frage beantwortet

Geprüft wird mechanisch der **Abstand** (`Abstandsfenster`), die
**Themensperre** (`Sperrbruch`) und die **Form** (`FormPruefen`) – nicht die
Relevanz. Ob eine Fälschung die Frage überhaupt beantwortet, lässt sich ohne
zweiten Modellaufruf nicht messen; dafür gibt es kein n-Gramm.

Also macht es der Prompt, mit dem Verfahren, das im Projekt ohnehin gilt: Der
Abschnitt **VERLANGT** steht VOR den Antworten. Das Modell legt zuerst fest, was
die Frage als Gegenstand will – „eine Regel der Eltern", „eine Kindheitsangst",
„ein letzter Streit und sein Anlass" –, und jede der drei Antworten muss genau
das liefern. Hinterher ließe sich das nicht mehr durchsetzen: Was dasteht,
steht da.

Das Feld `verlangt` geht nicht ins Spiel, sondern nur in die Protokollzeile des
Workers. Dort ist es die einzige Stelle, an der man sieht, dass eine Frage
falsch verstanden wurde.

Dazu die Reihenfolge im Prompt, ausdrücklich: **erst die Frage beantworten, dann
sehen, ob Material dazu passt** – nicht umgekehrt. Und höchstens zwei der drei
Antworten dürfen überhaupt auf Dossiermaterial stehen. Drei Antworten aus
derselben kleinen Quelle sind als Satz erkennbar, ohne dass man die Person
kennen muss: Die echte kommt aus dem Leben, nicht aus einer Liste.

## Der eigene Kartensatz

`/v1/parties/{id}/state` liefert unter `meine_karten` den Satz **über einen
selbst**: die eigene Antwort und die drei Fälschungen, jede mit `begruendung` —
dem Satz, woraus MIMIK sie gebaut hat. Die App zeigt das als Klonblick
(`APP.md`).

Das verrät nichts: Geraten wird über die andere Seite, und wer seine Antwort
selbst getippt hat, weiß, welche der vier Karten sie ist.

**Die Begründungen der Karten über das Gegenüber gehen dagegen nie hinaus.** Sie
zitieren dessen Dossier — Fakten, die es dem Spiel erzählt hat, unter Umständen
in einer ganz anderen Partie. `KarteAus.Begruendung` bleibt im Ratesatz leer,
und in der Auflösung steht nur `partner_tipp_grund`: die Begründung der einen
Fälschung, auf die die andere Seite hereingefallen ist — und die ist aus dem
eigenen Material gebaut. Ein Test (`TestBegruendungNurUeberMichSelbst`) hält
beide Richtungen fest.

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

## Wer sich anmelden darf

`POST /v1/devices` ist der einzige Endpunkt ohne Token. Zwei Riegel: das
Geheimnis aus `MIMIK_EINLADUNG` und eine Obergrenze je Adresse und Stunde – zehn
ohne Einladung, vierzig mit gültiger. Das Geheimnis **ist** der Riegel; die
Zählung schützt den offenen Server.

**Hinter einem Reverse Proxy braucht es `MIMIK_PROXY_HOPS`.** Ohne die Angabe
sieht der Server nur die Adresse des Proxys, und die Grenze gilt für alle
gemeinsam – im Betrieb bekam so eine eingeladene Person „zu viele anmeldungen
von dieser adresse", weil jemand anders die zehn Versuche verbraucht hatte. Mit
`MIMIK_PROXY_HOPS=1` wird die weitergereichte Adresse gezählt, und zwar **von
rechts**: Der letzte Eintrag stammt vom eigenen Proxy, alles weiter links kann
ein Klient selbst mitschicken. Ohne Proxy bleibt die Angabe leer – sonst wäre
`X-Forwarded-For` eine Einladung, die Grenze zu umgehen.

Der Zähler steht im Speicher. Ein `docker compose restart` setzt ihn zurück –
der schnellste Weg, jemanden wieder hereinzulassen.

**Umbenennen verlangt nur das Token.** Hier standen einmal dieselbe
Einladungsprüfung und derselbe Zähler: Damit war Umbenennen auf einem
geschlossenen Server unmöglich (die App schickt das Geheimnis nach dem Anmelden
nie wieder mit), und eine Umbenennung ging vom Anmeldebudget derselben Adresse
ab.

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

## Was ein Match kostet

Bis zum 14.09.2026 war die Frage nicht beantwortbar: Der Klient las das
`usage`-Objekt der Antwort gar nicht, und die Protokollzeile zählte Zeichen.
Jetzt steht jeder Aufruf in der Tabelle `aufrufe` — Zweck, Runde, Eingabe- und
Ausgabetoken, Denkspur, Cachetreffer, Zeichen, Sekunden. Einmal je Stunde
schreibt der Worker eine Summe ins Protokoll:

```
kosten (1h): 14 aufrufe, 27.412 token ein (86% aus dem cache), 3.140 aus (0 denkspur)
```

Drei Messungen am 14.09.2026 an `opencode.ai/zen/go/v1` mit
`deepseek-v4.1-flash` und dem echten Prompt haben den Preis erklärt:

**1. Die Denkspur war der größte Posten.** Mit Denkspur 2.019 Ausgabetoken,
davon rund 1.800 reines Nachdenken — ohne 227. Die Qualität war in sechs Fällen
nicht schlechter: beide 6/6 im Abstandsfenster, beide 24/24 in der Form, Nähe
0,06 gegen 0,07. Ausgabetoken sind die teure Seite, das Denken kostete also rund
neunzig Prozent davon für nichts. **Deshalb ist `MIMIK_DENKEN` standardmäßig
aus.** Ein Deckel auf die Ausgabe gilt nur ohne Denkspur — mit ihr schneidet er
das Denken ab und es kommt gar kein Inhalt zurück, voll bezahlt und die Runde
kaputt (auch das gemessen).

**2. Der Prefix-Cache hängt an der Sitzung.** 6.435 der rund 8.900 Zeichen eines
Kartenbau-Aufrufs sind derselbe Systemprompt. Mit frischer Sitzungskennung kam
davon **nichts** aus dem Cache (0 von 1.953 Token), mit fester **87 Prozent**
(1.664 von 1.920). Deshalb heißt die Marke in `MIMIK_HEADERS` jetzt
`{sitzung}` und dreht sich erst alle `MIMIK_SITZUNG_AUFRUFE` Aufrufe.

Der Grund für `{zufall}` — ein Anbieter, dessen Sitzungsverlauf mit jeder Runde
wächst, bis der Kontext überläuft — trägt dabei nicht mehr: Fünf Aufrufe in
einer Sitzung ließen `prompt_tokens` flach bei ~1.920. Die Sitzung ist hier
allein ein Cacheschlüssel. Der Block ist die Vorsicht, falls sich das ändert,
und der Server warnt, sobald die Eingabetoken über die Zeichenzahl steigen —
mehr Token als Zeichen kann nur fremder Kontext sein.

**3. Das Review lief je Runde.** Es hat die Aufrufe verdoppelt, als es am
13.09. dazukam. Ein Aufruf wertet jetzt **drei Runden desselben Spielers
zusammen** aus (`ReviewBuendelGroesse`): Von zwanzig Reviewaufrufen je Match
bleiben etwa sieben. Weniger als drei Runden laufen erst nach
`ReviewSchonfrist` (15 Minuten) — sonst blieben die letzten Runden eines
Matches liegen. Der Preis steht in `ProfilVerrechnen`: Ein Bündel zählt als
**ein** Beleg, das Profil festigt sich also langsamer. Das ist die richtige
Richtung — ein Profil, das langsam sicher wird, ist besser als eines, das sich
schnell in eine Annahme einspinnt.

### Der Deckel, der gefehlt hat

`OffeneRunden` kannte keinen Versuchszähler. Eine dauerhaft scheiternde Runde
rief **alle 20 Sekunden** erneut an, bis zu dreimal je Takt, unbegrenzt, solange
das Match offen stand: bis 540 Aufrufe je Stunde. Eine Nacht davon kostet mehr
als hundert Matches.

Die Tabelle `kartenbau` zählt jetzt mit und stellt zurück: 20 Sekunden, dann 40,
dann 80, höchstens eine Stunde. **Kein harter Stopp** — der parkte die Runde für
immer und blockierte das Match, und das ist für den Menschen davor schlimmer als
eine schwache Karte.

### Was nicht gedeckelt war

`gesperrte_themen` wächst um ein bis drei Themen je Runde, hängt am Spieler und
überlebt jedes Match — nach zehn Matches wären das rund 200 Themen in **jedem**
Aufruf, und genau dieser Teil ist nicht cachebar. `ThemenFuerPrompt` nimmt jetzt
die sechs jüngsten plus die fragennächsten, höchstens sechzehn. Nach Nähe, nicht
nach Alter: Ein Thema über Rennräder ist bei „Was isst du zum Frühstück?"
harmlos und kostet Tokens für nichts.

Dabei fiel ein stiller Fehler auf: `GesperrteThemen` gab eine `map[string]bool`
zurück, und Go durchläuft eine Map in zufälliger Reihenfolge. Der Block stand
also bei jedem Aufruf anders im Prompt — das verrauscht jeden Vergleich zweier
Prompts und verhindert, dass ein Cache je etwas davon tragen kann.

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
