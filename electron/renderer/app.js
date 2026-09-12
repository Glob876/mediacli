'use strict';
/* Renderer: talks to the Go daemon (or a mock with ?mock=1 for UI dev). */
const qs = new URLSearchParams(location.search);
const MOCK = qs.get('mock') === '1';
const BASE = MOCK ? null : `http://127.0.0.1:${qs.get('port')}`;
const TOKEN = MOCK ? null : qs.get('token');

const el = (id) => document.getElementById(id);
const cardsEl = el('cards');
const state = { lang: 'ru', accent: '#bfff00' };

/* ================= i18n (формальное «Вы»; en-US поддерживается) ================= */
const I18N = {
  ru: {
    tabHome: 'Главная', tabDownloads: 'Загрузки', tabLibrary: 'Видео', tabConvert: 'Конвертация', tabSettings: 'Настройки', tabDoctor: 'Система',
    urlPh: 'Вставьте ссылку (YouTube, VK, Twitch…) и нажмите Enter…',
    dlSettings: 'Настройки загрузки (пресеты)', download: 'Скачать',
    dlPreset: 'Пресет кодека:', dlQuality: 'Макс. качество:',
    qBest: 'Лучшее доступное (Max)', dlCut: 'Вырезка по времени (напр. 00:01:00-00:03:30, либо не указывайте):',
    dlSubs: 'Субтитры (ru,en)', dlSponsor: 'SponsorBlock (вырезать спонсорские сегменты)',
    dropHint: 'Перетащите файл в это окно, чтобы конвертировать его',
    dropSub: 'Файл будет открыт во вкладке «Конвертация»',
    stTasks: 'активных задач', stFiles: 'файлов в папке',
    shortcuts: 'Shift+H — история • Shift+I — настройки • Esc — закрыть',
    tasksHint: 'Активные и завершённые задачи загрузки', refresh: 'Обновить',
    tasksEmpty: 'Задач пока нет. Вставьте ссылку на главной и нажмите «Скачать».',
    cancel: 'Отмена', queued: 'В очереди…',
    libEmpty: 'Папка пуста. Скачайте что-нибудь — файлы появятся здесь с превью.',
    convDrop: 'Перетащите медиафайл сюда либо выберите его ниже',
    browse: 'Обзор файлов', up: 'Вверх', convInput: 'Входной файл:',
    convInputPh: 'Выберите файл слева, перетащите его или укажите путь…',
    noFile: 'Файл не выбран', selected: 'Выбрано', convPreset: 'Профиль конвертации:',
    convOutput: 'Выходной файл (необязательно):',
    convOutputPh: 'Не указывайте — имя будет подобрано автоматически',
    convertBtn: 'Конвертировать', pickFileFirst: 'Сначала выберите входной файл.',
    pickPreset: 'Выберите профиль конвертации.', dirEmpty: 'Пустая папка',
    appearance: 'Оформление', accentCustom: 'Свой акцентный цвет (hex):',
    language: 'Язык интерфейса:', videoPreset: 'Видео-пресет по умолчанию:',
    langNote: 'Американский английский (en-US) поддерживается полностью.',
    dlSettingsGroup: 'Загрузки', dlDir: 'Папка загрузок:', audioFmt: 'Аудио-формат:',
    subLangs: 'Языки субтитров:', proxy: 'Прокси:', frags: 'Параллельных фрагментов:',
    queueMax: 'Макс. фоновых задач:', noMtime: 'Не изменять время файла (--no-mtime)',
    winNames: 'Безопасные имена файлов (--windows-filenames)', archive: 'Архив загрузок (без повторов)',
    save: 'Сохранить настройки', history: 'История операций', clear: 'Очистить',
    histEmpty: 'История пуста.', remove: 'Убрать', delFile: 'Удалить файл',
    confirmDelFile: 'Удалить файл с диска?', confirmClear: 'Очистить всю историю операций?',
    recheck: 'Проверить снова', checking: 'Проверка…',
    tDlStarted: 'Загрузка началась. Следите за прогрессом во вкладке «Загрузки».',
    tDlFail: 'Не удалось начать загрузку:', tConvStarted: 'Конвертация началась.',
    tConvFail: 'Конвертация не началась:', tSaved: 'Настройки сохранены.',
    tSaveFail: 'Не удалось сохранить:', tLoadFail: 'Не удалось загрузить настройки:',
    tBadAccent: 'Укажите цвет в формате hex, например #bfff00.',
    tDropNoPath: 'Не удалось получить путь к файлу. Укажите его вручную во вкладке «Конвертация».',
    tErr: 'Ошибка', loading: 'Загрузка…',
  },
  en: {
    tabHome: 'Home', tabDownloads: 'Downloads', tabLibrary: 'Library', tabConvert: 'Convert', tabSettings: 'Settings', tabDoctor: 'System',
    urlPh: 'Paste a link (YouTube, VK, Twitch…) and press Enter…',
    dlSettings: 'Download settings (presets)', download: 'Download',
    dlPreset: 'Codec preset:', dlQuality: 'Max quality:',
    qBest: 'Best available (Max)', dlCut: 'Time range cut (e.g. 00:01:00-00:03:30, or leave empty):',
    dlSubs: 'Subtitles (ru,en)', dlSponsor: 'SponsorBlock (cut sponsor segments)',
    dropHint: 'Drag and drop a file here to convert it',
    dropSub: 'The file will open in the Convert tab',
    stTasks: 'active tasks', stFiles: 'files in folder',
    shortcuts: 'Shift+H — history • Shift+I — settings • Esc — close',
    tasksHint: 'Active and finished download tasks', refresh: 'Refresh',
    tasksEmpty: 'No tasks yet. Paste a link on the home screen and press Download.',
    cancel: 'Cancel', queued: 'Queued…',
    libEmpty: 'The folder is empty. Download something — files will appear here with thumbnails.',
    convDrop: 'Drag a media file here or pick one below',
    browse: 'Browse files', up: 'Up', convInput: 'Input file:',
    convInputPh: 'Pick a file on the left, drag it here or type a path…',
    noFile: 'No file selected', selected: 'Selected', convPreset: 'Conversion profile:',
    convOutput: 'Output file (optional):',
    convOutputPh: 'Leave empty — a name will be generated automatically',
    convertBtn: 'Convert', pickFileFirst: 'Please select an input file first.',
    pickPreset: 'Please select a conversion profile.', dirEmpty: 'Empty folder',
    appearance: 'Appearance (customise colours)', accentCustom: 'Custom accent colour (hex):',
    language: 'Interface language:', videoPreset: 'Default video preset:',
    langNote: 'British English spelling is used across the interface.',
    dlSettingsGroup: 'Downloads', dlDir: 'Download folder:', audioFmt: 'Audio format:',
    subLangs: 'Subtitle languages:', proxy: 'Proxy:', frags: 'Concurrent fragments:',
    queueMax: 'Max background tasks:', noMtime: 'Keep file timestamps (--no-mtime)',
    winNames: 'Safe filenames (--windows-filenames)', archive: 'Download archive (no duplicates)',
    save: 'Save settings', history: 'Operation history', clear: 'Clear',
    histEmpty: 'History is empty.', remove: 'Remove', delFile: 'Delete file',
    confirmDelFile: 'Delete the file from disk?', confirmClear: 'Clear the entire operation history?',
    recheck: 'Re-check', checking: 'Checking…',
    tDlStarted: 'Download started. Watch the progress in the Downloads tab.',
    tDlFail: 'Could not start the download:', tConvStarted: 'Conversion started.',
    tConvFail: 'Could not start the conversion:', tSaved: 'Settings saved.',
    tSaveFail: 'Could not save:', tLoadFail: 'Could not load settings:',
    tBadAccent: 'Please enter a hex colour, e.g. #bfff00.',
    tDropNoPath: 'Could not read the file path. Please enter it manually in the Convert tab.',
    tErr: 'Error', loading: 'Loading…',
  },
  'en-US': {
    tabHome: 'Home', tabDownloads: 'Downloads', tabLibrary: 'Library', tabConvert: 'Convert', tabSettings: 'Settings', tabDoctor: 'System',
    urlPh: 'Paste a link (YouTube, VK, Twitch…) and press Enter…',
    dlSettings: 'Download settings (presets)', download: 'Download',
    dlPreset: 'Codec preset:', dlQuality: 'Max quality:',
    qBest: 'Best available (Max)', dlCut: 'Time range cut (e.g. 00:01:00-00:03:30, or leave empty):',
    dlSubs: 'Subtitles (ru,en)', dlSponsor: 'SponsorBlock (cut sponsor segments)',
    dropHint: 'Drag and drop a file here to convert it',
    dropSub: 'The file will open in the Convert tab',
    stTasks: 'active tasks', stFiles: 'files in folder',
    shortcuts: 'Shift+H — history • Shift+I — settings • Esc — close',
    tasksHint: 'Active and finished download tasks', refresh: 'Refresh',
    tasksEmpty: 'No tasks yet. Paste a link on the home screen and press Download.',
    cancel: 'Cancel', queued: 'Queued…',
    libEmpty: 'The folder is empty. Download something — files will appear here with thumbnails.',
    convDrop: 'Drag a media file here or pick one below',
    browse: 'Browse files', up: 'Up', convInput: 'Input file:',
    convInputPh: 'Pick a file on the left, drag it here or type a path…',
    noFile: 'No file selected', selected: 'Selected', convPreset: 'Conversion profile:',
    convOutput: 'Output file (optional):',
    convOutputPh: 'Leave empty — a name will be generated automatically',
    convertBtn: 'Convert', pickFileFirst: 'Please select an input file first.',
    pickPreset: 'Please select a conversion profile.', dirEmpty: 'Empty folder',
    appearance: 'Appearance (customize colors)', accentCustom: 'Custom accent color (hex):',
    language: 'Interface language:', videoPreset: 'Default video preset:',
    langNote: 'American English spelling is used across the interface.',
    dlSettingsGroup: 'Downloads', dlDir: 'Download folder:', audioFmt: 'Audio format:',
    subLangs: 'Subtitle languages:', proxy: 'Proxy:', frags: 'Concurrent fragments:',
    queueMax: 'Max background tasks:', noMtime: 'Keep file timestamps (--no-mtime)',
    winNames: 'Safe filenames (--windows-filenames)', archive: 'Download archive (no duplicates)',
    save: 'Save settings', history: 'Operation history', clear: 'Clear',
    histEmpty: 'History is empty.', remove: 'Remove', delFile: 'Delete file',
    confirmDelFile: 'Delete the file from disk?', confirmClear: 'Clear the entire operation history?',
    recheck: 'Re-check', checking: 'Checking…',
    tDlStarted: 'Download started. Watch the progress in the Downloads tab.',
    tDlFail: 'Could not start the download:', tConvStarted: 'Conversion started.',
    tConvFail: 'Could not start the conversion:', tSaved: 'Settings saved.',
    tSaveFail: 'Could not save:', tLoadFail: 'Could not load settings:',
    tBadAccent: 'Please enter a hex color, e.g. #bfff00.',
    tDropNoPath: 'Could not read the file path. Please enter it manually in the Convert tab.',
    tErr: 'Error', loading: 'Loading…',
  },
};

