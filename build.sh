#!/usr/bin/env bash

set -euo pipefail

DIST_DIR="dist"
LOG_FILE="$DIST_DIR/build.log"

mkdir -p "$DIST_DIR"
: > "$LOG_FILE"

LDFLAGS="-s -w"

# Автоматическое определение текущей системы
HOST_OS=$(go env GOOS)
HOST_ARCH=$(go env GOARCH)

BIN_NAME="mediacli"
if [ "$HOST_OS" = "windows" ]; then
    BIN_NAME="mediacli.exe"
fi

TARGET_PATH="$DIST_DIR/$BIN_NAME"

START_TOTAL=$(date +%s)

echo "================================================================"
echo "MediaCLI Native Build & Install Pipeline"
echo "================================================================"
echo "Target OS : $HOST_OS ($HOST_ARCH)"
echo "Go Version: $(go version | awk '{print $3, $4}')"
echo "Binary    : $TARGET_PATH"
echo "Log File  : $LOG_FILE"
echo "================================================================"

# -----------------------------------------------------------------------------
# ЭТАП 1: Компиляция нативного бинарника
# -----------------------------------------------------------------------------
printf "[1/2] Compiling native binary for %s/%s... " "$HOST_OS" "$HOST_ARCH"

START_BUILD=$(date +%s)

if CGO_ENABLED=0 GOOS="$HOST_OS" GOARCH="$HOST_ARCH" go build \
    -trimpath \
    -ldflags="$LDFLAGS" \
    -o "$TARGET_PATH" \
    . >> "$LOG_FILE" 2>&1; then
    
    END_BUILD=$(date +%s)
    BUILD_DURATION=$((END_BUILD - START_BUILD))
    FILE_SIZE=$(ls -lh "$TARGET_PATH" | awk '{print $5}')
    printf "[OK] (%ds, %s)\n" "$BUILD_DURATION" "$FILE_SIZE"
else
    printf "[FAIL]\n"
    echo "Compilation failed. Log details:"
    tail -n 25 "$LOG_FILE"
    exit 1
fi

# -----------------------------------------------------------------------------
# ЭТАП 2: Установка в PATH
# -----------------------------------------------------------------------------
printf "[2/2] Installing to system PATH... "

INSTALL_DIR="/usr/local/bin"
PATH_WARNING=""

if [ "$HOST_OS" = "windows" ]; then
    INSTALL_DIR="$HOME/bin"
    mkdir -p "$INSTALL_DIR"
    cp "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME"
    printf "[OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
else
    # Linux / macOS
    if [ -w "$INSTALL_DIR" ]; then
        install -m 755 "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME"
        printf "[OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
    elif command -v sudo >/dev/null 2>&1; then
        echo ""
        echo "Root permissions required to install into $INSTALL_DIR:"
        sudo install -m 755 "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME"
        printf "[2/2] Installing to system PATH... [OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
    else
        # Запасной вариант без sudo
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
        install -m 755 "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME"
        printf "[OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
    fi

    # Проверка, входит ли каталог установки в системный PATH
    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        PATH_WARNING="[WARN] Directory $INSTALL_DIR is not in your PATH. Add it to ~/.bashrc or ~/.zshrc:\n       export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
fi

END_TOTAL=$(date +%s)
TOTAL_DURATION=$((END_TOTAL - START_TOTAL))

echo "================================================================"
echo "Build and installation completed in ${TOTAL_DURATION}s."
if [ -n "$PATH_WARNING" ]; then
    echo -e "$PATH_WARNING"
    echo "================================================================"
else
    echo "Verification: $(command -v "$BIN_NAME" || echo "$INSTALL_DIR/$BIN_NAME")"
    echo "You can now run 'mediacli' from any directory in your terminal."
    echo "================================================================"
fi