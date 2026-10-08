MODULE      := github.com/coremesh-labs/coremesh
BIN_DIR     := bin
EXT         := $(if $(filter Windows_NT,$(OS)),.exe,)
GOOS        := $(shell go env GOOS)
GOARCH      := $(shell go env GOARCH)

# Quelle der Verträge (öffentlich lesbar) und Ziel des generierten Codes
# (internal/ – für Plugin-Module nicht importierbar).
PROTO_DIR   := proto
PROTO_OUT   := internal/api
PROTO_FILES := $(wildcard $(PROTO_DIR)/plugin/v*/*.proto)
CONSOLE_PROTO := $(wildcard $(PROTO_DIR)/console/v*/*.proto)

.PHONY: all tools proto proto-buf proto-clean build host plugins run test vet tidy clean

all: proto build

## tools: installiert die protoc-Plugins für Go und gRPC
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## proto: proto/plugin/v1/*.proto  -> internal/api/plugin/v1/*.pb.go   (Plugin-Protokoll, intern)
##        proto/console/v1/*.proto -> pkg/consoleapi/console/v1/*.pb.go (Console-API, öffentlich)
proto:
	protoc --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
	protoc --proto_path=$(PROTO_DIR) \
		--go_out=pkg/consoleapi --go_opt=paths=source_relative \
		--go-grpc_out=pkg/consoleapi --go-grpc_opt=paths=source_relative \
		$(CONSOLE_PROTO)

## proto-buf: wie proto, aber ohne installiertes protoc (siehe buf.gen.yaml)
proto-buf:
	go run github.com/bufbuild/buf/cmd/buf@latest generate
	go run github.com/bufbuild/buf/cmd/buf@latest generate --template buf.gen.console.yaml

proto-clean:
	rm -f $(PROTO_OUT)/*/v*/*.pb.go

build: host plugins

host:
	go build -o $(BIN_DIR)/host$(EXT) ./cmd/host
	go build -o $(BIN_DIR)/console$(EXT) ./cmd/console

plugins:
	@# Namenskonvention des Resolvers: <dir>/<xx>/<name>-<version>-<os>-<arch>[.exe]
	go build -o $(BIN_DIR)/plugins/he/hello-0.3.0-$(GOOS)-$(GOARCH)$(EXT) ./examples/plugins/hello
	@# Eigenes Go-Modul: im Modulverzeichnis bauen
	cd cmd/plugins/webserver && go build -o ../../../$(BIN_DIR)/plugins/we/webserver-0.20.0-$(GOOS)-$(GOARCH)$(EXT) .
	cd cmd/plugins/console && go build -o ../../../$(BIN_DIR)/plugins/co/console-0.3.0-$(GOOS)-$(GOARCH)$(EXT) .
	cd cmd/plugins/partner && go build -o ../../../$(BIN_DIR)/plugins/pa/partner-0.8.0-$(GOOS)-$(GOARCH)$(EXT) .
	cd cmd/plugins/tag && go build -o ../../../$(BIN_DIR)/plugins/ta/tag-0.2.0-$(GOOS)-$(GOARCH)$(EXT) .

test:
	go test ./...
	cd cmd/plugins/webserver && go test ./...
	cd cmd/plugins/console && go test ./...
	cd cmd/plugins/partner && go test ./...
	cd cmd/plugins/tag && go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)

## run: Plugins bauen und Host mit configs/ starten
run: plugins
	go run ./cmd/host -config configs
