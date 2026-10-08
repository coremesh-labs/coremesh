-- | Schlankes gRPC über HTTP/2 (Paket @http2@): genau das, was ein Plugin
-- gegenüber go-plugin braucht.
--
--   * Server: einfache Aufrufe, Server-Ströme und bidirektionale Ströme
--     (GRPCBroker.StartStream), Routing über den Pfad @/paket.Dienst/Methode@.
--   * Client: einfache Aufrufe und Server-Ströme (HostService über den Broker).
--
-- Nachrichten sind rohe Protobuf-Bytes; (De-)Serialisierung macht der
-- Aufrufer mit proto-lens. Kompression wird nicht unterstützt (go-plugin
-- nutzt keine). Die Transportschicht (TLS oder Klartext) kommt von außen als
-- 'Transport'.
module CoreMesh.Plugin.Internal.Grpc
  ( -- * Status
    GrpcError (..)
  , Code (..)
  , codeNumber
  , codeFromNumber
    -- * Transport
  , Transport (..)
  , http2Config
    -- * Server
  , Method (..)
  , grpcServer
  , serverConfig
    -- * Client
  , GrpcClient
  , withGrpcClient
  , unary
  , serverStream
    -- * Rahmung (für Tests)
  , frame
  , FrameReader
  , newFrameReader
  , readFrame
  ) where

import Control.Concurrent.Async (withAsync)
import Control.Concurrent.MVar
import Control.Exception
import Control.Monad (unless)
import Data.Bits (shiftL, shiftR, (.&.), (.|.))
import Data.ByteString qualified as B
import Data.ByteString.Builder qualified as BB
import Data.ByteString.Char8 qualified as BC
import Data.IORef
import Data.Text (Text)
import Data.Text qualified as T
import Data.Text.Encoding qualified as TE
import Foreign.Marshal.Alloc (free, mallocBytes)
import Data.CaseInsensitive (foldedCase)
import Network.HTTP.Semantics (tokenKey)
import Network.HTTP.Types (RequestHeaders, ResponseHeaders, status200)
import Network.HTTP2.Client qualified as C
import Network.HTTP2.Server qualified as S
import Network.Socket (SockAddr)
import System.TimeManager qualified as TM
import Text.Read (readMaybe)

-- | gRPC-Statuscodes (Teilmenge, die auf der Go-Seite als sdk.Err* ankommt).
data Code
  = OK
  | Canceled
  | Unknown
  | InvalidArgument
  | DeadlineExceeded
  | NotFound
  | AlreadyExists
  | PermissionDenied
  | ResourceExhausted
  | FailedPrecondition
  | Aborted
  | OutOfRange
  | Unimplemented
  | Internal
  | Unavailable
  | DataLoss
  | Unauthenticated
  deriving stock (Eq, Show, Enum, Bounded)

codeNumber :: Code -> Int
codeNumber = fromEnum

codeFromNumber :: Int -> Code
codeFromNumber n
  | n >= 0 && n <= fromEnum (maxBound :: Code) = toEnum n
  | otherwise = Unknown

-- | Fehler eines gRPC-Aufrufs (Status ungleich OK).
data GrpcError = GrpcError Code Text
  deriving stock (Show)

instance Exception GrpcError

-- --- Rahmung ---------------------------------------------------------------

-- | Eine Nachricht mit 5-Byte-Präfix (unkomprimiert, Länge big-endian).
frame :: B.ByteString -> BB.Builder
frame msg =
  BB.word8 0 <> BB.word32BE (fromIntegral (B.length msg)) <> BB.byteString msg

-- | Liest Nachrichten aus einer Folge von Datenblöcken (leer = Ende).
data FrameReader = FrameReader (IO B.ByteString) (IORef B.ByteString)

newFrameReader :: IO B.ByteString -> IO FrameReader
newFrameReader next = FrameReader next <$> newIORef B.empty

-- | Nächste Nachricht oder Nothing am Ende des Stroms.
readFrame :: FrameReader -> IO (Maybe B.ByteString)
readFrame (FrameReader next ref) = do
  hdr <- need 5
  case hdr of
    Nothing -> pure Nothing
    Just h -> do
      unless (B.index h 0 == 0) $
        throwIO (GrpcError Unimplemented "komprimierte Nachrichten werden nicht unterstützt")
      let len = B.foldl' (\acc w -> acc `shiftL` 8 .|. fromIntegral w) (0 :: Int) (B.drop 1 h)
      need len >>= \case
        Nothing -> throwIO (GrpcError Internal "Nachricht unvollständig")
        Just m -> pure (Just m)
  where
    need n = do
      buf <- readIORef ref
      fill buf
      where
        fill buf
          | B.length buf >= n = do
              let (a, b) = B.splitAt n buf
              writeIORef ref b
              pure (Just a)
          | otherwise = do
              chunk <- next
              if B.null chunk
                then
                  if B.null buf && n > 0
                    then writeIORef ref B.empty >> pure Nothing
                    else
                      if n == 0
                        then pure (Just B.empty)
                        else throwIO (GrpcError Internal "Strom endet mitten in einer Nachricht")
                else fill (buf <> chunk)

