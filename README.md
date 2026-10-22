# CoreMesh

Kern einer modularen Geschäftsanwendung: Host, Plugin-SDK für Go und Haskell (`pkg/sdk`,
`sdk/haskell`), Core-Plugins (IAM, Nummernkreise, Hooks, Sicherung, Katalog …) und die
Plugins Webserver, Konsole, Geschäftspartner, Tags, Dokumente, Skripte und KI.

Fachplugins liegen in eigenen Repositories:
[coremesh-erp](https://github.com/coremesh-labs/coremesh-erp) (Go) und
[coremesh-erph](https://github.com/coremesh-labs/coremesh-erph) (Haskell).

## Einrichten und Aktualisieren: coremesh-setup

Fertige Stände liegen als [GitHub-Release](https://github.com/coremesh-labs/coremesh/releases).
`coremesh-setup-<plattform>` aus dem neuesten Release laden, ausführbar machen und starten:

```sh
chmod +x coremesh-setup-linux-amd64
./coremesh-setup-linux-amd64            # Oberfläche auf http://127.0.0.1:9870
# oder auf der Kommandozeile:
./coremesh-setup-linux-amd64 install --dir=~/coremesh --package=erp-vuv --data --port=8080
```

„mit Testbestand“ (`--data`) richtet die erfundene Hausverwaltung „Faust“ ein (Namen aus
Goethes Faust, Adressen in Musterstadt) und setzt sofort ein eigenes Passwort (`ZUGANG.txt`).
Einzelheiten: [cmd/coremesh-setup/README.md](cmd/coremesh-setup/README.md).


## Lizenz

Copyright © 2026 die CoreMesh-Autoren. Lizenziert unter der **European Union Public Licence
v. 1.2** (EUPL-1.2, Text in `LICENSE`; amtliche Fassungen in allen EU-Sprachen, auch Deutsch:
https://eur-lex.europa.eu/eli/dec_impl/2017/863/oj). Bibliotheken und eingebettete Programme
anderer Autoren mit ihren Lizenzen: `THIRD_PARTY_NOTICES.md` (erzeugt mit
`coremesh/tools/third_party_notices.py`; vor jedem Release neu erzeugen und mit den Programmen
ausliefern).
