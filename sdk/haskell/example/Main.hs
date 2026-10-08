-- | Beispiel-Plugin: zeigt jede Aufrufrichtung einmal.
--
--   * HsGreeting.say     – einfacher Aufruf (Handle)
--   * HsNumbers.list     – Datenstrom, den dieses Plugin liefert (Read)
--   * HsRelay.accounts   – Datenstrom eines anderen Plugins lesen (DispatchRead),
--                          z. B. die Sachkonten des Ledgers, und zusammenfassen
--   * HsRelay.count      – beliebigen Datenstrom über den Host lesen und zählen
--                          ({"object", "action", "payload"})
--   * HsRelay.dbtime     – SQL über den Host (Query)
--
-- Konfiguration (Host):
--
-- > plugins:
-- >   hs-hello:
-- >     version: 0.1.0
-- >     databases:
-- >       main: { access: read }
module Main (main) where

import Control.Monad (forM_)
import Data.Aeson (Value (..), object, (.=))
import Data.Aeson.KeyMap qualified as KM
import Data.IORef
import Data.Text (Text)
import Data.Text qualified as T

import CoreMesh.Plugin

main :: IO ()
main =
  serve
    (defaultPlugin "hs-hello" "0.1.0")
      { pluginDescription = "Beispiel-Plugin in Haskell"
      , pluginCapabilities =
          [ Capability "HsGreeting" ["say"] [] "Begrüßung"
          , Capability "HsNumbers" ["list"] ["list"] "Zahlen 1..n als Datenstrom"
          , Capability "HsRelay" ["accounts", "count", "dbtime"] [] "Liest Daten anderer Plugins"
          ]
      , pluginConfigure = \call settings ->
          logMessage call LogInfo "hs-hello konfiguriert" [("settings", tshow (KM.size settings))]
      , pluginHandle = handler
      , pluginRead = reader
      }

handler :: Call -> Request -> IO Response
handler call req = case (reqObject req, reqAction req) of
  ("HsGreeting", "say") -> do
    let name = case reqPayload req of
          Object o | Just (String n) <- KM.lookup "name" o -> n
          _ -> "Welt"
    pure (response (object ["text" .= ("Hallo " <> name <> " aus Haskell"), "user" .= callUserId call]))
  ("HsRelay", "accounts") -> do
    count <- newIORef (0 :: Int)
    cols <- newIORef []
    end <-
      dispatchRead call "GLAccount" "list" (reqPayload req) $
        RowWriter
          { writeHeader = writeIORef cols . headerColumns
          , writeRows = \rs -> modifyIORef' count (+ length rs)
          }
    n <- readIORef count
    c <- readIORef cols
    pure (response (object ["rows" .= n, "columns" .= c, "reported" .= endRows end]))
  ("HsRelay", "count") -> do
    let field k = case reqPayload req of
          Object o -> KM.lookup k o
          _ -> Nothing
        text k = case field k of
          Just (String t) -> t
          _ -> ""
    count <- newIORef (0 :: Int)
    end <-
      dispatchRead call (text "object") (text "action") (maybe (object []) id (field "payload")) $
        RowWriter {writeHeader = \_ -> pure (), writeRows = \rs -> modifyIORef' count (+ length rs)}
    n <- readIORef count
    pure (response (object ["rows" .= n, "reported" .= endRows end]))
  ("HsRelay", "dbtime") -> do
    r <- query call "main" "SELECT CURRENT_TIMESTAMP" []
    pure (response (object ["now" .= qrRows r]))
  _ -> pluginError Unimplemented (reqObject req <> "." <> reqAction req)

-- | HsNumbers.list: {"n": 12345} Zeilen (i, Quadrat, Text) in Blöcken zu 1000.
reader :: Call -> Request -> RowWriter -> IO ReadEnd
reader _ req w = case (reqObject req, reqAction req) of
  ("HsNumbers", "list") -> do
    let n = case reqPayload req of
          Object o | Just (Number x) <- KM.lookup "n" o -> floor x
          _ -> 10 :: Int
    writeHeader w (ReadHeader ["i", "square", "text"] mempty)
    forM_ (chunks 1000 [1 .. n]) $ \is ->
      writeRows w [[Number (fromIntegral i), Number (fromIntegral (i * i)), String ("Zeile " <> tshow i)] | i <- is]
    pure readEnd
  _ -> pluginError Unimplemented (reqObject req <> "." <> reqAction req)

chunks :: Int -> [a] -> [[a]]
chunks _ [] = []
chunks k xs = let (a, b) = splitAt k xs in a : chunks k b

tshow :: Show a => a -> Text
tshow = T.pack . show
