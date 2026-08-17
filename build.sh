#!/usr/bin/env bash

# Строгий режим: прерывание при любой ошибке, необъявленной переменной или сбое в пайпе
set -euo pipefail

DIST_DIR="dist"
LOG_FILE="$DIST_DIR/build.log"
CHECKSUM_FILE="$DIST_DIR/checksums.sha256"

# Создаем каталог для артефактов
mkdir -p "$DIST_DIR"

# Перенаправляем весь вывод (stdout и stderr) одновременно на экран и в файл лога
exec > >(tee -i "$LOG_FILE") 2>&1

log() {
    local level="$1"
    shift
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [${level}] $*"
}

trap 'log "ERROR" "Build failed on line $LINENO. Check $LOG_FILE for details."' ERR

START_TOTAL=$(date +%s)

log "INFO" "================================================================="
log "INFO" "MediaCLI Multi-Platform Build Pipeline"
log "INFO" "================================================================="
log "INFO" "Host OS: $(uname -s) ($(uname -m))"
log "INFO" "Go Version: $(go version)"
log "INFO" "Target Directory: $DIST_DIR"
log "INFO" "Log File: $LOG_FILE"
log "INFO" "================================================================="

# Флаги линковщика: -s (strip symbol table), -w (strip DWARF debugging info)
LDFLAGS="-s -w"

# Список целевых платформ: "OS ARCH OUTPUT_FILENAME"
TARGETS=(
    "linux amd64 mediacli-linux-amd64"
    "linux arm64 mediacli-linux-arm64"
    "windows amd64 mediacli-windows-amd64.exe"
    "darwin arm64 mediacli-darwin-arm64"
    "darwin amd64 mediacli-darwin-amd64"
)

TOTAL_TARGETS=${#TARGETS[@]}
CURRENT=1

for TARGET in "${TARGETS[@]}"; do
    read -r TARGET_OS TARGET_ARCH TARGET_BIN <<< "$TARGET"
    TARGET_PATH="$DIST_DIR/$TARGET_BIN"

    log "INFO" "[$CURRENT/$TOTAL_TARGETS] Compiling for target: OS=$TARGET_OS ARCH=$TARGET_ARCH..."
    START_TARGET=$(date +%s)

    # CGO_ENABLED=0 обеспечивает 100% статическую линковку без зависимости от системного glibc
    CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build \
        -v \
        -trimpath \
        -ldflags="$LDFLAGS" \
        -o "$TARGET_PATH" \
        .

    END_TARGET=$(date +%s)
    TARGET_DURATION=$((END_TARGET - START_TARGET))
    FILE_SIZE=$(ls -lh "$TARGET_PATH" | awk '{print $5}')

    log "INFO" "[$CURRENT/$TOTAL_TARGETS] Finished $TARGET_BIN (Size: $FILE_SIZE, Time: ${TARGET_DURATION}s)"
    CURRENT=$((CURRENT + 1))
done

log "INFO" "-----------------------------------------------------------------"
log "INFO" "Calculating SHA-256 checksums..."
rm -f "$CHECKSUM_FILE"

for TARGET in "${TARGETS[@]}"; do
    read -r _ _ TARGET_BIN <<< "$TARGET"
    (cd "$DIST_DIR" && sha256sum "$TARGET_BIN") >> "$CHECKSUM_FILE"
done

log "INFO" "Checksums saved to: $CHECKSUM_FILE"
log "INFO" "-----------------------------------------------------------------"

END_TOTAL=$(date +%s)
TOTAL_DURATION=$((END_TOTAL - START_TOTAL))

log "INFO" "Build Summary:"
log "INFO" "Total targets built: $TOTAL_TARGETS"
log "INFO" "Total build duration: ${TOTAL_DURATION}s"
log "INFO" "Artifacts list:"
ls -lh "$DIST_DIR"
log "INFO" "================================================================="
log "INFO" "Build completed successfully."
log "INFO" "================================================================="