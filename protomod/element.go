package protomod

import (
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/descriptorpb"
)

type descriptorInterface interface {
	GetName() string
}

type BaseElement struct {
	id         string
	parent     *BaseElement
	Descriptor descriptorInterface
	source     ElementSource

	ChildElements
}

func (elem *BaseElement) Base() *BaseElement {
	// Implements ElementWrapper
	return elem
}

func newBase(parent *BaseElement, desc descriptorInterface, loc *location) *BaseElement {
	source := ElementSource{}

	if loc != nil {
		source.initialStartLine = loc.startLine
		if loc.location != nil {
			source.LeadingComments = loc.location.LeadingComments
			source.TrailingComments = loc.location.TrailingComments
			source.LeadingDetached = loc.location.LeadingDetachedComments
		}
	}
	return &BaseElement{
		id:         uuid.NewString(), // this should probably be the pointer address or something
		parent:     parent,
		Descriptor: desc,
		source:     source,
	}
}

type ElementWrapper interface {
	Base() *BaseElement
	debug(indent int)
}

func (elem *BaseElement) debug(indent int) {
	elem.wrapper().debug(indent)
}

func (elem *BaseElement) wrapper() ElementWrapper {

	switch desc := elem.Descriptor.(type) {
	case *descriptorpb.FieldDescriptorProto:
		return &Field{BaseElement: elem, Descriptor: desc}

	case *descriptorpb.EnumDescriptorProto:
		return &Enum{BaseElement: elem, descriptor: desc}

	case *descriptorpb.EnumValueDescriptorProto:
		return &EnumValue{BaseElement: elem, descriptor: desc}

	case *descriptorpb.DescriptorProto:
		return &Message{BaseElement: elem, Descriptor: desc}

	case *descriptorpb.OneofDescriptorProto:
		return &Oneof{BaseElement: elem, descriptor: desc}

	case *descriptorpb.ServiceDescriptorProto:
		return &Service{BaseElement: elem, descriptor: desc}

	case *descriptorpb.MethodDescriptorProto:
		return &Method{BaseElement: elem, descriptor: desc}

	case *descriptorpb.FileDescriptorProto:
		return &File{BaseElement: elem, descriptor: desc}

	default:
		panic("unknown element type")
	}

}

func (elem *BaseElement) Parent() *BaseElement {
	return elem.parent
}

func (a *BaseElement) adoptChild(elem *BaseElement) {
	elem.setParent(a)
	a.elements = append(a.elements, elem)
}

func (elem *BaseElement) setParent(parent *BaseElement) {
	if elem.parent != nil {
		elem.parent.removeChild(elem)
	}
	elem.parent = parent
}

func (elem *BaseElement) name() string {
	return elem.Descriptor.GetName()
}

func (elem *BaseElement) fullName() string {
	parts := make([]string, 0, 1)
	parts = append(parts, elem.name())
	pp := elem.Parent()
	for pp != nil {
		fileElement, ok := pp.wrapper().(*File)
		if ok {
			parts = append(parts, fileElement.Package())
			break
		}
		parts = append(parts, pp.name())
		pp = pp.Parent()
	}
	out := make([]string, len(parts))
	for idx, part := range parts {
		out[len(parts)-idx-1] = part
	}

	return strings.Join(out, ".")
}

func (elem *BaseElement) startLine() int32 {
	return elem.source.initialStartLine
}

func (elem *BaseElement) sourceDescriptor(startLine int, numLines int, path []int32) (*namedLocation, int) {

	dd := &descriptorpb.SourceCodeInfo_Location{}

	dd.LeadingComments = elem.source.LeadingComments
	dd.TrailingComments = elem.source.TrailingComments
	dd.LeadingDetachedComments = elem.source.LeadingDetached

	dd.Path = path

	dd.Span = []int32{int32(startLine), 0, int32(startLine + numLines), 0}
	return &namedLocation{
		info:    dd,
		element: elem.fullName(),
	}, startLine + numLines

}

func (elem *BaseElement) ToDescriptor(ls locationSpec) (descriptorInterface, []*namedLocation, int) {
	// Default implementation assumes that the element is a leaf
	loc, end := elem.sourceDescriptor(ls.startLine, 1, ls.path)
	return elem.Descriptor, []*namedLocation{loc}, end

}
