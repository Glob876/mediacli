'use strict';

const { spawnSync } = require('node:child_process');
const { mkdirSync, rmSync, chmodSync } = require('node:fs');
const path = require('node:path');
const { targets, binaryPath } = require('../npm/platforms.cjs');

const root = path.resolve(__dirname, '..');
rmSync(path.join(root, 'npm/bin'), { recursive: true, force: true });
for (const [platform, arch, GOOS, GOARCH] of targets) {
  const output = binaryPath(platform, arch);
  mkdirSync(path.dirname(output), { recursive: true });
  console.log(`Building ${platform}/${arch}...`);
  const result = spawnSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', output, '.'], {
    cwd: root,
    env: { ...process.env, CGO_ENABLED: '0', GOOS, GOARCH },
    stdio: 'inherit',
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  if (platform !== 'win32') chmodSync(output, 0o755);
}
