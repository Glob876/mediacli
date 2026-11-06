'use strict';
const { createHash } = require('node:crypto');
const { readFileSync, readdirSync } = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
function sourceHash() {
  const files = ['Cargo.toml', 'Cargo.lock'];
  function collect(directory) {
    for (const entry of readdirSync(path.join(root, directory), { withFileTypes: true })) {
      const relative = `${directory}/${entry.name}`;
      if (entry.isDirectory()) collect(relative);
      else if (entry.isFile()) files.push(relative);
    }
  }
  collect('src');
  collect('resources');
  const hash = createHash('sha256');
  for (const file of files.sort()) {
    hash.update(file);
    hash.update('\0');
    // Git may check out text using CRLF on Windows runners.
    hash.update(readFileSync(path.join(root, file), 'utf8').replace(/\r\n/g, '\n'));
    hash.update('\0');
  }
  return hash.digest('hex');
}
function binaryHash(file) { return createHash('sha256').update(readFileSync(file)).digest('hex'); }
module.exports = { sourceHash, binaryHash };
