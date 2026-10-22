#!/usr/bin/env python3
"""Lizenzhinweise der Drittbibliotheken eines Repositories (THIRD_PARTY_NOTICES.md).

    tools/third_party_notices.py <repo> [--extra datei.md] [--check]

Go: alle Module, die die Pakete des Repositories einbinden (go list -deps über jedes
go.mod, ohne Tests und ohne coremesh-labs), mit den Lizenzdateien aus dem Modul-Cache.
Haskell: alle Pakete aus dist-newstyle/cache/plan.json (vorher cabal build), Lizenz und
Lizenzdatei über ghc-pkg. --extra hängt eine Datei mit Hinweisen zu eingebetteten
Programmen an (z. B. Tectonic). --check: Abbruch, wenn eine Lizenz fehlt, unbekannt ist
oder nicht zur EUPL-1.2 des Projekts passt (erlaubt: siehe ALLOWED).
"""
import json
import os
import re
import subprocess
import sys

ALLOWED = {"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "ISC", "MPL-2.0", "Zlib", "Unicode-DFS-2016",
           "PublicDomain", "LGPL-dynamic"}
LICENSE_FILE = re.compile(r"(?i)^(licen[cs]e|copying|notice|patents)([-._].*)?$")


def classify(text):
    t = text.lower()
    if "mozilla public license" in t:
        return "MPL-2.0"
    if "gnu affero" in t:
        return "AGPL"
    if "gnu lesser" in t:
        return "LGPL"
    if "gnu general public license" in t:
        return "GPL"
    if "apache license" in t:
        return "Apache-2.0"
    if "permission is hereby granted, free of charge" in t:
        return "MIT"
    if re.search(r"redistribution and use (of this software )?in source and binary forms", t):
        return "BSD-3-Clause" if ("neither the name" in t or "endorse or promote" in t) else "BSD-2-Clause"
    if "permission to use, copy, modify, and/or distribute" in t or "isc license" in t:
        return "ISC"
    if "public domain" in t or "unlicense" in t:
        return "PublicDomain"
    return "?"


def go_modules(repo):
    mods = {}
    for root, dirs, files in os.walk(repo):
        dirs[:] = [d for d in dirs if d not in (".git", "node_modules", "dist-newstyle", "bin")]
        if "go.mod" not in files:
            continue
        out = subprocess.run(["go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}", "./..."],
                             cwd=root, capture_output=True, text=True, check=True).stdout
        for line in out.splitlines():
            parts = line.split("\t")
            if len(parts) == 3 and parts[1] and not parts[0].startswith("github.com/coremesh-labs/"):
                mods[parts[0]] = (parts[1], parts[2])
    return [(p, v, d) for p, (v, d) in sorted(mods.items())]


def license_files(d):
    out = []
    for f in sorted(os.listdir(d)):
        p = os.path.join(d, f)
        if os.path.isfile(p) and LICENSE_FILE.match(f):
            out.append((f, open(p, encoding="utf-8", errors="replace").read()))
    return out


def haskell_packages(repo):
    plan = os.path.join(repo, "dist-newstyle", "cache", "plan.json")
    if not os.path.exists(plan):
        return []
    env = dict(os.environ, PATH=os.path.expanduser("~/.ghcup/bin") + os.pathsep + os.environ.get("PATH", ""))
    dbs = []
    for base in ("~/.local/state/cabal/store", "~/.cabal/store"):
        b = os.path.expanduser(base)
        if os.path.isdir(b):
            dbs += [os.path.join(b, g, "package.db") for g in os.listdir(b) if g.startswith("ghc-")]
    # Nur was in die ausgelieferten Programme gelinkt wird: Hülle der Abhängigkeiten der
    # exe-Komponenten (ohne Beispiele *-hello, ohne Tests und Build-Werkzeuge).
    units = {u["id"]: u for u in json.load(open(plan))["install-plan"]}
    todo = [i for i, u in units.items() if u.get("style") == "local" and str(u.get("component-name", "")).startswith("exe:")
            and not u["component-name"].endswith("-hello")]
    seen = set()
    while todo:
        i = todo.pop()
        if i in seen or i not in units:
            continue
        seen.add(i)
        u = units[i]
        deps = list(u.get("depends", []))
        for c in u.get("components", {}).values():
            deps += c.get("depends", [])
        todo += deps
    pkgs = {(units[i]["pkg-name"], units[i]["pkg-version"]): True for i in seen if units[i].get("style") != "local"}
    pkgs[("rts", "1.0.2")] = True  # Laufzeitsystem von GHC (Lizenz wie ghc)
    out = []
    for name, ver in sorted(pkgs):
        lic, text = "?", ""
        for db in [None] + dbs:
            cmd = ["ghc-pkg"] + (["--package-db", db] if db else []) + ["field", f"{name}-{ver}", "license,license-files,data-dir", "--simple-output"]
            r = subprocess.run(cmd, capture_output=True, text=True, env=env)
            if r.returncode == 0:
                lines = r.stdout.splitlines()
                lic = lines[0].strip() if lines else "?"
                break
        out.append((name, ver, lic, haskell_license_text(name, ver, dbs)))
    return out


