package protomod

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Message struct {
	*BaseElement
	Descriptor *descriptorpb.DescriptorProto
}

func (msg *ChildElements) MessageByName(name string) *Message {
	for _, elem := range msg.elements {
		if m, ok := elem.wrapper().(*Message); ok {
			if m.name() == name {
				return m
			}
			if m := m.MessageByName(name); m != nil {
				return m
			}
		}
	}
	return nil
}

func (msg *Message) AddFieldDescriptor(descriptor *descriptorpb.FieldDescriptorProto) *Field {
	elem := newBase(msg.BaseElement, descriptor, nil)
	msg.ChildElements.insert(elem, len(msg.elements))
	return elem.wrapper().(*Field)
}

func (msg *Message) AdoptField(field *Field) {
	field.Descriptor.Number = nil
	msg.adoptChild(field.Base())
}

func (msg *Message) RemoveField(field *Field) {
	msg.removeChild(field.Base())
}

func (msg *Message) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%smessage %s {\n", prefix, msg.name())
	for _, elem := range msg.elements {
		elem.debug(indent + 1)
	}
	fmt.Println(prefix + "}")
}

type locationSpec struct {
	startLine int

	locType *locType

	path          []int32
	indexInParent int
}

func (ls locationSpec) typeChild(typeInt elementType, nextStartLine int, index int) locationSpec {
	childType := ls.locType.children[typeInt]
	childPath := append(ls.path, int32(childType.protoDescFieldNum), int32(index))
	return locationSpec{
		startLine:     nextStartLine,
		path:          childPath,
		locType:       childType,
		indexInParent: index,
	}
}

func (msg *Message) ToDescriptor(ls locationSpec) (*descriptorpb.DescriptorProto, []*namedLocation, int) {
	desc := &descriptorpb.DescriptorProto{
		Name:    msg.Descriptor.Name,
		Options: msg.Descriptor.Options,
	}

	locations := make([]*namedLocation, 0)

	usedFieldNumbers := make(map[int32]struct{})

	allFields := msg.allFields()

	needsRenumber := false
	for _, field := range allFields {
		if field.Descriptor.Number == nil {
			needsRenumber = true
			break
		}

		fieldNum := *field.Descriptor.Number
		if _, ok := usedFieldNumbers[fieldNum]; ok {
			needsRenumber = true
			break
		}
		usedFieldNumbers[fieldNum] = struct{}{}
	}
	if needsRenumber {
		for idx, field := range allFields {
			field.Descriptor.Number = proto.Int32(int32(idx + 1))
		}
	}

	nextStart := ls.startLine
	for _, elem := range msg.elements {
		switch e := elem.wrapper().(type) {
		case *Field:
			if e.Descriptor.Proto3Optional != nil && *e.Descriptor.Proto3Optional {
				oneofDesc := &descriptorpb.OneofDescriptorProto{
					Name: proto.String(fmt.Sprintf("_%s", *e.Descriptor.Name)),
				}
				e.Descriptor.OneofIndex = proto.Int32(int32(len(desc.OneofDecl)))
				desc.OneofDecl = append(desc.OneofDecl, oneofDesc)
				// adds wrapper then field
			}
			fieldDesc, fieldLocations, endLine := e.ToDescriptor(ls.typeChild(fieldType, nextStart, len(desc.Field))) //int(e.Descriptor.GetNumber())))
			desc.Field = append(desc.Field, fieldDesc)
			locations = append(locations, fieldLocations...)
			nextStart = endLine

		case *Enum:
			enumDesc, enumLocations, endLine := e.ToDescriptor(ls.typeChild(enumType, nextStart, len(desc.EnumType)))
			desc.EnumType = append(desc.EnumType, enumDesc)
			locations = append(locations, enumLocations...)
			nextStart = endLine

		case *Message:
			msgDesc, msgLocations, endLine := e.ToDescriptor(ls.typeChild(messageType, nextStart, len(desc.NestedType)))
			desc.NestedType = append(desc.NestedType, msgDesc)
			locations = append(locations, msgLocations...)
			nextStart = endLine

		case *Oneof:
			oneofIndex := len(desc.OneofDecl)
			oneofDesc, oneofLocations, _ := e.ToDescriptor(ls.typeChild(oneofType, nextStart, oneofIndex))
			desc.OneofDecl = append(desc.OneofDecl, oneofDesc.(*descriptorpb.OneofDescriptorProto))
			locations = append(locations, oneofLocations...)

			for _, field := range e.Fields() {
				fieldDesc, fieldLocations, endLine := field.ToDescriptor(ls.typeChild(fieldType, nextStart, len(desc.Field))) //int(field.Descriptor.GetNumber())))

				fieldDesc.OneofIndex = proto.Int32(int32(oneofIndex))
				desc.Field = append(desc.Field, fieldDesc)
				locations = append(locations, fieldLocations...)
				nextStart = endLine
			}

		default:
			panic(fmt.Sprintf("unknown element type %T", elem))
		}
	}

	messageLoc, endLine := msg.sourceDescriptor(ls.startLine, nextStart-ls.startLine, ls.path)
	locations = append(locations, messageLoc)

	return desc, locations, endLine
}

