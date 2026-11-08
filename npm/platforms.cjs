'use strict';

const path = require('node:path');

const targets = [
  ['linux', 'x64', 'x86_64-unknown-linux-gnu'],
  ['linux', 'arm64', 'aarch64-unknown-linux-gnu'],
  ['darwin', 'x64', 'x86_64-apple-darwin'],
  ['darwin', 'arm64', 'aarch64-apple-darwin'],
  ['win32', 'x64', 'x86_64-pc-windows-msvc'],
  ['win32', 'arm64', 'aarch64-pc-windows-msvc'],
];

function binaryPath(platform = process.platform, arch = process.arch) {
  if (!targets.some(([os, cpu]) => os === platform && cpu === arch)) {
    throw new Error(`Unsupported platform: ${platform}/${arch}. Build MediaCLI from source instead.`);
  }
  return path.join(__dirname, 'bin', `${platform}-${arch}`, platform === 'win32' ? 'mediacli.exe' : 'mediacli');
}

function installedBinaryPath(platform = process.platform, arch = process.arch) {
  const developmentPath = binaryPath(platform, arch);
  try {
    const manifest = require.resolve(`@glob876/mediacli-${platform}-${arch}/package.json`);
    return path.join(path.dirname(manifest), platform === 'win32' ? 'mediacli.exe' : 'mediacli');
  } catch (error) {
    if (error.code !== 'MODULE_NOT_FOUND') throw error;
    if (require('node:fs').existsSync(developmentPath)) return developmentPath;
    throw new Error(`The native package @glob876/mediacli-${platform}-${arch} is missing. Reinstall MediaCLI with optional dependencies enabled.`);
  }
}

module.exports = { targets, binaryPath, installedBinaryPath };
