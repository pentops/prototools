package optionreflect

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Builder struct {
	exts map[protoreflect.FullName]map[protoreflect.FieldNumber]protoreflect.ExtensionDescriptor
}

func NewBuilder(exts []protoreflect.ExtensionDescriptor) *Builder {

	extMap := map[protoreflect.FullName]map[protoreflect.FieldNumber]protoreflect.ExtensionDescriptor{}

	for _, ext := range exts {
		msgName := ext.ContainingMessage().FullName()
		if _, ok := extMap[msgName]; !ok {
			extMap[msgName] = make(map[protoreflect.FieldNumber]protoreflect.ExtensionDescriptor)
		}
		fieldNum := ext.Number()
		extMap[msgName][fieldNum] = ext
	}

	return &Builder{
		exts: extMap,
	}
}

func (fb *Builder) findExtension(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionDescriptor, error) {
	if fb == nil {
		return nil, fmt.Errorf("builder is nil")
	}
	msgExt, ok := fb.exts[message]
	if !ok {
		return nil, fmt.Errorf("message not found")
	}
	if xt, ok := msgExt[field]; ok {
		return xt, nil
	}
	return nil, fmt.Errorf("extension not found")
}

func subLocations(locs []*descriptorpb.SourceCodeInfo_Location, pathRoot []int32) []*descriptorpb.SourceCodeInfo_Location {
	var filtered []*descriptorpb.SourceCodeInfo_Location
	for _, loc := range locs {
		if !isPrefix(pathRoot, loc.Path) {
			continue
		}
		subPath := loc.Path[len(pathRoot):]
		filtered = append(filtered, &descriptorpb.SourceCodeInfo_Location{
			Path:                    subPath,
			Span:                    loc.Span,
			LeadingComments:         loc.LeadingComments,
			TrailingComments:        loc.TrailingComments,
			LeadingDetachedComments: loc.LeadingDetachedComments,
		})
	}
	return filtered
}

func isPrefix(prefix, path []int32) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, p := range prefix {
		if p != path[i] {
			return false
		}
	}
	return true
}

func (fb *Builder) OptionsFor(parent protoreflect.Descriptor) ([]*OptionDefinition, error) {

	srcReflect := parent.Options().ProtoReflect()
	options := make([]*OptionDefinition, 0)

	// The reflection PB doesn't seem to give a way to get the source location
	// of the place the option was defined. This filters down all of the
	// locations in the parent object to the 'option' ones (7). Each field of
	// the option is then its own location.
	parentFile := parent.ParentFile()
	sourceLoc := protodesc.ToFileDescriptorProto(parentFile).SourceCodeInfo

	var optionsLocs []*descriptorpb.SourceCodeInfo_Location
	if sourceLoc != nil {
		parentRoot := parentFile.SourceLocations().ByDescriptor(parent)
		// options are at different indexes depending on the wrapper type.

		var optionFieldNumberInParent int32

		// field 7 of the Message type is the options field
		// see google/protobuf/descriptor.proto
		switch parent.(type) {
		case protoreflect.MessageDescriptor:
			optionFieldNumberInParent = 7
		case protoreflect.FieldDescriptor:
			optionFieldNumberInParent = 8
		case protoreflect.MethodDescriptor:
			optionFieldNumberInParent = 4
		case protoreflect.ServiceDescriptor:
			optionFieldNumberInParent = 3
		case protoreflect.EnumDescriptor:
			optionFieldNumberInParent = 3
		case protoreflect.EnumValueDescriptor:
			optionFieldNumberInParent = 3
		case protoreflect.OneofDescriptor:
			optionFieldNumberInParent = 2
		default:
			return nil, fmt.Errorf("unsupported parent type %T", parent)
		}

		parentPath := append(parentRoot.Path, optionFieldNumberInParent)
		optionsLocs = subLocations(sourceLoc.Location, parentPath)
	}

	type foundOption struct {
		optionNumber protoreflect.FieldNumber
		fieldDesc    protoreflect.FieldDescriptor
		fieldVal     protoreflect.Value
	}

	foundOptions := make([]foundOption, 0)

	srcReflect.Range(func(desc protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		//if !desc.IsExtension() {
		//	return true
		//}

		foundOptions = append(foundOptions, foundOption{
			optionNumber: desc.Number(),
			fieldDesc:    desc,
			fieldVal:     val,
		})

		return true
	})

	unknown := srcReflect.GetUnknown()
	if unknown != nil {
		b := unknown
		for len(b) > 0 {
			fNumber, fType, n := protowire.ConsumeTag(b)
			b = b[n:]

			if fType != protowire.BytesType {
				return nil, fmt.Errorf("unknown field type %d", fType)
			}

			raw, n := protowire.ConsumeBytes(b)
			b = b[n:]
			parentName := srcReflect.Descriptor().FullName()

			serviceExt, err := fb.findExtension(parentName, fNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to find extension: %w", err)
			}

			if serviceExt.Number() != fNumber {
				return nil, fmt.Errorf("extension number mismatch")
			}

			// TODO: This assumes all extensions are messages
			extMsg := serviceExt.Message()

			dynamicExt := dynamicpb.NewMessage(extMsg)
			if err := proto.Unmarshal(raw, dynamicExt); err != nil {
				return nil, fmt.Errorf("failed to unmarshal extension: %w", err)
			}

			foundOptions = append(foundOptions, foundOption{
				optionNumber: serviceExt.Number(),
				fieldDesc:    serviceExt,
				fieldVal:     protoreflect.ValueOfMessage(dynamicExt),
			})

		}
	}

	parentLocation := parent.ParentFile().SourceLocations().ByDescriptor(parent)

	for _, desc := range foundOptions {

		sourceLoc := buildSourceLocation(optionsLocs, parentLocation, desc.fieldDesc)
		built := &OptionDefinition{
			Context:        parent,
			Desc:           desc.fieldDesc,
			RootType:       desc.fieldDesc,
			Value:          desc.fieldVal,
			SourceLocation: sourceLoc,
		}

		options = append(options, built)
	}

	sort.Sort(optionsByLocation(options))
	return options, nil

}

type optionsByLocation []*OptionDefinition

func (o optionsByLocation) Len() int {
	return len(o)
}

func (o optionsByLocation) Less(i, j int) bool {
	if o[i].SourceLocation == nil || o[j].SourceLocation == nil {
		return o[i].Desc.Index() < o[j].Desc.Index()
	}
	if o[i].SourceLocation.StartLine == 0 || o[j].SourceLocation.StartLine == 0 {
		return o[i].Desc.Index() < o[j].Desc.Index()
	}
	return o[i].SourceLocation.StartLine < o[j].SourceLocation.StartLine
}

func (o optionsByLocation) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}
