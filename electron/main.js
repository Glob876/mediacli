'use strict';
// MediaCLI Electron shell: spawns `mediacli daemon --port 0`, reads
// MEDIACLI_DAEMON_PORT from its stdout and opens the renderer against it.
// Dev/mock UI without Go: `npm run dev` (=> --mock, no daemon spawned).
const { app, BrowserWindow, dialog, Menu, ipcMain } = require('electron');
const { spawn } = require('node:child_process');
const crypto = require('node:crypto');
const path = require('node:path');

const MOCK = process.argv.includes('--mock');
let daemonProc = null;

// Ответ renderer на вопрос об активных задачах (одноразовый).
let activeReportResolver = null;
ipcMain.on('mediacli:active-report', (_ev, payload) => {
  if (activeReportResolver) {
    const n = payload && Number.isFinite(payload.count) ? payload.count : -1;
    const lang = payload && typeof payload.lang === 'string' ? payload.lang : 'ru';
    activeReportResolver({ count: n, lang });
    activeReportResolver = null;
  }
});

// Спрашивает renderer: сколько активных загрузок/конвертаций.
// count: 0 — тихо закрываем; >0 — диалог; -1 — renderer молчит, тоже диалог.
function queryActiveWork(win, timeoutMs = 2000) {
  return new Promise((resolve) => {
    activeReportResolver = resolve;
    try {
      win.webContents.send('mediacli:query-active');
    } catch {
      activeReportResolver = null;
      resolve({ count: -1, lang: 'ru' });
      return;
    }
    setTimeout(() => {
      if (activeReportResolver) {
        activeReportResolver = null;
        resolve({ count: -1, lang: 'ru' });
      }
    }, timeoutMs);
  });
}

function confirmCloseText(lang, count) {
  const en = lang !== 'ru';
  if (count > 0) {
    return {
      message: en ? 'Are you sure?' : 'Вы уверены?',
      detail: en
        ? `Active tasks: ${count}. Unfinished downloads and conversions will be interrupted.`
        : `Активных задач: ${count}. Незавершённые загрузки и конвертации будут прерваны.`,
      buttons: en ? ['Stay', 'Quit MediaCLI'] : ['Остаться', 'Закрыть программу'],
    };
  }
  return {
    message: en ? 'Are you sure?' : 'Вы уверены?',
    detail: en
      ? 'Could not check active tasks. Unfinished downloads and conversions may be interrupted.'
      : 'Не удалось проверить активные задачи. Незавершённые загрузки и конвертации могут быть прерваны.',
    buttons: en ? ['Stay', 'Quit MediaCLI'] : ['Остаться', 'Закрыть программу'],
  };
}

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

  // Перехватываем Ctrl+Shift+I (обычно DevTools) — открываем настройки пресета.
  win.webContents.on('before-input-event', (event, input) => {
    const isI = (input.code === 'KeyI') || (typeof input.key === 'string' && input.key.toLowerCase() === 'i');
    if (input.control && input.shift && isI && !input.alt && !input.meta) {
      event.preventDefault();
      try { win.webContents.send('mediacli:open-preset-settings'); } catch { /* ignore */ }
    }
  });

  // Guard закрытия: пока есть активные загрузки/конвертации — не даём
  // закрыть окно молча, показываем «Вы уверены?».
  let quitting = false;
  win.on('close', async (e) => {
    if (quitting) return;
    e.preventDefault();
    let rep = { count: -1, lang: 'ru' };
    try {
      rep = await queryActiveWork(win);
    } catch {
      rep = { count: -1, lang: 'ru' };
    }
    if (rep.count === 0) {
      quitting = true;
      app.quit();
      return;
    }
    const txt = confirmCloseText(rep.lang, rep.count);
    const { response } = await dialog.showMessageBox(win, {
      type: 'question',
      title: 'MediaCLI',
      message: txt.message,
      detail: txt.detail,
      buttons: txt.buttons,
      defaultId: 0,
      cancelId: 0,
      noLink: true,
    });
    if (response === 1) {
      quitting = true;
      app.quit();
    }
  });

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
