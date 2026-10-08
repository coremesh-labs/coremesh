-- | Rückkanal zum Host (HostService): andere Plugins aufrufen, Datenströme
-- lesen, SQL auf den freigegebenen Datenbanken, Transaktionen, Log.
module CoreMesh.Plugin.Internal.Host
  ( dispatch
  , dispatchRead
  , query
  , exec
  , QueryResult (..)
  , ExecResult (..)
  , inTx
  , LogLevel (..)
  , logMessage
    -- * intern
  , contextToProto
  , contextFromProto
  , hostRpc
  , rethrowGrpc
  ) where

import Control.Exception
import Control.Monad (void)
import Data.Aeson qualified as J
import Data.ByteString (ByteString)
import Data.IORef
import Data.Int (Int64)
import Data.Map.Strict qualified as Map
import Data.ProtoLens (Message, decodeMessage, defMessage, encodeMessage)
import Data.Text (Text)
import Data.Text qualified as T
import Lens.Family2 ((&), (.~), (^.))
import Proto.Plugin.V1.Host qualified as H
import Proto.Plugin.V1.Host_Fields qualified as HF
import Proto.Plugin.V1.Plugin qualified as P
import Proto.Plugin.V1.Plugin_Fields qualified as PF

import CoreMesh.Plugin.Internal.Context
import CoreMesh.Plugin.Internal.Grpc (GrpcError (..))
import CoreMesh.Plugin.Value (fromProto, toProto)

contextToProto :: Call -> P.Context
contextToProto Call {..} =
  defMessage
    & PF.requestId .~ callRequestId
    & PF.tenantId .~ callTenantId
    & PF.userId .~ callUserId
    & PF.metadata .~ callMetadata
    & PF.txIds .~ callTxIds

contextFromProto :: Maybe HostConn -> P.Context -> Call
contextFromProto host c =
  Call
    { callRequestId = c ^. PF.requestId
    , callTenantId = c ^. PF.tenantId
    , callUserId = c ^. PF.userId
    , callMetadata = c ^. PF.metadata
    , callTxIds = c ^. PF.txIds
    , callHost = host
    }

-- | gRPC-Fehler des Hosts als PluginError (wie fromStatus im Go-Adapter).
rethrowGrpc :: IO a -> IO a
rethrowGrpc act = act `catch` \(GrpcError c m) -> throwIO (PluginError (errorCodeFrom c) m)

decodeOrFail :: Message m => ByteString -> IO m
decodeOrFail bs = either (\e -> pluginError Internal ("Antwort des Hosts nicht lesbar: " <> T.pack e)) pure (decodeMessage bs)

-- | Ein Aufruf des HostService; jede Antwortnachricht geht an onMsg.
hostRpc :: (Message req, Message resp) => Call -> ByteString -> req -> (resp -> IO ()) -> IO ()
hostRpc call method req onMsg = case callHost call of
  Nothing -> pluginError Unavailable "kein Host verbunden (Plugin noch nicht konfiguriert?)"
  Just h ->
    rethrowGrpc $
      hostCall h ("/coremesh.plugin.v1.HostService/" <> method) (encodeMessage req) $
        \bs -> decodeOrFail bs >>= onMsg

hostUnary :: (Message req, Message resp) => Call -> ByteString -> req -> IO resp
hostUnary call method req = do
  ref <- newIORef Nothing
  hostRpc call method req (writeIORef ref . Just)
  readIORef ref >>= maybe (pluginError Internal "keine Antwort des Hosts") pure

handleRequest :: Call -> Text -> Text -> J.Value -> P.HandleRequest
handleRequest call object action payload =
  defMessage
    & PF.context .~ contextToProto call
    & PF.object .~ object
    & PF.action .~ action
    & PF.payload .~ toProto payload

