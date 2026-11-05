'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const path = require('node:path');
const { binaryPath, targets } = require('./platforms.cjs');

test('all supported platforms select a bundled binary', () => {
  for (const [platform, arch] of targets) {
    assert.equal(binaryPath(platform, arch), path.join(__dirname, 'bin', `${platform}-${arch}`, platform === 'win32' ? 'mediacli.exe' : 'mediacli'));
  }
});

test('unsupported platforms fail before launching a process', () => {
  assert.throws(() => binaryPath('freebsd', 'x64'), /Unsupported platform/);
  assert.throws(() => binaryPath('linux', 'ia32'), /Unsupported platform/);
});
