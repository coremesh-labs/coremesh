{- This file was auto-generated from goplugin/grpc_stdio.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Goplugin.GrpcStdio (
        GRPCStdio(..), StdioData(), StdioData'Channel(..),
        StdioData'Channel(), StdioData'Channel'UnrecognizedValue
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
import qualified Proto.Google.Protobuf.Empty
{- | Fields :
     
         * 'Proto.Goplugin.GrpcStdio_Fields.channel' @:: Lens' StdioData StdioData'Channel@
         * 'Proto.Goplugin.GrpcStdio_Fields.data'' @:: Lens' StdioData Data.ByteString.ByteString@ -}
data StdioData
  = StdioData'_constructor {_StdioData'channel :: !StdioData'Channel,
                            _StdioData'data' :: !Data.ByteString.ByteString,
                            _StdioData'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show StdioData where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField StdioData "channel" StdioData'Channel where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _StdioData'channel (\ x__ y__ -> x__ {_StdioData'channel = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField StdioData "data'" Data.ByteString.ByteString where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _StdioData'data' (\ x__ y__ -> x__ {_StdioData'data' = y__}))
        Prelude.id
instance Data.ProtoLens.Message StdioData where
  messageName _ = Data.Text.pack "plugin.StdioData"
  packedMessageDescriptor _
    = "\n\
      \\tStdioData\DC23\n\
      \\achannel\CAN\SOH \SOH(\SO2\EM.plugin.StdioData.ChannelR\achannel\DC2\DC2\n\
      \\EOTdata\CAN\STX \SOH(\fR\EOTdata\".\n\
      \\aChannel\DC2\v\n\
      \\aINVALID\DLE\NUL\DC2\n\
      \\n\
      \\ACKSTDOUT\DLE\SOH\DC2\n\
      \\n\
      \\ACKSTDERR\DLE\STX"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        channel__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "channel"
              (Data.ProtoLens.ScalarField Data.ProtoLens.EnumField ::
                 Data.ProtoLens.FieldTypeDescriptor StdioData'Channel)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"channel")) ::
              Data.ProtoLens.FieldDescriptor StdioData
        data'__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "data"
              (Data.ProtoLens.ScalarField Data.ProtoLens.BytesField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.ByteString.ByteString)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"data'")) ::
              Data.ProtoLens.FieldDescriptor StdioData
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, channel__field_descriptor),
           (Data.ProtoLens.Tag 2, data'__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _StdioData'_unknownFields
        (\ x__ y__ -> x__ {_StdioData'_unknownFields = y__})
  defMessage
    = StdioData'_constructor
        {_StdioData'channel = Data.ProtoLens.fieldDefault,
         _StdioData'data' = Data.ProtoLens.fieldDefault,
         _StdioData'_unknownFields = []}
  parseMessage
    = let
        loop :: StdioData -> Data.ProtoLens.Encoding.Bytes.Parser StdioData
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
                                       "channel"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"channel") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getBytes
                                             (Prelude.fromIntegral len))
                                       "data"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"data'") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "StdioData"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"channel") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"data'") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData StdioData where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_StdioData'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_StdioData'channel x__)
                (Control.DeepSeq.deepseq (_StdioData'data' x__) ()))
newtype StdioData'Channel'UnrecognizedValue
  = StdioData'Channel'UnrecognizedValue Data.Int.Int32
  deriving stock (Prelude.Eq, Prelude.Ord, Prelude.Show)
data StdioData'Channel
  = StdioData'INVALID |
    StdioData'STDOUT |
    StdioData'STDERR |
    StdioData'Channel'Unrecognized !StdioData'Channel'UnrecognizedValue
  deriving stock (Prelude.Show, Prelude.Eq, Prelude.Ord)
