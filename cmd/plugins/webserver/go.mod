// Eigenes Modul: Der WebServer nutzt ausschließlich die öffentliche API
// (pkg/sdk/...). Der Go-Compiler verhindert jeden Import von internal/.
// Für die Auslagerung in ein eigenes Repository genügt es, die replace-Zeile
// durch eine Versionsangabe zu ersetzen.
module github.com/coremesh-lab/coremesh/cmd/plugins/webserver

go 1.27.1

require (
	github.com/coremesh-lab/coremesh v0.0.0
	golang.org/x/crypto v0.57.0
)

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-plugin v1.8.0 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/oklog/run v1.1.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/grpc v1.84.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/coremesh-lab/coremesh => ../../..
