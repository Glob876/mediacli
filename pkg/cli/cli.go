// Package cli provides non-interactive commands over the shared MediaCLI core.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mediacli/pkg/core"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type exitError struct {
	code    int
	message string
}

func (err exitError) Error() string { return err.message }
func usage(message string) error    { return exitError{2, message} }

type app struct {
	ctx           context.Context
	input         io.Reader
	out, errOut   io.Writer
	cfg           core.Config
	json          bool
	dry           bool
	background    bool
	server, token string
	timeout       time.Duration
}

type stringList []string

func (list *stringList) String() string         { return strings.Join(*list, ",") }
func (list *stringList) Set(value string) error { *list = append(*list, value); return nil }

func Run(args []string, input io.Reader, out, errOut io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunContext(ctx, args, input, out, errOut)
}

func RunContext(ctx context.Context, args []string, input io.Reader, out, errOut io.Writer) int {
	cli := &app{ctx: ctx, input: input, out: out, errOut: errOut}
	err := cli.run(args)
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(errOut, "mediacli:", err)
	if errors.Is(err, context.DeadlineExceeded) {
		return 124
	}
	if ctx.Err() != nil {
		return 130
	}
	var status exitError
	if errors.As(err, &status) {
		return status.code
	}
	return 1
}

func (cli *app) emit(value any) error {
	if text, ok := value.(string); ok && !cli.json {
		_, err := fmt.Fprintln(cli.out, text)
		return err
	}
	encoder := json.NewEncoder(cli.out)
	if !cli.json {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func (cli *app) flags(name string) *flag.FlagSet {
	flags := flag.NewFlagSet("mediacli "+name, flag.ContinueOnError)
	flags.SetOutput(cli.out)
	flags.BoolVar(&cli.json, "json", false, "Output JSON; tool logs always go to stderr")
	flags.BoolVar(&cli.dry, "dry-run", false, "Show the plan without downloading, encoding or queueing")
	flags.BoolVar(&cli.background, "background", false, "Submit to --server and return task IDs immediately")
	flags.StringVar(&cli.server, "server", os.Getenv("MEDIACLI_SERVER"), "Daemon URL, e.g. http://127.0.0.1:8181 (or MEDIACLI_SERVER)")
	flags.StringVar(&cli.token, "token", os.Getenv("MEDIACLI_TOKEN"), "Daemon bearer token (prefer MEDIACLI_TOKEN)")
	flags.DurationVar(&cli.timeout, "timeout", 0, "Command time limit, e.g. 10m (0 = no limit)")
	flags.Usage = func() { fmt.Fprintf(cli.out, "Usage: mediacli %s [flags] [arguments]\n", name); flags.PrintDefaults() }
	return flags
}

// The standard flag package stops at a positional argument; accept flags on either side.
func parse(flags *flag.FlagSet, args []string) error {
	var options, positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if !strings.HasPrefix(argument, "-") || argument == "-" {
			positionals = append(positionals, argument)
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, inline := strings.Cut(name, "=")
		if name == "h" || name == "help" {
			flags.Usage()
			return flag.ErrHelp
		}
		option := flags.Lookup(name)
		if option == nil {
			return usage("unknown flag: " + argument)
		}
		options = append(options, argument)
		boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
		if !inline && !(ok && boolean.IsBoolFlag()) {
			index++
			if index >= len(args) {
				return usage("missing value for " + argument)
			}
			options = append(options, args[index])
		}
	}
	options = append(options, "--")
	options = append(options, positionals...)
	if err := flags.Parse(options); err != nil {
		return usage(err.Error())
	}
	return nil
}

func (cli *app) setup() (context.CancelFunc, error) {
	if cli.background && cli.server == "" {
		return nil, usage("--background requires --server (start mediacli daemon --port 8181)")
	}
	if cli.timeout < 0 {
		return nil, usage("--timeout cannot be negative")
	}
	if cli.timeout > 0 {
		ctx, cancel := context.WithTimeout(cli.ctx, cli.timeout)
		cli.ctx = ctx
		return cancel, nil
	}
	return func() {}, nil
}

func (cli *app) run(args []string) error {
	if len(args) == 0 {
		return usage("specify a command; use mediacli --help")
	}
	command, rest := args[0], args[1:]
	if command == "help" && len(rest) > 0 {
		return cli.run([]string{rest[0], "--help"})
	}
	switch command {
	case "help", "--help", "-h":
		return cli.emit(help)
	case "version", "--version", "-v":
		flags := cli.flags("version")
		if err := parse(flags, rest); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return usage("version takes no positional arguments")
		}
		if cli.json {
			return cli.emit(map[string]string{"version": core.Version})
		}
		return cli.emit(core.Version)
	}
	cfg, err := core.LoadConfig()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	cli.cfg = cfg
	switch command {
	case "download", "video", "thumbnail":
		return cli.download(command, rest)
	case "probe", "inspect", "waveform":
		return cli.inspect(command, rest)
	case "pipeline", "editor", "convert", "transcode", "trim", "aspect", "mux", "demux", "concat", "filter", "audio", "subtitle", "metadata", "scale", "fps", "stabilize", "size", "ranges", "batch":
		return cli.media(command, rest)
	case "config", "presets", "history", "doctor", "library", "cookies", "queue":
		return cli.admin(command, rest)
	default:
		return usage("unknown command " + command + "; use mediacli --help")
	}
}

func (cli *app) read(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(io.LimitReader(cli.input, 16<<20))
	}
	return os.ReadFile(core.ParseUserPath(path))
}

