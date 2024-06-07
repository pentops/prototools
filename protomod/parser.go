package protomod

import (
	"sort"

	"google.golang.org/protobuf/types/descriptorpb"
)

func walkFileDescriptor(file *descriptorpb.FileDescriptorProto, rootLoc *location) *BaseElement {
	base := newBase(nil, file, rootLoc)

	for idx, message := range file.MessageType {
		loc := rootLoc.typeChild(messageType, int32(idx))
		base.append(walkMessageDescriptor(message, base, loc))
	}

	for idx, enum := range file.EnumType {
		loc := rootLoc.typeChild(enumType, int32(idx))
		base.append(walkEnumDescriptor(enum, base, loc))
	}

	for idx, service := range file.Service {
		loc := rootLoc.typeChild(serviceType, int32(idx))
		base.append(walkServiceDescriptor(service, base, loc))
	}

	sort.Sort(elementsByStartLine(base.elements))
	return base
}

func walkMessageDescriptor(desc *descriptorpb.DescriptorProto, parent *BaseElement, msgLoc *location) *BaseElement {
	base := newBase(parent, desc, msgLoc)

	syntheticOneofs := make(map[int32]struct{})
	for _, field := range desc.Field {
		if field.OneofIndex != nil && field.GetProto3Optional() {
			syntheticOneofs[*field.OneofIndex] = struct{}{}
		}
	}

	oneofByIndex := make(map[int32]*BaseElement)

	for idx, oneof := range desc.OneofDecl {
		if _, ok := syntheticOneofs[int32(idx)]; ok {
			continue
		}

		loc := msgLoc.typeChild(oneofType, int32(idx))
		elem := walkOneofDescriptor(oneof, base, loc)
		base.append(elem)
		oneofByIndex[int32(idx)] = elem
	}

	for idx, field := range desc.Field {
		loc := msgLoc.typeChild(fieldType, int32(idx))
		if field.OneofIndex != nil && !field.GetProto3Optional() {
			oneofIndex := *field.OneofIndex
			oneof, ok := oneofByIndex[oneofIndex]
			if !ok {
				panic("oneof not found")
			}

			fieldElement := walkFieldDescriptor(field, oneof, loc)
			oneof.elements = append(oneof.elements, fieldElement)
		} else {
			fieldElement := walkFieldDescriptor(field, base, loc)
			base.append(fieldElement)
		}
	}

	for idx, enum := range desc.EnumType {
		loc := msgLoc.typeChild(enumType, int32(idx))
		base.append(walkEnumDescriptor(enum, base, loc))
	}

	for idx, message := range desc.NestedType {
		loc := msgLoc.typeChild(messageType, int32(idx))
		base.append(walkMessageDescriptor(message, base, loc))
	}

	sort.Sort(elementsByStartLine(base.elements))
	return base
}

func walkServiceDescriptor(desc *descriptorpb.ServiceDescriptorProto, parent *BaseElement, serviceLoc *location) *BaseElement {
	base := newBase(parent, desc, serviceLoc)

	for idx, method := range desc.Method {
		loc := serviceLoc.typeChild(methodType, int32(idx))
		base.append(newBase(base, method, loc))
	}

	sort.Sort(elementsByStartLine(base.elements))
	return base
}

func walkEnumDescriptor(desc *descriptorpb.EnumDescriptorProto, parent *BaseElement, enumLoc *location) *BaseElement {
	base := newBase(parent, desc, enumLoc)
	for idx, value := range desc.Value {
		loc := enumLoc.typeChild(enumValueType, int32(idx))
		base.append(newBase(base, value, loc))
	}
	sort.Sort(elementsByStartLine(base.elements))
	return base
}

func walkOneofDescriptor(desc *descriptorpb.OneofDescriptorProto, parent *BaseElement, loc *location) *BaseElement {
	return newBase(parent, desc, loc)
}

func walkFieldDescriptor(desc *descriptorpb.FieldDescriptorProto, parent *BaseElement, loc *location) *BaseElement {
	return newBase(parent, desc, loc)
}
