-- | Plugin-Prozess nach dem Protokoll von HashiCorp go-plugin:
--
--   1. Magic Cookie prüfen (COREMESH_PLUGIN), Protokollversion 1.
--   2. Auf 127.0.0.1 lauschen (TCP auf allen Plattformen), eigenes Zertifikat
--      erzeugen, Handshake-Zeile auf stdout:
--      @1|1|tcp|127.0.0.1:<port>|grpc|<Zertifikat>@.
--   3. gRPC über HTTP/2 mit AutoMTLS: Health ("plugin"), GRPCBroker
--      (Verbindungsdaten des Hosts), GRPCController (Shutdown) und
--      PluginService (GetManifest, Configure, Handle, Read).
--   4. Configure: über den Broker zum HostService des Hosts verbinden.
module CoreMesh.Plugin.Internal.Serve
  ( Plugin (..)
  , Capability (..)
  , defaultPlugin
  , serve
  , stderrLog
  ) where

import Control.Concurrent (forkIO, threadDelay)
import Control.Concurrent.Async (async, cancel)
import Control.Concurrent.STM
import Control.Exception hiding (handle)
import Control.Monad (forM_, forever, unless, void, when)
import Data.Aeson qualified as J
import Data.Aeson.KeyMap qualified as KM
import Data.ByteString (ByteString)
import Data.ByteString qualified as B
import Data.ByteString.Char8 qualified as BC
import Data.ByteString.Lazy.Char8 qualified as BLC
import Data.IORef
import Data.Int (Int64)
import Data.Map.Strict (Map)
import Data.Map.Strict qualified as Map
import Data.ProtoLens (Message, decodeMessage, defMessage, encodeMessage)
import Data.Text (Text)
import Data.Text qualified as T
import Data.Text.Encoding qualified as TE
import Data.Word (Word32)
import Lens.Family2 ((&), (.~), (^.))
import Network.HTTP2.Server qualified as S
import Network.Socket
import Network.Socket.ByteString qualified as NSB
import Proto.Goplugin.GrpcBroker qualified as GB
import Proto.Goplugin.GrpcBroker_Fields qualified as GBF
import Proto.Goplugin.GrpcController qualified as GC
import Proto.Grpc.Health.V1.Health qualified as HC
import Proto.Grpc.Health.V1.Health_Fields qualified as HCF
import Proto.Plugin.V1.Plugin qualified as P
import Proto.Plugin.V1.Plugin_Fields qualified as PF
import System.Environment (lookupEnv)
import System.Exit (ExitCode (..), exitWith)
import System.IO
import System.Timeout (timeout)
import Text.Read (readMaybe)

import CoreMesh.Plugin.Internal.Context
import CoreMesh.Plugin.Internal.Host (LogLevel (..))
import CoreMesh.Plugin.Internal.Grpc (GrpcError (..), Method (..), Transport (..), grpcServer, http2Config, serverStream, withGrpcClient)
import CoreMesh.Plugin.Internal.Grpc qualified as G
import CoreMesh.Plugin.Internal.Host (contextFromProto)
import CoreMesh.Plugin.Internal.Tls
import CoreMesh.Plugin.Value (fromProto, structToJson, toProto)

-- | Business-Object mit seinen Actions (Routing-Einträge im Dispatcher).
data Capability = Capability
  { capObject :: Text
  , capActions :: [Text]
  , capReadActions :: [Text]
  -- ^ Teilmenge von capActions, zusätzlich als Datenstrom ('pluginRead')
  , capDescription :: Text
  }

-- | Ein Plugin. Mit 'defaultPlugin' beginnen und die Felder setzen.
data Plugin = Plugin
  { pluginName :: Text
  -- ^ wie in der Host-Config, [a-z0-9-]
  , pluginVersion :: Text
  , pluginDescription :: Text
  , pluginCapabilities :: [Capability]
  , pluginSchema :: Maybe Text
  -- ^ Atlas-HCL des Soll-Schemas: meldet DBSchema.Init (Tabellen mit Präfix <name>__)
  , pluginDescribe :: Maybe J.Value
  -- ^ Antwort auf Catalog.Describe (Metamodell): meldet Catalog.Describe
  , pluginConfigure :: Call -> KM.KeyMap J.Value -> IO ()
  -- ^ Einstellungen aus plugins.<name>.settings; der Host ist ab hier erreichbar
  , pluginHandle :: Call -> Request -> IO Response
  , pluginRead :: Call -> Request -> RowWriter -> IO ReadEnd
  , pluginShutdown :: IO ()
  }

