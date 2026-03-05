package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/xrehpicx/wts/internal/cli"
)

var (
	version = "dev"
	commit  = ""
)

func main() {
	if info, ok := debug.ReadBuildInfo(); ok {
		// go install sets Main.Version to the module version (e.g. v0.2.1)
		if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
		if commit == "" {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					if len(s.Value) >= 7 {
						commit = s.Value[:7]
					} else {
						commit = s.Value
					}
					break
				}
			}
		}
	}
	if err := cli.Execute(version, commit); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
