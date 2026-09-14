# agents.md

Regeln dieses Projekts für alle, die daran weiterarbeiten – Mensch wie Agent.
Was hier steht, ist an echtem Material gemessen oder aus einem Fehler gelernt.
Wenn du etwas davon änderst, ändere auch die Begründung.

---

## 1. Was MIMIK ist

Ein rundenbasiertes Spiel für **genau zwei** Personen je Partie, die sich gut
kennen. Beide beantworten dieselbe Frage; ein Sprachmodell schreibt drei
Fälschungen im Stil der schreibenden Person; jede Seite bekommt vier Karten über
die **andere** Person und sucht die echte. Treffer = Punkt für die Menschen,
Fehlgriff = Punkt für MIMIK. Erster auf zehn gewinnt. Kooperativ,
zeitunabhängig, kein Timer.

Eine Partie hat zwei Mitglieder – ein Spieler aber beliebig viele Partien
nebeneinander. Die Übersicht darüber heißt **Lobby**; von dort führt ein Weg in
jede Partie und aus jeder zurück. Partien entstehen über einen Einladungscode
oder über eine **Einladung** an einen gesuchten Namen.

**MIMIK ist der Name des Spiels, der Figur und des Gegners.** Die erfundene
Antwort heißt **Fälschung**, nie „Doppelgänger" – dieser Name ist vollständig
abgeschafft, auch als Untertitel.

### Was das Spiel schwer macht

Nicht der Inhalt, sondern der **Abstand**. Zwei Fehler ruinieren eine Runde:

- **Umkreisen** – eine Fälschung liegt zu nah an der echten Antwort. Dann gibt
  es zwei richtige Karten.
- **Ausreißer** – die drei Fälschungen liegen enger beieinander als an der
  echten. Dann ist die echte erkennbar, ohne die Person zu kennen.

Das misst `internal/mimik.Abstandsfenster` nach jedem Erzeugen. Verstoß heißt:
neu schreiben lassen, höchstens `MaxVersuche` (3) mal. Danach gilt der beste
Satz – eine schwache Karte ist besser als eine Runde, die hängt.

---

## 2. Sprache im Code

**Alle Bezeichner, Kommentare und Texte auf Deutsch.** Das ist keine Marotte:
Die Spielbegriffe (Runde, Party, Dossier, Fälschung, Abstandsfenster,
Themensperre, Normalform, Richtung) sind der Fachwortschatz des Projekts, und eine
halbe Übersetzung erzeugt zwei Vokabulare für dieselbe Sache.

Bezeichner ohne Umlaute (`Aufloesung`, `Faelschungen`, `gewaehlt`), Kommentare
und Nutzertexte **mit** (`Auflösung`).

Kommentare erklären **warum**, nicht was. Ein Kommentar, der den danebenstehenden
Code nacherzählt, gehört gelöscht. Ein Kommentar, der eine Entscheidung, eine
Messung oder einen Fehler festhält, gehört hin – und bleibt stehen.

---

## 3. Aufbau

```
cmd/server/          Kommando, liest Umgebung, startet HTTP und Worker
internal/game/       Reine Spielregeln. Kein HTTP, keine Datenbank, kein Modell.
internal/store/      SQLite. Ein Schema, eingebettet per go:embed.
internal/mimik/      Modellklient, Prompts, Prüfungen, Normalform
internal/sicher/     Putzt alles, was von außen kommt
internal/api/        HTTP-Schicht, Worker
internal/seed/       Fragen- und Tagvorrat als JSON, eingebettet
app/                 Android, Jetpack Compose
harness.py           Prompt-Werkbank, Python, ohne Server
pruefe-prompts.py    Hält harness.py und internal/mimik deckungsgleich
```

`internal/game` kennt weder Datenbank noch HTTP noch Modell. Das ist die
wichtigste Grenze im Projekt: Die Regeln lassen sich ohne alles andere testen.

### Zustand wird abgeleitet, nie gesetzt