const T = (key) => (I18N[state.lang] && I18N[state.lang][key]) || I18N.ru[key] || key;

function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach((n) => { n.textContent = T(n.dataset.i18n); });
  document.querySelectorAll('[data-i18n-ph]').forEach((n) => { n.placeholder = T(n.dataset.i18nPh); });
  document.querySelectorAll('[data-i18n-title]').forEach((n) => { n.title = T(n.dataset.i18nTitle); });
  document.documentElement.lang = state.lang;
  // Перестроить зависящие от языка списки с сохранением значений.
  cselectSet('quality', qualityOptions(), cselectGet('quality'));
  if (videoPresetsCache.length) {
    cselectSet('preset', videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })), cselectGet('preset'));
    cselectSet('set-video-preset', videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })), cselectGet('set-video-preset'));
  }
  if (convertPresetsCache.length) {
    cselectSet('conv-preset', convertPresetsCache.map((p) => ({ value: p.id, label: convPresetLabel(p) })), cselectGet('conv-preset'));
    updateConvertDesc();
  }
}

function presetLabel(p) { return state.lang === 'ru' ? (p.name_ru || p.name_en) : p.name_en; }
function convPresetLabel(p) { return state.lang === 'ru' ? (p.name_ru || p.name_en) : p.name_en; }

