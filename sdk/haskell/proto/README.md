# Fremde Protos

Neben den eigenen Verträgen (`../../../proto/plugin/v1`) spricht ein Plugin die Dienste,
die HashiCorp go-plugin auf jedem Plugin-Prozess erwartet:

| Datei | Herkunft | Lizenz |
|---|---|---|
| `goplugin/grpc_broker.proto`, `grpc_controller.proto`, `grpc_stdio.proto` | `github.com/hashicorp/go-plugin` v1.8.0, `internal/plugin/` (unverändert) | MPL-2.0 |
| `grpc/health/v1/health.proto` | gRPC Health Checking Protocol (gekürzt auf `Check`) | Apache-2.0 |

Neu erzeugen: `./gen.sh` (braucht `protoc` und `proto-lens-protoc` im PATH).
