#!/usr/bin/env bash

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

for argument in "$@"; do
    case "$argument" in
        --no-gui|--package|--linux|--win|--mac) ;;
        *) echo "Unknown build option: $argument. Launch the application with ./run.sh." >&2; exit 2 ;;
    esac
done

DIST_DIR="dist"
LOG_FILE="$DIST_DIR/build.log"

mkdir -p "$DIST_DIR"
: > "$LOG_FILE"



# Автоматическое определение текущей системы
case "$(uname -s)" in
    Linux*) HOST_OS=linux ;;
    Darwin*) HOST_OS=darwin ;;
    MINGW*|MSYS*|CYGWIN*) HOST_OS=windows ;;
    *) echo "Unsupported operating system" >&2; exit 1 ;;
esac
HOST_ARCH=$(uname -m)

BIN_NAME="mediacli"
if [ "$HOST_OS" = "windows" ]; then
    BIN_NAME="mediacli.exe"
fi

TARGET_PATH="$DIST_DIR/$BIN_NAME"

# Флаг --no-gui пропускает установку ярлыка в меню
SKIP_GUI=0
for arg in "$@"; do
    if [ "$arg" = "--no-gui" ]; then
        SKIP_GUI=1
        break
    fi
done

# Shared dependency preparation is also used by run.sh.
source "$PROJECT_DIR/scripts/electron-runtime.sh"

START_TOTAL=$(date +%s)

echo "================================================================"
echo "MediaCLI Native Build & Install Pipeline"
echo "================================================================"
echo "Target OS : $HOST_OS ($HOST_ARCH)"
echo "Rust Version: $(rustc --version)"
echo "Binary    : $TARGET_PATH"
echo "Log File  : $LOG_FILE"
echo "================================================================"

# -----------------------------------------------------------------------------
# ЭТАП 1: Компиляция нативного бинарника
# -----------------------------------------------------------------------------
printf "[1/3] Compiling native binary for %s/%s... " "$HOST_OS" "$HOST_ARCH"

START_BUILD=$(date +%s)

if cargo build --release --locked >> "$LOG_FILE" 2>&1; then
    temporary_path=$(mktemp "$PROJECT_DIR/dist/.mediacli.XXXXXX")
    trap 'rm -f "$temporary_path"' EXIT
    cp -p "target/release/$BIN_NAME" "$temporary_path"
    mv -f "$temporary_path" "$TARGET_PATH"
    trap - EXIT
    
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
printf "[2/3] Installing to system PATH... "

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
        if sudo install -m 755 "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME" 2>>"$LOG_FILE"; then
            printf "[2/3] Installing to system PATH... [OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
        else
            echo "sudo failed (no tty/password), falling back to ~/.local/bin"
            INSTALL_DIR="$HOME/.local/bin"
            mkdir -p "$INSTALL_DIR"
            install -m 755 "$TARGET_PATH" "$INSTALL_DIR/$BIN_NAME"
            printf "[OK] (Installed to %s)\n" "$INSTALL_DIR/$BIN_NAME"
        fi
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

