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
	cmdGroup.Add("nop", commander.NewCommand(runNop))
	cmdGroup.RunMain("prototool", Version)
}

func runNop(ctx context.Context, cfg struct {
	SourceDir string `flag:"dir" default:"."`
	SubDir    string `flag:"subdir" default:"."`
}) error {
	rootFS := os.DirFS(cfg.SourceDir)
	_, err := protosrc.ReadImageFromSourceDir(ctx, rootFS, cfg.SubDir)
	if err != nil {
		return fmt.Errorf("reading source %s: %w", cfg.SourceDir, err)
	}

	return nil
}

func runFmt(ctx context.Context, cfg struct {
	SourceDir string `flag:"dir" default:"."`
	SubDir    string `flag:"subdir" default:"."`
}) error {
	rootFS := os.DirFS(cfg.SourceDir)
	img, err := protosrc.ReadImageFromSourceDir(ctx, rootFS, cfg.SubDir)
	if err != nil {
		return fmt.Errorf("reading source %s: %w", cfg.SourceDir, err)
	}

	fsOutput := NewLocalOutput(cfg.SourceDir)

	err = protoprint.PrintProtoFiles(ctx, fsOutput, img, protoprint.Options{})
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