`Runde.Ableiten(Party)` rechnet den Zustand aus dem aus, was vorliegt –
Antworten, Karten, Tipps. Es gibt keinen Übergang, den jemand vergessen könnte.
Genauso in der App: `AppModel.bildschirm` ist eine Funktion des Zustands, keine
Navigation.

### Richtungskonvention

Merkt man sich einmal, sonst dreht man sie ständig um:

| Feld | Bedeutung |
|---|---|
| `Antworten[p]` | die Antwort, die **p selbst** geschrieben hat |
| `Karten[p]` | die vier Karten **über p** – gezeigt werden sie dem anderen |
| `Tipps[p]` | der Tipp, den **p abgegeben** hat |

---

## 4. Die Prompts

**Ein** Prompt, in `internal/mimik/prompts.go` **und** in `harness.py`.
`pruefe-prompts.py` vergleicht sie zeichenweise. **Nach jeder Prompt-Änderung
laufen lassen:**

```bash
python3 pruefe-prompts.py
```

### Prompt B

Gibt `normalform` → `fakt` → `sperre` → `antworten` aus, **in dieser
Reihenfolge**. Das ist keine Kosmetik: Ein autoregressives Modell, das das
gesperrte Thema erst nennt, vermeidet es danach messbar besser. **Die
Feldreihenfolge im Schema nie umstellen.**

`normalform` steht vorn, weil das Modell den Text erst sauber schreiben soll und
dann alles Weitere gegen diese Fassung tut – die Fälschungen imitieren die
Normalform, nicht den rohen Text.

Drei Regeln stehen darin, weil Läufe an echtem Material sie erzwungen haben:

| Regel | Befund |
|---|---|
| Satzgerüst der echten Antwort nicht wiederverwenden | Bei „Abends Nachrichten lesen, danach bin ich nur noch wacher“ hatten alle vier Karten denselben Rahmen. Ähnlichkeit danach 0.44 → 0.04 |
| Mindestens eine Fälschung kürzer als die echte Antwort | Die echte war in 5 von 9 Runden die kürzeste (p = 0.049), Fälschungen im Schnitt 1.15× so lang |
| Das Modell sucht sich die drei Richtungen selbst | Zugewiesene Anker sahen die FRAGE nie an – „schlaf" landete bei „Wofür gibst du zu viel Geld aus?" durch reinen Zufall, und das Modell musste eine Verbindung erfinden, die es nicht gibt |

### Die Normalform gehört in denselben Aufruf

Sie war einmal ein eigener Prompt (D) und lief **vor** dem Absenden. Das ging
zweimal nicht auf:

1. Der Endpunkt antwortet nach 15 bis 190 Sekunden. So lange wartet niemand, der
   gerade getippt hat.
2. Deshalb lief sie regelbasiert – und eine Regel kann im Deutschen keine
   Substantive großschreiben. `ein kochbuch` wurde `Ein kochbuch`. Damit stand
   die echte Karte in einer Schreibweise da, die weder ein sorgfältiger Mensch
   noch ein Modell erzeugt, und war **an der Form erkennbar statt am Inhalt**.

Jetzt schreibt ein Aufruf alle vier Texte. Das Modell sieht den rohen Text (gut
für die Imitation), schreibt ihn sauber und schreibt die Fälschungen gleich mit.
Der Aufruf lief ohnehin – es kostet nichts.

Danach laufen alle vier Texte noch durch dieselbe **mechanische Glättung**
(`ErsatzNormalform`): großer Satzanfang, ein Satzzeichen am Ende, keine
Mehrfachzeichen, keine Emoji. Das ist billiger als ein neuer Aufruf und behebt
genau die Mängel, die eine Regel beheben *kann*. `Formmangel` prüft danach nach.

Was eine Regel **nicht** kann, ist die Großschreibung mitten im Satz – dafür
bräuchte es Wortarten, nicht ein Wörterbuch: `essen`/`Essen`, `laufen`/`Laufen`,
`recht`/`Recht`. Genau deshalb schreibt das Modell alle vier Texte selbst.