-- --- Transport -------------------------------------------------------------

-- | Byte-Transport einer Verbindung (TLS-Kontext oder Socket).
data Transport = Transport
  { tSend :: B.ByteString -> IO ()
  , tRecv :: IO B.ByteString -- ^ leer = Verbindung zu
  , tMyAddr :: SockAddr
  , tPeerAddr :: SockAddr
  }

bufferSize :: Int
bufferSize = 16384

-- | HTTP/2-Konfiguration über einem Transport; mit 'freeConfig' freigeben.
http2Config :: Transport -> IO (S.Config, IO ())
http2Config Transport {..} = do
  buf <- mallocBytes bufferSize
  mgr <- TM.initialize (30 * 1000000)
  leftover <- newIORef B.empty
  let readN n = do
        acc <- readIORef leftover
        go acc
        where
          go acc
            | B.length acc >= n = do
                let (a, b) = B.splitAt n acc
                writeIORef leftover b
                pure a
            | otherwise = do
                chunk <- tRecv
                if B.null chunk
                  then writeIORef leftover B.empty >> pure acc
                  else go (acc <> chunk)
      conf =
        S.Config
          { S.confWriteBuffer = buf
          , S.confBufferSize = bufferSize
          , S.confSendAll = tSend
          , S.confReadN = readN
          , S.confPositionReadMaker = S.defaultPositionReadMaker
          , S.confTimeoutManager = mgr
          , S.confMySockAddr = tMyAddr
          , S.confPeerSockAddr = tPeerAddr
          }
  pure (conf, free buf)

-- --- Server ----------------------------------------------------------------

-- | Implementierung einer Methode. Nachrichten sind rohe Protobuf-Bytes.
data Method
  = -- | ein Request, eine Antwort
    Unary (B.ByteString -> IO B.ByteString)
  | -- | ein Request, Antworten über die übergebene Sendefunktion
    ServerStream (B.ByteString -> (B.ByteString -> IO ()) -> IO ())
  | -- | Requests über die Lesefunktion (Nothing = Ende), Antworten über die Sendefunktion
    Bidi (IO (Maybe B.ByteString) -> (B.ByteString -> IO ()) -> IO ())

-- | HTTP/2-Einstellungen für die Verbindung zum Host (beide Richtungen).
--
--   * PING: grpc-go schätzt beim Empfang großer Mengen laufend die Bandbreite
--     (BDP) und sendet dazu PINGs. Die Voreinstellung von @http2@ (10/s) wertet
--     das als Angriff und beendet die Verbindung. Die Gegenseite ist per mTLS
--     authentifiziert und läuft auf Loopback; die Grenze ist deshalb hoch.
--   * Fenster der Flusskontrolle: 4 MiB je Strom, 16 MiB je Verbindung, damit
--     große Datenströme nicht auf kleine Fenster warten.
tuned :: S.Settings -> S.Settings
tuned s = s {S.pingRateLimit = 100000, S.initialWindowSize = 4 * 1024 * 1024}

connWindow :: Int
connWindow = 16 * 1024 * 1024

serverConfig :: S.ServerConfig
serverConfig =
  S.defaultServerConfig
    { S.settings = tuned (S.settings S.defaultServerConfig)
    , S.connectionWindowSize = connWindow
    }

grpcHeaders :: ResponseHeaders
grpcHeaders = [("content-type", "application/grpc")]

-- | HTTP/2-Server für gRPC; route liefert die Methode zu einem Pfad.
grpcServer :: (B.ByteString -> Maybe Method) -> S.Server
grpcServer route req _aux respond = do
  let path = maybe "" id (S.requestPath req)
  outcome <- newIORef (Unimplemented, "")
  let finish c m = writeIORef outcome (c, m)
      body write flush = case route path of
        Nothing -> finish Unimplemented ("unbekannte Methode " <> TE.decodeUtf8Lenient path)
        Just m -> do
          reader <- newFrameReader (S.getRequestBodyChunk req)
          let send msg = write (frame msg) >> flush
              run = case m of
                Unary f -> do
                  msg <- readFrame reader >>= maybe (throwIO (GrpcError Internal "Request fehlt")) pure
                  f msg >>= send
                ServerStream f -> do
                  msg <- readFrame reader >>= maybe (throwIO (GrpcError Internal "Request fehlt")) pure
                  f msg send
                Bidi f -> f (readFrame reader) send
          r <- try run
          case r of
            Right () -> finish OK ""
            Left e -> case fromException e of
              Just (GrpcError c msg) -> finish c msg
              Nothing
                | Just (SomeAsyncException _) <- fromException e -> throwIO e
                | otherwise -> finish Internal (T.pack (displayException e))
      trailers = \case
        Just _ -> pure (S.NextTrailersMaker trailers)
        Nothing -> do
          (c, m) <- readIORef outcome
          pure $
            S.Trailers $
              ("grpc-status", BC.pack (show (codeNumber c)))
                : [("grpc-message", percentEncode m) | not (T.null m)]
      resp = S.setResponseTrailersMaker (S.responseStreaming status200 grpcHeaders body) trailers
  respond resp []

