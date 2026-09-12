# -*- coding: utf-8 -*-
"""Laeuft alle Runden, schreibt nach JEDER Runde raus und protokolliert fortlaufend."""
import json, io, sys, time, statistics, harness as H

profil = json.load(io.open(sys.argv[1], encoding="utf-8"))
profil.setdefault("gesperrte_themen", []); profil.setdefault("dossier_fakten", [])
ziel = sys.argv[2]
ergebnisse = []

def sichern():
    io.open(ziel, "w", encoding="utf-8").write(
        json.dumps(ergebnisse, ensure_ascii=False, indent=2) + "\n")

for i, r in enumerate(profil["runden"], 1):
    norm = r["antwort_roh"]
    anker, gestr = H.anker_waehlen(profil["tags"], profil["gesperrte_themen"], norm)
    t0, out, fehler = time.time(), None, None
    for versuch in range(2):
        try:
            out = H.faelschungen(r["frage"], norm, anker, profil); break
        except H.Zeitueberschreitung:
            fehler = "Zeitueberschreitung"
            print("R%-2d Versuch %d: Zeitueberschreitung" % (i, versuch + 1), flush=True)
        except SystemExit as e:
            fehler = str(e)[:120]; break
        except Exception as e:
            fehler = "%s: %s" % (type(e).__name__, e); break
    dauer = time.time() - t0
    if not out:
        print("R%-2d FEHLER nach %.0fs: %s" % (i, dauer, fehler), flush=True)
        ergebnisse.append({"runde": i, "fehler": fehler}); sichern(); continue

    fakes = [a["text"].strip() for a in out.get("antworten", [])][:3]
    mess = H.abstandsfenster(norm, fakes) if len(fakes) == 3 else None
    bruch = sorted({k for k, f in enumerate(fakes)
                    for t in out.get("sperre", []) if H.tag_naehe(t, f) >= 0.60})
    ln = [len(norm)] + [len(f) for f in fakes]
    ergebnisse.append({"runde": i, "frage": r["frage"], "echt": norm, "fakes": fakes,
                       "fakt": out.get("fakt"), "sperre": out.get("sperre", []),
                       "gestrichen": gestr, "anker": anker, "bruch": bruch,
                       "laengen": ln, "dauer": round(dauer, 1),
                       "mess": {k: mess[k] for k in ("max_echt", "mean_peers", "naehe_ok", "streuung_ok")} if mess else None})
    sichern()
    profil["dossier_fakten"].append(out.get("fakt"))
    profil["gesperrte_themen"].extend(s for s in out.get("sperre", []) if s not in profil["gesperrte_themen"])
    print("R%-2d %4.0fs  echt=%3d  fakes=%-16s %s%s" % (
        i, dauer, ln[0], str(ln[1:]),
        "ECHT-KUERZESTE " if ln[0] == min(ln) else "",
        "" if not bruch else "SPERRBRUCH%s" % bruch), flush=True)

ok = [e for e in ergebnisse if "laengen" in e]
print("\n" + "=" * 60, flush=True)
print("ausgewertet             : %d von %d" % (len(ok), len(profil["runden"])), flush=True)
if ok:
    kuerz = sum(1 for e in ok if e["laengen"][0] == min(e["laengen"]))
    verh = statistics.mean(statistics.mean(e["laengen"][1:]) / e["laengen"][0] for e in ok)
    print("echte ist die kuerzeste : %d von %d  (Zufall waere %.1f)" % (kuerz, len(ok), len(ok) / 4.0), flush=True)
    print("faelschung/echt         : %.2fx" % verh, flush=True)
    print("naehe verletzt          : %d" % sum(1 for e in ok if e["mess"] and not e["mess"]["naehe_ok"]), flush=True)
    print("streuung verletzt       : %d" % sum(1 for e in ok if e["mess"] and not e["mess"]["streuung_ok"]), flush=True)
    print("sperrbruch              : %d" % sum(1 for e in ok if e["bruch"]), flush=True)