Die mechanische Glättung enthält eine **Positivliste von 128 Wörtern** für die
Umlautrückbildung (`hoer` → `hör`). Eine allgemeine Regel `oe → ö` zerstört
*Poesie*, *Michael*, *Abenteuer*, *aktuell*, *Duell*. Die Liste steht doppelt,
in `umlaute.go` und in `harness.py`; `pruefe-prompts.py` vergleicht sie.

Die Notfassung beim Absenden ist ebenfalls `ErsatzNormalform` – sie hält den
Wartebildschirm gefüllt, bis der Worker durch ist, und wird dann ersetzt. Die
App sagt das im Paneltitel, statt den Wechsel klammheimlich passieren zu lassen.

### Schwellen

Zwei Sätze, weil die Werkbank Zeichen-n-Gramme rechnet und der Betrieb
Embeddings vorsieht:

| Prüfung | n-Gramme (heute) | Embeddings (geplant) |
|---|---|---|
| Nähe zur echten Antwort | `0.35` | `0.72` |
| Streuung der Fälschungen | `0.15` | `0.15` |
| Themensperre berührt | `0.60` gerichtet | `0.60` |

Antwort gegen Antwort ist **symmetrisch** (Jaccard über Vierergramme). Tag gegen
Antwort ist **gerichtet**: Gefragt ist, wie viel vom Tag in der Antwort steckt.
Symmetrisch gerechnet geht ein Ein-Wort-Tag gegen einen Dreizeiler immer gegen
null, und die Prüfung der Themensperre feuert nie.

**Es gab einmal Anker.** Drei Tags des Spielers wurden gewürfelt und je einer
Fälschung fest zugewiesen. Das ging aus zwei Gründen schief. Erstens sah die
Auswahl die **Frage** nie an – `AnkerWaehlen(tags, verbraucht, echt, mischen)`
filterte gegen die Antwort und mischte dann: „schlaf" landete bei „Wofür gibst
du zu viel Geld aus?" durch reinen Zufall, und das Modell musste eine Verbindung
erfinden, die es nicht gibt. Zweitens war das Filtern selbst zu grob:
`SperrNaehe("altes rom", "Einen Film über das alte Rom schauen")` ergibt 0.57 bei
einer Schwelle von 0.60 – drei Hundertstel daneben, weil „alte" und „altes"
verschiedene n-Gramme sind. Der Anker zum Thema der echten Antwort ging durch.

Jetzt sucht sich das Modell die drei Richtungen selbst, aus den Interessen und
den Fakten, und nennt sie, bevor es schreibt. Es liest Bedeutung, nicht
Zeichenketten – und es sieht die Frage. Die Streuung misst wie bisher nach.

---

## 5. Das Modell

| Einstellung | Wert |
|---|---|
| Endpunkt | OpenAI-kompatibel, `MIMIK_BASE_URL` + `/chat/completions` |
| Vorgabe | `https://opencode.ai/zen/go/v1` |
| Modell | `deepseek-v4-flash` |
| Nötige Kopfzeile | `x-opencode-session: …`, sonst HTTP 400 `MissingSessionID` |
| Nötiger User-Agent | irgendeiner außer dem Standard von `urllib`, sonst Cloudflare 1010 |

**Dauer, gemessen an neun Runden echten Materials:** 15, 19, 22, 35, 43, 105,
155, 165, 191 Sekunden, dazu ein HTTP 500 und drei Zeitüberschreitungen. Eine
volle Runde gegen den fertigen Server brauchte fünf Minuten.

Deshalb wartet **niemand synchron**. Die Runde steht auf `MIMIK_ARBEITET`, ein
Worker versucht es alle 20 Sekunden erneut, der nächste `GET /v1/state` sieht
die Karten. Wer diese Architektur „vereinfacht“, baut eine App, die minutenlang
einen Ladebalken zeigt.

**Das Modell bekommt keine Werkzeuge.** Kein `tools`, `functions`,
`tool_choice`, `function_call`. `TestModellBekommtKeineWerkzeuge` sichert diese
Abwesenheit ab.

---

## 6. Sicherheit

Vollständig in [SICHERHEIT.md](SICHERHEIT.md). Die drei Sätze, die beim
Weiterbauen zählen:

