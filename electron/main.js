'use strict';
// MediaCLI Electron shell: spawns `mediacli daemon --port 0`, reads
// MEDIACLI_DAEMON_PORT from its stdout and opens the renderer against it.
// Dev/mock UI without Go: `npm run dev` (=> --mock, no daemon spawned).
const { app, BrowserWindow, dialog, Menu } = require('electron');
const { spawn } = require('node:child_process');
const crypto = require('node:crypto');
const path = require('node:path');

const MOCK = process.argv.includes('--mock');
let daemonProc = null;

function daemonBinary() {
  if (process.env.MEDIACLI_BIN) return process.env.MEDIACLI_BIN;
  const exe = process.platform === 'win32' ? 'mediacli.exe' : 'mediacli';
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'bin', exe);
  }
  return path.join(__dirname, '..', 'dist', exe);
}

function waitForPort(proc, timeoutMs = 15000) {
  return new Promise((resolve, reject) => {
    let buf = '';
    const timer = setTimeout(() => reject(new Error('daemon port timeout')), timeoutMs);
    proc.stdout.on('data', (chunk) => {
      buf += chunk.toString();
      const m = buf.match(/MEDIACLI_DAEMON_PORT=(\d+)/);
      if (m) {
        clearTimeout(timer);
        resolve(parseInt(m[1], 10));
      }
    });
    proc.stderr.on('data', (chunk) => process.stderr.write(`[daemon] ${chunk}`));
    proc.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`daemon exited early with code ${code}`));
    });
  });
}

async function createWindow() {
  // Верхняя системная панель (Файл/Правка/Вид...) не нужна — убираем полностью.
  Menu.setApplicationMenu(null);
  const win = new BrowserWindow({
    width: 1140,
    height: 750,
    minWidth: 900,
    minHeight: 600,
    title: 'MediaCLI',
    backgroundColor: '#000000',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });
  win.setMenuBarVisibility(false);

  if (MOCK) {
    await win.loadFile(path.join(__dirname, 'renderer', 'index.html'), { query: { mock: '1' } });
    return;
  }

  const token = crypto.randomBytes(24).toString('hex');
  daemonProc = spawn(daemonBinary(), ['daemon', '--port', '0'], {
    env: { ...process.env, MEDIACLI_TOKEN: token },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  daemonProc.on('error', (err) => {
    dialog.showErrorBox('MediaCLI', `Cannot start daemon binary:\n${err.message}`);
    app.quit();
  });

  try {
    const port = await waitForPort(daemonProc);
    await win.loadFile(path.join(__dirname, 'renderer', 'index.html'), {
      query: { port: String(port), token },
    });
  } catch (err) {
    dialog.showErrorBox('MediaCLI', `Daemon failed to start:\n${err.message}`);
    app.quit();
  }
}

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});

app.on('before-quit', () => {
  if (daemonProc && !daemonProc.killed) daemonProc.kill();
});
