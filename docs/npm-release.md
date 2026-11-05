# npm releases

The root package is `@glob876/mediacli`; its executable is `mediacli`.
It includes the Go CLI/daemon for Linux, macOS and Windows on x64 and arm64.
The Electron application is separate and is not part of this npm package.
Node.js 18+ is required to launch the CLI. Go is only needed to build it.
`yt-dlp`, `ffmpeg` and `ffprobe` remain external dependencies in `PATH`.

All six binaries are included in one archive, so installation does not need
postinstall scripts, Go, or additional downloads from GitHub. npm binaries use
`CGO_ENABLED=0` for portable cross-compilation; `build.sh` retains its existing
native build behavior. Unsupported platforms must build from source.

## Workflow

`.github/workflows/npm.yml` runs on pushes to `dev` and `main`, pull requests
targeting `main`, and manual dispatch. It runs Go tests (including the FFmpeg
integration tests), `go vet`, and launcher tests, builds six binaries, packs the
archive, and tests an installation on Linux x64. Other platforms are compiled
but are not runtime-tested. Download the archive from the `npm-package` Actions
artifact even before npm publishing is configured:

```bash
npm install -g ./glob876-mediacli-0.1.0.tgz
mediacli --help
```

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
go test ./...
go vet ./...
npm test
npm run build
npm pack
```

`npm pack` refuses to package an incomplete build. Build outputs are ignored by
Git. The npm `files` allowlist excludes Go sources, tests, release scripts, and
the Electron app from the published archive.
