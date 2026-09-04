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
	info, _ := debug.ReadBuildInfo()
	buildVersion, buildCommit := buildMetadata(version, commit, info)
	if err := cli.Execute(buildVersion, buildCommit); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

// buildMetadata prefers release flags, falling back to Go's embedded module metadata.
func buildMetadata(version, commit string, info *debug.BuildInfo) (string, string) {
	version = strings.TrimPrefix(version, "v")
	if info != nil {
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
	return version, commit
}
