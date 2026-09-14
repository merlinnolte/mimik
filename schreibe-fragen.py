#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Laesst das Modell Fragen vorschlagen und siebt sie mechanisch.

    python3 schreibe-fragen.py alltag 36
    python3 schreibe-fragen.py --alle

Schreibt NIE nach internal/seed/fragen.json. Die Vorschlaege landen in
vorschlaege-fragen.json, je Zeile mit ihrer naechsten vorhandenen Frage und dem
Abstand dazu - damit Auswaehlen ein Blick je Zeile ist.

Der Filter ist keine Geschmacksfrage, er kommt aus der Mechanik des Spiels:

  Abstand      Fragennaehe < FrageDoppel gegen den Vorrat UND gegen die schon
               angenommenen Vorschlaege. Zwei Fragen, die dasselbe fragen, sind
               fuer denselben Menschen eine Frage - und fragen_vergeben merkt
               sich nur die id.
  Kern >= 2    Eine Frage mit einem einzigen Inhaltswort kollidiert mit allem,
               was dieses Wort benutzt ("Wovon moechtest du weniger haben?").
  kein warum   Verlangt eine Begruendung, die die Faelschungen nachmachen
               muessen - und MaxKausal = 1 bestraft dann genau das.
  Interrogativ Eine Frage, die mit einem Verb anfaengt, ist eine Ja/Nein-Frage.
               Vier Karten mit "ja" sind kein Spiel. Jede Frage im Bestand
               faengt mit einem Interrogativum an, hoechstens mit einer
               Praeposition davor ("Bei welcher Kleinigkeit ...").
  Laenge       28-105 Zeichen, endet auf ein Fragezeichen. Form des Bestands.
  keine Zahl   Eine gefaelschte Zahl ist trivial, und der Abstand daran nicht
               messbar.
  Rahmen       Hoechstens eine Frage je Satzgeruest. Gemessen am ersten Lauf
               (Rubrik zukunft): Das Modell faellt in eine Rille und schreibt
               "Was willst du in deinem Leben auf keinen Fall verpassen /
               verlieren / aufgeben / bereuen?". Fragennaehe sieht das NICHT -
               die Inhaltswoerter sind verschieden -, es ist derselbe
               Gerippebruch wie bei den Faelschungen, nur eine Ebene hoeher.
  Voraussetzung Keine Frage, die etwas voraussetzt, das nicht jeder hat -
               Garten, Kind, Auto, Haustier, Balkon. Wer keinen Garten hat,
               antwortet "ich habe keinen Garten", und die Runde ist weg:
               MIMIK kann das nicht faelschen und der Partner nicht raten.
  Bekenntnis   "in deinem Leben", "eines Tages", "im Alter", "auf keinen Fall"
               erzeugen ein Bekenntnis statt eines Dings, und vier Bekenntnisse
               klingen gleich. Der Filter kann Regel 1 nicht pruefen, aber er
               kann ihre haeufigsten Traeger wegwerfen.
"""
import importlib.util
import io
import json
import os
import re
import sys

_spec = importlib.util.spec_from_file_location("pf", "pruefe-fragen.py")
pf = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(pf)  # bringt kern(), naehe(), DOPPEL, NACHBAR aus frage.go

POOL = "internal/seed/fragen.json"
AUS = "vorschlaege-fragen.json"

# Heutige Mischung, hochskaliert - nicht gleichverteilt. Gezogen wird
# gleichverteilt ueber den ganzen Vorrat, also IST die Rubrikverteilung die
# Mischung, die ein Match zu sehen bekommt. Gleichmaessig aufzufuellen hiesse,
# aus jeder vierten Frage "alltag" jede achte zu machen.
# haltung steht auf 57 und nicht auf 59: Dort hatte das Modell eine Idee in
# dreissig Kostuemen ("was tust du, obwohl du X denkst"), und die letzten zwei
# Plaetze waeren nur mit einer Wiederholung zu fuellen gewesen. Eine Rubrik
# darf kleiner bleiben als geplant; eine Frage doppelt zu stellen nicht.
ZIEL = {
    "alltag": 66, "haltung": 57, "vergangenheit": 55, "vorliebe": 46,
    "hypothetisch": 37, "zukunft": 24, "menschen": 40, "koerper": 33,
}

RUBRIKEN = {
    "alltag": "Der gewoehnliche Tag: Dinge in der Wohnung, Handgriffe, "
              "Gewohnheiten, Aufgeschobenes, kleine Heimlichkeiten.",
    "haltung": "Was jemand fuer richtig haelt und wie er sich dabei erlebt: "
               "Meinungen, Regeln, Scham, Stolz, Grenzen.",
    "vergangenheit": "Was war: Kindheit, Schule, erste Male, Orte, Menschen, "
                     "Peinlichkeiten, Dinge, die man verlernt hat.",
    "vorliebe": "Was jemand mag, ohne es begruenden zu muessen: Essen, Musik, "
                "Geraeusche, Orte, Wetter, Tageszeiten, Gegenstaende.",
    "hypothetisch": "Ein gesetzter Fall: waere, koennte, muesste. Immer mit "
                    "einem konkreten Rahmen, nie eine blanke Wunschfrage.",
    "zukunft": "Was kommen soll oder nicht: Vorhaben, Befuerchtungen, Dinge, "
               "die bleiben sollen.",
    "menschen": "Andere Personen: wem man aehnlich wird, wen man anruft, "
                "wessen Urteil zaehlt, wen man meidet, von wem man etwas hat. "
                "NIE ueber den Spielpartner - dessen Antwort wuesste der "
                "Ratende schon.",
    "koerper": "Der Koerper als Alltagsgegenstand: Schlaf, Haende, Sinne, "
               "Hunger, Muedigkeit, Bewegung, Kaelte, Schmerz im harmlosen "
               "Sinn. Im Register des Bestands, nichts Intimes - die Antwort "
               "liest ein Mensch, und ein Modell baut sie nach.",
}

PROMPT = """Du schreibst Fragen fuer ein Spiel. Zwei Menschen, die sich gut
kennen, beantworten dieselbe Frage in einem Satz. Ein Sprachmodell schreibt drei
Faelschungen im Stil der antwortenden Person. Der andere bekommt vier Karten und
sucht die echte heraus.

