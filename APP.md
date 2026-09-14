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
adb install -r app/build/outputs/apk/debug/mimik-0.10-debug.apk
```

## Freigabe, Signatur und Updates

Was in ein Release hochgeladen wird, ist die Freigabefassung, nicht der
Debugbau:

```bash
./gradlew :app:assembleRelease
```

Dafür braucht es einen eigenen Signaturschlüssel. Einmalig anlegen:

```bash
keytool -genkeypair -v -keystore mimik.jks -alias mimik \
        -keyalg RSA -keysize 4096 -validity 10000
```

Daneben eine `keystore.properties` im Wurzelverzeichnis – beide Dateien stehen
in `.gitignore` und gehören nie ins Repo:

```properties
datei=mimik.jks
speicherwort=…
alias=mimik
schluesselwort=…
```

Fehlt die Datei, entsteht ein **unsigniertes** APK, das sich nicht installieren
lässt. Das ist Absicht: Ein Release, das stillschweigend mit dem
Debug-Schlüssel signiert wird, kann jeder fälschen – dieser Schlüssel liegt auf
jedem Entwicklerrechner und hat überall dasselbe Passwort.

**Den Schlüssel verlieren heißt: keine Updates mehr.** Android nimmt ein Update
nur bei gleicher Signatur an. Geht er verloren, bleibt nur Deinstallieren und
neu Installieren – für alle. Keystore und Passwort gehören deshalb zusammen in
den Passwortmanager, nicht nur auf die Festplatte, auf der gebaut wird.

Seit 0.10 signiert dieser Schlüssel, RSA 4096, `CN=merlinnolte, OU=mimik`:

```
SHA-256  cc6d9c79be852c8ab28ed403f27cd49d2fbf3bdb68c424b7c1ef2f9f204f607b
```

Der Abdruck ist kein Geheimnis, er steht in jedem APK. Er steht hier, damit sich
ein heruntergeladenes APK dagegen prüfen lässt – und damit auffällt, wenn eines
mit einem anderen Schlüssel unterwegs ist:

```bash
apksigner verify --print-certs mimik-<version>-release.apk
```

### Der Bruch bei 0.10

Bis 0.9 wurden Debugbauten verteilt, signiert mit dem Standardschlüssel. Die
Freigabefassung trägt einen anderen, also ist dieser eine Sprung kein Update,
sondern eine Neuinstallation. Damit dabei niemand sein Konto verliert, gibt es
den **Umzug** (`Umzug.kt`): Auf dem Server hängt alles am Token, und das steht in
den Einstellungen unter „Dieses Gerät". Vor dem Deinstallieren kopieren, nach
dem Installieren auf dem Startbildschirm unter „Ich hatte MIMIK schon"
eintragen – derselbe Spieler, mit Party, Punktestand und Dossier.

**Eine Brücke braucht es trotzdem.** In 0.9 gibt es den Schlüssel noch nicht zu
sehen; wer nur 0.9 hat, kommt gar nicht erst an ihn heran. Deshalb gehört in das
Release 0.10 **beides**:

```bash
./gradlew :app:assembleDebug :app:assembleRelease
```

`mimik-0.10-debug.apk` trägt weiter den Debug-Schlüssel, installiert sich also
über eine vorhandene 0.9 – und zeigt den Umzugsschlüssel. Reihenfolge für die
wenigen, die schon spielen: 0.10-debug darüber installieren, Schlüssel kopieren,
deinstallieren, `mimik-0.10-release.apk` installieren, Server und Schlüssel
eintragen. Danach ist Schluss mit Debugbauten im Release; die App sucht sich
ihre Updates von da an selbst und nimmt dabei ohnehin nur das Freigabe-APK
(`apkAsset` bevorzugt den Namen mit „release").

### Wie die App Updates findet

`Aktualisierung.kt` fragt beim Start – höchstens alle sechs Stunden – das
neueste Release bei `api.github.com` ab. Ein Entwurf, eine Vorabfassung oder ein
Release ohne APK ist keines; verglichen wird **zahlenweise**, denn als Text wäre
`0.10` kleiner als `0.9`. Liegt etwas vor, steht ein Hinweis in der Lobby und in
den Einstellungen. Geladen wird auf Knopfdruck ins Cacheverzeichnis, dann
übernimmt der Installer des Systems: MIMIK tauscht sich nicht still selbst aus.
Dafür `REQUEST_INSTALL_PACKAGES` und ein `FileProvider` – eine `file:`-Adresse
nimmt seit Android 7 niemand mehr an.

Alles Prüfbare daran steht als reine Funktion oben in der Datei;
`AktualisierungTest` hält die Fälle fest, darunter den Sprung von 0.9 auf 0.10.

## Serveradresse

**In der Freigabefassung ist keine voreingestellt.** Ein öffentliches APK darf
niemanden ungefragt auf einen fremden Server schicken – wer es installiert,
trägt seinen eigenen ein, und das Feld ist der erste Schritt beim Anmelden. Im
Debugbau steht `https://mimik.merlinnolte.de` als Vorgabe und das Feld bleibt
hinter der Zeile „Anderer Server" versteckt; beides kommt aus
`BuildConfig.VORGABE_SERVER`. Zum Testen: `http://10.0.2.2:8080` ist der Host aus
Sicht des Emulators, im eigenen WLAN die LAN-Adresse mit dem Port aus
`MIMIK_PORT`.