const help = `MediaCLI — terminal media tools
Usage: mediacli COMMAND [flags] [arguments]
       mediacli                          launch TUI

Download:
  download URL... [--audio-only] [--playlist] [--preset ID]
  video URL...                          alias for download
  thumbnail URL... [--format png|jpg|webp]

Inspect and edit:
  probe FILE                            full metadata and streams (inspect alias)
  convert FILE [-o OUTPUT] [--preset ID] [--vcodec libx264 --crf 20]
  trim FILE --start 00:00:10 --end 00:00:20 [--mode copy]
  aspect FILE --ratio 9:16 --mode blur
  mux FILE --extra-inputs AUDIO [--shortest]
  demux FILE --stream-type audio --stream-index 0 -o audio.m4a
  concat FILE... -o OUTPUT [--mode filter|demuxer]
  audio FILE [--volume 0.5 --normalize --tempo 1.2]
  filter FILE --vf hflip --af volume=0.8
  subtitle FILE --mode burn --subtitle-file captions.srt
  metadata FILE --title TITLE --artist ARTIST
  scale FILE --width 1280 --height 720
  fps FILE --fps 30
  stabilize FILE [--shakiness 10 --accuracy 15]
  size FILE --size-mb 50 [--two-pass]
  ranges FILE --range 0..5 --range 10..15 [--join]
  waveform FILE [--start 0 --columns 80]

Projects and batch:
  pipeline --project FILE.json [--input FILE] [--op 'trim={"start":"0","end":"5"}']
  editor                                alias for pipeline; --project editor loads Alt+3 draft
  pipeline FILE --op 'scale={"width":1280}' --save-project project.json
  pipeline --project project.json --preview 5 -o preview.mp4
  batch --directory DIR --preset ID [--output-dir DIR] [--skip-invalid]

Manage:
  config get [KEY] | set KEY VALUE | reset | import FILE | export FILE | options
  presets list|show|save|delete [ID] [--kind media|download|convert|video] [--file FILE]
  history list [--search TEXT] | show INDEX | retry INDEX | clear | delete INDEX
  library [DIRECTORY]
  doctor                                check dependencies
  cookies list | add DOMAIN NAME VALUE [--file FILE]
  queue list|show ID|logs ID|cancel ID|retry ID|wait ID|cancel-group GROUP --server URL

Shared options:
  --help                 command-specific flags, including all operation parameters
  --json                 JSON on stdout; progress/logs go to stderr
  --dry-run              print a masked command plan without executing it
  --server URL           use a running daemon (MEDIACLI_SERVER)
  --token TOKEN          bearer token (prefer MEDIACLI_TOKEN)
  --background           submit work to --server and return immediately
  --timeout 10m          stop waiting/running after a time limit

Run 'mediacli daemon --port 8181' to maintain a background queue.
Operation options accept --options JSON and every JSON field as a flag.
Flags may precede or follow positional arguments. Use -- for paths starting with '-'.
Exit codes: 0 success, 1 operation failure, 2 invalid arguments, 124 timed out, 130 interrupted.`

func optionalString(flags *flag.FlagSet, enabled bool, name, initial, description string) *string {
	value := new(string)
	*value = initial
	if enabled {
		flags.StringVar(value, name, initial, description)
	}
	return value
}
func optionalBool(flags *flag.FlagSet, enabled bool, name string, initial bool, description string) *bool {
	value := new(bool)
	*value = initial
	if enabled {
		flags.BoolVar(value, name, initial, description)
	}
	return value
}
func optionalFloat(flags *flag.FlagSet, enabled bool, name string, initial float64, description string) *float64 {
	value := new(float64)
	*value = initial
	if enabled {
		flags.Float64Var(value, name, initial, description)
	}
	return value
}