Daraus folgt alles, was eine Frage gut oder unbrauchbar macht:

1. DIE ANTWORT MUSS EIN GEGENSTAND SEIN, KEIN BEKENNTNIS.
   Gut:      "Was hebst du auf, obwohl es kaputt ist?"      -> ein Ding
   Unbrauchbar: "Was ist dir wichtig im Leben?"             -> vier gleich
   klingende Karten. Konkret heisst faelschbar heisst ratbar.

   Deshalb VERBOTEN: "in deinem Leben", "im Leben", "eines Tages", "im Alter",
   "auf keinen Fall", "bevor es zu spaet ist". Das sind Traeger von
   Bekenntnissen. Frage nach einem Ding, einem Handgriff, einem Ort, einem
   Menschen, einer Begebenheit - nie nach einer Einstellung.

2. EIN SATZ MUSS REICHEN. Keine Frage, die eine Geschichte verlangt.

3. KEIN "WARUM", KEIN "UND WARUM". Eine verlangte Begruendung muessen die
   Faelschungen nachmachen, und das verraet sie.

4. KEINE JA/NEIN-FRAGE. Jede Frage faengt mit einem Interrogativum an
   (Was, Welche, Wofuer, Wobei, Wovor, Wo, Wen, Wann ...), hoechstens mit einer
   Praeposition davor ("Bei welcher Kleinigkeit ...").

5. KEINE ZAHL, KEIN DATUM, KEIN EIGENNAME. Eine gefaelschte Zahl ist trivial.

6. DER PARTNER SOLL ES ERRATEN KOENNEN, ABER NICHT WISSEN. Am besten sind
   Fragen mit einer Einschraenkung, die die Antwort schaerft: "obwohl",
   "ohne dass", "das kaum jemand", "wenn niemand zusieht".

7. 28 BIS 105 ZEICHEN, Anrede mit "du", Fragezeichen am Ende.

So klingt der Bestand - Tonfall, Laenge, Bauart:
%s

Rubrik: %s
%s

Diese Fragen der Rubrik gibt es SCHON. Schreibe nichts, was dieselbe Antwort
bekaeme, und nichts mit demselben Satzbau:
%s

8. NICHTS VORAUSSETZEN, WAS NICHT JEDER HAT. Kein Garten, kein Kind, kein
   Auto, kein Haustier, kein Balkon, keine Beziehung. Wer es nicht hat,
   antwortet "habe ich nicht", und die Runde ist weg. Eine Wohnung, Haende,
   Schlaf, Essen, ein Weg zur Arbeit - das hat jeder.

9. JEDE FRAGE EIN EIGENES SATZGERUEST. Nicht "Was willst du unbedingt noch
   erreichen?" und daneben "Was willst du unbedingt noch erleben?" - das ist
   eine Frage mit zwei Verben. Wechsle den Satzbau, nicht das Verb.

Schreibe %d NEUE Fragen dieser Rubrik. Jede fragt nach etwas anderem als alle
anderen in deiner Liste, und keine zwei teilen ihren Satzbau. Keine
Nummerierung, keine Vorrede.

