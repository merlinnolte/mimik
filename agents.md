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
pruefe-fragen.py     Sieht den Fragenvorrat auf Doppel durch (§13)
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

### Was in den Prompt gehört und was nicht

**Die Anweisung gehört in den Prompt, die Begründung in den Code.** Zehn
Prompt-Regeln haben inzwischen eine mechanische Prüfung hinter sich
(`Formmangel`, `Laengenbruch`, `Kausalbruch`, `Floskelbruch`, `Satzbaubruch`,
`Gerippebruch`, `Sperrbruch`, `Abstandsfenster`, `Unzumutbar`, `Antwortbezug`).
Eine geprüfte Regel braucht im Prompt keine Überzeugungsarbeit mehr — die steht
in `stil.go` und `pruefung.go`, wo sie auch bei der nächsten Änderung gelesen
wird.

**Das Beispiel bleibt trotzdem.** Nachgemessen am 14.09.2026: 20 Läufe mit dem
Prompt gegen 20 mit einer um 29 Prozent gekürzten Fassung, dieselben zehn
Runden, dreizehn mechanische Kriterien — 20/20 gegen 19/20, also kein
Unterschied, den diese Stichprobe zeigen könnte. Zweimal war eine Abweichung
dennoch **zu sehen**, und beide Male an einer Regel, deren Beispiel gestrichen
war: ein Satzbaubruch, und drei Karten mit 19 bis 30 Zeichen neben einer echten
mit neun. Eine Regel nennt eine Grenze, ein Beispiel zeigt eine Verteilung.

