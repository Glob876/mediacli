'use strict';

const { spawnSync } = require('node:child_process');
const { name: rootName, version, optionalDependencies } = require('../package.json');
for (const name of [...Object.keys(optionalDependencies || {}), rootName]) {
const registry = 'https://registry.npmjs.org/';
const existing = spawnSync('npm', ['view', `${name}@${version}`, 'version', '--json', `--registry=${registry}`], { encoding: 'utf8' });
if (existing.error) throw existing.error;
if (existing.status === 0) {
  if (JSON.parse(existing.stdout) !== version) throw new Error('Unexpected npm registry response');
  console.log(`${name}@${version} is already published; skipping. Bump package.json for the next release.`);
} else {
  let response;
  try { response = JSON.parse(existing.stdout); } catch {
    throw new Error(`Cannot check npm registry: ${existing.stderr}`);
  }
  if (response.error?.code !== 'E404') throw new Error(`Cannot check npm registry: ${existing.stderr}`);
  const result = spawnSync('npm', ['publish', `./dist/${name.replace('@', '').replace('/', '-')}-${version}.tgz`, '--access=public', '--provenance', `--registry=${registry}`], { stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

}
