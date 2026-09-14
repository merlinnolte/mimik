#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sieht den Fragenvorrat auf Doppel durch.

    python3 pruefe-fragen.py            # nur rechnen, ohne Netz
    python3 pruefe-fragen.py --modell   # zusaetzlich das Modell fragen

Das Mass ist dasselbe, das internal/seed/fragen_test.go prueft - dieses
Werkzeug zeigt nur mehr: beide Baender, die duennen Kerne, die Rubrikverteilung.
Wer Fragen schreibt, laeuft hier zwischendurch durch, statt sich am Ende
fuenfzehn gesperrte Paare auf einmal abzuholen.

WICHTIG: Die Rahmenwoerter und die Schwellen werden aus internal/mimik/frage.go
GELESEN, nicht hier wiederholt. harness.py fuehrt schon eine Handkopie von
gerippewoerter, die niemand gegenprueft; eine dritte Kopie einer solchen Liste
waere eine dritte Stelle, an der sie auseinanderlaufen kann. So kann dieses
Werkzeug dem Test nicht widersprechen.

Das Mass findet, was gleiche Woerter benutzt. Es findet NICHT dieselbe Frage in
anderen Worten - "Was wuerdest du an einem Tag machen, an dem du unsichtbar
waerst?" gegen "Was wuerdest du tun, wenn dir niemand zusehen koennte?" liegt
bei 0.00. Dafuer ist --modell da.

