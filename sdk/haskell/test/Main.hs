-- | Tests ohne Go-Host: Werte, Rahmung, gRPC über mTLS (Loopback).
-- Gegen den echten Host testet das Beispiel-Plugin (siehe README).
module Main (main) where

import Control.Concurrent (forkIO)
import Control.Concurrent.MVar
import Control.Exception
import Control.Monad (forM_, replicateM_, unless, void)
import Data.Aeson qualified as J
import Data.ByteString qualified as B
import Data.ByteString.Builder qualified as BB
import Data.ByteString.Lazy qualified as BL
import Data.IORef
import Network.HTTP2.Server qualified as S
import Network.Socket
import System.Exit (exitFailure)

import CoreMesh.Plugin.Internal.Grpc
import CoreMesh.Plugin.Internal.Tls
import CoreMesh.Plugin.Value

main :: IO ()
main = do
  failures <- newIORef (0 :: Int)
  let check name ok = unless ok $ do
        putStrLn ("FEHLER: " <> name)
        modifyIORef' failures (+ 1)
  valueRoundTrip check
  framing check
  mtls check
  n <- readIORef failures
  if n == 0 then putStrLn "alle Tests bestanden" else exitFailure

valueRoundTrip :: (String -> Bool -> IO ()) -> IO ()
valueRoundTrip check = do
  let v = J.object ["a" J..= (1 :: Int), "b" J..= [J.Null, J.Bool True, J.String "ä€"], "c" J..= J.object ["d" J..= (2.5 :: Double)]]
  check "Value hin und zurück" (fromProto (toProto v) == v)

-- | Nachrichten über beliebige Blockgrenzen hinweg.
framing :: (String -> Bool -> IO ()) -> IO ()
framing check = do
  let msgs = [B.replicate n 0x41 | n <- [0, 1, 5, 4096, 70000]]
      bytes = BL.toStrict (BB.toLazyByteString (foldMap frame msgs))
  forM_ [1, 3, 7, 1000, B.length bytes] $ \size -> do
    rest <- newIORef bytes
    reader <- newFrameReader $ do
      b <- readIORef rest
      let (a, z) = B.splitAt size b
      writeIORef rest z
      pure a
    got <- collect reader
    check ("Rahmung, Blockgröße " <> show size) (got == msgs)
  where
    collect r = readFrame r >>= maybe (pure []) (\m -> (m :) <$> collect r)

-- | Server und Client mit je eigener Identität; jede Seite kennt nur das
-- Zertifikat der anderen (wie AutoMTLS zwischen Host und Plugin).
mtls :: (String -> Bool -> IO ()) -> IO ()
mtls check = do
  server <- newIdentity
  client <- newIdentity
  stranger <- newIdentity
  sock <- socket AF_INET Stream defaultProtocol
  bind sock (SockAddrInet 0 (tupleToHostAddress (127, 0, 0, 1)))
  listen sock 8
  port <- socketPort sock
  let route = \case
        "/t.T/Echo" -> Just (Unary pure)
        "/t.T/Many" -> Just . ServerStream $ \req send -> forM_ [1 .. 20000 :: Int] $ \_ -> send req
        "/t.T/Fail" -> Just . Unary $ \_ -> throwIO (GrpcError NotFound "gibt es nicht")
        _ -> Nothing
  -- Endet mit dem Schließen des Sockets am Testende.
  void . forkIO . handle (\(_ :: SomeException) -> pure ()) . forever' $ do
    (c, _) <- accept sock
    void . forkIO $ do
      r <- try $ do
        t <- serverTransport server (idDer client) c
        bracket (http2Config t) snd $ \(conf, _) -> S.run serverConfig conf (grpcServer route)
      either (\(_ :: SomeException) -> pure ()) pure r
      close c
  let connect' me = do
        s <- socket AF_INET Stream defaultProtocol
        connect s (SockAddrInet port (tupleToHostAddress (127, 0, 0, 1)))
        clientTransport me (idDer server) s
  t <- connect' client
  withGrpcClient t $ \c -> do
    echo <- unary c "/t.T/Echo" "hallo"
    check "einfacher Aufruf" (echo == "hallo")
    n <- newIORef (0 :: Int)
    serverStream c "/t.T/Many" (B.replicate 200 0x42) (\_ -> modifyIORef' n (+ 1))
    readIORef n >>= check "Server-Strom 20000 Nachrichten" . (== 20000)
    r <- try (unary c "/t.T/Fail" "")
    check "Fehlercode" $ case r of
      Left (GrpcError NotFound "gibt es nicht") -> True
      _ -> False
    r2 <- try (unary c "/t.T/Nope" "")
    check "unbekannte Methode" $ case r2 of
      Left (GrpcError Unimplemented _) -> True
      _ -> False
    -- nebenläufige Aufrufe über dieselbe Verbindung
    done <- newEmptyMVar
    replicateM_ 50 . forkIO $ unary c "/t.T/Echo" "x" >>= putMVar done
    rs <- mapM (const (takeMVar done)) [1 .. 50 :: Int]
    check "50 nebenläufige Aufrufe" (all (== "x") rs)
  -- fremdes Client-Zertifikat: der Server lehnt den Handshake ab
  r <- try (connect' stranger >>= \t' -> withGrpcClient t' (\c -> unary c "/t.T/Echo" "x"))
  check "fremdes Zertifikat abgelehnt" $ case r of
    Left (_ :: SomeException) -> True
    Right _ -> False
  close sock
  where
    forever' a = a >> forever' a
