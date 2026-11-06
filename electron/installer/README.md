# Windows installer

The installer is built with electron-builder's assisted NSIS target. It offers a directory choice, creates shortcuts and contains the Rust daemon; yt-dlp and FFmpeg remain separate dependencies.

On Windows, build the Windows binary and place it in the packaging resources before building the installer:

```powershell
cd ..
go build -trimpath -o bin/mediacli.exe ..
npm ci
npx electron-builder --win nsis --x64 --publish never
```

Run the commands from this `installer` directory: the first command changes to `electron`. Build on Windows or use a cross-built Windows binary and Wine on Linux. Only put Windows binaries in `electron/bin` when producing the Windows installer. The installer is unsigned.
