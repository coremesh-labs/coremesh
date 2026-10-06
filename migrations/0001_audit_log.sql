-- Host-eigene Tabellen tragen das Präfix des reservierten Moduls "coremesh".
CREATE TABLE coremesh__audit_log (
    id         INTEGER PRIMARY KEY,
    request_id VARCHAR(64),
    message    TEXT NOT NULL,
    created_at VARCHAR(40) NOT NULL
);
