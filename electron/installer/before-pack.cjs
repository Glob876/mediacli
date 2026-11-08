'use strict';
const { spawnSync } = require('node:child_process');
const { cpSync, mkdirSync, readFileSync } = require('node:fs');
const path = require('node:path');
const { targets } = require('../../npm/platforms.cjs');
const { sourceHash, binaryHash } = require('../../scripts/build-info.cjs');
const { version } = require('../../package.json');

module.exports = async function beforePack(context) {
  const platform = context.electronPlatformName;
  const arch = require('builder-util').Arch[context.arch];
  const selected = targets.find(([os, cpu]) => os === platform && cpu === arch);
  if (!selected) throw new Error(`Unsupported desktop target: ${platform}/${arch}`);
  const root = path.resolve(__dirname, '../..');
  const nativeDirectory = path.join(root, 'npm/bin', `${platform}-${arch}`);
  const executable = platform === 'win32' ? 'mediacli.exe' : 'mediacli';
  let valid = false;
  try {
    const receipt = JSON.parse(readFileSync(path.join(nativeDirectory, 'build.json'), 'utf8'));
    valid = receipt.version === version && receipt.target === selected[2] && receipt.source === sourceHash() && receipt.binary === binaryHash(path.join(nativeDirectory, executable));
  } catch { /* Build a fresh target below. */ }
  if (!valid) {
    const result = spawnSync(process.execPath, [path.join(root, 'scripts/build-npm.cjs'), selected[2]], { cwd: root, stdio: 'inherit' });
    if (result.error) throw result.error;
    if (result.status !== 0) throw new Error('Native application/tools build failed');
  } else {
    const result = spawnSync(process.platform === 'win32' ? 'python' : 'python3', [path.join(root, 'scripts/bundle-tools.py'), platform, arch, nativeDirectory], { cwd: root, stdio: 'inherit' });
    if (result.error) throw result.error;
    if (result.status !== 0) throw new Error('Bundled tool preparation failed');
  }
  const builderPlatform = { win32: 'win', darwin: 'mac', linux: 'linux' }[platform];
  const destination = path.join(__dirname, '../bundle', `${builderPlatform}-${arch}`);
  mkdirSync(destination, { recursive: true });
  for (const file of [executable, 'ffmpeg' + (platform === 'win32' ? '.exe' : ''), 'ffprobe' + (platform === 'win32' ? '.exe' : ''), 'deno' + (platform === 'win32' ? '.exe' : ''), 'licenses', 'tools.json']) {
    cpSync(path.join(nativeDirectory, file), path.join(destination, file), { recursive: true });
  }
};
