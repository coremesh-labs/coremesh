-- | Plugin-SDK für CoreMesh – die Haskell-Seite von @pkg/sdk@ und
-- @pkg/sdk/plugin@ des Go-SDK.
--
-- > import CoreMesh.Plugin
-- >
-- > main :: IO ()
-- > main = serve (defaultPlugin "hello-hs" "0.1.0")
-- >   { pluginCapabilities = [Capability "Greeting" ["say"] [] "Begrüßung"]
-- >   , pluginHandle = \call req -> case (reqObject req, reqAction req) of
-- >       ("Greeting", "say") -> pure (response (object ["text" .= ("Hallo" :: Text)]))
-- >       _ -> pluginError Unimplemented (reqAction req)
-- >   }
--
-- Aufrufe an den Host nehmen immer den 'Call' des laufenden Aufrufs mit:
-- Darüber bleiben Korrelations-ID, Mandant, Benutzer (Rechte) und laufende
-- Transaktionen über die ganze Aufrufkette erhalten.
module CoreMesh.Plugin
  ( -- * Plugin
    Plugin (..)
  , Capability (..)
  , defaultPlugin
  , serve
    -- * Aufruf
  , Call (..)
  , Request (..)
  , Response (..)
  , response
    -- * Fehler
  , PluginError (..)
  , ErrorCode (..)
  , pluginError
    -- * Datenströme
  , RowWriter (..)
  , ReadHeader (..)
  , ReadEnd (..)
  , readEnd
    -- * Host
  , dispatch
  , dispatchRead
  , query
  , exec
  , QueryResult (..)
  , ExecResult (..)
  , inTx
  , LogLevel (..)
  , logMessage
  , stderrLog
  ) where

import CoreMesh.Plugin.Internal.Context
import CoreMesh.Plugin.Internal.Host
import CoreMesh.Plugin.Internal.Serve