/* ================= Тосты ================= */
function toast(msg, isError) {
  const box = el('toasts');
  const div = document.createElement('div');
  div.className = 'toast' + (isError ? ' error' : '');
  div.textContent = msg;
  box.appendChild(div);
  setTimeout(() => {
    div.classList.add('out');
    setTimeout(() => div.remove(), 260);
  }, 3600);
}

/* ================= Кастомный dropdown ================= */
const cselects = {};

function cselectInit(id, onChange) {
  const root = el(id);
  root.classList.add('cselect');
  root.innerHTML = `<button type="button" class="cselect-btn"><span class="cselect-label">…</span><span class="cselect-arrow">▾</span></button><div class="cselect-list hidden"></div>`;
  const st = { value: '', options: [], onChange: onChange || null };
  cselects[id] = st;
  root.querySelector('.cselect-btn').onclick = (e) => {
    e.stopPropagation();
    const wasOpen = root.classList.contains('open');
    closeAllSelects();
    if (!wasOpen) {
      root.classList.add('open');
      root.querySelector('.cselect-list').classList.remove('hidden');
    }
  };
}

function cselectSet(id, options, value) {
  const st = cselects[id];
  if (!st) return;
  st.options = options || [];
  if (value === undefined || !st.options.some((o) => o.value === value)) {
    value = st.options.length ? st.options[0].value : '';
  }
  st.value = value;
  const root = el(id);
  root.querySelector('.cselect-label').textContent =
    (st.options.find((o) => o.value === value) || {}).label || '…';
  const list = root.querySelector('.cselect-list');
  list.innerHTML = '';
  st.options.forEach((o, i) => {
    const item = document.createElement('div');
    item.className = 'cselect-opt' + (o.value === value ? ' selected' : '');
    item.style.setProperty('--i', i);
    item.textContent = o.label;
    item.title = o.label;
    item.onclick = (e) => {
      e.stopPropagation();
      st.value = o.value;
      root.querySelector('.cselect-label').textContent = o.label;
      list.querySelectorAll('.cselect-opt').forEach((x) => x.classList.remove('selected'));
      item.classList.add('selected');
      closeAllSelects();
      if (st.onChange) st.onChange(o.value);
    };
    list.appendChild(item);
  });
}

function cselectGet(id) { return cselects[id] ? cselects[id].value : ''; }

function closeAllSelects() {
  document.querySelectorAll('.cselect.open').forEach((r) => {
    r.classList.remove('open');
    r.querySelector('.cselect-list').classList.add('hidden');
  });
}
document.addEventListener('click', closeAllSelects);

function qualityOptions() {
  return [
    { value: '', label: T('qBest') },
    { value: '2160', label: '4K Ultra HD (2160p)' },
    { value: '1440', label: '2K Quad HD (1440p)' },
    { value: '1080', label: 'Full HD (1080p)' },
    { value: '720', label: 'HD (720p)' },
    { value: '480', label: 'SD (480p)' },
  ];
}

/* ================= Акцентный цвет ================= */
const ACCENT_SWATCHES = ['#bfff00', '#00ff88', '#00e5ff', '#ffaa00', '#ff5da2'];

