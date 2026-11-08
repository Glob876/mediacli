#!/usr/bin/env node
'use strict';

const { spawn } = require('node:child_process');
const { installedBinaryPath } = require('./platforms.cjs');

try {
  const child = spawn(installedBinaryPath(), process.argv.slice(2), { stdio: 'inherit' });
  // Keep the wrapper alive when the terminal sends signals to the process group.
  for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal));
  child.on('error', (error) => {
    console.error(`MediaCLI: could not start the bundled binary: ${error.message}`);
    process.exitCode = 1;
  });
  child.on('exit', (code, signal) => {
    process.exitCode = code ?? (signal === 'SIGINT' ? 130 : 143);
  });
} catch (error) {
  console.error(`MediaCLI: ${error.message}`);
  process.exitCode = 1;
}
