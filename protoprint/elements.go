package protoprint

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

type sourceElement struct {
	typeOrder      int
	descriptor     protoreflect.Descriptor
	sourceLocation protoreflect.SourceLocation
}

type sourceElements []sourceElement

func newElements() sourceElements {
	return make(sourceElements, 0)
}

func (se *sourceElements) add(d protoreflect.Descriptor) {

	typeOrder := 0
	switch d.(type) {
	case protoreflect.MessageDescriptor:
		typeOrder = 1
	case protoreflect.EnumDescriptor:
		typeOrder = 2
	case protoreflect.ServiceDescriptor:
		typeOrder = 0
	}

	*se = append(*se, sourceElement{
		typeOrder:      typeOrder,
		descriptor:     d,
		sourceLocation: d.ParentFile().SourceLocations().ByDescriptor(d),
	})
}

func (se sourceElements) Len() int {
	return len(se)
}

func (se sourceElements) Less(i, j int) bool {
	if se[i].sourceLocation.StartLine == 0 || se[j].sourceLocation.StartLine == 0 {
		if se[i].typeOrder != se[j].typeOrder {
			return se[i].typeOrder < se[j].typeOrder
		}
		return se[i].descriptor.Index() < se[j].descriptor.Index()
	}
	return se[i].sourceLocation.StartLine < se[j].sourceLocation.StartLine
}

func (se sourceElements) Swap(i, j int) {
	se[i], se[j] = se[j], se[i]
}