-- | grpc-message: Prozent-Kodierung wie in der gRPC-Spezifikation.
percentEncode :: Text -> B.ByteString
percentEncode = B.concatMap enc . TE.encodeUtf8
  where
    enc w
      | w >= 0x20 && w <= 0x7e && w /= 0x25 = B.singleton w
      | otherwise = BC.pack ('%' : hex2 w)
    hex2 w = [hexDigit (w `shiftR` 4), hexDigit (w .&. 0x0f)]
    hexDigit n = "0123456789ABCDEF" !! fromIntegral n

percentDecode :: B.ByteString -> Text
percentDecode = TE.decodeUtf8Lenient . B.pack . go . B.unpack
  where
    go (0x25 : a : b : rest) | Just n <- readMaybe ['0', 'x', toC a, toC b] = n : go rest
    go (x : rest) = x : go rest
    go [] = []
    toC = toEnum . fromIntegral

-- --- Client ----------------------------------------------------------------

-- | Verbindung zu einem gRPC-Server; Aufrufe dürfen nebenläufig erfolgen.
newtype GrpcClient = GrpcClient (forall r. C.Request -> (C.Response -> IO r) -> IO r)

-- | Baut über einem Transport eine HTTP/2-Verbindung auf und hält sie, solange
-- action läuft.
withGrpcClient :: Transport -> (GrpcClient -> IO a) -> IO a
withGrpcClient transport action = do
  (conf, release) <- http2Config transport
  ready <- newEmptyMVar
  done <- newEmptyMVar
  let cconf =
        C.defaultClientConfig
          { C.scheme = "https"
          , C.authority = "localhost"
          , C.settings = tuned (C.settings C.defaultClientConfig)
          , C.connectionWindowSize = connWindow
          }
      client :: C.Client ()
      client sendReq _aux = do
        putMVar ready (GrpcClient sendReq)
        takeMVar done
      runner = C.run cconf conf client `finally` release
  withAsync runner $ \_ -> do
    c <- takeMVar ready
    action c `finally` putMVar done ()

requestHeaders :: RequestHeaders
requestHeaders = [("content-type", "application/grpc"), ("te", "trailers")]

-- | Einfacher Aufruf: Pfad, Request-Bytes → Antwort-Bytes.
unary :: GrpcClient -> B.ByteString -> B.ByteString -> IO B.ByteString
unary c path msg = do
  got <- newIORef Nothing
  serverStream c path msg $ \m -> writeIORef got (Just m)
  readIORef got >>= maybe (throwIO (GrpcError Internal "Antwort fehlt")) pure

-- | Server-Strom: jede Antwortnachricht geht an onMsg (blockiert = Gegendruck).
serverStream :: GrpcClient -> B.ByteString -> B.ByteString -> (B.ByteString -> IO ()) -> IO ()
serverStream (GrpcClient send) path msg onMsg = do
  let req = C.requestBuilder "POST" path requestHeaders (frame msg)
  send req $ \resp -> do
    -- Fehler ohne Nachrichten kommen als "trailers-only" in den Kopfzeilen.
    checkStatus (headerList (C.responseHeaders resp))
    reader <- newFrameReader (C.getResponseBodyChunk resp)
    let loop = readFrame reader >>= maybe (pure ()) (\m -> onMsg m >> loop)
    loop
    C.getResponseTrailers resp >>= \case
      Just t -> checkStatus (headerList t) >> requireStatus (headerList t)
      Nothing -> unless (hasStatus (headerList (C.responseHeaders resp))) $
        throwIO (GrpcError Internal "Antwort ohne grpc-status")
  where
    headerList (lst, _) = [(foldedCase (tokenKey t), v) | (t, v) <- lst]
    hasStatus = any ((== "grpc-status") . fst)
    requireStatus hs = unless (hasStatus hs) $ throwIO (GrpcError Internal "Trailer ohne grpc-status")
    checkStatus hs = case lookup "grpc-status" hs of
      Nothing -> pure ()
      Just s -> case readMaybe (BC.unpack s) of
        Just 0 -> pure ()
        Just n -> throwIO (GrpcError (codeFromNumber n) (maybe "" percentDecode (lookup "grpc-message" hs)))
        Nothing -> throwIO (GrpcError Internal ("ungültiger grpc-status " <> TE.decodeUtf8Lenient s))
