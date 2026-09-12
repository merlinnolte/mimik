#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Haelt Prompts und Umlautliste in harness.py und internal/mimik deckungsgleich.

Die Werkbank soll genau das testen, was das Spiel spielt. Driften die beiden
Texte auseinander, ist jeder Befund der Werkbank wertlos - und das faellt sonst
niemandem auf. Laeuft ohne Abhaengigkeiten:  python3 pruefe-prompts.py
"""
import difflib
import io
import re
import sys

PAARE = [
    ("PROMPT_B", "PromptFaelschungen"),
    ("PROMPT_D", "PromptNormalform"),
]


def aus_python(name):
    h = io.open("harness.py", encoding="utf-8").read()
    m = re.search(r'^%s = """(.*?)"""' % name, h, re.S | re.M)
    return m.group(1).strip() if m else None


def aus_go(name):
    h = io.open("internal/mimik/prompts.go", encoding="utf-8").read()
    m = re.search(r"^const %s = `(.*?)`" % name, h, re.S | re.M)
    return m.group(1).strip() if m else None


def umlautliste_go():
    h = io.open("internal/mimik/umlaute.go", encoding="utf-8").read()
    i = h.index("var umlautWoerter = map[string]string{")
    return dict(re.findall(r'"([^"]+)":\s*"([^"]+)"', h[i:h.index("\n}", i)]))


def umlautliste_py():
    h = io.open("harness.py", encoding="utf-8").read()
    i = h.index("UMLAUT_WOERTER = {")
    return dict(re.findall(r'"([^"]+)": "([^"]+)"', h[i:h.index("\n}", i)]))


def pruefe_umlaute():
    a, b = umlautliste_go(), umlautliste_py()
    if a == b:
        print("gleich : Umlautliste (%d Woerter)" % len(a))
        return 0
    print("DRIFT  : Umlautliste")
    for k in sorted(set(a) ^ set(b)):
        print("  nur in %s: %s" % ("go" if k in a else "py", k))
    for k in sorted(set(a) & set(b)):
        if a[k] != b[k]:
            print("  %s: go=%s py=%s" % (k, a[k], b[k]))
    return 1


def main():
    fehler = 0
    for py, go in PAARE:
        a, b = aus_python(py), aus_go(go)
        if a is None or b is None:
            print("FEHLT: %s=%s  %s=%s" % (py, a is not None, go, b is not None))
            fehler += 1
            continue
        if a == b:
            print("gleich : %-10s == %s" % (py, go))
            continue
        fehler += 1
        print("DRIFT  : %s != %s" % (py, go))
        for zeile in list(difflib.unified_diff(
                a.split("\n"), b.split("\n"), py, go, lineterm=""))[:20]:
            print("  " + zeile)
    fehler += pruefe_umlaute()
    return 1 if fehler else 0


if __name__ == "__main__":
    sys.exit(main())