# -----------------------------------------------------------------------------
# ЭТАП 3: Установка Electron GUI в меню приложений (KDE/GNOME и т.п.)
# Чтобы появилось в "Меню Пуск" / Application Launcher.
# Пропускается на Windows и при --no-gui.
# -----------------------------------------------------------------------------
if [ "$HOST_OS" != "windows" ] && [ "$SKIP_GUI" -eq 0 ]; then
    printf "[3/3] Installing MediaGUI to application menu... "

    if [ ! -f "electron/package.json" ]; then
        printf "[SKIP] (electron/ not found)\n"
    elif ! command -v npm >/dev/null 2>&1; then
        printf "[SKIP] (npm not found, GUI not installed)\n"
    else
        # Подготавливаем Electron бинарник (npm ci + dist/electron если нужно)
        if ! provision_electron >> "$LOG_FILE" 2>&1 || ! npm run build --prefix electron >> "$LOG_FILE" 2>&1; then
            printf "[SKIP] (Electron provision failed, see %s)\n" "$LOG_FILE"
        else
            # Определяем куда ставить wrapper / .desktop / иконку
            # Если можем писать в /usr/local/bin и /usr/share — ставим системно (нужен sudo),
            # иначе — в ~/.local
            USE_SYSTEM=0
            WRAPPER_DIR="$INSTALL_DIR"
            DESKTOP_DIR="$HOME/.local/share/applications"
            ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
            ICON_NAME="mediagui"
            WRAPPER_PATH="$WRAPPER_DIR/mediagui"
            DESKTOP_PATH="$DESKTOP_DIR/mediagui.desktop"

            # Если бинарник уехал в /usr/local/bin — пробуем системный .desktop тоже
            if [ "$INSTALL_DIR" = "/usr/local/bin" ]; then
                WRAPPER_DIR="/usr/local/bin"
                DESKTOP_DIR="/usr/share/applications"
                ICON_DIR="/usr/share/icons/hicolor/512x512/apps"
                WRAPPER_PATH="$WRAPPER_DIR/mediagui"
                DESKTOP_PATH="$DESKTOP_DIR/mediagui.desktop"
                USE_SYSTEM=1
            elif [ -w "/usr/local/bin" ] || { command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; } then
                if [ -w "/usr/share/applications" ] || command -v sudo >/dev/null 2>&1; then
                    WRAPPER_DIR="/usr/local/bin"
                    DESKTOP_DIR="/usr/share/applications"
                    ICON_DIR="/usr/share/icons/hicolor/512x512/apps"
                    WRAPPER_PATH="$WRAPPER_DIR/mediagui"
                    DESKTOP_PATH="$DESKTOP_DIR/mediagui.desktop"
                    USE_SYSTEM=1
                fi
            fi

            mkdir -p "$DESKTOP_DIR" "$ICON_DIR" "$WRAPPER_DIR" 2>/dev/null || true

            # 1) Wrapper-скрипт: запускает Electron с абсолютным путём к проекту
            #    Используем dist/electron напрямую для надёжности.
            WRAPPER_TMP="$DIST_DIR/mediagui.wrapper"
            cat > "$WRAPPER_TMP" <<WRAPPER_EOF
#!/bin/sh
# MediaGUI launcher — сгенерирован build.sh
# Запускает Electron-оболочку из $PROJECT_DIR/electron
set -e
PROJECT_DIR="$PROJECT_DIR"
ELECTRON_BIN="\$PROJECT_DIR/electron/node_modules/electron/dist/electron"
ELECTRON_DIR="\$PROJECT_DIR/electron"
if [ ! -x "\$ELECTRON_BIN" ]; then
    echo "MediaGUI: Electron binary not found at \$ELECTRON_BIN" >&2
    echo "Пересоберите: ./build.sh" >&2
    exit 1
