'use strict';

const { statSync } = require('node:fs');
const { targets, binaryPath } = require('../npm/platforms.cjs');

for (const [platform, arch] of targets) {
  const file = binaryPath(platform, arch);
  let stat;
  try { stat = statSync(file); } catch {
    throw new Error(`Missing ${platform}/${arch} binary. Run npm run build before npm pack.`);
  }
  if (!stat.isFile() || stat.size === 0) throw new Error(`Invalid binary: ${file}`);
  if (platform !== 'win32' && !(stat.mode & 0o111)) throw new Error(`Binary is not executable: ${file}`);
}