defaultPlugin :: Text -> Text -> Plugin
defaultPlugin name version =
  Plugin
    { pluginName = name
    , pluginVersion = version
    , pluginDescription = ""
    , pluginCapabilities = []
    , pluginSchema = Nothing
    , pluginDescribe = Nothing
    , pluginConfigure = \_ _ -> pure ()
    , pluginHandle = \_ r -> unimplemented r
    , pluginRead = \_ r _ -> unimplemented r
    , pluginShutdown = pure ()
    }
  where
    unimplemented r = pluginError Unimplemented (reqObject r <> "." <> reqAction r)

-- | Logzeile auf stderr im JSON-Format von hclog: go-plugin übernimmt sie mit
-- ihrer Stufe ins Host-Log (einfache Zeilen nur auf Debug-Stufe). Für Meldungen,
-- bevor der Host erreichbar ist, und für Fehler des SDK selbst.
stderrLog :: LogLevel -> Text -> IO ()
stderrLog level msg = do
  BLC.hPutStrLn stderr (J.encode (J.object ["@level" J..= lvl, "@message" J..= msg]))
  hFlush stderr
  where
    lvl :: Text
    lvl = case level of
      LogDebug -> "debug"
      LogInfo -> "info"
      LogWarn -> "warn"
      LogError -> "error"

magicKey, magicValue :: String
magicKey = "COREMESH_PLUGIN"
magicValue = "coremesh-7c2f9a41-handshake-v1"

data State = State
  { stPlugin :: Plugin
  , stIdentity :: Identity
  , stPeer :: Maybe ByteString -- ^ DER des Host-Zertifikats (AutoMTLS)
  , stBroker :: TVar (Map Word32 (Text, Text)) -- ^ Dienst-ID → (Netz, Adresse)
  , stHost :: IORef (Maybe HostConn)
  , stHostThread :: IORef (Maybe (IO ()))
  , stShutdown :: TMVar ()
  }

-- | Startet das Plugin; kehrt erst zurück, wenn der Host es beendet.
serve :: Plugin -> IO ()
serve plugin = do
  hSetBinaryMode stdout True -- nur "\n", auch unter Windows
  hSetBuffering stdout LineBuffering
  hSetEncoding stderr utf8
  cookie <- lookupEnv magicKey
  when (cookie /= Just magicValue) $ do
    hPutStrLn stderr "Dieses Programm ist ein CoreMesh-Plugin und wird vom Host gestartet."
    exitWith (ExitFailure 1)
  versions <- lookupEnv "PLUGIN_PROTOCOL_VERSIONS"
  case versions of
    Just vs | "1" `notElem` T.splitOn "," (T.pack vs) -> do
      hPutStrLn stderr ("Protokollversion 1 nicht angeboten: " <> vs)
      exitWith (ExitFailure 1)
    _ -> pure ()
  peer <-
    lookupEnv "PLUGIN_CLIENT_CERT" >>= \case
      Nothing -> pure Nothing
      Just "" -> pure Nothing
      Just pem -> either (\e -> fail ("PLUGIN_CLIENT_CERT: " <> e)) (pure . Just) (parsePemCert (TE.encodeUtf8 (T.pack pem)))
  ident <- newIdentity
  sock <- listenLoopback
  port <- socketPort sock
  st <- State plugin ident peer <$> newTVarIO mempty <*> newIORef Nothing <*> newIORef Nothing <*> newEmptyTMVarIO
  let cert = maybe "" (const ("|" <> certBase64 ident)) peer
  BC.hPutStr stdout ("1|1|tcp|127.0.0.1:" <> BC.pack (show port) <> "|grpc" <> cert <> "\n")
  hFlush stdout
  acceptor <- async (acceptLoop st sock)
  atomically (takeTMVar (stShutdown st))
  cancel acceptor
  readIORef (stHostThread st) >>= sequence_
  pluginShutdown plugin `catch` \(e :: SomeException) -> stderrLog LogError ("Shutdown: " <> T.pack (displayException e))
  close sock

