# MIMIK · Prompt-Werkbank

> Teil des Projekts [MIMIK](README.md). Diese Datei beschreibt nur die Werkbank,
> mit der der Prompt ohne Server und ohne App getestet wird.

Testet den Prompt, an dem das Spiel hängt – ohne Server, ohne App, ohne
Abhängigkeiten außer Python 3.

Ein Aufruf tut vier Dinge:

| Schritt | Aufgabe |
|---|---|
| **1** Normalform | Schreibt die echte Antwort sauber: Rechtschreibung, Zeichensetzung, Emoji raus |
| **2** Fakt | Was die Antwort über die Person verrät, ein Satz fürs Dossier |
| **3** Sperre | Das Thema, das in dieser Runde nicht mehr vorkommen darf |
| **4** Antworten | Drei Fälschungen im Stil der Person |

Dass die Normalform **in demselben Aufruf** entsteht wie die Fälschungen, ist
kein Sparen von Aufrufen, sondern der Punkt: Vier Texte aus einer Hand stehen in
derselben Schreibweise. Fiele einer aus der Reihe, wäre er erkannt, bevor jemand
seinen Inhalt gelesen hätte.

Danach läuft die **Abstandsprüfung**: Keine Fälschung darf der echten Antwort zu nah
kommen (sonst gäbe es zwei richtige Karten), und die drei dürfen nicht enger
beieinander liegen als zur echten (sonst ist die echte der erkennbare Ausreißer).

## Start

Über OpenCode (OpenAI-kompatibel, wie `provider.ts` im DungeonMaster):

```bash
export MIMIK_BASE_URL="http://localhost:4096/v1" MIMIK_API_KEY="..." MIMIK_HEADERS="x-opencode-session: ..." && python3 harness.py beispiele-kim.json
```

Direkt gegen DeepSeek:

```bash
export MIMIK_BASE_URL="https://api.deepseek.com" MIMIK_API_KEY="..." && python3 harness.py beispiele-kim.json
```

| Variable | Standard | Zweck |
|---|---|---|
| `MIMIK_BASE_URL` | `https://api.deepseek.com` | Basis-URL, `/chat/completions` wird angehängt |
| `MIMIK_API_KEY` | – | Pflicht |
| `MIMIK_MODEL` | `deepseek-v4-flash` | |
| `MIMIK_HEADERS` | – | Zusatz-Header, `Name: Wert` je Zeile – gleiches Format wie `state/settings.ts` |
| `MIMIK_JSON_MODE` | `1` | `0`, falls der Endpunkt `response_format` nicht kennt |

Antworten werden tolerant geparst: auch JSON in einem Codeblock oder in Fließtext
wird erkannt.

## Material

| Datei | Stand |
|---|---|
| `beispiele-kim.json` | **gefüllt, aber erfunden** – „Kim“ ist keine reale Person |
| `beispiele-vorlage.json` | Vorlage – dieselben zehn Fragen, eigene Antworten eintragen |

Die Prompts sind an zehn echten Antworten entwickelt worden. Die stehen nicht im
Repo, und sie kommen auch nicht hinein: Es sind die Antworten zweier realer
Personen auf Fragen wie „Wofür nimmst du dir zu wenig Zeit?“. `beispiele-kim.json`
baut nur ihre **Form** nach – Länge (9 bis 58 Zeichen, Median 45), kein
Schlusspunkt, Satzfragmente ohne Subjekt, Gedankenstrich mit nachgeschobener
Einschränkung, Doppelpunkt statt Nebensatz, eine sehr kurze Antwort. Genau an
diesen Merkmalen hängen die Messungen; deshalb ist die Datei kein beliebiger
Platzhalter.

Je Spieler eine Datei, je Lauf eine. Nicht glätten: die Rohform ist der
Testgegenstand. Die Laufprotokolle (`lauf-*.json`, `lauf-*.log`) stehen in
`.gitignore` – sie enthalten das Rohmaterial.

## Was der Report zeigt

Pro Runde die vier Karten in zufälliger Reihenfolge, so wie ein Spieler sie sähe,
mit der echten markiert. Darunter die Messwerte. Am Ende steht das Dossier, das
während des Laufs gewachsen ist – Fakt für Fakt, mit den verbrauchten Themen.

**Worauf du beim Lesen achtest:**

1. Kannst *du* die echte Karte sofort erkennen? Wenn ja: woran? Das ist der Befund.
2. Fällt eine Fälschung durch Länge oder Satzbau aus der Reihe?
3. Greift eine Fälschung das gesperrte Thema doch auf?
4. Klingen die drei Fälschungen wie Varianten derselben Idee?
5. Sind die Fakten im Dossier wirklich *in der Antwort enthalten* – oder hineingedeutet?

## Bekannte Vereinfachung

Die Ähnlichkeit wird über Zeichen-n-Gramme gerechnet, nicht über Embeddings. Für
einen Rauchtest reicht das, aber die Skala ist eine andere — deshalb **zwei
Schwellensätze**:

| Prüfung | Werkbank (n-Gramme) | Betrieb (Embeddings, Dossier §3.3) |
|---|---|---|
| Nähe zur echten Antwort | `0.35` | `0.72` |
| Streuung der Fälschungen | `0.15` | `0.15` |
| Anker-Streichung | `0.60` (gerichtet) | `0.60` |

Zwei Maße, nicht eins: Antwort gegen Antwort ist symmetrisch (Jaccard über
Vierergramme, ähnliche Längen). Tag gegen Antwort ist **gerichtet** — gefragt ist,
wie viel vom Tag in der Antwort steckt, nicht umgekehrt. Sonst geht ein
Ein-Wort-Tag gegen einen Dreizeiler immer gegen null und die Anker-Streichung
feuert nie.

Beim Wechsel auf das Embedding-Modell aus §10.3 gehören die rechten Werte in
`harness.py` und die Sonderbehandlung für Tags kann entfallen.

Gegengeprüft mit drei konstruierten Fällen: gestreute Fälschungen passieren,
umkreisende fallen an *Nähe*, geclusterte an *Streuung* — jeweils mit der
richtigen Karte als Schuldigem.
