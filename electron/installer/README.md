# Windows installer

The assisted NSIS installer includes the Rust daemon, FFmpeg, ffprobe and Deno.
yt-dlp is intentionally absent: MediaCLI installs and updates the official
standalone executable from GitHub releases in the user's data directory.

On Windows, from the repository root:

```powershell
npm ci --prefix electron
npm run build --prefix electron
cd electron
npx electron-builder --win nsis --x64 --publish never
# Use --arm64 instead for native Windows ARM64.
```

`before-pack.cjs` checks the Rust build receipt, builds the requested target if
needed, downloads checksum-pinned media tools, and stages only matching binaries
under `electron/bundle/win-<arch>`. Rust (with the requested MSVC target), Node.js,
Python 3, and the Windows linker are build dependencies. Python is not shipped
and is not needed by users. Cross-builds require the matching Rust linker and
Wine for NSIS; do not copy a Linux binary into a Windows installer.

The installer is unsigned. It offers a directory choice and creates shortcuts.
The GitHub Actions workflow produces separate x64 and ARM64 installer artifacts.
Third-party licenses are included under `resources/bin/licenses`; preserve the
corresponding source when distributing the GPL media executables.
