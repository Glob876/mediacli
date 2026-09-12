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

# -----------------------------------------------------------------------------
# Быстрый запуск Electron-оболочки: ./build.sh --startelectron
# Только проверки + досборка недостающего, без установки в PATH (без sudo).
# -----------------------------------------------------------------------------
if [ "${1:-}" = "--startelectron" ]; then
    echo "== MediaCLI Electron quick start =="

    if ! command -v go >/dev/null 2>&1; then
        echo "[FAIL] go not found in PATH"; exit 1
    fi
    if ! command -v npm >/dev/null 2>&1; then
        echo "[FAIL] npm not found in PATH"; exit 1
    fi

    # Go-бинарник: пересобираем только если отсутствует или исходники новее.
    if [ ! -x "$TARGET_PATH" ] || [ -n "$(find main.go go.mod go.sum pkg -newer "$TARGET_PATH" 2>/dev/null | head -n 1)" ]; then
        echo "[1/3] (Re)building Go binary..."
        CGO_ENABLED=1 go build -trimpath -ldflags="$LDFLAGS" -o "$TARGET_PATH" . >> "$LOG_FILE" 2>&1
    else
        echo "[1/3] Go binary is fresh, skipping build."
    fi

    # Node-зависимости оболочки.
    if [ ! -d "electron/node_modules" ]; then
        echo "[2/3] Installing electron deps (npm ci)..."
        npm ci --prefix electron >> "$LOG_FILE" 2>&1
    else
        echo "[2/3] electron/node_modules present, skipping npm ci."
    fi

    # Бинарник Electron. Штатный install.js зависит от политики npm-scripts
    # и в этом окружении молча распаковывает архив частично (только locales/),
    # поэтому проверяем результат и при необходимости распаковываем целый zip
    # вручную: сначала кеш @electron/get, иначе качаем с GitHub releases.
    E_VER=$(node -p "require('./electron/node_modules/electron/package.json').version")
    E_BIN="electron/node_modules/electron/dist/electron"
    if [ ! -x "$E_BIN" ]; then
        echo "[3/3] Provisioning Electron v${E_VER} binary (one-time, ~110MB)..."
        node electron/node_modules/electron/install.js >> "$LOG_FILE" 2>&1 || true
    else
        echo "[3/3] Electron binary present."
    fi
    if [ ! -x "$E_BIN" ]; then
        for tool in curl unzip python3; do
            if ! command -v "$tool" >/dev/null 2>&1; then
                echo "[FAIL] '$tool' not found in PATH"; exit 1
            fi
        done
        E_ZIP="$DIST_DIR/electron-v${E_VER}-linux-x64.zip"
        E_SIZE=0
        [ -f "$E_ZIP" ] && E_SIZE=$(wc -c < "$E_ZIP")
        if [ "$E_SIZE" -lt 100000000 ] || \
           [ "$(python3 -c "import zipfile,sys; print(zipfile.ZipFile(sys.argv[1]).testzip() or 'OK')" "$E_ZIP" 2>/dev/null)" != "OK" ]; then
            echo "[3/3] Downloading Electron v${E_VER} (progress below, do not interrupt)..."
            curl -L --progress-bar -o "$E_ZIP" \
                "https://github.com/electron/electron/releases/download/v${E_VER}/electron-v${E_VER}-linux-x64.zip"
        else
            echo "[3/3] Reusing verified $E_ZIP"
        fi
        rm -rf "electron/node_modules/electron/dist"
        mkdir -p "electron/node_modules/electron/dist"
        unzip -q "$E_ZIP" -d "electron/node_modules/electron/dist"
    fi
    # path.txt обязан содержать ровно 'electron' без перевода строки,
    # иначе electron/index.js склеит неверный путь (dist/dist/...).
    printf electron > electron/node_modules/electron/path.txt
    if [ ! -x "electron/node_modules/electron/dist/electron" ]; then
        echo "[FAIL] Electron binary still missing. Check network/CDN access and rerun."
        echo "       Log: $LOG_FILE"
        exit 1
    fi

    echo "Starting shell..."
    exec npm start --prefix electron
fi

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

if CGO_ENABLED=1 GOOS="$HOST_OS" GOARCH="$HOST_ARCH" go build \
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

# -----------------------------------------------------------------------------
# ЭТАП 3: Контрольные суммы (обновляем, чтобы dist/checksums.sha256 не протухал)
# -----------------------------------------------------------------------------
if command -v sha256sum >/dev/null 2>&1; then
    (sha256sum "$TARGET_PATH" | sed "s|$(pwd)/||" > "$DIST_DIR/checksums.sha256")
fi

# -----------------------------------------------------------------------------
# ЭТАП 4: Десктопный установщик (опционально: ./build.sh --package [--win|--mac])
# -----------------------------------------------------------------------------
if [ "${1:-}" = "--package" ]; then
    PKG_TARGET="${2:---linux}"
    printf "[4/4] Packaging desktop app (%s)... " "$PKG_TARGET"
    if ! command -v npm >/dev/null 2>&1; then
        printf "[FAIL] (npm not found)\n"; exit 1
    fi
    # Кладём свежий Go-бинарник туда, откуда electron-builder заберёт его
    # в resources/bin (см. electron/main.js + package.json extraResources).
    mkdir -p electron/bin
    cp "$TARGET_PATH" "electron/bin/$BIN_NAME"
    if npm ci --prefix electron >> "$LOG_FILE" 2>&1 \
        && npx --prefix electron electron-builder "$PKG_TARGET" >> "$LOG_FILE" 2>&1; then
        printf "[OK] (see electron/release/)\n"
    else
        printf "[FAIL]\n"
        tail -n 25 "$LOG_FILE"
        exit 1
    fi
fi

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