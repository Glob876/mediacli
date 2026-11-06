# Rust migration

The CLI, terminal UI, background workers and loopback HTTP daemon are implemented
in Rust. FFmpeg, ffprobe and yt-dlp remain external executables. The desktop shell
remains Electron; its main process, preload and renderer sources are TypeScript.
The existing desktop HTML and CSS are retained.

## Development

```sh
cargo build
cargo run -- --help
cargo run                  # requires an interactive terminal
cargo run -- daemon --port 0
cargo test --locked        # integration tests require FFmpeg and yt-dlp
cargo clippy --locked --all-targets -- -D warnings
cargo fmt --all --check

npm ci --prefix electron
npm run build --prefix electron
MEDIACLI_BIN="$PWD/target/debug/mediacli" npm start --prefix electron
```

`./run.sh` builds and starts the terminal interface.
`./run.sh --electron` (or `./run.sh -e`) builds the release binary and TypeScript
before starting the desktop shell. The terminal launcher does not require npm. `./build.sh` retains the existing installation flow.
Build-only use: `cargo build --release --locked`. The result is
`target/release/mediacli` (`mediacli.exe` on Windows).

## Source layout

| Module | Responsibility |
| --- | --- |
| `src/config.rs`, `resources/` | Settings, schemas and built-in presets |
| `src/storage.rs` | Private atomic files and interprocess update locks |
| `src/media.rs` | Probe, typed operation validation, FFmpeg plans, waveform |
| `src/download.rs` | yt-dlp arguments and external-transcode plans |
| `src/queue.rs` | Workers, multipass ordering, cancellation, retry, recovery |
| `src/cli.rs`, `src/remote.rs` | Commands, projects, batch processing and daemon client |
| `src/daemon.rs` | Loopback HTTP API, SSE and media range requests |
| `src/tui.rs` | Terminal screens, forms, picker, palette and editor |
| `electron/*.ts`, `electron/renderer/app.ts` | Desktop shell and renderer |

A queue item owns its complete command sequence. One item occupies one worker
slot; its passes execute in order and a failed pass prevents subsequent passes.
Separate items can run concurrently, up to `bg_queue_max`.

## Existing data

The platform-specific `mediacli` configuration directory is retained. Existing
`config.json`, `history.json`, `media_presets.json`, editor/pipeline drafts and
project operation envelopes (`type` + `config`) are readable. Before updating an
existing configuration, a private `config.json.bak-*` backup is retained.

The terminal uses `tui_queue.json`; a daemon started with `--persist` uses
`daemon_queue.json`, or `--queue-file`. Legacy Go queue records are read with
their dependency chains and replacement plans. A `*.go-backup.json` copy is
retained before subsequent writes use the Rust journal format. Unfinished jobs
are marked interrupted and require explicit retry. The old application cannot
read the new queue journal; use the backup when rolling back.

Temporary output replacement plans keep the original until FFmpeg has completed
successfully and produced a nonempty file. Retries choose a fresh output name,
and source aliases, including hard links, remain protected. CLI timeouts and
Ctrl+C wait for process cancellation before returning 124 and 130 respectively.
On Unix, each tool is started in a process group; cancellation kills its children
as well. Windows uses `taskkill /T` for the child tree.

## Terminal controls

- Left/right switch categories; up/down and Enter select a tool.
- Ctrl+P or F12 searches actions in Russian or English.
- Forms: up/down or Tab selects a field, Enter opens choices at the current value, left/right cycles known choices,
  Space toggles booleans, Ctrl+U clears, F2 opens the file picker, Ctrl+S applies.
  F5 saves the current download form as a preset; the Presets screen applies,
  renames and deletes saved download/media presets.
- Picker: Enter opens a directory or selects a file; Alt+Enter selects the
  current directory; Backspace opens its parent.
- Queue: C or Ctrl+C cancels, R retries the complete command sequence,
  F10 toggles logs; Esc returns to the menus while work continues.
- History: `/` searches, R repeats an operation, Delete removes a record,
  C clears records after confirmation. Media files are retained.
- Editor: I sets input/output, A adds an operation, Delete removes it,
  Ctrl+Z/Y undoes/redoes, S/L saves/loads a project, P saves a media preset,
  M sets target size, D displays commands, E edits an operation, [ / ] reorder
  operations, V renders a five-second preview, Enter exports. Drafts are autosaved.
- Ranges: arrows move by frame/second, I/O mark bounds, D removes a range,
  G jumps to time, Ctrl+Z/Y undo/redo, Enter exports.

## Packaging and validation

The npm launcher remains a small Node script selecting a bundled Rust binary.
`npm run build` builds the current host target. A target triple can be supplied
with `node scripts/build-npm.cjs TARGET`; install its Rust standard library and
linker first. CI builds six targets on Linux, macOS and Windows runners, then
combines their artifacts into the npm archive. Each binary carries a receipt
with its version, target, source hash and binary hash; packing rejects cached
Go binaries, changed binaries and mixed source revisions. Publishing still runs only on
`main` with the existing publishing configuration.

Local validation covers Linux. macOS and Windows require their CI runners and
runtime testing on those systems; configuration of the matrix alone does not
establish that those builds pass.

## Terminal themes

Settings show the application version at the bottom. Set `use_terminal_palette`
to `true` to use the terminal's configured ANSI colors for any theme, or choose
`theme = terminal`. With `use_terminal_bg = true`, the terminal's own background
is retained. Theme colors are defined in `src/theme.rs`; the shared theme list
is supplied through `/api/meta` for both TUI and Electron.
