# MediaCLI bundled executables

MediaCLI invokes these programs as separate processes. Its own source remains MIT licensed.

- FFmpeg and ffprobe: GPL builds. See the accompanying upstream LICENSE/README,
  `tools.json` and `packaging/tools.lock.json` for the exact binary release,
  checksums and build provenance. Linux/Windows builds come from
  https://github.com/yt-dlp/FFmpeg-Builds (build scripts and dependency recipes).
  Their FFmpeg source revision is caa98b4938:
  https://github.com/FFmpeg/FFmpeg/tree/caa98b4938
  macOS builds come from https://github.com/eugeneware/ffmpeg-static/releases/tag/b6.1.1
  and preserve that release's license and source/build documentation.
- Deno 2.9.8: MIT; accompanying DENO-LICENSE and V8-LICENSE apply. Dependency source and
  individual license notices are available in the pinned Deno source tree.
  Source: https://github.com/denoland/deno/tree/v2.9.8
- yt-dlp is NOT included. MediaCLI downloads official standalone release files
  from https://github.com/yt-dlp/yt-dlp/releases at runtime, checks SHA-256 and
  keeps each installed version separate. Upstream licenses:
  https://github.com/yt-dlp/yt-dlp/blob/master/THIRD_PARTY_LICENSES.txt

Distributors must retain these notices and make the complete corresponding
source of GPL executables (including linked dependencies and build scripts)
available in accordance with their licenses. The upstream binary archive alone
is not a substitute for corresponding source. Keep the matching source alongside
public release downloads; do not relabel these binaries as MIT.