Genommen wurden deshalb nur die Kürzungen, die keine Regel antasten: dreifach
Gesagtes, drei Sätze für eine Regel, die Begründung der eigenen
Feldreihenfolge, eine Aufzählung, die wörtlich `Formmangel` ist, ein durch das
JSON-Format erledigter Punkt — und eine **falsche** Angabe („neun bis sechzig
Zeichen sind der Normalfall"), die dem Längenfenster widersprach, seit es aus
der Normalform rechnet. Zusammen 485 Zeichen.

Und die Wortlisten der Prüfungen liest `harness.py` aus den Go-Dateien
(`wortliste_go`), statt sie zu kopieren. Eine Handkopie von `gerippewoerter` hat
dort eine Woche gelegen, ohne dass `pruefe-prompts.py` sie geprüft hätte; sie war
noch identisch — Glück, nicht Sicherheit.

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

**Die Fassung ist jetzt öffentlich sichtbar** – sie steht in den Einstellungen,
und die App vergleicht sie beim Start mit dem neuesten Release auf GitHub
(`Aktualisierung.kt`). Ein Release, dessen Tag nicht die Fassung des APK trägt,
bietet sich selbst als Update an oder verschweigt eines. Verglichen wird
**zahlenweise**: Als Text wäre `0.10` kleiner als `0.9`.

Hochgeladen werden ab 0.10 **Freigabefassungen**, keine Debugbauten. Wie sie
signiert werden und warum die Signatur nie wieder wechseln darf, steht in
[APP.md](APP.md#freigabe-signatur-und-updates).

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
python3 pruefe-fragen.py          # nur nach einer Änderung an fragen.json
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
- **Fünf Regeln, die nur im Prompt standen und nicht hielten:** dass die Antwort
  die Frage beantwortet, dass sie nicht dreimal so lang ist wie die echte, dass
  sie nicht aus dem Dossier kommt, dass sie die echte Antwort nicht abwandelt,
  dass die Begründung nicht über die echte Antwort spricht. **Eine Regel im Prompt ist
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
- **Der Prompt widersprach sich, und das Modell berichtete den Widerspruch.**
  Zur Begründung verlangte er, „die Formulierung aus `[echte_antwort_roh]`, an
  die du angeknüpft hast" beim Namen zu nennen – und verbot vier Zeilen später,
  auf der echten Antwort aufzubauen. Also stand in den Begründungen, was das
  Modell *nicht* benutzt hatte – „die beiden Gegenstände aus deiner Antwort habe
  ich nicht erwähnt, stattdessen habe ich X genommen". Inhaltlich
  richtig gearbeitet, aber die Begründung handelte von der eigenen Arbeit statt
  von der Person – und ausgerechnet von dem Material, auf das sie sich *nicht*
  bezieht. Die Zeile ist raus, ein Satz verbietet den Rückblick ausdrücklich,
  und `Antwortbezug` in `begruendung.go` prüft es nach.
  Verraten war dabei nie etwas: Die Begründung sieht nur der Mensch, um dessen
  eigene Antwort es geht. Es war eine Frage davon, wie es sich liest.
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

## 13. Der Fragenvorrat

**`internal/seed/fragen.json` ist die Wahrheit, `fragen_pool` ihre Projektion.**
Beim Start richtet `fragenAbgleichen` die Tabelle nach der Datei – einsäen,
korrigieren, zurücknehmen, alles in einer Transaktion. Vorher gab es nur
`INSERT OR IGNORE`, und das konnte keins der drei.

**Die Identität einer Frage ist ihre `kennung`, nicht ihr Text.** An der `id`
hängt `fragen_vergeben`, also das Gedächtnis, wer welche Frage schon hatte.
Hing die `id` am Text, dann bekam eine Frage nach der Korrektur eines
Tippfehlers eine neue `id` – und jeder, der sie beantwortet hat, war wieder für
sie berechtigt. Ein Tippfehler war damit unbehebbar. `fragen_kennungen` rettet
die Zuordnung über jeden Neustart, auch über eine Rücknahme hinweg: Kommt eine
Frage zurück, bekommt sie **dieselbe** `id`.

Daraus folgen drei Regeln:

| Was du willst | Was du tust |
|---|---|
| Frage hinzufügen | Eintrag anhängen, eigene Kennung |
| Tippfehler beheben | `text` ändern, **Kennung stehen lassen** |
| Frage zurückziehen | Eintrag löschen, Kennung nicht wiederverwenden |

**Eine Kennung umbenennen heißt, die Frage zu ersetzen.** Sie bekommt eine neue
`id` und wird jedem noch einmal gestellt. Wer das nicht will, ändert nur den
Text.

### Drei Arten von Doppel, und welche wehtut

| | Was schützt | Stand |
|---|---|---|
| Gleicher Text | `text UNIQUE`, `falten` im Test | gemessen: keins |
| Gleiche Frage beim Menschen | `fragen_vergeben`, zwei Stufen | trägt |
| **Zwei Texte, eine Frage** | `Fragennaehe` + Modelldurchgang | **war verletzt** |

Die dritte hebelt die zweite aus: Zwei Zeilen mit verschiedener `id` gelten als
verschiedene Fragen, und ein Mensch bekommt beide. Im Bestand vom 14.09.2026
standen dreizehn solcher Paare, eines davon wörtlich dieselbe Frage mit
„möchtest" statt „willst". Beim Auffüllen kam ein vierzehntes dazu. Alle
vierzehn sind zurückgezogen; der Vorrat steht bei **358** – 130 vom Altbestand
und 228 neue, über acht Rubriken.

### `Fragenkern` ist nicht das Komplement von `Gerippe`

Beide teilen das Deutsche in Rahmen und Gegenstand, und sie lesen die Teilung
von entgegengesetzten Seiten: `Gerippe` behält die Funktionswörter und findet
damit zwei Sätze mit demselben Bau bei ausgetauschtem Inhalt. Bei Fragen ist der
Bau das **Rauschen** – eine Frage ist kurz und besteht überwiegend aus Rahmen –,
also wird der Rahmen gestrichen.

Trotzdem **zwei Listen**, gegengemessen an allen 10.296 Paaren:

| Kern = | Paare ≥ 0.30 | ≥ 0.50 | davon Fehlalarme |
|---|---|---|---|
| Komplement von `gerippewoerter` | 119 | 6 | 3 |
| eigene Liste `fragerahmen` | 12 | 3 | 0 |

`gerippewoerter` ist absichtlich klein und enthält **kein** Interrogativum, kein
`du/dir/dich/dein` und keines der Frage-Modalverben. Genau diese fünfzehn Wörter
*sind* der Rahmen einer Frage. Wer die Listen zusammenlegt, blendet das Maß,
ohne dass ein Test rot wird.

Liste und Schwellen sind **ein** Ding. Belegt: „lang" aufzunehmen ließ „Was
würdest du tun, wenn du ein Jahr lang nicht arbeiten müsstest?" gegen „Was tust
du, wenn du eigentlich arbeiten solltest?" von 0.33 auf 0.50 steigen – aus dem
Warnband in die Sperre, für ein Wort.

### Ein Urteil hat eine Schranke, eine Suche nicht

`Fragennaehe` findet nur, was gleiche Wörter benutzt. „Was würdest du an einem
Tag machen, an dem du unsichtbar wärst?" gegen „Was würdest du tun, wenn dir
niemand zusehen könnte?" liegt bei 0.00. Dafür gibt es `pruefe-fragen.py
--modell`, und zwar in **zwei Stufen**.

Mit nur der ersten – „finde die Paare, die dasselbe fragen" – lieferte das
Modell für 136 Fragen **74 Paare, davon rund sechzig Unsinn** („Welche Erfindung
würdest du zurücknehmen?" gegen „Was würdest du deinem jüngeren Ich
verschweigen?"). Der Grund ist der Auftrag: Eine Suche hat keine Schranke, und
ein Modell, das Paare finden soll, findet Paare.

Die zweite Stufe legt jedes Paar **einzeln** vor und fragt nach einer Bedingung:
*Gäbe dieselbe Person auf beide Fragen dieselbe Antwort?* Aus 86 Verdachtsfällen
wurden **7 Urteile**, von denen fünf trugen. Über die fertigen 358 Fragen
gerechnet: 298 Verdachtsfälle, **22 Urteile**, davon 14 echt. Dasselbe
Verhältnis wie zwischen einer Regel im Prompt und einer Prüfung im Code (§11).

Und die Grenze davon ist dieselbe wie überall: Die Rubrik `haltung` steht bei 57
statt der geplanten 59, weil das Modell dort **eine Idee in dreißig Kostümen**
hatte – „was tust du, obwohl du X denkst". Jedes Paar für sich war verschieden,
der Stapel als Ganzes eine Frage. Das fängt keine Prüfung; das liest ein
Mensch.

### Die freiwillige Stimme

Nach der eigenen Antwort steht auf dem Warte- und dem Klonbildschirm eine Zeile:
**„Gute Frage?  ja · nicht so"**. Kein Bildschirm, kein Knopf, nichts
weggeklickt – wer sie überliest, verliert nichts.

Drei Entscheidungen daran sind keine Geschmacksfrage:

- **`urteilSenden` ist der einzige Aufruf der App, der nicht durch
  `imHintergrund` läuft.** Das setzt `laden` (Bildschirm blass, Knöpfe aus) und
  zeigt bei einem Fehlschlag ein rotes Band. Beides wäre falsch für eine Geste,
  die jemand aus Freundlichkeit macht. Die Wahl steht sofort lokal, der Server
  erfährt sie danach, und schlägt das fehl, sagt niemand etwas. Der Preis
  ausdrücklich: Eine Stimme kann verloren gehen, ohne dass es auffällt. Bei
  freiwilligem Feedback zu einer Frage, die derselbe Mensch nie wieder sieht,
  ist das der richtige Tausch.
- **`mein_urteil` trägt kein `omitempty`**, als einziges Feld in `RundeAus`. Hier
  ist die Null ein Wert und keine Leere; mit `omitempty` fällt
  „zurückgenommen" aus der Antwort, und ein Klient, der seine Struktur zwischen
  zwei Abgleichen wiederverwendet, behält die alte Stimme stehen. Daran ist der
  erste Durchlauf von `TestFrageUrteilen` gescheitert – nicht am Server.
- **Die Stimme ist privat.** Sie erscheint nie im Zustand des Mitspielers. Wer
  sieht, dass sein Gegenüber die Frage mies fand, liest daraus etwas über dessen
  Antwort. `TestFrageUrteilen` prüft das eigens.

**Wie sie wirkt:** `store.Fragengewicht(mag, magNicht)` gibt Lose in die
Ziehung, Mitte 6.

| Stimmen | Gewicht | |
|---|---|---|
| keine | 6 | die Mitte |
| ein Zuspruch | 9 | anderthalbmal so oft |
| eine Ablehnung | 2 | dreimal seltener |
| uneinig (1 : 1) | 5 | fast wieder Mitte |
| zwei Ablehnungen | 1 | Boden |
| drei Zusprüche | 15 | bis Deckel 18 |

Ablehnung wiegt schwerer als Zuspruch (4 gegen 3), weil der Schaden ungleich
verteilt ist: Eine schlechte Frage verbrennt eine Runde für **zwei** Menschen,
eine gute ist nur etwas besser als der Durchschnitt.

Der Boden ist 1 und nicht 0. Bei zwei Nutzern wäre eine Null das Recht eines
einzelnen Daumens, eine Frage für alle zu löschen – zu viel Macht für eine
Geste, die man auch aus Laune macht. Sechsmal seltener reicht, und der Vorrat
schrumpft nicht heimlich unter das, was `fragen_test.go` garantiert. Der Deckel
schützt die andere Seite: Ohne ihn verdrängte eine Frage, die drei Leuten
gefiel, alles andere.

**Die eigene Stimme wirkt nie auf den eigenen Vorrat.** Wer bewerten konnte, hat
die Frage gehabt, also steht sie in `fragen_vergeben` und wird ihm nicht mehr
gezogen. Was zählt, ist immer das Urteil der anderen – ohne eine Zeile Code
dafür.

**Gewürfelt wird in Go, nicht in SQL.** Eine gewichtete Ziehung mit `ORDER BY`
bräuchte einen Logarithmus, den SQLite hier nicht mitbringt; ohne ihn wäre sie
nur monoton, nicht proportional. `waehleGewichtet` bekommt das Los als Zahl
herein und ist damit ohne Datenbank nachrechenbar. Gemessen an drei Fragen mit
den Gewichten 6 : 2 : 9 und 240 Ziehungen: **88 : 26 : 126** gegen 84 : 28 : 129
erwartet.

### Was eine gute Frage für MIMIK ausmacht

Nicht Geschmack, sondern Mechanik:

- **Die Antwort muss ein Gegenstand sein, kein Bekenntnis.** „Was ist dir wichtig
  im Leben?" erzeugt vier gleich klingende Karten; „Was hebst du auf, obwohl es
  kaputt ist?" erzeugt ein Ding. Konkret = fälschbar = ratbar.
- **Kein „warum".** Das verlangt eine Begründung, die die Fälschungen
  nachmachen müssen – und `MaxKausal = 1` bestraft dann genau das. Ein `weil` in
  der Frage selbst ist in Ordnung, es verlangt nichts.
- **Keine Ja/Nein-Frage.** Vier Karten mit „ja" sind kein Spiel.
- **Keine Zahl, kein Datum, kein Eigenname.** Eine gefälschte Zahl ist trivial,
  und der Abstand daran nicht messbar.
- **Nichts über den Spielpartner.** Die Antwort wäre Wissen, das der Ratende
  schon hat.
- **Kern von mindestens zwei Inhaltswörtern.** „Wovon möchtest du weniger
  haben?" hat einen Kern aus einem Wort und kollidiert mit allem, was dieses
  Wort benutzt. `pruefe-fragen.py` listet diese Fragen eigens.

---

## 14. Was noch aussteht

- Docker-Abbild in eine Registry – braucht `write:packages` am GitHub-Token.
- Der gehärtete Behälter ist geschrieben, aber noch nie gefahren.
- Embeddings statt Zeichen-n-Grammen für die Abstandsprüfung.
- Bildstrecke der Bildschirme neu aufnehmen.
