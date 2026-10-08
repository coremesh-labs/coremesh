{- This file was auto-generated from grpc/health/v1/health.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Grpc.Health.V1.Health (
        Health(..), HealthCheckRequest(), HealthCheckResponse(),
        HealthCheckResponse'ServingStatus(..),
        HealthCheckResponse'ServingStatus(),
        HealthCheckResponse'ServingStatus'UnrecognizedValue
    ) where
import qualified Data.ProtoLens.Runtime.Control.DeepSeq as Control.DeepSeq
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Prism as Data.ProtoLens.Prism
import qualified Data.ProtoLens.Runtime.Prelude as Prelude
import qualified Data.ProtoLens.Runtime.Data.Int as Data.Int
import qualified Data.ProtoLens.Runtime.Data.Monoid as Data.Monoid
import qualified Data.ProtoLens.Runtime.Data.Word as Data.Word
import qualified Data.ProtoLens.Runtime.Data.ProtoLens as Data.ProtoLens
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Encoding.Bytes as Data.ProtoLens.Encoding.Bytes
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Encoding.Growing as Data.ProtoLens.Encoding.Growing
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Encoding.Parser.Unsafe as Data.ProtoLens.Encoding.Parser.Unsafe
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Encoding.Wire as Data.ProtoLens.Encoding.Wire
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Field as Data.ProtoLens.Field
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Message.Enum as Data.ProtoLens.Message.Enum
import qualified Data.ProtoLens.Runtime.Data.ProtoLens.Service.Types as Data.ProtoLens.Service.Types
import qualified Data.ProtoLens.Runtime.Lens.Family2 as Lens.Family2
import qualified Data.ProtoLens.Runtime.Lens.Family2.Unchecked as Lens.Family2.Unchecked
import qualified Data.ProtoLens.Runtime.Data.Text as Data.Text
import qualified Data.ProtoLens.Runtime.Data.Map as Data.Map
import qualified Data.ProtoLens.Runtime.Data.ByteString as Data.ByteString
import qualified Data.ProtoLens.Runtime.Data.ByteString.Char8 as Data.ByteString.Char8
import qualified Data.ProtoLens.Runtime.Data.Text.Encoding as Data.Text.Encoding
import qualified Data.ProtoLens.Runtime.Data.Vector as Data.Vector
import qualified Data.ProtoLens.Runtime.Data.Vector.Generic as Data.Vector.Generic
import qualified Data.ProtoLens.Runtime.Data.Vector.Unboxed as Data.Vector.Unboxed
import qualified Data.ProtoLens.Runtime.Text.Read as Text.Read
{- | Fields :
     
         * 'Proto.Grpc.Health.V1.Health_Fields.service' @:: Lens' HealthCheckRequest Data.Text.Text@ -}
data HealthCheckRequest
  = HealthCheckRequest'_constructor {_HealthCheckRequest'service :: !Data.Text.Text,
                                     _HealthCheckRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show HealthCheckRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField HealthCheckRequest "service" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HealthCheckRequest'service
           (\ x__ y__ -> x__ {_HealthCheckRequest'service = y__}))
        Prelude.id
instance Data.ProtoLens.Message HealthCheckRequest where
  messageName _ = Data.Text.pack "grpc.health.v1.HealthCheckRequest"
  packedMessageDescriptor _
    = "\n\
      \\DC2HealthCheckRequest\DC2\CAN\n\
      \\aservice\CAN\SOH \SOH(\tR\aservice"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        service__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "service"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"service")) ::
              Data.ProtoLens.FieldDescriptor HealthCheckRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, service__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _HealthCheckRequest'_unknownFields
        (\ x__ y__ -> x__ {_HealthCheckRequest'_unknownFields = y__})
  defMessage
    = HealthCheckRequest'_constructor
        {_HealthCheckRequest'service = Data.ProtoLens.fieldDefault,
         _HealthCheckRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          HealthCheckRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser HealthCheckRequest
        loop x
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do (let missing = []
                       in
                         if Prelude.null missing then
                             Prelude.return ()
                         else
                             Prelude.fail
                               ((Prelude.++)
                                  "Missing required fields: "
                                  (Prelude.show (missing :: [Prelude.String]))))
                      Prelude.return
                        (Lens.Family2.over
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t) x)
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "service"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"service") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "HealthCheckRequest"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"service") _x
              in
                if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                    Data.Monoid.mempty
                else
                    (Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                      ((Prelude..)
                         (\ bs
                            -> (Data.Monoid.<>)
                                 (Data.ProtoLens.Encoding.Bytes.putVarInt
                                    (Prelude.fromIntegral (Data.ByteString.length bs)))
                                 (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                         Data.Text.Encoding.encodeUtf8 _v))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData HealthCheckRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_HealthCheckRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq (_HealthCheckRequest'service x__) ())
{- | Fields :
     
         * 'Proto.Grpc.Health.V1.Health_Fields.status' @:: Lens' HealthCheckResponse HealthCheckResponse'ServingStatus@ -}
data HealthCheckResponse
  = HealthCheckResponse'_constructor {_HealthCheckResponse'status :: !HealthCheckResponse'ServingStatus,
                                      _HealthCheckResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show HealthCheckResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField HealthCheckResponse "status" HealthCheckResponse'ServingStatus where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HealthCheckResponse'status
           (\ x__ y__ -> x__ {_HealthCheckResponse'status = y__}))
        Prelude.id
instance Data.ProtoLens.Message HealthCheckResponse where
  messageName _ = Data.Text.pack "grpc.health.v1.HealthCheckResponse"
  packedMessageDescriptor _
    = "\n\
      \\DC3HealthCheckResponse\DC2I\n\
      \\ACKstatus\CAN\SOH \SOH(\SO21.grpc.health.v1.HealthCheckResponse.ServingStatusR\ACKstatus\"O\n\
      \\rServingStatus\DC2\v\n\
      \\aUNKNOWN\DLE\NUL\DC2\v\n\
      \\aSERVING\DLE\SOH\DC2\SI\n\
      \\vNOT_SERVING\DLE\STX\DC2\DC3\n\
      \\SISERVICE_UNKNOWN\DLE\ETX"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        status__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "status"
              (Data.ProtoLens.ScalarField Data.ProtoLens.EnumField ::
                 Data.ProtoLens.FieldTypeDescriptor HealthCheckResponse'ServingStatus)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"status")) ::
              Data.ProtoLens.FieldDescriptor HealthCheckResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, status__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _HealthCheckResponse'_unknownFields
        (\ x__ y__ -> x__ {_HealthCheckResponse'_unknownFields = y__})
  defMessage
    = HealthCheckResponse'_constructor
        {_HealthCheckResponse'status = Data.ProtoLens.fieldDefault,
         _HealthCheckResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          HealthCheckResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser HealthCheckResponse
        loop x
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do (let missing = []
                       in
                         if Prelude.null missing then
                             Prelude.return ()
                         else
                             Prelude.fail
                               ((Prelude.++)
                                  "Missing required fields: "
                                  (Prelude.show (missing :: [Prelude.String]))))
                      Prelude.return
                        (Lens.Family2.over
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t) x)
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        8 -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          Prelude.toEnum
                                          (Prelude.fmap
                                             Prelude.fromIntegral
                                             Data.ProtoLens.Encoding.Bytes.getVarInt))
                                       "status"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"status") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "HealthCheckResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"status") _x
              in
                if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                    Data.Monoid.mempty
                else
                    (Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.putVarInt 8)
                      ((Prelude..)
                         ((Prelude..)
                            Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral)
                         Prelude.fromEnum _v))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData HealthCheckResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_HealthCheckResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq (_HealthCheckResponse'status x__) ())
newtype HealthCheckResponse'ServingStatus'UnrecognizedValue
  = HealthCheckResponse'ServingStatus'UnrecognizedValue Data.Int.Int32
  deriving stock (Prelude.Eq, Prelude.Ord, Prelude.Show)
data HealthCheckResponse'ServingStatus
  = HealthCheckResponse'UNKNOWN |
    HealthCheckResponse'SERVING |
    HealthCheckResponse'NOT_SERVING |
    HealthCheckResponse'SERVICE_UNKNOWN |
    HealthCheckResponse'ServingStatus'Unrecognized !HealthCheckResponse'ServingStatus'UnrecognizedValue
  deriving stock (Prelude.Show, Prelude.Eq, Prelude.Ord)
instance Data.ProtoLens.MessageEnum HealthCheckResponse'ServingStatus where
  maybeToEnum 0 = Prelude.Just HealthCheckResponse'UNKNOWN
  maybeToEnum 1 = Prelude.Just HealthCheckResponse'SERVING
  maybeToEnum 2 = Prelude.Just HealthCheckResponse'NOT_SERVING
  maybeToEnum 3 = Prelude.Just HealthCheckResponse'SERVICE_UNKNOWN
  maybeToEnum k
    = Prelude.Just
        (HealthCheckResponse'ServingStatus'Unrecognized
           (HealthCheckResponse'ServingStatus'UnrecognizedValue
              (Prelude.fromIntegral k)))
  showEnum HealthCheckResponse'UNKNOWN = "UNKNOWN"
  showEnum HealthCheckResponse'SERVING = "SERVING"
  showEnum HealthCheckResponse'NOT_SERVING = "NOT_SERVING"
  showEnum HealthCheckResponse'SERVICE_UNKNOWN = "SERVICE_UNKNOWN"
  showEnum
    (HealthCheckResponse'ServingStatus'Unrecognized (HealthCheckResponse'ServingStatus'UnrecognizedValue k))
    = Prelude.show k
  readEnum k
    | (Prelude.==) k "UNKNOWN"
    = Prelude.Just HealthCheckResponse'UNKNOWN
    | (Prelude.==) k "SERVING"
    = Prelude.Just HealthCheckResponse'SERVING
    | (Prelude.==) k "NOT_SERVING"
    = Prelude.Just HealthCheckResponse'NOT_SERVING
    | (Prelude.==) k "SERVICE_UNKNOWN"
    = Prelude.Just HealthCheckResponse'SERVICE_UNKNOWN
    | Prelude.otherwise
    = (Prelude.>>=) (Text.Read.readMaybe k) Data.ProtoLens.maybeToEnum
instance Prelude.Bounded HealthCheckResponse'ServingStatus where
  minBound = HealthCheckResponse'UNKNOWN
  maxBound = HealthCheckResponse'SERVICE_UNKNOWN
instance Prelude.Enum HealthCheckResponse'ServingStatus where
  toEnum k__
    = Prelude.maybe
        (Prelude.error
           ((Prelude.++)
              "toEnum: unknown value for enum ServingStatus: "
              (Prelude.show k__)))
        Prelude.id (Data.ProtoLens.maybeToEnum k__)
  fromEnum HealthCheckResponse'UNKNOWN = 0
  fromEnum HealthCheckResponse'SERVING = 1
  fromEnum HealthCheckResponse'NOT_SERVING = 2
  fromEnum HealthCheckResponse'SERVICE_UNKNOWN = 3
  fromEnum
    (HealthCheckResponse'ServingStatus'Unrecognized (HealthCheckResponse'ServingStatus'UnrecognizedValue k))
    = Prelude.fromIntegral k
  succ HealthCheckResponse'SERVICE_UNKNOWN
    = Prelude.error
        "HealthCheckResponse'ServingStatus.succ: bad argument HealthCheckResponse'SERVICE_UNKNOWN. This value would be out of bounds."
  succ HealthCheckResponse'UNKNOWN = HealthCheckResponse'SERVING
  succ HealthCheckResponse'SERVING = HealthCheckResponse'NOT_SERVING
  succ HealthCheckResponse'NOT_SERVING
    = HealthCheckResponse'SERVICE_UNKNOWN
  succ (HealthCheckResponse'ServingStatus'Unrecognized _)
    = Prelude.error
        "HealthCheckResponse'ServingStatus.succ: bad argument: unrecognized value"
  pred HealthCheckResponse'UNKNOWN
    = Prelude.error
        "HealthCheckResponse'ServingStatus.pred: bad argument HealthCheckResponse'UNKNOWN. This value would be out of bounds."
  pred HealthCheckResponse'SERVING = HealthCheckResponse'UNKNOWN
  pred HealthCheckResponse'NOT_SERVING = HealthCheckResponse'SERVING
  pred HealthCheckResponse'SERVICE_UNKNOWN
    = HealthCheckResponse'NOT_SERVING
  pred (HealthCheckResponse'ServingStatus'Unrecognized _)
    = Prelude.error
        "HealthCheckResponse'ServingStatus.pred: bad argument: unrecognized value"
  enumFrom = Data.ProtoLens.Message.Enum.messageEnumFrom
  enumFromTo = Data.ProtoLens.Message.Enum.messageEnumFromTo
  enumFromThen = Data.ProtoLens.Message.Enum.messageEnumFromThen
  enumFromThenTo = Data.ProtoLens.Message.Enum.messageEnumFromThenTo
instance Data.ProtoLens.FieldDefault HealthCheckResponse'ServingStatus where
  fieldDefault = HealthCheckResponse'UNKNOWN
instance Control.DeepSeq.NFData HealthCheckResponse'ServingStatus where
  rnf x__ = Prelude.seq x__ ()
data Health = Health {}
instance Data.ProtoLens.Service.Types.Service Health where
  type ServiceName Health = "Health"
  type ServicePackage Health = "grpc.health.v1"
  type ServiceMethods Health = '["check"]
  packedServiceDescriptor _
    = "\n\
      \\ACKHealth\DC2P\n\
      \\ENQCheck\DC2\".grpc.health.v1.HealthCheckRequest\SUB#.grpc.health.v1.HealthCheckResponse"
instance Data.ProtoLens.Service.Types.HasMethodImpl Health "check" where
  type MethodName Health "check" = "Check"
  type MethodInput Health "check" = HealthCheckRequest
  type MethodOutput Health "check" = HealthCheckResponse
  type MethodStreamingType Health "check" = 'Data.ProtoLens.Service.Types.NonStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\ESCgrpc/health/v1/health.proto\DC2\SOgrpc.health.v1\".\n\
    \\DC2HealthCheckRequest\DC2\CAN\n\
    \\aservice\CAN\SOH \SOH(\tR\aservice\"\177\SOH\n\
    \\DC3HealthCheckResponse\DC2I\n\
    \\ACKstatus\CAN\SOH \SOH(\SO21.grpc.health.v1.HealthCheckResponse.ServingStatusR\ACKstatus\"O\n\
    \\rServingStatus\DC2\v\n\
    \\aUNKNOWN\DLE\NUL\DC2\v\n\
    \\aSERVING\DLE\SOH\DC2\SI\n\
    \\vNOT_SERVING\DLE\STX\DC2\DC3\n\
    \\SISERVICE_UNKNOWN\DLE\ETX2Z\n\
    \\ACKHealth\DC2P\n\
    \\ENQCheck\DC2\".grpc.health.v1.HealthCheckRequest\SUB#.grpc.health.v1.HealthCheckResponseJ\161\ENQ\n\
    \\ACK\DC2\EOT\STX\NUL\SYN\SOH\n\
    \\199\SOH\n\
    \\SOH\f\DC2\ETX\STX\NUL\DC2\SUB\188\SOH gRPC Health Checking Protocol (https://github.com/grpc/grpc/blob/master/doc/health-checking.md),\n\
    \ nur die Teile, die go-plugin nutzt. Copyright 2015 The gRPC Authors, Apache License 2.0.\n\
    \\n\
    \\b\n\
    \\SOH\STX\DC2\ETX\EOT\NUL\ETB\n\
    \\n\
    \\n\
    \\STX\EOT\NUL\DC2\EOT\ACK\NUL\b\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETX\ACK\b\SUB\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\NUL\DC2\ETX\a\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ENQ\DC2\ETX\a\STX\b\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\SOH\DC2\ETX\a\t\DLE\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ETX\DC2\ETX\a\DC3\DC4\n\
    \\n\
    \\n\
    \\STX\EOT\SOH\DC2\EOT\n\
    \\NUL\DC2\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\SOH\SOH\DC2\ETX\n\
    \\b\ESC\n\
    \\f\n\
    \\EOT\EOT\SOH\EOT\NUL\DC2\EOT\v\STX\DLE\ETX\n\
    \\f\n\
    \\ENQ\EOT\SOH\EOT\NUL\SOH\DC2\ETX\v\a\DC4\n\
    \\r\n\
    \\ACK\EOT\SOH\EOT\NUL\STX\NUL\DC2\ETX\f\EOT\DLE\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\NUL\SOH\DC2\ETX\f\EOT\v\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\NUL\STX\DC2\ETX\f\SO\SI\n\
    \\r\n\
    \\ACK\EOT\SOH\EOT\NUL\STX\SOH\DC2\ETX\r\EOT\DLE\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\SOH\SOH\DC2\ETX\r\EOT\v\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\SOH\STX\DC2\ETX\r\SO\SI\n\
    \\r\n\
    \\ACK\EOT\SOH\EOT\NUL\STX\STX\DC2\ETX\SO\EOT\DC4\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\STX\SOH\DC2\ETX\SO\EOT\SI\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\STX\STX\DC2\ETX\SO\DC2\DC3\n\
    \\r\n\
    \\ACK\EOT\SOH\EOT\NUL\STX\ETX\DC2\ETX\SI\EOT\CAN\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\ETX\SOH\DC2\ETX\SI\EOT\DC3\n\
    \\SO\n\
    \\a\EOT\SOH\EOT\NUL\STX\ETX\STX\DC2\ETX\SI\SYN\ETB\n\
    \\v\n\
    \\EOT\EOT\SOH\STX\NUL\DC2\ETX\DC1\STX\ESC\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\ACK\DC2\ETX\DC1\STX\SI\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\SOH\DC2\ETX\DC1\DLE\SYN\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\ETX\DC2\ETX\DC1\EM\SUB\n\
    \\n\
    \\n\
    \\STX\ACK\NUL\DC2\EOT\DC4\NUL\SYN\SOH\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\DC4\b\SO\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX\NAK\STX>\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX\NAK\ACK\v\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX\NAK\f\RS\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX\NAK)<b\ACKproto3"