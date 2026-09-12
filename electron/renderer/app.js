'use strict';
/* Renderer: talks to the Go daemon (or a mock with ?mock=1 for UI dev). */
const qs = new URLSearchParams(location.search);
const MOCK = qs.get('mock') === '1';
const BASE = MOCK ? null : `http://127.0.0.1:${qs.get('port')}`;
const TOKEN = MOCK ? null : qs.get('token');

const el = (id) => document.getElementById(id);
const cardsEl = el('cards');

/* ---------------- API layer ---------------- */
const api = {
  async req(method, path, body) {
    const res = await fetch(BASE + path, {
      method,
      headers: {
        Authorization: `Bearer ${TOKEN}`,
        ...(body ? { 'Content-Type': 'application/json' } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const txt = await res.text().catch(() => '');
      throw new Error(`${method} ${path}: ${res.status} ${txt}`);
    }
    return res.json();
  },
  presets: () => api.req('GET', '/api/presets'),
  convertPresets: () => api.req('GET', '/api/convert/presets'),
  convert: (payload) => api.req('POST', '/api/convert', payload),
  library: () => api.req('GET', '/api/library'),
  browse: (path) => api.req('GET', '/api/browse?path=' + encodeURIComponent(path || '')),
  tasks: () => api.req('GET', '/api/tasks'),
  download: (payload) => api.req('POST', '/api/downloads', payload),
  cancel: (id) => api.req('POST', `/api/tasks/${id}/cancel`),
  history: () => api.req('GET', '/api/history'),
  historyDelete: (entry, deleteFile) => api.req('POST', '/api/history/delete', { ...entry, delete_file: deleteFile }),
  historyClear: () => api.req('DELETE', '/api/history'),
  config: () => api.req('GET', '/api/config'),
  saveConfig: (cfg) => api.req('PUT', '/api/config', cfg),
  status: () => api.req('GET', '/api/status'),
  eventsUrl: (id) => `${BASE}/api/tasks/${id}/events?token=${encodeURIComponent(TOKEN)}`,
  fileUrl: (name) => `${BASE}/api/library/file?name=${encodeURIComponent(name)}&token=${encodeURIComponent(TOKEN)}`,
};

/* Mock for `npm run dev`: fake tasks with timer-driven progress. */
const mockApi = {
  _id: 0,
  _tasks: [],
  _listeners: {},
  _history: [{ time: '2026-01-01 10:00:00', type: 'Download', source: 'https://example/v', target: 'video.mp4', status: 'Success' }],
  _config: {
    download_dir: '/tmp/MediaCLI', language: 'en', video_preset: 'default',
    audio_format: 'mp3', sub_langs: 'ru,en', proxy_mode: 'system', proxy_url: '',
    concurrent_fragments: 4, bg_queue_max: 3, no_mtime: true, windows_filenames: true, use_archive: false,
  },
  _library: [
    { name: 'demo-video.mp4', size: 12345678, mtime: '2026-09-01 12:00:00', is_media: true },
    { name: 'demo-audio.mp3', size: 4567890, mtime: '2026-09-02 12:00:00', is_media: true },
    { name: 'notes.txt', size: 123, mtime: '2026-09-03 12:00:00', is_media: false },
  ],
  _convertPresets: [
    { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)', name_ru: 'Стандартный MP4', desc_en: '', desc_ru: '', ext: 'mp4', suffix: '_mp4', flags: [] },
    { id: 'audio_mp3', name_en: 'Extract Audio MP3', name_ru: 'Аудио MP3', desc_en: '', desc_ru: '', ext: 'mp3', suffix: '_audio', flags: [] },
  ],
  _browsePath: '/tmp/MediaCLI',
  async presets() {
    return {
      video_presets: [
        { id: 'default', name_en: 'Original / Lossless Merge — Best Quality' },
        { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)' },
        { id: 'mkv_av1', name_en: 'Modern MKV (AV1 + Opus/AAC)' },
      ],
    };
  },
  async convertPresets() { return { convert_presets: this._convertPresets }; },
  async library() { return { dir: '/tmp/MediaCLI', files: this._library }; },
  async browse() {
    return {
      path: this._browsePath, parent: '/tmp',
      dirs: ['subdir'],
      files: this._library.map((f) => ({ ...f })),
    };
  },
  async convert(payload) {
    const task = {
      id: ++this._id, title: 'Convert: ' + payload.input, source: payload.input,
      target: payload.output || (payload.input + '.out'),
      status: 'running', stage: '[Convert] 0.0%', progress: 0,
    };
    this._tasks.unshift(task);
    const timer = setInterval(() => {
      task.progress = Math.min(100, task.progress + 9);
      task.stage = `[Convert] ${task.progress.toFixed(1)}%`;
      (this._listeners[task.id] || []).forEach((cb) => cb({ ...task }));
      if (task.progress >= 100) {
        clearInterval(timer);
        task.status = 'done';
        task.stage = 'Completed successfully';
        (this._listeners[task.id] || []).forEach((cb) => cb({ ...task }));
      }
    }, 300);
    return { task_id: task.id, output: task.target };
  },
  async tasks() { return { tasks: this._tasks }; },
  async download(payload) {
    const task = {
      id: ++this._id, title: payload.url, source: payload.url,
      status: 'running', stage: '[Download] 0.0%', progress: 0,
    };
    this._tasks.unshift(task);
    const timer = setInterval(() => {
      task.progress = Math.min(100, task.progress + 7);
      task.stage = `[Download] ${task.progress.toFixed(1)}%`;
      (this._listeners[task.id] || []).forEach((cb) => cb({ ...task }));
      if (task.progress >= 100) {
        clearInterval(timer);
        task.status = 'done';
        task.stage = 'Completed successfully';
        (this._listeners[task.id] || []).forEach((cb) => cb({ ...task }));
      }
    }, 300);
    return { task_id: task.id };
  },
  async cancel(id) {
    const t = this._tasks.find((x) => x.id === id);
    if (!t) throw new Error('not found');
    t.status = 'cancelled';
    return { ok: true };
  },
  async history() { return { history: this._history }; },
  async historyDelete(entry) {
    this._history = this._history.filter((h) => h.time !== entry.time);
    return { ok: true };
  },
  async historyClear() { this._history = []; return { ok: true }; },
  async config() { return { ...this._config }; },
  async saveConfig(cfg) { this._config = { ...cfg }; return { ...this._config }; },
  async status() {
    return {
      ok: true, version: 'mock',
      dependencies: [
        { name: 'yt-dlp', available: true, required: true },
        { name: 'ffmpeg', available: true, required: true },
        { name: 'deno', available: false, required: false },
      ],
    };
  },
  eventsUrl: (id) => `mock:${id}`,
  fileUrl: (name) => '#',
};
if (MOCK) {
  mockApi.subscribe = (id, cb) => {
    (mockApi._listeners[id] = mockApi._listeners[id] || []).push(cb);
  };
}

const backend = MOCK ? mockApi : api;

/* ---------------- Tabs (плавное переключение) ---------------- */
function switchTab(key) {
  document.querySelectorAll('.tab').forEach((b) => b.classList.toggle('active', b.dataset.tab === key));
  document.querySelectorAll('.tabpage').forEach((p) => {
    const show = p.id === `tab-${key}`;
    p.classList.toggle('hidden', !show);
    if (show) {
      // Перезапуск анимации переключения вкладок.
      p.classList.remove('page-enter');
      void p.offsetWidth;
      p.classList.add('page-enter');
    }
  });
  if (key === 'home') loadHome();
  if (key === 'downloads') refreshTasks();
  if (key === 'library') loadLibrary();
  if (key === 'convert') { loadConvertPresets(); loadBrowse(currentBrowsePath); }
  if (key === 'settings') { loadSettings(); loadHistory(); }
  if (key === 'doctor') loadDoctor();
}
document.querySelectorAll('.tab').forEach((btn) => {
  btn.onclick = () => switchTab(btn.dataset.tab);
});
document.querySelectorAll('[data-goto]').forEach((btn) => {
  btn.onclick = () => switchTab(btn.dataset.goto);
});

/* ---------------- Downloads ---------------- */
function cardShell(task) {
  const div = document.createElement('div');
  div.className = 'dl-card active';
  div.id = `task-${task.id}`;
  div.innerHTML = `
    <div class="dl-title"></div>
    <div class="dl-stage"></div>
    <progress max="100" value="0"></progress>
    <div class="dl-meta"><span class="st"></span><span class="pc"></span></div>
    <div class="row"><button class="btn small btn-cancel-task">Отмена</button></div>`;
  div.querySelector('.btn-cancel-task').onclick = async () => {
    try { await backend.cancel(task.id); } catch (e) { alert(e.message); }
  };
  cardsEl.prepend(div);
  return div;
}

function paint(id, snap) {
  let div = el(`task-${id}`);
  if (!div) div = cardShell(snap);
  div.querySelector('.dl-title').textContent = snap.title || snap.source || `Task #${id}`;
  div.querySelector('.dl-stage').textContent = snap.stage || snap.status;
  div.querySelector('progress').value = snap.progress || 0;
  div.querySelector('.st').textContent = snap.status;
  div.querySelector('.pc').textContent = `${(snap.progress || 0).toFixed(1)}%`;
  const live = snap.status === 'running' || snap.status === 'queued';
  div.classList.toggle('active', live);
  div.classList.toggle('failed', String(snap.status).startsWith('fail'));
  const btn = div.querySelector('.btn-cancel-task');
  if (btn) btn.style.display = live ? '' : 'none';
}

function subscribe(id, onSnap) {
  const paintBoth = (snap) => { paint(id, snap); if (onSnap) onSnap(snap); };
  if (MOCK) {
    mockApi.subscribe(id, paintBoth);
    return;
  }
  const es = new EventSource(backend.eventsUrl(id));
  es.onmessage = (ev) => {
    try {
      const snap = JSON.parse(ev.data);
      paintBoth(snap);
      if (['done', 'failed', 'cancelled'].includes(snap.status)) es.close();
    } catch { /* keep-alive */ }
  };
  es.onerror = async () => {
    // SSE fallback: poll once per second.
    es.close();
    const timer = setInterval(async () => {
      try {
        const snap = await backend.req('GET', `/api/tasks/${id}`);
        paintBoth(snap);
        if (['done', 'failed', 'cancelled'].includes(snap.status)) clearInterval(timer);
      } catch { clearInterval(timer); }
    }, 1000);
  };
}

async function refreshTasks() {
  try {
    const { tasks } = await backend.tasks();
    tasks.forEach((t) => {
      paint(t.id, t);
      if (t.status === 'running' || t.status === 'queued') subscribe(t.id);
    });
  } catch { /* fresh start, nothing to restore */ }
}

/* ---------------- Home (главное меню) ---------------- */
function fmtSize(n) {
  if (n == null) return '';
  if (n < 1024) return `${n} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`;
  return `${(n / 1073741824).toFixed(2)} GB`;
}

async function loadHome() {
  try {
    const [{ tasks }, lib, st] = await Promise.all([
      backend.tasks().catch(() => ({ tasks: [] })),
      backend.library().catch(() => ({ files: [] })),
      backend.status().catch(() => ({ version: '?' })),
    ]);
    el('st-tasks').textContent = String((tasks || []).filter((t) => t.status === 'running' || t.status === 'queued').length);
    el('st-files').textContent = String((lib.files || []).length);
    el('st-daemon').textContent = st.version || 'ok';
  } catch { /* ignore */ }
}

/* ---------------- Library (предпросмотр всех скачанных видео) ---------------- */
function isVideo(name) { return /\.(mp4|mkv|mov|webm|avi|m4v)$/i.test(name); }
function isAudio(name) { return /\.(mp3|flac|wav|m4a|opus|ogg)$/i.test(name); }

async function loadLibrary() {
  const box = el('library');
  box.innerHTML = '<div class="muted">Загрузка…</div>';
  try {
    const { dir, files } = await backend.library();
    el('lib-dir').textContent = dir || '';
    if (!files.length) { box.innerHTML = '<div class="muted">Папка пуста — скачай что-нибудь во вкладке «Загрузки».</div>'; return; }
    box.innerHTML = '';
    files.forEach((f) => {
      const card = document.createElement('div');
      card.className = 'dl-card';
      const title = document.createElement('div');
      title.className = 'dl-title';
      title.textContent = f.name;
      title.title = f.name;
      card.appendChild(title);
      if (!MOCK && f.is_media !== false && (isVideo(f.name) || isAudio(f.name))) {
        let media;
        if (isVideo(f.name)) {
          media = document.createElement('video');
          media.className = 'dl-thumb';
          media.preload = 'metadata';
          media.controls = true;
          media.src = backend.fileUrl(f.name);
        } else {
          media = document.createElement('audio');
          media.className = 'dl-thumb';
          media.preload = 'metadata';
          media.controls = true;
          media.src = backend.fileUrl(f.name);
        }
        card.appendChild(media);
      } else {
        const ph = document.createElement('div');
        ph.className = 'dl-thumb';
        ph.style.display = 'flex';
        ph.style.alignItems = 'center';
        ph.style.justifyContent = 'center';
        ph.style.color = 'var(--lime)';
        ph.style.fontSize = '28px';
        ph.textContent = isAudio(f.name) ? '♪' : '▣';
        card.appendChild(ph);
      }
      const meta = document.createElement('div');
      meta.className = 'dl-meta';
      meta.innerHTML = '<span></span><span></span>';
      meta.children[0].textContent = f.mtime || '';
      meta.children[1].textContent = fmtSize(f.size);
      card.appendChild(meta);
      box.appendChild(card);
    });
  } catch (e) { box.innerHTML = `<div class="muted">Ошибка: ${e.message}</div>`; }
}

/* ---------------- Convert (полноценная вкладка с обзором) ---------------- */
let currentBrowsePath = '';
let convertPresetsCache = [];

async function loadConvertPresets() {
  try {
    const data = await backend.convertPresets();
    convertPresetsCache = data.convert_presets || [];
    const sel = el('conv-preset');
    if (!sel.options.length && convertPresetsCache.length) {
      convertPresetsCache.forEach((p) => {
        const opt = document.createElement('option');
        opt.value = p.id;
        opt.textContent = p.name_ru || p.name_en;
        sel.appendChild(opt);
      });
      updateConvertDesc();
    }
  } catch (e) {
    el('conv-preset-desc').textContent = `Пресеты недоступны: ${e.message}`;
  }
}

function updateConvertDesc() {
  const p = convertPresetsCache.find((x) => x.id === el('conv-preset').value);
  el('conv-preset-desc').textContent = p
    ? `${p.desc_ru || p.desc_en || ''} → .${p.ext} (суффикс ${p.suffix})`.trim()
    : '';
}

async function loadBrowse(path) {
  const dirsBox = el('browse-dirs');
  const filesBox = el('browse-files');
  try {
    const data = await backend.browse(path || '');
    currentBrowsePath = data.path || data.dir || '';
    el('browse-path').textContent = currentBrowsePath;
    dirsBox.innerHTML = '';
    filesBox.innerHTML = '';
    (data.dirs || []).forEach((d) => {
      const row = document.createElement('div');
      row.className = 'brow';
      row.innerHTML = `<span>📁 ${d}</span>`;
      row.onclick = () => loadBrowse(currentBrowsePath + '/' + d);
      dirsBox.appendChild(row);
    });
    (data.files || []).forEach((f) => {
      const row = document.createElement('div');
      row.className = 'brow';
      if (el('conv-input').value.endsWith(f.name)) row.classList.add('selected');
      row.innerHTML = `<span>${f.is_media ? '🎬' : '📄'} ${f.name}</span><span class="sz">${fmtSize(f.size)}</span>`;
      row.title = `${f.name} • ${f.mtime || ''} • ${fmtSize(f.size)}`;
      row.onclick = () => {
        const full = (currentBrowsePath ? currentBrowsePath + '/' : '') + f.name;
        el('conv-input').value = full;
        el('conv-fileinfo').textContent = `Выбрано: ${f.name} • ${fmtSize(f.size)} • ${f.mtime || ''}`;
        filesBox.querySelectorAll('.brow').forEach((b) => b.classList.remove('selected'));
        row.classList.add('selected');
      };
      row.ondblclick = () => {
        const full = (currentBrowsePath ? currentBrowsePath + '/' : '') + f.name;
        el('conv-input').value = full;
        el('conv-fileinfo').textContent = `Выбрано: ${f.name} • ${fmtSize(f.size)} • ${f.mtime || ''}`;
      };
      filesBox.appendChild(row);
    });
    if (!(data.dirs || []).length && !(data.files || []).length) {
      filesBox.innerHTML = '<div class="muted small">Пустая папка</div>';
    }
  } catch (e) {
    filesBox.innerHTML = `<div class="muted small">Ошибка обзора: ${e.message}</div>`;
  }
}

function paintConvertTask(id, snap) {
  let div = el(`conv-task-${id}`);
  const box = el('convert-tasks');
  if (!div) {
    div = document.createElement('div');
    div.className = 'hrow';
    div.id = `conv-task-${id}`;
    div.innerHTML = '<div><b></b><div class="muted small"></div><progress max="100" value="0" style="width:100%"></progress></div>';
    box.prepend(div);
  }
  div.querySelector('b').textContent = snap.title || `Task #${id}`;
  div.querySelector('.small').textContent = `${snap.stage || snap.status} • ${(snap.progress || 0).toFixed(1)}%`;
  div.querySelector('progress').value = snap.progress || 0;
}

/* ---------------- History ---------------- */
async function loadHistory() {
  const box = el('history');
  box.innerHTML = '<div class="muted">Загрузка…</div>';
  try {
    const { history } = await backend.history();
    if (!history.length) { box.innerHTML = '<div class="muted">Пока пусто.</div>'; return; }
    box.innerHTML = '';
    history.forEach((h) => {
      const row = document.createElement('div');
      row.className = 'hrow';
      row.innerHTML = `
        <div><b></b><div class="muted small"></div></div>
        <div class="row"><button class="btn small">Убрать</button><button class="btn small danger">Удалить файл</button></div>`;
      row.querySelector('b').textContent = h.target || h.source;
      row.querySelector('.small').textContent = `${h.time} • ${h.type} • ${h.status}`;
      const [btnRm, btnDel] = row.querySelectorAll('button');
      btnRm.onclick = async () => { await backend.historyDelete(h, false); loadHistory(); };
      btnDel.onclick = async () => {
        if (!confirm(`Удалить файл с диска?\n${h.target || h.source}`)) return;
        await backend.historyDelete(h, true); loadHistory();
      };
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted">Ошибка: ${e.message}</div>`; }
}

/* ---------------- Settings ---------------- */
let settingsCache = null;

async function loadSettings() {
  try {
    const cfg = await backend.config();
    settingsCache = cfg;
    el('set-download-dir').value = cfg.download_dir || '';
    el('set-language').value = cfg.language || 'en';
    el('set-audio-format').value = cfg.audio_format || 'mp3';
    el('set-sub-langs').value = cfg.sub_langs || '';
    el('set-proxy-mode').value = cfg.proxy_mode || 'system';
    el('set-proxy-url').value = cfg.proxy_url || '';
    el('set-fragments').value = String(cfg.concurrent_fragments || 4);
    el('set-queue-max').value = String(cfg.bg_queue_max || 3);
    el('set-no-mtime').checked = !!cfg.no_mtime;
    el('set-win-names').checked = !!cfg.windows_filenames;
    el('set-archive').checked = !!cfg.use_archive;
    const sel = el('set-video-preset');
    if (!sel.options.length) {
      const { video_presets } = await backend.presets();
      video_presets.forEach((p) => {
        const opt = document.createElement('option');
        opt.value = p.id;
        opt.textContent = p.name_en;
        sel.appendChild(opt);
      });
    }
    sel.value = cfg.video_preset || 'default';
  } catch (e) { alert(`Не удалось загрузить настройки: ${e.message}`); }
}

/* ---------------- Doctor ---------------- */
async function loadDoctor() {
  const box = el('doctor');
  box.innerHTML = '<div class="muted">Проверка…</div>';
  try {
    const st = await backend.status();
    box.innerHTML = `<div class="muted">daemon ${st.version}</div>`;
    st.dependencies.forEach((d) => {
      const row = document.createElement('div');
      row.className = 'hrow';
      const mark = d.available ? 'FOUND' : 'MISSING';
      row.innerHTML = `<div><b>${d.name}</b> <span class="muted small">${d.required ? 'required' : 'optional'}</span></div>
        <div class="muted small">${mark}${d.path ? ' (' + d.path + ')' : ''}</div>`;
      if (!d.available && d.required) row.classList.add('failed');
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted">Ошибка: ${e.message}</div>`; }
}

/* ---------------- Init ---------------- */
async function init() {
  try {
    const { video_presets } = await backend.presets();
    const sel = el('preset');
    video_presets.forEach((p, i) => {
      const opt = document.createElement('option');
      opt.value = p.id;
      opt.textContent = `${i}. ${p.name_en}`;
      sel.appendChild(opt);
    });
  } catch {
    el('preset').innerHTML = '<option>daemon недоступен</option>';
  }

  await refreshTasks();
  await loadHome();

  el('btn-go').onclick = () => {
    if (!el('url').value.trim()) return;
    el('confirm').classList.remove('hidden');
  };
  el('url').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && el('url').value.trim()) el('confirm').classList.remove('hidden');
  });
  el('btn-cancel').onclick = () => el('confirm').classList.add('hidden');

  el('btn-start').onclick = async () => {
    const url = el('url').value.trim();
    if (!url) return;
    const fields = { video_preset: el('preset').value || 'default' };
    if (el('quality').value) fields.quality = el('quality').value;
    if (el('timerange').value.trim()) fields.download_section = el('timerange').value.trim();
    if (el('subs').checked) { fields.subs_enabled = true; fields.embed_subs = true; }
    if (el('sponsor').checked) fields.sponsorblock = 'remove';
    el('btn-start').disabled = true;
    try {
      const { task_id } = await backend.download({ url, fields });
      paint(task_id, { id: task_id, title: url, source: url, status: 'running', stage: 'В очереди…', progress: 0 });
      subscribe(task_id);
      el('url').value = '';
      el('confirm').classList.add('hidden');
    } catch (e) {
      alert(`Не удалось начать загрузку:\n${e.message}`);
    } finally {
      el('btn-start').disabled = false;
    }
  };

  el('btn-library-refresh').onclick = loadLibrary;

  el('conv-preset').onchange = updateConvertDesc;
  el('btn-browse-up').onclick = async () => {
    try {
      const data = await backend.browse(currentBrowsePath);
      if (data.parent) loadBrowse(data.parent);
    } catch { /* ignore */ }
  };
  el('btn-browse-refresh').onclick = () => loadBrowse(currentBrowsePath);
  el('btn-convert-start').onclick = async () => {
    const input = el('conv-input').value.trim();
    const preset_id = el('conv-preset').value;
    const output = el('conv-output').value.trim();
    if (!input) { alert('Выбери входной файл слева или впиши путь.'); return; }
    if (!preset_id) { alert('Выбери профиль конвертации.'); return; }
    el('btn-convert-start').disabled = true;
    try {
      const { task_id } = await backend.convert({ input, preset_id, output });
      paintConvertTask(task_id, { id: task_id, title: `Convert: ${input}`, status: 'running', stage: 'В очереди…', progress: 0 });
      subscribe(task_id, (snap) => paintConvertTask(task_id, snap));
    } catch (e) {
      alert(`Конвертация не началась:\n${e.message}`);
    } finally {
      el('btn-convert-start').disabled = false;
    }
  };

  el('btn-history-refresh').onclick = loadHistory;
  el('btn-history-clear').onclick = async () => {
    if (!confirm('Очистить всю историю операций?')) return;
    await backend.historyClear();
    loadHistory();
  };
  el('btn-doctor-refresh').onclick = loadDoctor;
  el('btn-settings-save').onclick = async () => {
    if (!settingsCache) return;
    const cfg = {
      ...settingsCache,
      download_dir: el('set-download-dir').value.trim(),
      language: el('set-language').value,
      video_preset: el('set-video-preset').value,
      audio_format: el('set-audio-format').value,
      sub_langs: el('set-sub-langs').value.trim(),
      proxy_mode: el('set-proxy-mode').value,
      proxy_url: el('set-proxy-url').value.trim(),
      concurrent_fragments: parseInt(el('set-fragments').value, 10),
      bg_queue_max: parseInt(el('set-queue-max').value, 10),
      no_mtime: el('set-no-mtime').checked,
      windows_filenames: el('set-win-names').checked,
      use_archive: el('set-archive').checked,
    };
    try {
      settingsCache = await backend.saveConfig(cfg);
      alert('Настройки сохранены.');
    } catch (e) { alert(`Не удалось сохранить: ${e.message}`); }
  };
}

init();
