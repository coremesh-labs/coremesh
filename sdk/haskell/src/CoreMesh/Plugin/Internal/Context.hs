-- | Typen, die Plugin und SDK teilen: Aufrufkontext, Anfrage, Antwort,
-- Fehler und Datenströme. Öffentlich über "CoreMesh.Plugin".
module CoreMesh.Plugin.Internal.Context
  ( -- * Aufrufkontext
    Call (..)
  , HostConn (..)
    -- * Anfrage und Antwort
  , Request (..)
  , Response (..)
  , response
    -- * Fehler
  , PluginError (..)
  , ErrorCode (..)
  , pluginError
  , errorCodeFrom
  , errorCodeTo
    -- * Datenströme
  , RowWriter (..)
  , ReadHeader (..)
  , ReadEnd (..)
  , readEnd
  ) where

import Control.Exception (Exception, throwIO)
import Data.Aeson qualified as J
import Data.ByteString (ByteString)
import Data.Int (Int64)
import Data.Map.Strict (Map)
import Data.Text (Text)
import Data.Text qualified as T

import CoreMesh.Plugin.Internal.Grpc qualified as G

-- | Kontext eines Aufrufs. Mandant und Benutzer setzt ausschließlich der Host;
-- das Plugin leitet sie nie aus dem Payload ab. Jeder Aufruf an den Host
-- ('CoreMesh.Plugin.dispatch', 'CoreMesh.Plugin.query' …) nimmt den Call mit,
-- damit Korrelation, Rechte und laufende Transaktionen erhalten bleiben.
data Call = Call
  { callRequestId :: Text
  , callTenantId :: Text
  , callUserId :: Text
  , callMetadata :: Map Text Text
  , callTxIds :: Map Text Text
  -- ^ laufende Transaktionen der Aufrufkette: Datenbank → tx_id
  , callHost :: Maybe HostConn
  -- ^ Rückkanal zum Host (ab Configure)
  }

-- | Rückkanal zum HostService (intern; Aufrufe über "CoreMesh.Plugin").
newtype HostConn = HostConn
  { hostCall :: ByteString -> ByteString -> (ByteString -> IO ()) -> IO ()
  -- ^ Methode, Request-Bytes, je Antwortnachricht
  }

-- | Vom Dispatcher weitergeleitete Aktion auf einem Business-Object.
data Request = Request
  { reqObject :: Text
  , reqAction :: Text
  , reqPayload :: J.Value
  }
  deriving stock (Show)

-- | Antwort des Plugins; Metadaten ergänzen die Kopfzeilen des Aufrufs.
data Response = Response
  { respPayload :: J.Value
  , respMetadata :: Map Text Text
  }
  deriving stock (Show)

response :: J.ToJSON a => a -> Response
response a = Response (J.toJSON a) mempty

-- | Fehlerarten wie die sdk.Err*-Sentinels der Go-Seite (gRPC-Status).
data ErrorCode
  = InvalidArgument
  | NotFound
  | AlreadyExists
  | PermissionDenied
  | Unimplemented
  | Unavailable
  | FailedPrecondition
  | TxAborted
  | ResourceExhausted
  | Canceled
  | DeadlineExceeded
  | Internal
  deriving stock (Eq, Show)

-- | Fehler eines Plugins oder eines Host-Aufrufs. Auf der Go-Seite kommt er als
-- passender sdk.Err*-Sentinel an, z. B. @errors.Is(err, sdk.ErrNotFound)@.
data PluginError = PluginError ErrorCode Text
  deriving stock (Show)

instance Exception PluginError

pluginError :: ErrorCode -> Text -> IO a
pluginError c m = throwIO (PluginError c m)

errorCodeTo :: ErrorCode -> G.Code
errorCodeTo = \case
  InvalidArgument -> G.InvalidArgument
  NotFound -> G.NotFound
  AlreadyExists -> G.AlreadyExists
  PermissionDenied -> G.PermissionDenied
  Unimplemented -> G.Unimplemented
  Unavailable -> G.Unavailable
  FailedPrecondition -> G.FailedPrecondition
  TxAborted -> G.Aborted
  ResourceExhausted -> G.ResourceExhausted
  Canceled -> G.Canceled
  DeadlineExceeded -> G.DeadlineExceeded
  Internal -> G.Internal

errorCodeFrom :: G.Code -> ErrorCode
errorCodeFrom = \case
  G.InvalidArgument -> InvalidArgument
  G.OutOfRange -> InvalidArgument
  G.NotFound -> NotFound
  G.AlreadyExists -> AlreadyExists
  G.PermissionDenied -> PermissionDenied
  G.Unauthenticated -> PermissionDenied
  G.Unimplemented -> Unimplemented
  G.Unavailable -> Unavailable
  G.FailedPrecondition -> FailedPrecondition
  G.Aborted -> TxAborted
  G.ResourceExhausted -> ResourceExhausted
  G.Canceled -> Canceled
  G.DeadlineExceeded -> DeadlineExceeded
  _ -> Internal

-- | Ziel eines Datenstroms (sdk.RowWriter): genau ein Header, dann Blöcke.
-- 'writeRows' blockiert, solange die Gegenseite nicht nachkommt.
data RowWriter = RowWriter
  { writeHeader :: ReadHeader -> IO ()
  , writeRows :: [[J.Value]] -> IO ()
  }

data ReadHeader = ReadHeader
  { headerColumns :: [Text]
  , headerMetadata :: Map Text Text
  }
  deriving stock (Show)

data ReadEnd = ReadEnd
  { endRows :: Int64
  -- ^ 0 = vom SDK gezählt
  , endCursor :: Text
  , endMetadata :: Map Text Text
  }
  deriving stock (Show)

readEnd :: ReadEnd
readEnd = ReadEnd 0 T.empty mempty
