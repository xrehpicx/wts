package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra/doc"

	"github.com/xrehpicx/wts/internal/cli"
)

func main() {
	manDir := flag.String("man-dir", filepath.Join("docs", "man"), "output directory for man pages")
	mdDir := flag.String("md-dir", filepath.Join("docs", "cli"), "output directory for markdown command docs")
	version := flag.String("version", "dev", "version string to embed in docs")
	flag.Parse()

	root := cli.NewRootCmd(*version, "dev")
	root.DisableAutoGenTag = true

	if err := os.MkdirAll(*manDir, 0o755); err != nil {
		panic(fmt.Errorf("create man dir: %w", err))
	}
	if err := os.MkdirAll(*mdDir, 0o755); err != nil {
		panic(fmt.Errorf("create markdown dir: %w", err))
	}

	header := &doc.GenManHeader{
		Title:   "WORKSWITCH",
		Section: "1",
		Source:  "workswitch",
		Manual:  "workswitch Manual",
	}

	if err := doc.GenManTree(root, header, *manDir); err != nil {
		panic(fmt.Errorf("generate man pages: %w", err))
	}
	if err := doc.GenMarkdownTree(root, *mdDir); err != nil {
		panic(fmt.Errorf("generate markdown docs: %w", err))
	}
}