def haskell_license_text(name, ver, dbs):
    """Lizenzdatei aus dem cabal-Store (share/doc) bzw. der GHC-Installation (Bootpakete)."""
    cands = []
    for db in dbs:
        store = os.path.dirname(db)
        cands += [os.path.join(store, d, "share", "doc") for d in os.listdir(store) if d.startswith(f"{name}-{ver}-")]
    if name == "rts":
        name, ver = "ghc", ""
    ghcdoc = os.path.expanduser("~/.ghcup/ghc")
    if os.path.isdir(ghcdoc):
        for root, dirs, _ in os.walk(ghcdoc):
            base = os.path.basename(root)
            if "doc" in root and (base == f"{name}-{ver}" or (not ver and re.fullmatch(rf"{name}-[0-9.]+", base))):
                cands.append(root)
    for c in cands:
        if os.path.isdir(c):
            for f, t in license_files(c):
                return t
    return ""


def main():
    args = sys.argv[1:]
    check = "--check" in args
    extra = args[args.index("--extra") + 1] if "--extra" in args else None
    repo = os.path.abspath([a for a in args if not a.startswith("--") and a != extra][0])
    rows, texts, problems = [], [], []
    for path, ver, d in go_modules(repo):
        files = license_files(d)
        kinds = sorted({classify(t) for f, t in files if not f.upper().startswith(("NOTICE", "PATENTS"))}) or ["KEINE"]
        main_kind = kinds[0] if len(kinds) == 1 else "/".join(kinds)
        rows.append((path, ver, main_kind))
        if not any(k in ALLOWED for k in kinds):
            problems.append(f"Go {path} {ver}: {main_kind}")
        for f, t in files:
            texts.append((f"{path} {ver} – {f}", t))
    for name, ver, lic, text in haskell_packages(repo):
        if lic == "?" and text:
            lic = {"BSD-3-Clause": "BSD-3-Clause", "BSD-2-Clause": "BSD-2-Clause"}.get(classify(text), classify(text))
        rows.append((name, ver, lic))
        if lic not in ALLOWED or not text:
            problems.append(f"Haskell {name} {ver}: {lic}{'' if text else ', Lizenztext fehlt'}")
        texts.append((f"{name} {ver} – {lic}", text or f"Text im Quellpaket: https://hackage.haskell.org/package/{name}-{ver}"))
    md = ["# Drittbibliotheken", "",
          "CoreMesh steht unter der EUPL-1.2 (siehe LICENSE). Die Programme enthalten die folgenden",
          "Bibliotheken anderer Autoren unter deren Lizenzen. Erzeugt mit tools/third_party_notices.py.", "",
          "| Bibliothek | Version | Lizenz |", "|---|---|---|"]
    md += [f"| {p} | {v} | {k} |" for p, v, k in rows]
    if extra:
        md += ["", open(extra, encoding="utf-8").read().strip()]
    md += ["", "## Lizenztexte", ""]
    for title, t in texts:
        md += [f"### {title}", "", "```", t.strip(), "```", ""]
    open(os.path.join(repo, "THIRD_PARTY_NOTICES.md"), "w", encoding="utf-8").write("\n".join(md) + "\n")
    print(f"{len(rows)} Bibliotheken, {len(problems)} Auffälligkeiten")
    for p in problems:
        print("  ", p)
    if check and problems:
        sys.exit(1)


if __name__ == "__main__":
    main()
