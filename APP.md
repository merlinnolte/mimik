# MIMIK · Android

Kotlin, Jetpack Compose, minSdk 26. Keine Netzwerkbibliothek, kein Navigations-
Framework: die Endpunkte laufen über `HttpURLConnection`, und der Bildschirm
ergibt sich aus dem Spielzustand statt aus einem Navigationsgraphen.

## Bauen

```bash
./gradlew :app:assembleDebug
```

`local.properties` zeigt auf das Android-SDK. Das APK liegt danach unter
`app/build/outputs/apk/debug/mimik-<version>-debug.apk` – der Name kommt aus
`archivesName` in `app/build.gradle.kts` und trägt die Version, damit in einem
Downloadordner nicht drei gleichnamige Dateien liegen.

Auf ein Gerät oder den Emulator:

```bash
adb install -r app/build/outputs/apk/debug/mimik-0.8-debug.apk
```

## Serveradresse

Fest voreingestellt auf `https://mimik.merlinnolte.de` – den eigenen Server
hinter dem Reverse Proxy. Wer spielt, bekommt das Feld nicht zu sehen; es steht
hinter der Zeile „Anderer Server" unter dem Anmeldefeld. Zum Testen:
`http://10.0.2.2:8080` ist der Host aus Sicht des Emulators, im eigenen WLAN die
LAN-Adresse mit dem Port aus `MIMIK_PORT`.

**Klartext-HTTP ist nur im Debug-Build erlaubt**
(`src/debug/res/xml/netz_sicherheit.xml`). Der Release-Build besteht auf HTTPS,
weil Android seit API 28 unverschlüsselten Verkehr abweist und das für einen
Server im Internet auch richtig ist.

## Aufbau

| Datei | Inhalt |
|---|---|
| `Ableitung.kt` | Bildschirm, Auflösung, Lobbyzeilen, Meldeplan – ohne Android-Import, deshalb prüfbar |
| `Lobby.kt` | Übersicht, Warteraum, Namenswahl, der Rückweg oben links |
| `Theme.kt` | Die vier Paletten aus `themes.css`, unverändert. JetBrains Mono gebündelt. |
| `Mimik.kt` | Das animierte Gesicht als Zeichenmatrix – Leerlauf, Denkt, Triumph, Getroffen |
| `Bausteine.kt` | `Panel`, `Knopf`, `Feld`, `Punktebalken` nach `Panel.svelte` und `app.css` |
| `Netz.kt` | DTOs und HTTP-Klient |
| `AppModel.kt` | Zustand, Züge, Abfragetakt |
| `Screens.kt` | Alle Bildschirme, plus die Hülle, die jeden mittig stellt |
| `Melder.kt` | Benachrichtigungen: WorkManager fragt selbst beim Server nach |
| `Speicher.kt` | Token, Serveradresse, Palette, zwei Merker – mehr liegt nicht lokal |

### Warum die Schrift mitgeliefert wird

Androids System-Monospace deckt U+2500–U+259F nicht ab und fällt für Rahmen- und
Blockzeichen auf eine andere Schrift mit anderen Metriken zurück. MIMIKs Gesicht
zerfällt dann in Fragmente. Mit JetBrains Mono sitzt die Zeichenmatrix im Raster.
Zwei TTF, zusammen etwa 550 KB.

### Warum es keinen Navigationsgraphen gibt

**Der Bildschirm ist eine Funktion von `(Spielzustand, offene Partie)`** –
`bildschirmFuer` in `Ableitung.kt`: kein Token → Start, noch keine Lobby → Laden,
Name nicht eindeutig → Namenswahl, unter zehn Tags → Tags, keine Partie gewählt →
Lobby, offene Runde → Schreiben oder Raten. Damit gibt es keinen Weg, auf dem App
und Server auseinanderlaufen.