function hexToRgb(hex) {
  const m = /^#([0-9a-fA-F]{6})$/.exec((hex || '').trim());
  if (!m) return null;
  const n = parseInt(m[1], 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function applyAccent(hex) {
  const rgb = hexToRgb(hex);
  if (!rgb) return false;
  state.accent = '#' + hex.trim().slice(1).toLowerCase();
  const css = getComputedStyle(document.documentElement);
  document.documentElement.style.setProperty('--lime', state.accent);
  document.documentElement.style.setProperty('--lime-dim', `rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, 0.35)`);
  void css;
  const prev = el('accent-prev');
  if (prev) prev.style.background = state.accent;
  const inp = el('set-accent');
  if (inp && document.activeElement !== inp) inp.value = state.accent;
  document.querySelectorAll('.swatch').forEach((s) => {
    s.classList.toggle('current', s.dataset.color.toLowerCase() === state.accent);
  });
  return true;
}

function buildSwatches() {
  const box = el('accent-swatches');
  box.innerHTML = '';
  ACCENT_SWATCHES.forEach((c) => {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'swatch';
    b.dataset.color = c;
    b.style.background = c;
    b.title = c;
    b.onclick = () => applyAccent(c);
    box.appendChild(b);
  });
}

/* ================= API layer ================= */
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
  thumbUrl: (name) => `${BASE}/api/library/thumb?name=${encodeURIComponent(name)}&token=${encodeURIComponent(TOKEN)}`,
};

/* Mock for `npm run dev`. */
const mockApi = {
  _id: 0,
  _tasks: [],
  _listeners: {},
  _history: [{ time: '2026-01-01 10:00:00', type: 'Download', source: 'https://example/v', target: 'video.mp4', status: 'Success' }],
  _config: {
    download_dir: '/tmp/MediaCLI', language: 'ru', video_preset: 'default',
    audio_format: 'mp3', sub_langs: 'ru,en', proxy_mode: 'system', proxy_url: '',
    concurrent_fragments: 4, bg_queue_max: 3, no_mtime: true, windows_filenames: true, use_archive: false,
    accent_color: '#bfff00',
  },
  _library: [
    { name: 'demo-video.mp4', size: 12345678, mtime: '2026-09-01 12:00:00', is_media: true },
    { name: 'demo-audio.mp3', size: 4567890, mtime: '2026-09-02 12:00:00', is_media: true },
    { name: 'notes.txt', size: 123, mtime: '2026-09-03 12:00:00', is_media: false },
  ],
  _convertPresets: [
    { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)', name_ru: 'Стандартный MP4 (H.264 + AAC)', desc_en: 'H.264 video + AAC audio.', desc_ru: 'Видео H.264 + аудио AAC.', ext: 'mp4', suffix: '_mp4', flags: [] },
    { id: 'audio_mp3', name_en: 'Extract Audio MP3', name_ru: 'Извлечь аудио MP3', desc_en: 'Audio only.', desc_ru: 'Только аудио.', ext: 'mp3', suffix: '_audio', flags: [] },
  ],
  _browsePath: '/tmp/MediaCLI',
  async presets() {
    return {
      video_presets: [
        { id: 'default', name_en: 'Original / Lossless Merge — Best Quality', name_ru: 'Оригинал без пережатия — лучшее качество' },
        { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)', name_ru: 'Стандартный MP4 (H.264 + AAC)' },
        { id: 'mkv_av1', name_en: 'Modern MKV (AV1 + Opus/AAC)', name_ru: 'Современный MKV (AV1 + Opus/AAC)' },
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
  thumbUrl: (name) => null,
};
if (MOCK) {
  mockApi.subscribe = (id, cb) => {
    (mockApi._listeners[id] = mockApi._listeners[id] || []).push(cb);
  };
}

const backend = MOCK ? mockApi : api;

/* ================= Вкладки ================= */
function switchTab(key) {
  document.querySelectorAll('.tab').forEach((b) => b.classList.toggle('active', b.dataset.tab === key));
  document.querySelectorAll('.tabpage').forEach((p) => {
    const show = p.id === `tab-${key}`;
    p.classList.toggle('hidden', !show);
    if (show) {
      p.classList.remove('page-enter');
      void p.offsetWidth;
      p.classList.add('page-enter');
    }
  });
  if (key === 'home') loadHome();
  if (key === 'downloads') refreshTasks();
  if (key === 'library') loadLibrary();
  if (key === 'convert') { loadConvertPresets(); loadBrowse(currentBrowsePath); }
  if (key === 'settings') loadSettings();
  if (key === 'doctor') loadDoctor();
}
document.querySelectorAll('.tab').forEach((btn) => {
  btn.onclick = () => switchTab(btn.dataset.tab);
});

/* ================= Загрузки ================= */
function cardShell(task) {
  const div = document.createElement('div');
  div.className = 'dl-card active';
  div.id = `task-${task.id}`;
  div.innerHTML = `
    <div class="dl-title"></div>
    <div class="dl-stage"></div>
    <progress max="100" value="0"></progress>
    <div class="dl-meta"><span class="st"></span><span class="pc"></span></div>
    <div class="row"><button class="btn small btn-cancel-task"></button></div>`;
  div.querySelector('.btn-cancel-task').textContent = T('cancel');
  div.querySelector('.btn-cancel-task').onclick = async () => {
    try { await backend.cancel(task.id); } catch (e) { toast(`${T('tErr')}: ${e.message}`, true); }
  };
  cardsEl.prepend(div);
  return div;
}

function paint(id, snap) {
  let div = el(`task-${id}`);
  if (!div) div = cardShell(snap);
  div.querySelector('.dl-title').textContent = snap.title || snap.source || `Task #${id}`;
  const live = snap.status === 'running' || snap.status === 'queued';
  div.querySelector('.dl-stage').innerHTML = '';
  if (live) {
    const dot = document.createElement('span');
    dot.className = 'livedot';
    div.querySelector('.dl-stage').appendChild(dot);
  }
  div.querySelector('.dl-stage').appendChild(document.createTextNode(snap.stage || snap.status));
  div.querySelector('progress').value = snap.progress || 0;
  div.querySelector('.st').textContent = snap.status;
  div.querySelector('.pc').textContent = `${(snap.progress || 0).toFixed(1)}%`;
  div.classList.toggle('active', live);
  div.classList.toggle('failed', String(snap.status).startsWith('fail'));
  const btn = div.querySelector('.btn-cancel-task');
  if (btn) btn.style.display = live ? '' : 'none';
}

function subscribe(id, onSnap) {
  const paintBoth = (snap) => {
    paint(id, snap);
    if (onSnap) onSnap(snap);
    // Бейдж активных задач — только по терминальным состояниям, иначе
    // каждый SSE-кадр дёргал бы лишний GET /api/tasks.
    if (['done', 'failed', 'cancelled'].includes(snap.status)) updateBadge();
  };
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
    if (!tasks.length) {
      cardsEl.innerHTML = `<div class="muted small">${T('tasksEmpty')}</div>`;
    } else {
      cardsEl.innerHTML = '';
      tasks.forEach((t, i) => {
        paint(t.id, t);
        const div = el(`task-${t.id}`);
        if (div) div.style.setProperty('--i', i);
        if (t.status === 'running' || t.status === 'queued') subscribe(t.id);
      });
    }
    updateBadge(tasks);
  } catch { /* fresh start */ }
}

async function updateBadge(tasks) {
  try {
    const list = tasks || (await backend.tasks().catch(() => ({ tasks: [] }))).tasks || [];
    const n = list.filter((t) => t.status === 'running' || t.status === 'queued').length;
    const b = el('badge-tasks');
    b.classList.toggle('hidden', !n);
    b.textContent = String(n);
  } catch { /* ignore */ }
}

/* ================= Главная ================= */
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

async function startDownload() {
  const url = el('url').value.trim();
  if (!url) return;
  const fields = { video_preset: cselectGet('preset') || 'default' };
  if (cselectGet('quality')) fields.quality = cselectGet('quality');
  if (el('timerange').value.trim()) fields.download_section = el('timerange').value.trim();
  if (el('subs').checked) { fields.subs_enabled = true; fields.embed_subs = true; }
  if (el('sponsor').checked) fields.sponsorblock = 'remove';
  el('btn-dl-start').disabled = true;
  try {
    const { task_id } = await backend.download({ url, fields });
    el('url').value = '';
    el('dlpanel').classList.add('hidden');
    toast(T('tDlStarted'));
    updateBadge();
    // Задача появится во вкладке «Загрузки» при следующем визите.
    void task_id;
  } catch (e) {
    toast(`${T('tDlFail')}\n${e.message}`, true);
  } finally {
    el('btn-dl-start').disabled = false;
  }
}

/* ================= Библиотека (превьюшки) ================= */
function isVideo(name) { return /\.(mp4|mkv|mov|webm|avi|m4v)$/i.test(name); }
function isAudio(name) { return /\.(mp3|flac|wav|m4a|opus|ogg)$/i.test(name); }

function thumbPlaceholder(wrap, name) {
  wrap.innerHTML = `<div class="ph">${isAudio(name) ? '♪' : '▣'}</div>`;
}

async function loadLibrary() {
  const box = el('library');
  box.innerHTML = `<div class="muted small">${T('loading')}</div>`;
  try {
    const { dir, files } = await backend.library();
    el('lib-dir').textContent = dir || '';
    if (!files.length) { box.innerHTML = `<div class="muted small">${T('libEmpty')}</div>`; return; }
    box.innerHTML = '';
    files.forEach((f, i) => {
      const card = document.createElement('div');
      card.className = 'dl-card';
      card.style.setProperty('--i', i);
      const title = document.createElement('div');
      title.className = 'dl-title';
      title.textContent = f.name;
      title.title = f.name;
      card.appendChild(title);
      if (isVideo(f.name)) {
        const wrap = document.createElement('div');
        wrap.className = 'thumbwrap';
        wrap.title = f.name;
        const spin = document.createElement('div');
        spin.className = 'spin';
        wrap.appendChild(spin);
        const thumb = backend.thumbUrl ? backend.thumbUrl(f.name) : null;
        if (thumb) {
          const img = document.createElement('img');
          img.loading = 'lazy';
          img.alt = f.name;
          img.onload = () => spin.remove();
          img.onerror = () => thumbPlaceholder(wrap, f.name);
          img.src = thumb;
          wrap.appendChild(img);
        } else {
          thumbPlaceholder(wrap, f.name);
          spin.remove();
        }
        const play = document.createElement('div');
        play.className = 'play';
        play.textContent = '▶';
        wrap.appendChild(play);
        wrap.onclick = () => openLightbox(f.name, 'video', backend.fileUrl(f.name));
        card.appendChild(wrap);
      } else if (isAudio(f.name)) {
        const wrap = document.createElement('div');
        wrap.className = 'thumbwrap';
        wrap.style.cursor = 'pointer';
        thumbPlaceholder(wrap, f.name);
        const play = document.createElement('div');
        play.className = 'play';
        play.textContent = '▶';
        wrap.appendChild(play);
        wrap.onclick = () => openLightbox(f.name, 'audio', backend.fileUrl(f.name));
        card.appendChild(wrap);
      } else {
        const wrap = document.createElement('div');
        wrap.className = 'thumbwrap';
        wrap.style.cursor = 'default';
        thumbPlaceholder(wrap, f.name);
        card.appendChild(wrap);
      }
      const meta = document.createElement('div');
      meta.className = 'dl-meta';
      meta.innerHTML = '<span></span><span></span>';
      meta.children[0].textContent = f.mtime || '';
      meta.children[1].textContent = fmtSize(f.size);
      card.appendChild(meta);
      box.appendChild(card);
    });
  } catch (e) { box.innerHTML = `<div class="muted small">${T('tErr')}: ${e.message}</div>`; }
}

/* ================= Lightbox ================= */
function openLightbox(name, kind, url) {
  el('lightbox-title').textContent = name;
  const body = el('lightbox-body');
  body.innerHTML = '';
  body.className = '';
  let m;
  if (kind === 'video') {
    m = document.createElement('video');
    m.controls = true;
    m.autoplay = true;
    m.src = url;
  } else {
    body.className = 'modal-body';
    m = document.createElement('audio');
    m.controls = true;
    m.autoplay = true;
    m.src = url;
    body.appendChild(m);
    el('lightbox').classList.remove('hidden');
    return;
  }
  body.appendChild(m);
  el('lightbox').classList.remove('hidden');
}
function closeLightbox() {
  el('lightbox-body').innerHTML = '';
  el('lightbox').classList.add('hidden');
}

/* ================= Конвертация ================= */
let currentBrowsePath = '';
let convertPresetsCache = [];
let videoPresetsCache = [];

async function loadConvertPresets() {
  try {
    const data = await backend.convertPresets();
    convertPresetsCache = data.convert_presets || [];
    cselectSet('conv-preset', convertPresetsCache.map((p) => ({ value: p.id, label: convPresetLabel(p) })), cselectGet('conv-preset'));
    updateConvertDesc();
  } catch (e) {
    el('conv-preset-desc').textContent = `${T('tErr')}: ${e.message}`;
  }
}

function updateConvertDesc() {
  const p = convertPresetsCache.find((x) => x.id === cselectGet('conv-preset'));
  el('conv-preset-desc').textContent = p
    ? `${p.desc_ru && state.lang === 'ru' ? p.desc_ru : (p.desc_en || p.desc_ru || '')} → .${p.ext}`.trim()
    : '';
}

async function loadVideoPresets() {
  try {
    const { video_presets } = await backend.presets();
    videoPresetsCache = video_presets || [];
    const opts = videoPresetsCache.map((p, i) => ({ value: p.id, label: `${i}. ${presetLabel(p)}` }));
    const plain = videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) }));
    cselectSet('preset', opts, cselectGet('preset') || 'default');
    cselectSet('set-video-preset', plain, settingsCache ? (settingsCache.video_preset || 'default') : 'default');
  } catch {
    cselectSet('preset', [{ value: '', label: 'daemon…' }], '');
  }
}