type Field struct {
	*BaseElement
	Descriptor *descriptorpb.FieldDescriptorProto
}

func (ff *Field) ToDescriptor(ls locationSpec) (*descriptorpb.FieldDescriptorProto, []*namedLocation, int) {
	loc, end := ff.sourceDescriptor(ls.startLine, 1, ls.path)
	return ff.Descriptor, []*namedLocation{loc}, end
}

func (field *Field) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%sfield %s  {\n", prefix, field.name())
	switch *field.Descriptor.Type {
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		fmt.Printf("%s  message '%s';\n", prefix, field.Descriptor.GetTypeName())
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		fmt.Printf("%s  enum '%s';\n", prefix, field.Descriptor.GetTypeName())
	default:
		fmt.Printf("%s  %s;\n", prefix, field.Descriptor.GetType().String())
	}

	if field.Descriptor.GetProto3Optional() {
		fmt.Printf("%s  optional;\n", prefix)
	}
	fmt.Println(prefix + "}")
}

type Enum struct {
	*BaseElement
	descriptor *descriptorpb.EnumDescriptorProto
}

func (enum *Enum) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%senum %s {\n", prefix, enum.name())
	for _, elem := range enum.elements {
		value, ok := elem.wrapper().(*EnumValue)
		if !ok {
			panic("enum value is not an EnumValue")
		}
		value.debug(indent + 1)
	}
	fmt.Println(prefix + "}")
}

func (enum *Enum) ToDescriptor(ls locationSpec) (*descriptorpb.EnumDescriptorProto, []*namedLocation, int) {
	desc := &descriptorpb.EnumDescriptorProto{
		Name:    enum.descriptor.Name,
		Value:   make([]*descriptorpb.EnumValueDescriptorProto, 0),
		Options: enum.descriptor.Options,
	}

	locs := make([]*namedLocation, 0)

	nextStart := ls.startLine
	for _, elem := range enum.elements {
		value, ok := elem.wrapper().(*EnumValue)
		if !ok {
			panic("enum value is not an EnumValue")
		}
		valueDesc, valueLocations, endLine := value.ToDescriptor(ls.typeChild(enumValueType, nextStart, len(desc.Value)))
		locs = append(locs, valueLocations...)
		desc.Value = append(desc.Value, valueDesc.(*descriptorpb.EnumValueDescriptorProto))
		nextStart = endLine
	}

	enumLoc, endLine := enum.sourceDescriptor(ls.startLine, nextStart-ls.startLine, ls.path)
	locs = append(locs, enumLoc)

	return desc, locs, endLine
}

type EnumValue struct {
	*BaseElement
	descriptor *descriptorpb.EnumValueDescriptorProto
}

func (value *EnumValue) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%s%s = %d;\n", prefix, value.name(), *value.descriptor.Number)
}

type Service struct {
	*BaseElement
	descriptor *descriptorpb.ServiceDescriptorProto
}

func (service *Service) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%sservice %s {\n", prefix, service.descriptor.GetName())
	for _, method := range service.elements {
		method.debug(indent + 1)
	}
	fmt.Println(prefix + "}")
}

type Method struct {
	*BaseElement
	descriptor *descriptorpb.MethodDescriptorProto
}

func (method *Method) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%srpc %s(%s) returns (%s);\n", prefix, method.name(), method.descriptor.GetInputType(), method.descriptor.GetOutputType())
}

type Extension struct {
	// has no descriptor

	Fields []*ExtensionField
}

type ExtensionField struct {
	descriptor *descriptorpb.FieldDescriptorProto
}

type Oneof struct {
	*BaseElement
	descriptor *descriptorpb.OneofDescriptorProto
}

func (oneof *Oneof) IsSynthetic() bool {
	fields := oneof.Fields()
	if len(fields) != 1 {
		return false
	}

	field := fields[0]
	if field.Descriptor.Proto3Optional != nil && *field.Descriptor.Proto3Optional {
		return true
	}
	return false
}

func (oneof *Oneof) debug(indent int) {

	prefix := strings.Repeat("  ", indent)

	fmt.Printf("%soneof %s {\n", prefix, oneof.name())
	oneof.elements.debug(indent + 1)
	fmt.Println(prefix + "}")
}

func (oneof *Oneof) pathType() int32 {
	return -1
}
