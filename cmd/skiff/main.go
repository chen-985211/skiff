package main

import (
	"os"

	"skiff/internal/platform/desktop"
	"skiff/internal/platform/diagnostics"
)

func main() {
	logger := diagnostics.Stderr()
	run := logger.Start("desktop", "run")
	if err := desktop.Run(logger); err != nil {
		logger.Unexpected(run, err)
		os.Exit(1)
	}
}
