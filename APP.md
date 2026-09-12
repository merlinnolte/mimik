# MIMIK · Android

Kotlin, Jetpack Compose, minSdk 26. Keine Netzwerkbibliothek, kein Navigations-
Framework: zwölf Endpunkte über `HttpURLConnection`, und der Bildschirm ergibt
sich aus dem Spielzustand statt aus einem Navigationsgraphen.

## Bauen

```bash
./gradlew :app:assembleDebug
```

`local.properties` zeigt auf das Android-SDK. Das APK liegt danach unter
`app/build/outputs/apk/debug/app-debug.apk`.

Auf ein Gerät oder den Emulator:

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

## Serveradresse

Beim ersten Start abfragbar. Voreinstellung `http://10.0.2.2:8080` – das ist der
Host aus Sicht des Emulators. Auf einem echten Telefon die LAN-Adresse des
Servers eintragen, oder die öffentliche hinter Caddy.

**Klartext-HTTP ist nur im Debug-Build erlaubt**
(`src/debug/res/xml/netz_sicherheit.xml`). Der Release-Build besteht auf HTTPS,
weil Android seit API 28 unverschlüsselten Verkehr abweist und das für einen
Server im Internet auch richtig ist.

## Aufbau

| Datei | Inhalt |
|---|---|
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

`AppModel.bildschirm` leitet den Bildschirm aus dem Spielzustand ab: kein Token →
Start, noch keine Antwort vom Server → Laden, Intro noch nicht gesehen → Intro,
unter zehn Tags → Tags, keine Party → Party, offene Runde → Schreiben oder Raten.
Damit gibt es keinen Weg, auf dem App und Server auseinanderlaufen.

Die Reihenfolge ist die des Onboardings, und **die Tags stehen vor der Party**.
Was MIMIK über einen weiß, hängt am Spieler, nicht an der Party – es überlebt
jede neue Party, also gehört es auch davor abgefragt. Der Server liefert `tags`
deshalb auf oberster Ebene von `/v1/state`.

Eine Falle dabei: Der frühe Rücksprung bei fehlendem Token darf nicht dazu
führen, dass Compose den Spielzustand nie liest – sonst abonniert es die
Änderung nicht und rendert nach dem Anmelden nicht neu. Deshalb ist das Token
selbst Compose-State.

## Warum es keine Vorschau vor dem Absenden gibt

Es gab einmal einen Zwischenschritt: tippen, „Weiter", die geglättete Fassung
ansehen, „Passt". Der ist entfallen, weil die Glättung jetzt das Modell macht –
im selben Aufruf wie die Fälschungen, also im Worker. Synchron ginge es nicht:
Der Endpunkt braucht 15 bis 190 Sekunden.

Eine Vorschau, die etwas anderes zeigt als später auf der Karte steht, wäre
schlimmer als keine. Stattdessen sagt eine Zeile unter dem Feld, was passiert,
und der Wartebildschirm zeigt die Fassung, sobald sie da ist – mit dem Zusatz
„wird noch geglättet", solange MIMIK arbeitet.

## Benachrichtigungen

Ohne Push-Dienst: Ein wiederkehrender WorkManager-Auftrag holt `/v1/state` und
meldet, wenn dort etwas für dieses Gerät ansteht. Kein Firebase, kein
Google-Konto, keine dritte Partei zwischen Server und Telefon.

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

Darin: umbenennen, Farbschema, Intro erneut ansehen, und zwei getrennte
Löschknöpfe. „Mein Dossier löschen“ nimmt nur, was MIMIK gelernt hat; die Tags
bleiben, sie sind eine Einstellung. „Alles löschen“ verlangt den eigenen
Spitznamen als Bestätigung – bei etwas Unwiderruflichem ist ein Klick zu wenig.

## Was noch fehlt

- Chronik und Statistik
- Die Himmelsschlacht am Matchende
- Entwürfe lokal speichern
