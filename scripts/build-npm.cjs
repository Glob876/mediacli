'use strict';
const { spawnSync } = require('node:child_process');
const { mkdirSync, copyFileSync, chmodSync, writeFileSync } = require('node:fs');
const path = require('node:path');
const { targets, binaryPath } = require('../npm/platforms.cjs');
const root = path.resolve(__dirname, '..');
const { sourceHash, binaryHash } = require('./build-info.cjs');
const { version } = require('../package.json');
const requested = process.argv[2];
const selected = requested ? targets.filter(([, , target]) => target === requested) : targets.filter(([platform, arch]) => platform === process.platform && arch === process.arch);
if (!selected.length) throw new Error(`Unsupported Rust build target: ${requested || process.platform + '/' + process.arch}`);
for (const [platform, arch, target] of selected) {
  const output = binaryPath(platform, arch);
  console.log(`Building ${platform}/${arch} (${target})...`);
  const result = spawnSync('cargo', ['build', '--release', '--locked', '--target', target], { cwd: root, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  mkdirSync(path.dirname(output), { recursive: true });
  copyFileSync(path.join(root, 'target', target, 'release', platform === 'win32' ? 'mediacli.exe' : 'mediacli'), output);
  if (platform !== 'win32') chmodSync(output, 0o755);
  copyFileSync(path.join(root, 'LICENSE'), path.join(path.dirname(output), 'LICENSE'));
  writeFileSync(path.join(path.dirname(output), 'package.json'), JSON.stringify({ name: `@glob876/mediacli-${platform}-${arch}`, version, description: `MediaCLI native tools for ${platform}/${arch}`, license: 'MIT AND GPL-3.0-or-later', os: [platform], cpu: [arch], repository: { type: 'git', url: 'git+https://github.com/Glob876/mediacli.git' }, publishConfig: { access: 'public' } }, null, 2) + '\n');
  const tools = spawnSync(process.platform === 'win32' ? 'python' : 'python3', [path.join(root, 'scripts/bundle-tools.py'), platform, arch, path.dirname(output)], { cwd: root, stdio: 'inherit' });
  if (tools.error) throw tools.error;
  if (tools.status !== 0) process.exit(tools.status ?? 1);
  if (platform === process.platform && arch === process.arch) {
    for (const tool of ['ffmpeg', 'ffprobe', 'deno']) {
      const executable = path.join(path.dirname(output), tool + (platform === 'win32' ? '.exe' : ''));
      const result = spawnSync(executable, [tool === 'deno' ? '--version' : '-version'], { encoding: 'utf8', timeout: 20000 });
      if (result.error) throw result.error;
      if (result.status !== 0) throw new Error(`Bundled ${tool} failed to start: ${result.stderr}`);
      console.log(result.stdout.split('\n')[0]);
    }
  }
  writeFileSync(path.join(path.dirname(output), 'build.json'), JSON.stringify({ version, target, source: sourceHash(), binary: binaryHash(output) }) + '\n');
}
