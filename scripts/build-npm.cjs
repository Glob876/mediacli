'use strict';
const { spawnSync } = require('node:child_process');
const { mkdirSync, copyFileSync, chmodSync } = require('node:fs');
const path = require('node:path');
const { targets, binaryPath } = require('../npm/platforms.cjs');
const root = path.resolve(__dirname, '..');
const requested = process.argv[2];
const selected = requested ? targets.filter(([, , target]) => target === requested) : targets.filter(([platform, arch]) => platform === process.platform && arch === process.arch);
if (!selected.length) throw new Error(`Unsupported Rust build target: ${requested || process.platform + '/' + process.arch}`);
for (const [platform, arch, target] of selected) {
  const output = binaryPath(platform, arch);
  console.log(`Building ${platform}/${arch} (${target})...`);
  const result = spawnSync('cargo', ['build', '--release', '--locked', '--target', target], { cwd: root, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  mkdirSync(path.dirname(output), { recursive: true });
  copyFileSync(path.join(root, 'target', target, 'release', platform === 'win32' ? 'mediacli.exe' : 'mediacli'), output);
  if (platform !== 'win32') chmodSync(output, 0o755);
}
