package optionreflect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestParse(t *testing.T) {

	_ = &emptypb.Empty{}
	_ = &annotations.HttpRule{}
	// forces import, the parser assumes this exists.

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
		}},

		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("TestService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("PostMethod"),
				InputType:  proto.String(".test.v1.Test"),
				OutputType: proto.String("google.protobuf.Empty"),
				Options:    &descriptorpb.MethodOptions{},
			}, {

				Name:       proto.String("GetMethod"),
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
		Pattern: &annotations.HttpRule_Post{
			Post: "/test/v1/foo",
		},
		Body: "*",
	})
	proto.SetExtension(input.Service[0].Method[1].Options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Get{
			Get: "/test/v1/foo",
		},
	})

	testFile, err := protodesc.NewFile(input, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatal(err)
	}

	ob := NewBuilder(nil)

	{
		opts, err := ob.OptionsFor(testFile.Services().ByName("TestService").Methods().ByName("PostMethod"))
		if err != nil {
			t.Fatal(err)
		}

		if len(opts) != 1 {
			t.Fatalf("expected 1 option, got %d", len(opts))
		}
		opt := opts[0]

		assert.Equal(t, "google.api.http", string(opt.Desc.FullName()))
		assert.Len(t, opt.SubPath, 0)
		assert.Equal(t, "(google.api.http)", opt.FullType())

		root := WalkOptionField(opt.Desc, opt.Value)

		assert.Equal(t, FieldTypeMessage, root.FieldType)
		assert.Len(t, root.Children, 2)
		assert.Equal(t, "post", root.Children[0].Key)
		assert.Equal(t, `"/test/v1/foo"`, root.Children[0].ScalarValue)
		assert.Equal(t, "body", root.Children[1].Key)
		assert.Equal(t, `"*"`, root.Children[1].ScalarValue)
	}

	{
		opts, err := ob.OptionsFor(testFile.Services().ByName("TestService").Methods().ByName("GetMethod"))
		if err != nil {
			t.Fatal(err)
		}

		if len(opts) != 1 {
			t.Fatalf("expected 1 option, got %d", len(opts))
		}
		opt := opts[0]

		opt.Simplify(5)

		assert.Equal(t, "google.api.http", string(opt.RootType.FullName()))
		assert.Len(t, opt.SubPath, 1)
		assert.Equal(t, "(google.api.http).get", opt.FullType())
		assert.Equal(t, "/test/v1/foo", opt.Value.String())

	}

}
