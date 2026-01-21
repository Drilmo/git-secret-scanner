package main

import (
	"os"

	"github.com/Drilmo/git-secret-scanner/internal/tui"
	"github.com/charmbracelet/log"
)

func main() {
	// Configure logger
	log.SetLevel(log.DebugLevel)
	log.SetReportTimestamp(false)

	// Run TUI
	if err := tui.Run(); err != nil {
		log.Error("Application error", "err", err)
		os.Exit(1)
	}
}
