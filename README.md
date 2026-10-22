# CoreMesh

Kern einer modularen Geschäftsanwendung: Host, Plugin-SDK für Go und Haskell (`pkg/sdk`,
`sdk/haskell`), Core-Plugins (IAM, Nummernkreise, Hooks, Sicherung, Katalog …) und die
Plugins Webserver, Konsole, Geschäftspartner, Tags, Dokumente, Skripte und KI.

Fachplugins liegen in eigenen Repositories:
[coremesh-erp](https://github.com/coremesh-labs/coremesh-erp) (Go) und
[coremesh-erph](https://github.com/coremesh-labs/coremesh-erph) (Haskell).


## Lizenz

Copyright © 2026 die CoreMesh-Autoren. Lizenziert unter der **European Union Public Licence
v. 1.2** (EUPL-1.2, Text in `LICENSE`; amtliche Fassungen in allen EU-Sprachen, auch Deutsch:
https://eur-lex.europa.eu/eli/dec_impl/2017/863/oj). Bibliotheken und eingebettete Programme
anderer Autoren mit ihren Lizenzen: `THIRD_PARTY_NOTICES.md` (erzeugt mit
`coremesh/tools/third_party_notices.py`; vor jedem Release neu erzeugen und mit den Programmen
ausliefern).
