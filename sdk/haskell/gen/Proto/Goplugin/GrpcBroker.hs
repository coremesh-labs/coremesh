{- This file was auto-generated from goplugin/grpc_broker.proto by the proto-lens-protoc program. -}
{-# LANGUAGE ScopedTypeVariables, DataKinds, TypeFamilies, UndecidableInstances, GeneralizedNewtypeDeriving, MultiParamTypeClasses, FlexibleContexts, FlexibleInstances, PatternSynonyms, MagicHash, NoImplicitPrelude, DataKinds, BangPatterns, TypeApplications, OverloadedStrings, DerivingStrategies#-}
{-# OPTIONS_GHC -Wno-unused-imports#-}
{-# OPTIONS_GHC -Wno-duplicate-exports#-}
{-# OPTIONS_GHC -Wno-dodgy-exports#-}
module Proto.Goplugin.GrpcBroker (
        GRPCBroker(..), ConnInfo(), ConnInfo'Knock()
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
     
         * 'Proto.Goplugin.GrpcBroker_Fields.serviceId' @:: Lens' ConnInfo Data.Word.Word32@
         * 'Proto.Goplugin.GrpcBroker_Fields.network' @:: Lens' ConnInfo Data.Text.Text@
         * 'Proto.Goplugin.GrpcBroker_Fields.address' @:: Lens' ConnInfo Data.Text.Text@
         * 'Proto.Goplugin.GrpcBroker_Fields.knock' @:: Lens' ConnInfo ConnInfo'Knock@
         * 'Proto.Goplugin.GrpcBroker_Fields.maybe'knock' @:: Lens' ConnInfo (Prelude.Maybe ConnInfo'Knock)@ -}
data ConnInfo
  = ConnInfo'_constructor {_ConnInfo'serviceId :: !Data.Word.Word32,
                           _ConnInfo'network :: !Data.Text.Text,
                           _ConnInfo'address :: !Data.Text.Text,
                           _ConnInfo'knock :: !(Prelude.Maybe ConnInfo'Knock),
                           _ConnInfo'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ConnInfo where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ConnInfo "serviceId" Data.Word.Word32 where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'serviceId (\ x__ y__ -> x__ {_ConnInfo'serviceId = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConnInfo "network" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'network (\ x__ y__ -> x__ {_ConnInfo'network = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConnInfo "address" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'address (\ x__ y__ -> x__ {_ConnInfo'address = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConnInfo "knock" ConnInfo'Knock where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'knock (\ x__ y__ -> x__ {_ConnInfo'knock = y__}))
        (Data.ProtoLens.maybeLens Data.ProtoLens.defMessage)
instance Data.ProtoLens.Field.HasField ConnInfo "maybe'knock" (Prelude.Maybe ConnInfo'Knock) where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'knock (\ x__ y__ -> x__ {_ConnInfo'knock = y__}))
        Prelude.id
instance Data.ProtoLens.Message ConnInfo where
  messageName _ = Data.Text.pack "plugin.ConnInfo"
  packedMessageDescriptor _
    = "\n\
      \\bConnInfo\DC2\GS\n\
      \\n\
      \service_id\CAN\SOH \SOH(\rR\tserviceId\DC2\CAN\n\
      \\anetwork\CAN\STX \SOH(\tR\anetwork\DC2\CAN\n\
      \\aaddress\CAN\ETX \SOH(\tR\aaddress\DC2,\n\
      \\ENQknock\CAN\EOT \SOH(\v2\SYN.plugin.ConnInfo.KnockR\ENQknock\SUBE\n\
      \\ENQKnock\DC2\DC4\n\
      \\ENQknock\CAN\SOH \SOH(\bR\ENQknock\DC2\DLE\n\
      \\ETXack\CAN\STX \SOH(\bR\ETXack\DC2\DC4\n\
      \\ENQerror\CAN\ETX \SOH(\tR\ENQerror"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        serviceId__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "service_id"
              (Data.ProtoLens.ScalarField Data.ProtoLens.UInt32Field ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Word.Word32)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional
                 (Data.ProtoLens.Field.field @"serviceId")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo
        network__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "network"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"network")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo
        address__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "address"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"address")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo
        knock__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "knock"
              (Data.ProtoLens.MessageField Data.ProtoLens.MessageType ::
                 Data.ProtoLens.FieldTypeDescriptor ConnInfo'Knock)
              (Data.ProtoLens.OptionalField
                 (Data.ProtoLens.Field.field @"maybe'knock")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, serviceId__field_descriptor),
           (Data.ProtoLens.Tag 2, network__field_descriptor),
           (Data.ProtoLens.Tag 3, address__field_descriptor),
           (Data.ProtoLens.Tag 4, knock__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ConnInfo'_unknownFields
        (\ x__ y__ -> x__ {_ConnInfo'_unknownFields = y__})
  defMessage
    = ConnInfo'_constructor
        {_ConnInfo'serviceId = Data.ProtoLens.fieldDefault,
         _ConnInfo'network = Data.ProtoLens.fieldDefault,
         _ConnInfo'address = Data.ProtoLens.fieldDefault,
         _ConnInfo'knock = Prelude.Nothing, _ConnInfo'_unknownFields = []}
  parseMessage
    = let
        loop :: ConnInfo -> Data.ProtoLens.Encoding.Bytes.Parser ConnInfo
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
                                       "service_id"
                                loop
                                  (Lens.Family2.set (Data.ProtoLens.Field.field @"serviceId") y x)
                        18
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "network"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"network") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "address"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"address") y x)
                        34
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.isolate
                                             (Prelude.fromIntegral len) Data.ProtoLens.parseMessage)
                                       "knock"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"knock") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "ConnInfo"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"serviceId") _x
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
                   _v = Lens.Family2.view (Data.ProtoLens.Field.field @"network") _x
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
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"address") _x
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
                           Lens.Family2.view (Data.ProtoLens.Field.field @"maybe'knock") _x
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
instance Control.DeepSeq.NFData ConnInfo where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ConnInfo'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ConnInfo'serviceId x__)
                (Control.DeepSeq.deepseq
                   (_ConnInfo'network x__)
                   (Control.DeepSeq.deepseq
                      (_ConnInfo'address x__)
                      (Control.DeepSeq.deepseq (_ConnInfo'knock x__) ()))))
{- | Fields :
     
         * 'Proto.Goplugin.GrpcBroker_Fields.knock' @:: Lens' ConnInfo'Knock Prelude.Bool@
         * 'Proto.Goplugin.GrpcBroker_Fields.ack' @:: Lens' ConnInfo'Knock Prelude.Bool@
         * 'Proto.Goplugin.GrpcBroker_Fields.error' @:: Lens' ConnInfo'Knock Data.Text.Text@ -}
data ConnInfo'Knock
  = ConnInfo'Knock'_constructor {_ConnInfo'Knock'knock :: !Prelude.Bool,
                                 _ConnInfo'Knock'ack :: !Prelude.Bool,
                                 _ConnInfo'Knock'error :: !Data.Text.Text,
                                 _ConnInfo'Knock'_unknownFields :: !Data.ProtoLens.FieldSet}
  deriving stock (Prelude.Eq, Prelude.Ord)
instance Prelude.Show ConnInfo'Knock where
  showsPrec _ __x __s
    = Prelude.showChar
        '{'
        (Prelude.showString
           (Data.ProtoLens.showMessageShort __x) (Prelude.showChar '}' __s))
instance Data.ProtoLens.Field.HasField ConnInfo'Knock "knock" Prelude.Bool where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'Knock'knock
           (\ x__ y__ -> x__ {_ConnInfo'Knock'knock = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConnInfo'Knock "ack" Prelude.Bool where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'Knock'ack (\ x__ y__ -> x__ {_ConnInfo'Knock'ack = y__}))
        Prelude.id
instance Data.ProtoLens.Field.HasField ConnInfo'Knock "error" Data.Text.Text where
  fieldOf _
    = (Prelude..)
        (Lens.Family2.Unchecked.lens
           _ConnInfo'Knock'error
           (\ x__ y__ -> x__ {_ConnInfo'Knock'error = y__}))
        Prelude.id
instance Data.ProtoLens.Message ConnInfo'Knock where
  messageName _ = Data.Text.pack "plugin.ConnInfo.Knock"
  packedMessageDescriptor _
    = "\n\
      \\ENQKnock\DC2\DC4\n\
      \\ENQknock\CAN\SOH \SOH(\bR\ENQknock\DC2\DLE\n\
      \\ETXack\CAN\STX \SOH(\bR\ETXack\DC2\DC4\n\
      \\ENQerror\CAN\ETX \SOH(\tR\ENQerror"
  packedFileDescriptor _ = packedFileDescriptor
  fieldsByTag
    = let
        knock__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "knock"
              (Data.ProtoLens.ScalarField Data.ProtoLens.BoolField ::
                 Data.ProtoLens.FieldTypeDescriptor Prelude.Bool)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"knock")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo'Knock
        ack__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "ack"
              (Data.ProtoLens.ScalarField Data.ProtoLens.BoolField ::
                 Data.ProtoLens.FieldTypeDescriptor Prelude.Bool)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"ack")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo'Knock
        error__field_descriptor
          = Data.ProtoLens.FieldDescriptor
              "error"
              (Data.ProtoLens.ScalarField Data.ProtoLens.StringField ::
                 Data.ProtoLens.FieldTypeDescriptor Data.Text.Text)
              (Data.ProtoLens.PlainField
                 Data.ProtoLens.Optional (Data.ProtoLens.Field.field @"error")) ::
              Data.ProtoLens.FieldDescriptor ConnInfo'Knock
      in
        Data.Map.fromList
          [(Data.ProtoLens.Tag 1, knock__field_descriptor),
           (Data.ProtoLens.Tag 2, ack__field_descriptor),
           (Data.ProtoLens.Tag 3, error__field_descriptor)]
  unknownFields
    = Lens.Family2.Unchecked.lens
        _ConnInfo'Knock'_unknownFields
        (\ x__ y__ -> x__ {_ConnInfo'Knock'_unknownFields = y__})
  defMessage
    = ConnInfo'Knock'_constructor
        {_ConnInfo'Knock'knock = Data.ProtoLens.fieldDefault,
         _ConnInfo'Knock'ack = Data.ProtoLens.fieldDefault,
         _ConnInfo'Knock'error = Data.ProtoLens.fieldDefault,
         _ConnInfo'Knock'_unknownFields = []}
  parseMessage
    = let
        loop ::
          ConnInfo'Knock
          -> Data.ProtoLens.Encoding.Bytes.Parser ConnInfo'Knock
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
                                          ((Prelude./=) 0) Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "knock"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"knock") y x)
                        16
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (Prelude.fmap
                                          ((Prelude./=) 0) Data.ProtoLens.Encoding.Bytes.getVarInt)
                                       "ack"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"ack") y x)
                        26
                          -> do y <- (Data.ProtoLens.Encoding.Bytes.<?>)
                                       (do len <- Data.ProtoLens.Encoding.Bytes.getVarInt
                                           Data.ProtoLens.Encoding.Bytes.getText
                                             (Prelude.fromIntegral len))
                                       "error"
                                loop (Lens.Family2.set (Data.ProtoLens.Field.field @"error") y x)
                        wire
                          -> do !y <- Data.ProtoLens.Encoding.Wire.parseTaggedValueFromWire
                                        wire
                                loop
                                  (Lens.Family2.over
                                     Data.ProtoLens.unknownFields (\ !t -> (:) y t) x)
      in
        (Data.ProtoLens.Encoding.Bytes.<?>)
          (do loop Data.ProtoLens.defMessage) "Knock"
  buildMessage
    = \ _x
        -> (Data.Monoid.<>)
             (let
                _v = Lens.Family2.view (Data.ProtoLens.Field.field @"knock") _x
              in
                if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                    Data.Monoid.mempty
                else
                    (Data.Monoid.<>)
                      (Data.ProtoLens.Encoding.Bytes.putVarInt 8)
                      ((Prelude..)
                         Data.ProtoLens.Encoding.Bytes.putVarInt (\ b -> if b then 1 else 0)
                         _v))
             ((Data.Monoid.<>)
                (let _v = Lens.Family2.view (Data.ProtoLens.Field.field @"ack") _x
                 in
                   if (Prelude.==) _v Data.ProtoLens.fieldDefault then
                       Data.Monoid.mempty
                   else
                       (Data.Monoid.<>)
                         (Data.ProtoLens.Encoding.Bytes.putVarInt 16)
                         ((Prelude..)
                            Data.ProtoLens.Encoding.Bytes.putVarInt (\ b -> if b then 1 else 0)
                            _v))
                ((Data.Monoid.<>)
                   (let
                      _v = Lens.Family2.view (Data.ProtoLens.Field.field @"error") _x
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
                   (Data.ProtoLens.Encoding.Wire.buildFieldSet
                      (Lens.Family2.view Data.ProtoLens.unknownFields _x))))
instance Control.DeepSeq.NFData ConnInfo'Knock where
  rnf
    = \ x__
        -> Control.DeepSeq.deepseq
             (_ConnInfo'Knock'_unknownFields x__)
             (Control.DeepSeq.deepseq
                (_ConnInfo'Knock'knock x__)
                (Control.DeepSeq.deepseq
                   (_ConnInfo'Knock'ack x__)
                   (Control.DeepSeq.deepseq (_ConnInfo'Knock'error x__) ())))
data GRPCBroker = GRPCBroker {}
instance Data.ProtoLens.Service.Types.Service GRPCBroker where
  type ServiceName GRPCBroker = "GRPCBroker"
  type ServicePackage GRPCBroker = "plugin"
  type ServiceMethods GRPCBroker = '["startStream"]
  packedServiceDescriptor _
    = "\n\
      \\n\
      \GRPCBroker\DC25\n\
      \\vStartStream\DC2\DLE.plugin.ConnInfo\SUB\DLE.plugin.ConnInfo(\SOH0\SOH"
instance Data.ProtoLens.Service.Types.HasMethodImpl GRPCBroker "startStream" where
  type MethodName GRPCBroker "startStream" = "StartStream"
  type MethodInput GRPCBroker "startStream" = ConnInfo
  type MethodOutput GRPCBroker "startStream" = ConnInfo
  type MethodStreamingType GRPCBroker "startStream" = 'Data.ProtoLens.Service.Types.BiDiStreaming
packedFileDescriptor :: Data.ByteString.ByteString
packedFileDescriptor
  = "\n\
    \\SUBgoplugin/grpc_broker.proto\DC2\ACKplugin\"\210\SOH\n\
    \\bConnInfo\DC2\GS\n\
    \\n\
    \service_id\CAN\SOH \SOH(\rR\tserviceId\DC2\CAN\n\
    \\anetwork\CAN\STX \SOH(\tR\anetwork\DC2\CAN\n\
    \\aaddress\CAN\ETX \SOH(\tR\aaddress\DC2,\n\
    \\ENQknock\CAN\EOT \SOH(\v2\SYN.plugin.ConnInfo.KnockR\ENQknock\SUBE\n\
    \\ENQKnock\DC2\DC4\n\
    \\ENQknock\CAN\SOH \SOH(\bR\ENQknock\DC2\DLE\n\
    \\ETXack\CAN\STX \SOH(\bR\ETXack\DC2\DC4\n\
    \\ENQerror\CAN\ETX \SOH(\tR\ENQerror2C\n\
    \\n\
    \GRPCBroker\DC25\n\
    \\vStartStream\DC2\DLE.plugin.ConnInfo\SUB\DLE.plugin.ConnInfo(\SOH0\SOHB0Z.github.com/hashicorp/go-plugin/internal/pluginJ\173\ENQ\n\
    \\ACK\DC2\EOT\ETX\NUL\NAK\SOH\n\
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
    \\STX\EOT\NUL\DC2\EOT\a\NUL\DC1\SOH\n\
    \\n\
    \\n\
    \\ETX\EOT\NUL\SOH\DC2\ETX\a\b\DLE\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\NUL\DC2\ETX\b\EOT\SUB\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ENQ\DC2\ETX\b\EOT\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\SOH\DC2\ETX\b\v\NAK\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\NUL\ETX\DC2\ETX\b\CAN\EM\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\SOH\DC2\ETX\t\EOT\ETB\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ENQ\DC2\ETX\t\EOT\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\SOH\DC2\ETX\t\v\DC2\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\SOH\ETX\DC2\ETX\t\NAK\SYN\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\STX\DC2\ETX\n\
    \\EOT\ETB\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ENQ\DC2\ETX\n\
    \\EOT\n\
    \\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\SOH\DC2\ETX\n\
    \\v\DC2\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\STX\ETX\DC2\ETX\n\
    \\NAK\SYN\n\
    \\f\n\
    \\EOT\EOT\NUL\ETX\NUL\DC2\EOT\v\EOT\SI\ENQ\n\
    \\f\n\
    \\ENQ\EOT\NUL\ETX\NUL\SOH\DC2\ETX\v\f\DC1\n\
    \\r\n\
    \\ACK\EOT\NUL\ETX\NUL\STX\NUL\DC2\ETX\f\b\ETB\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\NUL\ENQ\DC2\ETX\f\b\f\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\NUL\SOH\DC2\ETX\f\r\DC2\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\NUL\ETX\DC2\ETX\f\NAK\SYN\n\
    \\r\n\
    \\ACK\EOT\NUL\ETX\NUL\STX\SOH\DC2\ETX\r\b\NAK\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\SOH\ENQ\DC2\ETX\r\b\f\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\SOH\SOH\DC2\ETX\r\r\DLE\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\SOH\ETX\DC2\ETX\r\DC3\DC4\n\
    \\r\n\
    \\ACK\EOT\NUL\ETX\NUL\STX\STX\DC2\ETX\SO\b\EM\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\STX\ENQ\DC2\ETX\SO\b\SO\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\STX\SOH\DC2\ETX\SO\SI\DC4\n\
    \\SO\n\
    \\a\EOT\NUL\ETX\NUL\STX\STX\ETX\DC2\ETX\SO\ETB\CAN\n\
    \\v\n\
    \\EOT\EOT\NUL\STX\ETX\DC2\ETX\DLE\EOT\DC4\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ACK\DC2\ETX\DLE\EOT\t\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\SOH\DC2\ETX\DLE\n\
    \\SI\n\
    \\f\n\
    \\ENQ\EOT\NUL\STX\ETX\ETX\DC2\ETX\DLE\DC2\DC3\n\
    \\n\
    \\n\
    \\STX\ACK\NUL\DC2\EOT\DC3\NUL\NAK\SOH\n\
    \\n\
    \\n\
    \\ETX\ACK\NUL\SOH\DC2\ETX\DC3\b\DC2\n\
    \\v\n\
    \\EOT\ACK\NUL\STX\NUL\DC2\ETX\DC4\EOT?\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\SOH\DC2\ETX\DC4\b\DC3\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ENQ\DC2\ETX\DC4\DC4\SUB\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\STX\DC2\ETX\DC4\ESC#\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ACK\DC2\ETX\DC4.4\n\
    \\f\n\
    \\ENQ\ACK\NUL\STX\NUL\ETX\DC2\ETX\DC45=b\ACKproto3"