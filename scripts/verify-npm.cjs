'use strict';

const { statSync, readFileSync } = require('node:fs');
const path = require('node:path');
const { sourceHash, binaryHash } = require('./build-info.cjs');
const { version } = require('../package.json');
const expectedSource = sourceHash();
const { targets, binaryPath } = require('../npm/platforms.cjs');

for (const [platform, arch, target] of targets) {
  const file = binaryPath(platform, arch);
  let stat;
  try { stat = statSync(file); } catch {
    throw new Error(`Missing ${platform}/${arch} binary. Run npm run build before npm pack.`);
  }
  if (!stat.isFile() || stat.size === 0) throw new Error(`Invalid binary: ${file}`);
  let receipt;
  try { receipt = JSON.parse(readFileSync(path.join(path.dirname(file), 'build.json'), 'utf8')); } catch {
    throw new Error(`Missing Rust build receipt for ${platform}/${arch}. Rebuild this target.`);
  }
  if (receipt.version !== version || receipt.target !== target || receipt.source !== expectedSource || receipt.binary !== binaryHash(file)) {
    throw new Error(`Stale or mismatched ${platform}/${arch} binary. Rebuild all targets from this source before packing.`);
  }
  if (platform !== 'win32' && !(stat.mode & 0o111)) throw new Error(`Binary is not executable: ${file}`);
}
