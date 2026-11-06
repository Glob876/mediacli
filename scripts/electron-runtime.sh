#!/usr/bin/env bash
# Sourced by build.sh and run.sh after changing to the project directory.
provision_electron() {
    if ! command -v npm >/dev/null 2>&1; then
        echo "npm not found in PATH" >&2
        return 1
    fi
    if [ ! -f electron/node_modules/electron/package.json ] || [ ! -f electron/node_modules/typescript/bin/tsc ]; then
        npm ci --prefix electron >> "$LOG_FILE" 2>&1 || return 1
    fi
    local electron_binary
    electron_binary=$(node -e 'try { process.stdout.write(require("./electron/node_modules/electron")); } catch { process.exitCode = 1; }') || electron_binary=""
    if [ -z "$electron_binary" ] || [ ! -f "$electron_binary" ]; then
        node electron/node_modules/electron/install.js >> "$LOG_FILE" 2>&1 || return 1
        electron_binary=$(node -e 'process.stdout.write(require("./electron/node_modules/electron"));') || return 1
    fi
    if [ ! -f "$electron_binary" ]; then
        echo "Electron executable is missing" >&2
        return 1
    fi
}