-- | Lauscht auf 127.0.0.1, im Portbereich PLUGIN_MIN_PORT..PLUGIN_MAX_PORT, falls gesetzt.
listenLoopback :: IO Socket
listenLoopback = do
  lo <- (>>= readMaybe) <$> lookupEnv "PLUGIN_MIN_PORT"
  hi <- (>>= readMaybe) <$> lookupEnv "PLUGIN_MAX_PORT"
  let ports = case (lo, hi) of
        (Just a, Just b) | a <= b -> [a .. b :: Int]
        _ -> [0]
      try1 [] = fail "kein freier Port im Bereich PLUGIN_MIN_PORT..PLUGIN_MAX_PORT"
      try1 (p : ps) = do
        s <- socket AF_INET Stream defaultProtocol
        r <- try (bind s (SockAddrInet (fromIntegral p) (tupleToHostAddress (127, 0, 0, 1))))
        case r of
          Right () -> listen s 16 >> pure s
          Left (_ :: IOException) -> close s >> try1 ps
  try1 ports

acceptLoop :: State -> Socket -> IO ()
acceptLoop st sock = forever $ do
  (conn, _) <- accept sock
  void . forkIO $ serveConnection st conn `finally` close conn

serveConnection :: State -> Socket -> IO ()
serveConnection st conn = do
  r <- try $ do
    t <- maybe (plainTransport conn) (\der -> serverTransport (stIdentity st) der conn) (stPeer st)
    bracket (http2Config t) snd $ \(conf, _) ->
      S.run G.serverConfig conf (grpcServer (route st))
  case r of
    Left (e :: SomeException)
      | Just (SomeAsyncException _) <- fromException e -> throwIO e
      | otherwise -> stderrLog LogWarn ("Verbindung des Hosts beendet: " <> T.pack (displayException e))
    Right () -> pure ()

plainTransport :: Socket -> IO Transport
plainTransport s = do
  me <- getSocketName s
  peer <- getPeerName s
  pure Transport {tSend = NSB.sendAll s, tRecv = NSB.recv s 16384, tMyAddr = me, tPeerAddr = peer}

-- --- Routing ---------------------------------------------------------------

