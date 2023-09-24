package protobuflog

import (
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func NewEncoder[T proto.Message](dst io.Writer) *Encoder[T] {
	return &Encoder[T]{dst: dst}
}

type Encoder[T proto.Message] struct {
	dst io.Writer
}

func (t Encoder[T]) Encode(msgs ...T) error {
	return EncodeEvery(t.dst, msgs...)
}

func EncodeEvery[T proto.Message](dst io.Writer, msg ...T) (err error) {
	for _, m := range msg {
		if err = encode(dst, m); err != nil {
			return err
		}
	}
	return nil
}

// Encode fundamental encoding method. creates a byte array representing the proto.Message.
func Encode(m proto.Message) (encoded []byte, err error) {
	if encoded, err = proto.Marshal(m); err != nil {
		return encoded, err
	}

	return encodebytes(encoded), nil
}

func encode(dst io.Writer, m proto.Message) (err error) {
	var (
		encoded []byte
	)

	if encoded, err = Encode(m); err != nil {
		return err
	}

	if _, err = dst.Write(encoded); err != nil {
		return err
	}

	return nil
}

func encodebytes(encoded []byte) []byte {
	var (
		buf = make([]byte, 8)
	)

	binary.LittleEndian.PutUint64(buf, uint64(len(encoded)))
	return append(buf, encoded...)
}

func NewDecoder[T proto.Message](src io.Reader) *Decoder[T] {
	return &Decoder[T]{src: src}
}

type Decoder[T proto.Message] struct {
	src io.Reader
}

func (t Decoder[T]) Decode(msgs *[]T) error {
	return DecodeEvery(t.src, msgs)
}

// Decode a single proto message from the buffer.
func Decode(src io.Reader, m protoreflect.ProtoMessage) (err error) {
	var (
		buf []byte
	)

	if buf, err = decodebytes(src); err != nil {
		return err
	}

	return proto.Unmarshal(buf, m)
}

// DecodeEvery reads all messages from the reader into an array
func DecodeEvery[T proto.Message](src io.Reader, buf *[]T) (err error) {
	for i := 0; i < cap(*buf); i++ {
		var decoded T
		x := decoded.ProtoReflect().New().Interface()
		if err = Decode(src, x); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		*buf = append(*buf, x.(T))
	}

	return nil
}

func decodebytes(src io.Reader) (buf []byte, err error) {
	buf = make([]byte, 8)

	if _, err = io.ReadFull(src, buf); err != nil {
		return buf, err
	}

	length := binary.LittleEndian.Uint64(buf)
	buf = make([]byte, length)

	if _, err = io.ReadFull(src, buf); err != nil {
		return buf, err
	}

	return buf, err
}