async function loadBrowse(path) {
  const dirsBox = el('browse-dirs');
  const filesBox = el('browse-files');
  try {
    const data = await backend.browse(path || '');
    currentBrowsePath = data.path || '';
    el('browse-path').textContent = currentBrowsePath;
    dirsBox.innerHTML = '';
    filesBox.innerHTML = '';
    (data.dirs || []).forEach((d, i) => {
      const row = document.createElement('div');
      row.className = 'brow';
      row.style.setProperty('--i', i);
      row.innerHTML = `<span>📁 </span>`;
      row.firstChild.appendChild(document.createTextNode(d));
      row.onclick = () => loadBrowse(currentBrowsePath + '/' + d);
      dirsBox.appendChild(row);
    });
    (data.files || []).forEach((f, i) => {
      const row = document.createElement('div');
      row.className = 'brow';
      row.style.setProperty('--i', i);
      if (el('conv-input').value.endsWith(f.name)) row.classList.add('selected');
      const left = document.createElement('span');
      left.textContent = `${f.is_media ? '🎬' : '📄'} ${f.name}`;
      const right = document.createElement('span');
      right.className = 'sz';
      right.textContent = fmtSize(f.size);
      row.appendChild(left);
      row.appendChild(right);
      row.title = `${f.name} • ${f.mtime || ''} • ${fmtSize(f.size)}`;
      row.onclick = () => selectBrowseFile(f, row);
      row.ondblclick = () => selectBrowseFile(f, row);
      filesBox.appendChild(row);
    });
    if (!(data.dirs || []).length && !(data.files || []).length) {
      filesBox.innerHTML = `<div class="muted small">${T('dirEmpty')}</div>`;
    }
  } catch (e) {
    filesBox.innerHTML = `<div class="muted small">${T('tErr')}: ${e.message}</div>`;
  }
}