Gib NUR JSON zurueck: {"fragen": ["...", "..."]}"""

INTERROGATIV = re.compile(
    r"^(bei|an|in|auf|aus|mit|von|fuer|für|ueber|über|zu|nach|vor|um|durch|gegen)?\s*"
    r"(was|wer|wen|wem|wessen|wie|wo|wohin|woher|wann|wof[uü]r|wobei|worauf|"
    r"wor[uü]ber|wovor|wovon|womit|welch\w*)\b", re.I)
# Gesucht ist die Frage, deren ANTWORT eine Zahl ist - nicht jede Frage, in der
# eine Zahl vorkommt. Erster Versuch war eine Liste von Zahlwoertern, und die
# warf 23 gute Fragen weg, weil "ein" der unbestimmte Artikel ist und "gehst du
# ein" ein trennbares Verb ("Welchen Kompromiss gehst du ein, obwohl er dir
# widerstrebt?"). Eine gezaehlte Antwort kuendigt sich im Fragewort an.
ZAHLFRAGE = re.compile(r"(\bwie\s+(viel\w*|oft|lange|h[aä]ufig)\b|"
                       r"\bwelches\s+jahr\b|\d)", re.I)
BEKENNTNIS = ("in deinem leben", "im leben", "eines tages", "im alter",
              "auf keinen fall", "bevor es zu spät ist", "in deinem alltag")
# Was nicht jeder hat. "Wohnung" steht hier bewusst NICHT - irgendwo wohnt
# jeder, und mehrere gute Fragen im Bestand fragen danach.
#
# Als Wortanfang gepruefft warf "kind" 41 gute Fragen weg, weil "als Kind" die
# eigene Kindheit meint und nicht ein eigenes Kind ("Welches Lied hast du als
# Kind falsch verstanden?"). Besitz kuendigt sich durch das Possessivpronomen
# an, also steht es im Muster.
VORAUSSETZUNG = (r"garten", r"dein\w*\s+kind\w*", r"deine\s+kinder",
                 r"dein\w*\s+(sohn|tochter)", r"enkel\w*", r"dein\w*\s+auto",
                 r"haustier\w*", r"dein\w*\s+(hund|katze)", r"balkon\w*",
                 r"dein\w*\s+(partner\w*|freund\w*|beziehung)",
                 r"dein\w*\s+(chef\w*|kolleg\w*)")

# So viele fuehrende Rahmenwoerter am Stueck, und zwei Fragen haben dasselbe
# Geruest. Fuenf und nicht vier wie bei MinGleicherAnfang: Fragen fangen alle
# gleich an ("was willst du ..."), und bei vier fielen gute Fragen mit.
RAHMENANFANG = 5


def geruest(text):
    """Der Satzbau einer Frage ohne ihren Gegenstand - Gerippe, eine Ebene
    hoeher angewandt. Fuer Faelschungen ist das geteilte Geruest ein
    Verraeter; hier ist es eine Rille, in die das Modell faellt."""
    kern = pf.kern(text)
    return [w for w in re.findall(r"[\wäöüß]+", text.lower()) if w not in kern]


def siebe(kandidat, pool, angenommen):
    """Gibt (ok, grund, naechste, wert) zurueck."""
    t = kandidat.strip()
    if not t.endswith("?"):
        return False, "kein Fragezeichen", "", 0.0
    if not 28 <= len(t) <= 105:
        return False, "%d Zeichen" % len(t), "", 0.0
    if not INTERROGATIV.match(t):
        return False, "faengt nicht mit einem Interrogativum an", "", 0.0
    if "warum" in t.lower():
        return False, "verlangt eine Begruendung", "", 0.0
    if not re.search(r"\b(du|dir|dich|dein\w*)\b", t.lower()):
        return False, "spricht nicht mit du", "", 0.0
    if len(pf.kern(t)) < 2:
        return False, "Kern unter zwei Woertern", "", 0.0
    if ZAHLFRAGE.search(t):
        return False, "Antwort waere eine Zahl", "", 0.0
    k = t.lower()
    for w in BEKENNTNIS:
        if w in k:
            return False, "Bekenntnis statt Ding (%s)" % w, "", 0.0
    for w in VORAUSSETZUNG:
        m = re.search(r"\b%s\b" % w, k)
        if m:
            return False, "setzt voraus: %s" % m.group(0), "", 0.0
    g = geruest(t)[:RAHMENANFANG]
    if len(g) == RAHMENANFANG:
        for a in pool + angenommen:
            if geruest(a)[:RAHMENANFANG] == g:
                return False, "gleiches Satzgeruest", a, 0.0
    naechste, wert = "", 0.0
    for a in pool + angenommen:
        n, _ = pf.naehe(t, a)
        if n > wert:
            naechste, wert = a, n
    if wert >= pf.DOPPEL:
        return False, "Doppel (%.2f)" % wert, naechste, wert
    return True, "", naechste, wert


# Feste Auswahl und keine gewuerfelte: Der Systemprompt soll zwischen den
# Aufrufen bytegleich bleiben, sonst traegt der Prefix-Cache des Anbieters
# nicht (gemessen: 87 Prozent der Eingabe).
STILPROBEN = [
    "Was hebst du auf, obwohl es kaputt ist?",
    "Welches Geräusch magst du, obwohl die meisten es nicht mögen?",
    "Was liegt bei dir seit Monaten auf dem gleichen Stapel?",
    "Bei welcher Kleinigkeit bist du unerwartet pingelig?",
    "Wer hat dir etwas beigebracht, das du noch täglich benutzt?",
    "Was ziehst du an, wenn es egal ist, wie du aussiehst?",
    "Welchen Raum in einer fremden Wohnung siehst du dir zuerst an?",
    "Was macht dich in Gesellschaft still?",
    "Welche Peinlichkeit fällt dir noch nach Jahren ein?",
    "Was isst du, wenn niemand zuschaut?",
]


def schreiben(rubrik, wieviele, vorhandene):
    nachricht = PROMPT % ("\n".join("- " + a for a in STILPROBEN),
                          rubrik, RUBRIKEN[rubrik],
                          "\n".join("- " + a for a in vorhandene) or "- (noch keine)",
                          wieviele)
    antwort = pf._ruf("Du schreibst deutsche Fragen. Antworte nur mit JSON.",
                      nachricht, rubrik)
    return antwort.get("fragen", [])


def rubrik_fuellen(rubrik, wieviele, alle):
    pool = [f["text"] for f in alle]
    eigene = [f["text"] for f in alle if f["rubrik"] == rubrik]
    angenommen, verworfen = [], []
    # Ueberschuss in Haeppchen: Ein Aufruf ueber dreissig Fragen wird gegen
    # Ende einfallslos, und jeder Haeppchen sieht die vorigen als vergeben.
    versuche = 0
    while len(angenommen) < wieviele and versuche < 8:
        versuche += 1
        fehlt = wieviele - len(angenommen)
        for k in schreiben(rubrik, min(20, fehlt + 6), eigene + angenommen) or []:
            if not isinstance(k, str):
                continue
            ok, grund, naechste, wert = siebe(k, pool, angenommen)
            if ok:
                if len(angenommen) < wieviele:
                    angenommen.append(k.strip())
            else:
                verworfen.append({"frage": k.strip(), "grund": grund,
                                  "naechste": naechste, "wert": round(wert, 2)})
        print("  %s: %d/%d angenommen, %d verworfen (Runde %d)"
              % (rubrik, len(angenommen), wieviele, len(verworfen), versuche))
    return angenommen, verworfen


def main():
    alle = json.load(io.open(POOL, encoding="utf-8"))
    zahl = {}
    for f in alle:
        zahl[f["rubrik"]] = zahl.get(f["rubrik"], 0) + 1

    if "--alle" in sys.argv:
        # Ueberschuss mit Absicht: Der Filter faengt Form und Abstand, aber
        # nicht "beantwortet dieselbe Sache mit anderen Woertern" und nicht
        # "gehoert in eine andere Rubrik". Das liest ein Mensch nach, und dafuer
        # braucht er mehr Zeilen als Plaetze.
        auftrag = [(r, int((ZIEL[r] - zahl.get(r, 0)) * 1.5)) for r in ZIEL
                   if ZIEL[r] - zahl.get(r, 0) > 0]
    elif len(sys.argv) >= 3:
        auftrag = [(sys.argv[1], int(sys.argv[2]))]
    else:
        for r in ZIEL:
            print("  %-14s %3d von %3d, fehlen %d"
                  % (r, zahl.get(r, 0), ZIEL[r], max(0, ZIEL[r] - zahl.get(r, 0))))
        return 0

    ergebnis = {}
    if os.path.exists(AUS):
        ergebnis = json.load(io.open(AUS, encoding="utf-8"))
    for rubrik, n in auftrag:
        schon = [x["frage"] for x in ergebnis.get(rubrik, {}).get("angenommen", [])]
        angenommen, verworfen = rubrik_fuellen(
            rubrik, n - len(schon),
            alle + [{"text": s, "rubrik": rubrik} for s in schon])
        ergebnis[rubrik] = {
            "angenommen": [{"frage": f} for f in schon + angenommen],
            "verworfen": ergebnis.get(rubrik, {}).get("verworfen", []) + verworfen,
        }
        io.open(AUS, "w", encoding="utf-8").write(
            json.dumps(ergebnis, ensure_ascii=False, indent=2) + "\n")
    print("geschrieben nach %s" % AUS)
    return 0


if __name__ == "__main__":
    sys.exit(main())
