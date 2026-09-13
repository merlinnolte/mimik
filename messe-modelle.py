#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
MIMIK · Modelle messen
==============================

Welches Modell antwortet schnell genug, um eine Runde nicht zur Geduldsprobe zu
machen? Diese Frage beantwortet keine Produktseite, sondern nur der Endpunkt,
den du wirklich benutzt - an dem Prompt, den das Spiel wirklich schickt.

Das Skript listet die Modelle deines Endpunkts, schickt jedem denselben echten
Prompt B und misst. Gewertet wird nicht nur die Zeit: Ein Modell, das in zwei
Sekunden etwas Unbrauchbares liefert, hilft nicht.

    export MIMIK_BASE_URL="https://opencode.ai/zen/go/v1"
    export MIMIK_API_KEY="..."
    export MIMIK_HEADERS="x-opencode-session: mimik-{zufall}"
    python3 messe-modelle.py --liste         # nur auflisten, nichts messen
    python3 messe-modelle.py                 # alle gelisteten Modelle, je 1 Lauf
    python3 messe-modelle.py -n 3            # je 3 Laeufe, Median
    python3 messe-modelle.py modell-a modell-b

Ohne Abhaengigkeiten ausser Python 3.
"""
import json
import os
import re
import statistics
import sys
import time
import urllib.error
import urllib.request
import uuid

BASIS = os.environ.get("MIMIK_BASE_URL", "https://api.deepseek.com").rstrip("/")
KEY = os.environ.get("MIMIK_API_KEY", "")
TIMEOUT = float(os.environ.get("MIMIK_TIMEOUT", "300"))

# Dieselbe Frage, dieselbe Antwort, dasselbe Dossier wie im Betrieb - sonst misst
# man etwas anderes als das, was das Spiel tut.
MATERIAL = """<material>
[frage]
Was ist das Unvernünftigste, das du dir in den letzten zwei Jahren gekauft hast?

[echte_antwort_roh]
eine zweite kaffeemuehle, aber die erste mahlt zu grob

[interessen]
einkaufen, handarbeit, kaffee, kartenspiele, kindheit, kochen, nachrichten,
pflanzen, wohnen, zugfahren

[dossier · fakten]
- Kocht regelmäßig und unterscheidet dabei zwischen Aufwärmen und richtigem Kochen.
- Liest abends Nachrichten, obwohl das den Schlaf verschlechtert.
- Besitzt eine Nähmaschine, die nie benutzt wird.
- Sammelt Zeitschriften, die neben dem Sofa liegen bleiben.
- Erinnert sich an Regenwürmer in der Jackentasche aus der Kindheit.

[dossier · verbrauchte themen]
kochen, nachrichten, naehmaschine, zeitschriften, kindheit

