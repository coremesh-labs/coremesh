{- This file was auto-generated from plugin/v1/host.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Plugin.V1.Host (
        HostService(..), BeginTxRequest(), BeginTxResponse(),
        CommitTxRequest(), CommitTxResponse(), ExecRequest(),
        ExecResponse(), IsolationLevel(..), IsolationLevel(),
        IsolationLevel'UnrecognizedValue, LogLevel(..), LogLevel(),
        LogLevel'UnrecognizedValue, LogRequest(), LogRequest'FieldsEntry(),
        LogResponse(), QueryRequest(), QueryResponse(),
        RollbackTxRequest(), RollbackTxResponse(), Row()
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
import qualified Proto.Plugin.V1.Plugin
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' BeginTxRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' BeginTxRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.database' @:: Lens' BeginTxRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.isolation' @:: Lens' BeginTxRequest IsolationLevel@
         * 'Proto.Plugin.V1.Host_Fields.readOnly' @:: Lens' BeginTxRequest Prelude.Bool@ -}
data BeginTxRequest
  = BeginTxRequest'_constructor {_BeginTxRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                                 _BeginTxRequest'database :: !Data.Text.Text,
                                 _BeginTxRequest'isolation :: !IsolationLevel,
                                 _BeginTxRequest'readOnly :: !Prelude.Bool,
                                 _BeginTxRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show BeginTxRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField BeginTxRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxRequest'context
           (\ x__ y__ -> x__ {_BeginTxRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField BeginTxRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxRequest'context
           (\ x__ y__ -> x__ {_BeginTxRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField BeginTxRequest "database" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxRequest'database
           (\ x__ y__ -> x__ {_BeginTxRequest'database = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField BeginTxRequest "isolation" IsolationLevel where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxRequest'isolation
           (\ x__ y__ -> x__ {_BeginTxRequest'isolation = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField BeginTxRequest "readOnly" Prelude.Bool where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxRequest'readOnly
           (\ x__ y__ -> x__ {_BeginTxRequest'readOnly = y__}))
        Prelude.id
instance Data.ProtoLens.Message BeginTxRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.BeginTxRequest"
  packedMessageDescriptor _
    = "\n\
      \\SOBeginTxRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
      \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2@\n\
      \\tisolation\CAN\ETX \SOH(\SO2\".coremesh.plugin.v1.IsolationLevelR\tisolation\DC2\ESC\n\
      \\tread_only\CAN\EOT \SOH(\bR\breadOnly"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor BeginTxRequest
        database__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "database"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"database")) ::
              Data.ProtoLens.FieldDescriptor BeginTxRequest
        isolation__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "isolation"
              (Data.ProtoLens.ScalarField Data.ProtoLens.EnumField ::
                 Data.ProtoLens.FieldTypeDescriptor IsolationLevel)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"isolation")) ::
              Data.ProtoLens.FieldDescriptor BeginTxRequest
        readOnly__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "read_only"
              (Data.ProtoLens.ScalarField Data.ProtoLens.BoolField ::
                 Data.ProtoLens.FieldTypeDescriptor Prelude.Bool)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"readOnly")) ::
              Data.ProtoLens.FieldDescriptor BeginTxRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, database__field_descriptor),
           (Data.ProtoLens.Tag 3, isolation__field_descriptor),
           (Data.ProtoLens.Tag 4, readOnly__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _BeginTxRequest'_unknownFields
        (\ x__ y__ -> x__ {_BeginTxRequest'_unknownFields = y__})
  defMessage
    = BeginTxRequest'_constructor
        {_BeginTxRequest'context = Prelude.Nothing,
         _BeginTxRequest'database = Data.ProtoLens.fieldDefault,
         _BeginTxRequest'isolation = Data.ProtoLens.fieldDefault,
         _BeginTxRequest'readOnly = Data.ProtoLens.fieldDefault,
         _BeginTxRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          BeginTxRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser BeginTxRequest
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
                                       "database"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"database") y x)
                        24
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          Prelude.toEnum
                                          (Prelude.fmap
                                             Prelude.fromIntegral
                                             Data.ProtoLens.Encoding.Bytes.getVarInt))
                                       "isolation"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"isolation") y x)
                        32
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          ((Prelude./=) 0) Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "read_only"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"readOnly") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "BeginTxRequest"
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
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"database") _x
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
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"isolation") _x
                    in
                      if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                          Data.Monoid.mempty
                      else
                          (Data.Monoid.<>)
                            (Data.ProtoLens.Encoding.Bytes.putVarInt 24)
                            ((Prelude..)
                               ((Prelude..)
                                  Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral)
                               Prelude.fromEnum _v))
                   ((Data.Monoid.<>)
                      (let
                         _v = Lens.Family2.view (Data.ProtoLens.Field.field @"readOnly") _x
                       in
                         if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                             Data.Monoid.mempty
                         else
                             (Data.Monoid.<>)
                               (Data.ProtoLens.Encoding.Bytes.putVarInt 32)
                               ((Prelude..)
                                  Data.ProtoLens.Encoding.Bytes.putVarInt
                                  (\ b -> if b then 1 else 0) _v))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData BeginTxRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_BeginTxRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_BeginTxRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_BeginTxRequest'database x__)
                   (Control.DeepSeq.deepseq
                      (_BeginTxRequest'isolation x__)
                      (Control.DeepSeq.deepseq (_BeginTxRequest'readOnly x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.txId' @:: Lens' BeginTxResponse Data.Text.Text@ -}
data BeginTxResponse
  = BeginTxResponse'_constructor {_BeginTxResponse'txId :: !Data.Text.Text,
                                  _BeginTxResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show BeginTxResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField BeginTxResponse "txId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _BeginTxResponse'txId
           (\ x__ y__ -> x__ {_BeginTxResponse'txId = y__}))
        Prelude.id
instance Data.ProtoLens.Message BeginTxResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.BeginTxResponse"
  packedMessageDescriptor _
    = "\n\
      \\SIBeginTxResponse\DC2\DC3\n\
      \\ENQtx_id\CAN\SOH \SOH(\tR\EOTtxId"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        txId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "tx_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"txId")) ::
              Data.ProtoLens.FieldDescriptor BeginTxResponse
      in
        Data.Map.fromList [(Data.ProtoLens.Tag 1, txId__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _BeginTxResponse'_unknownFields
        (\ x__ y__ -> x__ {_BeginTxResponse'_unknownFields = y__})
  defMessage
    = BeginTxResponse'_constructor
        {_BeginTxResponse'txId = Data.ProtoLens.fieldDefault,
         _BeginTxResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          BeginTxResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser BeginTxResponse
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
                                       "tx_id"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"txId") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "BeginTxResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"txId") _x
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
instance Control.DeepSeq.NFData BeginTxResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_BeginTxResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq (_BeginTxResponse'txId x__) ())
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' CommitTxRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' CommitTxRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.txId' @:: Lens' CommitTxRequest Data.Text.Text@ -}
data CommitTxRequest
  = CommitTxRequest'_constructor {_CommitTxRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                                  _CommitTxRequest'txId :: !Data.Text.Text,
                                  _CommitTxRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show CommitTxRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField CommitTxRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _CommitTxRequest'context
           (\ x__ y__ -> x__ {_CommitTxRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField CommitTxRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _CommitTxRequest'context
           (\ x__ y__ -> x__ {_CommitTxRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField CommitTxRequest "txId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _CommitTxRequest'txId
           (\ x__ y__ -> x__ {_CommitTxRequest'txId = y__}))
        Prelude.id
instance Data.ProtoLens.Message CommitTxRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.CommitTxRequest"
  packedMessageDescriptor _
    = "\n\
      \\SICommitTxRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\DC3\n\
      \\ENQtx_id\CAN\STX \SOH(\tR\EOTtxId"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor CommitTxRequest
        txId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "tx_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"txId")) ::
              Data.ProtoLens.FieldDescriptor CommitTxRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, txId__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _CommitTxRequest'_unknownFields
        (\ x__ y__ -> x__ {_CommitTxRequest'_unknownFields = y__})
  defMessage
    = CommitTxRequest'_constructor
        {_CommitTxRequest'context = Prelude.Nothing,
         _CommitTxRequest'txId = Data.ProtoLens.fieldDefault,
         _CommitTxRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          CommitTxRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser CommitTxRequest
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
                                       "tx_id"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"txId") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "CommitTxRequest"
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
                (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"txId") _x
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
instance Control.DeepSeq.NFData CommitTxRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_CommitTxRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_CommitTxRequest'context x__)
                (Control.DeepSeq.deepseq (_CommitTxRequest'txId x__) ()))
{- | Fields :
      -}
data CommitTxResponse
  = CommitTxResponse'_constructor {_CommitTxResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show CommitTxResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Message CommitTxResponse where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.CommitTxResponse"
  packedMessageDescriptor _
    = "\n\
      \\DLECommitTxResponse"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag = let in Data.Map.fromList []
  unknownFields
    = Lens.Family2.Unchecked.lens
        _CommitTxResponse'_unknownFields
        (\ x__ y__ -> x__ {_CommitTxResponse'_unknownFields = y__})
  defMessage
    = CommitTxResponse'_constructor
        {_CommitTxResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          CommitTxResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser CommitTxResponse
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
          (do loop Data.ProtoLens.defMessage) "CommitTxResponse"
  buildMessage
    = \ _x
        -> Data.ProtoLens.Encoding.Wire.buildFieldSet
             (Lens.Family2.view Data.ProtoLens.unknownFields _x)
instance Control.DeepSeq.NFData CommitTxResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_CommitTxResponse'_unknownFields x__) ()
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' ExecRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' ExecRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.database' @:: Lens' ExecRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.sql' @:: Lens' ExecRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.args' @:: Lens' ExecRequest [Proto.Google.Protobuf.Struct.Value]@
         * 'Proto.Plugin.V1.Host_Fields.vec'args' @:: Lens' ExecRequest (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value)@ -}
data ExecRequest
  = ExecRequest'_constructor {_ExecRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                              _ExecRequest'database :: !Data.Text.Text,
                              _ExecRequest'sql :: !Data.Text.Text,
                              _ExecRequest'args :: !(Data.Vector.Vector Proto.Google.Protobuf.Struct.Value),
                              _ExecRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ExecRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ExecRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'context
           (\ x__ y__ -> x__ {_ExecRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField ExecRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'context
           (\ x__ y__ -> x__ {_ExecRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ExecRequest "database" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'database
           (\ x__ y__ -> x__ {_ExecRequest'database = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ExecRequest "sql" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'sql (\ x__ y__ -> x__ {_ExecRequest'sql = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ExecRequest "args" [Proto.Google.Protobuf.Struct.Value] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'args (\ x__ y__ -> x__ {_ExecRequest'args = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField ExecRequest "vec'args" (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecRequest'args (\ x__ y__ -> x__ {_ExecRequest'args = y__}))
        Prelude.id
instance Data.ProtoLens.Message ExecRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ExecRequest"
  packedMessageDescriptor _
    = "\n\
      \\vExecRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
      \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2\DLE\n\
      \\ETXsql\CAN\ETX \SOH(\tR\ETXsql\DC2*\n\
      \\EOTargs\CAN\EOT \ETX(\v2\SYN.google.protobuf.ValueR\EOTargs"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor ExecRequest
        database__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "database"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"database")) ::
              Data.ProtoLens.FieldDescriptor ExecRequest
        sql__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "sql"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"sql")) ::
              Data.ProtoLens.FieldDescriptor ExecRequest
        args__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "args"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Value)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"args")) ::
              Data.ProtoLens.FieldDescriptor ExecRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, database__field_descriptor),
           (Data.ProtoLens.Tag 3, sql__field_descriptor),
           (Data.ProtoLens.Tag 4, args__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ExecRequest'_unknownFields
        (\ x__ y__ -> x__ {_ExecRequest'_unknownFields = y__})
  defMessage
    = ExecRequest'_constructor
        {_ExecRequest'context = Prelude.Nothing,
         _ExecRequest'database = Data.ProtoLens.fieldDefault,
         _ExecRequest'sql = Data.ProtoLens.fieldDefault,
         _ExecRequest'args = Data.Vector.Generic.empty,
         _ExecRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ExecRequest
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Proto.Google.Protobuf.Struct.Value
             -> Data.ProtoLens.Encoding.Bytes.Parser ExecRequest
        loop x mutable'args
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'args <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.unsafeFreeze mutable'args)
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
                              (Data.ProtoLens.Field.field @"vec'args") frozen'args x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                                  mutable'args
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "database"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"database") y x)
                                  mutable'args
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "sql"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"sql") y x)
                                  mutable'args
                        34
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "args"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'args y)
                                loop x v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'args
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'args <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'args)
          "ExecRequest"
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
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"database") _x
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
                   (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"sql") _x
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
                         (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'args") _x))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData ExecRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ExecRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ExecRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_ExecRequest'database x__)
                   (Control.DeepSeq.deepseq
                      (_ExecRequest'sql x__)
                      (Control.DeepSeq.deepseq (_ExecRequest'args x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.rowsAffected' @:: Lens' ExecResponse Data.Int.Int64@
         * 'Proto.Plugin.V1.Host_Fields.lastInsertId' @:: Lens' ExecResponse Data.Int.Int64@ -}
data ExecResponse
  = ExecResponse'_constructor {_ExecResponse'rowsAffected :: !Data.Int.Int64,
                               _ExecResponse'lastInsertId :: !Data.Int.Int64,
                               _ExecResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ExecResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ExecResponse "rowsAffected" Data.Int.Int64 where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecResponse'rowsAffected
           (\ x__ y__ -> x__ {_ExecResponse'rowsAffected = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ExecResponse "lastInsertId" Data.Int.Int64 where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ExecResponse'lastInsertId
           (\ x__ y__ -> x__ {_ExecResponse'lastInsertId = y__}))
        Prelude.id
instance Data.ProtoLens.Message ExecResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.ExecResponse"
  packedMessageDescriptor _
    = "\n\
      \\fExecResponse\DC2#\n\
      \\rrows_affected\CAN\SOH \SOH(\ETXR\frowsAffected\DC2$\n\
      \\SOlast_insert_id\CAN\STX \SOH(\ETXR\flastInsertId"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        rowsAffected__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "rows_affected"
              (Data.ProtoLens.ScalarField Data.ProtoLens.Int64Field ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Int.Int64)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"rowsAffected")) ::
              Data.ProtoLens.FieldDescriptor ExecResponse
        lastInsertId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "last_insert_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.Int64Field ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Int.Int64)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"lastInsertId")) ::
              Data.ProtoLens.FieldDescriptor ExecResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, rowsAffected__field_descriptor),
           (Data.ProtoLens.Tag 2, lastInsertId__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ExecResponse'_unknownFields
        (\ x__ y__ -> x__ {_ExecResponse'_unknownFields = y__})
  defMessage
    = ExecResponse'_constructor
        {_ExecResponse'rowsAffected = Data.ProtoLens.fieldDefault,
         _ExecResponse'lastInsertId = Data.ProtoLens.fieldDefault,
         _ExecResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ExecResponse -> Data.ProtoLens.Encoding.Bytes.Parser ExecResponse
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
                                       "rows_affected"
                                loop
                                  (Lens.Family2.set
                                     (Data.ProtoLens.Field.field @"rowsAffected") y x)
                        16
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          Prelude.fromIntegral
                                          Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "last_insert_id"
                                loop
                                  (Lens.Family2.set
                                     (Data.ProtoLens.Field.field @"lastInsertId") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "ExecResponse"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v
                  = Lens.Family2.view (Data.ProtoLens.Field.field @"rowsAffected") _x
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
                   _v
                     = Lens.Family2.view (Data.ProtoLens.Field.field @"lastInsertId") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 16)
                         ((Prelude..)
                            Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral _v))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData ExecResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ExecResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ExecResponse'rowsAffected x__)
                (Control.DeepSeq.deepseq (_ExecResponse'lastInsertId x__) ()))
newtype IsolationLevel'UnrecognizedValue
  = IsolationLevel'UnrecognizedValue Data.Int.Int32
  deriving stock (Prelude.Eq, Prelude.Ord, Prelude.Show)
data IsolationLevel
  = ISOLATION_LEVEL_DEFAULT |
    ISOLATION_LEVEL_READ_UNCOMMITTED |
    ISOLATION_LEVEL_READ_COMMITTED |
    ISOLATION_LEVEL_WRITE_COMMITTED |
    ISOLATION_LEVEL_REPEATABLE_READ |
    ISOLATION_LEVEL_SNAPSHOT |
    ISOLATION_LEVEL_SERIALIZABLE |
    ISOLATION_LEVEL_LINEARIZABLE |
    IsolationLevel'Unrecognized !IsolationLevel'UnrecognizedValue
  deriving stock (Prelude.Show, Prelude.Eq, Prelude.Ord)
instance Data.ProtoLens.MessageEnum IsolationLevel where
  maybeToEnum 0 = Prelude.Just ISOLATION_LEVEL_DEFAULT
  maybeToEnum 1 = Prelude.Just ISOLATION_LEVEL_READ_UNCOMMITTED
  maybeToEnum 2 = Prelude.Just ISOLATION_LEVEL_READ_COMMITTED
  maybeToEnum 3 = Prelude.Just ISOLATION_LEVEL_WRITE_COMMITTED
  maybeToEnum 4 = Prelude.Just ISOLATION_LEVEL_REPEATABLE_READ
  maybeToEnum 5 = Prelude.Just ISOLATION_LEVEL_SNAPSHOT
  maybeToEnum 6 = Prelude.Just ISOLATION_LEVEL_SERIALIZABLE
  maybeToEnum 7 = Prelude.Just ISOLATION_LEVEL_LINEARIZABLE
  maybeToEnum k
    = Prelude.Just
        (IsolationLevel'Unrecognized
           (IsolationLevel'UnrecognizedValue (Prelude.fromIntegral k)))
  showEnum ISOLATION_LEVEL_DEFAULT = "ISOLATION_LEVEL_DEFAULT"
  showEnum ISOLATION_LEVEL_READ_UNCOMMITTED
    = "ISOLATION_LEVEL_READ_UNCOMMITTED"
  showEnum ISOLATION_LEVEL_READ_COMMITTED
    = "ISOLATION_LEVEL_READ_COMMITTED"
  showEnum ISOLATION_LEVEL_WRITE_COMMITTED
    = "ISOLATION_LEVEL_WRITE_COMMITTED"
  showEnum ISOLATION_LEVEL_REPEATABLE_READ
    = "ISOLATION_LEVEL_REPEATABLE_READ"
  showEnum ISOLATION_LEVEL_SNAPSHOT = "ISOLATION_LEVEL_SNAPSHOT"
  showEnum ISOLATION_LEVEL_SERIALIZABLE
    = "ISOLATION_LEVEL_SERIALIZABLE"
  showEnum ISOLATION_LEVEL_LINEARIZABLE
    = "ISOLATION_LEVEL_LINEARIZABLE"
  showEnum
    (IsolationLevel'Unrecognized (IsolationLevel'UnrecognizedValue k))
    = Prelude.show k
  readEnum k
    | (Prelude.==) k "ISOLATION_LEVEL_DEFAULT"
    = Prelude.Just ISOLATION_LEVEL_DEFAULT
    | (Prelude.==) k "ISOLATION_LEVEL_READ_UNCOMMITTED"
    = Prelude.Just ISOLATION_LEVEL_READ_UNCOMMITTED
    | (Prelude.==) k "ISOLATION_LEVEL_READ_COMMITTED"
    = Prelude.Just ISOLATION_LEVEL_READ_COMMITTED
    | (Prelude.==) k "ISOLATION_LEVEL_WRITE_COMMITTED"
    = Prelude.Just ISOLATION_LEVEL_WRITE_COMMITTED
    | (Prelude.==) k "ISOLATION_LEVEL_REPEATABLE_READ"
    = Prelude.Just ISOLATION_LEVEL_REPEATABLE_READ
    | (Prelude.==) k "ISOLATION_LEVEL_SNAPSHOT"
    = Prelude.Just ISOLATION_LEVEL_SNAPSHOT
    | (Prelude.==) k "ISOLATION_LEVEL_SERIALIZABLE"
    = Prelude.Just ISOLATION_LEVEL_SERIALIZABLE
    | (Prelude.==) k "ISOLATION_LEVEL_LINEARIZABLE"
    = Prelude.Just ISOLATION_LEVEL_LINEARIZABLE
    | Prelude.otherwise
    = (Prelude.>>=) (Text.Read.readMaybe k) Data.ProtoLens.maybeToEnum
instance Prelude.Bounded IsolationLevel where
  minBound = ISOLATION_LEVEL_DEFAULT
  maxBound = ISOLATION_LEVEL_LINEARIZABLE
instance Prelude.Enum IsolationLevel where
  toEnum k__
    = Prelude.maybe
        (Prelude.error
           ((Prelude.++)
              "toEnum: unknown value for enum IsolationLevel: "
              (Prelude.show k__)))
        Prelude.id (Data.ProtoLens.maybeToEnum k__)
  fromEnum ISOLATION_LEVEL_DEFAULT = 0
  fromEnum ISOLATION_LEVEL_READ_UNCOMMITTED = 1
  fromEnum ISOLATION_LEVEL_READ_COMMITTED = 2
  fromEnum ISOLATION_LEVEL_WRITE_COMMITTED = 3
  fromEnum ISOLATION_LEVEL_REPEATABLE_READ = 4
  fromEnum ISOLATION_LEVEL_SNAPSHOT = 5
  fromEnum ISOLATION_LEVEL_SERIALIZABLE = 6
  fromEnum ISOLATION_LEVEL_LINEARIZABLE = 7
  fromEnum
    (IsolationLevel'Unrecognized (IsolationLevel'UnrecognizedValue k))
    = Prelude.fromIntegral k
  succ ISOLATION_LEVEL_LINEARIZABLE
    = Prelude.error
        "IsolationLevel.succ: bad argument ISOLATION_LEVEL_LINEARIZABLE. This value would be out of bounds."
  succ ISOLATION_LEVEL_DEFAULT = ISOLATION_LEVEL_READ_UNCOMMITTED
  succ ISOLATION_LEVEL_READ_UNCOMMITTED
    = ISOLATION_LEVEL_READ_COMMITTED
  succ ISOLATION_LEVEL_READ_COMMITTED
    = ISOLATION_LEVEL_WRITE_COMMITTED
  succ ISOLATION_LEVEL_WRITE_COMMITTED
    = ISOLATION_LEVEL_REPEATABLE_READ
  succ ISOLATION_LEVEL_REPEATABLE_READ = ISOLATION_LEVEL_SNAPSHOT
  succ ISOLATION_LEVEL_SNAPSHOT = ISOLATION_LEVEL_SERIALIZABLE
  succ ISOLATION_LEVEL_SERIALIZABLE = ISOLATION_LEVEL_LINEARIZABLE
  succ (IsolationLevel'Unrecognized _)
    = Prelude.error
        "IsolationLevel.succ: bad argument: unrecognized value"
  pred ISOLATION_LEVEL_DEFAULT
    = Prelude.error
        "IsolationLevel.pred: bad argument ISOLATION_LEVEL_DEFAULT. This value would be out of bounds."
  pred ISOLATION_LEVEL_READ_UNCOMMITTED = ISOLATION_LEVEL_DEFAULT
  pred ISOLATION_LEVEL_READ_COMMITTED
    = ISOLATION_LEVEL_READ_UNCOMMITTED
  pred ISOLATION_LEVEL_WRITE_COMMITTED
    = ISOLATION_LEVEL_READ_COMMITTED
  pred ISOLATION_LEVEL_REPEATABLE_READ
    = ISOLATION_LEVEL_WRITE_COMMITTED
  pred ISOLATION_LEVEL_SNAPSHOT = ISOLATION_LEVEL_REPEATABLE_READ
  pred ISOLATION_LEVEL_SERIALIZABLE = ISOLATION_LEVEL_SNAPSHOT
  pred ISOLATION_LEVEL_LINEARIZABLE = ISOLATION_LEVEL_SERIALIZABLE
  pred (IsolationLevel'Unrecognized _)
    = Prelude.error
        "IsolationLevel.pred: bad argument: unrecognized value"
  enumFrom = Data.ProtoLens.Message.Enum.messageEnumFrom
  enumFromTo = Data.ProtoLens.Message.Enum.messageEnumFromTo
  enumFromThen = Data.ProtoLens.Message.Enum.messageEnumFromThen
  enumFromThenTo = Data.ProtoLens.Message.Enum.messageEnumFromThenTo
instance Data.ProtoLens.FieldDefault IsolationLevel where
  fieldDefault = ISOLATION_LEVEL_DEFAULT
instance Control.DeepSeq.NFData IsolationLevel where
  rnf x__ = Prelude.seq x__ ()
newtype LogLevel'UnrecognizedValue
  = LogLevel'UnrecognizedValue Data.Int.Int32
  deriving stock (Prelude.Eq, Prelude.Ord, Prelude.Show)
data LogLevel
  = LOG_LEVEL_UNSPECIFIED |
    LOG_LEVEL_DEBUG |
    LOG_LEVEL_INFO |
    LOG_LEVEL_WARN |
    LOG_LEVEL_ERROR |
    LogLevel'Unrecognized !LogLevel'UnrecognizedValue
  deriving stock (Prelude.Show, Prelude.Eq, Prelude.Ord)
instance Data.ProtoLens.MessageEnum LogLevel where
  maybeToEnum 0 = Prelude.Just LOG_LEVEL_UNSPECIFIED
  maybeToEnum 1 = Prelude.Just LOG_LEVEL_DEBUG
  maybeToEnum 2 = Prelude.Just LOG_LEVEL_INFO
  maybeToEnum 3 = Prelude.Just LOG_LEVEL_WARN
  maybeToEnum 4 = Prelude.Just LOG_LEVEL_ERROR
  maybeToEnum k
    = Prelude.Just
        (LogLevel'Unrecognized
           (LogLevel'UnrecognizedValue (Prelude.fromIntegral k)))
  showEnum LOG_LEVEL_UNSPECIFIED = "LOG_LEVEL_UNSPECIFIED"
  showEnum LOG_LEVEL_DEBUG = "LOG_LEVEL_DEBUG"
  showEnum LOG_LEVEL_INFO = "LOG_LEVEL_INFO"
  showEnum LOG_LEVEL_WARN = "LOG_LEVEL_WARN"
  showEnum LOG_LEVEL_ERROR = "LOG_LEVEL_ERROR"
  showEnum (LogLevel'Unrecognized (LogLevel'UnrecognizedValue k))
    = Prelude.show k
  readEnum k
    | (Prelude.==) k "LOG_LEVEL_UNSPECIFIED"
    = Prelude.Just LOG_LEVEL_UNSPECIFIED
    | (Prelude.==) k "LOG_LEVEL_DEBUG" = Prelude.Just LOG_LEVEL_DEBUG
    | (Prelude.==) k "LOG_LEVEL_INFO" = Prelude.Just LOG_LEVEL_INFO
    | (Prelude.==) k "LOG_LEVEL_WARN" = Prelude.Just LOG_LEVEL_WARN
    | (Prelude.==) k "LOG_LEVEL_ERROR" = Prelude.Just LOG_LEVEL_ERROR
    | Prelude.otherwise
    = (Prelude.>>=) (Text.Read.readMaybe k) Data.ProtoLens.maybeToEnum
instance Prelude.Bounded LogLevel where
  minBound = LOG_LEVEL_UNSPECIFIED
  maxBound = LOG_LEVEL_ERROR
instance Prelude.Enum LogLevel where
  toEnum k__
    = Prelude.maybe
        (Prelude.error
           ((Prelude.++)
              "toEnum: unknown value for enum LogLevel: " (Prelude.show k__)))
        Prelude.id (Data.ProtoLens.maybeToEnum k__)
  fromEnum LOG_LEVEL_UNSPECIFIED = 0
  fromEnum LOG_LEVEL_DEBUG = 1
  fromEnum LOG_LEVEL_INFO = 2
  fromEnum LOG_LEVEL_WARN = 3
  fromEnum LOG_LEVEL_ERROR = 4
  fromEnum (LogLevel'Unrecognized (LogLevel'UnrecognizedValue k))
    = Prelude.fromIntegral k
  succ LOG_LEVEL_ERROR
    = Prelude.error
        "LogLevel.succ: bad argument LOG_LEVEL_ERROR. This value would be out of bounds."
  succ LOG_LEVEL_UNSPECIFIED = LOG_LEVEL_DEBUG
  succ LOG_LEVEL_DEBUG = LOG_LEVEL_INFO
  succ LOG_LEVEL_INFO = LOG_LEVEL_WARN
  succ LOG_LEVEL_WARN = LOG_LEVEL_ERROR
  succ (LogLevel'Unrecognized _)
    = Prelude.error "LogLevel.succ: bad argument: unrecognized value"
  pred LOG_LEVEL_UNSPECIFIED
    = Prelude.error
        "LogLevel.pred: bad argument LOG_LEVEL_UNSPECIFIED. This value would be out of bounds."
  pred LOG_LEVEL_DEBUG = LOG_LEVEL_UNSPECIFIED
  pred LOG_LEVEL_INFO = LOG_LEVEL_DEBUG
  pred LOG_LEVEL_WARN = LOG_LEVEL_INFO
  pred LOG_LEVEL_ERROR = LOG_LEVEL_WARN
  pred (LogLevel'Unrecognized _)
    = Prelude.error "LogLevel.pred: bad argument: unrecognized value"
  enumFrom = Data.ProtoLens.Message.Enum.messageEnumFrom
  enumFromTo = Data.ProtoLens.Message.Enum.messageEnumFromTo
  enumFromThen = Data.ProtoLens.Message.Enum.messageEnumFromThen
  enumFromThenTo = Data.ProtoLens.Message.Enum.messageEnumFromThenTo
instance Data.ProtoLens.FieldDefault LogLevel where
  fieldDefault = LOG_LEVEL_UNSPECIFIED
instance Control.DeepSeq.NFData LogLevel where
  rnf x__ = Prelude.seq x__ ()
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' LogRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' LogRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.level' @:: Lens' LogRequest LogLevel@
         * 'Proto.Plugin.V1.Host_Fields.message' @:: Lens' LogRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.fields' @:: Lens' LogRequest (Data.Map.Map Data.Text.Text Data.Text.Text)@ -}
data LogRequest
  = LogRequest'_constructor {_LogRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                             _LogRequest'level :: !LogLevel,
                             _LogRequest'message :: !Data.Text.Text,
                             _LogRequest'fields :: !(Data.Map.Map Data.Text.Text Data.Text.Text),
                             _LogRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show LogRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField LogRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'context (\ x__ y__ -> x__ {_LogRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField LogRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'context (\ x__ y__ -> x__ {_LogRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField LogRequest "level" LogLevel where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'level (\ x__ y__ -> x__ {_LogRequest'level = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField LogRequest "message" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'message (\ x__ y__ -> x__ {_LogRequest'message = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField LogRequest "fields" (Data.Map.Map Data.Text.Text Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'fields (\ x__ y__ -> x__ {_LogRequest'fields = y__}))
        Prelude.id
instance Data.ProtoLens.Message LogRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.LogRequest"
  packedMessageDescriptor _
    = "\n\
      \\n\
      \LogRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC22\n\
      \\ENQlevel\CAN\STX \SOH(\SO2\FS.coremesh.plugin.v1.LogLevelR\ENQlevel\DC2\CAN\n\
      \\amessage\CAN\ETX \SOH(\tR\amessage\DC2B\n\
      \\ACKfields\CAN\EOT \ETX(\v2*.coremesh.plugin.v1.LogRequest.FieldsEntryR\ACKfields\SUB9\n\
      \\vFieldsEntry\DC2\DLE\n\
      \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
      \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor LogRequest
        level__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "level"
              (Data.ProtoLens.ScalarField Data.ProtoLens.EnumField ::
                 Data.ProtoLens.FieldTypeDescriptor LogLevel)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"level")) ::
              Data.ProtoLens.FieldDescriptor LogRequest
        message__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "message"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"message")) ::
              Data.ProtoLens.FieldDescriptor LogRequest
        fields__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "fields"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor LogRequest'FieldsEntry)
              (Data.ProtoLens.MapField
                 (Data.ProtoLens.Field.field @"key")
                 (Data.ProtoLens.Field.field @"value")
                 (Data.ProtoLens.Field.field @"fields")) ::
              Data.ProtoLens.FieldDescriptor LogRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, level__field_descriptor),
           (Data.ProtoLens.Tag 3, message__field_descriptor),
           (Data.ProtoLens.Tag 4, fields__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _LogRequest'_unknownFields
        (\ x__ y__ -> x__ {_LogRequest'_unknownFields = y__})
  defMessage
    = LogRequest'_constructor
        {_LogRequest'context = Prelude.Nothing,
         _LogRequest'level = Data.ProtoLens.fieldDefault,
         _LogRequest'message = Data.ProtoLens.fieldDefault,
         _LogRequest'fields = Data.Map.empty,
         _LogRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          LogRequest -> Data.ProtoLens.Encoding.Bytes.Parser LogRequest
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
                        16
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          Prelude.toEnum
                                          (Prelude.fmap
                                             Prelude.fromIntegral
                                             Data.ProtoLens.Encoding.Bytes.getVarInt))
                                       "level"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"level") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "message"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"message") y x)
                        34
                          -> do !(entry :: LogRequest'FieldsEntry) <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                                                            Data.ProtoLens.Encoding.Bytes.isolate
                                                                              (Prelude.fromIntegral
                                                                                 len)
                                                                              Data.ProtoLens.parseMessage)
                                                                        "fields"
                                (let
                                   key = Lens.Family2.view (Data.ProtoLens.Field.field @"key") entry
                                   value
                                     = Lens.Family2.view (Data.ProtoLens.Field.field @"value") entry
                                 in
                                   loop
                                     (Lens.Family2.over
                                        (Data.ProtoLens.Field.field @"fields")
                                        (\ !t -> Data.Map.insert key value t) x))
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "LogRequest"
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
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"level") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 16)
                         ((Prelude..)
                            ((Prelude..)
                               Data.ProtoLens.Encoding.Bytes.putVarInt Prelude.fromIntegral)
                            Prelude.fromEnum _v))
                ((Data.Monoid.<>)
                   (let
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"message") _x
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
                                                LogRequest'FieldsEntry)))))
                            (Data.Map.toList
                               (Lens.Family2.view (Data.ProtoLens.Field.field @"fields") _x))))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData LogRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_LogRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_LogRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_LogRequest'level x__)
                   (Control.DeepSeq.deepseq
                      (_LogRequest'message x__)
                      (Control.DeepSeq.deepseq (_LogRequest'fields x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.key' @:: Lens' LogRequest'FieldsEntry Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.value' @:: Lens' LogRequest'FieldsEntry Data.Text.Text@ -}
data LogRequest'FieldsEntry
  = LogRequest'FieldsEntry'_constructor {_LogRequest'FieldsEntry'key :: !Data.Text.Text,
                                         _LogRequest'FieldsEntry'value :: !Data.Text.Text,
                                         _LogRequest'FieldsEntry'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show LogRequest'FieldsEntry where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField LogRequest'FieldsEntry "key" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'FieldsEntry'key
           (\ x__ y__ -> x__ {_LogRequest'FieldsEntry'key = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField LogRequest'FieldsEntry "value" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _LogRequest'FieldsEntry'value
           (\ x__ y__ -> x__ {_LogRequest'FieldsEntry'value = y__}))
        Prelude.id
instance Data.ProtoLens.Message LogRequest'FieldsEntry where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.LogRequest.FieldsEntry"
  packedMessageDescriptor _
    = "\n\
      \\vFieldsEntry\DC2\DLE\n\
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
              Data.ProtoLens.FieldDescriptor LogRequest'FieldsEntry
        value__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "value"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"value")) ::
              Data.ProtoLens.FieldDescriptor LogRequest'FieldsEntry
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, key__field_descriptor),
           (Data.ProtoLens.Tag 2, value__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _LogRequest'FieldsEntry'_unknownFields
        (\ x__ y__ -> x__ {_LogRequest'FieldsEntry'_unknownFields = y__})
  defMessage
    = LogRequest'FieldsEntry'_constructor
        {_LogRequest'FieldsEntry'key = Data.ProtoLens.fieldDefault,
         _LogRequest'FieldsEntry'value = Data.ProtoLens.fieldDefault,
         _LogRequest'FieldsEntry'_unknownFields = []}
  parseMessage
    = let
        loop ::
          LogRequest'FieldsEntry
          -> Data.ProtoLens.Encoding.Bytes.Parser LogRequest'FieldsEntry
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
          (do loop Data.ProtoLens.defMessage) "FieldsEntry"
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
instance Control.DeepSeq.NFData LogRequest'FieldsEntry where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_LogRequest'FieldsEntry'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_LogRequest'FieldsEntry'key x__)
                (Control.DeepSeq.deepseq (_LogRequest'FieldsEntry'value x__) ()))
{- | Fields :
      -}
data LogResponse
  = LogResponse'_constructor {_LogResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show LogResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Message LogResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.LogResponse"
  packedMessageDescriptor _
    = "\n\
      \\vLogResponse"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag = let in Data.Map.fromList []
  unknownFields
    = Lens.Family2.Unchecked.lens
        _LogResponse'_unknownFields
        (\ x__ y__ -> x__ {_LogResponse'_unknownFields = y__})
  defMessage
    = LogResponse'_constructor {_LogResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          LogResponse -> Data.ProtoLens.Encoding.Bytes.Parser LogResponse
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
          (do loop Data.ProtoLens.defMessage) "LogResponse"
  buildMessage
    = \ _x
        -> Data.ProtoLens.Encoding.Wire.buildFieldSet
             (Lens.Family2.view Data.ProtoLens.unknownFields _x)
instance Control.DeepSeq.NFData LogResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq (_LogResponse'_unknownFields x__) ()
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' QueryRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' QueryRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.database' @:: Lens' QueryRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.sql' @:: Lens' QueryRequest Data.Text.Text@
         * 'Proto.Plugin.V1.Host_Fields.args' @:: Lens' QueryRequest [Proto.Google.Protobuf.Struct.Value]@
         * 'Proto.Plugin.V1.Host_Fields.vec'args' @:: Lens' QueryRequest (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value)@ -}
data QueryRequest
  = QueryRequest'_constructor {_QueryRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                               _QueryRequest'database :: !Data.Text.Text,
                               _QueryRequest'sql :: !Data.Text.Text,
                               _QueryRequest'args :: !(Data.Vector.Vector Proto.Google.Protobuf.Struct.Value),
                               _QueryRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show QueryRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField QueryRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'context
           (\ x__ y__ -> x__ {_QueryRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField QueryRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'context
           (\ x__ y__ -> x__ {_QueryRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField QueryRequest "database" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'database
           (\ x__ y__ -> x__ {_QueryRequest'database = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField QueryRequest "sql" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'sql (\ x__ y__ -> x__ {_QueryRequest'sql = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField QueryRequest "args" [Proto.Google.Protobuf.Struct.Value] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'args (\ x__ y__ -> x__ {_QueryRequest'args = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField QueryRequest "vec'args" (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryRequest'args (\ x__ y__ -> x__ {_QueryRequest'args = y__}))
        Prelude.id
instance Data.ProtoLens.Message QueryRequest where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.QueryRequest"
  packedMessageDescriptor _
    = "\n\
      \\fQueryRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
      \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2\DLE\n\
      \\ETXsql\CAN\ETX \SOH(\tR\ETXsql\DC2*\n\
      \\EOTargs\CAN\EOT \ETX(\v2\SYN.google.protobuf.ValueR\EOTargs"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor QueryRequest
        database__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "database"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"database")) ::
              Data.ProtoLens.FieldDescriptor QueryRequest
        sql__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "sql"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"sql")) ::
              Data.ProtoLens.FieldDescriptor QueryRequest
        args__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "args"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Google.Protobuf.Struct.Value)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"args")) ::
              Data.ProtoLens.FieldDescriptor QueryRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, database__field_descriptor),
           (Data.ProtoLens.Tag 3, sql__field_descriptor),
           (Data.ProtoLens.Tag 4, args__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _QueryRequest'_unknownFields
        (\ x__ y__ -> x__ {_QueryRequest'_unknownFields = y__})
  defMessage
    = QueryRequest'_constructor
        {_QueryRequest'context = Prelude.Nothing,
         _QueryRequest'database = Data.ProtoLens.fieldDefault,
         _QueryRequest'sql = Data.ProtoLens.fieldDefault,
         _QueryRequest'args = Data.Vector.Generic.empty,
         _QueryRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          QueryRequest
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Proto.Google.Protobuf.Struct.Value
             -> Data.ProtoLens.Encoding.Bytes.Parser QueryRequest
        loop x mutable'args
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'args <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.unsafeFreeze mutable'args)
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
                              (Data.ProtoLens.Field.field @"vec'args") frozen'args x))
               else
                   do tag <- Data.ProtoLens.Encoding.Bytes.getVarInt
                      case tag of
                        10
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "context"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"context") y x)
                                  mutable'args
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "database"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"database") y x)
                                  mutable'args
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "sql"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"sql") y x)
                                  mutable'args
                        34
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "args"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'args y)
                                loop x v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'args
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'args <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'args)
          "QueryRequest"
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
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"database") _x
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
                   (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"sql") _x
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
                         (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'args") _x))
                      (Data.ProtoLens.Encoding.Wire.buildFieldSet
                         (Lens.Family2.view Data.ProtoLens.unknownFields _x)))))
instance Control.DeepSeq.NFData QueryRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_QueryRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_QueryRequest'context x__)
                (Control.DeepSeq.deepseq
                   (_QueryRequest'database x__)
                   (Control.DeepSeq.deepseq
                      (_QueryRequest'sql x__)
                      (Control.DeepSeq.deepseq (_QueryRequest'args x__) ()))))
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.columns' @:: Lens' QueryResponse [Data.Text.Text]@
         * 'Proto.Plugin.V1.Host_Fields.vec'columns' @:: Lens' QueryResponse (Data.Vector.Vector Data.Text.Text)@
         * 'Proto.Plugin.V1.Host_Fields.rows' @:: Lens' QueryResponse [Row]@
         * 'Proto.Plugin.V1.Host_Fields.vec'rows' @:: Lens' QueryResponse (Data.Vector.Vector Row)@ -}
data QueryResponse
  = QueryResponse'_constructor {_QueryResponse'columns :: !(Data.Vector.Vector Data.Text.Text),
                                _QueryResponse'rows :: !(Data.Vector.Vector Row),
                                _QueryResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show QueryResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField QueryResponse "columns" [Data.Text.Text] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryResponse'columns
           (\ x__ y__ -> x__ {_QueryResponse'columns = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField QueryResponse "vec'columns" (Data.Vector.Vector Data.Text.Text) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryResponse'columns
           (\ x__ y__ -> x__ {_QueryResponse'columns = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField QueryResponse "rows" [Row] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryResponse'rows (\ x__ y__ -> x__ {_QueryResponse'rows = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField QueryResponse "vec'rows" (Data.Vector.Vector Row) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _QueryResponse'rows (\ x__ y__ -> x__ {_QueryResponse'rows = y__}))
        Prelude.id
instance Data.ProtoLens.Message QueryResponse where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.QueryResponse"
  packedMessageDescriptor _
    = "\n\
      \\rQueryResponse\DC2\CAN\n\
      \\acolumns\CAN\SOH \ETX(\tR\acolumns\DC2+\n\
      \\EOTrows\CAN\STX \ETX(\v2\ETB.coremesh.plugin.v1.RowR\EOTrows"
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
              Data.ProtoLens.FieldDescriptor QueryResponse
        rows__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "rows"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Row)
              (Data.ProtoLens.RepeatedField
                 Data.ProtoLens.Unpacked (Data.ProtoLens.Field.field @"rows")) ::
              Data.ProtoLens.FieldDescriptor QueryResponse
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, columns__field_descriptor),
           (Data.ProtoLens.Tag 2, rows__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _QueryResponse'_unknownFields
        (\ x__ y__ -> x__ {_QueryResponse'_unknownFields = y__})
  defMessage
    = QueryResponse'_constructor
        {_QueryResponse'columns = Data.Vector.Generic.empty,
         _QueryResponse'rows = Data.Vector.Generic.empty,
         _QueryResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          QueryResponse
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Data.Text.Text
             -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Row
                -> Data.ProtoLens.Encoding.Bytes.Parser QueryResponse
        loop x mutable'columns mutable'rows
          = do end <- Data.ProtoLens.Encoding.Bytes.atEnd
               if end then
                   do frozen'columns <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                          (Data.ProtoLens.Encoding.Growing.unsafeFreeze
                                             mutable'columns)
                      frozen'rows <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
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
                              (Data.ProtoLens.Field.field @"vec'columns") frozen'columns
                              (Lens.Family2.set
                                 (Data.ProtoLens.Field.field @"vec'rows") frozen'rows x)))
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
                                loop x v mutable'rows
                        18
                          -> do !y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                        (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                            Data.ProtoLens.Encoding.Bytes.isolate
                                              (Prelude.fromIntegral len)
                                              Data.ProtoLens.parseMessage)
                                        "rows"
                                v <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                       (Data.ProtoLens.Encoding.Growing.append mutable'rows y)
                                loop x mutable'columns v
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
                                  mutable'columns mutable'rows
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do mutable'columns <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                   Data.ProtoLens.Encoding.Growing.new
              mutable'rows <- Data.ProtoLens.Encoding.Parser.Unsafe.unsafeLiftIO
                                Data.ProtoLens.Encoding.Growing.new
              loop Data.ProtoLens.defMessage mutable'columns mutable'rows)
          "QueryResponse"
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
                              Data.ProtoLens.encodeMessage _v))
                   (Lens.Family2.view (Data.ProtoLens.Field.field @"vec'rows") _x))
                (Data.ProtoLens.Encoding.Wire.buildFieldSet
                   (Lens.Family2.view Data.ProtoLens.unknownFields _x)))
instance Control.DeepSeq.NFData QueryResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_QueryResponse'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_QueryResponse'columns x__)
                (Control.DeepSeq.deepseq (_QueryResponse'rows x__) ()))
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.context' @:: Lens' RollbackTxRequest Proto.Plugin.V1.Plugin.Context@
         * 'Proto.Plugin.V1.Host_Fields.maybe'context' @:: Lens' RollbackTxRequest (Prelude.Maybe Proto.Plugin.V1.Plugin.Context)@
         * 'Proto.Plugin.V1.Host_Fields.txId' @:: Lens' RollbackTxRequest Data.Text.Text@ -}
data RollbackTxRequest
  = RollbackTxRequest'_constructor {_RollbackTxRequest'context :: !(Prelude.Maybe Proto.Plugin.V1.Plugin.Context),
                                    _RollbackTxRequest'txId :: !Data.Text.Text,
                                    _RollbackTxRequest'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show RollbackTxRequest where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField RollbackTxRequest "context" Proto.Plugin.V1.Plugin.Context where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _RollbackTxRequest'context
           (\ x__ y__ -> x__ {_RollbackTxRequest'context = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField RollbackTxRequest "maybe'context" (Prelude.Maybe Proto.Plugin.V1.Plugin.Context) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _RollbackTxRequest'context
           (\ x__ y__ -> x__ {_RollbackTxRequest'context = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField RollbackTxRequest "txId" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _RollbackTxRequest'txId
           (\ x__ y__ -> x__ {_RollbackTxRequest'txId = y__}))
        Prelude.id
instance Data.ProtoLens.Message RollbackTxRequest where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.RollbackTxRequest"
  packedMessageDescriptor _
    = "\n\
      \\DC1RollbackTxRequest\DC25\n\
      \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\DC3\n\
      \\ENQtx_id\CAN\STX \SOH(\tR\EOTtxId"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        context__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "context"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor Proto.Plugin.V1.Plugin.Context)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'context")) ::
              Data.ProtoLens.FieldDescriptor RollbackTxRequest
        txId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "tx_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"txId")) ::
              Data.ProtoLens.FieldDescriptor RollbackTxRequest
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, context__field_descriptor),
           (Data.ProtoLens.Tag 2, txId__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _RollbackTxRequest'_unknownFields
        (\ x__ y__ -> x__ {_RollbackTxRequest'_unknownFields = y__})
  defMessage
    = RollbackTxRequest'_constructor
        {_RollbackTxRequest'context = Prelude.Nothing,
         _RollbackTxRequest'txId = Data.ProtoLens.fieldDefault,
         _RollbackTxRequest'_unknownFields = []}
  parseMessage
    = let
        loop ::
          RollbackTxRequest
          -> Data.ProtoLens.Encoding.Bytes.Parser RollbackTxRequest
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
                                       "tx_id"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"txId") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "RollbackTxRequest"
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
                (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"txId") _x
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
instance Control.DeepSeq.NFData RollbackTxRequest where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_RollbackTxRequest'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_RollbackTxRequest'context x__)
                (Control.DeepSeq.deepseq (_RollbackTxRequest'txId x__) ()))
{- | Fields :
      -}
data RollbackTxResponse
  = RollbackTxResponse'_constructor {_RollbackTxResponse'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show RollbackTxResponse where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Message RollbackTxResponse where
  messageName _
    = Data.Text.pack "coremesh.plugin.v1.RollbackTxResponse"
  packedMessageDescriptor _
    = "\n\
      \\DC2RollbackTxResponse"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag = let in Data.Map.fromList []
  unknownFields
    = Lens.Family2.Unchecked.lens
        _RollbackTxResponse'_unknownFields
        (\ x__ y__ -> x__ {_RollbackTxResponse'_unknownFields = y__})
  defMessage
    = RollbackTxResponse'_constructor
        {_RollbackTxResponse'_unknownFields = []}
  parseMessage
    = let
        loop ::
          RollbackTxResponse
          -> Data.ProtoLens.Encoding.Bytes.Parser RollbackTxResponse
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
          (do loop Data.ProtoLens.defMessage) "RollbackTxResponse"
  buildMessage
    = \ _x
        -> Data.ProtoLens.Encoding.Wire.buildFieldSet
             (Lens.Family2.view Data.ProtoLens.unknownFields _x)
instance Control.DeepSeq.NFData RollbackTxResponse where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_RollbackTxResponse'_unknownFields x__) ()
{- | Fields :
     
         * 'Proto.Plugin.V1.Host_Fields.values' @:: Lens' Row [Proto.Google.Protobuf.Struct.Value]@
         * 'Proto.Plugin.V1.Host_Fields.vec'values' @:: Lens' Row (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value)@ -}
data Row
  = Row'_constructor {_Row'values :: !(Data.Vector.Vector Proto.Google.Protobuf.Struct.Value),
                      _Row'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show Row where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField Row "values" [Proto.Google.Protobuf.Struct.Value] where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Row'values (\ x__ y__ -> x__ {_Row'values = y__}))
        (Lens.Family2.Unchecked.lens
           Data.Vector.Generic.toList
           (\ _ y__ -> Data.Vector.Generic.fromList y__))
instance Data.ProtoLens.Field.HasField Row "vec'values" (Data.Vector.Vector Proto.Google.Protobuf.Struct.Value) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _Row'values (\ x__ y__ -> x__ {_Row'values = y__}))
        Prelude.id
instance Data.ProtoLens.Message Row where
  messageName _ = Data.Text.pack "coremesh.plugin.v1.Row"
  packedMessageDescriptor _
    = "\n\
      \\ETXRow\DC2.\n\
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
              Data.ProtoLens.FieldDescriptor Row
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, values__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _Row'_unknownFields (\ x__ y__ -> x__ {_Row'_unknownFields = y__})
  defMessage
    = Row'_constructor
        {_Row'values = Data.Vector.Generic.empty, _Row'_unknownFields = []}
  parseMessage
    = let
        loop ::
          Row
          -> Data.ProtoLens.Encoding.Growing.Growing Data.Vector.Vector Data.ProtoLens.Encoding.Growing.RealWorld Proto.Google.Protobuf.Struct.Value
             -> Data.ProtoLens.Encoding.Bytes.Parser Row
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
          "Row"
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
instance Control.DeepSeq.NFData Row where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_Row'_unknownFields x__)
             (Control.DeepSeq.deepseq (_Row'values x__) ())
data HostService = HostService {}
instance Data.ProtoLens.Service.Types.Service HostService where
  type ServiceName HostService = "HostService"
  type ServicePackage HostService = "coremesh.plugin.v1"
  type ServiceMethods HostService = '["beginTx",
                                      "commitTx",
                                      "dispatch",
                                      "dispatchRead",
                                      "exec",
                                      "log",
                                      "query",
                                      "rollbackTx"]
  packedServiceDescriptor _
    = "\n\
      \\vHostService\DC2Q\n\
      \\bDispatch\DC2!.coremesh.plugin.v1.HandleRequest\SUB\".coremesh.plugin.v1.HandleResponse\DC2U\n\
      \\fDispatchRead\DC2!.coremesh.plugin.v1.HandleRequest\SUB .coremesh.plugin.v1.ReadResponse0\SOH\DC2F\n\
      \\ETXLog\DC2\RS.coremesh.plugin.v1.LogRequest\SUB\US.coremesh.plugin.v1.LogResponse\DC2L\n\
      \\ENQQuery\DC2 .coremesh.plugin.v1.QueryRequest\SUB!.coremesh.plugin.v1.QueryResponse\DC2I\n\
      \\EOTExec\DC2\US.coremesh.plugin.v1.ExecRequest\SUB .coremesh.plugin.v1.ExecResponse\DC2R\n\
      \\aBeginTx\DC2\".coremesh.plugin.v1.BeginTxRequest\SUB#.coremesh.plugin.v1.BeginTxResponse\DC2U\n\
      \\bCommitTx\DC2#.coremesh.plugin.v1.CommitTxRequest\SUB$.coremesh.plugin.v1.CommitTxResponse\DC2[\n\
      \\n\
      \RollbackTx\DC2%.coremesh.plugin.v1.RollbackTxRequest\SUB&.coremesh.plugin.v1.RollbackTxResponse"
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "dispatch" where
  type MethodName HostService "dispatch" = "Dispatch"
  type MethodInput HostService "dispatch" = Proto.Plugin.V1.Plugin.HandleRequest
  type MethodOutput HostService "dispatch" = Proto.Plugin.V1.Plugin.HandleResponse
  type MethodStreamingType HostService "dispatch" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "dispatchRead" where
  type MethodName HostService "dispatchRead" = "DispatchRead"
  type MethodInput HostService "dispatchRead" = Proto.Plugin.V1.Plugin.HandleRequest
  type MethodOutput HostService "dispatchRead" = Proto.Plugin.V1.Plugin.ReadResponse
  type MethodStreamingType HostService "dispatchRead" = 'Data.ProtoLens.Service.Types.ServerStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "log" where
  type MethodName HostService "log" = "Log"
  type MethodInput HostService "log" = LogRequest
  type MethodOutput HostService "log" = LogResponse
  type MethodStreamingType HostService "log" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "query" where
  type MethodName HostService "query" = "Query"
  type MethodInput HostService "query" = QueryRequest
  type MethodOutput HostService "query" = QueryResponse
  type MethodStreamingType HostService "query" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "exec" where
  type MethodName HostService "exec" = "Exec"
  type MethodInput HostService "exec" = ExecRequest
  type MethodOutput HostService "exec" = ExecResponse
  type MethodStreamingType HostService "exec" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "beginTx" where
  type MethodName HostService "beginTx" = "BeginTx"
  type MethodInput HostService "beginTx" = BeginTxRequest
  type MethodOutput HostService "beginTx" = BeginTxResponse
  type MethodStreamingType HostService "beginTx" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "commitTx" where
  type MethodName HostService "commitTx" = "CommitTx"
  type MethodInput HostService "commitTx" = CommitTxRequest
  type MethodOutput HostService "commitTx" = CommitTxResponse
  type MethodStreamingType HostService "commitTx" = 'Data.ProtoLens.Service.Types.NonStreaming
instance Data.ProtoLens.Service.Types.HasMethodImpl HostService "rollbackTx" where
  type MethodName HostService "rollbackTx" = "RollbackTx"
  type MethodInput HostService "rollbackTx" = RollbackTxRequest
  type MethodOutput HostService "rollbackTx" = RollbackTxResponse
  type MethodStreamingType HostService "rollbackTx" = 'Data.ProtoLens.Service.Types.NonStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\DC4plugin/v1/host.proto\DC2\DC2coremesh.plugin.v1\SUB\FSgoogle/protobuf/struct.proto\SUB\SYNplugin/v1/plugin.proto\"\144\STX\n\
    \\n\
    \LogRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC22\n\
    \\ENQlevel\CAN\STX \SOH(\SO2\FS.coremesh.plugin.v1.LogLevelR\ENQlevel\DC2\CAN\n\
    \\amessage\CAN\ETX \SOH(\tR\amessage\DC2B\n\
    \\ACKfields\CAN\EOT \ETX(\v2*.coremesh.plugin.v1.LogRequest.FieldsEntryR\ACKfields\SUB9\n\
    \\vFieldsEntry\DC2\DLE\n\
    \\ETXkey\CAN\SOH \SOH(\tR\ETXkey\DC2\DC4\n\
    \\ENQvalue\CAN\STX \SOH(\tR\ENQvalue:\STX8\SOH\"\r\n\
    \\vLogResponse\"\159\SOH\n\
    \\fQueryRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
    \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2\DLE\n\
    \\ETXsql\CAN\ETX \SOH(\tR\ETXsql\DC2*\n\
    \\EOTargs\CAN\EOT \ETX(\v2\SYN.google.protobuf.ValueR\EOTargs\"5\n\
    \\ETXRow\DC2.\n\
    \\ACKvalues\CAN\SOH \ETX(\v2\SYN.google.protobuf.ValueR\ACKvalues\"V\n\
    \\rQueryResponse\DC2\CAN\n\
    \\acolumns\CAN\SOH \ETX(\tR\acolumns\DC2+\n\
    \\EOTrows\CAN\STX \ETX(\v2\ETB.coremesh.plugin.v1.RowR\EOTrows\"\158\SOH\n\
    \\vExecRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
    \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2\DLE\n\
    \\ETXsql\CAN\ETX \SOH(\tR\ETXsql\DC2*\n\
    \\EOTargs\CAN\EOT \ETX(\v2\SYN.google.protobuf.ValueR\EOTargs\"Y\n\
    \\fExecResponse\DC2#\n\
    \\rrows_affected\CAN\SOH \SOH(\ETXR\frowsAffected\DC2$\n\
    \\SOlast_insert_id\CAN\STX \SOH(\ETXR\flastInsertId\"\194\SOH\n\
    \\SOBeginTxRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\SUB\n\
    \\bdatabase\CAN\STX \SOH(\tR\bdatabase\DC2@\n\
    \\tisolation\CAN\ETX \SOH(\SO2\".coremesh.plugin.v1.IsolationLevelR\tisolation\DC2\ESC\n\
    \\tread_only\CAN\EOT \SOH(\bR\breadOnly\"&\n\
    \\SIBeginTxResponse\DC2\DC3\n\
    \\ENQtx_id\CAN\SOH \SOH(\tR\EOTtxId\"]\n\
    \\SICommitTxRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\DC3\n\
    \\ENQtx_id\CAN\STX \SOH(\tR\EOTtxId\"\DC2\n\
    \\DLECommitTxResponse\"_\n\
    \\DC1RollbackTxRequest\DC25\n\
    \\acontext\CAN\SOH \SOH(\v2\ESC.coremesh.plugin.v1.ContextR\acontext\DC2\DC3\n\
    \\ENQtx_id\CAN\STX \SOH(\tR\EOTtxId\"\DC4\n\
    \\DC2RollbackTxResponse*w\n\
    \\bLogLevel\DC2\EM\n\
    \\NAKLOG_LEVEL_UNSPECIFIED\DLE\NUL\DC2\DC3\n\
    \\SILOG_LEVEL_DEBUG\DLE\SOH\DC2\DC2\n\
    \\SOLOG_LEVEL_INFO\DLE\STX\DC2\DC2\n\
    \\SOLOG_LEVEL_WARN\DLE\ETX\DC2\DC3\n\
    \\SILOG_LEVEL_ERROR\DLE\EOT*\163\STX\n\
    \\SOIsolationLevel\DC2\ESC\n\
    \\ETBISOLATION_LEVEL_DEFAULT\DLE\NUL\DC2$\n\
    \ ISOLATION_LEVEL_READ_UNCOMMITTED\DLE\SOH\DC2\"\n\
    \\RSISOLATION_LEVEL_READ_COMMITTED\DLE\STX\DC2#\n\
    \\USISOLATION_LEVEL_WRITE_COMMITTED\DLE\ETX\DC2#\n\
    \\USISOLATION_LEVEL_REPEATABLE_READ\DLE\EOT\DC2\FS\n\
    \\CANISOLATION_LEVEL_SNAPSHOT\DLE\ENQ\DC2 \n\
    \\FSISOLATION_LEVEL_SERIALIZABLE\DLE\ACK\DC2 \n\
    \\FSISOLATION_LEVEL_LINEARIZABLE\DLE\a2\160\ENQ\n\
    \\vHostService\DC2Q\n\
    \\bDispatch\DC2!.coremesh.plugin.v1.HandleRequest\SUB\".coremesh.plugin.v1.HandleResponse\DC2U\n\
    \\fDispatchRead\DC2!.coremesh.plugin.v1.HandleRequest\SUB .coremesh.plugin.v1.ReadResponse0\SOH\DC2F\n\
    \\ETXLog\DC2\RS.coremesh.plugin.v1.LogRequest\SUB\US.coremesh.plugin.v1.LogResponse\DC2L\n\
    \\ENQQuery\DC2 .coremesh.plugin.v1.QueryRequest\SUB!.coremesh.plugin.v1.QueryResponse\DC2I\n\
    \\EOTExec\DC2\US.coremesh.plugin.v1.ExecRequest\SUB .coremesh.plugin.v1.ExecResponse\DC2R\n\
    \\aBeginTx\DC2\".coremesh.plugin.v1.BeginTxRequest\SUB#.coremesh.plugin.v1.BeginTxResponse\DC2U\n\
    \\bCommitTx\DC2#.coremesh.plugin.v1.CommitTxRequest\SUB$.coremesh.plugin.v1.CommitTxResponse\DC2[\n\
    \\n\
    \RollbackTx\DC2%.coremesh.plugin.v1.RollbackTxRequest\SUB&.coremesh.plugin.v1.RollbackTxResponseBCZAgithub.com/coremesh-labs/coremesh/internal/api/plugin/v1;pluginv1J\251,\n\
    \\a\DC2\ENQ\NUL\NUL\152\SOH\GS\n\
    \\b\n\
    \\SOH\f\DC2\ETX\NUL\NUL\DC2\n\
    \\b\n\
    \\SOH\STX\DC2\ETX\STX\NUL\ESC\n\
    \\t\n\
    \\STX\ETX\NUL\DC2\ETX\EOT\NUL&\n\
    \\t\n\
    \\STX\ETX\SOH\DC2\ETX\ENQ\NUL \n\
    \\b\n\
    \\SOH\b\DC2\ETX\a\NULX\n\
    \\t\n\
    \\STX\b\v\DC2\ETX\a\NULX\n\
    \\159\ENQ\n\
    \\STX\ACK\NUL\DC2\EOT\DC4\NULB\SOH\SUB\146\ENQ HostService wird vom Host implementiert und vom Plugin \195\188ber den\n\
    \ go-plugin GRPCBroker aufgerufen (R\195\188ckkanal). So rufen Plugins andere\n\
    \ Plugins auf und nutzen Logging und die SQL-Pools des Hosts, ohne selbst\n\
    \ Zugangsdaten zu kennen.\n\
    \\n\
    \ Jeder Request tr\195\164gt den Context des ausl\195\182senden Handle-Aufrufs. Der Host\n\
    \ vertraut dabei nur der request_id: Er ordnet sie dem laufenden Handle-Aufruf\n\
    \ zu und verwendet dessen tenant_id/user_id \226\128\147 vom Plugin gesendete Werte f\195\188r\n\
    \ Mandant und Benutzer werden ignoriert. Zus\195\164tzlich pr\195\188ft der Host jeden\n\
    \ Aufruf gegen die Freigaben des Plugins in der Host-Config (welche\n\
    \ Datenbanken, nur lesend oder auch schreibend).\n\
    \\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\DC4\b\DC3\n\
    \\156\ENQ\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX \STX7\SUB\142\ENQ Dispatch \195\188bergibt eine (object, action)-Anfrage an den Dispatcher des\n\
    \ Hosts, der sie an das zust\195\164ndige Plugin weiterleitet \226\128\147 nach derselben\n\
    \ Match-Logik wie bei Anfragen von au\195\159en (siehe PluginService). Das\n\
    \ aufrufende Plugin kennt das Ziel-Plugin nicht.\n\
    \\n\
    \ Der Host \195\188bernimmt request_id, tenant_id und user_id des ausl\195\182senden\n\
    \ Handle-Aufrufs, sodass Mandant und Berechtigungen \195\188ber die ganze\n\
    \ Aufrufkette erhalten bleiben. Deadlines und Abbruch propagieren \195\188ber gRPC.\n\
    \ Zyklen (A -> B -> A ...) begrenzt der Host \195\188ber eine maximale\n\
    \ Verschachtelungstiefe pro request_id und antwortet dann mit\n\
    \ FailedPrecondition. Kein Treffer: Unimplemented.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX \ACK\SO\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX \SI\FS\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX '5\n\
    \\249\SOH\n\
    \\EOT\ACK\NUL\STX\SOH\DC2\ETX&\STX@\SUB\235\SOH DispatchRead ist das Gegenst\195\188ck zu PluginService.Read: Der Host leitet\n\
    \ die Anfrage wie Dispatch weiter (gleiche Pr\195\188fung von request_id,\n\
    \ Aufruftiefe und Berechtigung) und reicht den Datenstrom des Ziel-Plugins\n\
    \ unver\195\164ndert durch.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\SOH\DC2\ETX&\ACK\DC2\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\STX\DC2\ETX&\DC3 \n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\ACK\DC2\ETX&+1\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\SOH\ETX\DC2\ETX&2>\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\STX\DC2\ETX(\STX,\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\SOH\DC2\ETX(\ACK\t\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\STX\DC2\ETX(\n\
    \\DC4\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\STX\ETX\DC2\ETX(\US*\n\
    \J\n\
    \\EOT\ACK\NUL\STX\ETX\DC2\ETX+\STX2\SUB= Query f\195\188hrt ein SELECT aus und liefert die Zeilen zur\195\188ck.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\SOH\DC2\ETX+\ACK\v\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\STX\DC2\ETX+\f\CAN\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ETX\ETX\DC2\ETX+#0\n\
    \4\n\
    \\EOT\ACK\NUL\STX\EOT\DC2\ETX.\STX/\SUB' Exec f\195\188hrt INSERT/UPDATE/DELETE aus.\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\EOT\SOH\DC2\ETX.\ACK\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\EOT\STX\DC2\ETX.\v\SYN\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\EOT\ETX\DC2\ETX.!-\n\
    \\237\ENQ\n\
    \\EOT\ACK\NUL\STX\ENQ\DC2\ETX?\STX8\SUB\223\ENQ Transaktionen\n\
    \\n\
    \ Eine Transaktion gilt f\195\188r genau eine Datenbank und ist an die request_id\n\
    \ der Aufrufkette gebunden. Query/Exec laufen in ihr, wenn\n\
    \ Context.tx_ids[database] gesetzt ist \226\128\147 auch in Plugins, die \195\188ber Dispatch\n\
    \ aufgerufen wurden. Mehrere Datenbanken werden nicht atomar verbunden.\n\
    \\n\
    \ Commit darf nur der Er\195\182ffner (gleiche request_id, gleiches Plugin).\n\
    \ Rollback darf jeder Teilnehmer ausl\195\182sen; danach ist die Transaktion\n\
    \ abgebrochen und jeder weitere Zugriff bzw. Commit liefert Aborted.\n\
    \\n\
    \ Der Host rollt automatisch zur\195\188ck, wenn die \195\164u\195\159erste Anfrage endet, die\n\
    \ maximale Laufzeit \195\188berschritten ist oder der Plugin-Prozess abst\195\188rzt.\n\
    \ Die Zahl offener Transaktionen pro Plugin ist begrenzt\n\
    \ (ResourceExhausted).\n\
    \\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ENQ\SOH\DC2\ETX?\ACK\r\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ENQ\STX\DC2\ETX?\SO\FS\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ENQ\ETX\DC2\ETX?'6\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\ACK\DC2\ETX@\STX;\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ACK\SOH\DC2\ETX@\ACK\SO\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ACK\STX\DC2\ETX@\SI\RS\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\ACK\ETX\DC2\ETX@)9\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\a\DC2\ETXA\STXA\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\a\SOH\DC2\ETXA\ACK\DLE\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\a\STX\DC2\ETXA\DC1\"\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\a\ETX\DC2\ETXA-?\n\
    \\n\
    \\n\
    \\STX\ENQ\NUL\DC2\EOTD\NULJ\SOH\n\
    \\n\
    \\n\
    \\ETX\ENQ\NUL\SOH\DC2\ETXD\ENQ\r\n\
    \\v\n\
    \\EOT\ENQ\NUL\STX\NUL\DC2\ETXE\STX\FS\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\NUL\SOH\DC2\ETXE\STX\ETB\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\NUL\STX\DC2\ETXE\SUB\ESC\n\
    \\v\n\
    \\EOT\ENQ\NUL\STX\SOH\DC2\ETXF\STX\SYN\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\SOH\SOH\DC2\ETXF\STX\DC1\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\SOH\STX\DC2\ETXF\DC4\NAK\n\
    \\v\n\
    \\EOT\ENQ\NUL\STX\STX\DC2\ETXG\STX\NAK\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\STX\SOH\DC2\ETXG\STX\DLE\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\STX\STX\DC2\ETXG\DC3\DC4\n\
    \\v\n\
    \\EOT\ENQ\NUL\STX\ETX\DC2\ETXH\STX\NAK\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\ETX\SOH\DC2\ETXH\STX\DLE\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\ETX\STX\DC2\ETXH\DC3\DC4\n\
    \\v\n\
    \\EOT\ENQ\NUL\STX\EOT\DC2\ETXI\STX\SYN\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\EOT\SOH\DC2\ETXI\STX\DC1\n\
    \\f\n\
    \\ENQ\ENQ\NUL\STX\EOT\STX\DC2\ETXI\DC4\NAK\n\
    \\n\
    \\n\
    \\STX\EOT\NUL\DC2\EOTL\NULQ\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETXL\b\DC2\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\NUL\DC2\ETXM\STX\SYN\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ACK\DC2\ETXM\STX\t\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\SOH\DC2\ETXM\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ETX\DC2\ETXM\DC4\NAK\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\SOH\DC2\ETXN\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ACK\DC2\ETXN\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\SOH\DC2\ETXN\v\DLE\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ETX\DC2\ETXN\DC3\DC4\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\STX\DC2\ETXO\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ENQ\DC2\ETXO\STX\b\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\SOH\DC2\ETXO\t\DLE\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ETX\DC2\ETXO\DC3\DC4\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\ETX\DC2\ETXP\STX!\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ACK\DC2\ETXP\STX\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\SOH\DC2\ETXP\SYN\FS\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ETX\DC2\ETXP\US \n\
    \\t\n\
    \\STX\EOT\SOH\DC2\ETXS\NUL\SYN\n\
    \\n\
    \\n\
    \\ETX\EOT\SOH\SOH\DC2\ETXS\b\DC3\n\
    \\n\
    \\n\
    \\STX\EOT\STX\DC2\EOTU\NUL\\\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\STX\SOH\DC2\ETXU\b\DC4\n\
    \\v\n\
    \\EOT\EOT\STX\STX\NUL\DC2\ETXV\STX\SYN\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\ACK\DC2\ETXV\STX\t\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\SOH\DC2\ETXV\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\NUL\ETX\DC2\ETXV\DC4\NAK\n\
    \E\n\
    \\EOT\EOT\STX\STX\SOH\DC2\ETXX\STX\SYN\SUB8 Logischer Pool-Name aus der Host-Config, z. B. \"main\".\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\ENQ\DC2\ETXX\STX\b\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\SOH\DC2\ETXX\t\DC1\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\SOH\ETX\DC2\ETXX\DC4\NAK\n\
    \W\n\
    \\EOT\EOT\STX\STX\STX\DC2\ETXZ\STX\DC1\SUBJ SQL mit Platzhaltern; Werte immer \195\188ber args, nie per String-Verkettung.\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\ENQ\DC2\ETXZ\STX\b\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\SOH\DC2\ETXZ\t\f\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\STX\ETX\DC2\ETXZ\SI\DLE\n\
    \\v\n\
    \\EOT\EOT\STX\STX\ETX\DC2\ETX[\STX*\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\EOT\DC2\ETX[\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\ACK\DC2\ETX[\v \n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\SOH\DC2\ETX[!%\n\
    \\f\n\
    \\ENQ\EOT\STX\STX\ETX\ETX\DC2\ETX[()\n\
    \\n\
    \\n\
    \\STX\EOT\ETX\DC2\EOT^\NUL`\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\ETX\SOH\DC2\ETX^\b\v\n\
    \\v\n\
    \\EOT\EOT\ETX\STX\NUL\DC2\ETX_\STX,\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\EOT\DC2\ETX_\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\ACK\DC2\ETX_\v \n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\SOH\DC2\ETX_!'\n\
    \\f\n\
    \\ENQ\EOT\ETX\STX\NUL\ETX\DC2\ETX_*+\n\
    \\n\
    \\n\
    \\STX\EOT\EOT\DC2\EOTb\NULe\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\EOT\SOH\DC2\ETXb\b\NAK\n\
    \\v\n\
    \\EOT\EOT\EOT\STX\NUL\DC2\ETXc\STX\RS\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\EOT\DC2\ETXc\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\ENQ\DC2\ETXc\v\DC1\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\SOH\DC2\ETXc\DC2\EM\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\NUL\ETX\DC2\ETXc\FS\GS\n\
    \\v\n\
    \\EOT\EOT\EOT\STX\SOH\DC2\ETXd\STX\CAN\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\SOH\EOT\DC2\ETXd\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\SOH\ACK\DC2\ETXd\v\SO\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\SOH\SOH\DC2\ETXd\SI\DC3\n\
    \\f\n\
    \\ENQ\EOT\EOT\STX\SOH\ETX\DC2\ETXd\SYN\ETB\n\
    \\n\
    \\n\
    \\STX\EOT\ENQ\DC2\EOTg\NULl\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\ENQ\SOH\DC2\ETXg\b\DC3\n\
    \\v\n\
    \\EOT\EOT\ENQ\STX\NUL\DC2\ETXh\STX\SYN\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\ACK\DC2\ETXh\STX\t\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\SOH\DC2\ETXh\n\
    \\DC1\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\NUL\ETX\DC2\ETXh\DC4\NAK\n\
    \\v\n\
    \\EOT\EOT\ENQ\STX\SOH\DC2\ETXi\STX\SYN\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\ENQ\DC2\ETXi\STX\b\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\SOH\DC2\ETXi\t\DC1\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\SOH\ETX\DC2\ETXi\DC4\NAK\n\
    \\v\n\
    \\EOT\EOT\ENQ\STX\STX\DC2\ETXj\STX\DC1\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\ENQ\DC2\ETXj\STX\b\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\SOH\DC2\ETXj\t\f\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\STX\ETX\DC2\ETXj\SI\DLE\n\
    \\v\n\
    \\EOT\EOT\ENQ\STX\ETX\DC2\ETXk\STX*\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\ETX\EOT\DC2\ETXk\STX\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\ETX\ACK\DC2\ETXk\v \n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\ETX\SOH\DC2\ETXk!%\n\
    \\f\n\
    \\ENQ\EOT\ENQ\STX\ETX\ETX\DC2\ETXk()\n\
    \\n\
    \\n\
    \\STX\EOT\ACK\DC2\EOTn\NULq\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\ACK\SOH\DC2\ETXn\b\DC4\n\
    \\v\n\
    \\EOT\EOT\ACK\STX\NUL\DC2\ETXo\STX\SUB\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\NUL\ENQ\DC2\ETXo\STX\a\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\NUL\SOH\DC2\ETXo\b\NAK\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\NUL\ETX\DC2\ETXo\CAN\EM\n\
    \\v\n\
    \\EOT\EOT\ACK\STX\SOH\DC2\ETXp\STX\ESC\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\SOH\ENQ\DC2\ETXp\STX\a\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\SOH\SOH\DC2\ETXp\b\SYN\n\
    \\f\n\
    \\ENQ\EOT\ACK\STX\SOH\ETX\DC2\ETXp\EM\SUB\n\
    \[\n\
    \\STX\ENQ\SOH\DC2\EOTt\NUL}\SOH\SUBO IsolationLevel entspricht database/sql.IsolationLevel (gleiche Nummerierung).\n\
    \\n\
    \\n\
    \\n\
    \\ETX\ENQ\SOH\SOH\DC2\ETXt\ENQ\DC3\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\NUL\DC2\ETXu\STX\RS\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\NUL\SOH\DC2\ETXu\STX\EM\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\NUL\STX\DC2\ETXu\FS\GS\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\SOH\DC2\ETXv\STX'\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\SOH\SOH\DC2\ETXv\STX\"\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\SOH\STX\DC2\ETXv%&\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\STX\DC2\ETXw\STX%\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\STX\SOH\DC2\ETXw\STX \n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\STX\STX\DC2\ETXw#$\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\ETX\DC2\ETXx\STX&\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ETX\SOH\DC2\ETXx\STX!\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ETX\STX\DC2\ETXx$%\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\EOT\DC2\ETXy\STX&\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\EOT\SOH\DC2\ETXy\STX!\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\EOT\STX\DC2\ETXy$%\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\ENQ\DC2\ETXz\STX\US\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ENQ\SOH\DC2\ETXz\STX\SUB\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ENQ\STX\DC2\ETXz\GS\RS\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\ACK\DC2\ETX{\STX#\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ACK\SOH\DC2\ETX{\STX\RS\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\ACK\STX\DC2\ETX{!\"\n\
    \\v\n\
    \\EOT\ENQ\SOH\STX\a\DC2\ETX|\STX#\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\a\SOH\DC2\ETX|\STX\RS\n\
    \\f\n\
    \\ENQ\ENQ\SOH\STX\a\STX\DC2\ETX|!\"\n\
    \\v\n\
    \\STX\EOT\a\DC2\ENQ\DEL\NUL\133\SOH\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\a\SOH\DC2\ETX\DEL\b\SYN\n\
    \\f\n\
    \\EOT\EOT\a\STX\NUL\DC2\EOT\128\SOH\STX\SYN\n\
    \\r\n\
    \\ENQ\EOT\a\STX\NUL\ACK\DC2\EOT\128\SOH\STX\t\n\
    \\r\n\
    \\ENQ\EOT\a\STX\NUL\SOH\DC2\EOT\128\SOH\n\
    \\DC1\n\
    \\r\n\
    \\ENQ\EOT\a\STX\NUL\ETX\DC2\EOT\128\SOH\DC4\NAK\n\
    \\f\n\
    \\EOT\EOT\a\STX\SOH\DC2\EOT\129\SOH\STX\SYN\n\
    \\r\n\
    \\ENQ\EOT\a\STX\SOH\ENQ\DC2\EOT\129\SOH\STX\b\n\
    \\r\n\
    \\ENQ\EOT\a\STX\SOH\SOH\DC2\EOT\129\SOH\t\DC1\n\
    \\r\n\
    \\ENQ\EOT\a\STX\SOH\ETX\DC2\EOT\129\SOH\DC4\NAK\n\
    \\f\n\
    \\EOT\EOT\a\STX\STX\DC2\EOT\130\SOH\STX\US\n\
    \\r\n\
    \\ENQ\EOT\a\STX\STX\ACK\DC2\EOT\130\SOH\STX\DLE\n\
    \\r\n\
    \\ENQ\EOT\a\STX\STX\SOH\DC2\EOT\130\SOH\DC1\SUB\n\
    \\r\n\
    \\ENQ\EOT\a\STX\STX\ETX\DC2\EOT\130\SOH\GS\RS\n\
    \Z\n\
    \\EOT\EOT\a\STX\ETX\DC2\EOT\132\SOH\STX\NAK\SUBL Nur lesend; auch f\195\188r Plugins ohne Schreibrecht auf die Datenbank erlaubt.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\ENQ\DC2\EOT\132\SOH\STX\ACK\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\SOH\DC2\EOT\132\SOH\a\DLE\n\
    \\r\n\
    \\ENQ\EOT\a\STX\ETX\ETX\DC2\EOT\132\SOH\DC3\DC4\n\
    \\f\n\
    \\STX\EOT\b\DC2\ACK\135\SOH\NUL\138\SOH\SOH\n\
    \\v\n\
    \\ETX\EOT\b\SOH\DC2\EOT\135\SOH\b\ETB\n\
    \R\n\
    \\EOT\EOT\b\STX\NUL\DC2\EOT\137\SOH\STX\DC3\SUBD Zuf\195\164llige, nicht erratbare ID; g\195\188ltig nur f\195\188r diese request_id.\n\
    \\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\ENQ\DC2\EOT\137\SOH\STX\b\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\SOH\DC2\EOT\137\SOH\t\SO\n\
    \\r\n\
    \\ENQ\EOT\b\STX\NUL\ETX\DC2\EOT\137\SOH\DC1\DC2\n\
    \\f\n\
    \\STX\EOT\t\DC2\ACK\140\SOH\NUL\143\SOH\SOH\n\
    \\v\n\
    \\ETX\EOT\t\SOH\DC2\EOT\140\SOH\b\ETB\n\
    \\f\n\
    \\EOT\EOT\t\STX\NUL\DC2\EOT\141\SOH\STX\SYN\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\ACK\DC2\EOT\141\SOH\STX\t\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\SOH\DC2\EOT\141\SOH\n\
    \\DC1\n\
    \\r\n\
    \\ENQ\EOT\t\STX\NUL\ETX\DC2\EOT\141\SOH\DC4\NAK\n\
    \\f\n\
    \\EOT\EOT\t\STX\SOH\DC2\EOT\142\SOH\STX\DC3\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\ENQ\DC2\EOT\142\SOH\STX\b\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\SOH\DC2\EOT\142\SOH\t\SO\n\
    \\r\n\
    \\ENQ\EOT\t\STX\SOH\ETX\DC2\EOT\142\SOH\DC1\DC2\n\
    \\n\
    \\n\
    \\STX\EOT\n\
    \\DC2\EOT\145\SOH\NUL\ESC\n\
    \\v\n\
    \\ETX\EOT\n\
    \\SOH\DC2\EOT\145\SOH\b\CAN\n\
    \\f\n\
    \\STX\EOT\v\DC2\ACK\147\SOH\NUL\150\SOH\SOH\n\
    \\v\n\
    \\ETX\EOT\v\SOH\DC2\EOT\147\SOH\b\EM\n\
    \\f\n\
    \\EOT\EOT\v\STX\NUL\DC2\EOT\148\SOH\STX\SYN\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\ACK\DC2\EOT\148\SOH\STX\t\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\SOH\DC2\EOT\148\SOH\n\
    \\DC1\n\
    \\r\n\
    \\ENQ\EOT\v\STX\NUL\ETX\DC2\EOT\148\SOH\DC4\NAK\n\
    \\f\n\
    \\EOT\EOT\v\STX\SOH\DC2\EOT\149\SOH\STX\DC3\n\
    \\r\n\
    \\ENQ\EOT\v\STX\SOH\ENQ\DC2\EOT\149\SOH\STX\b\n\
    \\r\n\
    \\ENQ\EOT\v\STX\SOH\SOH\DC2\EOT\149\SOH\t\SO\n\
    \\r\n\
    \\ENQ\EOT\v\STX\SOH\ETX\DC2\EOT\149\SOH\DC1\DC2\n\
    \\n\
    \\n\
    \\STX\EOT\f\DC2\EOT\152\SOH\NUL\GS\n\
    \\v\n\
    \\ETX\EOT\f\SOH\DC2\EOT\152\SOH\b\SUBb\ACKproto3"