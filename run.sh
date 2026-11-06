#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    echo "Usage: ./run.sh [MediaCLI arguments]"
    echo "       ./run.sh --electron | -e [Electron arguments]"
    echo "Without arguments, builds and starts the terminal interface."
    exit 0
fi

START_ELECTRON=0
if [ "${1:-}" = "--electron" ] || [ "${1:-}" = "-e" ]; then
    START_ELECTRON=1
    shift
fi

mkdir -p dist
LOG_FILE="$PROJECT_DIR/dist/run.log"
: > "$LOG_FILE"
trap 'echo "Launch failed. See $LOG_FILE" >&2' ERR

command -v cargo >/dev/null 2>&1 || { echo "cargo not found in PATH" >&2; exit 1; }
if [ "$START_ELECTRON" -eq 1 ]; then
    command -v npm >/dev/null 2>&1 || { echo "npm not found in PATH" >&2; exit 1; }
fi

BIN_NAME=mediacli
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) BIN_NAME=mediacli.exe ;;
esac
TARGET_PATH="$PROJECT_DIR/dist/$BIN_NAME"

# Cargo tracks source changes, compiler options and dependency changes itself.
echo "Building MediaCLI..."
cargo build --release --locked >> "$LOG_FILE" 2>&1
cp "target/release/$BIN_NAME" "$TARGET_PATH"

if [ "$START_ELECTRON" -eq 0 ]; then
    exec "$TARGET_PATH" "$@"
fi

source "$PROJECT_DIR/scripts/electron-runtime.sh"
echo "Preparing Electron..."
provision_electron
export MEDIACLI_BIN="$TARGET_PATH"
echo "Starting MediaGUI..."
exec npm start --prefix electron -- "$@"
