'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { mkdtempSync, mkdirSync, copyFileSync, writeFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { targets } = require('../npm/platforms.cjs');
function fixture(t) {
  const root = mkdtempSync(path.join(tmpdir(), 'mediacli-verify-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['scripts', 'npm', 'src', 'resources']) mkdirSync(path.join(root, directory));
  for (const name of ['build-info.cjs', 'verify-npm.cjs']) copyFileSync(path.join(__dirname, name), path.join(root, 'scripts', name));
  copyFileSync(path.join(__dirname, '../npm/platforms.cjs'), path.join(root, 'npm/platforms.cjs'));
  writeFileSync(path.join(root, 'Cargo.toml'), '[package]\nname="test"\nversion="0.1.2"\n');
  writeFileSync(path.join(root, 'Cargo.lock'), '# test lock\n');
  writeFileSync(path.join(root, 'src/main.rs'), 'fn main() {}\n');
  writeFileSync(path.join(root, 'package.json'), '{"version":"0.1.2"}');
  const { sourceHash, binaryHash } = require(path.join(root, 'scripts/build-info.cjs'));
  const binaries = [];
  for (const [platform, arch, target] of targets) {
    const directory = path.join(root, 'npm/bin', `${platform}-${arch}`);
    mkdirSync(directory, { recursive: true });
    const binary = path.join(directory, platform === 'win32' ? 'mediacli.exe' : 'mediacli');
    writeFileSync(binary, `fixture ${target}`, { mode: 0o755 });
    writeFileSync(path.join(directory, 'build.json'), JSON.stringify({ version: '0.1.2', target, source: sourceHash(), binary: binaryHash(binary) }));
    binaries.push(binary);
  }
  return { root, binaries, verify: () => spawnSync(process.execPath, [path.join(root, 'scripts/verify-npm.cjs')], { encoding: 'utf8' }) };
}
test('packaging accepts six binaries from one Rust source snapshot', t => {
  const built = fixture(t);
  const result = built.verify();
  assert.equal(result.status, 0, result.stderr);
});
test('packaging rejects a cached binary changed after its build', t => {
  const built = fixture(t);
  writeFileSync(built.binaries[0], 'old Go binary');
  const result = built.verify();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Stale or mismatched/);
});
test('packaging rejects binaries from a previous Rust source revision', t => {
  const built = fixture(t);
  writeFileSync(path.join(built.root, 'src/main.rs'), 'fn main() { println!("new version"); }\n');
  const result = built.verify();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Stale or mismatched/);
});