instance Data.ProtoLens.MessageEnum StdioData'Channel where
  maybeToEnum 0 = Prelude.Just StdioData'INVALID
  maybeToEnum 1 = Prelude.Just StdioData'STDOUT
  maybeToEnum 2 = Prelude.Just StdioData'STDERR
  maybeToEnum k
    = Prelude.Just
        (StdioData'Channel'Unrecognized
           (StdioData'Channel'UnrecognizedValue (Prelude.fromIntegral k)))
  showEnum StdioData'INVALID = "INVALID"
  showEnum StdioData'STDOUT = "STDOUT"
  showEnum StdioData'STDERR = "STDERR"
  showEnum
    (StdioData'Channel'Unrecognized (StdioData'Channel'UnrecognizedValue k))
    = Prelude.show k
  readEnum k
    | (Prelude.==) k "INVALID" = Prelude.Just StdioData'INVALID
    | (Prelude.==) k "STDOUT" = Prelude.Just StdioData'STDOUT
    | (Prelude.==) k "STDERR" = Prelude.Just StdioData'STDERR
    | Prelude.otherwise
    = (Prelude.>>=) (Text.Read.readMaybe k) Data.ProtoLens.maybeToEnum
instance Prelude.Bounded StdioData'Channel where
  minBound = StdioData'INVALID
  maxBound = StdioData'STDERR
instance Prelude.Enum StdioData'Channel where
  toEnum k__
    = Prelude.maybe
        (Prelude.error
           ((Prelude.++)
              "toEnum: unknown value for enum Channel: " (Prelude.show k__)))
        Prelude.id (Data.ProtoLens.maybeToEnum k__)
  fromEnum StdioData'INVALID = 0
  fromEnum StdioData'STDOUT = 1
  fromEnum StdioData'STDERR = 2
  fromEnum
    (StdioData'Channel'Unrecognized (StdioData'Channel'UnrecognizedValue k))
    = Prelude.fromIntegral k
  succ StdioData'STDERR
    = Prelude.error
        "StdioData'Channel.succ: bad argument StdioData'STDERR. This value would be out of bounds."
  succ StdioData'INVALID = StdioData'STDOUT
  succ StdioData'STDOUT = StdioData'STDERR
  succ (StdioData'Channel'Unrecognized _)
    = Prelude.error
        "StdioData'Channel.succ: bad argument: unrecognized value"
  pred StdioData'INVALID
    = Prelude.error
        "StdioData'Channel.pred: bad argument StdioData'INVALID. This value would be out of bounds."
  pred StdioData'STDOUT = StdioData'INVALID
  pred StdioData'STDERR = StdioData'STDOUT
  pred (StdioData'Channel'Unrecognized _)
    = Prelude.error
        "StdioData'Channel.pred: bad argument: unrecognized value"
  enumFrom = Data.ProtoLens.Message.Enum.messageEnumFrom
  enumFromTo = Data.ProtoLens.Message.Enum.messageEnumFromTo
  enumFromThen = Data.ProtoLens.Message.Enum.messageEnumFromThen
  enumFromThenTo = Data.ProtoLens.Message.Enum.messageEnumFromThenTo
instance Data.ProtoLens.FieldDefault StdioData'Channel where
  fieldDefault = StdioData'INVALID
instance Control.DeepSeq.NFData StdioData'Channel where
  rnf x__ = Prelude.seq x__ ()
data GRPCStdio = GRPCStdio {}
instance Data.ProtoLens.Service.Types.Service GRPCStdio where
  type ServiceName GRPCStdio = "GRPCStdio"
  type ServicePackage GRPCStdio = "plugin"
  type ServiceMethods GRPCStdio = '["streamStdio"]
  packedServiceDescriptor _
    = "\n\
      \\tGRPCStdio\DC2:\n\
      \\vStreamStdio\DC2\SYN.google.protobuf.Empty\SUB\DC1.plugin.StdioData0\SOH"
