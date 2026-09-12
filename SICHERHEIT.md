# MIMIK · Sicherheit

Was dieses Projekt annimmt, was es weiterreicht und was es ausdrücklich nicht
tut. Die Kurzfassung: **Alles, was von außen kommt, ist Text und bleibt Text.**
Spielerantworten und Modellausgaben sind dabei dieselbe Sorte Quelle – die
Modellausgabe entsteht ja aus der Spielereingabe.

## Das Angriffsbild

Vier Wege führen hier hinein, und drei davon sind derselbe Weg:

| Weg | Wer schreibt | Wo landet es |
|---|---|---|
| Antwortfeld | Spieler | Prompt, Datenbank, Karte auf dem anderen Gerät |
| Spitzname, Tags, Einladungscode | Spieler | Prompt, Anzeige, Benachrichtigung |
| Anfragepfad | jeder im Netz | Protokoll des Betreibers |
| Modellantwort | Modellanbieter, mittelbar der Spieler | Karte, Dossier, Protokoll |

Alle vier gehen durch `internal/sicher`. Was dort herauskommt, ist einzeiliger,
druckbarer Text mit einer Längengrenze – mehr braucht das Spiel nicht.

## Was `internal/sicher` entfernt

| Entfernt | Warum |
|---|---|
| ANSI-Folgen (`ESC [ …`) | `ESC[2J` löscht dem Betreiber den Bildschirm; Protokollzeilen lassen sich fälschen |
| OSC-Folgen (`ESC ] …`) | Darin steckt der anklickbare Terminal-Hyperlink |
| C0- und C1-Steuerzeichen | Rücktaste, Wagenrücklauf, Nullbyte |
| `unicode.Cf` | Leserichtung (U+202E) und unsichtbare Breitenlose. U+202E kehrt Text auf dem Schirm um: Angezeigt stünde etwas anderes als in der Datenbank – in einem Spiel, in dem man vier Texte gegeneinander liest, kein Schönheitsfehler |
| Private Use, Surrogate, ungültiges UTF-8 | kein darstellbarer Sinn |
| Zeilenumbrüche | werden zu Leerzeichen: im Kartentext sprengen sie das Raster, im Protokoll täuschen sie eine Zeile vor |

Gekürzt wird in **Runen**, nicht in Bytes – sonst schneidet die Grenze mitten in
ein Zeichen. Umlaute, Gedankenstriche und Emoji bleiben: Sie gehören zur
Antwort, und genau an ihnen erkennt man einen Menschen.

## Was das Modell darf

Nichts außer antworten.

- Der Anfragekörper enthält **kein** `tools`, `functions`, `tool_choice` oder
  `function_call`. `TestModellBekommtKeineWerkzeuge` hält diese Abwesenheit
  fest – Werkzeuge schaltet man versehentlich zu, nicht versehentlich ab.
- Genau zwei Nachrichten gehen hinaus: der Systemprompt und das Material.
- Spielerinhalt steckt in einer `<material>`-Hülle. Marken im Text werden
  entschärft, eine getippte `</material>`-Zeile kann die Hülle also nicht
  schließen (`TestHuelleHaeltDenAusbruchAus`).
- Die Antwort wird als JSON gelesen und Feld für Feld geputzt, bevor sie
  irgendwo hinkommt.

Der API-Schlüssel liegt beim Server. Kein Endpunkt gibt ihn aus, und die App
kennt ihn nicht – sie spricht nie direkt mit dem Modellanbieter.

## Was die App nicht tut

Die Anzeige ist der zweite Ort, an dem Fremdtext etwas auslösen könnte. Sie tut
es nicht:

- Kein `WebView`, kein `AndroidView`, kein HTML-Rendering.
- Kein `Linkify`, kein `autoLink`, kein `UriHandler`, kein `ClickableText`. Ein
  Kartentext, der aussieht wie eine Adresse, ist damit genau das: ein Text, der
  so aussieht.
- Kein `startActivity` aus Inhalten. Der einzige `PendingIntent` im Projekt
  öffnet die eigene `MainActivity`.
- Keine Ausführung von irgendetwas: kein `Runtime.exec`, kein `ProcessBuilder`.

Lokal liegen nur Token, Serveradresse, Farbschema und zwei Merker
(`Speicher.kt`). Alles Inhaltliche steht auf dem Server.