1. **Spielertext und Modellausgabe sind dieselbe Sorte Quelle.** Beides geht
   durch `internal/sicher`, bevor es gespeichert, angezeigt oder protokolliert
   wird. Auch der Anfragepfad – er landet im Protokoll des Betreibers und kann
   prozentkodierte Steuerzeichen tragen.
2. **Nichts in der App öffnet etwas.** Kein `WebView`, kein `Linkify`, kein
   `autoLink`, kein `UriHandler`, kein `startActivity` aus Inhalten. Wer das
   ändert, hebt die zugesagte Eigenschaft auf.
3. **Der Schlüssel bleibt beim Server.** Die App spricht nie direkt mit dem
   Modellanbieter.

---

## 7. Gestaltung

Übernommen aus dem DungeonMaster-Projekt (`src/ui/theme/`), unverändert.

**Vier Paletten**, alle dunkel: tokyo-night (Vorgabe), gruvbox, nord,
catppuccin. Rollennamen bleiben gleich wie im Web-Projekt:
`bg bgAlt bgInset fg fgDim accent accent2 border success warn error selection`.

**Zwei Farben tragen Bedeutung und dürfen nicht getauscht werden:**

- `accent2` = **Mensch**
- `accent` = **MIMIK**

**Schrift:** JetBrains Mono, gebündelt, 14sp, Zeilenhöhe 1.5. Nicht die
System-Monospace von Android: Ihr fehlen U+2500–U+259F, sie weicht still auf
eine andere Schrift mit anderen Metriken aus, und MIMIKs Gesicht zerfällt. Auch
Antworttexte laufen im Monospace – sonst fiele eine Karte schon durch den Satz
aus der Reihe. Material3 zieht die Knopfbeschriftung aus `labelLarge`; ohne
diese Zeile in `Theme.kt` läuft sie in Roboto.

**Keine Rundungen, keine Schatten.** `RectangleShape`, 1px Rahmen.

**Jeder Bildschirm ist eine mittig stehende Spalte** (`Huelle` in `Screens.kt`).
`fillMaxSize` steht vor `verticalScroll`, damit `Arrangement.Center` wirklich
zentriert und der Inhalt trotzdem wachsen kann. Nichts klebt oben.

**MIMIKs Gesicht** ist eine Zeichenmatrix aus sieben Zeilen zu elf Spalten,
zusammengesetzt aus vier beweglichen Teilen (Antenne, Stiel, Auge, Mund) plus
Mittelzeile. Vier Mienen: `Bereit`, `Denkt`, `Triumph`, `Getroffen`. Triumph und
Getroffen sind **Einmalfiguren** – hinein, kurz stehen, zurück in den Leerlauf.
Nur auf Ergebnisbildschirmen hält `haltend = true` das längste Bild fest; dort
ist die Miene eine Aussage über den Ausgang, keine flüchtige Reaktion.

Auf Seitenköpfen steht das Gesicht **ohne Rahmen**, horizontal zentriert, mit
einer Zeile darunter – als frage man MIMIK, was ansteht.

**In Symbolen** (Startsymbol, Statusleiste) ist die Figur ein Vektor, keine
Zeichenmatrix: Blockzeichen hingen dort vom Rasterzufall der jeweiligen Schrift
ab. Das Startsymbol zeigt die Figur **gefüllt** mit dunklen Zügen und dem
selbstzufriedenen Grinsen; die einfarbige Ebene (themed icons) und das
Statusleistensymbol zeigen die **Strichfassung**, weil das System sie einfärbt
und eine Fläche dort zum Klotz würde. Alles bleibt innerhalb von Radius 25 um
die Mitte der 108×108-Fläche, damit ein runder Zuschnitt nichts abschneidet.

---

## 8. Versionierung

Eine Version für alles: Server, App, Werkbank. Sie steht an **zwei** Stellen:

- `internal/version.go` → `const Version`
- `app/build.gradle.kts` → `versionName` (und `versionCode` hochzählen)

Beim Anheben beide ändern.

Der Paketname ist **`app.mimik`**. Nicht `mimik`: Android verlangt für die
`applicationId` mindestens zwei durch Punkte getrennte Teile.