fi
exec "\$ELECTRON_BIN" "\$ELECTRON_DIR" "\$@"
WRAPPER_EOF
            chmod 755 "$WRAPPER_TMP"

            if [ "$USE_SYSTEM" -eq 1 ] && [ ! -w "$WRAPPER_DIR" ]; then
                if ! sudo install -m 755 "$WRAPPER_TMP" "$WRAPPER_PATH" >> "$LOG_FILE" 2>&1; then
                    echo "sudo failed for wrapper, falling back to ~/.local/bin" >> "$LOG_FILE"
                    WRAPPER_DIR="$HOME/.local/bin"
                    WRAPPER_PATH="$WRAPPER_DIR/mediagui"
                    DESKTOP_DIR="$HOME/.local/share/applications"
                    DESKTOP_PATH="$DESKTOP_DIR/mediagui.desktop"
                    ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
                    USE_SYSTEM=0
                    mkdir -p "$WRAPPER_DIR" "$DESKTOP_DIR" "$ICON_DIR"
                    install -m 755 "$WRAPPER_TMP" "$WRAPPER_PATH" >> "$LOG_FILE" 2>&1
                fi
            else
                install -m 755 "$WRAPPER_TMP" "$WRAPPER_PATH" >> "$LOG_FILE" 2>&1
            fi

            # 2) Иконка
            SRC_ICON="$PROJECT_DIR/electron/renderer/assets/logo.png"
            DEST_ICON="$ICON_DIR/$ICON_NAME.png"
            ICON_FOR_DESKTOP="$ICON_NAME"
            if [ -f "$SRC_ICON" ]; then
                if [ "$USE_SYSTEM" -eq 1 ] && [ ! -w "$ICON_DIR" ]; then
                    sudo install -Dm644 "$SRC_ICON" "$DEST_ICON" >> "$LOG_FILE" 2>&1 || true
                else
                    install -Dm644 "$SRC_ICON" "$DEST_ICON" >> "$LOG_FILE" 2>&1 || true
                fi
                # Проверяем что иконка действительно установилась, иначе fallback на абсолютный путь
                if [ ! -f "$DEST_ICON" ]; then
                    ICON_FOR_DESKTOP="$SRC_ICON"
                fi
            else
                ICON_FOR_DESKTOP="video-display"
            fi

            # 3) .desktop файл (совместим с KDE, GNOME, Xfce)
            # Важно: обновляем Exec на случай fallback wrapper после sudo failure
            DESKTOP_TMP="$DIST_DIR/mediagui.desktop"
            cat > "$DESKTOP_TMP" <<DESKTOP_EOF
[Desktop Entry]
Name=MediaGUI
GenericName=Media Downloader
Comment=Графическая оболочка MediaCLI — загрузка видео/аудио через yt-dlp и FFmpeg
Exec=$WRAPPER_PATH %U
Icon=$ICON_FOR_DESKTOP
Terminal=false
Type=Application
Categories=AudioVideo;Video;
Keywords=yt-dlp;ffmpeg;youtube;download;video;mediacli;
StartupWMClass=MediaGUI
MimeType=x-scheme-handler/http;x-scheme-handler/https;
Actions=

DESKTOP_EOF
            chmod 644 "$DESKTOP_TMP"

            if [ "$USE_SYSTEM" -eq 1 ] && [ ! -w "$DESKTOP_DIR" ]; then
                if ! sudo install -m 644 "$DESKTOP_TMP" "$DESKTOP_PATH" >> "$LOG_FILE" 2>&1; then
                    DESKTOP_DIR="$HOME/.local/share/applications"
                    DESKTOP_PATH="$DESKTOP_DIR/mediagui.desktop"
                    mkdir -p "$DESKTOP_DIR"
                    # Перегенерируем .desktop с актуальным Exec
                    cat > "$DESKTOP_TMP" <<DESKTOP_EOF2
[Desktop Entry]
Name=MediaGUI
GenericName=Media Downloader
Comment=Графическая оболочка MediaCLI — загрузка видео/аудио через yt-dlp и FFmpeg
Exec=$WRAPPER_PATH %U
Icon=$ICON_FOR_DESKTOP
Terminal=false
Type=Application
Categories=AudioVideo;Video;
Keywords=yt-dlp;ffmpeg;youtube;download;video;mediacli;
StartupWMClass=MediaGUI
MimeType=x-scheme-handler/http;x-scheme-handler/https;
Actions=

