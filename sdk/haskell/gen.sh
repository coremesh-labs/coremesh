#!/usr/bin/env bash
# Erzeugt gen/ aus den Protos (nur nötig, wenn sich ein .proto ändert; gen/ ist eingecheckt).
# Braucht protoc und proto-lens-protoc (cabal install proto-lens-protoc).
set -euo pipefail
cd "$(dirname "$0")"
plugin=$(command -v proto-lens-protoc)
rm -rf gen && mkdir gen
protoc --plugin=protoc-gen-haskell="$plugin" --haskell_out=gen \
  -I../../proto plugin/v1/plugin.proto plugin/v1/host.proto
protoc --plugin=protoc-gen-haskell="$plugin" --haskell_out=gen \
  -Iproto goplugin/grpc_broker.proto goplugin/grpc_controller.proto goplugin/grpc_stdio.proto grpc/health/v1/health.proto
find gen -name '*.hs' | sort
