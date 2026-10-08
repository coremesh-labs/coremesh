{- This file was auto-generated from goplugin/grpc_controller.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Goplugin.GrpcController (
        GRPCController(..), Empty()
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
      -}
data Empty
  = Empty'_constructor {_Empty'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Empty where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Message Empty where
  messageName _ = Data.Text.pack "plugin.Empty"
  packedMessageDescriptor _
    = "\n\
      \\ENQEmpty"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag = let in Data.Map.fromList []
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Empty'_unknownFields
        (\ x__ y__ -> x__ {_Empty'_unknownFields = y__})
  defMessage = Empty'_constructor {_Empty'_unknownFields = []}
  parseMessage
    = let
        loop :: Empty -> Data.ProtoLens.Encoding.Bytes.Parser Empty
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
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "Empty"
  buildMessage
    = \ _x
        -> Data.ProtoLens.Encoding.Wire.buildFieldSet
             (Lens.Family2.view Data.ProtoLens.unknownFields _x)
instance Control.DeepSeq.NFData Empty where
  rnf
    = \ x__ -> Control.DeepSeq.deepseq (_Empty'_unknownFields x__) ()
data GRPCController = GRPCController {}
instance Data.ProtoLens.Service.Types.Service GRPCController where
  type ServiceName GRPCController = "GRPCController"
  type ServicePackage GRPCController = "plugin"
  type ServiceMethods GRPCController = '["shutdown"]
  packedServiceDescriptor _
    = "\n\
      \\SOGRPCController\DC2(\n\
      \\bShutdown\DC2\r.plugin.Empty\SUB\r.plugin.Empty"
instance Data.ProtoLens.Service.Types.HasMethodImpl GRPCController "shutdown" where
  type MethodName GRPCController "shutdown" = "Shutdown"
  type MethodInput GRPCController "shutdown" = Empty
  type MethodOutput GRPCController "shutdown" = Empty
  type MethodStreamingType GRPCController "shutdown" = 'Data.ProtoLens.Service.Types.NonStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\RSgoplugin/grpc_controller.proto\DC2\ACKplugin\"\a\n\
    \\ENQEmpty2:\n\
    \\SOGRPCController\DC2(\n\
    \\bShutdown\DC2\r.plugin.Empty\SUB\r.plugin.EmptyB0Z.github.com/hashicorp/go-plugin/internal/pluginJ\172\STX\n\
    \\ACK\DC2\EOT\ETX\NUL\r\SOH\n\
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
    \\n\
    \\n\
    \\STX\EOT\NUL\DC2\EOT\a\NUL\b\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETX\a\b\r\n\
    \Z\n\
    \\STX\ACK\NUL\DC2\EOT\v\NUL\r\SOH\SUBN The GRPCController is responsible for telling the plugin server to shutdown.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\v\b\SYN\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX\f\EOT(\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX\f\b\DLE\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX\f\DC1\SYN\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX\f!&b\ACKproto3"