---

## 9. Werkzeuge

| Werkzeug | Fassung |
|---|---|
| Go | 1.26 |
| Kotlin | 2.1.20 |
| AGP | 8.13.0 |
| Gradle | 9.3.1 |
| Compose BOM | 2024.12.01 |
| minSdk / compileSdk | 26 / 36 |
| Java | 17 |

Bewusste Verzichte, bitte nicht „nachrüsten“:

- **Keine Netzwerkbibliothek.** `HttpURLConnection` reicht für zwölf Endpunkte
  mit demselben Muster. (Folge: kein `PATCH`, und `DELETE` bekommt keinen Rumpf
  – deshalb heißen die Konto-Endpunkte `POST /v1/me/name` und
  `POST /v1/me/delete`.)
- **Kein zweiter Prompt für die Normalform.** Zwei Prompts, die beide Text
  glätten, sind zwei Vokabulare für dieselbe Sache. Siehe §4.
- **Kein Push-Dienst.** Benachrichtigungen laufen über WorkManager, der selbst
  beim Server nachfragt (`Melder.kt`). Kein Firebase, kein Google-Konto, keine
  dritte Partei – der Server bleibt das Einzige, was erreichbar sein muss. Preis
  ist die Latenz: frühestens alle 15 Minuten. Für ein zeitunabhängiges Spiel der
  richtige Tausch.
- **Kein cgo.** `modernc.org/sqlite`, damit das Binary ohne Systembibliotheken
  läuft.
- **Go-Regexp kennt keine Rückverweise.** `([!?.,])\1+` gibt es nicht;
  `entdoppeln` in `erzeugen.go` ist deshalb von Hand geschrieben.
- **`--` in XML-Kommentaren bricht das Zusammenführen der Ressourcen.** In
  Kommentaren also `Rolle bg` statt `--bg`.

---

## 10. Vor dem Abgeben

```bash
go build ./... && go vet ./... && go test ./...
python3 pruefe-prompts.py
./gradlew :app:assembleDebug
python3 -c "import yaml; yaml.safe_load(open('docker-compose.yml'))"
```

Alle müssen durchlaufen.

Die letzte Zeile steht da, weil `docker-compose.yml` beim ersten echten Start an
einer Kleinigkeit zerbrach: `MIMIK_HEADERS: ${MIMIK_HEADERS:-x-opencode-session:
mimik}` – der Vorgabewert enthält `": "`, und YAML liest das als verschachtelte
Zuordnung. Hier lief kein Docker, also fiel es erst auf dem Server auf. Werte in
dieser Datei gehören in Anführungszeichen. `pruefe-prompts.py` schweigt nicht – es meldet
jede Zeile, die auseinandergelaufen ist.

---

## 11. Was schon einmal schiefging

Damit es nicht noch einmal passiert:

- **`MinAntwort = 25`** hätte „Geh raus!“ (9 Zeichen) abgewiesen. Echte Antworten
  sind kurz. Jetzt 4, mit einem weichen Hinweis ab 20.
- **`/v1/state` verlor beendete Matches**, weil nur nach `ergebnis='OFFEN'`
  gesucht wurde. Der Endstand war damit unsichtbar. `LetztesMatch` behebt das,
  ein Test hält es fest.
- **Go marshalt nil-Slices zu `null`.** Die App brach an `"tags":null` ab.
  `nichtNil()` in `runde.go`, zwei Regressionstests.
- **Compose abonniert nur, was gelesen wird.** `bildschirm` kehrte bei fehlendem
  Token zurück, ohne `zustand` je anzufassen – nach dem Anmelden rendete nichts
  neu. Der Token ist deshalb Compose-State.
- **„Weitere" im Onboarding tat nichts.** `TagVorschlaege` filterte nur nach
  GESPEICHERTEN Tags – im Onboarding ist aber noch nichts gespeichert, also kamen
  jedes Mal dieselben zwanzig. Und die App setzte bei jedem Laden
  `gewaehlteTags` auf den Stand des Servers, löschte also die Auswahl. Jetzt
  blättert der Server (`?ab=`) und die App hängt an, statt zu ersetzen.
