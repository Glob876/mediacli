'use strict';

const { statSync, readFileSync } = require('node:fs');
const path = require('node:path');
const { sourceHash, binaryHash } = require('./build-info.cjs');
const { version } = require('../package.json');
const expectedSource = sourceHash();
const toolLock = require('../packaging/tools.lock.json');
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
  const directory = path.dirname(file);
  let tools;
  try { tools = JSON.parse(readFileSync(path.join(directory, 'tools.json'), 'utf8')); } catch {
    throw new Error(`Missing bundled tool receipt for ${platform}/${arch}`);
  }
  if (JSON.stringify(tools.sources) !== JSON.stringify(toolLock[`${platform}-${arch}`])) throw new Error('Bundled tool sources do not match the pinned lock file');
  if (tools.platform !== `${platform}-${arch}`) throw new Error('Bundled tool platform mismatch');
  for (const name of ['ffmpeg', 'ffprobe', 'deno']) {
    const tool = name + (platform === 'win32' ? '.exe' : '');
    const toolPath = path.join(directory, tool);
    if (!tools.files?.[tool] || binaryHash(toolPath) !== tools.files[tool]) throw new Error(`Missing or corrupt bundled tool: ${toolPath}`);
    if (platform !== 'win32' && !(statSync(toolPath).mode & 0o111)) throw new Error(`Tool is not executable: ${toolPath}`);
  }
  for (const [relative, checksum] of Object.entries(tools.files)) {
    if (relative.split('/').includes('..') || path.isAbsolute(relative)) throw new Error('Invalid tool receipt path');
    if (binaryHash(path.join(directory, relative)) !== checksum) throw new Error(`Bundled file checksum mismatch: ${relative}`);
  }
  if (Object.keys(tools.files).some(name => /yt-dlp/i.test(name))) throw new Error('yt-dlp must not be bundled');

}
