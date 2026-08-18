package main

import (
	"flag"
	"fmt"
	"mediacli/pkg/gui"
	"mediacli/pkg/ui"
	"os"
)

func main() {
	var isGUI bool

	for _, arg := range os.Args[1:] {
		if arg == "--gui" || arg == "-g" || arg == "gui" {
			isGUI = true
			break
		}
	}

	guiFlag := flag.Bool("gui", false, "Launch graphical user interface")
	flag.BoolVar(guiFlag, "g", false, "Launch graphical user interface (shorthand)")
	flag.Parse()

	if isGUI || *guiFlag {
		fmt.Println("[MediaCLI] Starting graphical desktop interface...")
		gui.RunGUI()
		return
	}

	if err := ui.RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "MediaCLI error: %v\n", err)
		os.Exit(1)
	}
}