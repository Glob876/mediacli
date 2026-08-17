#!/usr/bin/env bash

set -euo pipefail

DIST_DIR="dist"
LOG_FILE="$DIST_DIR/build.log"
CHECKSUM_FILE="$DIST_DIR/checksums.sha256"

mkdir -p "$DIST_DIR"
: > "$LOG_FILE"

LDFLAGS="-s -w"

TARGETS=(
    "linux amd64 mediacli-linux-amd64"
    "linux arm64 mediacli-linux-arm64"
    "windows amd64 mediacli-windows-amd64.exe"
    "darwin arm64 mediacli-darwin-arm64"
    "darwin amd64 mediacli-darwin-amd64"
)

TOTAL_TARGETS=${#TARGETS[@]}
START_TOTAL=$(date +%s)

echo "================================================================"
echo "MediaCLI Multi-Platform Build Pipeline"
echo "================================================================"
echo "Host OS   : $(uname -s) ($(uname -m))"
echo "Go Version: $(go version | awk '{print $3, $4}')"
echo "Output Dir: $DIST_DIR"
echo "Log File  : $LOG_FILE"
echo "================================================================"

CURRENT=1
for TARGET in "${TARGETS[@]}"; do
    read -r TARGET_OS TARGET_ARCH TARGET_BIN <<< "$TARGET"
    TARGET_PATH="$DIST_DIR/$TARGET_BIN"

    printf "[%d/%d] Compiling %-7s / %-7s -> %-30s " "$CURRENT" "$TOTAL_TARGETS" "$TARGET_OS" "$TARGET_ARCH" "$TARGET_BIN"
    
    START_TARGET=$(date +%s)

    # Без флага -v вывод идет чисто, а ошибки пишутся в лог
    if CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build \
        -trimpath \
        -ldflags="$LDFLAGS" \
        -o "$TARGET_PATH" \
        . >> "$LOG_FILE" 2>&1; then
        
        END_TARGET=$(date +%s)
        TARGET_DURATION=$((END_TARGET - START_TARGET))
        FILE_SIZE=$(ls -lh "$TARGET_PATH" | awk '{print $5}')
        printf "[OK] (%ds, %s)\n" "$TARGET_DURATION" "$FILE_SIZE"
    else
        printf "[FAIL]\n"
        echo "Error details:"
        tail -n 20 "$LOG_FILE"
        exit 1
    fi

    CURRENT=$((CURRENT + 1))
done

echo "----------------------------------------------------------------"
printf "Generating SHA-256 checksums... "
rm -f "$CHECKSUM_FILE"

for TARGET in "${TARGETS[@]}"; do
    read -r _ _ TARGET_BIN <<< "$TARGET"
    (cd "$DIST_DIR" && sha256sum "$TARGET_BIN") >> "$CHECKSUM_FILE"
done
printf "[OK]\n"

END_TOTAL=$(date +%s)
TOTAL_DURATION=$((END_TOTAL - START_TOTAL))

echo "================================================================"
echo "Build Summary: $TOTAL_TARGETS targets built in ${TOTAL_DURATION}s"
echo "Artifacts generated in $DIST_DIR:"
ls -lh "$DIST_DIR" | grep -v "total" | grep -v "build.log"
echo "================================================================"