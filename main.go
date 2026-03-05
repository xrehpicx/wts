package main

import (
	"os"
	"runtime/debug"

	"github.com/xrehpicx/wts/internal/cli"
)

var (
	version = "0.2.0"
	commit  = ""
)

func main() {
	if commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" && len(s.Value) >= 7 {
					commit = s.Value[:7]
					break
				}
			}
		}
	}
	if commit == "" {
		commit = "unknown"
	}
	if err := cli.Execute(version, commit); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
