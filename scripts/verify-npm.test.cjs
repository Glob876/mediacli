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
  for (const directory of ['scripts', 'npm', 'src', 'resources', 'packaging']) mkdirSync(path.join(root, directory));
  for (const name of ['build-info.cjs', 'verify-npm.cjs']) copyFileSync(path.join(__dirname, name), path.join(root, 'scripts', name));
  copyFileSync(path.join(__dirname, '../packaging/tools.lock.json'), path.join(root, 'packaging/tools.lock.json'));
  const toolLock = require('../packaging/tools.lock.json');
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
    const files = {};
    for (const name of ['ffmpeg', 'ffprobe', 'deno']) {
      const filename = name + (platform === 'win32' ? '.exe' : '');
      const tool = path.join(directory, filename);
      writeFileSync(tool, `fixture ${name} ${target}`, { mode: 0o755 });
      files[filename] = binaryHash(tool);
    }
    writeFileSync(path.join(directory, 'tools.json'), JSON.stringify({ platform: `${platform}-${arch}`, files, sources: toolLock[`${platform}-${arch}`] }));
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

test('packaging rejects missing FFmpeg even when the Rust binary is valid', t => {
  const built = fixture(t);
  rmSync(path.join(path.dirname(built.binaries[0]), 'ffmpeg'));
  assert.notEqual(built.verify().status, 0);
});
test('packaging rejects a modified bundled tool', t => {
  const built = fixture(t);
  writeFileSync(path.join(path.dirname(built.binaries[0]), 'deno'), 'corrupt');
  assert.match(built.verify().stderr, /Missing or corrupt bundled tool/);
});

test('packaging rejects tools from an older pinned release', t => {
  const built = fixture(t);
  const file = path.join(path.dirname(built.binaries[0]), 'tools.json');
  const receipt = JSON.parse(require('node:fs').readFileSync(file, 'utf8'));
  receipt.sources.ffmpeg_release = 'old-release';
  writeFileSync(file, JSON.stringify(receipt));
  assert.match(built.verify().stderr, /do not match the pinned lock/);
});