function selectBrowseFile(f, row) {
  const full = (currentBrowsePath ? currentBrowsePath + '/' : '') + f.name;
  el('conv-input').value = full;
  el('conv-fileinfo').textContent = `${T('selected')}: ${f.name} • ${fmtSize(f.size)} • ${f.mtime || ''}`;
  if (row && row.parentNode) {
    row.parentNode.querySelectorAll('.brow').forEach((b) => b.classList.remove('selected'));
    row.classList.add('selected');
  }
}

function setConvertInput(path, meta) {
  el('conv-input').value = path;
  el('conv-fileinfo').textContent = meta || `${T('selected')}: ${path.split('/').pop()}`;
}

function paintConvertTask(id, snap) {
  let div = el(`conv-task-${id}`);
  const box = el('convert-tasks');
  if (!div) {
    div = document.createElement('div');
    div.className = 'hrow';
    div.id = `conv-task-${id}`;
    div.innerHTML = '<div style="flex:1;min-width:0"><b></b><div class="muted small"></div><progress max="100" value="0" style="width:100%"></progress></div>';
    box.prepend(div);
  }
  div.querySelector('b').textContent = snap.title || `Task #${id}`;
  div.querySelector('.small').textContent = `${snap.stage || snap.status} • ${(snap.progress || 0).toFixed(1)}%`;
  div.querySelector('progress').value = snap.progress || 0;
}

/* ================= Drag & Drop ================= */
function setupDrop(zone, onFile) {
  zone.addEventListener('dragenter', (e) => { e.preventDefault(); zone.classList.add('dragover'); });
  zone.addEventListener('dragover', (e) => { e.preventDefault(); zone.classList.add('dragover'); });
  zone.addEventListener('dragleave', (e) => {
    if (e.target === zone) zone.classList.remove('dragover');
    if (!zone.contains(e.relatedTarget)) zone.classList.remove('dragover');
  });
  zone.addEventListener('drop', (e) => {
    e.preventDefault();
    zone.classList.remove('dragover');
    const files = e.dataTransfer && e.dataTransfer.files;
    if (!files || !files.length) return;
    const f = files[0];
    // В Electron у File есть полный путь .path; в обычном браузере — только имя.
    const p = f.path || f.name;
    if (!p || (MOCK && !f.path)) {
      // В mock-режиме браузера пути нет — подставляем имя для наглядности.
      if (MOCK && f.name) { onFile(f.name, `${T('selected')}: ${f.name}`); return; }
      toast(T('tDropNoPath'), true);
      return;
    }
    onFile(p);
  });
}
// Чтобы перетаскивание мимо зон не открывало файл в окне.
window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());

