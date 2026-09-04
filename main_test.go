package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildMetadata(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version, commit   string
		info                    *debug.BuildInfo
		wantVersion, wantCommit string
	}{
		{name: "development", version: "dev", wantVersion: "dev"},
		{name: "release flags", version: "v1.2.3", commit: "release", info: &debug.BuildInfo{Main: debug.Module{Version: "v9.0.0"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abcdef01234"}}}, wantVersion: "1.2.3", wantCommit: "release"},
		{name: "go install", version: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, wantVersion: "1.2.3"},
		{name: "local build", version: "dev", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abcdef01234"}}}, wantVersion: "dev", wantCommit: "abcdef0"},
		{name: "short revision", version: "dev", info: &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}}, wantVersion: "dev", wantCommit: "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			version, commit := buildMetadata(tc.version, tc.commit, tc.info)
			if version != tc.wantVersion || commit != tc.wantCommit {
				t.Fatalf("buildMetadata() = (%q,%q); want (%q,%q)", version, commit, tc.wantVersion, tc.wantCommit)
			}
		})
	}
}
