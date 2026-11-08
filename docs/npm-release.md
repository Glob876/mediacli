# npm releases

The root package is `@glob876/mediacli`; its executable is `mediacli`.
It includes the Rust CLI/daemon for Linux, macOS and Windows on x64 and arm64.
The Electron application is separate and is not part of this npm package.
Node.js 18+ is required to launch the CLI. Rust is only needed to build it.
FFmpeg, ffprobe and Deno are included in native packages. yt-dlp is downloaded
on first use from official GitHub releases, SHA-256 checked, and updated at most
once per day during downloads. `mediacli tools update` forces an update check.

The root package depends optionally on six platform packages named
`@glob876/mediacli-<platform>-<arch>`. npm installs only the package matching the
host OS and CPU; do not use `--omit=optional`. Each native package includes
MediaCLI, FFmpeg, ffprobe, Deno, checksums and third-party licenses. No postinstall
script is needed. Tool download URLs and SHA-256 values are pinned in
`packaging/tools.lock.json`; Python 3 downloads and stages tools at build time,
not on the user's machine. Cargo uses the target platform's linker.
`build.sh` retains its native source installation flow; source builds without
bundled tools need FFmpeg/ffprobe in PATH. Unsupported platforms build from source.

## Workflow

`.github/workflows/npm.yml` runs on pushes to `dev` and `main`, pull requests
targeting `main`, and manual dispatch. It runs Rust tests (including FFmpeg/yt-dlp
integration tests), Clippy, formatting, TypeScript checks and launcher tests, builds six native tool packages, packs the root and platform
archives, builds Windows x64/ARM64 installers, and tests an installation on Linux x64. Other platforms are compiled
but are not runtime-tested. Download the archive from the `npm-package` Actions
artifact even before npm publishing is configured:

```bash
npm install -g ./glob876-mediacli-linux-x64-0.1.4.tgz ./glob876-mediacli-0.1.4.tgz
mediacli --help
```

Platform packages are published before the root package. All seven packages
use the root version. For first publication, the npm credential must be allowed
to create the six new scoped packages as well as update MediaCLI.

Publication runs only after a successful build on `main` (including a merge).
Pull requests and `dev` never publish. The publishing job explicitly reports
when npm authentication is not configured; the build artifact is still saved.

## Connect npm once

The npm account must own the `glob876` scope, or change `name` in `package.json`
and regenerate `package-lock.json` before the first release.

For the first publication, create a granular npm token with write permission
for this package/scope and permission to publish from CI (including the npm
account's required 2FA configuration). Store it as the GitHub Actions repository
secret `NPM_TOKEN`:

```bash
# Run locally; gh prompts for the secret without putting it in shell history.
gh secret set NPM_TOKEN --repo Glob876/mediacli
```

Merge this workflow into `main`, or run it manually on `main` if already merged:

```bash
gh workflow run npm.yml --ref main --repo Glob876/mediacli
```

After the package exists, prefer npm's
[trusted publishing](https://docs.npmjs.com/trusted-publishers/). In the package's
npm settings, add a GitHub Actions trusted publisher with these exact values:

| Field | Value |
| --- | --- |
| Organization/user | `Glob876` |
| Repository | `mediacli` |
| Workflow filename | `npm.yml` |
| Environment | Leave empty |

Allow direct `npm publish` for this publisher. Enable it in GitHub:

```bash
gh variable set NPM_TRUSTED_PUBLISHING --body true --repo Glob876/mediacli
```

Remove the `NPM_TOKEN` secret after switching to OIDC. The workflow uses Node 24
with a compatible npm CLI, `id-token: write`, and provenance. GitHub access
alone does not grant permission to publish to npm.

## Release a new version

Before merging a release from `dev`, increment the root version and commit both
manifests, for example:

```bash
npm version patch --no-git-tag-version
git add package.json package-lock.json
git commit -m "Release the next CLI patch"
```

Every merge into `main` builds an archive. It publishes only if that exact
version is absent from npm; already published versions are skipped. Registry
errors other than a missing package/version fail the publishing job. Publishing
does not change Git history, create tags, or automatically bump versions.

After publication, install with:

```bash
npm install -g @glob876/mediacli
# Or run without a global installation:
npx @glob876/mediacli --help
```

## Local checks

```bash
npm ci --ignore-scripts
cargo test --locked
cargo clippy --locked --all-targets -- -D warnings
cargo fmt --all --check
npm run build --prefix electron
npm test
npm run build
# Build all other target triples on their native runners before packing.
npm run pack:release
```

`npm pack` refuses to package an incomplete build. Build outputs are ignored by
Git. The npm `files` allowlist excludes Rust sources, tests, release scripts, and
the Electron app from the published archive.

## Third-party binaries

Update `packaging/tools.lock.json` deliberately, using release-specific URLs and
verified SHA-256 values. The bundler rejects wrong-architecture executables and
never includes yt-dlp. It preserves FFmpeg build documentation and licenses.
Deno's license and dependency notices are shipped in `packaging/licenses`.
Before public distribution, provide the complete corresponding source for the
GPL FFmpeg builds, including dependencies and build recipes; see the bundle notice.
The workflow creates build artifacts on `dev`; it does not publish from `dev`.
