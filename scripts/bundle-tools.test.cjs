'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const path = require('node:path');

test('tool staging validates architecture, contains archive paths and excludes yt-dlp', () => {
  const result = spawnSync(process.platform === 'win32' ? 'python' : 'python3', [path.join(__dirname, 'test_bundle_tools.py')], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
});
