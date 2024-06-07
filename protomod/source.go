package protomod

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/types/descriptorpb"
)

type elementType string
type protoSourceNumber int32

const (
	fileType      = elementType("file")
	messageType   = elementType("message")
	enumType      = elementType("enum")
	extensionType = elementType("extension")
	optionsType   = elementType("options")
	serviceType   = elementType("service")
	methodType    = elementType("method")
	oneofType     = elementType("oneof")
	enumValueType = elementType("enum value")
	fieldType     = elementType("field")

	nameType       = elementType("name")
	packageType    = elementType("package")
	dependencyType = elementType("dependency")
	syntaxType     = elementType("syntax")
	editionType    = elementType("edition")

	reservedRangeType = elementType("reserved range")
)

type locType struct {
	protoDescFieldNum protoSourceNumber
	name              string
	children          map[elementType]*locType
	fromProto         map[protoSourceNumber]elementType
}

func (lt *locType) fillChildren() {
	if lt.fromProto != nil {
		return
	}
	lt.fromProto = make(map[protoSourceNumber]elementType)
	for eleType, child := range lt.children {
		if child.name == "" {
			child.name = string(eleType)
		}
		lt.fromProto[child.protoDescFieldNum] = eleType
		child.fillChildren()
	}
}

var fileLocType *locType

func init() {
	messageLocType := &locType{
		protoDescFieldNum: 4,
		name:              "message",
		children: map[elementType]*locType{
			fieldType: {
				protoDescFieldNum: 2,
				name:              "field",
				children: map[elementType]*locType{
					optionsType: {
						protoDescFieldNum: 8,
						name:              "options",
					},
				},
			},
			oneofType:         {protoDescFieldNum: 8, name: "oneof"},
			optionsType:       {protoDescFieldNum: 7, name: "options"},
			enumType:          {protoDescFieldNum: 4, name: "enum"},
			reservedRangeType: {protoDescFieldNum: 9, name: "reserved range"},
		},
	}

	nestedMessageType := &locType{
		protoDescFieldNum: 3,
		name:              "nested message",
		children:          messageLocType.children,
	}

	messageLocType.children[messageType] = nestedMessageType

	fileLocType = &locType{
		protoDescFieldNum: 0,
		name:              "file",
		children: map[elementType]*locType{
			messageType: messageLocType,
			enumType: {
				name:              "enum",
				protoDescFieldNum: 5,
				children: map[elementType]*locType{
					enumValueType: {protoDescFieldNum: 2},
				},
			},
			serviceType: {
				name:              "service",
				protoDescFieldNum: 6,
				children: map[elementType]*locType{
					methodType: {protoDescFieldNum: 2},
				},
			},
			extensionType: {protoDescFieldNum: 7},
			optionsType:   {protoDescFieldNum: 8},

			// Boring
			nameType:       {protoDescFieldNum: 1},  // name
			packageType:    {protoDescFieldNum: 2},  // package
			dependencyType: {protoDescFieldNum: 3},  // dependency
			syntaxType:     {protoDescFieldNum: 12}, // syntax
			editionType:    {protoDescFieldNum: 14}, // edition

			"_25": {protoDescFieldNum: 25}, // NFI 1
			"_30": {protoDescFieldNum: 30}, // NFI 2
			"_20": {protoDescFieldNum: 20}, // NFI 3
			"_10": {protoDescFieldNum: 10}, // Public Dependency

		},
	}
	fileLocType.fillChildren()

}

type locationSet struct {
	info *descriptorpb.SourceCodeInfo

	root *location
}

