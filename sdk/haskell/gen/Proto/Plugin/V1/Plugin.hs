{- This file was auto-generated from plugin/v1/plugin.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Plugin.V1.Plugin (
        PluginService(..), Capability(), ConfigureRequest(),
        ConfigureResponse(), Context(), Context'MetadataEntry(),
        Context'TxIdsEntry(), GetManifestRequest(), GetManifestResponse(),
        HandleRequest(), HandleResponse(), Manifest(), ReadBatch(),
        ReadEnd(), ReadEnd'MetadataEntry(), ReadHeader(),
        ReadHeader'MetadataEntry(), ReadResponse(), ReadResponse'Part(..),
        _ReadResponse'Header, _ReadResponse'Batch, _ReadResponse'End,
        ReadRow()
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
import qualified Proto.Google.Protobuf.Struct
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.object' @:: Lens' Capability Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.actions' @:: Lens' Capability [Data.Text.Text]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'actions' @:: Lens' Capability (Data.Vector.Vector Data.Text.Text)@
         * 'Proto.Plugin.V1.Plugin_Fields.description' @:: Lens' Capability Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.readActions' @:: Lens' Capability [Data.Text.Text]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'readActions' @:: Lens' Capability (Data.Vector.Vector Data.Text.Text)@ -}
data Capability
  = Capability'_constructor {_Capability'object :: !Data.Text.Text,
                             _Capability'actions :: !(Data.Vector.Vector Data.Text.Text),
                             _Capability'description :: !Data.Text.Text,
                             _Capability'readActions :: !(Data.Vector.Vector Data.Text.Text),
                             _Capability'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Capability where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Capability "object" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'object (\ x__ y__ -> x__ {_Capability'object = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Capability "actions" [Data.Text.Text] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'actions (\ x__ y__ -> x__ {_Capability'actions = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField Capability "vec'actions" (Data.Vector.Vector Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'actions (\ x__ y__ -> x__ {_Capability'actions = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Capability "description" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'description
           (\ x__ y__ -> x__ {_Capability'description = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Capability "readActions" [Data.Text.Text] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'readActions
           (\ x__ y__ -> x__ {_Capability'readActions = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField Capability "vec'readActions" (Data.Vector.Vector Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Capability'readActions
           (\ x__ y__ -> x__ {_Capability'readActions = y__}))
        Prelude.id
instance Data.ProtoLens.Message Capability where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.Capability"
  packedMessageDescriptor _
    = "\n\
      \\n\
      \Capability\DC2\SYN\n\
      \\ACKobject\CAN\SOH \SOH(\tR\ACKobject\DC2\CAN\n\
      \\aactions\CAN\STX \ETX(\tR\aactions\DC2 \n\
      \\vdescription\CAN\ETX \SOH(\tR\vdescription\DC2!\n\
      \\fread_actions\CAN\EOT \ETX(\tR\vreadActions"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        object__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "object"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"object")) ::
              Data.ProtoLens.FieldDescriptor Capability
        actions__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "actions"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"actions")) ::
              Data.ProtoLens.FieldDescriptor Capability
        description__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "description"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"description")) ::
              Data.ProtoLens.FieldDescriptor Capability
        readActions__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "read_actions"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked
                 (Data.ProtoLens.Field.field @"readActions")) ::
              Data.ProtoLens.FieldDescriptor Capability
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, object__field_descriptor),
           (Data.ProtoLens.Tag 2, actions__field_descriptor),
           (Data.ProtoLens.Tag 3, description__field_descriptor),
           (Data.ProtoLens.Tag 4, readActions__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Capability'_unknownFields
        (\ x__ y__ -> x__ {_Capability'_unknownFields = y__})
  defMessage
    = Capability'_constructor
        {_Capability'object = Data.ProtoLens.fieldDefault,
         _Capability'actions = Data.Vector.Generic.empty,
         _Capability'description = Data.ProtoLens.fieldDefault,
         _Capability'readActions = Data.Vector.Generic.empty,
         _Capability'_unknownFields = []}
  parseMessage
    = let
        loop ::
          Capability
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Data.Text.Text
             -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Data.Text.Text
                -> Data.ProtoLens.Encoding.Bytes.Parser Capability
        loop x mutable'actions mutable'readActions
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'actions <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                          (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                             mutable'actions)
                      frozen'readActions <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                              (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                                 mutable'readActions)
                      (let missing = []
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
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t)
                           (Lens.Family2.set
                              (Data.ProtoLens.Field.field @"vec'actions") frozen'actions
                              (Lens.Family2.set
                                 (Data.ProtoLens.Field.field @"vec'readActions") frozen'readActions
                                 x)))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "object"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"object") y x)
                                  mutable'actions mutable'readActions
                        18
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.getText
                                              (Prelude.fromIntegral len))
                                        "actions"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'actions y)
                                loop x v mutable'readActions
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "description"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"description") y x)
                                  mutable'actions mutable'readActions
                        34
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.getText
                                              (Prelude.fromIntegral len))
                                        "read_actions"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append
                                          mutable'readActions y)
                                loop x mutable'actions v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'actions mutable'readActions
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'actions <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                   Data.ProtoLens.Encoding.Growing.new
              mutable'readActions <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'actions mutable'readActions)
          "Capability"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"object") _x
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
             ((Data.Monoid.<>)
                (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                   (\ _v
                      -> (Data.Monoid.<>)
                           (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                           ((Prelude..)
                              (\ bs
                                 -> (Data.Monoid.<>)
                                      (Data.ProtoLens.Encoding.Bytes.putVarInt
                                         (Prelude.fromIntegral (Data.ByteString.length bs)))
                                      (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                              Data.Text.Encoding.encodeUtf8 _v))
                   (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'actions") _x))
                ((Data.Monoid.<>)
                   (let
                      _v
                        = Lens.Family2.view (Data.ProtoLens.Field.field @"description") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                            ((Prelude..)
                               (\ bs
                                  -> (Data.Monoid.<>)
                                       (Data.ProtoLens.Encoding.Bytes.putVarInt
                                          (Prelude.fromIntegral (Data.ByteString.length bs)))
                                       (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                               Data.Text.Encoding.encodeUtf8 _v))
                   ((Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                         (\ _v
                            -> (Data.Monoid.<>)
                                 (Data.ProtoLens.Encoding.Bytes.putVarInt 34)
                                 ((Prelude..)
                                    (\ bs
                                       -> (Data.Monoid.<>)
                                            (Data.ProtoLens.Encoding.Bytes.putVarInt
                                               (Prelude.fromIntegral (Data.ByteString.length bs)))
                                            (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                    Data.Text.Encoding.encodeUtf8 _v))
                         (Lens.Family2.view
                            (Data.ProtoLens.Field.field @"vec'readActions") _x))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData Capability where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Capability'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_Capability'object x__)
                (Control.DeepSeq.deepseq
                   (_Capability'actions x__)
                   (Control.DeepSeq.deepseq
                      (_Capability'description x__)
                      (Control.DeepSeq.deepseq (_Capability'readActions x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.context' @:: Lens' ConfigureRequest Context@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'context' @:: Lens' ConfigureRequest (Prelude.Maybe Context)@
         * 'Proto.Plugin.V1.Plugin_Fields.settings' @:: Lens' ConfigureRequest Proto.Google.Protobuf.Struct.Struct@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'settings' @:: Lens' ConfigureRequest (Prelude.Maybe Proto.Google.Protobuf.Struct.Struct)@
         * 'Proto.Plugin.V1.Plugin_Fields.hostServiceBrokerId' @:: Lens' ConfigureRequest Data.Word.Word32@ -}
data ConfigureRequest
  = ConfigureRequest'_constructor {_ConfigureRequest'context :: !(Prelude.Maybe Context),
                                   _ConfigureRequest'settings :: !(Prelude.Maybe Proto.Google.Protobuf.Struct.Struct),
                                   _ConfigureRequest'hostServiceBrokerId :: !Data.Word.Word32,
                                   _ConfigureRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ConfigureRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ConfigureRequest "context" Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConfigureRequest'context
           (\ x__ y__ -> x__ {_ConfigureRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField ConfigureRequest "maybe'context" (Prelude.Maybe Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConfigureRequest'context
           (\ x__ y__ -> x__ {_ConfigureRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConfigureRequest "settings" Proto.Google.Protobuf.Struct.Struct where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConfigureRequest'settings
           (\ x__ y__ -> x__ {_ConfigureRequest'settings = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField ConfigureRequest "maybe'settings" (Prelude.Maybe Proto.Google.Protobuf.Struct.Struct) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConfigureRequest'settings
           (\ x__ y__ -> x__ {_ConfigureRequest'settings = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConfigureRequest "hostServiceBrokerId" Data.Word.Word32 where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConfigureRequest'hostServiceBrokerId
           (\ x__ y__ -> x__ {_ConfigureRequest'hostServiceBrokerId = y__}))
        Prelude.id
instance Data.ProtoLens.Message ConfigureRequest where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.ConfigureRequest"
  packedMessageDescriptor _
    = "\n\
      \\DLEConfigureRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC23\n\
      \\bsettings\CAN\STX \SOH(\v2\ETB.google.protobuf.StructR\bsettings\DC23\n\
      \\SYNhost_service_broker_id\CAN\ETX \SOH(\rR\DC3hostServiceBrokerId"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor ConfigureRequest
        settings__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "settings"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Struct)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'settings")) ::
              Data.ProtoLens.FieldDescriptor ConfigureRequest
        hostServiceBrokerId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "host_service_broker_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.UInt32Field ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Word.Word32)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"hostServiceBrokerId")) ::
              Data.ProtoLens.FieldDescriptor ConfigureRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, settings__field_descriptor),
           (Data.ProtoLens.Tag 3, hostServiceBrokerId__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ConfigureRequest'_unknownFields
        (\ x__ y__ -> x__ {_ConfigureRequest'_unknownFields = y__})
  defMessage
    = ConfigureRequest'_constructor
        {_ConfigureRequest'context = Prelude.Nothing,
         _ConfigureRequest'settings = Prelude.Nothing,
         _ConfigureRequest'hostServiceBrokerId = Data.ProtoLens.fieldDefault,
         _ConfigureRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ConfigureRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser ConfigureRequest
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "settings"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"settings") y x)
                        24
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          Prelude.fromIntegral
                                          Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "host_service_broker_id"
                                loop
                                  (Lens.Family2.set
                                     (Data.ProtoLens.Field.field @"hostServiceBrokerId") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "ConfigureRequest"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'context") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             ((Data.Monoid.<>)
                (case
                     Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'settings") _x
                 of
                   Prelude.Nothing -> Data.Monoid.mempty
                   (Prelude.Just _v)
                     -> (Data.Monoid.<>)
                          (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                          ((Prelude..)
                             (\ bs
                                -> (Data.Monoid.<>)
                                     (Data.ProtoLens.Encoding.Bytes.putVarInt
                                        (Prelude.fromIntegral (Data.ByteString.length bs)))
                                     (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                             Data.ProtoLens.encodeMessage _v))
                ((Data.Monoid.<>)
                   (let
                      _v
                        = Lens.Family2.view
                            (Data.ProtoLens.Field.field @"hostServiceBrokerId") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 24)
                            ((Prelude..)
                               Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral _v))
                   (Data.ProtoLens.Encoding.Wire.buildFieldSet
                      (Lens.Family2.view Data.ProtoLens.unknownFields _x))))
instance Control.DeepSeq.NFData ConfigureRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ConfigureRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ConfigureRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_ConfigureRequest'settings x__)
                   (Control.DeepSeq.deepseq
                      (_ConfigureRequest'hostServiceBrokerId x__) ())))
{- | Fields :
      -}
data ConfigureResponse
  = ConfigureResponse'_constructor {_ConfigureResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ConfigureResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Message ConfigureResponse where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.ConfigureResponse"
  packedMessageDescriptor _
    = "\n\
      \\DC1ConfigureResponse"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag = let in Data.Map.fromList []
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ConfigureResponse'_unknownFields
        (\ x__ y__ -> x__ {_ConfigureResponse'_unknownFields = y__})
  defMessage
    = ConfigureResponse'_constructor
        {_ConfigureResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ConfigureResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser ConfigureResponse
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
          (do loop Data.ProtoLens.defMessage) "ConfigureResponse"
  buildMessage
    = \ _x
        -> Data.ProtoLens.Encoding.Wire.buildFieldSet
             (Lens.Family2.view Data.ProtoLens.unknownFields _x)
instance Control.DeepSeq.NFData ConfigureResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ConfigureResponse'_unknownFields x__) ()
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.requestId' @:: Lens' Context Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.tenantId' @:: Lens' Context Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.userId' @:: Lens' Context Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.metadata' @:: Lens' Context (Data.Map.Map Data.Text.Text Data.Text.Text)@
         * 'Proto.Plugin.V1.Plugin_Fields.txIds' @:: Lens' Context (Data.Map.Map Data.Text.Text Data.Text.Text)@ -}
data Context
  = Context'_constructor {_Context'requestId :: !Data.Text.Text,
                          _Context'tenantId :: !Data.Text.Text,
                          _Context'userId :: !Data.Text.Text,
                          _Context'metadata :: !(Data.Map.Map Data.Text.Text Data.Text.Text),
                          _Context'txIds :: !(Data.Map.Map Data.Text.Text Data.Text.Text),
                          _Context'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Context where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Context "requestId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'requestId (\ x__ y__ -> x__ {_Context'requestId = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context "tenantId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'tenantId (\ x__ y__ -> x__ {_Context'tenantId = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context "userId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'userId (\ x__ y__ -> x__ {_Context'userId = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context "metadata" (Data.Map.Map Data.Text.Text Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'metadata (\ x__ y__ -> x__ {_Context'metadata = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context "txIds" (Data.Map.Map Data.Text.Text Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'txIds (\ x__ y__ -> x__ {_Context'txIds = y__}))
        Prelude.id
instance Data.ProtoLens.Message Context where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.Context"
  packedMessageDescriptor _
    = "\n\
      \\aContext\DC2\GS\n\
      \\n\
      \request_id\CAN\SOH \SOH(\tR\trequestId\DC2\ESC\n\
      \\ttenant_id\CAN\STX \SOH(\tR\btenantId\DC2\ETB\n\
      \\auser_id\CAN\ETX \SOH(\tR\ACKuserId\DC2E\n\
      \\bmetadata\CAN\EOT \ETX(\v2).coremesh.plugin.v1.Context.MetadataEntryR\bmetadata\DC2=\n\
      \\ACKtx_ids\CAN\ENQ \ETX(\v2&.coremesh.plugin.v1.Context.TxIdsEntryR\ENQtxIds\SUB;\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH\SUB8\n\
      \\n\
      \TxIdsEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        requestId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "request_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"requestId")) ::
              Data.ProtoLens.FieldDescriptor Context
        tenantId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "tenant_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"tenantId")) ::
              Data.ProtoLens.FieldDescriptor Context
        userId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "user_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"userId")) ::
              Data.ProtoLens.FieldDescriptor Context
        metadata__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "metadata"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context'MetadataEntry)
              (Data.ProtoLens.MapField
                 (Data.ProtoLens.Field.field @"key")
                 (Data.ProtoLens.Field.field @"value")
                 (Data.ProtoLens.Field.field @"metadata")) ::
              Data.ProtoLens.FieldDescriptor Context
        txIds__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "tx_ids"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context'TxIdsEntry)
              (Data.ProtoLens.MapField
                 (Data.ProtoLens.Field.field @"key")
                 (Data.ProtoLens.Field.field @"value")
                 (Data.ProtoLens.Field.field @"txIds")) ::
              Data.ProtoLens.FieldDescriptor Context
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, requestId__field_descriptor),
           (Data.ProtoLens.Tag 2, tenantId__field_descriptor),
           (Data.ProtoLens.Tag 3, userId__field_descriptor),
           (Data.ProtoLens.Tag 4, metadata__field_descriptor),
           (Data.ProtoLens.Tag 5, txIds__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Context'_unknownFields
        (\ x__ y__ -> x__ {_Context'_unknownFields = y__})
  defMessage
    = Context'_constructor
        {_Context'requestId = Data.ProtoLens.fieldDefault,
         _Context'tenantId = Data.ProtoLens.fieldDefault,
         _Context'userId = Data.ProtoLens.fieldDefault,
         _Context'metadata = Data.Map.empty,
         _Context'txIds = Data.Map.empty, _Context'_unknownFields = []}
  parseMessage
    = let
        loop :: Context -> Data.ProtoLens.Encoding.Bytes.Parser Context
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
                                       "request_id"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"requestId") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "tenant_id"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"tenantId") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "user_id"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"userId") y x)
                        34
                          -> do !(entry :: Context'MetadataEntry) <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                                                           Data.ProtoLens.Encoding.Bytes.isolate
                                                                             (Prelude.fromIntegral
                                                                                len)
                                                                             Data.ProtoLens.parseMessage)
                                                                       "metadata"
                                (let
                                   key = Lens.Family2.view (Data.ProtoLens.Field.field @"key") entry
                                   value
                                     = Lens.Family2.view (Data.ProtoLens.Field.field @"value") entry
                                 in
                                   loop
                                     (Lens.Family2.over
                                        (Data.ProtoLens.Field.field @"metadata")
                                        (\ !t -> Data.Map.insert key value t) x))
                        42
                          -> do !(entry :: Context'TxIdsEntry) <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                                                    (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                                                        Data.ProtoLens.Encoding.Bytes.isolate
                                                                          (Prelude.fromIntegral len)
                                                                          Data.ProtoLens.parseMessage)
                                                                    "tx_ids"
                                (let
                                   key = Lens.Family2.view (Data.ProtoLens.Field.field @"key") entry
                                   value
                                     = Lens.Family2.view (Data.ProtoLens.Field.field @"value") entry
                                 in
                                   loop
                                     (Lens.Family2.over
                                        (Data.ProtoLens.Field.field @"txIds")
                                        (\ !t -> Data.Map.insert key value t) x))
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "Context"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"requestId") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"tenantId") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                ((Data.Monoid.<>)
                   (let
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"userId") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                            ((Prelude..)
                               (\ bs
                                  -> (Data.Monoid.<>)
                                       (Data.ProtoLens.Encoding.Bytes.putVarInt
                                          (Prelude.fromIntegral (Data.ByteString.length bs)))
                                       (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                               Data.Text.Encoding.encodeUtf8 _v))
                   ((Data.Monoid.<>)
                      (Data.Monoid.mconcat
                         (Prelude.map
                            (\ _v
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt 34)
                                    ((Prelude..)
                                       (\ bs
                                          -> (Data.Monoid.<>)
                                               (Data.ProtoLens.Encoding.Bytes.putVarInt
                                                  (Prelude.fromIntegral
                                                     (Data.ByteString.length bs)))
                                               (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                       Data.ProtoLens.encodeMessage
                                       (Lens.Family2.set
                                          (Data.ProtoLens.Field.field @"key") (Prelude.fst _v)
                                          (Lens.Family2.set
                                             (Data.ProtoLens.Field.field @"value") (Prelude.snd _v)
                                             (Data.ProtoLens.defMessage ::
                                                Context'MetadataEntry)))))
                            (Data.Map.toList
                               (Lens.Family2.view (Data.ProtoLens.Field.field @"metadata") _x))))
                      ((Data.Monoid.<>)
                         (Data.Monoid.mconcat
                            (Prelude.map
                               (\ _v
                                  -> (Data.Monoid.<>)
                                       (Data.ProtoLens.Encoding.Bytes.putVarInt 42)
                                       ((Prelude..)
                                          (\ bs
                                             -> (Data.Monoid.<>)
                                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                                     (Prelude.fromIntegral
                                                        (Data.ByteString.length bs)))
                                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                          Data.ProtoLens.encodeMessage
                                          (Lens.Family2.set
                                             (Data.ProtoLens.Field.field @"key") (Prelude.fst _v)
                                             (Lens.Family2.set
                                                (Data.ProtoLens.Field.field @"value")
                                                (Prelude.snd _v)
                                                (Data.ProtoLens.defMessage ::
                                                   Context'TxIdsEntry)))))
                               (Data.Map.toList
                                  (Lens.Family2.view (Data.ProtoLens.Field.field @"txIds") _x))))
                         (Data.ProtoLens.Encoding.Wire.buildFieldSet
                            (Lens.Family2.view Data.ProtoLens.unknownFields _x))))))
instance Control.DeepSeq.NFData Context where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Context'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_Context'requestId x__)
                (Control.DeepSeq.deepseq
                   (_Context'tenantId x__)
                   (Control.DeepSeq.deepseq
                      (_Context'userId x__)
                      (Control.DeepSeq.deepseq
                         (_Context'metadata x__)
                         (Control.DeepSeq.deepseq (_Context'txIds x__) ())))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.key' @:: Lens' Context'MetadataEntry Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.value' @:: Lens' Context'MetadataEntry Data.Text.Text@ -}
data Context'MetadataEntry
  = Context'MetadataEntry'_constructor {_Context'MetadataEntry'key :: !Data.Text.Text,
                                        _Context'MetadataEntry'value :: !Data.Text.Text,
                                        _Context'MetadataEntry'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Context'MetadataEntry where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Context'MetadataEntry "key" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'MetadataEntry'key
           (\ x__ y__ -> x__ {_Context'MetadataEntry'key = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context'MetadataEntry "value" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'MetadataEntry'value
           (\ x__ y__ -> x__ {_Context'MetadataEntry'value = y__}))
        Prelude.id
instance Data.ProtoLens.Message Context'MetadataEntry where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.Context.MetadataEntry"
  packedMessageDescriptor _
    = "\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        key__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "key"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"key")) ::
              Data.ProtoLens.FieldDescriptor Context'MetadataEntry
        value__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "value"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"value")) ::
              Data.ProtoLens.FieldDescriptor Context'MetadataEntry
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, key__field_descriptor),
           (Data.ProtoLens.Tag 2, value__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Context'MetadataEntry'_unknownFields
        (\ x__ y__ -> x__ {_Context'MetadataEntry'_unknownFields = y__})
  defMessage
    = Context'MetadataEntry'_constructor
        {_Context'MetadataEntry'key = Data.ProtoLens.fieldDefault,
         _Context'MetadataEntry'value = Data.ProtoLens.fieldDefault,
         _Context'MetadataEntry'_unknownFields = []}
  parseMessage
    = let
        loop ::
          Context'MetadataEntry
          -> Data.ProtoLens.Encoding.Bytes.Parser Context'MetadataEntry
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
                                       "key"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"key") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "value"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"value") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "MetadataEntry"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"key") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"value") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData Context'MetadataEntry where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Context'MetadataEntry'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_Context'MetadataEntry'key x__)
                (Control.DeepSeq.deepseq (_Context'MetadataEntry'value x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.key' @:: Lens' Context'TxIdsEntry Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.value' @:: Lens' Context'TxIdsEntry Data.Text.Text@ -}
data Context'TxIdsEntry
  = Context'TxIdsEntry'_constructor {_Context'TxIdsEntry'key :: !Data.Text.Text,
                                     _Context'TxIdsEntry'value :: !Data.Text.Text,
                                     _Context'TxIdsEntry'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Context'TxIdsEntry where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Context'TxIdsEntry "key" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'TxIdsEntry'key
           (\ x__ y__ -> x__ {_Context'TxIdsEntry'key = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Context'TxIdsEntry "value" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Context'TxIdsEntry'value
           (\ x__ y__ -> x__ {_Context'TxIdsEntry'value = y__}))
        Prelude.id
instance Data.ProtoLens.Message Context'TxIdsEntry where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.Context.TxIdsEntry"
  packedMessageDescriptor _
    = "\n\
      \\n\
      \TxIdsEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        key__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "key"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"key")) ::
              Data.ProtoLens.FieldDescriptor Context'TxIdsEntry
        value__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "value"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"value")) ::
              Data.ProtoLens.FieldDescriptor Context'TxIdsEntry
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, key__field_descriptor),
           (Data.ProtoLens.Tag 2, value__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Context'TxIdsEntry'_unknownFields
        (\ x__ y__ -> x__ {_Context'TxIdsEntry'_unknownFields = y__})
  defMessage
    = Context'TxIdsEntry'_constructor
        {_Context'TxIdsEntry'key = Data.ProtoLens.fieldDefault,
         _Context'TxIdsEntry'value = Data.ProtoLens.fieldDefault,
         _Context'TxIdsEntry'_unknownFields = []}
  parseMessage
    = let
        loop ::
          Context'TxIdsEntry
          -> Data.ProtoLens.Encoding.Bytes.Parser Context'TxIdsEntry
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
                                       "key"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"key") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "value"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"value") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "TxIdsEntry"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"key") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"value") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData Context'TxIdsEntry where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Context'TxIdsEntry'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_Context'TxIdsEntry'key x__)
                (Control.DeepSeq.deepseq (_Context'TxIdsEntry'value x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.context' @:: Lens' GetManifestRequest Context@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'context' @:: Lens' GetManifestRequest (Prelude.Maybe Context)@ -}
data GetManifestRequest
  = GetManifestRequest'_constructor {_GetManifestRequest'context :: !(Prelude.Maybe Context),
                                     _GetManifestRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show GetManifestRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField GetManifestRequest "context" Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _GetManifestRequest'context
           (\ x__ y__ -> x__ {_GetManifestRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField GetManifestRequest "maybe'context" (Prelude.Maybe Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _GetManifestRequest'context
           (\ x__ y__ -> x__ {_GetManifestRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Message GetManifestRequest where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.GetManifestRequest"
  packedMessageDescriptor _
    = "\n\
      \\DC2GetManifestRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor GetManifestRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _GetManifestRequest'_unknownFields
        (\ x__ y__ -> x__ {_GetManifestRequest'_unknownFields = y__})
  defMessage
    = GetManifestRequest'_constructor
        {_GetManifestRequest'context = Prelude.Nothing,
         _GetManifestRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          GetManifestRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser GetManifestRequest
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "GetManifestRequest"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'context") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData GetManifestRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_GetManifestRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq (_GetManifestRequest'context x__) ())
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.manifest' @:: Lens' GetManifestResponse Manifest@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'manifest' @:: Lens' GetManifestResponse (Prelude.Maybe Manifest)@ -}
data GetManifestResponse
  = GetManifestResponse'_constructor {_GetManifestResponse'manifest :: !(Prelude.Maybe Manifest),
                                      _GetManifestResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show GetManifestResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField GetManifestResponse "manifest" Manifest where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _GetManifestResponse'manifest
           (\ x__ y__ -> x__ {_GetManifestResponse'manifest = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField GetManifestResponse "maybe'manifest" (Prelude.Maybe Manifest) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _GetManifestResponse'manifest
           (\ x__ y__ -> x__ {_GetManifestResponse'manifest = y__}))
        Prelude.id
instance Data.ProtoLens.Message GetManifestResponse where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.GetManifestResponse"
  packedMessageDescriptor _
    = "\n\
      \\DC3GetManifestResponse\DC28\n\
      \\bmanifest\CAN\SOH \SOH(\v2\FS.coremesh.plugin.v1.ManifestR\bmanifest"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        manifest__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "manifest"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Manifest)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'manifest")) ::
              Data.ProtoLens.FieldDescriptor GetManifestResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, manifest__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _GetManifestResponse'_unknownFields
        (\ x__ y__ -> x__ {_GetManifestResponse'_unknownFields = y__})
  defMessage
    = GetManifestResponse'_constructor
        {_GetManifestResponse'manifest = Prelude.Nothing,
         _GetManifestResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          GetManifestResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser GetManifestResponse
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "manifest"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"manifest") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "GetManifestResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'manifest") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData GetManifestResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_GetManifestResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq (_GetManifestResponse'manifest x__) ())
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.context' @:: Lens' HandleRequest Context@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'context' @:: Lens' HandleRequest (Prelude.Maybe Context)@
         * 'Proto.Plugin.V1.Plugin_Fields.object' @:: Lens' HandleRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.action' @:: Lens' HandleRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.payload' @:: Lens' HandleRequest Proto.Google.Protobuf.Struct.Value@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'payload' @:: Lens' HandleRequest (Prelude.Maybe Proto.Google.Protobuf.Struct.Value)@ -}
data HandleRequest
  = HandleRequest'_constructor {_HandleRequest'context :: !(Prelude.Maybe Context),
                                _HandleRequest'object :: !Data.Text.Text,
                                _HandleRequest'action :: !Data.Text.Text,
                                _HandleRequest'payload :: !(Prelude.Maybe Proto.Google.Protobuf.Struct.Value),
                                _HandleRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show HandleRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField HandleRequest "context" Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'context
           (\ x__ y__ -> x__ {_HandleRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField HandleRequest "maybe'context" (Prelude.Maybe Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'context
           (\ x__ y__ -> x__ {_HandleRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField HandleRequest "object" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'object
           (\ x__ y__ -> x__ {_HandleRequest'object = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField HandleRequest "action" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'action
           (\ x__ y__ -> x__ {_HandleRequest'action = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField HandleRequest "payload" Proto.Google.Protobuf.Struct.Value where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'payload
           (\ x__ y__ -> x__ {_HandleRequest'payload = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField HandleRequest "maybe'payload" (Prelude.Maybe Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleRequest'payload
           (\ x__ y__ -> x__ {_HandleRequest'payload = y__}))
        Prelude.id
instance Data.ProtoLens.Message HandleRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.HandleRequest"
  packedMessageDescriptor _
    = "\n\
      \\rHandleRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SYN\n\
      \\ACKobject\CAN\STX \SOH(\tR\ACKobject\DC2\SYN\n\
      \\ACKaction\CAN\ETX \SOH(\tR\ACKaction\DC20\n\
      \\apayload\CAN\EOT \SOH(\v2\SYN.google.protobuf.ValueR\apayload"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor HandleRequest
        object__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "object"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"object")) ::
              Data.ProtoLens.FieldDescriptor HandleRequest
        action__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "action"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"action")) ::
              Data.ProtoLens.FieldDescriptor HandleRequest
        payload__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "payload"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Value)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'payload")) ::
              Data.ProtoLens.FieldDescriptor HandleRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, object__field_descriptor),
           (Data.ProtoLens.Tag 3, action__field_descriptor),
           (Data.ProtoLens.Tag 4, payload__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _HandleRequest'_unknownFields
        (\ x__ y__ -> x__ {_HandleRequest'_unknownFields = y__})
  defMessage
    = HandleRequest'_constructor
        {_HandleRequest'context = Prelude.Nothing,
         _HandleRequest'object = Data.ProtoLens.fieldDefault,
         _HandleRequest'action = Data.ProtoLens.fieldDefault,
         _HandleRequest'payload = Prelude.Nothing,
         _HandleRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          HandleRequest -> Data.ProtoLens.Encoding.Bytes.Parser HandleRequest
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "object"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"object") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "action"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"action") y x)
                        34
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "payload"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"payload") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "HandleRequest"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'context") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"object") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                ((Data.Monoid.<>)
                   (let
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"action") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                            ((Prelude..)
                               (\ bs
                                  -> (Data.Monoid.<>)
                                       (Data.ProtoLens.Encoding.Bytes.putVarInt
                                          (Prelude.fromIntegral (Data.ByteString.length bs)))
                                       (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                               Data.Text.Encoding.encodeUtf8 _v))
                   ((Data.Monoid.<>)
                      (case
                           Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'payload") _x
                       of
                         Prelude.Nothing -> Data.Monoid.mempty
                         (Prelude.Just _v)
                           -> (Data.Monoid.<>)
                                (Data.ProtoLens.Encoding.Bytes.putVarInt 34)
                                ((Prelude..)
                                   (\ bs
                                      -> (Data.Monoid.<>)
                                           (Data.ProtoLens.Encoding.Bytes.putVarInt
                                              (Prelude.fromIntegral (Data.ByteString.length bs)))
                                           (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                   Data.ProtoLens.encodeMessage _v))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData HandleRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_HandleRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_HandleRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_HandleRequest'object x__)
                   (Control.DeepSeq.deepseq
                      (_HandleRequest'action x__)
                      (Control.DeepSeq.deepseq (_HandleRequest'payload x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.context' @:: Lens' HandleResponse Context@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'context' @:: Lens' HandleResponse (Prelude.Maybe Context)@
         * 'Proto.Plugin.V1.Plugin_Fields.payload' @:: Lens' HandleResponse Proto.Google.Protobuf.Struct.Value@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'payload' @:: Lens' HandleResponse (Prelude.Maybe Proto.Google.Protobuf.Struct.Value)@ -}
data HandleResponse
  = HandleResponse'_constructor {_HandleResponse'context :: !(Prelude.Maybe Context),
                                 _HandleResponse'payload :: !(Prelude.Maybe Proto.Google.Protobuf.Struct.Value),
                                 _HandleResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show HandleResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField HandleResponse "context" Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleResponse'context
           (\ x__ y__ -> x__ {_HandleResponse'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField HandleResponse "maybe'context" (Prelude.Maybe Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleResponse'context
           (\ x__ y__ -> x__ {_HandleResponse'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField HandleResponse "payload" Proto.Google.Protobuf.Struct.Value where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleResponse'payload
           (\ x__ y__ -> x__ {_HandleResponse'payload = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField HandleResponse "maybe'payload" (Prelude.Maybe Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _HandleResponse'payload
           (\ x__ y__ -> x__ {_HandleResponse'payload = y__}))
        Prelude.id
instance Data.ProtoLens.Message HandleResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.HandleResponse"
  packedMessageDescriptor _
    = "\n\
      \\SO\&HandleResponse\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC20\n\
      \\apayload\CAN\STX \SOH(\v2\SYN.google.protobuf.ValueR\apayload"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor HandleResponse
        payload__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "payload"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Value)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'payload")) ::
              Data.ProtoLens.FieldDescriptor HandleResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, payload__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _HandleResponse'_unknownFields
        (\ x__ y__ -> x__ {_HandleResponse'_unknownFields = y__})
  defMessage
    = HandleResponse'_constructor
        {_HandleResponse'context = Prelude.Nothing,
         _HandleResponse'payload = Prelude.Nothing,
         _HandleResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          HandleResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser HandleResponse
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "payload"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"payload") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "HandleResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'context") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             ((Data.Monoid.<>)
                (case
                     Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'payload") _x
                 of
                   Prelude.Nothing -> Data.Monoid.mempty
                   (Prelude.Just _v)
                     -> (Data.Monoid.<>)
                          (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                          ((Prelude..)
                             (\ bs
                                -> (Data.Monoid.<>)
                                     (Data.ProtoLens.Encoding.Bytes.putVarInt
                                        (Prelude.fromIntegral (Data.ByteString.length bs)))
                                     (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                             Data.ProtoLens.encodeMessage _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData HandleResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_HandleResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_HandleResponse'context x__)
                (Control.DeepSeq.deepseq (_HandleResponse'payload x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.name' @:: Lens' Manifest Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.version' @:: Lens' Manifest Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.description' @:: Lens' Manifest Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.capabilities' @:: Lens' Manifest [Capability]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'capabilities' @:: Lens' Manifest (Data.Vector.Vector Capability)@ -}
data Manifest
  = Manifest'_constructor {_Manifest'name :: !Data.Text.Text,
                           _Manifest'version :: !Data.Text.Text,
                           _Manifest'description :: !Data.Text.Text,
                           _Manifest'capabilities :: !(Data.Vector.Vector Capability),
                           _Manifest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Manifest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Manifest "name" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Manifest'name (\ x__ y__ -> x__ {_Manifest'name = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Manifest "version" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Manifest'version (\ x__ y__ -> x__ {_Manifest'version = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Manifest "description" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Manifest'description
           (\ x__ y__ -> x__ {_Manifest'description = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField Manifest "capabilities" [Capability] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Manifest'capabilities
           (\ x__ y__ -> x__ {_Manifest'capabilities = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField Manifest "vec'capabilities" (Data.Vector.Vector Capability) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Manifest'capabilities
           (\ x__ y__ -> x__ {_Manifest'capabilities = y__}))
        Prelude.id
instance Data.ProtoLens.Message Manifest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.Manifest"
  packedMessageDescriptor _
    = "\n\
      \\bManifest\DC2\DC2\n\
      \\EOTname\CAN\SOH \SOH(\tR\EOTname\DC2\CAN\n\
      \\aversion\CAN\STX \SOH(\tR\aversion\DC2 \n\
      \\vdescription\CAN\ETX \SOH(\tR\vdescription\DC2B\n\
      \\fcapabilities\CAN\EOT \ETX(\v2\RS.coremesh.plugin.v1.CapabilityR\fcapabilities"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        name__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "name"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"name")) ::
              Data.ProtoLens.FieldDescriptor Manifest
        version__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "version"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"version")) ::
              Data.ProtoLens.FieldDescriptor Manifest
        description__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "description"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"description")) ::
              Data.ProtoLens.FieldDescriptor Manifest
        capabilities__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "capabilities"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Capability)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked
                 (Data.ProtoLens.Field.field @"capabilities")) ::
              Data.ProtoLens.FieldDescriptor Manifest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, name__field_descriptor),
           (Data.ProtoLens.Tag 2, version__field_descriptor),
           (Data.ProtoLens.Tag 3, description__field_descriptor),
           (Data.ProtoLens.Tag 4, capabilities__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Manifest'_unknownFields
        (\ x__ y__ -> x__ {_Manifest'_unknownFields = y__})
  defMessage
    = Manifest'_constructor
        {_Manifest'name = Data.ProtoLens.fieldDefault,
         _Manifest'version = Data.ProtoLens.fieldDefault,
         _Manifest'description = Data.ProtoLens.fieldDefault,
         _Manifest'capabilities = Data.Vector.Generic.empty,
         _Manifest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          Manifest
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Capability
             -> Data.ProtoLens.Encoding.Bytes.Parser Manifest
        loop x mutable'capabilities
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'capabilities <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                               (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                                  mutable'capabilities)
                      (let missing = []
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
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t)
                           (Lens.Family2.set
                              (Data.ProtoLens.Field.field @"vec'capabilities")
                              frozen'capabilities x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "name"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"name") y x)
                                  mutable'capabilities
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "version"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"version") y x)
                                  mutable'capabilities
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "description"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"description") y x)
                                  mutable'capabilities
                        34
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "capabilities"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append
                                          mutable'capabilities y)
                                loop x v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'capabilities
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'capabilities <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                        Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'capabilities)
          "Manifest"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"name") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"version") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                ((Data.Monoid.<>)
                   (let
                      _v
                        = Lens.Family2.view (Data.ProtoLens.Field.field @"description") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                            ((Prelude..)
                               (\ bs
                                  -> (Data.Monoid.<>)
                                       (Data.ProtoLens.Encoding.Bytes.putVarInt
                                          (Prelude.fromIntegral (Data.ByteString.length bs)))
                                       (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                               Data.Text.Encoding.encodeUtf8 _v))
                   ((Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                         (\ _v
                            -> (Data.Monoid.<>)
                                 (Data.ProtoLens.Encoding.Bytes.putVarInt 34)
                                 ((Prelude..)
                                    (\ bs
                                       -> (Data.Monoid.<>)
                                            (Data.ProtoLens.Encoding.Bytes.putVarInt
                                               (Prelude.fromIntegral (Data.ByteString.length bs)))
                                            (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                    Data.ProtoLens.encodeMessage _v))
                         (Lens.Family2.view
                            (Data.ProtoLens.Field.field @"vec'capabilities") _x))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData Manifest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Manifest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_Manifest'name x__)
                (Control.DeepSeq.deepseq
                   (_Manifest'version x__)
                   (Control.DeepSeq.deepseq
                      (_Manifest'description x__)
                      (Control.DeepSeq.deepseq (_Manifest'capabilities x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.rows' @:: Lens' ReadBatch [ReadRow]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'rows' @:: Lens' ReadBatch (Data.Vector.Vector ReadRow)@ -}
data ReadBatch
  = ReadBatch'_constructor {_ReadBatch'rows :: !(Data.Vector.Vector ReadRow),
                            _ReadBatch'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadBatch where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadBatch "rows" [ReadRow] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadBatch'rows (\ x__ y__ -> x__ {_ReadBatch'rows = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField ReadBatch "vec'rows" (Data.Vector.Vector ReadRow) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadBatch'rows (\ x__ y__ -> x__ {_ReadBatch'rows = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadBatch where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ReadBatch"
  packedMessageDescriptor _
    = "\n\
      \\tReadBatch\DC2/\n\
      \\EOTrows\CAN\SOH \ETX(\v2\ESC.coremesh.plugin.v1.ReadRowR\EOTrows"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        rows__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "rows"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadRow)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"rows")) ::
              Data.ProtoLens.FieldDescriptor ReadBatch
      in
        Data.Map.fromList [(Data.ProtoLens.Tag 1, rows__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadBatch'_unknownFields
        (\ x__ y__ -> x__ {_ReadBatch'_unknownFields = y__})
  defMessage
    = ReadBatch'_constructor
        {_ReadBatch'rows = Data.Vector.Generic.empty,
         _ReadBatch'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadBatch
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld ReadRow
             -> Data.ProtoLens.Encoding.Bytes.Parser ReadBatch
        loop x mutable'rows
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'rows <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.unsafeFreeze mutable'rows)
                      (let missing = []
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
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t)
                           (Lens.Family2.set
                              (Data.ProtoLens.Field.field @"vec'rows") frozen'rows x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "rows"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'rows y)
                                loop x v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'rows
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'rows <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'rows)
          "ReadBatch"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                (\ _v
                   -> (Data.Monoid.<>)
                        (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                        ((Prelude..)
                           (\ bs
                              -> (Data.Monoid.<>)
                                   (Data.ProtoLens.Encoding.Bytes.putVarInt
                                      (Prelude.fromIntegral (Data.ByteString.length bs)))
                                   (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                           Data.ProtoLens.encodeMessage _v))
                (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'rows") _x))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData ReadBatch where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadBatch'_unknownFields x__)
             (Control.DeepSeq.deepseq (_ReadBatch'rows x__) ())
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.rows' @:: Lens' ReadEnd Data.Int.Int64@
         * 'Proto.Plugin.V1.Plugin_Fields.cursor' @:: Lens' ReadEnd Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.metadata' @:: Lens' ReadEnd (Data.Map.Map Data.Text.Text Data.Text.Text)@ -}
data ReadEnd
  = ReadEnd'_constructor {_ReadEnd'rows :: !Data.Int.Int64,
                          _ReadEnd'cursor :: !Data.Text.Text,
                          _ReadEnd'metadata :: !(Data.Map.Map Data.Text.Text Data.Text.Text),
                          _ReadEnd'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadEnd where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadEnd "rows" Data.Int.Int64 where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadEnd'rows (\ x__ y__ -> x__ {_ReadEnd'rows = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadEnd "cursor" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadEnd'cursor (\ x__ y__ -> x__ {_ReadEnd'cursor = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadEnd "metadata" (Data.Map.Map Data.Text.Text Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadEnd'metadata (\ x__ y__ -> x__ {_ReadEnd'metadata = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadEnd where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ReadEnd"
  packedMessageDescriptor _
    = "\n\
      \\aReadEnd\DC2\DC2\n\
      \\EOTrows\CAN\SOH \SOH(\ETXR\EOTrows\DC2\SYN\n\
      \\ACKcursor\CAN\STX \SOH(\tR\ACKcursor\DC2E\n\
      \\bmetadata\CAN\ETX \ETX(\v2).coremesh.plugin.v1.ReadEnd.MetadataEntryR\bmetadata\SUB;\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        rows__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "rows"
              (Data.ProtoLens.ScalarField Data.ProtoLens.Int64Field ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Int.Int64)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"rows")) ::
              Data.ProtoLens.FieldDescriptor ReadEnd
        cursor__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "cursor"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"cursor")) ::
              Data.ProtoLens.FieldDescriptor ReadEnd
        metadata__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "metadata"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadEnd'MetadataEntry)
              (Data.ProtoLens.MapField
                 (Data.ProtoLens.Field.field @"key")
                 (Data.ProtoLens.Field.field @"value")
                 (Data.ProtoLens.Field.field @"metadata")) ::
              Data.ProtoLens.FieldDescriptor ReadEnd
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, rows__field_descriptor),
           (Data.ProtoLens.Tag 2, cursor__field_descriptor),
           (Data.ProtoLens.Tag 3, metadata__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadEnd'_unknownFields
        (\ x__ y__ -> x__ {_ReadEnd'_unknownFields = y__})
  defMessage
    = ReadEnd'_constructor
        {_ReadEnd'rows = Data.ProtoLens.fieldDefault,
         _ReadEnd'cursor = Data.ProtoLens.fieldDefault,
         _ReadEnd'metadata = Data.Map.empty, _ReadEnd'_unknownFields = []}
  parseMessage
    = let
        loop :: ReadEnd -> Data.ProtoLens.Encoding.Bytes.Parser ReadEnd
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
                                          Prelude.fromIntegral
                                          Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "rows"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"rows") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "cursor"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"cursor") y x)
                        26
                          -> do !(entry :: ReadEnd'MetadataEntry) <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                                                           Data.ProtoLens.Encoding.Bytes.isolate
                                                                             (Prelude.fromIntegral
                                                                                len)
                                                                             Data.ProtoLens.parseMessage)
                                                                       "metadata"
                                (let
                                   key = Lens.Family2.view (Data.ProtoLens.Field.field @"key") entry
                                   value
                                     = Lens.Family2.view (Data.ProtoLens.Field.field @"value") entry
                                 in
                                   loop
                                     (Lens.Family2.over
                                        (Data.ProtoLens.Field.field @"metadata")
                                        (\ !t -> Data.Map.insert key value t) x))
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "ReadEnd"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"rows") _x
              in
                if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                    Data.Monoid.mempty
                else
                    (Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.putVarInt 8)
                      ((Prelude..)
                         Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral _v))
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"cursor") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                ((Data.Monoid.<>)
                   (Data.Monoid.mconcat
                      (Prelude.map
                         (\ _v
                            -> (Data.Monoid.<>)
                                 (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                                 ((Prelude..)
                                    (\ bs
                                       -> (Data.Monoid.<>)
                                            (Data.ProtoLens.Encoding.Bytes.putVarInt
                                               (Prelude.fromIntegral (Data.ByteString.length bs)))
                                            (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                    Data.ProtoLens.encodeMessage
                                    (Lens.Family2.set
                                       (Data.ProtoLens.Field.field @"key") (Prelude.fst _v)
                                       (Lens.Family2.set
                                          (Data.ProtoLens.Field.field @"value") (Prelude.snd _v)
                                          (Data.ProtoLens.defMessage :: ReadEnd'MetadataEntry)))))
                         (Data.Map.toList
                            (Lens.Family2.view (Data.ProtoLens.Field.field @"metadata") _x))))
                   (Data.ProtoLens.Encoding.Wire.buildFieldSet
                      (Lens.Family2.view Data.ProtoLens.unknownFields _x))))
instance Control.DeepSeq.NFData ReadEnd where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadEnd'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ReadEnd'rows x__)
                (Control.DeepSeq.deepseq
                   (_ReadEnd'cursor x__)
                   (Control.DeepSeq.deepseq (_ReadEnd'metadata x__) ())))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.key' @:: Lens' ReadEnd'MetadataEntry Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.value' @:: Lens' ReadEnd'MetadataEntry Data.Text.Text@ -}
data ReadEnd'MetadataEntry
  = ReadEnd'MetadataEntry'_constructor {_ReadEnd'MetadataEntry'key :: !Data.Text.Text,
                                        _ReadEnd'MetadataEntry'value :: !Data.Text.Text,
                                        _ReadEnd'MetadataEntry'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadEnd'MetadataEntry where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadEnd'MetadataEntry "key" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadEnd'MetadataEntry'key
           (\ x__ y__ -> x__ {_ReadEnd'MetadataEntry'key = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadEnd'MetadataEntry "value" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadEnd'MetadataEntry'value
           (\ x__ y__ -> x__ {_ReadEnd'MetadataEntry'value = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadEnd'MetadataEntry where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.ReadEnd.MetadataEntry"
  packedMessageDescriptor _
    = "\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        key__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "key"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"key")) ::
              Data.ProtoLens.FieldDescriptor ReadEnd'MetadataEntry
        value__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "value"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"value")) ::
              Data.ProtoLens.FieldDescriptor ReadEnd'MetadataEntry
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, key__field_descriptor),
           (Data.ProtoLens.Tag 2, value__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadEnd'MetadataEntry'_unknownFields
        (\ x__ y__ -> x__ {_ReadEnd'MetadataEntry'_unknownFields = y__})
  defMessage
    = ReadEnd'MetadataEntry'_constructor
        {_ReadEnd'MetadataEntry'key = Data.ProtoLens.fieldDefault,
         _ReadEnd'MetadataEntry'value = Data.ProtoLens.fieldDefault,
         _ReadEnd'MetadataEntry'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadEnd'MetadataEntry
          -> Data.ProtoLens.Encoding.Bytes.Parser ReadEnd'MetadataEntry
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
                                       "key"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"key") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "value"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"value") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "MetadataEntry"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"key") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"value") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData ReadEnd'MetadataEntry where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadEnd'MetadataEntry'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ReadEnd'MetadataEntry'key x__)
                (Control.DeepSeq.deepseq (_ReadEnd'MetadataEntry'value x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.columns' @:: Lens' ReadHeader [Data.Text.Text]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'columns' @:: Lens' ReadHeader (Data.Vector.Vector Data.Text.Text)@
         * 'Proto.Plugin.V1.Plugin_Fields.metadata' @:: Lens' ReadHeader (Data.Map.Map Data.Text.Text Data.Text.Text)@ -}
data ReadHeader
  = ReadHeader'_constructor {_ReadHeader'columns :: !(Data.Vector.Vector Data.Text.Text),
                             _ReadHeader'metadata :: !(Data.Map.Map Data.Text.Text Data.Text.Text),
                             _ReadHeader'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadHeader where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadHeader "columns" [Data.Text.Text] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadHeader'columns (\ x__ y__ -> x__ {_ReadHeader'columns = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField ReadHeader "vec'columns" (Data.Vector.Vector Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadHeader'columns (\ x__ y__ -> x__ {_ReadHeader'columns = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadHeader "metadata" (Data.Map.Map Data.Text.Text Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadHeader'metadata
           (\ x__ y__ -> x__ {_ReadHeader'metadata = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadHeader where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ReadHeader"
  packedMessageDescriptor _
    = "\n\
      \\n\
      \ReadHeader\DC2\CAN\n\
      \\acolumns\CAN\SOH \ETX(\tR\acolumns\DC2H\n\
      \\bmetadata\CAN\STX \ETX(\v2,.coremesh.plugin.v1.ReadHeader.MetadataEntryR\bmetadata\SUB;\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        columns__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "columns"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"columns")) ::
              Data.ProtoLens.FieldDescriptor ReadHeader
        metadata__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "metadata"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadHeader'MetadataEntry)
              (Data.ProtoLens.MapField
                 (Data.ProtoLens.Field.field @"key")
                 (Data.ProtoLens.Field.field @"value")
                 (Data.ProtoLens.Field.field @"metadata")) ::
              Data.ProtoLens.FieldDescriptor ReadHeader
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, columns__field_descriptor),
           (Data.ProtoLens.Tag 2, metadata__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadHeader'_unknownFields
        (\ x__ y__ -> x__ {_ReadHeader'_unknownFields = y__})
  defMessage
    = ReadHeader'_constructor
        {_ReadHeader'columns = Data.Vector.Generic.empty,
         _ReadHeader'metadata = Data.Map.empty,
         _ReadHeader'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadHeader
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Data.Text.Text
             -> Data.ProtoLens.Encoding.Bytes.Parser ReadHeader
        loop x mutable'columns
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'columns <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                          (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                             mutable'columns)
                      (let missing = []
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
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t)
                           (Lens.Family2.set
                              (Data.ProtoLens.Field.field @"vec'columns") frozen'columns x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.getText
                                              (Prelude.fromIntegral len))
                                        "columns"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'columns y)
                                loop x v
                        18
                          -> do !(entry :: ReadHeader'MetadataEntry) <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                                                          (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                                                              Data.ProtoLens.Encoding.Bytes.isolate
                                                                                (Prelude.fromIntegral
                                                                                   len)
                                                                                Data.ProtoLens.parseMessage)
                                                                          "metadata"
                                (let
                                   key = Lens.Family2.view (Data.ProtoLens.Field.field @"key") entry
                                   value
                                     = Lens.Family2.view (Data.ProtoLens.Field.field @"value") entry
                                 in
                                   loop
                                     (Lens.Family2.over
                                        (Data.ProtoLens.Field.field @"metadata")
                                        (\ !t -> Data.Map.insert key value t) x)
                                     mutable'columns)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'columns
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'columns <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                   Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'columns)
          "ReadHeader"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                (\ _v
                   -> (Data.Monoid.<>)
                        (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                        ((Prelude..)
                           (\ bs
                              -> (Data.Monoid.<>)
                                   (Data.ProtoLens.Encoding.Bytes.putVarInt
                                      (Prelude.fromIntegral (Data.ByteString.length bs)))
                                   (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                           Data.Text.Encoding.encodeUtf8 _v))
                (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'columns") _x))
             ((Data.Monoid.<>)
                (Data.Monoid.mconcat
                   (Prelude.map
                      (\ _v
                         -> (Data.Monoid.<>)
                              (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                              ((Prelude..)
                                 (\ bs
                                    -> (Data.Monoid.<>)
                                         (Data.ProtoLens.Encoding.Bytes.putVarInt
                                            (Prelude.fromIntegral (Data.ByteString.length bs)))
                                         (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                                 Data.ProtoLens.encodeMessage
                                 (Lens.Family2.set
                                    (Data.ProtoLens.Field.field @"key") (Prelude.fst _v)
                                    (Lens.Family2.set
                                       (Data.ProtoLens.Field.field @"value") (Prelude.snd _v)
                                       (Data.ProtoLens.defMessage :: ReadHeader'MetadataEntry)))))
                      (Data.Map.toList
                         (Lens.Family2.view (Data.ProtoLens.Field.field @"metadata") _x))))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData ReadHeader where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadHeader'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ReadHeader'columns x__)
                (Control.DeepSeq.deepseq (_ReadHeader'metadata x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.key' @:: Lens' ReadHeader'MetadataEntry Data.Text.Text@
         * 'Proto.Plugin.V1.Plugin_Fields.value' @:: Lens' ReadHeader'MetadataEntry Data.Text.Text@ -}
data ReadHeader'MetadataEntry
  = ReadHeader'MetadataEntry'_constructor {_ReadHeader'MetadataEntry'key :: !Data.Text.Text,
                                           _ReadHeader'MetadataEntry'value :: !Data.Text.Text,
                                           _ReadHeader'MetadataEntry'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadHeader'MetadataEntry where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadHeader'MetadataEntry "key" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadHeader'MetadataEntry'key
           (\ x__ y__ -> x__ {_ReadHeader'MetadataEntry'key = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadHeader'MetadataEntry "value" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadHeader'MetadataEntry'value
           (\ x__ y__ -> x__ {_ReadHeader'MetadataEntry'value = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadHeader'MetadataEntry where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.ReadHeader.MetadataEntry"
  packedMessageDescriptor _
    = "\n\
      \\rMetadataEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        key__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "key"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"key")) ::
              Data.ProtoLens.FieldDescriptor ReadHeader'MetadataEntry
        value__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "value"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"value")) ::
              Data.ProtoLens.FieldDescriptor ReadHeader'MetadataEntry
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, key__field_descriptor),
           (Data.ProtoLens.Tag 2, value__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadHeader'MetadataEntry'_unknownFields
        (\ x__ y__ -> x__ {_ReadHeader'MetadataEntry'_unknownFields = y__})
  defMessage
    = ReadHeader'MetadataEntry'_constructor
        {_ReadHeader'MetadataEntry'key = Data.ProtoLens.fieldDefault,
         _ReadHeader'MetadataEntry'value = Data.ProtoLens.fieldDefault,
         _ReadHeader'MetadataEntry'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadHeader'MetadataEntry
          -> Data.ProtoLens.Encoding.Bytes.Parser ReadHeader'MetadataEntry
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
                                       "key"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"key") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "value"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"value") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "MetadataEntry"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"key") _x
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
             ((Data.Monoid.<>)
                (let
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"value") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                         ((Prelude..)
                            (\ bs
                               -> (Data.Monoid.<>)
                                    (Data.ProtoLens.Encoding.Bytes.putVarInt
                                       (Prelude.fromIntegral (Data.ByteString.length bs)))
                                    (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                            Data.Text.Encoding.encodeUtf8 _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData ReadHeader'MetadataEntry where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadHeader'MetadataEntry'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ReadHeader'MetadataEntry'key x__)
                (Control.DeepSeq.deepseq (_ReadHeader'MetadataEntry'value x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.context' @:: Lens' ReadResponse Context@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'context' @:: Lens' ReadResponse (Prelude.Maybe Context)@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'part' @:: Lens' ReadResponse (Prelude.Maybe ReadResponse'Part)@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'header' @:: Lens' ReadResponse (Prelude.Maybe ReadHeader)@
         * 'Proto.Plugin.V1.Plugin_Fields.header' @:: Lens' ReadResponse ReadHeader@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'batch' @:: Lens' ReadResponse (Prelude.Maybe ReadBatch)@
         * 'Proto.Plugin.V1.Plugin_Fields.batch' @:: Lens' ReadResponse ReadBatch@
         * 'Proto.Plugin.V1.Plugin_Fields.maybe'end' @:: Lens' ReadResponse (Prelude.Maybe ReadEnd)@
         * 'Proto.Plugin.V1.Plugin_Fields.end' @:: Lens' ReadResponse ReadEnd@ -}
data ReadResponse
  = ReadResponse'_constructor {_ReadResponse'context :: !(Prelude.Maybe Context),
                               _ReadResponse'part :: !(Prelude.Maybe ReadResponse'Part),
                               _ReadResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
data ReadResponse'Part
  = ReadResponse'Header !ReadHeader |
    ReadResponse'Batch !ReadBatch |
    ReadResponse'End !ReadEnd
  deriving stock (Prelude.Show, Prelude.Eq, Prelude.Ord)
instance Data.ProtoLens.Field.HasField ReadResponse "context" Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'context
           (\ x__ y__ -> x__ {_ReadResponse'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField ReadResponse "maybe'context" (Prelude.Maybe Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'context
           (\ x__ y__ -> x__ {_ReadResponse'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadResponse "maybe'part" (Prelude.Maybe ReadResponse'Part) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ReadResponse "maybe'header" (Prelude.Maybe ReadHeader) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        (Lens.Family2.Unchecked.lens
           (\ x__
              -> case x__ of
                   (Prelude.Just (ReadResponse'Header x__val)) -> Prelude.Just x__val
                   _otherwise -> Prelude.Nothing)
           (\ _ y__ -> Prelude.fmap ReadResponse'Header y__))
instance Data.ProtoLens.Field.HasField ReadResponse "header" ReadHeader where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        ((Prelude..)
           (Lens.Family2.Unchecked.lens
              (\ x__
                 -> case x__ of
                      (Prelude.Just (ReadResponse'Header x__val)) -> Prelude.Just x__val
                      _otherwise -> Prelude.Nothing)
              (\ _ y__ -> Prelude.fmap ReadResponse'Header y__))
           (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage))
instance Data.ProtoLens.Field.HasField ReadResponse "maybe'batch" (Prelude.Maybe ReadBatch) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        (Lens.Family2.Unchecked.lens
           (\ x__
              -> case x__ of
                   (Prelude.Just (ReadResponse'Batch x__val)) -> Prelude.Just x__val
                   _otherwise -> Prelude.Nothing)
           (\ _ y__ -> Prelude.fmap ReadResponse'Batch y__))
instance Data.ProtoLens.Field.HasField ReadResponse "batch" ReadBatch where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        ((Prelude..)
           (Lens.Family2.Unchecked.lens
              (\ x__
                 -> case x__ of
                      (Prelude.Just (ReadResponse'Batch x__val)) -> Prelude.Just x__val
                      _otherwise -> Prelude.Nothing)
              (\ _ y__ -> Prelude.fmap ReadResponse'Batch y__))
           (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage))
instance Data.ProtoLens.Field.HasField ReadResponse "maybe'end" (Prelude.Maybe ReadEnd) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        (Lens.Family2.Unchecked.lens
           (\ x__
              -> case x__ of
                   (Prelude.Just (ReadResponse'End x__val)) -> Prelude.Just x__val
                   _otherwise -> Prelude.Nothing)
           (\ _ y__ -> Prelude.fmap ReadResponse'End y__))
instance Data.ProtoLens.Field.HasField ReadResponse "end" ReadEnd where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadResponse'part (\ x__ y__ -> x__ {_ReadResponse'part = y__}))
        ((Prelude..)
           (Lens.Family2.Unchecked.lens
              (\ x__
                 -> case x__ of
                      (Prelude.Just (ReadResponse'End x__val)) -> Prelude.Just x__val
                      _otherwise -> Prelude.Nothing)
              (\ _ y__ -> Prelude.fmap ReadResponse'End y__))
           (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage))
instance Data.ProtoLens.Message ReadResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ReadResponse"
  packedMessageDescriptor _
    = "\n\
      \\fReadResponse\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC28\n\
      \\ACKheader\CAN\STX \SOH(\v2\RS.coremesh.plugin.v1.ReadHeaderH\NULR\ACKheader\DC25\n\
      \\ENQbatch\CAN\ETX \SOH(\v2\GS.coremesh.plugin.v1.ReadBatchH\NULR\ENQbatch\DC2/\n\
      \\ETXend\CAN\EOT \SOH(\v2\ESC.coremesh.plugin.v1.ReadEndH\NULR\ETXendB\ACK\n\
      \\EOTpart"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor ReadResponse
        header__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "header"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadHeader)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'header")) ::
              Data.ProtoLens.FieldDescriptor ReadResponse
        batch__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "batch"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadBatch)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'batch")) ::
              Data.ProtoLens.FieldDescriptor ReadResponse
        end__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "end"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ReadEnd)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'end")) ::
              Data.ProtoLens.FieldDescriptor ReadResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, header__field_descriptor),
           (Data.ProtoLens.Tag 3, batch__field_descriptor),
           (Data.ProtoLens.Tag 4, end__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadResponse'_unknownFields
        (\ x__ y__ -> x__ {_ReadResponse'_unknownFields = y__})
  defMessage
    = ReadResponse'_constructor
        {_ReadResponse'context = Prelude.Nothing,
         _ReadResponse'part = Prelude.Nothing,
         _ReadResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadResponse -> Data.ProtoLens.Encoding.Bytes.Parser ReadResponse
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
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "header"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"header") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "batch"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"batch") y x)
                        34
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "end"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"end") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "ReadResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (case
                  Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'context") _x
              of
                Prelude.Nothing -> Data.Monoid.mempty
                (Prelude.Just _v)
                  -> (Data.Monoid.<>)
                       (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                       ((Prelude..)
                          (\ bs
                             -> (Data.Monoid.<>)
                                  (Data.ProtoLens.Encoding.Bytes.putVarInt
                                     (Prelude.fromIntegral (Data.ByteString.length bs)))
                                  (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                          Data.ProtoLens.encodeMessage _v))
             ((Data.Monoid.<>)
                (case
                     Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'part") _x
                 of
                   Prelude.Nothing -> Data.Monoid.mempty
                   (Prelude.Just (ReadResponse'Header v))
                     -> (Data.Monoid.<>)
                          (Data.ProtoLens.Encoding.Bytes.putVarInt 18)
                          ((Prelude..)
                             (\ bs
                                -> (Data.Monoid.<>)
                                     (Data.ProtoLens.Encoding.Bytes.putVarInt
                                        (Prelude.fromIntegral (Data.ByteString.length bs)))
                                     (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                             Data.ProtoLens.encodeMessage v)
                   (Prelude.Just (ReadResponse'Batch v))
                     -> (Data.Monoid.<>)
                          (Data.ProtoLens.Encoding.Bytes.putVarInt 26)
                          ((Prelude..)
                             (\ bs
                                -> (Data.Monoid.<>)
                                     (Data.ProtoLens.Encoding.Bytes.putVarInt
                                        (Prelude.fromIntegral (Data.ByteString.length bs)))
                                     (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                             Data.ProtoLens.encodeMessage v)
                   (Prelude.Just (ReadResponse'End v))
                     -> (Data.Monoid.<>)
                          (Data.ProtoLens.Encoding.Bytes.putVarInt 34)
                          ((Prelude..)
                             (\ bs
                                -> (Data.Monoid.<>)
                                     (Data.ProtoLens.Encoding.Bytes.putVarInt
                                        (Prelude.fromIntegral (Data.ByteString.length bs)))
                                     (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                             Data.ProtoLens.encodeMessage v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData ReadResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ReadResponse'context x__)
                (Control.DeepSeq.deepseq (_ReadResponse'part x__) ()))
instance Control.DeepSeq.NFData ReadResponse'Part where
  rnf (ReadResponse'Header x__) = Control.DeepSeq.rnf x__
  rnf (ReadResponse'Batch x__) = Control.DeepSeq.rnf x__
  rnf (ReadResponse'End x__) = Control.DeepSeq.rnf x__
_ReadResponse'Header ::
  Data.ProtoLens.Prism.Prism' ReadResponse'Part ReadHeader
_ReadResponse'Header
  = Data.ProtoLens.Prism.prism'
      ReadResponse'Header
      (\ p__
         -> case p__ of
              (ReadResponse'Header p__val) -> Prelude.Just p__val
              _otherwise -> Prelude.Nothing)
_ReadResponse'Batch ::
  Data.ProtoLens.Prism.Prism' ReadResponse'Part ReadBatch
_ReadResponse'Batch
  = Data.ProtoLens.Prism.prism'
      ReadResponse'Batch
      (\ p__
         -> case p__ of
              (ReadResponse'Batch p__val) -> Prelude.Just p__val
              _otherwise -> Prelude.Nothing)
_ReadResponse'End ::
  Data.ProtoLens.Prism.Prism' ReadResponse'Part ReadEnd
_ReadResponse'End
  = Data.ProtoLens.Prism.prism'
      ReadResponse'End
      (\ p__
         -> case p__ of
              (ReadResponse'End p__val) -> Prelude.Just p__val
              _otherwise -> Prelude.Nothing)
{- | Fields :
     
         * 'Proto.Plugin.V1.Plugin_Fields.values' @:: Lens' ReadRow [Proto.Google.Protobuf.Struct.Value]@
         * 'Proto.Plugin.V1.Plugin_Fields.vec'values' @:: Lens' ReadRow (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value)@ -}
data ReadRow
  = ReadRow'_constructor {_ReadRow'values :: !(Data.Vector.Vector Proto.Google.Protobuf.Struct.Value),
                          _ReadRow'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ReadRow where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ReadRow "values" [Proto.Google.Protobuf.Struct.Value] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadRow'values (\ x__ y__ -> x__ {_ReadRow'values = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField ReadRow "vec'values" (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ReadRow'values (\ x__ y__ -> x__ {_ReadRow'values = y__}))
        Prelude.id
instance Data.ProtoLens.Message ReadRow where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ReadRow"
  packedMessageDescriptor _
    = "\n\
      \\aReadRow\DC2.\n\
      \\ACKvalues\CAN\SOH \ETX(\v2\SYN.google.protobuf.ValueR\ACKvalues"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        values__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "values"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Value)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"values")) ::
              Data.ProtoLens.FieldDescriptor ReadRow
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, values__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ReadRow'_unknownFields
        (\ x__ y__ -> x__ {_ReadRow'_unknownFields = y__})
  defMessage
    = ReadRow'_constructor
        {_ReadRow'values = Data.Vector.Generic.empty,
         _ReadRow'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ReadRow
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Proto.Google.Protobuf.Struct.Value
             -> Data.ProtoLens.Encoding.Bytes.Parser ReadRow
        loop x mutable'values
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'values <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                         (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                            mutable'values)
                      (let missing = []
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
                           Data.ProtoLens.unknownFields (\ !t -> Prelude.reverse t)
                           (Lens.Family2.set
                              (Data.ProtoLens.Field.field @"vec'values") frozen'values x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "values"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'values y)
                                loop x v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'values
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'values <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                  Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'values)
          "ReadRow"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (Data.ProtoLens.Encoding.Bytes.foldMapBuilder
                (\ _v
                   -> (Data.Monoid.<>)
                        (Data.ProtoLens.Encoding.Bytes.putVarInt 10)
                        ((Prelude..)
                           (\ bs
                              -> (Data.Monoid.<>)
                                   (Data.ProtoLens.Encoding.Bytes.putVarInt
                                      (Prelude.fromIntegral (Data.ByteString.length bs)))
                                   (Data.ProtoLens.Encoding.Bytes.putBytes bs))
                           Data.ProtoLens.encodeMessage _v))
                (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'values") _x))
             (Data.ProtoLens.Encoding.Wire.buildFieldSet
                (Lens.Family2.view Data.ProtoLens.unknownFields _x))
instance Control.DeepSeq.NFData ReadRow where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ReadRow'_unknownFields x__)
             (Control.DeepSeq.deepseq (_ReadRow'values x__) ())
data PluginService = PluginService {}
instance Data.ProtoLens.Service.Types.Service PluginService where
  type ServiceName PluginService = "PluginService"
  type ServicePackage PluginService = "coremesh.plugin.v1"
  type ServiceMethods PluginService = '["configure",
                                        "getManifest",
                                        "handle",
                                        "read"]
  packedServiceDescriptor _
    = "\n\
      \\rPluginService\DC2^\n\
      \\vGetManifest\DC2&.coremesh.plugin.v1.GetManifestRequest\SUB'.coremesh.plugin.v1.GetManifestResponse\DC2X\n\
      \\tConfigure\DC2$.coremesh.plugin.v1.ConfigureRequest\SUB%.coremesh.plugin.v1.ConfigureResponse\DC2O\n\
      \\ACKHandle\DC2!.coremesh.plugin.v1.HandleRequest\SUB\".coremesh.plugin.v1.HandleResponse\DC2M\n\
      \\EOTRead\DC2!.coremesh.plugin.v1.HandleRequest\SUB .coremesh.plugin.v1.ReadResponse0\SOH"
instance Data.ProtoLens.Service.Types.HasMethodImpl PluginService "getManifest" where
  type MethodName PluginService "getManifest" = "GetManifest"
  type MethodInput PluginService "getManifest" = GetManifestRequest
  type MethodOutput PluginService "getManifest" = GetManifestResponse
  type MethodStreamingType PluginService "getManifest" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl PluginService "configure" where
  type MethodName PluginService "configure" = "Configure"
  type MethodInput PluginService "configure" = ConfigureRequest
  type MethodOutput PluginService "configure" = ConfigureResponse
  type MethodStreamingType PluginService "configure" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl PluginService "handle" where
  type MethodName PluginService "handle" = "Handle"
  type MethodInput PluginService "handle" = HandleRequest
  type MethodOutput PluginService "handle" = HandleResponse
  type MethodStreamingType PluginService "handle" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl PluginService "read" where
  type MethodName PluginService "read" = "Read"
  type MethodInput PluginService "read" = HandleRequest
  type MethodOutput PluginService "read" = ReadResponse
  type MethodStreamingType PluginService "read" = 'Data.ProtoLens.Service.Types.ServerStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\SYNplugin/v1/plugin.proto\DC2\DC2coremesh.plugin.v1\SUB\FSgoogle/protobuf/struct.proto\"\219\STX\n\
    \\aContext\DC2\GS\n\
    \\n\
    \request_id\CAN\SOH \SOH(\tR\trequestId\DC2\ESC\n\
    \\ttenant_id\CAN\STX \SOH(\tR\btenantId\DC2\ETB\n\
    \\auser_id\CAN\ETX \SOH(\tR\ACKuserId\DC2E\n\
    \\bmetadata\CAN\EOT \ETX(\v2).coremesh.plugin.v1.Context.MetadataEntryR\bmetadata\DC2=\n\
    \\ACKtx_ids\CAN\ENQ \ETX(\v2&.coremesh.plugin.v1.Context.TxIdsEntryR\ENQtxIds\SUB;\n\
    \\rMetadataEntry\DC2\DLE\n\
    \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
    \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH\SUB8\n\
    \\n\
    \TxIdsEntry\DC2\DLE\n\
    \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
    \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH\"\158\SOH\n\
    \\bManifest\DC2\DC2\n\
    \\EOTname\CAN\SOH \SOH(\tR\EOTname\DC2\CAN\n\
    \\aversion\CAN\STX \SOH(\tR\aversion\DC2 \n\
    \\vdescription\CAN\ETX \SOH(\tR\vdescription\DC2B\n\
    \\fcapabilities\CAN\EOT \ETX(\v2\RS.coremesh.plugin.v1.CapabilityR\fcapabilities\"\131\SOH\n\
    \\n\
    \Capability\DC2\SYN\n\
    \\ACKobject\CAN\SOH \SOH(\tR\ACKobject\DC2\CAN\n\
    \\aactions\CAN\STX \ETX(\tR\aactions\DC2 \n\
    \\vdescription\CAN\ETX \SOH(\tR\vdescription\DC2!\n\
    \\fread_actions\CAN\EOT \ETX(\tR\vreadActions\"K\n\
    \\DC2GetManifestRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\"O\n\
    \\DC3GetManifestResponse\DC28\n\
    \\bmanifest\CAN\SOH \SOH(\v2\FS.coremesh.plugin.v1.ManifestR\bmanifest\"\179\SOH\n\
    \\DLEConfigureRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC23\n\
    \\bsettings\CAN\STX \SOH(\v2\ETB.google.protobuf.StructR\bsettings\DC23\n\
    \\SYNhost_service_broker_id\CAN\ETX \SOH(\rR\DC3hostServiceBrokerId\"\DC3\n\
    \\DC1ConfigureResponse\"\168\SOH\n\
    \\rHandleRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SYN\n\
    \\ACKobject\CAN\STX \SOH(\tR\ACKobject\DC2\SYN\n\
    \\ACKaction\CAN\ETX \SOH(\tR\ACKaction\DC20\n\
    \\apayload\CAN\EOT \SOH(\v2\SYN.google.protobuf.ValueR\apayload\"y\n\
    \\SO\&HandleResponse\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC20\n\
    \\apayload\CAN\STX \SOH(\v2\SYN.google.protobuf.ValueR\apayload\"\239\SOH\n\
    \\fReadResponse\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC28\n\
    \\ACKheader\CAN\STX \SOH(\v2\RS.coremesh.plugin.v1.ReadHeaderH\NULR\ACKheader\DC25\n\
    \\ENQbatch\CAN\ETX \SOH(\v2\GS.coremesh.plugin.v1.ReadBatchH\NULR\ENQbatch\DC2/\n\
    \\ETXend\CAN\EOT \SOH(\v2\ESC.coremesh.plugin.v1.ReadEndH\NULR\ETXendB\ACK\n\
    \\EOTpart\"\173\SOH\n\
    \\n\
    \ReadHeader\DC2\CAN\n\
    \\acolumns\CAN\SOH \ETX(\tR\acolumns\DC2H\n\
    \\bmetadata\CAN\STX \ETX(\v2,.coremesh.plugin.v1.ReadHeader.MetadataEntryR\bmetadata\SUB;\n\
    \\rMetadataEntry\DC2\DLE\n\
    \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
    \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH\"<\n\
    \\tReadBatch\DC2/\n\
    \\EOTrows\CAN\SOH \ETX(\v2\ESC.coremesh.plugin.v1.ReadRowR\EOTrows\"9\n\
    \\aReadRow\DC2.\n\
    \\ACKvalues\CAN\SOH \ETX(\v2\SYN.google.protobuf.ValueR\ACKvalues\"\185\SOH\n\
    \\aReadEnd\DC2\DC2\n\
    \\EOTrows\CAN\SOH \SOH(\ETXR\EOTrows\DC2\SYN\n\
    \\ACKcursor\CAN\STX \SOH(\tR\ACKcursor\DC2E\n\
    \\bmetadata\CAN\ETX \ETX(\v2).coremesh.plugin.v1.ReadEnd.MetadataEntryR\bmetadata\SUB;\n\
    \\rMetadataEntry\DC2\DLE\n\
    \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
    \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH2\233\STX\n\
    \\rPluginService\DC2^\n\
    \\vGetManifest\DC2&.coremesh.plugin.v1.GetManifestRequest\SUB'.coremesh.plugin.v1.GetManifestResponse\DC2X\n\
    \\tConfigure\DC2$.coremesh.plugin.v1.ConfigureRequest\SUB%.coremesh.plugin.v1.ConfigureResponse\DC2O\n\
    \\ACKHandle\DC2!.coremesh.plugin.v1.HandleRequest\SUB\".coremesh.plugin.v1.HandleResponse\DC2M\n\
    \\EOTRead\DC2!.coremesh.plugin.v1.HandleRequest\SUB .coremesh.plugin.v1.ReadResponse0\SOHBCZAgithub.com/coremesh-labs/coremesh/internal/api/plugin/v1;pluginv1J\180?\n\
    \\a\DC2\ENQ\NUL\NUL\181\SOH\SOH\n\
    \\b\n\
    \\SOH\f\DC2\ETX\NUL\NUL\DC2\n\
    \\b\n\
    \\SOH\STX\DC2\ETX\STX\NUL\ESC\n\
    \\t\n\
    \\STX\ETX\NUL\DC2\ETX\EOT\NUL&\n\
    \\b\n\
    \\SOH\b\DC2\ETX\ACK\NULX\n\
    \\t\n\
    \\STX\b\v\DC2\ETX\ACK\NULX\n\
    \\255\ACK\n\
    \\STX\ACK\NUL\DC2\EOT\ETB\NUL.\SOH\SUB\242\ACK PluginService wird vom Plugin implementiert und vom Host aufgerufen.\n\
    \\n\
    \ Lebenszyklus: GetManifest -> Configure -> Handle (beliebig oft).\n\
    \ Start, Health-Checks und Beenden der Prozesse \195\188bernimmt HashiCorp go-plugin.\n\
    \ Fehler werden als gRPC-Status gemeldet (z. B. InvalidArgument, NotFound,\n\
    \ PermissionDenied), nicht als Feld in den Antworten.\n\
    \\n\
    \ Dispatcher-Match-Logik:\n\
    \   Der Dispatcher leitet ein HandleRequest an genau das Plugin weiter, dessen\n\
    \   Manifest eine Capability mit identischem `object` enth\195\164lt, in deren\n\
    \   `actions` die angefragte `action` vorkommt. Der Vergleich ist exakt und\n\
    \   case-sensitiv; Wildcards gibt es nicht. Jedes Paar (object, action) darf\n\
    \   host-weit nur einem Plugin geh\195\182ren \226\128\147 meldet ein zweites Plugin dasselbe\n\
    \   Paar, lehnt der Host dessen Registrierung ab. Findet sich kein Treffer,\n\
    \   antwortet der Dispatcher mit gRPC-Status Unimplemented.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\ETB\b\NAK\n\
    \\170\SOH\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX\SUB\STXD\SUB\156\SOH GetManifest liefert Identit\195\164t und F\195\164higkeiten. Wird direkt nach dem Start\n\
    \ aufgerufen; der Dispatcher baut daraus seine (object, action)-Routingtabelle.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX\SUB\ACK\DC1\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX\SUB\DC2$\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX\SUB/B\n\
    \\161\SOH\n\
    \\EOT\ACK\NUL\STX\SOH\DC2\ETX\RS\STX>\SUB\147\SOH Configure \195\188bergibt die pluginspezifischen Einstellungen aus der Host-Config\n\
    \ sowie die Broker-ID, unter der das Plugin den HostService erreicht.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\SOH\DC2\ETX\RS\ACK\SI\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\STX\DC2\ETX\RS\DLE \n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\ETX\DC2\ETX\RS+<\n\
    \W\n\
    \\EOT\ACK\NUL\STX\STX\DC2\ETX!\STX5\SUBJ Handle verarbeitet eine vom Dispatcher weitergeleitete (object, action).\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\SOH\DC2\ETX!\ACK\f\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\STX\DC2\ETX!\r\SUB\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\ETX\DC2\ETX!%3\n\
    \\151\ENQ\n\
    \\EOT\ACK\NUL\STX\ETX\DC2\ETX-\STX8\SUB\137\ENQ Read liefert das Ergebnis einer (object, action) als Datenstrom \226\128\147 f\195\188r\n\
    \ gro\195\159e Datenmengen, z. B. alle Einzelposten eines Gesch\195\164ftsjahres f\195\188r ein\n\
    \ Rechenmodul. Routing, Mandant, Benutzer und Berechtigung wie bei Handle;\n\
    \ die Action muss im Manifest zus\195\164tzlich unter Capability.read_actions\n\
    \ stehen, sonst antwortet der Dispatcher mit Unimplemented.\n\
    \\n\
    \ Ablauf des Stroms (siehe ReadResponse): genau ein header, dann beliebig\n\
    \ viele batch, zum Schluss genau ein end. Ein Fehler mitten im Strom endet\n\
    \ als gRPC-Status; bereits gelieferte Zeilen sind dann unvollst\195\164ndig.\n\
    \ Gegendruck regelt gRPC: Liest der Empf\195\164nger langsamer, wartet der Sender.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\SOH\DC2\ETX-\ACK\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\STX\DC2\ETX-\v\CAN\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\ACK\DC2\ETX-#)\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\ETX\DC2\ETX-*6\n\
    \\134\STX\n\
    \\STX\EOT\NUL\DC2\EOT5\NULG\SOH\SUB\249\SOH Context begleitet jeden Aufruf \195\188ber die Prozessgrenze hinweg.\n\
    \\n\
    \ Die Felder werden ausschlie\195\159lich vom Host gesetzt (aus Authentifizierung und\n\
    \ Routing). Plugins vertrauen diesen Werten und leiten Mandant oder Benutzer\n\
    \ niemals aus dem Payload ab.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETX5\b\SI\n\
    \r\n\
    \\EOT\EOT\NUL\STX\NUL\DC2\ETX8\STX\CAN\SUBe Korrelations-ID f\195\188r Logging und Tracing. Ein Plugin gibt sie in der\n\
    \ Antwort unver\195\164ndert zur\195\188ck.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ENQ\DC2\ETX8\STX\b\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\SOH\DC2\ETX8\t\DC3\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ETX\DC2\ETX8\SYN\ETB\n\
    \t\n\
    \\EOT\EOT\NUL\STX\SOH\DC2\ETX;\STX\ETB\SUBg Mandant, in dessen Namen der Aufruf erfolgt. Leer bei host-weiten\n\
    \ Aufrufen (GetManifest, Configure).\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ENQ\DC2\ETX;\STX\b\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\SOH\DC2\ETX;\t\DC2\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ETX\DC2\ETX;\NAK\SYN\n\
    \U\n\
    \\EOT\EOT\NUL\STX\STX\DC2\ETX=\STX\NAK\SUBH Authentifizierter Benutzer. Leer bei System- und host-weiten Aufrufen.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ENQ\DC2\ETX=\STX\b\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\SOH\DC2\ETX=\t\DLE\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ETX\DC2\ETX=\DC3\DC4\n\
    \\162\SOH\n\
    \\EOT\EOT\NUL\STX\ETX\DC2\ETX@\STX#\SUB\148\SOH Flexible Header, z. B. \"locale\", \"traceparent\". Schl\195\188ssel in\n\
    \ Kleinbuchstaben; f\195\188r die Fachlogik nicht vorgesehen \226\128\147 daf\195\188r ist der Payload da.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ACK\DC2\ETX@\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\SOH\DC2\ETX@\SYN\RS\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ETX\DC2\ETX@!\"\n\
    \\243\STX\n\
    \\EOT\EOT\NUL\STX\EOT\DC2\ETXF\STX!\SUB\229\STX Laufende Transaktionen dieser Aufrufkette: logischer Datenbankname ->\n\
    \ tx_id (von HostService.BeginTx vergeben). Wird mit jedem Query, Exec und\n\
    \ Dispatch mitgeschickt, sodass auch \195\188ber Dispatch aufgerufene Plugins in\n\
    \ derselben Transaktion arbeiten. Der Host pr\195\188ft jede tx_id gegen\n\
    \ request_id und Datenbank; unbekannte oder fremde IDs werden abgelehnt.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\EOT\ACK\DC2\ETXF\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\EOT\SOH\DC2\ETXF\SYN\FS\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\EOT\ETX\DC2\ETXF\US \n\
    \\n\
    \\n\
    \\STX\EOT\SOH\DC2\EOTI\NULQ\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\SOH\SOH\DC2\ETXI\b\DLE\n\
    \M\n\
    \\EOT\EOT\SOH\STX\NUL\DC2\ETXK\STX\DC2\SUB@ Eindeutiger Name, z. B. \"partner-service\". Erlaubt: [a-z0-9-].\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\ENQ\DC2\ETXK\STX\b\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\SOH\DC2\ETXK\t\r\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\NUL\ETX\DC2\ETXK\DLE\DC1\n\
    \>\n\
    \\EOT\EOT\SOH\STX\SOH\DC2\ETXM\STX\NAK\SUB1 Semantische Version des Plugins, z. B. \"1.2.0\".\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\SOH\ENQ\DC2\ETXM\STX\b\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\SOH\SOH\DC2\ETXM\t\DLE\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\SOH\ETX\DC2\ETXM\DC3\DC4\n\
    \\v\n\
    \\EOT\EOT\SOH\STX\STX\DC2\ETXN\STX\EM\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\STX\ENQ\DC2\ETXN\STX\b\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\STX\SOH\DC2\ETXN\t\DC4\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\STX\ETX\DC2\ETXN\ETB\CAN\n\
    \H\n\
    \\EOT\EOT\SOH\STX\ETX\DC2\ETXP\STX'\SUB; Business-Objects und Aktionen, die dieses Plugin bedient.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\ETX\EOT\DC2\ETXP\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\ETX\ACK\DC2\ETXP\v\NAK\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\ETX\SOH\DC2\ETXP\SYN\"\n\
    \\f\n\
    \\ENQ\EOT\SOH\STX\ETX\ETX\DC2\ETXP%&\n\
    \\169\SOH\n\
    \\STX\EOT\STX\DC2\EOTV\NUL`\SOH\SUB\156\SOH Capability beschreibt ein Business-Object und die darauf unterst\195\188tzten\n\
    \ Aktionen. Jedes Paar (object, actions[i]) ist ein Routing-Eintrag im\n\
    \ Dispatcher.\n\
    \\n\
    \\n\
    \\n\
    \\ETX\EOT\STX\SOH\DC2\ETXV\b\DC2\n\
    \T\n\
    \\EOT\EOT\STX\STX\NUL\DC2\ETXX\STX\DC4\SUBG Business-Object in PascalCase, z. B. \"BusinessPartner\", \"SalesOrder\".\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\ENQ\DC2\ETXX\STX\b\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\SOH\DC2\ETXX\t\SI\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\ETX\DC2\ETXX\DC2\DC3\n\
    \\162\SOH\n\
    \\EOT\EOT\STX\STX\SOH\DC2\ETX[\STX\RS\SUB\148\SOH Aktionen auf diesem Objekt in camelCase, z. B. [\"get\", \"list\", \"create\"].\n\
    \ Muss mindestens einen Eintrag enthalten und darf keine Duplikate haben.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\EOT\DC2\ETX[\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\ENQ\DC2\ETX[\v\DC1\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\SOH\DC2\ETX[\DC2\EM\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\ETX\DC2\ETX[\FS\GS\n\
    \\v\n\
    \\EOT\EOT\STX\STX\STX\DC2\ETX\\\STX\EM\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\ENQ\DC2\ETX\\\STX\b\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\SOH\DC2\ETX\\\t\DC4\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\ETX\DC2\ETX\\\ETB\CAN\n\
    \\152\SOH\n\
    \\EOT\EOT\STX\STX\ETX\DC2\ETX_\STX#\SUB\138\SOH Teilmenge von actions, die zus\195\164tzlich \195\188ber Read als Datenstrom abrufbar\n\
    \ sind (z. B. [\"list\"]). Berechtigt wird \195\188ber dieselbe Action.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\EOT\DC2\ETX_\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\ENQ\DC2\ETX_\v\DC1\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\SOH\DC2\ETX_\DC2\RS\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\ETX\DC2\ETX_!\"\n\
    \\n\
    \\n\
    \\STX\EOT\ETX\DC2\EOTb\NULe\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\ETX\SOH\DC2\ETXb\b\SUB\n\
    \*\n\
    \\EOT\EOT\ETX\STX\NUL\DC2\ETXd\STX\SYN\SUB\GS Nur request_id ist gesetzt.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\ACK\DC2\ETXd\STX\t\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\SOH\DC2\ETXd\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\ETX\DC2\ETXd\DC4\NAK\n\
    \\n\
    \\n\
    \\STX\EOT\EOT\DC2\EOTg\NULi\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\EOT\SOH\DC2\ETXg\b\ESC\n\
    \\v\n\
    \\EOT\EOT\EOT\STX\NUL\DC2\ETXh\STX\CAN\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\ACK\DC2\ETXh\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\SOH\DC2\ETXh\v\DC3\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\ETX\DC2\ETXh\SYN\ETB\n\
    \\n\
    \\n\
    \\STX\EOT\ENQ\DC2\EOTk\NULr\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\ENQ\SOH\DC2\ETXk\b\CAN\n\
    \*\n\
    \\EOT\EOT\ENQ\STX\NUL\DC2\ETXm\STX\SYN\SUB\GS Nur request_id ist gesetzt.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\ACK\DC2\ETXm\STX\t\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\SOH\DC2\ETXm\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\ETX\DC2\ETXm\DC4\NAK\n\
    \I\n\
    \\EOT\EOT\ENQ\STX\SOH\DC2\ETXo\STX&\SUB< Abschnitt `plugins.<name>.settings` aus configs/host.yaml.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\ACK\DC2\ETXo\STX\CAN\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\SOH\DC2\ETXo\EM!\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\ETX\DC2\ETXo$%\n\
    \P\n\
    \\EOT\EOT\ENQ\STX\STX\DC2\ETXq\STX$\SUBC go-plugin GRPCBroker-ID f\195\188r die R\195\188ckverbindung zum HostService.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\ENQ\DC2\ETXq\STX\b\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\SOH\DC2\ETXq\t\US\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\ETX\DC2\ETXq\"#\n\
    \\t\n\
    \\STX\EOT\ACK\DC2\ETXt\NUL\FS\n\
    \\n\
    \\n\
    \\ETX\EOT\ACK\SOH\DC2\ETXt\b\EM\n\
    \\v\n\
    \\STX\EOT\a\DC2\ENQv\NUL\130\SOH\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\a\SOH\DC2\ETXv\b\NAK\n\
    \F\n\
    \\EOT\EOT\a\STX\NUL\DC2\ETXx\STX\SYN\SUB9 Aufrufkontext (Korrelation, Mandant, Benutzer, Header).\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\a\STX\NUL\ACK\DC2\ETXx\STX\t\n\
    \\f\n\
    \\ENQ\EOT\a\STX\NUL\SOH\DC2\ETXx\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\a\STX\NUL\ETX\DC2\ETXx\DC4\NAK\n\
    \~\n\
    \\EOT\EOT\a\STX\SOH\DC2\ETX{\STX\DC4\SUBq Business-Object, z. B. \"BusinessPartner\". Bildet zusammen mit `action`\n\
    \ den Routing-Schl\195\188ssel des Dispatchers.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\a\STX\SOH\ENQ\DC2\ETX{\STX\b\n\
    \\f\n\
    \\ENQ\EOT\a\STX\SOH\SOH\DC2\ETX{\t\SI\n\
    \\f\n\
    \\ENQ\EOT\a\STX\SOH\ETX\DC2\ETX{\DC2\DC3\n\
    \;\n\
    \\EOT\EOT\a\STX\STX\DC2\ETX}\STX\DC4\SUB. Aktion auf dem Business-Object, z. B. \"get\".\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\a\STX\STX\ENQ\DC2\ETX}\STX\b\n\
    \\f\n\
    \\ENQ\EOT\a\STX\STX\SOH\DC2\ETX}\t\SI\n\
    \\f\n\
    \\ENQ\EOT\a\STX\STX\ETX\DC2\ETX}\DC2\DC3\n\
    \\217\SOH\n\
    \\EOT\EOT\a\STX\ETX\DC2\EOT\129\SOH\STX$\SUB\202\SOH Fachliche Eingabedaten, frei strukturiert (Objekt, Liste, Skalar oder\n\
    \ null). Das erwartete Schema legt das Plugin pro (object, action) fest;\n\
    \ passt es nicht, antwortet das Plugin mit InvalidArgument.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\ACK\DC2\EOT\129\SOH\STX\ETB\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\SOH\DC2\EOT\129\SOH\CAN\US\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\ETX\DC2\EOT\129\SOH\"#\n\
    \\f\n\
    \\STX\EOT\b\DC2\ACK\132\SOH\NUL\138\SOH\SOH\n\
    \\v\n\
    \\ETX\EOT\b\SOH\DC2\EOT\132\SOH\b\SYN\n\
    \\156\SOH\n\
    \\EOT\EOT\b\STX\NUL\DC2\EOT\135\SOH\STX\SYN\SUB\141\SOH Kontext der Anfrage; request_id unver\195\164ndert, metadata darf das Plugin\n\
    \ erg\195\164nzen. tenant_id und user_id ignoriert der Host in der Antwort.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\ACK\DC2\EOT\135\SOH\STX\t\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\SOH\DC2\EOT\135\SOH\n\
    \\DC1\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\ETX\DC2\EOT\135\SOH\DC4\NAK\n\
    \7\n\
    \\EOT\EOT\b\STX\SOH\DC2\EOT\137\SOH\STX$\SUB) Fachliches Ergebnis, frei strukturiert.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\b\STX\SOH\ACK\DC2\EOT\137\SOH\STX\ETB\n\
    \\r\n\
    \\ENQ\EOT\b\STX\SOH\SOH\DC2\EOT\137\SOH\CAN\US\n\
    \\r\n\
    \\ENQ\EOT\b\STX\SOH\ETX\DC2\EOT\137\SOH\"#\n\
    \f\n\
    \\STX\EOT\t\DC2\ACK\142\SOH\NUL\150\SOH\SOH\SUBX ReadResponse ist eine Nachricht im Datenstrom von Read bzw.\n\
    \ HostService.DispatchRead.\n\
    \\n\
    \\v\n\
    \\ETX\EOT\t\SOH\DC2\EOT\142\SOH\b\DC4\n\
    \J\n\
    \\EOT\EOT\t\STX\NUL\DC2\EOT\144\SOH\STX\SYN\SUB< Kontext der Anfrage (nur in der ersten Nachricht gesetzt).\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\ACK\DC2\EOT\144\SOH\STX\t\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\SOH\DC2\EOT\144\SOH\n\
    \\DC1\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\ETX\DC2\EOT\144\SOH\DC4\NAK\n\
    \\SO\n\
    \\EOT\EOT\t\b\NUL\DC2\ACK\145\SOH\STX\149\SOH\ETX\n\
    \\r\n\
    \\ENQ\EOT\t\b\NUL\SOH\DC2\EOT\145\SOH\b\f\n\
    \\f\n\
    \\EOT\EOT\t\STX\SOH\DC2\EOT\146\SOH\EOT\SUB\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\ACK\DC2\EOT\146\SOH\EOT\SO\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\SOH\DC2\EOT\146\SOH\SI\NAK\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\ETX\DC2\EOT\146\SOH\CAN\EM\n\
    \\f\n\
    \\EOT\EOT\t\STX\STX\DC2\EOT\147\SOH\EOT\CAN\n\
    \\r\n\
    \\ENQ\EOT\t\STX\STX\ACK\DC2\EOT\147\SOH\EOT\r\n\
    \\r\n\
    \\ENQ\EOT\t\STX\STX\SOH\DC2\EOT\147\SOH\SO\DC3\n\
    \\r\n\
    \\ENQ\EOT\t\STX\STX\ETX\DC2\EOT\147\SOH\SYN\ETB\n\
    \\f\n\
    \\EOT\EOT\t\STX\ETX\DC2\EOT\148\SOH\EOT\DC4\n\
    \\r\n\
    \\ENQ\EOT\t\STX\ETX\ACK\DC2\EOT\148\SOH\EOT\v\n\
    \\r\n\
    \\ENQ\EOT\t\STX\ETX\SOH\DC2\EOT\148\SOH\f\SI\n\
    \\r\n\
    \\ENQ\EOT\t\STX\ETX\ETX\DC2\EOT\148\SOH\DC2\DC3\n\
    \D\n\
    \\STX\EOT\n\
    \\DC2\ACK\153\SOH\NUL\158\SOH\SOH\SUB6 ReadHeader kommt genau einmal, vor dem ersten batch.\n\
    \\n\
    \\v\n\
    \\ETX\EOT\n\
    \\SOH\DC2\EOT\153\SOH\b\DC2\n\
    \Z\n\
    \\EOT\EOT\n\
    \\STX\NUL\DC2\EOT\155\SOH\STX\RS\SUBL Spaltennamen; jede Zeile eines batch hat ihre Werte in dieser Reihenfolge.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\NUL\EOT\DC2\EOT\155\SOH\STX\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\NUL\ENQ\DC2\EOT\155\SOH\v\DC1\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\NUL\SOH\DC2\EOT\155\SOH\DC2\EM\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\NUL\ETX\DC2\EOT\155\SOH\FS\GS\n\
    \Y\n\
    \\EOT\EOT\n\
    \\STX\SOH\DC2\EOT\157\SOH\STX#\SUBK Frei, z. B. Typen der Spalten oder Quelle; Schl\195\188ssel in Kleinbuchstaben.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\SOH\ACK\DC2\EOT\157\SOH\STX\NAK\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\SOH\SOH\DC2\EOT\157\SOH\SYN\RS\n\
    \\r\n\
    \\ENQ\EOT\n\
    \\STX\SOH\ETX\DC2\EOT\157\SOH!\"\n\
    \\140\SOH\n\
    \\STX\EOT\v\DC2\ACK\162\SOH\NUL\164\SOH\SOH\SUB~ ReadBatch ist ein Block von Zeilen. Der Sender teilt so, dass eine\n\
    \ Nachricht deutlich unter der gRPC-Grenze (4 MiB) bleibt.\n\
    \\n\
    \\v\n\
    \\ETX\EOT\v\SOH\DC2\EOT\162\SOH\b\DC1\n\
    \\f\n\
    \\EOT\EOT\v\STX\NUL\DC2\EOT\163\SOH\STX\FS\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\EOT\DC2\EOT\163\SOH\STX\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\ACK\DC2\EOT\163\SOH\v\DC2\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\SOH\DC2\EOT\163\SOH\DC3\ETB\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\ETX\DC2\EOT\163\SOH\SUB\ESC\n\
    \\f\n\
    \\STX\EOT\f\DC2\ACK\166\SOH\NUL\170\SOH\SOH\n\
    \\v\n\
    \\ETX\EOT\f\SOH\DC2\EOT\166\SOH\b\SI\n\
    \\DEL\n\
    \\EOT\EOT\f\STX\NUL\DC2\EOT\169\SOH\STX,\SUBq Werte wie im Payload: Zahlen als double (Ganzzahlen \195\188ber 2^53 als Text),\n\
    \ Datum als Text, NULL als null_value.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\f\STX\NUL\EOT\DC2\EOT\169\SOH\STX\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\f\STX\NUL\ACK\DC2\EOT\169\SOH\v \n\
    \\r\n\
    \\ENQ\EOT\f\STX\NUL\SOH\DC2\EOT\169\SOH!'\n\
    \\r\n\
    \\ENQ\EOT\f\STX\NUL\ETX\DC2\EOT\169\SOH*+\n\
    \?\n\
    \\STX\EOT\r\DC2\ACK\173\SOH\NUL\181\SOH\SOH\SUB1 ReadEnd schlie\195\159t einen erfolgreichen Strom ab.\n\
    \\n\
    \\v\n\
    \\ETX\EOT\r\SOH\DC2\EOT\173\SOH\b\SI\n\
    \*\n\
    \\EOT\EOT\r\STX\NUL\DC2\EOT\175\SOH\STX\DC1\SUB\FS Anzahl gelieferter Zeilen.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\r\STX\NUL\ENQ\DC2\EOT\175\SOH\STX\a\n\
    \\r\n\
    \\ENQ\EOT\r\STX\NUL\SOH\DC2\EOT\175\SOH\b\f\n\
    \\r\n\
    \\ENQ\EOT\r\STX\NUL\ETX\DC2\EOT\175\SOH\SI\DLE\n\
    \\179\SOH\n\
    \\EOT\EOT\r\STX\SOH\DC2\EOT\179\SOH\STX\DC4\SUB\164\SOH Fortsetzungsmarke: gesetzt, wenn der Anbieter nach einer Grenze (z. B.\n\
    \ limit) aufgeh\195\182rt hat; ein neuer Read mit diesem Wert (Payload \"after\")\n\
    \ liefert den Rest.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\r\STX\SOH\ENQ\DC2\EOT\179\SOH\STX\b\n\
    \\r\n\
    \\ENQ\EOT\r\STX\SOH\SOH\DC2\EOT\179\SOH\t\SI\n\
    \\r\n\
    \\ENQ\EOT\r\STX\SOH\ETX\DC2\EOT\179\SOH\DC2\DC3\n\
    \\f\n\
    \\EOT\EOT\r\STX\STX\DC2\EOT\180\SOH\STX#\n\
    \\r\n\
    \\ENQ\EOT\r\STX\STX\ACK\DC2\EOT\180\SOH\STX\NAK\n\
    \\r\n\
    \\ENQ\EOT\r\STX\STX\SOH\DC2\EOT\180\SOH\SYN\RS\n\
    \\r\n\
    \\ENQ\EOT\r\STX\STX\ETX\DC2\EOT\180\SOH!\"b\ACKproto3"