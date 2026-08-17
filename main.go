package main

import (
	"fmt"
	"mediacli/pkg/ui"
	"os"
)

func main() {
	if err := ui.RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "MediaCLI error: %v\n", err)
		os.Exit(1)
	}
}