- **Ein sortierter Vorrat zeigt sich nie ganz.** Bei 352 Begriffen und zwanzig
  pro Seite sähe alphabetisch jeder Mensch dieselben zwanzig. Die Reihenfolge
  wird deshalb je Spieler aus dessen ID gemischt – stabil, sonst verschöbe sich
  die Seitengrenze zwischen zwei Griffen.
- **Eine erfundene Person als Platzhalter.** In einem früheren Entwurf tauchte
  ein Name auf, der nach einer realen Person aussah, ohne als erfunden
  gekennzeichnet zu sein. `beispiele-kim.json` sagt in der ersten Zeile, dass
  „Kim“ erfunden ist. **Erfundenes immer als solches kennzeichnen.**
- **Aus einer Party kam man nicht heraus.** Sobald `party.partner != null` war,
  blieb der Party-Bildschirm für immer unerreichbar; der einzige Ausweg war
  „Alles löschen" – samt Dossier. Jetzt gibt es `POST /v1/parties/verlassen`.
  Die Party-Zeile bleibt dabei stehen und wird nur auf `BEENDET` gesetzt: Ein
  `DELETE` scheitert am Fremdschlüssel von `matches`, und das zu Recht – daran
  hängt die Chronik.
- **`mimik` in `.gitignore` ohne Schrägstrich** hätte `internal/mimik/`
  verschluckt. Jetzt `/mimik`.
- **Der Merker „gesehen bis" war eine nackte Rundennummer.** Rundennummern
  fangen in jedem Match wieder bei 1 an – nach dem ersten Match lag jede
  Auflösung unter dem alten Höchststand und wurde übersprungen. Er führt jetzt
  Match **und** Partie mit. Zu sehen war das erst beim zweiten Spiel, und
  deshalb lange nicht.
- **`wartet_seit` zählte ab der eigenen Antwort.** Wer zuerst schrieb und dann
  eine halbe Stunde wartete, sah den Fortschrittsbalken beim ersten Blick schon
  voll – die Zeit war ja vergangen, nur nicht mit Arbeit. Gearbeitet wird erst,
  wenn **beide** geschrieben haben (`Runde.Ableiten`), und das ist der Nullpunkt.
- **Meldungen zu einer Phase, die schon vorbei war.** Drei Ursachen, alle in
  `APP.md` festgehalten: keine Vordergrundprüfung, eine Meldung, die im Schacht
  stehen blieb, und ein Merker, der im Vordergrund nie gekürzt wurde.
- **Vier Regeln an einem Tag, die nur im Prompt standen und nicht hielten:**
  dass die Antwort die Frage beantwortet, dass sie nicht dreimal so lang ist wie
  die echte, dass sie nicht aus dem Dossier kommt, dass sie die echte Antwort
  nicht abwandelt. **Eine Regel im Prompt ist
  eine Bitte, eine Prüfung ist eine Bedingung.** Was sich mechanisch prüfen
  lässt, gehört nach `internal/mimik` – und die Schwelle gehört gemessen: Das
  Längenfenster stand zuerst zu eng und kostete einen ganzen Modellaufruf für
  drei Zeichen.