function dropToConvert(path) {
  setConvertInput(path);
  switchTab('convert');
}

/* ================= История (drawer) ================= */
function openHistory() {
  el('history-drawer').classList.remove('hidden');
  el('drawer-scrim').classList.remove('hidden');
  el('history-drawer').setAttribute('aria-hidden', 'false');
  loadHistory();
}
function closeHistory() {
  el('history-drawer').classList.add('hidden');
  el('drawer-scrim').classList.add('hidden');
  el('history-drawer').setAttribute('aria-hidden', 'true');
}

async function loadHistory() {
  const box = el('history');
  box.innerHTML = `<div class="muted small">${T('loading')}</div>`;
  try {
    const { history } = await backend.history();
    if (!history.length) { box.innerHTML = `<div class="muted small">${T('histEmpty')}</div>`; return; }
    box.innerHTML = '';
    history.forEach((h, i) => {
      const row = document.createElement('div');
      row.className = 'hrow';
      row.style.setProperty('--i', i);
      const info = document.createElement('div');
      info.style.minWidth = '0';
      const b = document.createElement('b');
      b.textContent = h.target || h.source;
      const sub = document.createElement('div');
      sub.className = 'muted small';
      sub.textContent = `${h.time} • ${h.type} • ${h.status}`;
      info.appendChild(b);
      info.appendChild(sub);
      const btns = document.createElement('div');
      btns.className = 'row';
      const btnRm = document.createElement('button');
      btnRm.className = 'btn small';
      btnRm.textContent = T('remove');
      btnRm.onclick = async () => { await backend.historyDelete(h, false); loadHistory(); };
      const btnDel = document.createElement('button');
      btnDel.className = 'btn small danger';
      btnDel.textContent = T('delFile');
      btnDel.onclick = async () => {
        if (!confirm(`${T('confirmDelFile')}\n${h.target || h.source}`)) return;
        await backend.historyDelete(h, true);
        loadHistory();
      };
      btns.appendChild(btnRm);
      btns.appendChild(btnDel);
      row.appendChild(info);
      row.appendChild(btns);
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted small">${T('tErr')}: ${e.message}</div>`; }
}

/* ================= Настройки ================= */
let settingsCache = null;

async function loadSettings() {
  try {
    const cfg = await backend.config();
    settingsCache = cfg;
    if (cfg.accent_color && hexToRgb(cfg.accent_color)) applyAccent(cfg.accent_color);
    if (cfg.language && I18N[cfg.language]) {
      if (state.lang !== cfg.language) { state.lang = cfg.language; applyI18n(); }
    }
    el('set-download-dir').value = cfg.download_dir || '';
    el('set-sub-langs').value = cfg.sub_langs || '';
    el('set-proxy-url').value = cfg.proxy_url || '';
    el('set-no-mtime').checked = !!cfg.no_mtime;
    el('set-win-names').checked = !!cfg.windows_filenames;
    el('set-archive').checked = !!cfg.use_archive;
    cselectSet('set-language', [
      { value: 'ru', label: 'Русский' },
      { value: 'en', label: 'English' },
      { value: 'en-US', label: 'English (US)' },
    ], cfg.language || 'ru');
    cselectSet('set-audio-format',
      ['mp3', 'flac', 'wav', 'm4a', 'opus'].map((v) => ({ value: v, label: v })),
      cfg.audio_format || 'mp3');
    cselectSet('set-proxy-mode',
      ['system', 'custom', 'none'].map((v) => ({ value: v, label: v })),
      cfg.proxy_mode || 'system');
    cselectSet('set-fragments',
      ['2', '4', '8', '16'].map((v) => ({ value: v, label: v })),
      String(cfg.concurrent_fragments || 4));
    cselectSet('set-queue-max',
      ['1', '2', '3', '4'].map((v) => ({ value: v, label: v })),
      String(cfg.bg_queue_max || 3));
    if (videoPresetsCache.length) {
      cselectSet('set-video-preset',
        videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })),
        cfg.video_preset || 'default');
    } else {
      try {
        const { video_presets } = await backend.presets();
        videoPresetsCache = video_presets || [];
        cselectSet('set-video-preset',
          videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })),
          cfg.video_preset || 'default');
      } catch { /* ignore */ }
    }
  } catch (e) { toast(`${T('tLoadFail')} ${e.message}`, true); }
}

