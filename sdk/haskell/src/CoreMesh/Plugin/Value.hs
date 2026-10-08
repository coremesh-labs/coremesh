-- | Umwandlung zwischen @google.protobuf.Value@ (Payload auf dem Draht) und
-- Aeson-Werten (Payload im Plugin) – wie @sdk.Decode@ bzw. @toValue@ im Go-SDK.
--
-- Zahlen sind auf dem Draht @double@: Ganzzahlen über 2^53 verlieren Stellen
-- und sollten als Text übertragen werden.
module CoreMesh.Plugin.Value
  ( toProto
  , fromProto
  , structToJson
  , jsonToStruct
  ) where

import Data.Aeson qualified as J
import Data.Aeson.Key qualified as K
import Data.Aeson.KeyMap qualified as KM
import Data.Map.Strict qualified as Map
import Data.ProtoLens (defMessage)
import Data.Scientific (fromFloatDigits, toRealFloat)
import Data.Vector qualified as V
import Lens.Family2 ((&), (.~), (^.))
import Proto.Google.Protobuf.Struct qualified as P
import Proto.Google.Protobuf.Struct_Fields qualified as P

toProto :: J.Value -> P.Value
toProto v = defMessage & P.maybe'kind .~ Just (kind v)
  where
    kind = \case
      J.Null -> P.Value'NullValue P.NULL_VALUE
      J.Bool b -> P.Value'BoolValue b
      J.Number n -> P.Value'NumberValue (toRealFloat n)
      J.String s -> P.Value'StringValue s
      J.Array xs -> P.Value'ListValue (defMessage & P.values .~ map toProto (V.toList xs))
      J.Object o -> P.Value'StructValue (jsonToStruct o)

fromProto :: P.Value -> J.Value
fromProto v = case v ^. P.maybe'kind of
  Nothing -> J.Null
  Just (P.Value'NullValue _) -> J.Null
  Just (P.Value'BoolValue b) -> J.Bool b
  Just (P.Value'NumberValue d) -> J.Number (fromFloatDigits d)
  Just (P.Value'StringValue s) -> J.String s
  Just (P.Value'ListValue l) -> J.Array (V.fromList (map fromProto (l ^. P.values)))
  Just (P.Value'StructValue s) -> J.Object (structToJson s)

structToJson :: P.Struct -> KM.KeyMap J.Value
structToJson s = KM.fromList [(K.fromText k, fromProto x) | (k, x) <- Map.toList (s ^. P.fields)]

jsonToStruct :: KM.KeyMap J.Value -> P.Struct
jsonToStruct o = defMessage & P.fields .~ Map.fromList [(K.toText k, toProto x) | (k, x) <- KM.toList o]