Das zweite Argument ist eine **Auswahl über einer Menge**, kein
Navigationszustand: `offenePartie` kann nur Werte annehmen, die in der Lobby
vorkommen, und wird bei jedem Lesen dagegen geprüft. Verschwindet die Partie,
fällt die Auswahl weg und der Weg endet von selbst in der Übersicht. Ausgewählt
wird über die **ID**, nie über einen Index – sonst spränge die offene Partie weg,
sobald der Server anders sortiert. Der Test dazu (`umsortierte lobby aendert den
bildschirm nicht`) ist der, der diese Entscheidung festhält.

Die Reihenfolge ist die des Onboardings, und **die Tags stehen vor der Party**.
Was MIMIK über einen weiß, hängt am Spieler, nicht an der Party – es überlebt
jede neue Party, also gehört es auch davor abgefragt. Der Server liefert `tags`
deshalb auf oberster Ebene von `/v1/state`.

Die Reihenfolge ist die des Onboardings. Eine Falle dabei: Der frühe Rücksprung bei fehlendem Token darf nicht dazu
führen, dass Compose den Spielzustand nie liest – sonst abonniert es die
Änderung nicht und rendert nach dem Anmelden nicht neu. Deshalb ist das Token
selbst Compose-State.

## Warum es keine Vorschau vor dem Absenden gibt

Es gab einmal einen Zwischenschritt: tippen, „Weiter", die geglättete Fassung
ansehen, „Passt". Der ist entfallen, weil die Glättung jetzt das Modell macht –
im selben Aufruf wie die Fälschungen, also im Worker. Synchron ginge es nicht:
Der Endpunkt braucht 15 bis 190 Sekunden.

Eine Vorschau, die etwas anderes zeigt als später auf der Karte steht, wäre
schlimmer als keine. Der Wartebildschirm zeigt die Fassung, sobald sie da ist –
mit dem Zusatz „wird noch geglättet", solange MIMIK arbeitet.

Unter dem Eingabefeld stand dazu einmal eine erklärende Zeile. Sie ist raus:
Wer eine Frage beantwortet, soll über die Antwort nachdenken und nicht über die
Mechanik dahinter.

### Der Fortschrittsbalken

`wartet_seit` aus `/v1/state` sind die Sekunden seit der **späteren** der beiden
Antworten – erst dann arbeitet MIMIK überhaupt (`Runde.Ableiten`). Gezählt ab
der eigenen Antwort stand der Balken für den, der zuerst schrieb und dann eine
halbe Stunde wartete, beim ersten Blick schon voll: Die Zeit war ja wirklich
vergangen, nur nicht mit Arbeit. Weil der Wert vom Server kommt, steht der
Balken auch nach einem Neustart der App richtig.

Er läuft in ungleichen Schritten und ungleichen Abständen auf fünfzehn Sekunden
zu – wie etwas, das arbeitet, und nicht wie eine Uhr. Gemessen liegt ein Aufruf
bei rund zehn Sekunden Median, aber weist die Abstandsprüfung eine Fassung
zurück, kommt ein zweiter dazu: Eine echte Restzeit wäre geraten. Deshalb steht
darunter die tatsächlich verstrichene Zeit in Sekunden – die Erwartung im
Balken, die Wahrheit in der Zeile.

Sind die Karten früher da, läuft der Balken trotzdem sichtbar voll: `AppModel`
hält den Wartebildschirm dafür 900 ms länger (`balkenLaeuftVoll`). Ein Balken,
der mitten im Lauf verschwindet, lässt den Moment unfertig aussehen.

## Benachrichtigungen

Ohne Push-Dienst: Ein wiederkehrender WorkManager-Auftrag holt `/v1/lobby` und
meldet, wenn dort etwas ansteht. Kein Firebase, kein Google-Konto, keine dritte
Partei zwischen Server und Telefon.

**Gemeldet wird genau zweierlei:** eine Antwort fehlt noch, oder die Karten
liegen und es kann geraten werden. Beides steht in `dran`; der Server setzt
`raten` erst, wenn die vier Karten da sind. Alles andere ist entfallen – „Die
Party ist vollständig" war kein Auftrag, und eine Auflösung wartet.

Drei Dinge waren hier kaputt und sind es nicht mehr:

