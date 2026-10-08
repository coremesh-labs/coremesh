# CoreMesh Plugin-SDK für Haskell (`coremesh-plugin`)

Haskell-Plugins für den CoreMesh-Host. Ein Plugin ist ein eigener Prozess, den der Host über
[HashiCorp go-plugin](https://github.com/hashicorp/go-plugin) startet – genau wie die Go-Plugins.
Das SDK spricht dessen Protokoll vollständig selbst:

| Teil | Inhalt |
|---|---|
| Handshake | Magic Cookie `COREMESH_PLUGIN`, Protokoll 1, Zeile `1\|1\|tcp\|127.0.0.1:<port>\|grpc\|<Zertifikat>` |
| AutoMTLS | eigenes Ed25519-Zertifikat je Start; vertraut nur dem Host-Zertifikat aus `PLUGIN_CLIENT_CERT` – in beide Richtungen |
| go-plugin-Dienste | Health (`plugin`), GRPCBroker (Verbindungsdaten des Hosts), GRPCController (Shutdown) |
| PluginService | `GetManifest`, `Configure`, `Handle`, `Read` (Datenstrom) |
| HostService (Rückkanal über den Broker) | `Dispatch`, `DispatchRead`, `Query`, `Exec`, `BeginTx`/`CommitTx`/`RollbackTx`, `Log` |
| Lebenszyklus | `DBSchema.Init` (Atlas-HCL aus `pluginSchema`), `Catalog.Describe` (aus `pluginDescribe`) |

gRPC läuft schlank direkt auf `http2` und `tls` (nur einfache Aufrufe, Server-Ströme und der
bidirektionale Broker-Strom); keine C-Bibliotheken außer denen, die GHC selbst braucht. Damit baut
das SDK unter Linux, macOS und Windows gleich.

## Ein Plugin

```haskell
import CoreMesh.Plugin
import Data.Aeson (object, (.=))

main :: IO ()
main = serve (defaultPlugin "hs-hello" "0.1.0")
  { pluginCapabilities =
      [ Capability "HsGreeting" ["say"] [] "Begrüßung"
      , Capability "HsNumbers" ["list"] ["list"] "als Datenstrom"   -- ReadActions
      ]
  , pluginHandle = \call req -> case (reqObject req, reqAction req) of
      ("HsGreeting", "say") -> pure (response (object ["text" .= ("Hallo" :: String)]))
      _ -> pluginError Unimplemented (reqAction req)
  , pluginRead = \call req w -> do
      writeHeader w (ReadHeader ["i"] mempty)
      writeRows w [[Number 1], [Number 2]]
      pure readEnd
  }
```

Vollständiges Beispiel mit allen Aufrufrichtungen: [`example/Main.hs`](example/Main.hs).

- **Payloads** sind Aeson-Werte (`Data.Aeson.Value`) – auf dem Draht `google.protobuf.Value`.
  Zahlen sind `double`; Ganzzahlen über 2^53 als Text übertragen.
- **`Call`** ist der Kontext des laufenden Aufrufs (request_id, Mandant, Benutzer, laufende
  Transaktionen). Jeder Host-Aufruf nimmt ihn mit: `dispatch call "JournalDraft" "create" payload`,
  `query call "main" "SELECT …" [args]`, `inTx call "main" $ \call' -> …`.
- **Fehler** wirft man mit `pluginError NotFound "…"`; auf der Go-Seite kommen sie als
  `sdk.ErrNotFound` usw. an. Fehler des Hosts kommen umgekehrt als `PluginError` mit Code.
- **Datenströme:** `pluginRead` schreibt einen Header, dann Zeilenblöcke; das SDK teilt in
  Blöcke bis 1 MiB / 5000 Zeilen. `dispatchRead` liest einen Strom eines anderen Plugins;
  `writeRows` des Empfängers blockiert, solange er nicht nachkommt (Gegendruck).
- **SQL** (`query`, `exec`) nur auf eigene Tabellen ändernd (Präfix `<plugin>__`, Schema über
  `pluginSchema`); der Host lehnt Änderungen fremder Tabellen, DDL und mehrere Anweisungen ab
  (`PermissionDenied`). Fremde Daten über `dispatch`/`dispatchRead`.
- **Transaktionen:** `inTx` öffnet eine Transaktion (oder nimmt an der laufenden teil). Plugins,
  die man darin per `dispatch` aufruft, schreiben in dieselbe Transaktion (wie `sdk.InTx`).
- **Logs:** `logMessage call LogInfo "…" [("feld", "wert")]` schreibt ins Host-Log.
  `stderrLog` geht ohne Host (vor Configure): JSON-Zeilen im hclog-Format, die go-plugin mit
  ihrer Stufe übernimmt. stdout gehört dem Handshake.

## Bauen und testen

Voraussetzungen: GHC 9.10 und cabal (am einfachsten über [ghcup](https://www.haskell.org/ghcup/),
auch unter Windows und macOS) sowie **`protoc`** im PATH – das Paket mit den
Google-Standardtypen (`proto-lens-protobuf-types`) erzeugt seinen Code beim Bauen damit.

```bash
cabal build          # Bibliothek und Beispiel-Plugin
cabal test           # Werte, Rahmung, gRPC über mTLS (ohne Go-Host)
```

Das Beispiel gegen den echten Host: Binary nach der Namenskonvention des Resolvers ablegen
(`<plugin_dir>/<xx>/<name>-<version>-<os>-<arch>[.exe]`, Go-Namen: `linux`/`darwin`/`windows`,
`amd64`/`arm64`) und eintragen:

```yaml
plugins:
  hs-hello:
    version: 0.1.0
    databases:
      main: { access: read }
```

```bash
console --object HsGreeting --action say --param name=Welt
console --object HsNumbers --action list --read --param n=100000 --out zahlen.csv
console --object HsRelay --action accounts          # liest GLAccount.list als Strom
```

## Generierter Code

`gen/` ist eingecheckt (proto-lens). Neu erzeugen, wenn sich ein `.proto` ändert:
`./gen.sh` (braucht `protoc` und `proto-lens-protoc`, `cabal install proto-lens-protoc`).
Fremde Protos und ihre Lizenzen: [`proto/README.md`](proto/README.md).

## Grenzen

- Keine gRPC-Kompression (go-plugin nutzt keine), kein Broker-Multiplexing
  (`PLUGIN_MULTIPLEX_GRPC`, vom Host nicht eingeschaltet), kein `GRPCStdio` (meldet
  Unimplemented; go-plugin liest stdout/stderr dann direkt).
- Plattformen: unter Linux gegen den Host getestet. Windows und macOS nutzen dieselben Wege
  (TCP-Handshake, Broker über Unix-Socket bzw. unter Windows TCP); dort steht der erste Lauf aus.
