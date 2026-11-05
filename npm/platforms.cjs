'use strict';

const path = require('node:path');

const targets = [
  ['linux', 'x64', 'linux', 'amd64'],
  ['linux', 'arm64', 'linux', 'arm64'],
  ['darwin', 'x64', 'darwin', 'amd64'],
  ['darwin', 'arm64', 'darwin', 'arm64'],
  ['win32', 'x64', 'windows', 'amd64'],
  ['win32', 'arm64', 'windows', 'arm64'],
];

function binaryPath(platform = process.platform, arch = process.arch) {
  if (!targets.some(([os, cpu]) => os === platform && cpu === arch)) {
    throw new Error(`Unsupported platform: ${platform}/${arch}. Build MediaCLI from source instead.`);
  }
  return path.join(__dirname, 'bin', `${platform}-${arch}`, platform === 'win32' ? 'mediacli.exe' : 'mediacli');
}

module.exports = { targets, binaryPath };