/* ================= System ================= */
async function loadDoctor() {
  const box = el('doctor');
  box.innerHTML = `<div class="muted small">${T('checking')}</div>`;
  try {
    const st = await backend.status();
    box.innerHTML = `<div class="muted small">daemon ${st.version}</div>`;
    st.dependencies.forEach((d, i) => {
      const row = document.createElement('div');
      row.className = 'hrow' + ((!d.available && d.required) ? ' failed' : '');
      row.style.setProperty('--i', i);
      const left = document.createElement('div');
      const b = document.createElement('b');
      b.textContent = d.name;
      const tag = document.createElement('span');
      tag.className = 'muted small';
      tag.textContent = ' ' + (d.required ? 'required' : 'optional');
      left.appendChild(b);
      left.appendChild(tag);
      const right = document.createElement('div');
      right.className = 'muted small';
      right.textContent = (d.available ? 'FOUND' : 'MISSING') + (d.path ? ` (${d.path})` : '');
      row.appendChild(left);
      row.appendChild(right);
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted small">${T('tErr')}: ${e.message}</div>`; }
}

/* ================= Горячие клавиши ================= */
document.addEventListener('keydown', (e) => {
  const tag = (e.target && e.target.tagName) || '';
  const typing = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (e.target && e.target.isContentEditable);
  if (e.key === 'Escape') {
    if (!el('lightbox').classList.contains('hidden')) { closeLightbox(); return; }
    if (document.querySelector('.cselect.open')) { closeAllSelects(); return; }
    if (!el('history-drawer').classList.contains('hidden')) { closeHistory(); }
    return;
  }
  // Shift+H — история, Shift+I — настройки. В полях ввода не срабатывают.
  if (e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey && !typing) {
    if (e.code === 'KeyH') {
      e.preventDefault();
      if (el('history-drawer').classList.contains('hidden')) openHistory();
      else closeHistory();
    } else if (e.code === 'KeyI') {
      e.preventDefault();
      switchTab('settings');
    }
  }
});

/* ================= Init ================= */
async function init() {
  buildSwatches();
  // Dropdowns.
  cselectInit('preset');
  cselectInit('quality');
  cselectInit('conv-preset');
  cselectInit('set-language');
  cselectInit('set-video-preset');
  cselectInit('set-audio-format');
  cselectInit('set-proxy-mode');
  cselectInit('set-fragments');
  cselectInit('set-queue-max');
  cselectSet('quality', qualityOptions(), '');

  // Конфиг раньше всего: язык и акцент.
  try {
    const cfg = await backend.config();
    settingsCache = cfg;
    if (cfg.language && I18N[cfg.language]) state.lang = cfg.language;
    if (cfg.accent_color && hexToRgb(cfg.accent_color)) state.accent = cfg.accent_color.toLowerCase();
  } catch { /* defaults */ }
  applyI18n();
  applyAccent(state.accent);

  await loadVideoPresets();
  await loadConvertPresets();
  await refreshTasks();
  await loadHome();

  // Главная: топбар.
  el('btn-dl-start').onclick = startDownload;
  el('url').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') startDownload();
  });
  el('btn-dl-settings').onclick = () => el('dlpanel').classList.toggle('hidden');

  // Drag & drop: главная → конвертация, зона конвертации — на месте.
  setupDrop(el('home-drop'), (p) => dropToConvert(p));
  setupDrop(el('conv-drop'), (p) => setConvertInput(p));

  // Загрузки / библиотека.
  el('btn-tasks-refresh').onclick = refreshTasks;
  el('btn-library-refresh').onclick = loadLibrary;

  // Конвертация.
  el('btn-browse-up').onclick = async () => {
    try {
      const data = await backend.browse(currentBrowsePath);
      if (data.parent) loadBrowse(data.parent);
    } catch { /* ignore */ }
  };
  el('btn-browse-refresh').onclick = () => loadBrowse(currentBrowsePath);
  el('btn-convert-start').onclick = async () => {
    const input = el('conv-input').value.trim();
    const preset_id = cselectGet('conv-preset');
    const output = el('conv-output').value.trim();
    if (!input) { toast(T('pickFileFirst'), true); return; }
    if (!preset_id) { toast(T('pickPreset'), true); return; }
    el('btn-convert-start').disabled = true;
    try {
      const { task_id } = await backend.convert({ input, preset_id, output });
      paintConvertTask(task_id, { id: task_id, title: `Convert: ${input}`, status: 'running', stage: T('queued'), progress: 0 });
      subscribe(task_id, (snap) => paintConvertTask(task_id, snap));
      toast(T('tConvStarted'));
    } catch (e) {
      toast(`${T('tConvFail')}\n${e.message}`, true);
    } finally {
      el('btn-convert-start').disabled = false;
    }
  };

  // История.
  el('btn-history-close').onclick = closeHistory;
  el('drawer-scrim').onclick = closeHistory;
  el('btn-history-clear').onclick = async () => {
    if (!confirm(T('confirmClear'))) return;
    await backend.historyClear();
    loadHistory();
  };

  // Lightbox.
  el('btn-lightbox-close').onclick = closeLightbox;
  el('lightbox').addEventListener('click', (e) => {
    if (e.target === el('lightbox')) closeLightbox();
  });

  // Система.
  el('btn-doctor-refresh').onclick = loadDoctor;

  // Настройки.
  el('set-accent').addEventListener('input', (e) => {
    const v = e.target.value.trim();
    if (hexToRgb(v)) applyAccent(v);
    else el('accent-prev').style.background = 'transparent';
  });
  el('btn-settings-save').onclick = async () => {
    if (!settingsCache) return;
    const accentRaw = el('set-accent').value.trim();
    const accent = accentRaw === '' ? state.accent : accentRaw;
    if (!hexToRgb(accent)) { toast(T('tBadAccent'), true); return; }
    applyAccent(accent);
    const cfg = {
      ...settingsCache,
      download_dir: el('set-download-dir').value.trim(),
      language: cselectGet('set-language'),
      video_preset: cselectGet('set-video-preset'),
      audio_format: cselectGet('set-audio-format'),
      sub_langs: el('set-sub-langs').value.trim(),
      proxy_mode: cselectGet('set-proxy-mode'),
      proxy_url: el('set-proxy-url').value.trim(),
      concurrent_fragments: parseInt(cselectGet('set-fragments'), 10),
      bg_queue_max: parseInt(cselectGet('set-queue-max'), 10),
      no_mtime: el('set-no-mtime').checked,
      windows_filenames: el('set-win-names').checked,
      use_archive: el('set-archive').checked,
      accent_color: state.accent,
    };
    try {
      settingsCache = await backend.saveConfig(cfg);
      if (settingsCache.language && I18N[settingsCache.language] && settingsCache.language !== state.lang) {
        state.lang = settingsCache.language;
      }
      applyI18n();
      toast(T('tSaved'));
    } catch (e) { toast(`${T('tSaveFail')} ${e.message}`, true); }
  };
}

init();