Eingetippt wird ohne Schema – `serverNormalform` setzt `https://` davor und
nimmt den Schrägstrich am Ende weg. Klartext geht nur, wenn jemand `http://`
ausdrücklich tippt: Ein Vertippen soll die Verschlüsselung nicht abschalten.

`Speicher.wandern()` ist die Klappe dazu. Die Vorgabe stand bisher nur im Code
und wurde nur beim Ändern geschrieben; wer angemeldet ist, ohne je eine Adresse
eingetragen zu haben, stünde nach dem Update vor einem leeren Feld – das Token
noch da, aber niemand mehr, dem man es zeigt. Die Wanderung trägt die alte
Adresse nach, bevor `Netz` überhaupt gebaut wird.

**Klartext-HTTP ist nur im Debug-Build erlaubt**
(`src/debug/res/xml/netz_sicherheit.xml`). Der Release-Build besteht auf HTTPS,
weil Android seit API 28 unverschlüsselten Verkehr abweist und das für einen
Server im Internet auch richtig ist.

## Aufbau

| Datei | Inhalt |
|---|---|
| `Ableitung.kt` | Bildschirm, Auflösung, Lobbyzeilen, Meldeplan – ohne Android-Import, deshalb prüfbar |
| `Lobby.kt` | Übersicht, Warteraum, Namenswahl, Klonblick, der Rückweg oben links |
| `Theme.kt` | Die vier Paletten aus `themes.css`, unverändert. JetBrains Mono gebündelt. |
| `Mimik.kt` | Das animierte Gesicht als Zeichenmatrix – Leerlauf, Denkt, Triumph, Getroffen |
| `Bausteine.kt` | `Panel`, `Knopf`, `Feld`, `Punktebalken` nach `Panel.svelte` und `app.css` |
| `Netz.kt` | DTOs und HTTP-Klient |
| `AppModel.kt` | Zustand, Züge, Abfragetakt |
| `Screens.kt` | Alle Bildschirme, plus die Hülle, die jeden mittig stellt |
| `Melder.kt` | Benachrichtigungen: WorkManager fragt selbst beim Server nach |
| `Speicher.kt` | Token, Serveradresse, Palette, zwei Merker – mehr liegt nicht lokal |
| `Aktualisierung.kt` | Updates von der Releaseseite: suchen, laden, dem Installer hinhalten |
| `Umzug.kt` | Serveradresse normalisieren, Schlüssel lesen und zeigen – reiner Text, deshalb prüfbar |

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

Darin: umbenennen, Farbschema, Intro erneut ansehen, die laufende Fassung mit
der Suche nach einer neueren, der eigene Schlüssel für den Umzug, das eigene Profil und
Dossier ansehen – was MIMIK aus den eigenen Antworten mitgeschrieben hat, welche Themen
verbraucht sind, welche Tags gesetzt – und zwei getrennte
Löschknöpfe. „Mein Dossier löschen“ nimmt nur, was MIMIK gelernt hat; die Tags
bleiben, sie sind eine Einstellung. „Alles löschen“ verlangt den eigenen
Spitznamen als Bestätigung – bei etwas Unwiderruflichem ist ein Klick zu wenig.

## Der Klonblick

Zwischen Absenden und Raten steht ein Bildschirm, den es vorher nicht gab: die
eigene Antwort, und dahinter die drei Fälschungen, die MIMIK daraus gebaut hat —
jede mit einem Satz, **woraus** sie sie gebaut hat („Du liest abends
Nachrichten, obwohl das den Schlaf verschlechtert – das habe ich zu einer
kleinen Selbstanklage umgebaut").

Drei Dinge machen ihn möglich, und alle drei waren schon da:

- **Der eigene Kartensatz steht früh.** Seit 0.5 läuft die Erzeugung vor: MIMIK
  baut die Karten über einen, sobald man geantwortet hat — lange bevor die
  andere Seite dran war. Genau dort lag bisher nur Wartezeit.
- **Er verrät nichts.** Geraten wird über die *andere* Seite. Wer seine eigene
  Antwort getippt hat, weiß ohnehin, welche der vier Karten sie ist.
- **Die Begründung ist das eigene Material.** Sie zitiert Fakten aus den eigenen
  Antworten. Die Begründungen der Karten über das Gegenüber gehen **nie** an
  diese Seite — sie zitieren dessen Dossier, unter Umständen aus einer anderen
  Partie. Der Server liefert sie deshalb nur im Block `meine_karten`; ein Test
  hält das fest.

Die Klone erscheinen einer nach dem anderen (900 ms, dann 650). Nicht als
Zierde: Sie treten hinter der echten Antwort an, und das Nacheinander ist das,
was aus vier Textblöcken ein Klonen macht. Ein Platzhalter fester Höhe hält die
Spalte ruhig, solange noch welche fehlen — sonst rutscht der mittig gesetzte
Bildschirm bei jedem Schritt nach oben, derselbe Fehler wie einst bei der
Wartezeile.

Der Merker (`Speicher.klone`) hängt an Partie **und** Match: Rundennummern
fangen in jedem Match wieder bei 1 an.

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
