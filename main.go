package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"mediacli/pkg/cli"
	"mediacli/pkg/core"
	"mediacli/pkg/daemon"
	"mediacli/pkg/ui"

	"golang.org/x/term"
)

func main() {
	// Подкоманда для Electron-шелла: `mediacli daemon --port 0`.
	// Перехватываем до flag.Parse, т.к. у daemon свой FlagSet.
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		os.Exit(daemon.Run(os.Args[2:]))
	}

	if len(os.Args) > 1 && os.Args[1] != "gui" && os.Args[1] != "--gui" && os.Args[1] != "-gui" && os.Args[1] != "-g" {
		os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}

	var isGUI bool
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print application version")
	flag.BoolVar(&showVersion, "v", false, "Print application version")
	flag.BoolVar(&isGUI, "gui", false, "Launch graphical user interface")
	flag.BoolVar(&isGUI, "g", false, "Launch graphical user interface (shorthand)")
	flag.Parse()
	if showVersion {
		fmt.Println(core.Version)
		return
	}

	// Совместимость: `mediacli gui` (позиционный аргумент).
	if !isGUI {
		for _, arg := range flag.Args() {
			if arg == "gui" {
				isGUI = true
				break
			}
		}
	}

	if isGUI {
		fmt.Println("[MediaCLI] Fyne GUI удалён. Новый интерфейс — Electron:")
		fmt.Println("  • ./build.sh --startelectron")
		fmt.Println("  • или: cd electron && npm install && npm start")
		fmt.Println("  • TUI без изменений: ./mediacli (в терминале)")
		return
	}

	// Быстрая проверка TTY до инициализации tcell — иначе tcell.NewScreen().Init()
	// блокируется или падает с неочевидным "open /dev/tty: no such device"
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, "MediaCLI error: no interactive terminal detected (stdin/stdout is not a TTY).")
		fmt.Fprintln(os.Stderr, "  • Запусти mediacli напрямую в терминале (не через пайп/редирект IDE).")
		fmt.Fprintln(os.Stderr, "  • Для Electron-интерфейса: ./build.sh --startelectron")
		fmt.Fprintln(os.Stderr, "  • Для TUI: запускай в реальном терминале.")
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "[MediaCLI] Initializing TUI...")

	// Применяем лимит фоновых задач из конфига до старта UI.
	if cfg, err := core.LoadConfig(); err == nil {
		core.GlobalQueue.SyncMaxTasksFromConfig(cfg)
	}

	if err := core.GlobalQueue.EnablePersistence(filepath.Join(core.GetConfigDir(), "tui_queue.json")); err != nil {
		fmt.Fprintf(os.Stderr, "Queue recovery: %v\n", err)
	}

	if err := ui.RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "MediaCLI error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Hint: проверь $TERM (должен быть xterm-256color / xterm-ghostty) и что запускаешь в реальном терминале.")
		os.Exit(1)
	}
}
