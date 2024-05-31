package optionreflect

import (
	"math"
	"math/bits"
	"strconv"
	"unicode/utf8"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type FieldType int

const (
	FieldTypeScalar  FieldType = 0
	FieldTypeMessage FieldType = 1
	FieldTypeArray   FieldType = 2
)

type OptionField struct {
	FieldType   FieldType
	Key         string
	ScalarValue string
	Children    []OptionField
}

func WalkOptionField(fieldDesc protoreflect.FieldDescriptor, val protoreflect.Value) OptionField {
	if fieldDesc.IsList() {
		return walkOptionList(fieldDesc, val.List())
	}

	if fieldDesc.Kind() == protoreflect.MessageKind {
		return walkOptionMessage(fieldDesc, val.Message())
	}

	return walkOptionScalar(fieldDesc, val)
}

func walkOptionList(fieldDesc protoreflect.FieldDescriptor, list protoreflect.List) OptionField {
	out := OptionField{
		FieldType: FieldTypeArray,
		Key:       string(fieldDesc.Name()),
		Children:  make([]OptionField, 0, list.Len()),
	}

	if fieldDesc.Kind() == protoreflect.MessageKind {
		for i := 0; i < list.Len(); i++ {
			item := list.Get(i)
			msgChild := walkOptionMessage(fieldDesc, item.Message())
			out.Children = append(out.Children, msgChild)
		}
		return out
	}

	for i := 0; i < list.Len(); i++ {
		item := list.Get(i)
		msgChild := walkOptionScalar(fieldDesc, item)
		out.Children = append(out.Children, msgChild)
	}

	return out

}

func walkOptionMessage(fieldDesc protoreflect.FieldDescriptor, msgVal protoreflect.Message) OptionField {
	out := OptionField{
		FieldType: FieldTypeMessage,
		Key:       string(fieldDesc.Name()),
		Children:  make([]OptionField, 0),
	}

	fields := msgVal.Descriptor().Fields()
	for idx := 0; idx < fields.Len(); idx++ {
		fieldRefl := fields.Get(idx)
		if !msgVal.Has(fieldRefl) {
			continue
		}
		val := msgVal.Get(fieldRefl)

		child := WalkOptionField(fieldRefl, val)
		out.Children = append(out.Children, child)
	}
	return out
}

func walkOptionScalar(fieldDesc protoreflect.FieldDescriptor, val protoreflect.Value) OptionField {
	scalar, ok := marshalSingular(fieldDesc, val)
	if !ok {
		panic("unexpected scalar")
	}

	return OptionField{
		FieldType:   FieldTypeScalar,
		Key:         string(fieldDesc.Name()),
		ScalarValue: scalar,
	}
}

// adapted from prototext
func marshalSingular(fd protoreflect.FieldDescriptor, val protoreflect.Value) (string, bool) {
	kind := fd.Kind()
	switch kind {
	case protoreflect.BoolKind:
		if val.Bool() {
			return "true", true
		} else {
			return "false", true
		}

	case protoreflect.StringKind:
		return prototextString(val.String()), true

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return strconv.FormatInt(val.Int(), 10), true

	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return strconv.FormatUint(val.Uint(), 10), true

	case protoreflect.FloatKind:
		return fFloat(val.Float(), 32), true

	case protoreflect.DoubleKind:
		return fFloat(val.Float(), 64), true

	case protoreflect.BytesKind:
		return prototextString(string(val.Bytes())), true

	case protoreflect.EnumKind:
		num := val.Enum()
		if desc := fd.Enum().Values().ByNumber(num); desc != nil {
			return prototextString(string(desc.Name())), true
		} else {
			// Use numeric value if there is no enum description.
			return strconv.FormatInt(int64(num), 10), true
		}

	default:
		return "", false
	}
}

func fFloat(n float64, bitSize int) string {
	switch {
	case math.IsNaN(n):
		return "nan"
	case math.IsInf(n, +1):
		return "inf"
	case math.IsInf(n, -1):
		return "-inf"
	default:
		return strconv.FormatFloat(n, 'g', -1, bitSize)
	}
}

func prototextString(in string) string {
	outputASCII := true
	out := make([]byte, 0, len(in)+2)
	out = append(out, '"')
	i := indexNeedEscapeInString(in)
	in, out = in[i:], append(out, in[:i]...)
	for len(in) > 0 {
		switch r, n := utf8.DecodeRuneInString(in); {
		case r == utf8.RuneError && n == 1:
			// We do not report invalid UTF-8 because strings in the text format
			// are used to represent both the proto string and bytes type.
			r = rune(in[0])
			fallthrough
		case r < ' ' || r == '"' || r == '\\' || r == 0x7f:
			out = append(out, '\\')
			switch r {
			case '"', '\\':
				out = append(out, byte(r))
			case '\n':
				out = append(out, 'n')
			case '\r':
				out = append(out, 'r')
			case '\t':
				out = append(out, 't')
			default:
				out = append(out, 'x')
				out = append(out, "00"[1+(bits.Len32(uint32(r))-1)/4:]...)
				out = strconv.AppendUint(out, uint64(r), 16)
			}
			in = in[n:]
		case r >= utf8.RuneSelf && (outputASCII || r <= 0x009f):
			out = append(out, '\\')
			if r <= math.MaxUint16 {
				out = append(out, 'u')
				out = append(out, "0000"[1+(bits.Len32(uint32(r))-1)/4:]...)
				out = strconv.AppendUint(out, uint64(r), 16)
			} else {
				out = append(out, 'U')
				out = append(out, "00000000"[1+(bits.Len32(uint32(r))-1)/4:]...)
				out = strconv.AppendUint(out, uint64(r), 16)
			}
			in = in[n:]
		default:
			i := indexNeedEscapeInString(in[n:])
			in, out = in[n+i:], append(out, in[:n+i]...)
		}
	}
	out = append(out, '"')
	return string(out)
}

// indexNeedEscapeInString returns the index of the character that needs
// escaping. If no characters need escaping, this returns the input length.
func indexNeedEscapeInString(s string) int {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < ' ' || c == '"' || c == '\'' || c == '\\' || c >= 0x7f {
			return i
		}
	}
	return len(s)
}