- **Das Modell baute aus dem Dossier und bog es dann zur Frage.** Auf „Welche
  Regel deiner Eltern findest du heute richtig?" kam „Beim Kochen ständig
  abschmecken" – begründet mit „du kochst regelmäßig". Das ist keine Regel der
  Eltern. Der Prompt sagte „Sie beantworten die Frage wirklich" als eine von
  sieben Regeln in einer Liste; das genügte nicht. Jetzt steht **VERLANGT** als
  eigener Abschnitt VOR den Antworten: Das Modell legt erst fest, was die Frage
  als Gegenstand will („eine Kindheitsangst"), und liefert das dann dreimal.
  Dasselbe Verfahren wie bei der Sperre – die Feldreihenfolge erzwingt die
  Denkreihenfolge. Nachgemessen an fünf harten Fragen: vorher mehrere Antworten
  daneben, danach fünfzehn von fünfzehn auf die Frage.
- **Ein Ausgabefeld kann das Verhalten kippen.** Das Feld, mit dem MIMIK
  begründet, woraus sie eine Fälschung gebaut hat, belohnte genau das Bauen aus
  Material – jede Begründung sagte „daraus habe ich…". Wer ein Feld hinzufügt,
  fügt einen Anreiz hinzu. Gegenmittel im selben Prompt: „nichts benutzt" ist
  ausdrücklich der Normalfall, und höchstens zwei der drei Antworten dürfen
  überhaupt auf Material stehen.
- **Die Denkspur kostete neunzig Prozent der Ausgabe für nichts.** Am 14.09.2026
  gemessen: 2.019 Ausgabetoken mit Denkspur, 227 ohne, bei gleicher Qualität in
  sechs Fällen. Niemand hatte je hingesehen, weil der Klient das `usage`-Objekt
  nicht las. **Was nicht gemessen wird, wird geraten** — und die Schätzung lag
  um das Zehnfache daneben.
- **`max_tokens` ohne abgeschaltete Denkspur ist eine Falle.** Mit
  `max_tokens=600` kamen 600 Token Nachdenken und **kein Inhalt** zurück: voll
  bezahlt, Runde kaputt. Der Deckel gilt deshalb nur, wenn nicht gedacht wird.
- **Der Kartenbau hatte keinen Versuchszähler.** Bis 540 Aufrufe je Stunde für
  eine einzige hängende Runde, unbegrenzt. Reviews hatten den Zähler von Anfang
  an, der Kartenbau nicht — und der ist der teurere.
- **`AllesLoeschen` griff sich genau eine Party.** Mit mehreren Partien lief es
  in einen Fremdschlüsselfehler auf `DELETE FROM players` – das Konto ließ sich
  nicht mehr löschen. Dasselbe galt für `PartyVerlassen`, das stillschweigend
  irgendeine Partie aufgelöst hätte.

---

## 12. Allein spielen

Der Einladungscode **`TEST`** setzt einen Testspieler an den Tisch: eine Party,
in der die zweite Seite vom Server gespielt wird. Zu zweit zu spielen heißt
sonst, zu zweit sein zu müssen – wer eine Frage, eine Runde oder den ganzen
Ablauf ausprobieren will, bräuchte ein zweites Telefon und eine zweite Person.

Der Testspieler ist ein **ganz normaler Spieler** mit Tags und Dossier; der
einzige Unterschied steht in der Tabelle `bots`. Er antwortet über
`PromptBotAntwort` – den einzigen zweiten Prompt im Projekt, und ausdrücklich
kein Teil des Spiels: Er steht nicht in `harness.py` und wird von
`pruefe-prompts.py` nicht verglichen.

**Geraten wird gewürfelt, nicht gerechnet.** Geprüft werden soll der Ablauf für
den Menschen davor, nicht wie gut ein Modell rät – und jeder Modellaufruf kostet
hier eine weitere Minute.

Aus demselben Grund läuft über Testpartien **kein Review**: Aus einem
gewürfelten Tipp etwas über einen Menschen zu schließen, wäre ein Scheinbeleg,
und ein Scheinbeleg im Profil ist schlimmer als kein Beleg. Höchstens drei
Testpartien je Spieler – jede erzeugt einen Spieler und laufende Modellaufrufe
auf Rechnung des Betreibers.

`BotRunden` liefert je Match **genau eine** Runde: die kleinste noch nicht
aufgelöste. Ein Match legt seine Runden im Voraus an; ohne das beantwortete der
Testspieler auf dem Gerät alle sechs auf einmal.

## 13. Was noch aussteht

- Release-Signierung der App – braucht einen Keystore mit Passwort.
- Docker-Abbild in eine Registry – braucht `write:packages` am GitHub-Token.
- Der gehärtete Behälter ist geschrieben, aber noch nie gefahren.
- Embeddings statt Zeichen-n-Grammen für die Abstandsprüfung.
- Bildstrecke der Bildschirme neu aufnehmen.
