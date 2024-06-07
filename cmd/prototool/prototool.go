package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pentops/prototools/protoprint"
	"github.com/pentops/prototools/protosrc"
	"github.com/pentops/runner/commander"
)

var Version = "dev"

func main() {
	cmdGroup := commander.NewCommandSet()

	cmdGroup.Add("fmt", commander.NewCommand(runFmt))
	cmdGroup.RunMain("prototool", Version)
}

func runFmt(ctx context.Context, cfg struct {
	SourceDir   string   `flag:"dir" default:"."`
	IgnoreFiles []string `flag:"ignore-files" optional:"true"`
	OnlyFiles   []string `flag:"only-files" optional:"true"`
}) error {
	img, err := protosrc.ReadImageFromSourceDir(ctx, cfg.SourceDir)
	if err != nil {
		return fmt.Errorf("reading source %s: %w", cfg.SourceDir, err)
	}

	fsOutput := NewLocalOutput(cfg.SourceDir)

	err = protoprint.PrintProtoFiles(ctx, fsOutput, img, protoprint.Options{
		IgnoreFilenames: cfg.IgnoreFiles,
		OnlyFilenames:   cfg.OnlyFiles,
	})
	if err != nil {
		return fmt.Errorf("printing: %w", err)
	}
	return nil
}

type LocalOutput struct {
	root string
}

func NewLocalOutput(root string) *LocalOutput {
	return &LocalOutput{root: root}
}

func (lo *LocalOutput) PutFile(ctx context.Context, path string, data []byte) error {
	fmt.Printf("writing %s\n", path)
	return os.WriteFile(filepath.Join(lo.root, path), data, 0666)
}
