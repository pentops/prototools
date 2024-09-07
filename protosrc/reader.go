package protosrc

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ParsedSource struct {
	Files        []*descriptorpb.FileDescriptorProto
	Dependencies []*descriptorpb.FileDescriptorProto
}

func ReadImageFromSourceDir(ctx context.Context, rootFS fs.FS, subPath string) ([]protoreflect.FileDescriptor, error) {

	walkRoot, err := fs.Sub(rootFS, subPath)
	if err != nil {
		return nil, err
	}

	filenames := []string{}
	filenameMap := map[string]struct{}{}
	err = fs.WalkDir(walkRoot, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))

		switch ext {
		case ".proto":

			filenames = append(filenames, path)
			filenameMap[path] = struct{}{}
			return nil
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	bufCache := NewBufCache()
	extFiles, err := bufCache.GetDeps(ctx, rootFS, subPath)
	if err != nil {
		return nil, err
	}

	resolver := protocompile.ResolverFunc(func(filename string) (protocompile.SearchResult, error) {
		if content, ok := extFiles[filename]; ok {
			return protocompile.SearchResult{
				Source: bytes.NewReader(content),
			}, nil
		}
		// unclear if Source gets closed, so just parse to memory.
		file, err := fs.ReadFile(walkRoot, filename)
		if err != nil {
			return protocompile.SearchResult{}, err
		}
		return protocompile.SearchResult{
			Source: bytes.NewReader(file),
		}, nil
	})

	compiler := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(resolver),
		SourceInfoMode: protocompile.SourceInfoExtraComments,
	}

	desc, err := compiler.Compile(ctx, filenames...)
	if err != nil {
		return nil, err
	}

	descriptors := make([]protoreflect.FileDescriptor, len(desc))
	for i, d := range desc {
		descriptors[i] = d
	}

	return descriptors, nil
}
