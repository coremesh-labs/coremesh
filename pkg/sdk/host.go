package sdk

import "context"

// LogLevel entspricht pluginv1.LogLevel.
type LogLevel int32

const (
	LogDebug LogLevel = 1
	LogInfo  LogLevel = 2
	LogWarn  LogLevel = 3
	LogError LogLevel = 4
)

// Host sind die Dienste, die der Host einem Plugin anbietet. Im Plugin kommt
// die Implementierung über Config.Host (gRPC-Rückkanal); auf Host-Seite
// implementiert internal/ das Interface.
//
// Immer den ctx des laufenden Aufrufs weiterreichen: Darüber erhält der Host
// Korrelations-ID, Mandant und Benutzer sowie Deadline und Abbruch.
type Host interface {
	// Handle ruft ein anderes Plugin auf: Der Dispatcher des Hosts leitet die
	// Anfrage an das für (req.Object, req.Action) zuständige Plugin weiter.
	// Kein zuständiges Plugin: ErrUnimplemented. Zu tiefe Aufrufkette
	// (z. B. Zyklus A -> B -> A): ErrFailedPrecondition.
	Handler
	// Read ruft ein anderes Plugin als Datenstrom auf (sdk.Reader): gleiche
	// Weiterleitung und Prüfung wie Handle, der Strom des Ziel-Plugins geht an w.
	// Ziel-Action nicht in dessen ReadActions: ErrUnimplemented.
	Reader

	Log(ctx context.Context, level LogLevel, msg string, fields map[string]string) error
	// Query führt ein SELECT auf dem logischen Pool database aus.
	// Werte immer über args übergeben, nie per String-Verkettung.
	Query(ctx context.Context, database, sql string, args ...any) (*QueryResult, error)
	// Exec führt INSERT/UPDATE/DELETE auf dem logischen Pool database aus.
	Exec(ctx context.Context, database, sql string, args ...any) (ExecResult, error)
}

// QueryResult enthält die Spaltennamen und Zeilen eines SELECT.
// Zahlen kommen als float64 an (google.protobuf.Value); Ganzzahlen über 2^53
// sollte das SQL daher als Text liefern.
type QueryResult struct {
	Columns []string
	Rows    [][]any
}

// ExecResult ist das Ergebnis eines INSERT/UPDATE/DELETE.
type ExecResult struct {
	RowsAffected int64
	LastInsertID int64
}
