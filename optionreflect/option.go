package optionreflect

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type OptionDefinition struct {
	Context protoreflect.Descriptor

	RootType protoreflect.FieldDescriptor
	SubPath  []string // ().after.the.brackets

	Desc  protoreflect.FieldDescriptor
	Value protoreflect.Value

	SourceLocation *OptionSourceLocation
}

func (opt *OptionDefinition) FullType() string {
	if len(opt.SubPath) == 0 {
		return fmt.Sprintf("(%s)", opt.RootType.FullName())
	}

	return fmt.Sprintf("(%s).%s", opt.RootType.FullName(), strings.Join(opt.SubPath, "."))
}

// Simplify moves as much of the definition tree possible to the SubPath.
func (opt *OptionDefinition) Simplify(maxDepth int) {
	encoderDesc := opt.Desc
	encoderVal := opt.Value

	if len(opt.SubPath) > maxDepth {
		return
	}

	if encoderDesc.Kind() != protoreflect.MessageKind {
		// Can't walk scalars.
		return

		//printAsScalar = true
		//break
	}

	if encoderDesc.IsList() || encoderDesc.IsMap() {
		// Can't walk lists or maps.
		return
	}

	encoderMessageDesc := encoderDesc.Message()
	encoderMessageVal := encoderVal.Message()
	descFields := encoderMessageDesc.Fields()

	definedFields := make([]protoreflect.FieldDescriptor, 0, descFields.Len())
	for i := 0; i < descFields.Len(); i++ {
		field := descFields.Get(i)
		if encoderMessageVal.Has(field) {
			definedFields = append(definedFields, field)
		}
	}

	if len(definedFields) != 1 {
		// More than one field, can't be simplified
		return
	}

	field := definedFields[0]
	if field.IsMap() || field.IsList() {
		// Maps or lists begin with '[', there does not appear to be a syntax like
		// `(foo.bar).thing = [`
		// so we can't simplify the message *containing* the list.
		return
	}

	opt.SubPath = append(opt.SubPath, string(field.Name()))
	opt.Desc = field
	opt.Value = encoderMessageVal.Get(field)

	opt.Simplify(maxDepth)
}

type OptionSourceLocation struct {
	// The option is specified on the same line as the thing it is an option for
	InLineWithParent bool

	// The option is squashed up into one line
	SingleLine bool

	StartLine int32

	Src    *descriptorpb.SourceCodeInfo_Location
	Parent protoreflect.SourceLocation
}

func buildSourceLocation(optionsLocs []*descriptorpb.SourceCodeInfo_Location, parentLocation protoreflect.SourceLocation, field protoreflect.FieldDescriptor) *OptionSourceLocation {

	num := field.Number()
	srcLoc := subLocations(optionsLocs, []int32{int32(num)})

	if len(srcLoc) != 1 {
		return nil
	}

	singleLine := false
	startLine := srcLoc[0].Span[0]
	var endLine int32
	if len(srcLoc[0].Span) == 3 {
		endLine = startLine
	} else {
		endLine = srcLoc[0].Span[2]
	}
	if startLine == endLine {
		singleLine = true
	}
	inlineWithPareht := singleLine && parentLocation.StartLine == int(startLine)
	return &OptionSourceLocation{
		InLineWithParent: inlineWithPareht,
		SingleLine:       singleLine,
		StartLine:        startLine,

		Src:    srcLoc[0],
		Parent: parentLocation,
	}

}
