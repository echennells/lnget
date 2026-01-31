//go:build ignore

// gen_man.go generates man pages for lnget using cobra/doc.
// Run with: go run docs/gen_man.go
package main

import (
	"log"
	"os"
	"time"

	"github.com/lightninglabs/lnget/cli"
	"github.com/spf13/cobra/doc"
)

func main() {
	// Create output directory.
	outDir := "docs/man"
	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Get the root command.
	cmd := cli.NewRootCmd()

	// Configure man page header.
	header := &doc.GenManHeader{
		Title:   "LNGET",
		Section: "1",
		Source:  "lnget",
		Manual:  "lnget Manual",
		Date:    timePtr(time.Now()),
	}

	// Generate man pages.
	err = doc.GenManTree(cmd, header, outDir)
	if err != nil {
		log.Fatalf("Failed to generate man pages: %v", err)
	}

	log.Printf("Man pages generated in %s/", outDir)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
