'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { mkdtempSync, writeFileSync, readFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { name, version, optionalDependencies } = require('../package.json');

// Stub the npm executable: these tests must never contact npm or publish.
function runPublisher(t, mode) {
  const dir = mkdtempSync(path.join(tmpdir(), 'mediacli-publish-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const log = path.join(dir, 'calls.jsonl');
  writeFileSync(path.join(dir, 'npm'), `#!/usr/bin/env node
const fs = require('node:fs');
const args = process.argv.slice(2);
fs.appendFileSync(process.env.TEST_NPM_LOG, JSON.stringify(args) + '\\n');
if (args[0] === 'view') {
  if (process.env.TEST_NPM_MODE === 'existing') console.log(JSON.stringify(${JSON.stringify(version)}));
  else {
    console.log(JSON.stringify({error: {code: process.env.TEST_NPM_MODE === 'registry-error' ? 'E503' : 'E404'}}));
    process.exitCode = 1;
  }
} else if (process.env.TEST_NPM_MODE === 'publish-error') process.exitCode = 1;
`, { mode: 0o755 });
  const result = spawnSync(process.execPath, [path.join(__dirname, 'publish-npm.cjs')], {
    env: { ...process.env, PATH: `${dir}${path.delimiter}${process.env.PATH}`, TEST_NPM_MODE: mode, TEST_NPM_LOG: log },
    encoding: 'utf8',
  });
  const calls = readFileSync(log, 'utf8').trim().split('\n').map(line => JSON.parse(line));
  return { ...result, calls };
}

test('a missing version publishes the exact built archive with provenance', t => {
  const result = runPublisher(t, 'missing');
  assert.equal(result.status, 0, result.stderr);
  const packages = [...Object.keys(optionalDependencies), name];
  assert.equal(result.calls.length, packages.length * 2);
  for (const [index, packageName] of packages.entries()) {
    assert.deepEqual(result.calls[index * 2 + 1], ['publish', `./dist/${packageName.replace('@', '').replace('/', '-')}-${version}.tgz`, '--access=public', '--provenance', '--registry=https://registry.npmjs.org/']);
  }
});

test('an existing version is skipped without publishing', t => {
  const result = runPublisher(t, 'existing');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.calls.length, Object.keys(optionalDependencies).length + 1);
  assert.match(result.stdout, /already published/);
});

test('registry failures do not cause an attempted publish', t => {
  const result = runPublisher(t, 'registry-error');
  assert.notEqual(result.status, 0);
  assert.equal(result.calls.length, 1);
});

test('publish failures fail the release', t => {
  const result = runPublisher(t, 'publish-error');
  assert.equal(result.status, 1);
  assert.equal(result.calls.length, 2);
});