DESKTOP_EOF2
                    install -m 644 "$DESKTOP_TMP" "$DESKTOP_PATH" >> "$LOG_FILE" 2>&1
                fi
            else
                install -m 644 "$DESKTOP_TMP" "$DESKTOP_PATH" >> "$LOG_FILE" 2>&1
            fi

            # 4) Обновляем кэши меню/иконок
            if command -v update-desktop-database >/dev/null 2>&1; then
                if [ "$USE_SYSTEM" -eq 1 ] && [ ! -w "$DESKTOP_DIR" ]; then
                    sudo update-desktop-database "$DESKTOP_DIR" >> "$LOG_FILE" 2>&1 || true
                else
                    update-desktop-database "$DESKTOP_DIR" >> "$LOG_FILE" 2>&1 || true
                fi
            fi
            if command -v gtk-update-icon-cache >/dev/null 2>&1 && [ -d "$(dirname "$ICON_DIR")" ]; then
                if [ "$USE_SYSTEM" -eq 1 ]; then
                    sudo gtk-update-icon-cache -f -t "$(dirname "$(dirname "$ICON_DIR")")" >> "$LOG_FILE" 2>&1 || true
                else
                    gtk-update-icon-cache -f -t "$HOME/.local/share/icons/hicolor" >> "$LOG_FILE" 2>&1 || true
                fi
            fi
            # KDE может требовать kbuildsycoca6
            if command -v kbuildsycoca6 >/dev/null 2>&1; then
                kbuildsycoca6 >> "$LOG_FILE" 2>&1 || true
            elif command -v kbuildsycoca5 >/dev/null 2>&1; then
                kbuildsycoca5 >> "$LOG_FILE" 2>&1 || true
            fi

            if [ -f "$DESKTOP_PATH" ] && [ -x "$WRAPPER_PATH" ]; then
                printf "[OK] (MediaGUI -> %s, wrapper %s)\n" "$DESKTOP_PATH" "$WRAPPER_PATH"
            else
                printf "[FAIL] (desktop/wrapper not created, see %s)\n" "$LOG_FILE"
            fi
        fi
    fi
else
    if [ "$SKIP_GUI" -eq 1 ]; then
        printf "[3/3] Skipping MediaGUI desktop entry (--no-gui)\n"
    fi
fi

END_TOTAL=$(date +%s)
TOTAL_DURATION=$((END_TOTAL - START_TOTAL))

# -----------------------------------------------------------------------------
# Контрольные суммы (обновляем, чтобы dist/checksums.sha256 не протухал)
# -----------------------------------------------------------------------------
if command -v sha256sum >/dev/null 2>&1; then
    (sha256sum "$TARGET_PATH" | sed "s|$(pwd)/||" > "$DIST_DIR/checksums.sha256")
fi

# -----------------------------------------------------------------------------
# Десктопный установщик (опционально: ./build.sh --package [--win|--mac])
# Собирает AppImage/deb через electron-builder
# -----------------------------------------------------------------------------
if [ "${1:-}" = "--package" ] || [ "${2:-}" = "--package" ]; then
    # Находим аргумент --package и следующий за ним таргет
    PKG_TARGET="--linux"
    for i in 1 2 3; do
        eval "arg=\${$i:-}"
        if [ "$arg" = "--package" ]; then
            eval "next=\${$((i+1)):-}"
            if [ -n "${next:-}" ] && [ "${next:0:1}" != "-" ] || [[ "$next" == --* ]]; then
                # только --linux/--win/--mac считаем таргетом
                case "$next" in --linux|--win|--mac) PKG_TARGET="$next" ;; esac
            fi
        fi
    done
    # Совместимость: ./build.sh --package --win  -> второй аргумент это таргет
    if [ "${2:-}" = "--linux" ] || [ "${2:-}" = "--win" ] || [ "${2:-}" = "--mac" ]; then
        PKG_TARGET="${2}"
    fi
    printf "[package] Packaging desktop app (%s)... " "$PKG_TARGET"
    if ! command -v npm >/dev/null 2>&1; then
        printf "[FAIL] (npm not found)\n"; exit 1
    fi
    # Кладём свежий Rust-бинарник туда, откуда electron-builder заберёт его
    # в resources/bin (см. electron/main.ts + package.json extraResources).
    mkdir -p electron/bin
    cp "$TARGET_PATH" "electron/bin/$BIN_NAME"
    if npm ci --prefix electron >> "$LOG_FILE" 2>&1 \
        && npm run build --prefix electron >> "$LOG_FILE" 2>&1 \
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
    if [ "$HOST_OS" != "windows" ] && [ "$SKIP_GUI" -eq 0 ]; then
        echo "MediaGUI is available in your application menu (KDE/GNOME) as 'MediaGUI'."
        echo "  Launcher: $(command -v mediagui || echo "$HOME/.local/bin/mediagui / /usr/local/bin/mediagui")"
        echo "  Or run: ./run.sh --electron"
    fi
    echo "================================================================"
fi
