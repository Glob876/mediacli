'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { mkdtempSync, mkdirSync, copyFileSync, writeFileSync, readFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

test('run prepares the backend from another directory and forwards Electron arguments', { skip: process.platform === 'win32' }, t => {
  const root = mkdtempSync(path.join(tmpdir(), 'mediacli-run-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['scripts', 'tools', 'electron/node_modules/electron', 'electron/node_modules/typescript/bin']) mkdirSync(path.join(root, directory), { recursive: true });
  copyFileSync(path.join(__dirname, '../run.sh'), path.join(root, 'run.sh'));
  copyFileSync(path.join(__dirname, 'electron-runtime.sh'), path.join(root, 'scripts/electron-runtime.sh'));
  const executable = path.join(root, 'electron/node_modules/electron/runtime');
  writeFileSync(executable, '#!/bin/sh\nexit 0\n', { mode: 0o755 });
  writeFileSync(path.join(root, 'electron/node_modules/electron/package.json'), '{"main":"index.js"}');
  writeFileSync(path.join(root, 'electron/node_modules/electron/index.js'), `module.exports = ${JSON.stringify(executable)};\n`);
  writeFileSync(path.join(root, 'electron/node_modules/typescript/bin/tsc'), '');
  writeFileSync(path.join(root, 'tools/cargo'), `#!/usr/bin/env node
const fs = require('node:fs');
fs.mkdirSync('target/release', {recursive:true});
fs.writeFileSync('target/release/mediacli', 'Rust fixture', {mode:0o755});
fs.writeFileSync('cargo-arguments.json', JSON.stringify(process.argv.slice(2)));
`, { mode: 0o755 });
  writeFileSync(path.join(root, 'tools/npm'), `#!/usr/bin/env node
require('node:fs').writeFileSync('launch.json', JSON.stringify({args:process.argv.slice(2),binary:process.env.MEDIACLI_BIN,cwd:process.cwd()}));
`, { mode: 0o755 });
  const result = spawnSync('bash', [path.join(root, 'run.sh'), '--mock'], { cwd: tmpdir(), env: { ...process.env, PATH: `${path.join(root, 'tools')}${path.delimiter}${process.env.PATH}` }, encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  const launch = JSON.parse(readFileSync(path.join(root, 'launch.json'), 'utf8'));
  assert.deepEqual(launch.args, ['start', '--prefix', 'electron', '--', '--mock']);
  assert.equal(launch.binary, path.join(root, 'dist/mediacli'));
  assert.equal(launch.cwd, root);
  assert.equal(readFileSync(launch.binary, 'utf8'), 'Rust fixture');
  assert.deepEqual(JSON.parse(readFileSync(path.join(root, 'cargo-arguments.json'), 'utf8')), ['build', '--release', '--locked']);
});

test('build rejects launch options before installing anything', { skip: process.platform === 'win32' }, () => {
  const result = spawnSync('bash', [path.join(__dirname, '../build.sh'), '--startelectron'], { encoding: 'utf8' });
  assert.equal(result.status, 2);
  assert.match(result.stderr, /Launch the application with .\/run.sh/);
});