--modell laeuft in ZWEI Stufen, und das ist nicht Vorsicht, sondern eine
Messung: Mit nur der ersten Stufe lieferte das Modell am 14.09.2026 fuer 136
Fragen 74 Paare, von denen rund sechzig keine waren ("Welche Erfindung wuerdest
du zuruecknehmen?" gegen "Was wuerdest du deinem juengeren Ich verschweigen?").
Der Grund ist der Auftrag: "Finde die Paare" hat keine Schranke, und ein Modell,
das Paare finden soll, findet Paare. Die zweite Stufe legt jedes Paar EINZELN
vor und fragt nach einer Bedingung ("gibt derselbe Mensch beide Male dieselbe
Antwort?"). Ein Urteil hat eine Schranke, eine Suche nicht - dasselbe Verhaeltnis
wie zwischen einer Regel im Prompt und einer Pruefung im Code.
"""
import io
import json
import os
import re
import sys
import urllib.request

GO = "internal/mimik/frage.go"
POOL = "internal/seed/fragen.json"


def rahmen():
    h = io.open(GO, encoding="utf-8").read()
    i = h.index("var fragerahmen = map[string]bool{")
    return set(re.findall(r'"([^"]+)":\s*true', h[i:h.index("\n}", i)]))


def schwelle(name):
    h = io.open(GO, encoding="utf-8").read()
    m = re.search(r"^\t%s\s*=\s*([0-9.]+)" % name, h, re.M)
    if not m:
        raise SystemExit("Schwelle %s nicht in %s gefunden" % (name, GO))
    return float(m.group(1))


RAHMEN = rahmen()
DOPPEL = schwelle("FrageDoppel")
NACHBAR = schwelle("FrageNachbar")


def kern(text):
    return {w for w in re.findall(r"[\wäöüß]+", text.lower())
            if w not in RAHMEN and len(w) > 2}


def naehe(a, b):
    ka, kb = kern(a), kern(b)
    if not ka or not kb:
        return 0.0, set()
    g = ka & kb
    return len(g) / float(len(ka | kb)), g


def paare(fragen):
    out = []
    for i in range(len(fragen)):
        for j in range(i + 1, len(fragen)):
            n, g = naehe(fragen[i]["text"], fragen[j]["text"])
            if n >= NACHBAR:
                out.append((n, i, j, sorted(g)))
    out.sort(key=lambda x: (-x[0], x[1], x[2]))
    return out


def zeige(fragen):
    ps = paare(fragen)
    gesperrt = [p for p in ps if p[0] >= DOPPEL]
    nachbarn = [p for p in ps if p[0] < DOPPEL]

    def block(n, i, j, g):
        print("  %.2f  %s  [%s]" % (n, fragen[i]["text"], fragen[i]["rubrik"]))
        print("        %s  [%s]" % (fragen[j]["text"], fragen[j]["rubrik"]))
        print("        gemeinsam: %s" % " ".join(g))

    print("%d Fragen, %d Paare gerechnet" % (len(fragen), len(fragen) * (len(fragen) - 1) // 2))
    print()
    print("DOPPEL (>= %.2f) - diese sperrt der Test:" % DOPPEL)
    if not gesperrt:
        print("  keine")
    for p in gesperrt:
        block(*p)
    print()
    print("NACHBARN (%.2f bis %.2f) - ansehen, nicht sperren:" % (NACHBAR, DOPPEL))
    if not nachbarn:
        print("  keine")
    for p in nachbarn:
        block(*p)
    print()
    duenn = [f for f in fragen if len(kern(f["text"])) < 2]
    print("DUENNE KERNE (%d) - kollidieren mit allem, was ihr Wort benutzt:" % len(duenn))
    for f in duenn:
        print("  %-14s %s" % ("{" + ",".join(sorted(kern(f["text"]))) + "}", f["text"]))
    print()
    zahl = {}
    for f in fragen:
        zahl[f["rubrik"]] = zahl.get(f["rubrik"], 0) + 1
    for r in sorted(zahl, key=lambda r: -zahl[r]):
        print("  %-14s %3d" % (r, zahl[r]))
    return len(gesperrt)


# --------------------------------------------------------------- Modell ---

SUCHE = """Du siehst eine Liste numerierter Fragen aus einem Spiel, in dem zwei
Menschen dieselbe Frage beantworten und danach die echte Antwort des anderen
unter Faelschungen heraussuchen. Eine Frage darf einem Menschen nur EINMAL
gestellt werden.

Nenne die Paare, bei denen der Verdacht besteht, dass sie DIESELBE Frage in
anderen Worten stellen - dass derselbe Mensch also beide Male dieselbe Antwort
gaebe. Gleiche Woerter sind kein Kriterium; gleiche Antwort ist es.

Kein Paar ist auch eine Antwort. Erfinde nichts hinzu.

Gib NUR JSON zurueck, ohne Vorrede:
{"paare": [{"a": 3, "b": 17, "grund": "beides fragt nach ..."}]}"""

URTEIL = """Du bekommst numerierte Paare von Fragen aus einem Spiel. Zwei
Menschen, die sich gut kennen, beantworten dieselbe Frage; danach sucht jeder
die echte Antwort des anderen unter drei Faelschungen heraus. Eine Frage darf
einem Menschen nur einmal gestellt werden.

Entscheide fuer JEDES Paar genau eine Sache:
Gaebe dieselbe Person auf beide Fragen DIESELBE Antwort - denselben Gegenstand,
dieselbe Begebenheit?

JA nur dann. Nicht schon, wenn beide Fragen dasselbe Thema haben. Nicht schon,
wenn sie sich aehnlich anhoeren. Nicht schon, wenn sie gut zusammenpassen.

NEIN bei Spiegelbildern ("was hast du geliebt" gegen "was hast du gehasst").
NEIN bei verschiedenen Zeitraeumen ("ein freier Samstag" gegen "ein freies
Jahr" - das eine ist ein Nachmittag, das andere ein Lebensabschnitt).
NEIN bei verschiedenen Gegenstandsklassen ("ein Buch" gegen "ein Lied").
NEIN, wenn die eine Frage nach einer Vorliebe und die andere nach einem
Ereignis fragt.

Im Zweifel NEIN. Ein falsches JA kostet eine brauchbare Frage; ein falsches
NEIN kostet nichts, weil daneben noch ein Mass rechnet.

Gib NUR JSON zurueck, ohne Vorrede:
{"urteile": [{"nr": 1, "gleich": false, "grund": "..."}]}"""


def _ruf(prompt, nachricht, kennzeichen):
    schluessel = os.environ.get("MIMIK_API_KEY")
    if not schluessel:
        raise SystemExit("MIMIK_API_KEY fehlt")
    basis = os.environ.get("MIMIK_BASE_URL", "https://opencode.ai/zen/go/v1")
    modell = os.environ.get("MIMIK_MODEL", "deepseek-v4.1-flash")
    koerper = json.dumps({
        "model": modell,
        "messages": [{"role": "system", "content": prompt},
                     {"role": "user", "content": nachricht}],
        "temperature": 0.2,
        # Kein Denken: gemessen am 14.09.2026 gleiche Qualitaet, ein Zehntel
        # der Ausgabetoken.
        "reasoning_effort": "none",
    }).encode("utf-8")
    kopf = {"Content-Type": "application/json",
            "Authorization": "Bearer " + schluessel,
            # Cloudflare wirft den Standard-User-Agent von urllib mit 1010 weg.
            "User-Agent": "mimik-pruefe-fragen/1"}
    for z in os.environ.get("MIMIK_HEADERS", "x-opencode-session: mimik-fragen").split("\n"):
        if ":" in z:
            k, _, v = z.partition(":")
            kopf[k.strip()] = v.strip()
    anfrage = urllib.request.Request(basis + "/chat/completions", koerper, kopf)
    with urllib.request.urlopen(anfrage, timeout=180) as a:
        roh = json.load(a)
    inhalt = roh["choices"][0]["message"]["content"]
    m = re.search(r"\{.*\}", inhalt, re.S)
    if not m:
        print("  %s: keine JSON-Antwort: %s" % (kennzeichen, inhalt[:200]))
        return {}
    try:
        return json.loads(m.group(0))
    except ValueError as e:
        print("  %s: JSON unlesbar (%s)" % (kennzeichen, e))
        return {}


def suchen(fragen, nummeriert, kennzeichen):
    liste = "\n".join("%d. %s" % (i, fragen[i]["text"]) for i in nummeriert)
    return _ruf(SUCHE, liste, kennzeichen).get("paare", [])


def urteilen(fragen, verdacht, kennzeichen):
    """Zweite Stufe: jedes Paar einzeln, mit einer Bedingung statt eines Auftrags."""
    zeilen = []
    for n, (a, b) in enumerate(verdacht, 1):
        zeilen.append("%d.\n  A: %s\n  B: %s" % (n, fragen[a]["text"], fragen[b]["text"]))
    urteile = _ruf(URTEIL, "\n".join(zeilen), kennzeichen).get("urteile", [])
    ja = {}
    for u in urteile:
        n = u.get("nr")
        if isinstance(n, int) and 1 <= n <= len(verdacht) and u.get("gleich") is True:
            ja[verdacht[n - 1]] = u.get("grund", "")
    return ja


def modellgang(fragen):
    """Rubrikweise suchen, dann ueber alles urteilen.

    Nicht paarweise suchen: 358 Fragen sind 63.903 Paare. Rubrikweise sind es
    acht Aufrufe mit je hoechstens 66 Fragen. Der Querdurchgang ueber alle ist
    nicht optional - das Umweg-Paar im Bestand lag in zwei Rubriken.
    """
    nach = {}
    for i, f in enumerate(fragen):
        nach.setdefault(f["rubrik"], []).append(i)

    verdacht = []
    for kennzeichen, nummern in list(sorted(nach.items())) + [("quer", list(range(len(fragen))))]:
        for p in suchen(fragen, nummern, kennzeichen):
            a, b = p.get("a"), p.get("b")
            if not isinstance(a, int) or not isinstance(b, int):
                continue
            if not (0 <= a < len(fragen) and 0 <= b < len(fragen)) or a == b:
                continue
            schl = tuple(sorted((a, b)))
            if schl not in verdacht:
                verdacht.append(schl)
    print("MODELL, Stufe 1: %d Paare unter Verdacht" % len(verdacht))
    if not verdacht:
        return

    # In Haeppchen, damit ein Stapel nicht an einer abgeschnittenen Antwort
    # scheitert und alle Urteile mitnimmt.
    bestaetigt = {}
    for i in range(0, len(verdacht), 25):
        teil = verdacht[i:i + 25]
        bestaetigt.update(urteilen(fragen, teil, "urteil %d" % (i // 25 + 1)))

    print("MODELL, Stufe 2: %d davon bestaetigt" % len(bestaetigt))
    for (a, b), grund in sorted(bestaetigt.items(),
                                key=lambda x: -naehe(fragen[x[0][0]]["text"],
                                                     fragen[x[0][1]]["text"])[0]):
        n, _ = naehe(fragen[a]["text"], fragen[b]["text"])
        print("  Mass %.2f - %s" % (n, grund))
        print("        %s  [%s]" % (fragen[a]["text"], fragen[a]["rubrik"]))
        print("        %s  [%s]" % (fragen[b]["text"], fragen[b]["rubrik"]))


def main():
    fragen = json.load(io.open(POOL, encoding="utf-8"))
    gesperrt = zeige(fragen)
    if "--modell" in sys.argv:
        print()
        modellgang(fragen)
    return 1 if gesperrt else 0


if __name__ == "__main__":
    sys.exit(main())
