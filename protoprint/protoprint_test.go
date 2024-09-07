package protoprint

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pentops/prototools/protosrc"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestSimplePrint(t *testing.T) {

	input := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("test.v1"),
		Dependency: []string{
			"google/protobuf/empty.proto",
		},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("go_package_value"),
		},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Test"),

			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:   proto.String("f1"),
				Number: proto.Int32(1),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			}, {
				Name:           proto.String("f2"),
				Number:         proto.Int32(2),
				Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:           descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Proto3Optional: proto.Bool(true),
			}, {
				Name:   proto.String("f3"),
				Number: proto.Int32(3),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			}, {
				Name:     proto.String("f4"),
				Number:   proto.Int32(4),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				TypeName: proto.String(".google.protobuf.Empty"),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			}, {
				Name:     proto.String("f5"),
				Number:   proto.Int32(5),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".test.v1.Test.F5Entry"),
			}, {
				Name:     proto.String("f6"),
				Number:   proto.Int32(6),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				TypeName: proto.String(".test.v1.Foo"),
			}},

			NestedType: []*descriptorpb.DescriptorProto{{
				Name: proto.String("F5Entry"),
				Field: []*descriptorpb.FieldDescriptorProto{{
					Name:   proto.String("key"),
					Number: proto.Int32(1),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				}, {
					Name:   proto.String("value"),
					Number: proto.Int32(2),
					Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				}},
				Options: &descriptorpb.MessageOptions{
					MapEntry: proto.Bool(true),
				},
			}},
		}},

		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("TestService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("GetMethod"),
				InputType:  proto.String(".test.v1.Test"),
				OutputType: proto.String("google.protobuf.Empty"),
				Options:    &descriptorpb.MethodOptions{},
			}, {
				Name:       proto.String("PostMethod"),
				InputType:  proto.String(".test.v1.Test"),
				OutputType: proto.String("google.protobuf.Empty"),
				Options:    &descriptorpb.MethodOptions{},
			}},
		}},

		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Foo"),
			Value: []*descriptorpb.EnumValueDescriptorProto{{
				Name:   proto.String("VAL_0"),
				Number: proto.Int32(0),
			}, {
				Name:   proto.String("VAL_1"),
				Number: proto.Int32(1),
			}},
		}},
	}

	proto.SetExtension(input.Service[0].Method[0].Options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{
			Get: "/test/v1/foo",
		},
	})
	proto.SetExtension(input.Service[0].Method[1].Options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Post{
			Post: "/test/v1/foo",
		},
		Body: "*",
		AdditionalBindings: []*annotations.HttpRule{{
			Pattern: &annotations.HttpRule_Post{
				Post: "/test/v1/foo/2",
			},
		}, {
			Pattern: &annotations.HttpRule_Post{
				Post: "/test/v1/foo/3",
			},
		}},
	})

	testFile, err := protodesc.NewFile(input, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}

	output, err := printFile(testFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(prototext.Format(input))

	expected := []string{
		`syntax = "proto3";`,
		"",
		`package test.v1;`,
		"",
		`import "google/protobuf/empty.proto";`,
		``,
		`option go_package = "go_package_value";`,
		"",
		`service TestService {`,
		`  rpc GetMethod(Test) returns (google.protobuf.Empty) {`,
		`    option (google.api.http) = {get: "/test/v1/foo"};`,
		`  }`,
		``,
		`  rpc PostMethod(Test) returns (google.protobuf.Empty) {`,
		`    option (google.api.http) = {`,
		`      post: "/test/v1/foo"`,
		`      body: "*"`,
		`      additional_bindings: [{`,
		`        post: "/test/v1/foo/2"`,
		`      }, {`,
		`        post: "/test/v1/foo/3"`,
		`      }]`,
		`    };`,
		`  }`,
		`}`,
		"",
		`message Test {`,
		`  string f1 = 1;`,
		`  optional string f2 = 2;`,
		`  repeated string f3 = 3;`,
		`  google.protobuf.Empty f4 = 4;`,
		`  map<string, string> f5 = 5;`,
		`  Foo f6 = 6;`,
		`}`,
		"",
		`enum Foo {`,
		`  VAL_0 = 0;`,
		`  VAL_1 = 1;`,
		`}`,
		"",
	}

	outputLines := strings.Split(string(output), "\n")
	assertEqualLines(t, expected, outputLines)
}

func assertEqualLines(t *testing.T, wantLines, gotLines []string) {

	for idx, line := range gotLines {
		t.Logf("got %03d: '%s'", idx, line)
		if idx >= len(wantLines) {
			t.Errorf("   EXTRA")
			continue
		}
		if wantLines[idx] != line {
			t.Errorf("   want: '%s'", wantLines[idx])
		}
	}

	for idx := len(gotLines); idx < len(wantLines); idx++ {
		t.Errorf("mis %03d: '%s'", idx, wantLines[idx])
	}
}

type fileMap map[string][]byte

func NewFileMap() fileMap {
	return make(fileMap)
}

func (fm fileMap) GetFile(filename string) ([]byte, error) {
	if b, ok := fm[filename]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

func (fm fileMap) PutFile(ctx context.Context, filename string, content []byte) error {
	fm[filename] = content
	return nil
}

func TestExampleReal(t *testing.T) {
	rootDir := os.DirFS("../")
	descriptors, err := protosrc.ReadImageFromSourceDir(context.Background(), rootDir, "proto/test")
	if err != nil {
		t.Fatal(err)
	}

	outputMap := NewFileMap()

	err = PrintReflect(context.Background(), outputMap, descriptors, Options{
		OnlyFilenames: []string{"test/foo/v1/test.proto"},
	})

	if err != nil {
		t.Fatal(err)
	}

	output, err := outputMap.GetFile("test/foo/v1/test.proto")
	if err != nil {
		t.Fatal(err)
	}

	realFile, err := os.ReadFile("../proto/test/test/foo/v1/test.proto")
	if err != nil {
		t.Fatal(err)
	}

	realLines := strings.Split(string(realFile), "\n")
	gotLines := strings.Split(string(output), "\n")

	assertEqualLines(t, realLines, gotLines)

}
