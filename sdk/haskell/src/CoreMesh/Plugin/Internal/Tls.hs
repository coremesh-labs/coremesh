-- | AutoMTLS von go-plugin: Der Host schickt sein Zertifikat in
-- @PLUGIN_CLIENT_CERT@ (PEM), das Plugin erzeugt bei jedem Start ein eigenes
-- und meldet es im Handshake (DER, Base64 ohne Padding). Beide Seiten
-- vertrauen genau dem Zertifikat der anderen – in beide Richtungen: Host →
-- Plugin (PluginService) und Plugin → Host (Broker-Verbindung zum HostService).
--
-- Schlüssel: Ed25519 (go-plugin selbst nutzt ECDSA P-521; Go akzeptiert beides).
module CoreMesh.Plugin.Internal.Tls
  ( Identity (..)
  , newIdentity
  , certBase64
  , parsePemCert
  , serverTransport
  , clientTransport
  ) where

import Control.Exception (throwIO)
import Crypto.PubKey.Ed25519 qualified as Ed
import Crypto.Random (getRandomBytes)
import Data.ASN1.Types (ASN1CharacterString (..), ASN1StringEncoding (UTF8), getObjectID)
import Data.ByteArray (convert)
import Data.ByteString qualified as B
import Data.ByteString.Base64 qualified as B64
import Data.ByteString.Char8 qualified as BC
import Data.ByteString.Lazy qualified as BL
import Data.Default (def)
import Data.Hourglass (DateTime, Elapsed (..), Seconds (..), timeConvert)
import Data.PEM (pemContent, pemName, pemParseBS)
import Data.Time.Clock.POSIX (getPOSIXTime)
import Data.X509
import Data.X509.Validation (FailedReason (UnknownCA))
import Network.Socket (Socket, getPeerName, getSocketName)
import Network.TLS qualified as TLS
import Network.TLS.Extra.Cipher qualified as TLS

import CoreMesh.Plugin.Internal.Grpc (Transport (..))

-- | Eigenes Zertifikat (DER) mit Schlüssel.
data Identity = Identity
  { idCert :: SignedExact Certificate
  , idDer :: B.ByteString
  , idKey :: Ed.SecretKey
  }

newIdentity :: IO Identity
newIdentity = do
  sk <- Ed.generateSecretKey
  serialBytes <- getRandomBytes 16 :: IO B.ByteString
  now <- floor <$> getPOSIXTime :: IO Integer
  let pk = Ed.toPublic sk
      serial = B.foldl' (\acc w -> acc * 256 + fromIntegral w) 0 serialBytes
      at :: Integer -> DateTime
      at s = timeConvert (Elapsed (Seconds (fromIntegral s)))
      dn = DistinguishedName
        [ (getObjectID DnCommonName, ASN1CharacterString UTF8 "localhost")
        , (getObjectID DnOrganization, ASN1CharacterString UTF8 "CoreMesh")
        ]
      exts =
        Extensions $
          Just
            [ extensionEncode True (ExtBasicConstraints True Nothing)
            , extensionEncode True (ExtKeyUsage [KeyUsage_digitalSignature, KeyUsage_keyCertSign])
            , extensionEncode False (ExtExtendedKeyUsage [KeyUsagePurpose_ServerAuth, KeyUsagePurpose_ClientAuth])
            , extensionEncode False (ExtSubjectAltName [AltNameDNS "localhost"])
            ]
      alg = SignatureALG_IntrinsicHash PubKeyALG_Ed25519
      cert =
        Certificate
          { certVersion = 2
          , certSerial = serial
          , certSignatureAlg = alg
          , certIssuerDN = dn
          , certValidity = (at (now - 30), at (now + 30 * 365 * 24 * 3600))
          , certSubjectDN = dn
          , certPubKey = PubKeyEd25519 pk
          , certExtensions = exts
          }
      sign bs = (convert (Ed.sign sk pk bs), alg, ())
      (signed, ()) = objectToSignedExact sign cert
  pure Identity {idCert = signed, idDer = encodeSignedObject signed, idKey = sk}

-- | Zertifikat für die Handshake-Zeile (base64.RawStdEncoding).
certBase64 :: Identity -> B.ByteString
certBase64 = BC.takeWhile (/= '=') . B64.encode . idDer

-- | DER des ersten Zertifikats in einem PEM-Text (PLUGIN_CLIENT_CERT).
parsePemCert :: B.ByteString -> Either String B.ByteString
parsePemCert txt = do
  pems <- pemParseBS txt
  case [pemContent p | p <- pems, pemName p == "CERTIFICATE"] of
    der : _ -> Right der
    [] -> Left "kein CERTIFICATE im PEM"

credential :: Identity -> TLS.Credential
credential i = (CertificateChain [idCert i], PrivKeyEd25519 (idKey i))

-- | Prüft, dass die Gegenseite genau das erwartete Zertifikat vorlegt.
peerMatches :: B.ByteString -> CertificateChain -> Bool
peerMatches expected (CertificateChain (c : _)) = encodeSignedObject c == expected
peerMatches _ _ = False

supported :: TLS.Supported
supported =
  def
    { TLS.supportedVersions = [TLS.TLS13, TLS.TLS12]
    , TLS.supportedCiphers = TLS.ciphersuite_strong
    }

-- | Server-Seite (Host verbindet sich mit dem Plugin): Client-Zertifikat Pflicht.
serverTransport :: Identity -> B.ByteString -> Socket -> IO Transport
serverTransport me peerDer sock = do
  let params =
        def
          { TLS.serverWantClientCert = True
          , TLS.serverShared = def {TLS.sharedCredentials = TLS.Credentials [credential me]}
          , TLS.serverSupported = supported
          , TLS.serverHooks =
              def
                { TLS.onClientCertificate = \chain ->
                    pure $
                      if peerMatches peerDer chain
                        then TLS.CertificateUsageAccept
                        else TLS.CertificateUsageReject (TLS.CertificateRejectOther "unbekanntes Client-Zertifikat")
                , TLS.onALPNClientSuggest = Just $ \protos ->
                    pure (if "h2" `elem` protos then "h2" else B.empty)
                }
          }
  ctx <- TLS.contextNew sock params
  TLS.handshake ctx
  transport ctx sock

-- | Client-Seite (Plugin verbindet sich über den Broker mit dem Host).
clientTransport :: Identity -> B.ByteString -> Socket -> IO Transport
clientTransport me peerDer sock = do
  let params =
        (TLS.defaultParamsClient "localhost" B.empty)
          { TLS.clientSupported = supported
          , TLS.clientShared = def
          , TLS.clientHooks =
              def
                { TLS.onCertificateRequest = \_ -> pure (Just (credential me))
                , TLS.onServerCertificate = \_ _ _ chain ->
                    pure [UnknownCA | not (peerMatches peerDer chain)]
                , TLS.onSuggestALPN = pure (Just ["h2"])
                }
          }
  ctx <- TLS.contextNew sock params
  TLS.handshake ctx
  TLS.getNegotiatedProtocol ctx >>= \case
    Just "h2" -> pure ()
    other -> throwIO (userError ("Host hat kein h2 ausgehandelt: " <> show other))
  transport ctx sock

transport :: TLS.Context -> Socket -> IO Transport
transport ctx sock = do
  me <- getSocketName sock
  peer <- getPeerName sock
  pure
    Transport
      { tSend = TLS.sendData ctx . BL.fromStrict
      , tRecv = TLS.recvData ctx
      , tMyAddr = me
      , tPeerAddr = peer
      }
