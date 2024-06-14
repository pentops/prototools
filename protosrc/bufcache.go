package protosrc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pentops/log.go/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v2"

	registry_spb "buf.build/gen/go/bufbuild/buf/grpc/go/buf/alpha/registry/v1alpha1/registryv1alpha1grpc"
	registry_pb "buf.build/gen/go/bufbuild/buf/protocolbuffers/go/buf/alpha/registry/v1alpha1"
)

type BufLockFile struct {
	Version string                   `yaml:"version"`
	Deps    []*BufLockFileDependency `yaml:"deps"`
}

type BufLockFileDependency struct {
	Remote     string `yaml:"remote"`
	Owner      string `yaml:"owner"`
	Repository string `yaml:"repository"`
	Commit     string `yaml:"commit"`
	Digest     string `yaml:"digest"`
	Name       string `yaml:"name"`
}

type file struct {
	path    string
	content []byte
}

type BufCache struct {
	root string
}

func NewBufCache() *BufCache {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache")
	specified := os.Getenv("BUF_CACHE_DIR")
	if specified != "" {
		cacheDir = specified
	}
	root := filepath.Join(cacheDir, "buf")
	return &BufCache{root: root}
}

func (bc *BufCache) GetDeps(ctx context.Context, root fs.FS, subDir string) (map[string][]byte, error) {

	var lockFileData []byte
	searchPath := subDir
	for {
		lockFile, err := fs.ReadFile(root, path.Join(searchPath, "buf.lock"))
		if err == nil {
			lockFileData = lockFile
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if searchPath == "." {
			break
		}
		searchPath = filepath.Dir(searchPath)
	}

	if lockFileData == nil {
		return nil, fmt.Errorf("buf.lock not found")
	}

	bufLockFile := &BufLockFile{}
	if err := yaml.Unmarshal(lockFileData, bufLockFile); err != nil {
		return nil, err
	}

	switch bufLockFile.Version {
	case "", "v1":

	case "v2":
		for _, dep := range bufLockFile.Deps {
			parts := strings.Split(dep.Name, "/")
			if len(parts) != 3 {
				return nil, fmt.Errorf("invalid remote %s", dep.Remote)
			}

			if parts[0] != "buf.build" {
				return nil, fmt.Errorf("unsupported remote %s", parts[0])
			}
			dep.Owner = parts[1]
			dep.Repository = parts[2]
		}

	default:
		return nil, fmt.Errorf("unsupported buf.lock version %s", bufLockFile.Version)

	}

	bufClient, err := grpc.NewClient("buf.build:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return nil, err
	}
	registryClient := registry_spb.NewDownloadServiceClient(bufClient)

	externalFiles := map[string][]byte{}
	for _, dep := range bufLockFile.Deps {
		cached, err := bc.tryDep(ctx, dep)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			for _, file := range cached {
				if _, ok := externalFiles[file.path]; ok {
					return nil, fmt.Errorf("duplicate file %s", file.path)
				}
				externalFiles[file.path] = file.content
			}
			continue
		}

		downloadRes, err := registryClient.Download(ctx, &registry_pb.DownloadRequest{
			Owner:      dep.Owner,
			Repository: dep.Repository,
			Reference:  dep.Commit,
		})
		if err != nil {
			return nil, err
		}

		for _, file := range downloadRes.Module.Files {
			if _, ok := externalFiles[file.Path]; ok {
				return nil, fmt.Errorf("duplicate file %s", file.Path)
			}

			externalFiles[file.Path] = file.Content
		}
	}

	return externalFiles, nil

}

func (bc *BufCache) tryDep(ctx context.Context, dep *BufLockFileDependency) ([]file, error) {
	ctx = log.WithFields(ctx, map[string]interface{}{
		"owner":      dep.Owner,
		"repository": dep.Repository,
		"commit":     dep.Commit,
	})

	v3Dep := filepath.Join(bc.root, "v3", "modules", "shake256", "buf.build", dep.Owner, dep.Repository, dep.Commit, "files")

	if _, err := os.Stat(v3Dep); err == nil {
		log.WithField(ctx, "v3Path", v3Dep).Debug("found v3 dep")
		files := make([]file, 0)
		err := filepath.Walk(v3Dep, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if !strings.HasSuffix(path, ".proto") {
				return nil
			}

			rel, err := filepath.Rel(v3Dep, path)
			if err != nil {
				return err
			}

			files = append(files, file{path: rel, content: content})

			return nil
		})
		return files, err
	}

	log.WithField(ctx, "v3Path", v3Dep).Debug("No v3 found, falling back to v2")

	contentStr := dep.Digest
	hdr, rem := contentStr[9:11], contentStr[11:]

	indexPath := filepath.Join("v2", "module", "buf.build", bc.root, dep.Owner, dep.Repository, "blobs", hdr, rem)
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.WithField(ctx, "mod", indexPath).Warn("buf mod not found")
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(string(indexContent), "\n")
	files := make([]file, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		header, fDir, fPath, filename := line[:8], line[9:11], line[11:137], line[139:]

		if header != "shake256" {
			return nil, fmt.Errorf("invalid cache entry")
		}

		if !strings.HasSuffix(filename, ".proto") {
			continue
		}

		fileContent, err := os.ReadFile(filepath.Join(bc.root, dep.Owner, dep.Repository, "blobs", fDir, fPath))
		if err != nil {
			return nil, err
		}

		files = append(files, file{path: filename, content: fileContent})
	}

	return files, nil
}