1. **Keine Vordergrundprüfung.** Wer gerade auf dem Bildschirm saß, bekam
   trotzdem einen Zettel. `Speicher.gesehenStempel` wird in `onResume` gesetzt,
   im Beobachtungstakt aufgefrischt und in `onStop` genullt; `imVordergrund()`
   heißt „jünger als 60 Sekunden". Ein Ja/Nein-Merker wäre eine Falle: Stirbt der
   Prozess auf „ja", käme nie wieder eine Meldung.
2. **Meldungen zu einer vergangenen Phase.** Der Abruf darf zehn Sekunden auf
   die Verbindung warten – in der Zeit kann der Zug gemacht sein. Deshalb wird
   **vor und nach** dem Abruf geprüft. Und eine gestellte Meldung blieb stehen,
   weil `setAutoCancel` nur beim Antippen abräumt: Jetzt liefert `meldeplan` eine
   **Löschliste**, und was nicht mehr fällig ist, fällt aus dem Schacht.
3. **Der Merker wurde im Vordergrund nie zurückgesetzt.** `lobbyUebernehmen`
   kürzt ihn nach jedem Abgleich – **nur kürzen, nie erweitern**: Zuschauen ist
   keine Meldung.

Eine Meldungs-ID je Partie statt der festen `1`; sonst überschreibt bei zwei
Partien die eine die andere.

Der Preis ist die Latenz: WorkManager lässt einen wiederkehrenden Auftrag
frühestens alle 15 Minuten laufen, im Dösen auch seltener. Für ein Spiel, dessen
ganze Idee Zeitunabhängigkeit ist, ist das der richtige Tausch.

Ab Android 13 braucht es `POST_NOTIFICATIONS`. Die App fragt am Ende des Intros
danach und bleibt ohne die Erlaubnis voll benutzbar.

Ein Merker (`letzteMeldung`) verhindert, dass jeder Durchlauf dieselbe offene
Runde erneut meldet.

## Einstellungen

Zahnrad oben rechts, auf jedem Bildschirm außer Anmelden, Laden, Intro und den
Einstellungen selbst. Es liegt als Überlagerung über dem Bildschirm, nicht in
ihm – sonst bräuchte jeder Bildschirm eine Kopfleiste, und die mittige Anordnung
wäre hin.

Darin: umbenennen, Farbschema, Intro erneut ansehen, das eigene Profil und
Dossier ansehen – was MIMIK aus den eigenen Antworten mitgeschrieben hat, welche Themen
verbraucht sind, welche Tags gesetzt – und zwei getrennte
Löschknöpfe. „Mein Dossier löschen“ nimmt nur, was MIMIK gelernt hat; die Tags
bleiben, sie sind eine Einstellung. „Alles löschen“ verlangt den eigenen
Spitznamen als Bestätigung – bei etwas Unwiderruflichem ist ein Klick zu wenig.

## Der Zeilenumbruch

`Theme.kt` setzt `LineBreak.Strategy.Balanced` auf alle Fließtextstile. Der
gierige Umbruch füllt die erste Zeile bis zum Rand und lässt den Rest als
Stummel stehen – bei zweizeiligen Sätzen, und aus denen besteht die App fast
nur, sieht das aus wie ein Fehler.

Vier Ausnahmen, jede mit Grund: MIMIKs Gesicht (`softWrap = false` – jeder
Umbruch zerlegt die Zeichenmatrix), das Intro (der Absatz wächst wortweise, ein
ausgeglichener Umbruch verteilte bei jedem Wort neu und ließe den gelesenen Teil
springen), die Werte mit fester Breite in `Bausteine.kt` und der Partycode.

Die zwei handgesetzten `\n` sind weg. Sie standen da, um genau diesen Mangel von
Hand zu umgehen – und würden ihn jetzt wieder herstellen, weil ein hartes `\n`
in zwei getrennt ausgeglichene Absätze teilt.

## Was noch fehlt

- Chronik und Statistik
- Die Himmelsschlacht am Matchende
- Entwürfe lokal speichern