instance Data.ProtoLens.Service.Types.HasMethodImpl GRPCStdio "streamStdio" where
  type MethodName GRPCStdio "streamStdio" = "StreamStdio"
  type MethodInput GRPCStdio "streamStdio" = Proto.Google.Protobuf.Empty.Empty
  type MethodOutput GRPCStdio "streamStdio" = StdioData
  type MethodStreamingType GRPCStdio "streamStdio" = 'Data.ProtoLens.Service.Types.ServerStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\EMgoplugin/grpc_stdio.proto\DC2\ACKplugin\SUB\ESCgoogle/protobuf/empty.proto\"\132\SOH\n\
    \\tStdioData\DC23\n\
    \\achannel\CAN\SOH \SOH(\SO2\EM.plugin.StdioData.ChannelR\achannel\DC2\DC2\n\
    \\EOTdata\CAN\STX \SOH(\fR\EOTdata\".\n\
    \\aChannel\DC2\v\n\
    \\aINVALID\DLE\NUL\DC2\n\
    \\n\
    \\ACKSTDOUT\DLE\SOH\DC2\n\
    \\n\
    \\ACKSTDERR\DLE\STX2G\n\
    \\tGRPCStdio\DC2:\n\
    \\vStreamStdio\DC2\SYN.google.protobuf.Empty\SUB\DC1.plugin.StdioData0\SOHB0Z.github.com/hashicorp/go-plugin/internal/pluginJ\247\a\n\
    \\ACK\DC2\EOT\ETX\NUL \SOH\n\
    \L\n\
    \\SOH\f\DC2\ETX\ETX\NUL\DC22B Copyright IBM Corp. 2016, 2025\n\
    \ SPDX-License-Identifier: MPL-2.0\n\
    \\n\
    \\b\n\
    \\SOH\STX\DC2\ETX\EOT\NUL\SI\n\
    \\b\n\
    \\SOH\b\DC2\ETX\ENQ\NULE\n\
    \\t\n\
    \\STX\b\v\DC2\ETX\ENQ\NULE\n\
    \\t\n\
    \\STX\ETX\NUL\DC2\ETX\a\NUL%\n\
    \\169\SOH\n\
    \\STX\ACK\NUL\DC2\EOT\f\NUL\DC3\SOH\SUB\156\SOH GRPCStdio is a service that is automatically run by the plugin process\n\
    \ to stream any stdout/err data so that it can be mirrored on the plugin\n\
    \ host side.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\f\b\DC1\n\
    \\251\SOH\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX\DC2\STXD\SUB\237\SOH StreamStdio returns a stream that contains all the stdout/stderr.\n\
    \ This RPC endpoint must only be called ONCE. Once stdio data is consumed\n\
    \ it is not sent again.\n\
    \\n\
    \ Callers should connect early to prevent blocking on the plugin process.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX\DC2\ACK\DC1\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX\DC2\DC2'\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ACK\DC2\ETX\DC228\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX\DC29B\n\
    \d\n\
    \\STX\EOT\NUL\DC2\EOT\ETB\NUL \SOH\SUBX StdioData is a single chunk of stdout or stderr data that is streamed\n\
    \ from GRPCStdio.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETX\ETB\b\DC1\n\
    \\f\n\
    \\EOT\EOT\NUL\EOT\NUL\DC2\EOT\CAN\STX\FS\ETX\n\
    \\f\n\
    \\ENQ\EOT\NUL\EOT\NUL\SOH\DC2\ETX\CAN\a\SO\n\
    \\r\n\
    \\ACK\EOT\NUL\EOT\NUL\STX\NUL\DC2\ETX\EM\EOT\DLE\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\NUL\SOH\DC2\ETX\EM\EOT\v\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\NUL\STX\DC2\ETX\EM\SO\SI\n\
    \\r\n\
    \\ACK\EOT\NUL\EOT\NUL\STX\SOH\DC2\ETX\SUB\EOT\SI\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\SOH\SOH\DC2\ETX\SUB\EOT\n\
    \\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\SOH\STX\DC2\ETX\SUB\r\SO\n\
    \\r\n\
    \\ACK\EOT\NUL\EOT\NUL\STX\STX\DC2\ETX\ESC\EOT\SI\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\STX\SOH\DC2\ETX\ESC\EOT\n\
    \\n\
    \\SO\n\
    \\a\EOT\NUL\EOT\NUL\STX\STX\STX\DC2\ETX\ESC\r\SO\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\NUL\DC2\ETX\RS\STX\SYN\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ACK\DC2\ETX\RS\STX\t\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\SOH\DC2\ETX\RS\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ETX\DC2\ETX\RS\DC4\NAK\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\SOH\DC2\ETX\US\STX\DC1\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ENQ\DC2\ETX\US\STX\a\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\SOH\DC2\ETX\US\b\f\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ETX\DC2\ETX\US\SI\DLEb\ACKproto3"