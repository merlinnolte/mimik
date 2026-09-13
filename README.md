# MIMIK

Ein rundenbasiertes Spiel für **genau zwei Personen**, die sich gut kennen.

Beide beantworten dieselbe Frage. MIMIK – ein Sprachmodell – liest die echte
Antwort, schreibt drei Fälschungen im Stil der schreibenden Person und legt
alle vier gemischt der anderen Seite vor. Wer die echte findet, holt einen Punkt
für die Menschen. Wer danebengreift, gibt einen an MIMIK ab. Erster auf zehn
gewinnt.

Das Spiel ist **kooperativ**: Ihr steht gemeinsam gegen MIMIK, nicht gegeneinander.
Und es ist **zeitunabhängig** – eine Runde kann Minuten oder Tage dauern, es gibt
keine Uhr.

```
        ◦                      ╔═════════╗
        │                      ║ ▄     ▄ ║
   ╔═════════╗                 ║    ·    ║
   ║ ▀     ▀ ║   →  ratet  →   ║  ─────  ║
   ║    ·    ║                 ╚══╤═══╤══╝
   ║  ╲___╱  ║
   ╚══╤═══╤══╝
```

## Die drei Teile

| Teil | Was | Doku |
|---|---|---|
| **Server** | Go-Binary, SQLite, ein Docker-Container. Hält das ganze Spiel. | [SERVER.md](SERVER.md) |
| **App** | Android, Jetpack Compose, Terminal-Optik. | [APP.md](APP.md) |
| **Werkbank** | Python-Skript, das den Prompt ohne Server testet. | [WERKBANK.md](WERKBANK.md) |
| **Modellmessung** | `messe-modelle.py` – misst, welches Modell deines Endpunkts schnell **und** brauchbar antwortet | — |

Dazu: [SICHERHEIT.md](SICHERHEIT.md) – was der Server annimmt, was er dem Modell
gibt und was er nicht tut. [agents.md](agents.md) – die Regeln des Projekts für
alle, die daran weiterarbeiten, Mensch wie Agent.

Version **0.7**. Sie steht an zwei Stellen und muss dort gleich bleiben:
`internal/version.go` und `app/build.gradle.kts` (`versionName`).

## Loslegen

**Server:**

```bash
cp .env.beispiel .env   # MIMIK_API_KEY und MIMIK_EINLADUNG eintragen
docker compose up -d --build
```

**App:**

```bash
./gradlew :app:assembleDebug
```

Das APK liegt danach unter `app/build/outputs/apk/debug/`. Beim ersten Start
Serveradresse und Spitzname eintragen; verlangt der Server eine Einladung, auch
die.

## Wie eine Runde abläuft

1. Beide bekommen dieselbe Frage und antworten für sich.
2. Sobald beide geantwortet haben, arbeitet MIMIK. Das dauert – gemessen 15
   Sekunden bis über fünf Minuten. Die Runde steht solange auf `MIMIK_ARBEITET`.
3. In **einem** Aufruf entstehen alle vier Karten: MIMIK schreibt die echte
   Antwort sauber und die drei Fälschungen gleich mit. Vier Texte aus einer
   Hand, in derselben Rechtschreibung – sonst verriete schon ein fehlendes Komma,
   wer getippt hat.
4. Jede Seite bekommt **vier Karten über die andere Person** und wählt eine.
5. Sobald beide getippt haben, löst die Runde auf, die Punkte fallen, die
   nächste Frage steht.

Und nach jeder Runde wächst das **Dossier**: MIMIK zieht aus der echten Antwort
einen Fakt, merkt ihn sich und sperrt das Thema, damit es in späteren Runden
nicht noch einmal vorkommt. Das Dossier hängt am Spieler, nicht an der Party –
es überlebt jede neue Party.

## Was das Spiel schwer macht

Nicht der Inhalt, sondern der **Abstand**. Zwei Fehler ruinieren eine Runde:

- Eine Fälschung liegt zu nah an der echten Antwort. Dann gibt es zwei richtige
  Karten und die Runde ist Zufall.
- Die drei Fälschungen liegen enger beieinander als an der echten. Dann ist die
  echte der erkennbare Ausreißer, und man braucht die Person gar nicht zu kennen.

Beides misst der Server nach dem Erzeugen und lässt neu schreiben, wenn es nicht
passt. Die Werkbank misst dasselbe an echtem Material, siehe
[WERKBANK.md](WERKBANK.md).

## Lizenz

Privates Projekt, keine Lizenz vergeben.
