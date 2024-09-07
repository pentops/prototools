package protoprint

import (
	"fmt"
	"strings"

	"github.com/pentops/prototools/optionreflect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var maxExtDepth = map[protoreflect.FullName]int{
	"google.api.http": 0,
	//"buf.validate.field": 1,
}

type parsedOption struct {
	def          *optionreflect.OptionDefinition
	root         optionreflect.OptionField
	inline       bool
	inlineString *string
}

func parseOption(opt *optionreflect.OptionDefinition) parsedOption {

	maxDepth, ok := maxExtDepth[opt.Desc.FullName()]
	if !ok {
		maxDepth = 5
	}
	simplify := true

	// convention seems to dictate these are always specified as
	// (google.api.http) = { ... }
	// even if it's just a get.
	if opt.Desc.FullName() == "google.api.http" {
		simplify = false
	}

	if simplify {
		opt.Simplify(maxDepth)
	}

	root := optionreflect.WalkOptionField(opt.Desc, opt.Value)

	sourceSingleLine := opt.SourceLocation == nil || opt.SourceLocation.SingleLine
	inlineWithParent := opt.SourceLocation == nil || opt.SourceLocation.InLineWithParent

	parsed := parsedOption{
		root:   root,
		def:    opt,
		inline: inlineWithParent,
	}

	if !sourceSingleLine {
		return parsed
	}

	switch root.FieldType {
	case optionreflect.FieldTypeMessage:
		if len(root.Children) == 0 {
			parsed.inlineString = proto.String("{}")
			return parsed
		}
		if len(root.Children) == 1 {
			child := root.Children[0]
			if child.FieldType == optionreflect.FieldTypeScalar {
				parsed.inlineString = proto.String(fmt.Sprintf("{%s: %s}", child.Key, child.ScalarValue))
				return parsed
			}
		}
		return parsed
	case optionreflect.FieldTypeArray:
		if len(root.Children) == 0 {
			parsed.inlineString = proto.String("[]")
			return parsed
		}
		return parsed

	case optionreflect.FieldTypeScalar:
		parsed.inlineString = proto.String(root.ScalarValue)
		return parsed

	default:
		panic(fmt.Sprintf("unexpected type %v", root.FieldType))
	}
}

func (fb *fileBuilder) optionsFor(thing protoreflect.Descriptor) ([]parsedOption, error) {

	options, err := fb.out.extensions.OptionsFor(thing)
	if err != nil {
		return nil, err
	}

	parsed := make([]parsedOption, 0, len(options))
	for _, opt := range options {
		parsed = append(parsed, parseOption(opt))
	}
	return parsed, nil
}

func (extInd *fileBuilder) printOption(opt *optionreflect.OptionDefinition) {

	parsed := parseOption(opt)

	typeName := optionTypeName(opt)
	if parsed.inlineString != nil {
		extInd.p("option ", typeName, " = ", *parsed.inlineString, ";")
		return
	}

	switch parsed.root.FieldType {
	case optionreflect.FieldTypeMessage:
		if len(parsed.root.Children) == 0 {
			extInd.p("option ", typeName, " = {};")
		}
		extInd.p("option ", typeName, " = {")
		extInd.printOptionMessageFields(parsed.root.Children)
		extInd.endElem("};")

	case optionreflect.FieldTypeArray:
		opener := fmt.Sprintf("option %s", typeName)
		extInd.printOptionArray(opener, parsed.root.Children, ";")

	case optionreflect.FieldTypeScalar:
		extInd.p("option ", typeName, " = ", parsed.root.ScalarValue, ";")

	}

}

func optionTypeName(opt *optionreflect.OptionDefinition) string {

	name, err := contextRefName(opt.Context, opt.RootType)
	if err != nil {
		panic(err.Error())
	}

	if len(opt.SubPath) == 0 {
		return fmt.Sprintf("(%s)", name)
	}

	return fmt.Sprintf("(%s).%s", name, strings.Join(opt.SubPath, "."))
}

func (ind *fileBuilder) printOptionArray(opener string, children []optionreflect.OptionField, trailer string) {
	if len(children) == 0 {
		ind.p(opener, ": []", trailer)
		return
	}
	if len(children) == 1 && children[0].FieldType == optionreflect.FieldTypeScalar {
		ind.p(opener, ": [", children[0].ScalarValue, "]", trailer)
		return
	}
	if children[0].FieldType == optionreflect.FieldTypeMessage {
		ind.p(opener, ": [{")
		for idx, child := range children {
			if idx != 0 {
				ind.p("}, {")
			}
			ind.printOptionMessageFields(child.Children)
		}
		ind.endElem("}]", trailer)
		return
	}
}

func (ind *fileBuilder) printOptionMessageFields(children []optionreflect.OptionField) {
	ind2 := ind.indent()
	for _, child := range children {
		switch child.FieldType {
		case optionreflect.FieldTypeMessage:
			ind2.p(child.Key, ": {")
			ind2.printOptionMessageFields(child.Children)
			ind2.endElem("}")
		case optionreflect.FieldTypeArray:
			ind2.printOptionArray(child.Key, child.Children, "")
		case optionreflect.FieldTypeScalar:
			ind2.p(child.Key, ": ", child.ScalarValue)
		}
	}

}

func (fb *fileBuilder) printFieldStyle(name string, number int32, elem protoreflect.Descriptor) error {

	srcLoc := elem.ParentFile().SourceLocations().ByDescriptor(elem)

	options, err := fb.optionsFor(elem)
	if err != nil {
		return err
	}

	fb.leadingComments(srcLoc)

	if len(options) == 0 {
		fb.p(name, " = ", number, ";", inlineComment(srcLoc))
	} else if len(options) == 1 && options[0].inline && options[0].inlineString != nil {
		opt := options[0]
		fb.p(name, " = ", number, " [", optionTypeName(opt.def), " = ", *opt.inlineString, "];", inlineComment(srcLoc))
	} else {
		fb.p(name, " = ", number, " [", inlineComment(srcLoc))
		extInd := fb.indent()
		for idx, parsed := range options {
			trailer := ","
			if idx == len(options)-1 {
				trailer = ""
			}
			opt := parsed.def
			val := parsed.root

			if parsed.inlineString != nil {
				extInd.p(optionTypeName(opt), " = ", *parsed.inlineString, trailer)
				continue
			}

			switch val.FieldType {
			case optionreflect.FieldTypeMessage:
				extInd.p(optionTypeName(opt), " = {")
				extInd.printOptionMessageFields(parsed.root.Children)
				extInd.endElem("}", trailer)
			case optionreflect.FieldTypeArray:
				extInd.printOptionArray(optionTypeName(opt), parsed.root.Children, trailer)
			case optionreflect.FieldTypeScalar:
				extInd.p(optionTypeName(opt), " = ", parsed.root.ScalarValue, trailer)
			}
		}
		fb.endElem("];", inlineComment(srcLoc))
	}
	fb.trailingComments(srcLoc)

	return nil
}