-- | Ruft (object, action) eines anderen Plugins über den Dispatcher auf.
dispatch :: Call -> Text -> Text -> J.Value -> IO J.Value
dispatch call object action payload = do
  resp <- hostUnary call "Dispatch" (handleRequest call object action payload) :: IO P.HandleResponse
  pure (maybe J.Null fromProto (resp ^. PF.maybe'payload))

-- | Ruft (object, action) als Datenstrom auf; Header und Zeilenblöcke gehen an w.
dispatchRead :: Call -> Text -> Text -> J.Value -> RowWriter -> IO ReadEnd
dispatchRead call object action payload w = do
  done <- newIORef Nothing
  hostRpc call "DispatchRead" (handleRequest call object action payload) $ \(msg :: P.ReadResponse) ->
    case msg ^. PF.maybe'part of
      Just (P.ReadResponse'Header h) ->
        writeHeader w (ReadHeader (h ^. PF.columns) (h ^. PF.metadata))
      Just (P.ReadResponse'Batch b) ->
        writeRows w [map fromProto (r ^. PF.values) | r <- b ^. PF.rows]
      Just (P.ReadResponse'End e) ->
        writeIORef done (Just (ReadEnd (e ^. PF.rows) (e ^. PF.cursor) (e ^. PF.metadata)))
      Nothing -> pure ()
  readIORef done
    >>= maybe (pluginError Unavailable ("Read " <> object <> "." <> action <> ": Strom ohne Abschluss")) pure

data QueryResult = QueryResult
  { qrColumns :: [Text]
  , qrRows :: [[J.Value]]
  }
  deriving stock (Show)

data ExecResult = ExecResult
  { erRowsAffected :: Int64
  , erLastInsertId :: Int64
  }
  deriving stock (Show)

-- | SELECT auf der logischen Datenbank (z. B. "main"); Werte immer über args
-- mit ?-Platzhaltern.
query :: Call -> Text -> Text -> [J.Value] -> IO QueryResult
query call db sql args = do
  resp :: H.QueryResponse <-
    hostUnary call "Query" $
      (defMessage :: H.QueryRequest)
        & HF.context .~ contextToProto call
        & HF.database .~ db
        & HF.sql .~ sql
        & HF.args .~ map toProto args
  pure (QueryResult (resp ^. HF.columns) [map fromProto (r ^. HF.values) | (r :: H.Row) <- resp ^. HF.rows])

-- | INSERT/UPDATE/DELETE.
exec :: Call -> Text -> Text -> [J.Value] -> IO ExecResult
exec call db sql args = do
  resp :: H.ExecResponse <-
    hostUnary call "Exec" $
      (defMessage :: H.ExecRequest)
        & HF.context .~ contextToProto call
        & HF.database .~ db
        & HF.sql .~ sql
        & HF.args .~ map toProto args
  pure (ExecResult (resp ^. HF.rowsAffected) (resp ^. HF.lastInsertId))

-- | Führt act in einer Transaktion auf db aus (wie sdk.InTx): Alle Host-Aufrufe
-- mit dem übergebenen Call laufen in ihr, auch die anderer Plugins über
-- 'dispatch'. Läuft schon eine, nimmt act an ihr teil. Ausnahme = Rollback.
inTx :: Call -> Text -> (Call -> IO a) -> IO a
inTx call db act = case Map.lookup db (callTxIds call) of
  Just txId -> act call `onException` rollback txId
  Nothing -> do
    resp :: H.BeginTxResponse <-
      hostUnary call "BeginTx" $
        (defMessage :: H.BeginTxRequest) & HF.context .~ contextToProto call & HF.database .~ db
    let txId = resp ^. HF.txId
        txCall = call {callTxIds = Map.insert db txId (callTxIds call)}
    r <- act txCall `onException` rollback txId
    void
      ( hostUnary txCall "CommitTx" $
          (defMessage :: H.CommitTxRequest) & HF.context .~ contextToProto txCall & HF.txId .~ txId
          :: IO H.CommitTxResponse
      )
    pure r
  where
    rollback txId = do
      r <-
        try
          ( hostUnary call "RollbackTx" $
              (defMessage :: H.RollbackTxRequest) & HF.context .~ contextToProto call & HF.txId .~ txId
              :: IO H.RollbackTxResponse
          )
      case r of
        Left (_ :: PluginError) -> pure () -- bereits abgebrochen
        Right _ -> pure ()

data LogLevel = LogDebug | LogInfo | LogWarn | LogError
  deriving stock (Eq, Show)

-- | Schreibt in das Log des Hosts (mit plugin und request_id).
logMessage :: Call -> LogLevel -> Text -> [(Text, Text)] -> IO ()
logMessage call level msg fields =
  void
    ( hostUnary call "Log" $
        (defMessage :: H.LogRequest)
          & HF.context .~ contextToProto call
          & HF.level .~ lvl
          & HF.message .~ msg
          & HF.fields .~ Map.fromList fields
        :: IO H.LogResponse
    )
  where
    lvl = case level of
      LogDebug -> H.LOG_LEVEL_DEBUG
      LogInfo -> H.LOG_LEVEL_INFO
      LogWarn -> H.LOG_LEVEL_WARN
      LogError -> H.LOG_LEVEL_ERROR
