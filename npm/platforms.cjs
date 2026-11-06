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

module.exports = { targets, binaryPath };
