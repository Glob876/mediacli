package main

import (
	"flag"
	"fmt"
	"mediacli/pkg/gui"
	"mediacli/pkg/ui"
	"os"

	"golang.org/x/term"
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

	// Быстрая проверка TTY до инициализации tcell — иначе tcell.NewScreen().Init()
	// блокируется или падает с неочевидным "open /dev/tty: no such device"
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "MediaCLI error: no interactive terminal detected (stdin/stdout is not a TTY).")
		fmt.Fprintln(os.Stderr, "  • Запусти mediacli напрямую в терминале (не через пайп/редирект IDE).")
		fmt.Fprintln(os.Stderr, "  • Для GUI режима: go run . -- --gui  или  ./mediacli --gui")
		fmt.Fprintln(os.Stderr, "  • Совет: первая сборка с fyne может занять 30-60с — это нормально (кеш Go).")
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "[MediaCLI] Initializing TUI...")

	if err := ui.RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "MediaCLI error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: проверь $TERM (должен быть xterm-256color / xterm-ghostty) и что запускаешь в реальном терминале.")
		os.Exit(1)
	}
}