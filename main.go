package main

import (
	"os"

	"github.com/xrehpicx/wts/internal/cli"
)

var (
	version = "0.1.0"
	commit  = "unknown"
)

func main() {
	if err := cli.Execute(version, commit); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