func buildLocations(file *descriptorpb.FileDescriptorProto) *locationSet {

	fmt.Printf("buildLocations %s\n", file.GetName())
	root := newLocation(fileLocType, nil)

	for _, loc := range file.SourceCodeInfo.Location {
		if len(loc.Path) == 0 {
			root.location = loc
			continue
		}
		if len(loc.Path) == 1 {
			// scalar elements on the file
			continue
		}

		walkRoot := root
		rest := loc.Path

		var partIndex int32
		var partType protoSourceNumber

		for len(rest) > 0 {
			if len(rest) == 1 {
				child := walkRoot.singleChild(protoSourceNumber(rest[0]))
				child.location = loc
				break
			}
			partType, partIndex, rest = protoSourceNumber(rest[0]), rest[1], rest[2:]
			child := walkRoot.child(partType, partIndex)
			if len(rest) == 0 {
				child.location = loc
			}
			walkRoot = child
		}
	}

	if err := root.walk(func(l *location) error {
		l.startLine = l.location.Span[0]
		if len(l.location.Span) == 4 {
			l.endLine = l.location.Span[2]
		} else {
			l.endLine = l.startLine
		}
		return nil
	}); err != nil {

		panic(err)
	}

	return &locationSet{
		info: file.SourceCodeInfo,
		root: root,
	}

}

type location struct {
	location *descriptorpb.SourceCodeInfo_Location

	startLine int32
	endLine   int32

	pathIndex int32

	parent *location

	locType *locType

	children map[protoSourceNumber]map[int32]*location

	singles map[protoSourceNumber]*location
}

func newLocation(typeSet *locType, parent *location) *location {
	return &location{
		children: make(map[protoSourceNumber]map[int32]*location),
		singles:  make(map[protoSourceNumber]*location),
		locType:  typeSet,
		parent:   parent,
	}
}

func (l *location) singleChild(index protoSourceNumber) *location {
	child, ok := l.singles[index]
	if ok {
		return child
	}

	child = newLocation(l.locType, l)
	l.singles[index] = child
	return child
}

func (l *location) child(partType protoSourceNumber, index int32) *location {
	forType, ok := l.children[partType]
	if !ok {
		forType = make(map[int32]*location)
		l.children[partType] = forType
	}

	child, ok := forType[index]
	if ok {
		return child
	}

	var typeOfChild *locType

	if l.locType != nil {
		protoType, ok := l.locType.fromProto[partType]
		if ok {
			foundChild, ok := l.locType.children[protoType]
			if ok {
				typeOfChild = foundChild
			}
		}
	}

	child = newLocation(typeOfChild, l)
	forType[index] = child
	return child
}

func (l *location) typeChild(tt elementType, index int32) *location {
	protoType := l.locType.children[tt]
	return l.child(protoType.protoDescFieldNum, index)

}

func (l *location) walk(callback func(*location) error) error {
	if l.location != nil {
		if err := callback(l); err != nil {
			return err
		}
	}

	for _, forType := range l.children {
		for idx, child := range forType {
			if err := child.walk(callback); err != nil {
				return fmt.Errorf("ch %d: %w", idx, err)
			}
		}
	}
	return nil
}

func (l *location) moveDown(delta int32) {
	l.startLine += delta
	l.endLine += delta

	if len(l.location.Span) == 4 {
		l.location.Span[0] = l.startLine
		l.location.Span[2] = l.endLine
	} else {
		l.location.Span[0] = l.startLine
	}
}

func (sl *locationSet) Export() *descriptorpb.SourceCodeInfo {
	locations := make([]*descriptorpb.SourceCodeInfo_Location, 0)

	sl.root.walk(func(l *location) error {
		locations = append(locations, l.location)
		return nil
	})

	sort.Sort(byStartLine(locations))

	for _, loc := range locations {
		fmt.Printf("Export %30s -> %v    %s\n", fmt.Sprintf("%v", loc.Path), loc.Span, loc.GetTrailingComments())
	}

	return &descriptorpb.SourceCodeInfo{
		Location: locations,
	}
}

type byStartLine []*descriptorpb.SourceCodeInfo_Location

func (a byStartLine) Len() int      { return len(a) }
func (a byStartLine) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byStartLine) Less(i, j int) bool {
	if a[i].Span[0] != a[j].Span[0] {
		return a[i].Span[0] < a[j].Span[0]
	}
	return a[i].Span[1] < a[j].Span[1]
}

func pathHasPrefix(a, b []int32) ([]int32, bool) {
	if len(a) < len(b) {
		return nil, false
	}
	for i, v := range b {
		if a[i] != v {
			return nil, false
		}
	}
	return a[len(b):], true
}
func equalPaths(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
