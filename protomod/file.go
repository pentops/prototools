package protomod

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type File struct {
	*BaseElement
	descriptor *descriptorpb.FileDescriptorProto
}

func (wf *File) Import(name string) {
	for _, importFile := range wf.descriptor.Dependency {
		if importFile == name {
			return
		}
	}
	wf.descriptor.Dependency = append(wf.descriptor.Dependency, name)
}

func (wf *File) ToDescriptor() *descriptorpb.FileDescriptorProto {

	desc := &descriptorpb.FileDescriptorProto{
		Syntax:     wf.descriptor.Syntax,
		Name:       wf.descriptor.Name,
		Package:    wf.descriptor.Package,
		Dependency: wf.descriptor.Dependency,
		Options:    wf.descriptor.Options,

		SourceCodeInfo: &descriptorpb.SourceCodeInfo{},
	}

	locations := make([]*namedLocation, 0)

	ls := &locationSpec{
		startLine:     10,
		locType:       fileLocType,
		path:          []int32{},
		indexInParent: 0,
	}

	nextStart := 10
	for _, elem := range wf.elements {
		switch e := elem.wrapper().(type) {
		case *Message:
			msgDesc, msgLoc, endLine := e.ToDescriptor(ls.typeChild(messageType, nextStart, len(desc.MessageType)))
			desc.MessageType = append(desc.MessageType, msgDesc)
			locations = append(locations, msgLoc...)
			nextStart = endLine
		case *Enum:
			enumDesc, enumLoc, endLine := e.ToDescriptor(ls.typeChild(enumType, nextStart, len(desc.EnumType)))
			desc.EnumType = append(desc.EnumType, enumDesc)
			locations = append(locations, enumLoc...)
			nextStart = endLine
		default:
			panic(fmt.Sprintf("unknown element type %T", elem))
		}
	}

	for _, loc := range locations {
		if loc == nil {
			continue
		}
		if loc.info == nil || len(loc.info.Span) == 0 {
			fmt.Printf("item %s has no span\n", loc.element)
		}

		fmt.Printf("loc %v %v %s\n", loc.info.Span, loc.info.Path, loc.element)
		desc.SourceCodeInfo.Location = append(desc.SourceCodeInfo.Location, loc.info)
	}

	return desc
}

func (wf *File) NewMessage(name string, insertAt int) *Message {
	desc := &descriptorpb.DescriptorProto{
		Name:    proto.String(name),
		Options: &descriptorpb.MessageOptions{},
	}

	elem := newBase(wf.BaseElement, desc, nil)
	wf.ChildElements.insert(elem, insertAt)
	return elem.wrapper().(*Message)
}

func (wf *File) MessageByName(name string) *Message {
	for _, elem := range wf.elements {
		if msg, ok := elem.wrapper().(*Message); ok {
			if msg.Descriptor.GetName() == name {
				return msg
			}
			if nested := msg.MessageByName(name); nested != nil {
				return nested
			}
		}
	}
	return nil
}

func (wf *File) Package() string {
	return wf.descriptor.GetPackage()
}

func (wf *File) Debug() {
	wf.debug(0)
}

func (wf *File) debug(indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%sfile %s {\n", prefix, wf.name())
	for _, elem := range wf.elements {
		elem.debug(indent + 1)
	}
	fmt.Println(prefix + "}")
}
