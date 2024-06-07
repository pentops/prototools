package protomod

import (
	"google.golang.org/protobuf/types/descriptorpb"
)

func WalkFileDescriptor(file *descriptorpb.FileDescriptorProto) *File {
	locations := buildLocations(file)
	root := locations.root

	elem := walkFileDescriptor(file, root)
	return elem.wrapper().(*File)
}

type Elements []*BaseElement

type ChildElements struct {
	elements Elements
}

func (a *ChildElements) append(elem *BaseElement) {
	a.elements = append(a.elements, elem)
}

func (a *ChildElements) Fields() []*Field {
	out := make([]*Field, 0, len(a.elements))
	for _, elem := range a.elements {
		if field, ok := elem.wrapper().(*Field); ok {
			out = append(out, field)
		}
	}
	return out
}

func (a *ChildElements) insert(elem *BaseElement, index int) {
	if index >= len(a.elements) {
		newElements := append(a.elements, elem)
		a.elements = newElements
		return
	}
	newElements := make([]*BaseElement, 0, len(a.elements)+1)
	for idx, e := range a.elements {
		if idx == index {
			newElements = append(newElements, elem)
		}
		newElements = append(newElements, e)
	}

	a.elements = newElements
}

func (a *ChildElements) removeChild(elem *BaseElement) {
	newElements := make([]*BaseElement, 0, len(a.elements)-1)
	for _, e := range a.elements {
		if e.id != elem.id {
			newElements = append(newElements, e)
		}
	}
	a.elements = newElements
}

func (a *ChildElements) allFields() []*Field {
	out := make([]*Field, 0, len(a.elements))
	for _, elem := range a.elements {
		if field, ok := elem.wrapper().(*Field); ok {
			out = append(out, field)
			continue
		}
		if oneof, ok := elem.wrapper().(*Oneof); ok {
			out = append(out, oneof.Fields()...)
		}
	}
	return out
}

func (a Elements) debug(indent int) {
	for _, elem := range a {
		elem.debug(indent)
	}
}

type elementsByStartLine []*BaseElement

func (a elementsByStartLine) Len() int           { return len(a) }
func (a elementsByStartLine) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a elementsByStartLine) Less(i, j int) bool { return a[i].startLine() < a[j].startLine() }

type namedLocation struct {
	element string
	info    *descriptorpb.SourceCodeInfo_Location
}

type ElementSource struct {
	LeadingComments  *string
	TrailingComments *string
	LeadingDetached  []string
	initialStartLine int32
}
