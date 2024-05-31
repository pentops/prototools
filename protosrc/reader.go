package protosrc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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

func ReadImageFromSourceDir(ctx context.Context, src string) (*ParsedSource, error) {
	fileStat, err := os.Lstat(src)
	if err != nil {
		return nil, err
	}
	if !fileStat.IsDir() {
		return nil, fmt.Errorf("src must be a directory")
	}

	buf := NewBufCache()

	extFiles, err := buf.GetDeps(ctx, src)
	if err != nil {
		return nil, err
	}

	filenames := []string{}
	filenameMap := map[string]struct{}{}
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		switch ext {
		case ".proto":
			filenames = append(filenames, rel)
			filenameMap[rel] = struct{}{}
			return nil
		}

		return nil
	})
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
			return os.Open(filepath.Join(src, filename))
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
