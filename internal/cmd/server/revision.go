package server

import (
	"log/slog"
	"runtime/debug"
)

var Revision = "unknown"

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		slog.Error("failed loading build info")
	}

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			Revision = s.Value
			break
		}
	}
}
