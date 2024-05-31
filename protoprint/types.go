package protoprint

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func (fb *fileBuilder) printSection(typeName string, wrapper protoreflect.Descriptor, elements sourceElements) error {

	sort.Sort(elements)

	sourceLocation := wrapper.ParentFile().SourceLocations().ByDescriptor(wrapper)

	extensions, err := fb.out.extensions.OptionsFor(wrapper)
	if err != nil {
		return err
	}

	fb.leadingComments(sourceLocation)

	if len(elements) == 0 && len(extensions) == 0 {
		fb.p(typeName, " ", wrapper.Name(), " {}", inlineComment(sourceLocation))
		fb.trailingComments(sourceLocation)
		return nil
	}

	fb.p(typeName, " ", wrapper.Name(), " {", inlineComment(sourceLocation))
	ind := fb.indent()
	ind.trailingComments(sourceLocation)

	if len(extensions) > 0 {
		for _, ext := range extensions {
			ind.printOption(ext)
			ind.addGap()
		}
	}

	if err := ind.printElements(elements); err != nil {
		return err
	}

	fb.endElem("}")
	return nil
}

func (fb *fileBuilder) printElements(elements sourceElements) error {
	sort.Sort(elements)

	lastEnd := 0
	lastType := 0
	for _, element := range elements {

		// if there is a newline in the source file, add one here. This isn't
		// strictly necessary, but sometimes the code looks a bit better that
		// way, the reformat should preserve.
		if lastEnd > 0 && (element.sourceLocation.StartLine > lastEnd+1 || element.typeOrder != lastType) {
			fb.addGap()
		}

		lastEnd = element.sourceLocation.EndLine
		lastType = element.typeOrder

		switch et := element.descriptor.(type) {
		case protoreflect.MessageDescriptor:
			if err := fb.printMessage(et); err != nil {
				return err
			}
			fb.addGap()

		case protoreflect.ServiceDescriptor:
			if err := fb.printService(et); err != nil {
				return err
			}
			fb.addGap()

		case protoreflect.EnumDescriptor:
			if err := fb.printEnum(et); err != nil {
				return err
			}
			fb.addGap()

		case protoreflect.OneofDescriptor:
			if err := fb.printOneof(et); err != nil {
				return err
			}
			fb.addGap()

		case protoreflect.FieldDescriptor:
			if err := fb.printField(et); err != nil {
				return err
			}

		case protoreflect.EnumValueDescriptor:
			if err := fb.printEnumValue(et); err != nil {
				return err
			}

		case protoreflect.MethodDescriptor:
			if err := fb.printMethod(et); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown element type %T", et)
		}

	}
	return nil
}

func (fb *fileBuilder) printOneof(et protoreflect.OneofDescriptor) error {
	elements := newElements()
	fields := et.Fields()
	for idx := 0; idx < fields.Len(); idx++ {
		elements.add(fields.Get(idx))
	}
	return fb.printSection("oneof", et, elements)
}

func (fb *fileBuilder) printEnum(enum protoreflect.EnumDescriptor) error {
	elements := newElements()
	values := enum.Values()
	for idx := 0; idx < values.Len(); idx++ {
		elements.add(values.Get(idx))
	}
	return fb.printSection("enum", enum, elements)
}

func (fb *fileBuilder) printService(svc protoreflect.ServiceDescriptor) error {

	elements := newElements()
	methods := svc.Methods()
	for idx := 0; idx < methods.Len(); idx++ {
		elements.add(methods.Get(idx))
	}

	return fb.printSection("service", svc, elements)
}

func (fb *fileBuilder) printMessage(msg protoreflect.MessageDescriptor) error {

	elements := newElements()

	fields := msg.Fields()
	for idx := 0; idx < fields.Len(); idx++ {
		field := fields.Get(idx)
		inOneof := field.ContainingOneof()
		if inOneof != nil && !inOneof.IsSynthetic() {
			continue
		}
		elements.add(field)
	}

	oneofs := msg.Oneofs()
	for idx := 0; idx < oneofs.Len(); idx++ {
		oneof := oneofs.Get(idx)
		if oneof.IsSynthetic() {
			continue
		}
		elements.add(oneof)
	}

	nestedMessages := msg.Messages()
	for idx := 0; idx < nestedMessages.Len(); idx++ {
		nested := nestedMessages.Get(idx)
		if nested.IsMapEntry() {
			continue
		}
		elements.add(nested)
	}

	enums := msg.Enums()
	for idx := 0; idx < enums.Len(); idx++ {
		elements.add(enums.Get(idx))
	}

	return fb.printSection("message", msg, elements)
}

func (ind *fileBuilder) printMethod(method protoreflect.MethodDescriptor) error {
	svc := method.Parent()

	inputType, err := contextRefName(svc, method.Input())
	if err != nil {
		return err
	}
	outputType, err := contextRefName(svc, method.Output())
	if err != nil {
		return err
	}

	extensions, err := ind.out.extensions.OptionsFor(method)
	if err != nil {
		return err
	}

	end := " {}"
	if len(extensions) > 0 {
		end = " {"
	}

	srcLoc := method.ParentFile().SourceLocations().ByDescriptor(method)
	ind.leadingComments(srcLoc)
	ind.p("rpc ", method.Name(), "(", inputType, ") returns (", outputType, ")", end, inlineComment(srcLoc))
	ind.trailingComments(srcLoc)
	extInd := ind.indent()
	if len(extensions) > 0 {
		for _, ext := range extensions {
			extInd.printOption(ext)
		}
		ind.endElem("}")
	}

	ind.addGap()

	return nil
}

func (fb *fileBuilder) printEnumValue(field protoreflect.EnumValueDescriptor) error {

	return fb.printFieldStyle(string(field.Name()), int32(field.Number()), field)
}

type extBlock struct {
	extends protoreflect.FullName
	fields  []protoreflect.FieldDescriptor
}

func (ind *fileBuilder) printExtension(block extBlock) error {
	ind.p("extend ", block.extends, " {")
	ind2 := ind.indent()
	for _, extField := range block.fields {
		if err := ind2.printField(extField); err != nil {
			return err
		}
	}
	ind.endElem("}")
	ind.addGap()

	return nil
}

func (ind *fileBuilder) printField(field protoreflect.FieldDescriptor) error {

	var err error
	var typeName string
	var label string

	if field.IsMap() {
		keyTypeName, err := fieldTypeName(field.MapKey())
		if err != nil {
			return err
		}
		valueTypeName, err := fieldTypeName(field.MapValue())
		if err != nil {
			return err
		}
		typeName = fmt.Sprintf("map<%s, %s>", keyTypeName, valueTypeName)
	} else {
		typeName, err = fieldTypeName(field)
		if err != nil {
			return err
		}

		if field.IsList() {
			label = "repeated "
		} else if field.HasOptionalKeyword() {
			label = "optional "
		}
	}
	fieldKey := fmt.Sprintf("%s%s %s", label, typeName, field.Name())

	return ind.printFieldStyle(fieldKey, int32(field.Number()), field)

}