[anti-beispiele]
- Das ist eine spannende Frage! Ich würde sagen ...
- Am Ende zählt doch, dass man glücklich ist.
</material>"""


def kopfzeilen():
    """MIMIK_HEADERS wie im Server: 'Name: Wert' je Zeile, {zufall} je Aufruf neu."""
    aus = {}
    for zeile in os.environ.get("MIMIK_HEADERS", "").splitlines():
        if ":" in zeile:
            k, v = zeile.split(":", 1)
            aus[k.strip()] = v.strip().replace("{zufall}", uuid.uuid4().hex[:16])
    return aus


def ruf(pfad, koerper=None, methode=None):
    req = urllib.request.Request(
        BASIS + pfad,
        data=json.dumps(koerper).encode("utf-8") if koerper is not None else None,
        method=methode or ("POST" if koerper is not None else "GET"))
    req.add_header("Authorization", "Bearer " + KEY)
    req.add_header("Content-Type", "application/json")
    # Der Standard-User-Agent von urllib wird von Cloudflare mit 1010 abgewiesen.
    req.add_header("User-Agent", "mimik-messung/1.0")
    for k, v in kopfzeilen().items():
        req.add_header(k, v)
    with urllib.request.urlopen(req, timeout=TIMEOUT) as r:
        return json.loads(r.read().decode("utf-8"))


def prompt_b():
    go = open("internal/mimik/prompts.go", encoding="utf-8").read()
    m = re.search(r"^const PromptFaelschungen = `(.*?)`", go, re.S | re.M)
    if not m:
        sys.exit("PromptFaelschungen nicht gefunden - vom Projektverzeichnis aus starten.")
    return m.group(1)


def modelle(roh=False):
    """Die Modelle des Endpunkts. OpenAI-kompatible APIs listen sie unter /models."""
    try:
        d = ruf("/models")
    except Exception as e:
        print("Modelliste nicht abrufbar (%s) - dann bitte Namen als Argumente." % e)
        return []
    xs = d.get("data", d if isinstance(d, list) else [])
    if roh:
        return sorted(xs, key=lambda x: x.get("id", ""))
    return sorted(x.get("id", "") for x in xs if x.get("id"))


def brauchbar(inhalt):
    """Hat die Antwort die Felder, die das Spiel braucht?"""
    s = inhalt.strip()
    if s.startswith("```"):
        s = s.split("\n", 1)[-1].rsplit("```", 1)[0]
    i, j = s.find("{"), s.rfind("}")
    if i < 0 or j <= i:
        return False, "kein JSON"
    try:
        d = json.loads(s[i:j + 1])
    except Exception:
        return False, "JSON kaputt"
    for feld in ("normalform", "fakt", "sperre", "antworten"):
        if feld not in d:
            return False, "Feld %s fehlt" % feld
    if len(d.get("antworten") or []) < 3:
        return False, "weniger als 3 Antworten"
    return True, "ok"


def messe(modell, system, laeufe):
    zeiten, befund = [], "—"
    for _ in range(laeufe):
        begonnen = time.time()
        try:
            d = ruf("/chat/completions", {
                "model": modell, "temperature": 1.0,
                "messages": [{"role": "system", "content": system},
                             {"role": "user", "content": MATERIAL}]})
        except urllib.error.HTTPError as e:
            return None, "HTTP %d" % e.code
        except Exception as e:
            return None, type(e).__name__
        zeiten.append(time.time() - begonnen)
        inhalt = (d.get("choices") or [{}])[0].get("message", {}).get("content", "")
        ok, befund = brauchbar(inhalt)
        if not ok:
            return statistics.median(zeiten), befund
    return statistics.median(zeiten), befund


def main():
    if not KEY:
        sys.exit("MIMIK_API_KEY fehlt.")
    args = [a for a in sys.argv[1:] if not a.startswith("-")]
    laeufe = 1
    if "-n" in sys.argv:
        laeufe = int(sys.argv[sys.argv.index("-n") + 1])

    # Nur auflisten: Was gibt es ueberhaupt, und was steht sonst noch dabei?
    if "--liste" in sys.argv:
        xs = modelle(roh=True)
        if not xs:
            sys.exit(1)
        print("%d Modelle an %s\n" % (len(xs), BASIS))
        for x in xs:
            extra = {k: v for k, v in x.items()
                     if k not in ("id", "object") and v not in (None, "", [], {})}
            print("  %s" % x.get("id", "?"))
            if extra:
                print("      " + json.dumps(extra, ensure_ascii=False)[:160])
        return

    system = prompt_b()
    namen = args or modelle()
    if not namen:
        sys.exit("Keine Modelle zu messen.")

    print("Endpunkt : %s" % BASIS)
    print("Prompt   : %d Zeichen System + %d Zeichen Material" % (len(system), len(MATERIAL)))
    print("Laeufe   : %d je Modell\n" % laeufe)
    print("%-38s %10s  %s" % ("Modell", "Median", "Ergebnis"))
    print("-" * 70)

    ergebnisse = []
    for name in namen:
        t, befund = messe(name, system, laeufe)
        ergebnisse.append((t if t is not None else float("inf"), name, t, befund))
        print("%-38s %10s  %s" % (
            name, "%.1fs" % t if t is not None else "—", befund))
        sys.stdout.flush()

    brauchbare = [(t, n) for _, n, t, b in ergebnisse if t is not None and b == "ok"]
    if brauchbare:
        brauchbare.sort()
        print("\nSchnellstes brauchbares Modell: %s (%.1fs)" % (brauchbare[0][1], brauchbare[0][0]))
        print("Eintragen als MIMIK_MODEL in der .env, dann docker compose up -d.")
    else:
        print("\nKein Modell lieferte eine brauchbare Antwort.")


if __name__ == "__main__":
    main()