route :: State -> ByteString -> Maybe Method
route st = \case
  "/grpc.health.v1.Health/Check" -> Just . Unary $ \_ ->
    pure (encodeMessage ((defMessage :: HC.HealthCheckResponse) & HCF.status .~ HC.HealthCheckResponse'SERVING))
  "/plugin.GRPCBroker/StartStream" -> Just . Bidi $ \recv _send -> brokerLoop st recv
  "/plugin.GRPCController/Shutdown" -> Just . Unary $ \_ -> do
    -- Erst antworten, dann beenden.
    void . forkIO $ threadDelay 100000 >> atomically (void (tryPutTMVar (stShutdown st) ()))
    pure (encodeMessage (defMessage :: GC.Empty))
  "/coremesh.plugin.v1.PluginService/GetManifest" -> Just . Unary $ \_ -> pure (encodeMessage (manifest (stPlugin st)))
  "/coremesh.plugin.v1.PluginService/Configure" -> Just . Unary $ guarded . configure st
  "/coremesh.plugin.v1.PluginService/Handle" -> Just . Unary $ guarded . handle st
  "/coremesh.plugin.v1.PluginService/Read" -> Just . ServerStream $ \bs send -> guarded (readStream st bs send)
  _ -> Nothing

-- | PluginError → gRPC-Status; andere Ausnahmen werden Internal.
guarded :: IO a -> IO a
guarded act =
  act `catches`
    [ Handler $ \(PluginError c m) -> throwIO (GrpcError (errorCodeTo c) m)
    , Handler $ \(e :: GrpcError) -> throwIO e
    , Handler $ \(e :: SomeAsyncException) -> throwIO e
    , Handler $ \(e :: SomeException) -> throwIO (GrpcError G.Internal (T.pack (displayException e)))
    ]

decodeReq :: Message m => ByteString -> IO m
decodeReq = either (\e -> pluginError InvalidArgument ("Request nicht lesbar: " <> T.pack e)) pure . decodeMessage

-- --- Broker ----------------------------------------------------------------

-- | Der Host meldet über den Strom, wo er Dienste anbietet (AcceptAndServe).
brokerLoop :: State -> IO (Maybe ByteString) -> IO ()
brokerLoop st recv =
  recv >>= \case
    Nothing -> pure ()
    Just bs -> do
      case decodeMessage bs :: Either String GB.ConnInfo of
        Right ci
          | ci ^. GBF.maybe'knock == Nothing ->
              atomically $ modifyTVar' (stBroker st) (Map.insert (ci ^. GBF.serviceId) (ci ^. GBF.network, ci ^. GBF.address))
        _ -> pure ()
      brokerLoop st recv

-- | Wartet auf die Verbindungsdaten zu id und verbindet sich (TLS wie der Host).
dialBroker :: State -> Word32 -> IO Transport
dialBroker st sid = do
  found <- timeout (10 * 1000000) . atomically $ do
    m <- readTVar (stBroker st)
    maybe retry pure (Map.lookup sid m)
  (network, address) <- maybe (pluginError Unavailable ("Broker: keine Verbindungsdaten für Dienst " <> T.pack (show sid))) pure found
  s <- case network of
    "unix" -> do
      s <- socket AF_UNIX Stream defaultProtocol
      connect s (SockAddrUnix (T.unpack address)) `onException` close s
      pure s
    "tcp" -> do
      let (h, p) = T.breakOnEnd ":" address
      addr : _ <- getAddrInfo (Just defaultHints {addrSocketType = Stream}) (Just (T.unpack (T.dropEnd 1 h))) (Just (T.unpack p))
      s <- socket (addrFamily addr) Stream defaultProtocol
      connect s (addrAddress addr) `onException` close s
      pure s
    other -> pluginError Unavailable ("Broker: unbekanntes Netz " <> other)
  maybe (plainTransport s) (\der -> clientTransport (stIdentity st) der s) (stPeer st)

-- --- PluginService ---------------------------------------------------------

manifest :: Plugin -> P.GetManifestResponse
manifest p = defMessage & PF.manifest .~ m
  where
    m =
      defMessage
        & PF.name .~ pluginName p
        & PF.version .~ pluginVersion p
        & PF.description .~ pluginDescription p
        & PF.capabilities .~ map cap (pluginCapabilities p <> lifecycle)
    cap c =
      defMessage
        & PF.object .~ capObject c
        & PF.actions .~ capActions c
        & PF.readActions .~ capReadActions c
        & PF.description .~ capDescription c
    lifecycle =
      [Capability "DBSchema" ["Init"] [] "" | Just _ <- [pluginSchema p]]
        <> [Capability "Catalog" ["Describe"] [] "" | Just _ <- [pluginDescribe p]]

configure :: State -> ByteString -> IO ByteString
configure st bs = do
  req <- decodeReq bs :: IO P.ConfigureRequest
  let sid = req ^. PF.hostServiceBrokerId
  when (sid == 0) $ pluginError InvalidArgument "host_service_broker_id fehlt"
  t <- dialBroker st sid
  -- Die Verbindung hält ein eigener Thread, solange das Plugin läuft.
  ready <- newEmptyTMVarIO
  stop <- newEmptyTMVarIO
  void . forkIO $
    ( withGrpcClient t $ \c -> do
        let conn = HostConn (serverStream c)
        atomically (putTMVar ready (Right conn))
        atomically (takeTMVar stop)
    )
      `catch` \(e :: SomeException) -> atomically (void (tryPutTMVar ready (Left e)))
  conn <- atomically (takeTMVar ready) >>= either throwIO pure
  old <- readIORef (stHostThread st)
  writeIORef (stHost st) (Just conn)
  writeIORef (stHostThread st) (Just (atomically (void (tryPutTMVar stop ()))))
  sequence_ old
  let call = contextFromProto (Just conn) (req ^. PF.context)
      settings = maybe mempty structToJson (req ^. PF.maybe'settings)
  pluginConfigure (stPlugin st) call settings
  pure (encodeMessage (defMessage :: P.ConfigureResponse))

currentCall :: State -> P.HandleRequest -> IO (Call, Request)
currentCall st req = do
  host <- readIORef (stHost st)
  let call = contextFromProto host (req ^. PF.context)
      r = Request (req ^. PF.object) (req ^. PF.action) (maybe J.Null fromProto (req ^. PF.maybe'payload))
  pure (call, r)

handle :: State -> ByteString -> IO ByteString
handle st bs = do
  req <- decodeReq bs :: IO P.HandleRequest
  (call, r) <- currentCall st req
  let p = stPlugin st
  resp <- case (reqObject r, reqAction r) of
    ("DBSchema", "Init") | Just schema <- pluginSchema p -> do
      let modName = case reqPayload r of
            J.Object o | Just (J.String m) <- KM.lookup "module" o -> m
            _ -> pluginName p
      pure (response (J.object ["module" J..= modName, "schema" J..= schema]))
    ("Catalog", "Describe") | Just d <- pluginDescribe p -> pure (Response d mempty)
    _ -> pluginHandle p call r
  pure . encodeMessage $
    (defMessage :: P.HandleResponse)
      & PF.context .~ (defMessage & PF.requestId .~ callRequestId call & PF.metadata .~ respMetadata resp)
      & PF.payload .~ toProto (respPayload resp)

-- | Grenzen eines batch wie im Go-Adapter (deutlich unter 4 MiB je Nachricht).
maxBatchBytes, maxBatchRows :: Int
maxBatchBytes = 1024 * 1024
maxBatchRows = 5000

readStream :: State -> ByteString -> (ByteString -> IO ()) -> IO ()
readStream st bs send = do
  req <- decodeReq bs :: IO P.HandleRequest
  (call, r) <- currentCall st req
  headerSent <- newIORef Nothing -- Anzahl Spalten
  firstMsg <- newIORef True
  batch <- newIORef ([], 0 :: Int, 0 :: Int) -- Zeilen (umgekehrt), Bytes, Anzahl
  delivered <- newIORef (0 :: Int64)
  let ctxFor = do
        f <- readIORef firstMsg
        writeIORef firstMsg False
        pure (if f then Just (defMessage & PF.requestId .~ callRequestId call) else Nothing)
      emit part = do
        c <- ctxFor
        send . encodeMessage $ (defMessage :: P.ReadResponse) & PF.maybe'context .~ c & PF.maybe'part .~ Just part
      flush = do
        (rows, _, n) <- readIORef batch
        unless (n == 0) $ do
          writeIORef batch ([], 0, 0)
          emit (P.ReadResponse'Batch (defMessage & PF.rows .~ reverse rows))
      header h = do
        readIORef headerSent >>= \case
          Just _ -> pluginError Internal "Read: Header doppelt"
          Nothing -> writeIORef headerSent (Just (length (headerColumns h)))
        emit (P.ReadResponse'Header (defMessage & PF.columns .~ headerColumns h & PF.metadata .~ headerMetadata h))
      rows rs = do
        cols <- readIORef headerSent >>= maybe (pluginError Internal "Read: Zeilen vor dem Header") pure
        forM_ rs $ \row -> do
          when (length row /= cols) $
            pluginError Internal ("Read: Zeile mit " <> T.pack (show (length row)) <> " Werten, Header hat " <> T.pack (show cols) <> " Spalten")
          let msg = defMessage & PF.values .~ map toProto row :: P.ReadRow
              size = B.length (encodeMessage msg)
          when (size > maxBatchBytes) $ pluginError Internal "Read: Zeile zu groß"
          (_, bytes, _) <- readIORef batch
          when (bytes + size > maxBatchBytes) flush
          modifyIORef' batch (\(xs, b, n) -> (msg : xs, b + size, n + 1))
          modifyIORef' delivered (+ 1)
          (_, _, n) <- readIORef batch
          when (n >= maxBatchRows) flush
  end <- pluginRead (stPlugin st) call r (RowWriter header rows)
  readIORef headerSent >>= \case
    Nothing -> header (ReadHeader [] mempty)
    Just _ -> pure ()
  flush
  n <- readIORef delivered
  emit . P.ReadResponse'End $
    defMessage
      & PF.rows .~ (if endRows end == 0 then n else endRows end)
      & PF.cursor .~ endCursor end
      & PF.metadata .~ endMetadata end
