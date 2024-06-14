package protosrc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ParsedSource struct {
	Files        []*descriptorpb.FileDescriptorProto
	Dependencies []*descriptorpb.FileDescriptorProto
}

func ReadImageFromSourceDir(ctx context.Context, rootFS fs.FS, subPath string) (*ParsedSource, error) {

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

	parser := protoparse.Parser{
		ImportPaths:           []string{""},
		IncludeSourceCodeInfo: true,
		WarningReporter: func(err reporter.ErrorWithPos) {
			fmt.Printf("WRAN: %s", err)
		},

		Accessor: func(filename string) (io.ReadCloser, error) {
			if content, ok := extFiles[filename]; ok {
				return io.NopCloser(bytes.NewReader(content)), nil
			}
			return walkRoot.Open(filename)
		},
	}

	customDesc, err := parser.ParseFiles(filenames...)
	if err != nil {
		return nil, fmt.Errorf("protoparse: %w", err)
	}

	fds := desc.ToFileDescriptorSet(customDesc...)

	out := &ParsedSource{}

	for _, fd := range fds.File {
		if _, ok := filenameMap[fd.GetName()]; ok {
			out.Files = append(out.Files, fd)
		} else {
			out.Dependencies = append(out.Dependencies, fd)
		}
	}

	return out, nil
}
