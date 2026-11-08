'use strict';
const { spawnSync } = require('node:child_process');
const { mkdirSync } = require('node:fs');
const path = require('node:path');
const { targets } = require('../npm/platforms.cjs');
const root = path.resolve(__dirname, '..');
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
function run(arguments_) {
  const result = spawnSync(npm, arguments_, { cwd: root, stdio: 'inherit', shell: process.platform === 'win32' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
require('./verify-npm.cjs');
mkdirSync(path.join(root, 'dist'), { recursive: true });
for (const [platform, arch] of targets) run(['pack', `./npm/bin/${platform}-${arch}`, '--pack-destination', 'dist']);
run(['pack', '--pack-destination', 'dist']);