## Wer sich anmelden darf

`POST /v1/devices` ist der einzige Endpunkt ohne Token – und wer sich ein Gerät
anlegen kann, kann sich eine Party mit sich selbst bauen und ein Match starten.
Jede Runde darin sind zwei Modellaufrufe **auf Rechnung des Betreibers**.

Zwei Riegel:

- **`MIMIK_EINLADUNG`** – ist es gesetzt, muss jede Anmeldung es mitschicken.
  Verglichen wird in konstanter Zeit. Ist es leer, bleibt der Server offen und
  warnt beim Start.
- **Zehn Anmeldungen je Adresse und Stunde.** Hinter einem Reverse Proxy zählt
  die Grenze für alle gemeinsam – das ist gewollt konservativ, denn eine
  weitergereichte Adresse kann sich ein Klient selbst ausdenken.

Erzeugen:

```bash
openssl rand -base64 24
```

## Token

Ein Gerätetoken wird bei der Anmeldung einmal ausgegeben und nur als
**SHA-256-Hash** gespeichert. Wer die Datenbank liest, kann sich damit nicht
anmelden. Ein verlorenes Token lässt sich nicht wiederherstellen – das Gerät
meldet sich neu an.

## Der Behälter

`docker-compose.yml` fährt den Dienst mit:

| Einstellung | Wirkung |
|---|---|
| `read_only: true` | Beschreibbar ist nur das Volume unter `/daten` |
| `tmpfs: /tmp` | 16 MB für SQLites Zwischendateien, sonst nichts |
| `cap_drop: ALL` | Keine Linux-Capability |
| `no-new-privileges` | setuid kann keine Rechte nachladen |
| `pids_limit: 128`, `mem_limit: 512m` | Ein Fehler bleibt im Behälter |
| `USER spiel` (uid 10001) | Kein root, schon im Abbild |
| `127.0.0.1:8080:8080` | Von außen nur über einen Reverse Proxy mit TLS |

Im Abbild liegen `ca-certificates`, `tzdata` und ein Binary. Was nicht da ist,
lässt sich auch nicht ausführen.

## Löschen

| Aktion | Nimmt mit | Lässt stehen |
|---|---|---|
| **Dossier löschen** | Fakten, gesperrte Themen | Tags, Konto, Party, Punktestand |
| **Alles löschen** | Konto, Geräte, Tags, Dossier, Party samt allen Runden | Konto, Tags und Dossier des Gegenübers |

„Alles löschen“ verlangt den eigenen Spitznamen als Bestätigung, nicht nur einen
Klick. Die Party geht mit, weil sie geteilt ist – ein Match ohne zweite Seite
wäre eine Ruine. Fremde Daten löscht man nicht für jemanden mit. Die vergebenen
Fragen fallen in den Pool zurück.

## Was hier nicht abgesichert ist

Ehrlichkeitshalber:

- **Kein TLS im Dienst selbst.** Das gehört vor den Server, in einen Reverse
  Proxy. Die Debug-Fassung der App erlaubt Klartext-HTTP für Tests im eigenen
  Netz (`app/src/debug/res/xml/netz_sicherheit.xml`); die Release-Fassung nicht.
- **Keine Härtung gegen den Modellanbieter.** Wer dort mitliest, liest eure
  Antworten. Das ist der Preis dafür, dass ein fremdes Modell die Fälschungen
  schreibt.
- **Keine Verschlüsselung der Datenbank.** Wer das Volume hat, hat das Dossier.
  Sicherung ist ein `cp` – Zugriff darauf auch.
Der Behälter ist inzwischen gefahren, nicht nur geschrieben: `read_only`,
`cap_drop: ALL`, `no-new-privileges`, `pids_limit`, `mem_limit` und das
16-MB-`tmpfs` sind im laufenden Container nachgewiesen, `touch /probe` scheitert
am schreibgeschützten Wurzeldateisystem, und SQLite legt seine Dateien als uid
10001 im Volume an.

Dabei kam heraus, dass `VOLUME /daten` allein nicht reicht: Docker legt das
Verzeichnis sonst als root mit 0755 an, und der Dienst als uid 10001 bekommt
`unable to open database file (14)`. Das Dockerfile erzeugt `/daten` deshalb
selbst und übergibt es an `spiel`, bevor `VOLUME` kommt.
