'use strict';
/* Renderer: talks to the Go daemon (or a mock with ?mock=1 for UI dev). */
const qs = new URLSearchParams(location.search);
const MOCK = qs.get('mock') === '1';
const BASE = MOCK ? null : `http://127.0.0.1:${qs.get('port')}`;
const TOKEN = MOCK ? null : qs.get('token');

const el = (id) => document.getElementById(id);
const state = { lang: 'ru', accent: '#bfff00' };

/* ================= i18n (формальное «Вы»; en-US поддерживается) ================= */
const I18N = {
  ru: {
    navHome: 'Главная', navLibrary: 'Видео', navConvert: 'Конвертация', navHistory: 'История операций', navSettings: 'Настройки', navDoctor: 'Система',
    urlPh: 'Вставьте ссылку (YouTube, VK, Twitch…) и нажмите Enter…',
    dlSettings: 'Настройки загрузки (пресеты)', download: 'Скачать',
    dlPreset: 'Пресет кодека:', dlQuality: 'Макс. качество:',
    qBest: 'Лучшее доступное (Max)', dlCut: 'Вырезка по времени (напр. 00:01:00-00:03:30, либо не указывайте):',
    dlSubs: 'Субтитры (ru,en)', dlSponsor: 'SponsorBlock (вырезать спонсорские сегменты)',
    dlMeta: 'Встраивать метаданные', dlThumb: 'Встраивать обложку (поиск миниатюр + перезапись файла)',
    brandHint1: 'Вставьте ссылку выше и нажмите Enter — начнётся загрузка',
    brandHint2: 'Перетащите файл в окно — откроется конвертация',
    slotsTitle: 'Загрузки', slotSizeHint: 'Ctrl + колесо — размер слотов',
    clearDone: 'Очистить завершённые', groupBtn: 'Сгруппировать', ungroup: 'Расформировать',
    groupName: 'Группа', waiting: 'Ожидает очереди', queued: 'В очереди…',
    interrupted: 'Прервано перезапуском', removeSlot: 'Убрать',
    dropHint: 'Перетащите файл в это окно, чтобы конвертировать его',
    dropSub: 'Файл будет открыт во вкладке «Конвертация»',
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
    cancel: 'Отмена', refresh: 'Обновить',
    libEmpty: 'Папка пуста. Скачайте что-нибудь — файлы появятся здесь с превью.',
    confirmDelFile: 'Удалить файл с диска?', confirmClear: 'Очистить всю историю операций?',
    recheck: 'Проверить снова', checking: 'Проверка…',
    tDlFail: 'Не удалось начать загрузку:', tConvStarted: 'Конвертация началась.',
    tConvFail: 'Конвертация не началась:', tSaved: 'Настройки сохранены.',
    tSaveFail: 'Не удалось сохранить:', tLoadFail: 'Не удалось загрузить настройки:',
    tBadAccent: 'Укажите цвет в формате hex, например #bfff00.',
    tDropNoPath: 'Не удалось получить путь к файлу. Укажите его вручную в «Конвертации».',
    tErr: 'Ошибка', taskLost: 'Связь с задачей потеряна', loading: 'Загрузка…',
    setTabGen: 'Основные', setTabCodecs: 'Видео и кодеки', setTabNet: 'Ускорение и сеть', setTabTools: 'Инструменты', setTabIface: 'Интерфейс',
    userGoal: 'Основная цель:', cookiesMode: 'Авторизация (Cookies):',
    cookiesFile: 'Путь к cookies.txt:', cookiesBrowser: 'Браузер для чтения cookies:',
    archiveFile: 'Файл архива:', proxyUrl: 'Proxy URL:',
    transcodeMode: 'Движок транскодинга:', transcEmbedded: 'Встроенный (yt-dlp --recode)',
    transcExternal: 'Внешний FFmpeg (отдельно, тот же результат)',
    thumbFormat: 'Формат обложек:', ffmpegSuffix: 'Суффиксы в именах FFmpeg',
    overwriteOrig: 'Заменять исходный файл',
    ffmpegPath: 'Путь к ffmpeg:', ffmpegPathPh: 'Пусто — искать в PATH',
    ffmpegHint: 'Укажите полный путь, если yt-dlp не находит ffmpeg из PATH.', ffmpegCheck: 'Проверить',
    tuiTheme: 'Тема консоли (TUI):', progressStyle: 'Стиль прогресса (консоль):',
    termBg: 'Прозрачный фон терминала', notifyBell: 'Звуковой сигнал по завершении',
    autoCheck: 'Проверять зависимости при старте', logoMode: 'Режим логотипа (консоль):',
    logoAscii: 'ASCII-пресет:', logoProto: 'Протокол картинок:', logoImage: 'Картинка логотипа (путь):',
    logoNote: 'Картинки работают только в Kitty / iTerm2 / WezTerm.',
    defaultEditor: 'Редактор по умолчанию:', resetBtn: 'Сбросить к заводским',
    confirmReset: 'Сбросить все настройки к заводским?',
    logWaiting: 'Логов пока нет — слот ожидает очереди.', logEmpty: 'Логов пока нет.',
    ckNone: 'Отключено', ckFile: 'Файл cookies.txt', ckBrowser: 'Из браузера',
    logoAsciiMode: 'ASCII', logoImgMode: 'Картинка',
    dlSavedPreset: 'Сохранённый пресет:', dlSavePreset: 'Сохранить как пресет…', dlNoPresets: 'Нет сохранённых пресетов', dlManual: 'Не использовать (ручные настройки)',
    presetsTitle: 'Пресеты загрузок', presetsHint: 'Ctrl+Shift+I — быстрый доступ', presetHint: 'Сохранённые наборы настроек для быстрой загрузки. Выберите пресет на шестерёнке при скачивании.',
    presetCreate: 'Сохранить текущие…', presetEmpty: 'Нет сохранённых пресетов. Сохраните текущие настройки на шестерёнке.', presetDelete: 'Удалить', presetUse: 'Использовать',
    presetDeleteConfirm: 'Удалить пресет?', presetNamePrompt: 'Имя пресета:', presetCreated: 'Пресет сохранён', presetDeleted: 'Пресет удалён', tPresetFail: 'Не удалось сохранить пресет:',
  },
  en: {
    navHome: 'Home', navLibrary: 'Library', navConvert: 'Convert', navHistory: 'Operation history', navSettings: 'Settings', navDoctor: 'System',
    urlPh: 'Paste a link (YouTube, VK, Twitch…) and press Enter…',
    dlSettings: 'Download settings (presets)', download: 'Download',
    dlPreset: 'Codec preset:', dlQuality: 'Max quality:',
    qBest: 'Best available (Max)', dlCut: 'Time range cut (e.g. 00:01:00-00:03:30, or leave empty):',
    dlSubs: 'Subtitles (ru,en)', dlSponsor: 'SponsorBlock (cut sponsor segments)',
    dlMeta: 'Embed metadata', dlThumb: 'Embed thumbnail (thumbnail hunt + file rewrite)',
    brandHint1: 'Paste a link above and press Enter to start a download',
    brandHint2: 'Drag a file into the window to convert it',
    slotsTitle: 'Downloads', slotSizeHint: 'Ctrl + wheel — slot size',
    clearDone: 'Clear finished', groupBtn: 'Group', ungroup: 'Ungroup',
    groupName: 'Group', waiting: 'Queued in group', queued: 'Queued…',
    interrupted: 'Interrupted by restart', removeSlot: 'Remove',
    dropHint: 'Drag and drop a file here to convert it',
    dropSub: 'The file will open in the Convert view',
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
    cancel: 'Cancel', refresh: 'Refresh',
    libEmpty: 'The folder is empty. Download something — files will appear here with thumbnails.',
    confirmDelFile: 'Delete the file from disk?', confirmClear: 'Clear the entire operation history?',
    recheck: 'Re-check', checking: 'Checking…',
    tDlFail: 'Could not start the download:', tConvStarted: 'Conversion started.',
    tConvFail: 'Could not start the conversion:', tSaved: 'Settings saved.',
    tSaveFail: 'Could not save:', tLoadFail: 'Could not load settings:',
    tBadAccent: 'Please enter a hex colour, e.g. #bfff00.',
    tDropNoPath: 'Could not read the file path. Please enter it manually in Convert.',
    tErr: 'Error', taskLost: 'Lost contact with the task', loading: 'Loading…',
    setTabGen: 'General', setTabCodecs: 'Video & Codecs', setTabNet: 'Speed & Network', setTabTools: 'Tools', setTabIface: 'Interface',
    userGoal: 'Primary use-case:', cookiesMode: 'Authentication (Cookies):',
    cookiesFile: 'Path to cookies.txt:', cookiesBrowser: 'Browser to read cookies from:',
    archiveFile: 'Archive file:', proxyUrl: 'Proxy URL:',
    transcodeMode: 'Transcoding engine:', transcEmbedded: 'Embedded (yt-dlp --recode)',
    transcExternal: 'External FFmpeg (separate, same output)',
    thumbFormat: 'Thumbnail format:', ffmpegSuffix: 'FFmpeg filename suffixes',
    overwriteOrig: 'Replace the source file',
    ffmpegPath: 'Path to ffmpeg:', ffmpegPathPh: 'Empty — search in PATH',
    ffmpegHint: 'Set a full path if yt-dlp cannot find ffmpeg from PATH.', ffmpegCheck: 'Check',
    tuiTheme: 'Console theme (TUI):', progressStyle: 'Progress style (console):',
    termBg: 'Transparent terminal background', notifyBell: 'Bell on finish',
    autoCheck: 'Check dependencies at startup', logoMode: 'Logo mode (console):',
    logoAscii: 'ASCII preset:', logoProto: 'Image protocol:', logoImage: 'Logo image (path):',
    logoNote: 'Images work only in Kitty / iTerm2 / WezTerm.',
    defaultEditor: 'Default editor:', resetBtn: 'Reset to defaults',
    confirmReset: 'Reset all settings to factory defaults?',
    logWaiting: 'No logs yet — the slot is queued.', logEmpty: 'No logs yet.',
    ckNone: 'Disabled', ckFile: 'cookies.txt file', ckBrowser: 'From browser',
    logoAsciiMode: 'ASCII', logoImgMode: 'Image',
    dlSavedPreset: 'Saved preset:', dlSavePreset: 'Save as preset…', dlNoPresets: 'No saved presets', dlManual: 'Manual settings (no preset)',
    presetsTitle: 'Download Presets', presetsHint: 'Ctrl+Shift+I — quick access', presetHint: 'Saved settings bundles for quick downloads. Pick a preset from the gear menu when downloading.',
    presetCreate: 'Save current…', presetEmpty: 'No saved presets yet. Save current gear settings as a preset.', presetDelete: 'Delete', presetUse: 'Use',
    presetDeleteConfirm: 'Delete preset?', presetNamePrompt: 'Preset name:', presetCreated: 'Preset saved', presetDeleted: 'Preset deleted', tPresetFail: 'Could not save preset:',
  },
  'en-US': {
    navHome: 'Home', navLibrary: 'Library', navConvert: 'Convert', navHistory: 'Operation history', navSettings: 'Settings', navDoctor: 'System',
    urlPh: 'Paste a link (YouTube, VK, Twitch…) and press Enter…',
    dlSettings: 'Download settings (presets)', download: 'Download',
    dlPreset: 'Codec preset:', dlQuality: 'Max quality:',
    qBest: 'Best available (Max)', dlCut: 'Time range cut (e.g. 00:01:00-00:03:30, or leave empty):',
    dlSubs: 'Subtitles (ru,en)', dlSponsor: 'SponsorBlock (cut sponsor segments)',
    dlMeta: 'Embed metadata', dlThumb: 'Embed thumbnail (thumbnail hunt + file rewrite)',
    brandHint1: 'Paste a link above and press Enter to start a download',
    brandHint2: 'Drag a file into the window to convert it',
    slotsTitle: 'Downloads', slotSizeHint: 'Ctrl + wheel — slot size',
    clearDone: 'Clear finished', groupBtn: 'Group', ungroup: 'Ungroup',
    groupName: 'Group', waiting: 'Queued in group', queued: 'Queued…',
    interrupted: 'Interrupted by restart', removeSlot: 'Remove',
    dropHint: 'Drag and drop a file here to convert it',
    dropSub: 'The file will open in the Convert view',
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
    cancel: 'Cancel', refresh: 'Refresh',
    libEmpty: 'The folder is empty. Download something — files will appear here with thumbnails.',
    confirmDelFile: 'Delete the file from disk?', confirmClear: 'Clear the entire operation history?',
    recheck: 'Re-check', checking: 'Checking…',
    tDlFail: 'Could not start the download:', tConvStarted: 'Conversion started.',
    tConvFail: 'Could not start the conversion:', tSaved: 'Settings saved.',
    tSaveFail: 'Could not save:', tLoadFail: 'Could not load settings:',
    tBadAccent: 'Please enter a hex color, e.g. #bfff00.',
    tDropNoPath: 'Could not read the file path. Please enter it manually in Convert.',
    tErr: 'Error', taskLost: 'Lost contact with the task', loading: 'Loading…',
    setTabGen: 'General', setTabCodecs: 'Video & Codecs', setTabNet: 'Speed & Network', setTabTools: 'Tools', setTabIface: 'Interface',
    userGoal: 'Primary use-case:', cookiesMode: 'Authentication (Cookies):',
    cookiesFile: 'Path to cookies.txt:', cookiesBrowser: 'Browser to read cookies from:',
    archiveFile: 'Archive file:', proxyUrl: 'Proxy URL:',
    transcodeMode: 'Transcoding engine:', transcEmbedded: 'Embedded (yt-dlp --recode)',
    transcExternal: 'External FFmpeg (separate, same output)',
    thumbFormat: 'Thumbnail format:', ffmpegSuffix: 'FFmpeg filename suffixes',
    overwriteOrig: 'Replace the source file',
    ffmpegPath: 'Path to ffmpeg:', ffmpegPathPh: 'Empty — search in PATH',
    ffmpegHint: 'Set a full path if yt-dlp cannot find ffmpeg from PATH.', ffmpegCheck: 'Check',
    tuiTheme: 'Console theme (TUI):', progressStyle: 'Progress style (console):',
    termBg: 'Transparent terminal background', notifyBell: 'Bell on finish',
    autoCheck: 'Check dependencies at startup', logoMode: 'Logo mode (console):',
    logoAscii: 'ASCII preset:', logoProto: 'Image protocol:', logoImage: 'Logo image (path):',
    logoNote: 'Images work only in Kitty / iTerm2 / WezTerm.',
    defaultEditor: 'Default editor:', resetBtn: 'Reset to defaults',
    confirmReset: 'Reset all settings to factory defaults?',
    logWaiting: 'No logs yet — the slot is queued.', logEmpty: 'No logs yet.',
    ckNone: 'Disabled', ckFile: 'cookies.txt file', ckBrowser: 'From browser',
    logoAsciiMode: 'ASCII', logoImgMode: 'Image',
    dlSavedPreset: 'Saved preset:', dlSavePreset: 'Save as preset…', dlNoPresets: 'No saved presets', dlManual: 'Manual settings (no preset)',
    presetsTitle: 'Download Presets', presetsHint: 'Ctrl+Shift+I — quick access', presetHint: 'Saved settings bundles for quick downloads. Pick a preset from the gear menu when downloading.',
    presetCreate: 'Save current…', presetEmpty: 'No saved presets yet. Save current gear settings as a preset.', presetDelete: 'Delete', presetUse: 'Use',
    presetDeleteConfirm: 'Delete preset?', presetNamePrompt: 'Preset name:', presetCreated: 'Preset saved', presetDeleted: 'Preset deleted', tPresetFail: 'Could not save preset:',
  },
};

const T = (key) => (I18N[state.lang] && I18N[state.lang][key]) || I18N.ru[key] || key;

function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach((n) => { n.textContent = T(n.dataset.i18n); });
  document.querySelectorAll('[data-i18n-ph]').forEach((n) => { n.placeholder = T(n.dataset.i18nPh); });
  document.querySelectorAll('[data-i18n-title]').forEach((n) => { n.title = T(n.dataset.i18nTitle); });
  document.documentElement.lang = state.lang;
  cselectSet('quality', qualityOptions(), cselectGet('quality'));
  if (videoPresetsCache.length) {
    cselectSet('preset', videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })), cselectGet('preset'));
    cselectSet('set-video-preset', videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })), cselectGet('set-video-preset'));
  }
  if (convertPresetsCache.length) {
    cselectSet('conv-preset', convertPresetsCache.map((p) => ({ value: p.id, label: convPresetLabel(p) })), cselectGet('conv-preset'));
    updateConvertDesc();
  }
  if (cselects['dl-saved-preset']) refreshSavedPresetSelect();
  rebuildSettingsSelects();
  renderSlots();
  renderPresetsList();
}

function presetLabel(p) { return state.lang === 'ru' ? (p.name_ru || p.name_en) : p.name_en; }
function convPresetLabel(p) { return state.lang === 'ru' ? (p.name_ru || p.name_en) : p.name_en; }
function themeLabel(t) { return state.lang === 'ru' ? (t.name_ru || t.name_en) : t.name_en; }

const GOALS = {
  ru: {
    editing: 'Видеомонтаж (DaVinci / Premiere)', downloading: 'Обычная загрузка видео',
    audio: 'Извлечение и архивация аудио', transcoding: 'Локальная конвертация FFmpeg',
  },
  en: {
    editing: 'Video Editing (DaVinci / Premiere)', downloading: 'General Media Downloading',
    audio: 'Audio Extraction & Archiving', transcoding: 'FFmpeg Transcoding & Encoding',
  },
  'en-US': {
    editing: 'Video Editing (DaVinci / Premiere)', downloading: 'General Media Downloading',
    audio: 'Audio Extraction & Archiving', transcoding: 'FFmpeg Transcoding & Encoding',
  },
};
const goalLabel = (v) => ((GOALS[state.lang] || GOALS.ru)[v] || v);

/* Справочники из GET /api/meta (single source of truth — Go). */
let metaCache = {
  themes: [],
  browsers: ['chrome', 'firefox', 'brave', 'edge', 'opera', 'vivaldi', 'chromium', 'safari'],
  logo_ascii: ['standard', 'coder_mini', 'toilet', 'rubifont'],
  logo_protocols: ['kitty', 'iterm2'],
  progress_styles: ['blocks', 'classic', 'dots', 'minimal'],
  user_goals: ['editing', 'downloading', 'audio', 'transcoding'],
  thumbnail_format: ['png', 'jpg', 'webp'],
  audio_formats: ['mp3', 'flac', 'wav', 'm4a', 'opus'],
};
async function loadMeta() {
  try {
    const m = await backend.meta();
    if (m) metaCache = { ...metaCache, ...m };
  } catch { /* fallback-списки выше */ }
}

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
  document.documentElement.style.setProperty('--lime', state.accent);
  document.documentElement.style.setProperty('--lime-dim', `rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, 0.35)`);
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
      const err = new Error(`${method} ${path}: ${res.status} ${txt}`);
      err.status = res.status;
      throw err;
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
  resetConfig: () => api.req('POST', '/api/config/reset'),
  status: () => api.req('GET', '/api/status'),
  meta: () => api.req('GET', '/api/meta'),
  task: (id) => api.req('GET', `/api/tasks/${id}`),
  ffmpegCheck: (path) => api.req('GET', '/api/tools/ffmpeg?path=' + encodeURIComponent(path || '')),
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
    accent_color: '#bfff00', download_presets: [],
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
    // also expose saved download_presets in mock (persisted via localStorage config)
    let dlPresets = [];
    try {
      const raw = localStorage.getItem('mc_mock_cfg_v1');
      if (raw) {
        const c = JSON.parse(raw);
        if (Array.isArray(c.download_presets)) dlPresets = c.download_presets;
      }
    } catch { /* ignore */ }
    if (!dlPresets.length && this._config.download_presets) dlPresets = this._config.download_presets;
    return {
      video_presets: [
        { id: 'default', name_en: 'Original / Lossless Merge — Best Quality', name_ru: 'Оригинал без пережатия — лучшее качество' },
        { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)', name_ru: 'Стандартный MP4 (H.264 + AAC)' },
        { id: 'mkv_av1', name_en: 'Modern MKV (AV1 + Opus/AAC)', name_ru: 'Современный MKV (AV1 + Opus/AAC)' },
      ],
      download_presets: dlPresets,
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
  async config() {
    try {
      const raw = localStorage.getItem('mc_mock_cfg_v1');
      if (raw) {
        const saved = JSON.parse(raw);
        this._config = { ...this._config, ...saved };
      }
    } catch { /* ignore */ }
    return { ...this._config };
  },
  async saveConfig(cfg) {
    this._config = { ...cfg };
    try { localStorage.setItem('mc_mock_cfg_v1', JSON.stringify(this._config)); } catch { /* ignore */ }
    return { ...this._config };
  },
  async resetConfig() {
    this._config = {
      download_dir: '/tmp/MediaCLI', language: 'ru', video_preset: 'default',
      audio_format: 'mp3', sub_langs: 'ru,en', proxy_mode: 'system', proxy_url: '',
      concurrent_fragments: 4, bg_queue_max: 3, no_mtime: true, windows_filenames: true, use_archive: false,
      accent_color: '#bfff00', ffmpeg_path: '', download_presets: [],
    };
    try { localStorage.setItem('mc_mock_cfg_v1', JSON.stringify(this._config)); } catch { /* ignore */ }
    return { ...this._config };
  },
  async meta() {
    return {
      themes: [
        { id: 'cyan', name_en: 'Arch Cyan (Default)', name_ru: 'Arch Cyan (По умолчанию)' },
        { id: 'nord', name_en: 'Nord Blue', name_ru: 'Nord Blue' },
      ],
      browsers: ['chrome', 'firefox'], logo_ascii: ['standard'], logo_protocols: ['kitty', 'iterm2'],
      progress_styles: ['blocks', 'classic'], user_goals: ['editing', 'downloading'],
      thumbnail_format: ['png', 'jpg'], audio_formats: ['mp3', 'flac'],
    };
  },
  async task(id) {
    const t = this._tasks.find((x) => x.id === Number(id));
    if (!t) throw new Error('not found');
    return { ...t, log_tail: [t.stage || t.status] };
  },
  async ffmpegCheck(path) {
    if (path && path.includes('bad')) return { configured: path, resolved: path, found: false, error: 'mock: file not found' };
    return { configured: path || '', resolved: 'ffmpeg', found: true, version: 'mock ffmpeg 7.x' };
  },
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

/* ================= Виды (только горячие клавиши) ================= */
function showView(name) {
   document.querySelectorAll('.view').forEach((p) => {
     const show = p.id === `view-${name}`;
     p.classList.toggle('hidden', !show);
     if (show) {
       p.classList.remove('page-enter');
       void p.offsetWidth;
       p.classList.add('page-enter');
     }
   });
   if (name === 'home') renderSlots();
   if (name === 'library') loadLibrary();
   if (name === 'convert') { loadConvertPresets(); loadBrowse(currentBrowsePath); }
   if (name === 'settings') loadSettings();
   if (name === 'doctor') loadDoctor();
   if (name === 'presets') { void loadVideoPresets(); renderPresetsList(); }
}

/* ================= Пресеты загрузок (Ctrl+Shift+I, шестерёнка) ================= */
function collectDlFields() {
  const fields = { video_preset: cselectGet('preset') || 'default' };
  if (cselectGet('quality')) fields.quality = cselectGet('quality');
  if (el('timerange').value.trim()) fields.download_section = el('timerange').value.trim();
  if (el('subs').checked) { fields.subs_enabled = true; fields.embed_subs = true; }
  if (el('sponsor').checked) fields.sponsorblock = 'remove';
  fields.embed_metadata = el('emb-meta').checked;
  fields.embed_thumbnail = el('emb-thumb').checked;
  return fields;
}

async function saveCurrentDlAsPreset() {
  // window.prompt в Electron не реализован — пробуем, при неудаче даём автоимя.
  let name = '';
  try {
    if (typeof prompt === 'function') name = prompt(T('presetNamePrompt'), `Preset ${savedPresetsCache.length + 1}`) || '';
  } catch { name = ''; }
  // prompt вернул null при отмене — выходим; пустая строка из-за неподдержки — автоимя.
  if (name === null) return;
  if (!name.trim()) name = `Preset ${savedPresetsCache.length + 1}`;
  try {
    const cfg = await backend.config();
    const fields = collectDlFields();
    const preset = { id: `preset_${Date.now()}`, name: name.trim(), fields };
    cfg.download_presets = cfg.download_presets || [];
    cfg.download_presets.push(preset);
    await backend.saveConfig(cfg);
    savedPresetsCache = cfg.download_presets;
    refreshSavedPresetSelect();
    // Выбираем только что созданный пресет.
    if (cselects['dl-saved-preset']) cselectSet('dl-saved-preset',
      [{ value: '', label: T('dlManual') }].concat(savedPresetsCache.map((p) => ({ value: p.id, label: p.name }))),
      preset.id);
    updateDlPresetDesc();
    renderPresetsList();
    toast(T('presetCreated'));
  } catch (e) {
    toast(`${T('tPresetFail')} ${e.message}`, true);
  }
}

function renderPresetsList() {
  const box = el('presets-list');
  if (!box) return;
  if (!savedPresetsCache.length) {
    box.innerHTML = `<div class="muted small">${T('presetEmpty')}</div>`;
    return;
  }
  box.innerHTML = '';
  savedPresetsCache.forEach((p, i) => {
    const row = document.createElement('div');
    row.className = 'hrow';
    row.style.setProperty('--i', i);
    const info = document.createElement('div');
    info.style.minWidth = '0';
    const b = document.createElement('b');
    b.textContent = p.name;
    const sub = document.createElement('div');
    sub.className = 'muted small';
    const f = p.fields || {};
    const parts = [];
    if (f.video_preset) parts.push(f.video_preset);
    if (f.quality) parts.push(f.quality + 'p');
    if (f.download_section) parts.push(f.download_section);
    sub.textContent = parts.join(' • ') || p.id;
    info.appendChild(b);
    info.appendChild(sub);
    const btns = document.createElement('div');
    btns.className = 'row';
    const btnUse = document.createElement('button');
    btnUse.className = 'btn small';
    btnUse.textContent = T('presetUse');
    btnUse.onclick = () => {
      cselectSet('dl-saved-preset', [{ value: '', label: T('dlManual') }].concat(savedPresetsCache.map((x) => ({ value: x.id, label: x.name }))), p.id);
      updateDlPresetDesc();
      showView('home');
      el('dlpanel').classList.remove('hidden');
      toast(p.name);
    };
    const btnDel = document.createElement('button');
    btnDel.className = 'btn small danger';
    btnDel.textContent = T('presetDelete');
    btnDel.onclick = async () => {
      if (!confirm(`${T('presetDeleteConfirm')}\n${p.name}`)) return;
      try {
        const cfg = await backend.config();
        cfg.download_presets = (cfg.download_presets || []).filter((x) => x.id !== p.id);
        await backend.saveConfig(cfg);
        savedPresetsCache = cfg.download_presets || [];
        refreshSavedPresetSelect();
        renderPresetsList();
        toast(T('presetDeleted'));
      } catch (e) { toast(`${T('tErr')}: ${e.message}`, true); }
    };
    btns.appendChild(btnUse);
    btns.appendChild(btnDel);
    row.appendChild(info);
    row.appendChild(btns);
    box.appendChild(row);
  });
}

/* ================= Полоса по краям экрана ================= */
function edgeFlash() {
  const d = document.createElement('div');
  d.className = 'edge-run';
  d.innerHTML = '<svg preserveAspectRatio="none" viewBox="0 0 100 100"><rect x="1" y="1" width="98" height="98" pathLength="1"/></svg>';
  document.body.appendChild(d);
  setTimeout(() => d.remove(), 1200);
}

/* ================= Слоты загрузок + группы ================= */
let slots = [];   // {key,url,fields,status,taskId,groupId,createdAt,title,stage,progress}
let groups = [];  // {id,name,createdAt}
let groupSeq = 0;
let slotSeq = 0;
const selected = new Set();
const TERMINAL = ['done', 'failed', 'cancelled'];
const isTerminalSlot = (s) => TERMINAL.includes(s.status);

function persistUI() {
  try {
    const term = slots.filter(isTerminalSlot).slice(-30);
    const live = slots.filter((s) => !isTerminalSlot(s));
    localStorage.setItem('mc_ui_v1', JSON.stringify({ slots: [...live, ...term], groups, groupSeq, slotSeq }));
  } catch { /* приватный режим — просто не сохраняем */ }
}

function restoreUI() {
  try {
    const raw = localStorage.getItem('mc_ui_v1');
    if (!raw) return;
    const data = JSON.parse(raw);
    slots = Array.isArray(data.slots) ? data.slots : [];
    groups = Array.isArray(data.groups) ? data.groups : [];
    groupSeq = data.groupSeq || groups.length;
    slotSeq = data.slotSeq || slots.length;
    // Задачи прошлого запуска мертвы вместе с демоном — помечаем честно.
    // Развёрнутые логи закрываем: контент подтянется заново при клике.
    slots.forEach((s) => {
      if (s.status === 'active') { s.status = 'failed'; s.stage = T('interrupted'); s.progress = 0; }
      s.open = false;
    });
    // Чистим ссылки на удалённые группы.
    const gids = new Set(groups.map((g) => g.id));
    slots.forEach((s) => { if (s.groupId && !gids.has(s.groupId)) s.groupId = null; });
  } catch { slots = []; groups = []; }
}

function groupName(g) { return g.name; }
function groupOf(slot) { return groups.find((g) => g.id === slot.groupId) || null; }

/* Планировщик: вне групп — сразу, внутри группы — строго по очереди. */
function pumpQueue() {
  slots.filter((s) => s.status === 'waiting' && !s.groupId).forEach((s) => { void startSlot(s); });
  groups.forEach((g) => {
    const members = slots.filter((s) => s.groupId === g.id);
    if (members.some((s) => s.status === 'active')) return;
    const next = members.filter((s) => s.status === 'waiting').sort((a, b) => a.createdAt - b.createdAt)[0];
    if (next) void startSlot(next);
  });
}

async function startSlot(slot) {
  if (slot.status !== 'waiting') return;
  slot.status = 'active';
  slot.stage = T('queued');
  slot.progress = 0;
  paintSlot(slot);
  persistUI();
  try {
    const { task_id } = await backend.download({ url: slot.url, fields: slot.fields });
    slot.taskId = task_id;
    // Слот могли удалить/расформировать, пока шёл запрос, — проверяем.
    if (!slots.includes(slot)) return;
    subscribeSlot(slot);
  } catch (e) {
    if (!slots.includes(slot)) return;
    slot.status = 'failed';
    slot.stage = `${T('tErr')}: ${e.message}`;
    renderSlots();
    persistUI();
    pumpQueue();
  }
}

/* Опрос задачи до терминального статуса. Переживает transient-сбои сети
   (бюджет последовательных ошибок), а пропавшую задачу (404) и молчание
   демона фиксирует честно — вместо вечного зависания слота. */
function pollTaskUntilTerminal(taskId, onSnap) {
  let fails = 0;
  const timer = setInterval(async () => {
    let snap;
    try {
      snap = await backend.req('GET', `/api/tasks/${taskId}`);
    } catch (e) {
      if (e && e.status === 404) {
        clearInterval(timer);
        onSnap({ status: 'failed', stage: T('taskLost'), progress: 0 });
        return;
      }
      fails += 1;
      if (fails >= 30) {
        clearInterval(timer);
        onSnap({ status: 'failed', stage: T('taskLost'), progress: 0 });
      }
      return;
    }
    fails = 0;
    onSnap(snap);
    if (TERMINAL.includes(snap.status)) clearInterval(timer);
  }, 1000);
  return () => clearInterval(timer);
}

function stopSlotLive(slot) {
  if (slot._stopLive) {
    try { slot._stopLive(); } catch { /* ignore */ }
    slot._stopLive = null;
  }
}

function subscribeSlot(slot) {
  slot._lastSnapAt = Date.now();
  const onSnap = (snap) => {
    if (!slots.includes(slot)) { stopSlotLive(slot); return; }
    slot._lastSnapAt = Date.now();
    slot.title = snap.title || snap.source || slot.title;
    slot.stage = snap.stage || snap.status;
    slot.progress = snap.progress || 0;
    if (TERMINAL.includes(snap.status)) {
      slot.status = snap.status;
      stopSlotLive(slot);
      trimTerminal();
      renderSlots();
      persistUI();
      pumpQueue();
    } else {
      slot.status = 'active';
      paintSlot(slot);
    }
  };
  if (MOCK) {
    mockApi.subscribe(slot.taskId, onSnap);
    return;
  }
  stopSlotLive(slot);
  const stops = [];
  const es = new EventSource(backend.eventsUrl(slot.taskId));
  es.onmessage = (ev) => {
    try {
      const snap = JSON.parse(ev.data);
      onSnap(snap);
      if (TERMINAL.includes(snap.status)) es.close();
    } catch { /* keep-alive */ }
  };
  es.onerror = () => {
    es.close();
    // SSE оборвался до терминального статуса — добираем опросом.
    stops.push(pollTaskUntilTerminal(slot.taskId, onSnap));
  };
  stops.push(() => { try { es.close(); } catch { /* ignore */ } });
  // Сторож тишины: соединение может молча зависнуть без error —
  // тогда дёргаем API сами, слот не бросаем.
  const watch = setInterval(async () => {
    if (!slots.includes(slot) || isTerminalSlot(slot)) { stopSlotLive(slot); return; }
    if (Date.now() - (slot._lastSnapAt || 0) < 20000) return;
    try {
      onSnap(await backend.req('GET', `/api/tasks/${slot.taskId}`));
    } catch (e) {
      if (e && e.status === 404) {
        onSnap({ status: 'failed', stage: T('taskLost'), progress: 0 });
      }
      // transient — ждём следующий тик, слот не бросаем
    }
  }, 5000);
  stops.push(() => clearInterval(watch));
  slot._stopLive = () => { stops.forEach((fn) => { try { fn(); } catch { /* ignore */ } }); };
}

function trimTerminal() {
  const term = slots.filter(isTerminalSlot);
  if (term.length > 30) {
    const drop = new Set(term.slice(0, term.length - 30).map((s) => s.key));
    slots = slots.filter((s) => !drop.has(s.key));
  }
}

function createDownloadSlot(url, fields) {
  const slot = {
    key: `s${Date.now()}_${slotSeq++}`,
    url, fields,
    status: 'waiting',
    taskId: null,
    groupId: null,
    createdAt: Date.now(),
    title: url,
    stage: T('queued'),
    progress: 0,
  };
  slots.unshift(slot);
  renderSlots();
  persistUI();
  pumpQueue();
  return slot;
}

async function startDownload() {
  const url = el('url').value.trim();
  if (!url) return;
  let fields;
  const savedId = cselectGet('dl-saved-preset');
  if (savedId) {
    const p = savedPresetsCache.find((x) => x.id === savedId);
    if (p && p.fields) fields = { ...p.fields };
    else fields = collectDlFields();
  } else {
    fields = collectDlFields();
  }
  el('url').value = '';
  el('dlpanel').classList.add('hidden');
  edgeFlash();
  // Слот появляется вместо заголовка с подсказками.
  showView('home');
  createDownloadSlot(url, fields);
}

/* ---------- Рендер слотов ---------- */
const logTimers = {};

function stopAllLogPolls() {
  Object.keys(logTimers).forEach((k) => { clearInterval(logTimers[k]); delete logTimers[k]; });
}

async function fetchSlotLogs(slot) {
  const div = el(`slot-${slot.key}`);
  const pre = div && div.querySelector('.slot-log');
  if (!pre) return;
  if (slot.status === 'waiting' || !slot.taskId) {
    pre.textContent = T('logWaiting');
    return;
  }
  try {
    const snap = await backend.task(slot.taskId);
    const lines = snap.log_tail || [];
    pre.textContent = lines.length ? lines.join('\n') : T('logEmpty');
    pre.scrollTop = pre.scrollHeight;
  } catch (e) {
    pre.textContent = `${T('tErr')}: ${e.message}`;
  }
}

function syncLogPolls() {
  stopAllLogPolls();
  slots.forEach((s) => {
    if (!s.open) return;
    void fetchSlotLogs(s);
    if (s.status === 'active' && s.taskId) {
      logTimers[s.key] = setInterval(() => { void fetchSlotLogs(s); }, 1000);
    }
  });
}

/* ---------- Логи на весь экран (двойной клик/тап) ---------- */
let logViewerSlot = null;
let logViewerTimer = null;

async function refreshLogViewer() {
  const body = el('logviewer-body');
  const s = logViewerSlot;
  if (!s || !slots.includes(s)) return;
  el('logviewer-title').textContent = s.title || s.url;
  const live = s.status === 'active';
  el('logviewer-dot').classList.toggle('hidden', !live);
  if (s.status === 'waiting' || !s.taskId) {
    body.textContent = T('logWaiting');
    return;
  }
  try {
    const snap = await backend.task(s.taskId);
    if (logViewerSlot !== s) return;
    const lines = snap.log_tail || [];
    body.textContent = lines.length ? lines.join('\n') : T('logEmpty');
    body.scrollTop = body.scrollHeight;
  } catch (e) {
    body.textContent = `${T('tErr')}: ${e.message}`;
  }
}

function openLogViewer(slot) {
  closeLogViewer();
  logViewerSlot = slot;
  el('logviewer').classList.remove('hidden');
  void refreshLogViewer();
  if (slot.status === 'active' && slot.taskId) {
    logViewerTimer = setInterval(() => {
      if (!logViewerSlot || !slots.includes(logViewerSlot)) {
        closeLogViewer();
        return;
      }
      void refreshLogViewer();
      if (logViewerSlot.status !== 'active') {
        clearInterval(logViewerTimer);
        logViewerTimer = null;
      }
    }, 1000);
  }
}

function closeLogViewer() {
  if (logViewerTimer) {
    clearInterval(logViewerTimer);
    logViewerTimer = null;
  }
  logViewerSlot = null;
  el('logviewer').classList.add('hidden');
}

// Одинарный клик — инлайн-разворот, двойной (два клика <260мс) — весь экран.
let slotClickTimer = null;
function handleSlotMainClick(slot, div) {
  if (slotClickTimer) {
    clearTimeout(slotClickTimer);
    slotClickTimer = null;
    openLogViewer(slot);
    return;
  }
  slotClickTimer = setTimeout(() => {
    slotClickTimer = null;
    slot.open = !slot.open;
    div.classList.toggle('open', slot.open);
    if (slot.open) {
      void fetchSlotLogs(slot);
      if (slot.status === 'active' && slot.taskId && !logTimers[slot.key]) {
        logTimers[slot.key] = setInterval(() => { void fetchSlotLogs(slot); }, 1000);
      }
    } else if (logTimers[slot.key]) {
      clearInterval(logTimers[slot.key]);
      delete logTimers[slot.key];
    }
  }, 260);
}

function slotCard(slot, idx) {
  const div = document.createElement('div');
  div.className = 'dl-card slot' + (slot.status === 'active' ? ' active' : '') +
    (slot.status === 'failed' ? ' failed' : '') + (slot.status === 'waiting' ? ' waiting' : '') +
    (slot.open ? ' open' : '');
  div.id = `slot-${slot.key}`;
  div.style.setProperty('--i', idx);

  const main = document.createElement('div');
  main.className = 'slot-main';
  main.title = slot.url;
  main.onclick = () => handleSlotMainClick(slot, div);

  const title = document.createElement('div');
  title.className = 'dl-title';
  title.textContent = slot.title || slot.url;
  title.title = slot.url;
  main.appendChild(title);

  const stage = document.createElement('div');
  stage.className = 'dl-stage';
  main.appendChild(stage);
  div.appendChild(main);

  const bar = document.createElement('progress');
  bar.max = 100;
  bar.value = slot.progress || 0;
  div.appendChild(bar);

  const meta = document.createElement('div');
  meta.className = 'dl-meta';
  meta.innerHTML = '<span class="st"></span><span class="pc"></span>';
  div.appendChild(meta);

  const foot = document.createElement('div');
  foot.className = 'slot-foot';
  const act = document.createElement('button');
  act.className = 'btn small' + (slot.status === 'active' ? '' : ' danger');
  if (slot.status === 'active') {
    act.textContent = T('cancel');
    act.onclick = async () => {
      try { await backend.cancel(slot.taskId); } catch (e) { toast(`${T('tErr')}: ${e.message}`, true); }
    };
  } else {
    act.textContent = T('removeSlot');
    act.onclick = () => {
      stopSlotLive(slot);
      slots = slots.filter((s) => s !== slot);
      selected.delete(slot.key);
      renderSlots();
      persistUI();
      pumpQueue();
    };
  }
  foot.appendChild(act);
  const sp = document.createElement('span');
  sp.className = 'spacer';
  foot.appendChild(sp);
  const g = groupOf(slot);
  if (g) {
    const tag = document.createElement('span');
    tag.className = 'gtag';
    tag.textContent = groupName(g);
    tag.title = groupName(g);
    foot.appendChild(tag);
  }
  if (!isTerminalSlot(slot)) {
    const sel = document.createElement('button');
    sel.type = 'button';
    sel.className = 'selbox' + (selected.has(slot.key) ? ' on' : '');
    sel.title = T('groupBtn');
    sel.onclick = () => {
      if (selected.has(slot.key)) selected.delete(slot.key);
      else selected.add(slot.key);
      renderSlots();
    };
    foot.appendChild(sel);
  }
  div.appendChild(foot);
  const logwrap = document.createElement('div');
  logwrap.className = 'slot-logwrap';
  const logpre = document.createElement('pre');
  logpre.className = 'slot-log';
  logwrap.appendChild(logpre);
  div.appendChild(logwrap);
  paintSlotInto(slot, div);
  return div;
}

function paintSlotInto(slot, div) {
  div.querySelector('.dl-title').textContent = slot.title || slot.url;
  const stageEl = div.querySelector('.dl-stage');
  stageEl.innerHTML = '';
  if (slot.status === 'active') {
    const dot = document.createElement('span');
    dot.className = 'livedot';
    stageEl.appendChild(dot);
  }
  stageEl.appendChild(document.createTextNode(slot.stage || slot.status));
  div.querySelector('progress').value = slot.progress || 0;
  const st = div.querySelector('.st');
  if (st) st.textContent = slot.status === 'waiting' ? T('waiting') : slot.status;
  div.querySelector('.pc').textContent = `${(slot.progress || 0).toFixed(1)}%`;
}

function paintSlot(slot) {
  const div = el(`slot-${slot.key}`);
  if (div) paintSlotInto(slot, div);
}

function renderSlots() {
  const empty = el('home-empty');
  const wrap = el('slots-wrap');
  const box = el('slots');
  const gbox = el('groups');
  stopAllLogPolls();
  if (!slots.length) {
    empty.classList.remove('hidden');
    wrap.classList.add('hidden');
    return;
  }
  empty.classList.add('hidden');
  wrap.classList.remove('hidden');

  // Кнопка группировки.
  const gb = el('btn-group');
  if (selected.size >= 2) {
    gb.classList.remove('hidden');
    gb.textContent = `${T('groupBtn')} (${selected.size})`;
  } else {
    gb.classList.add('hidden');
  }

  // Группы — в порядке создания.
  gbox.innerHTML = '';
  const ordered = [...groups].sort((a, b) => a.createdAt - b.createdAt);
  ordered.forEach((g) => {
    const members = slots.filter((s) => s.groupId === g.id);
    if (!members.length) return;
    const gd = document.createElement('div');
    gd.className = 'group';
    const head = document.createElement('div');
    head.className = 'group-head';
    const b = document.createElement('b');
    b.textContent = groupName(g);
    const cnt = document.createElement('span');
    cnt.className = 'cnt';
    cnt.textContent = `• ${members.length}`;
    const sp = document.createElement('span');
    sp.className = 'spacer';
    const un = document.createElement('button');
    un.className = 'btn small';
    un.textContent = T('ungroup');
    un.onclick = () => {
      members.forEach((m) => { m.groupId = null; });
      renderSlots();
      persistUI();
      pumpQueue();
    };
    head.appendChild(b);
    head.appendChild(cnt);
    head.appendChild(sp);
    head.appendChild(un);
    gd.appendChild(head);
    const grid = document.createElement('div');
    grid.className = 'cards';
    members.forEach((m, i) => grid.appendChild(slotCard(m, i)));
    gd.appendChild(grid);
    gbox.appendChild(gd);
  });

  // Вне групп.
  box.innerHTML = '';
  slots.filter((s) => !s.groupId).forEach((s, i) => box.appendChild(slotCard(s, i)));
  syncLogPolls();
}

/* ================= Главная ================= */
function fmtSize(n) {
  if (n == null) return '';
  if (n < 1024) return `${n} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1073741824) return `${(n / 1048576).toFixed(1)} MB`;
  return `${(n / 1073741824).toFixed(2)} GB`;
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
let savedPresetsCache = [];

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
    const data = await backend.presets();
    videoPresetsCache = data.video_presets || [];
    savedPresetsCache = data.download_presets || [];
    const opts = videoPresetsCache.map((p, i) => ({ value: p.id, label: `${i}. ${presetLabel(p)}` }));
    const plain = videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) }));
    cselectSet('preset', opts, cselectGet('preset') || 'default');
    cselectSet('set-video-preset', plain, settingsCache ? (settingsCache.video_preset || 'default') : 'default');
    refreshSavedPresetSelect();
    renderPresetsList();
  } catch {
    cselectSet('preset', [{ value: '', label: 'daemon…' }], '');
  }
}

function refreshSavedPresetSelect() {
  const cur = cselectGet('dl-saved-preset');
  const opts = [{ value: '', label: T('dlManual') }];
  if (savedPresetsCache.length) {
    savedPresetsCache.forEach((p) => opts.push({ value: p.id, label: p.name }));
  } else {
    opts[0].label = T('dlManual') + ' — ' + T('dlNoPresets');
  }
  const keep = opts.some((o) => o.value === cur) ? cur : '';
  cselectSet('dl-saved-preset', opts, keep);
  updateDlPresetDesc();
}

function updateDlPresetDesc() {
  const id = cselectGet('dl-saved-preset');
  const box = el('dl-saved-desc');
  if (!box) return;
  if (!id) {
    box.textContent = '';
    // enable manual controls
    el('preset').style.opacity = '';
    el('quality').style.opacity = '';
    return;
  }
  const p = savedPresetsCache.find((x) => x.id === id);
  if (!p) { box.textContent = ''; return; }
  const f = p.fields || {};
  const parts = [];
  if (f.video_preset) {
    const vp = videoPresetsCache.find((x) => x.id === f.video_preset);
    parts.push(vp ? presetLabel(vp) : f.video_preset);
  }
  if (f.quality) parts.push(f.quality + 'p');
  if (f.download_section) parts.push(f.download_section);
  if (f.subs_enabled) parts.push('subs:' + (f.sub_langs || ''));
  if (f.sponsorblock && f.sponsorblock !== 'off') parts.push('SponsorBlock:' + f.sponsorblock);
  box.textContent = parts.length ? parts.join(' • ') : p.name;
  // dim manual controls when preset active (they're ignored)
  el('preset').style.opacity = '0.55';
  el('quality').style.opacity = '0.55';
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
      const label = document.createElement('span');
      label.textContent = `📁 ${d}`;
      row.appendChild(label);
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
function dropFileOf(e) {
  const files = e.dataTransfer && e.dataTransfer.files;
  if (!files || !files.length) return null;
  const f = files[0];
  const p = f.path || f.name;
  if (!p || (MOCK && !f.path)) return MOCK && f.name ? f.name : null;
  return p;
}

function setupDrop(zone, onFile, hoverEl) {
  const hov = hoverEl || zone;
  zone.addEventListener('dragenter', (e) => { e.preventDefault(); hov.classList.add('dragover'); });
  zone.addEventListener('dragover', (e) => { e.preventDefault(); hov.classList.add('dragover'); });
  zone.addEventListener('dragleave', (e) => {
    if (!zone.contains(e.relatedTarget)) hov.classList.remove('dragover');
  });
  zone.addEventListener('drop', (e) => {
    e.preventDefault();
    hov.classList.remove('dragover');
    const p = dropFileOf(e);
    if (!p) { toast(T('tDropNoPath'), true); return; }
    onFile(p);
  });
}
window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());

function dropToConvert(path) {
  setConvertInput(path);
  showView('convert');
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

/* ================= Настройки: вкладки слева ================= */
function showSetPane(name) {
  document.querySelectorAll('.settab').forEach((b) => b.classList.toggle('active', b.dataset.spane === name));
  document.querySelectorAll('.setpane').forEach((p) => {
    const show = p.id === `sp-${name}`;
    p.classList.toggle('hidden', !show);
    if (show) {
      p.classList.remove('page-enter');
      void p.offsetWidth;
      p.classList.add('page-enter');
    }
  });
}

function cookiesModeOptions() {
  return [
    { value: 'none', label: T('ckNone') },
    { value: 'file', label: T('ckFile') },
    { value: 'browser', label: T('ckBrowser') },
  ];
}

function transcodeOptions() {
  return [
    { value: 'embedded', label: T('transcEmbedded') },
    { value: 'external', label: T('transcExternal') },
  ];
}

function goalOptions() {
  return (metaCache.user_goals || ['editing']).map((v) => ({ value: v, label: goalLabel(v) }));
}

function themeOptions() {
  return (metaCache.themes || []).map((t) => ({ value: t.id, label: themeLabel(t) }));
}

function rebuildSettingsSelects(cfg) {
  cfg = cfg || settingsCache || {};
  cselectSet('set-language', [
    { value: 'ru', label: 'Русский' },
    { value: 'en', label: 'English' },
    { value: 'en-US', label: 'English (US)' },
  ], cfg.language || 'ru');
  cselectSet('set-user-goal', goalOptions(), cfg.user_goal || 'editing');
  cselectSet('set-cookies-mode', cookiesModeOptions(), cfg.cookies_mode || 'none');
  cselectSet('set-cookies-browser',
    (metaCache.browsers || ['chrome']).map((v) => ({ value: v, label: v })),
    cfg.cookies_browser || 'chrome');
  cselectSet('set-proxy-mode',
    ['system', 'custom', 'none'].map((v) => ({ value: v, label: v })),
    cfg.proxy_mode || 'system');
  cselectSet('set-video-preset',
    videoPresetsCache.map((p) => ({ value: p.id, label: presetLabel(p) })),
    cfg.video_preset || 'default');
  cselectSet('set-transcode-mode', transcodeOptions(), cfg.transcode_mode || 'embedded');
  cselectSet('set-audio-format',
    (metaCache.audio_formats || ['mp3']).map((v) => ({ value: v, label: v })),
    cfg.audio_format || 'mp3');
  cselectSet('set-thumb-format',
    (metaCache.thumbnail_format || ['png']).map((v) => ({ value: v, label: v.toUpperCase() })),
    cfg.thumbnail_format || 'png');
  cselectSet('set-fragments',
    ['2', '4', '8', '16'].map((v) => ({ value: v, label: v })),
    String(cfg.concurrent_fragments || 4));
  cselectSet('set-queue-max',
    ['1', '2', '3', '4'].map((v) => ({ value: v, label: v })),
    String(cfg.bg_queue_max || 3));
  cselectSet('set-theme', themeOptions(), cfg.theme || 'cyan');
  cselectSet('set-progress-style',
    (metaCache.progress_styles || ['blocks']).map((v) => ({ value: v, label: v })),
    cfg.progress_style || 'blocks');
  cselectSet('set-logo-mode', [
    { value: 'ascii', label: T('logoAsciiMode') },
    { value: 'image', label: T('logoImgMode') },
  ], cfg.logo_mode || 'ascii');
  cselectSet('set-logo-ascii',
    (metaCache.logo_ascii || ['standard']).map((v) => ({ value: v, label: v })),
    cfg.logo_ascii_preset || 'standard');
  cselectSet('set-logo-protocol',
    (metaCache.logo_protocols || ['kitty']).map((v) => ({ value: v, label: v })),
    cfg.logo_protocol || 'kitty');
  syncConditionalRows();
}

function syncConditionalRows() {
  const cm = cselectGet('set-cookies-mode');
  el('row-cookies-file').classList.toggle('hidden', cm !== 'file');
  el('row-cookies-browser').classList.toggle('hidden', cm !== 'browser');
  const lm = cselectGet('set-logo-mode');
  el('row-logo-ascii').classList.toggle('hidden', lm !== 'ascii');
  el('row-logo-img').classList.toggle('hidden', lm !== 'image');
}

async function checkFfmpeg() {
  const box = el('ffmpeg-status');
  const path = el('set-ffmpeg-path').value.trim();
  box.className = 'muted small';
  box.textContent = T('checking');
  try {
    const r = await backend.ffmpegCheck(path);
    if (r.found) {
      box.className = 'small ff-ok';
      box.textContent = `✓ ${r.version || 'ffmpeg'} (${r.resolved})`;
    } else {
      box.className = 'small ff-bad';
      box.textContent = `✗ ${r.error || '?'} (${r.resolved})`;
    }
  } catch (e) {
    box.className = 'small ff-bad';
    box.textContent = `✗ ${e.message}`;
  }
}

async function loadSettings() {
  try {
    await loadMeta();
    const cfg = await backend.config();
    settingsCache = cfg;
    if (cfg.accent_color && hexToRgb(cfg.accent_color)) applyAccent(cfg.accent_color);
    if (cfg.language && I18N[cfg.language]) {
      if (state.lang !== cfg.language) { state.lang = cfg.language; applyI18n(); }
    }
    if (!videoPresetsCache.length) {
      try {
        const { video_presets } = await backend.presets();
        videoPresetsCache = video_presets || [];
      } catch { /* ignore */ }
    }
    el('set-download-dir').value = cfg.download_dir || '';
    el('set-sub-langs').value = cfg.sub_langs || '';
    el('set-proxy-url').value = cfg.proxy_url || '';
    el('set-cookies-file').value = cfg.cookies_file || '';
    el('set-archive-file').value = cfg.archive_file || '';
    el('set-ffmpeg-path').value = cfg.ffmpeg_path || '';
    el('set-logo-image').value = cfg.logo_image_path || '';
    el('set-default-editor').value = cfg.default_editor || '';
    el('set-no-mtime').checked = !!cfg.no_mtime;
    el('set-win-names').checked = !!cfg.windows_filenames;
    el('set-archive').checked = !!cfg.use_archive;
    el('set-ffmpeg-suffix').checked = cfg.use_ffmpeg_suffix !== false;
    el('set-overwrite').checked = !!cfg.overwrite_original;
    el('set-terminal-bg').checked = cfg.use_terminal_bg !== false;
    el('set-notify-bell').checked = cfg.notify_bell !== false;
    el('set-auto-check-deps').checked = cfg.auto_check_deps !== false;
    rebuildSettingsSelects(cfg);
    void checkFfmpeg();
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
    if (!el('logviewer').classList.contains('hidden')) { closeLogViewer(); return; }
    if (!el('lightbox').classList.contains('hidden')) { closeLightbox(); return; }
    if (document.querySelector('.cselect.open')) { closeAllSelects(); return; }
    if (!el('history-drawer').classList.contains('hidden')) { closeHistory(); return; }
    const current = document.querySelector('.view:not(.hidden)');
    if (current && current.id !== 'view-home') { showView('home'); return; }
    return;
  }
  if (e.altKey && !e.ctrlKey && !e.shiftKey && !e.metaKey && !typing) {
     if (e.code === 'Digit1') { e.preventDefault(); showView('home'); return; }
     if (e.code === 'Digit2') { e.preventDefault(); showView('library'); return; }
     if (e.code === 'Digit3') { e.preventDefault(); showView('convert'); return; }
     if (e.code === 'Digit4') { e.preventDefault(); showView('doctor'); return; }
   }
   // Ctrl+Shift+I — настройки пресета (требование: перехват DevTools)
   if (e.ctrlKey && e.shiftKey && !e.altKey && !e.metaKey && !typing && e.code === 'KeyI') {
     e.preventDefault();
     showView('presets');
     return;
   }
   if (e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey && !typing) {
     if (e.code === 'KeyH') {
       e.preventDefault();
       if (el('history-drawer').classList.contains('hidden')) openHistory();
       else closeHistory();
     } else if (e.code === 'KeyI') {
       e.preventDefault();
       showView('settings');
     }
   }
 });

/* ================= Guard закрытия окна ================= */
// Считает активные работы: слоты + задачи демона вне слотов (конвертации).
// main-процесс спрашивает это перед закрытием и показывает «Вы уверены?».
async function countActiveWork() {
  const ids = new Set();
  let n = 0;
  slots.forEach((s) => {
    if (s.status === 'active') {
      n += 1;
      if (s.taskId) ids.add(s.taskId);
    }
  });
  try {
    const { tasks } = await backend.tasks();
    (tasks || []).forEach((t) => {
      if ((t.status === 'running' || t.status === 'queued') && !ids.has(t.id)) n += 1;
    });
  } catch { /* daemon недоступен — считаем только слоты */ }
  return n;
}

if (window.mediacliGuard) {
  window.mediacliGuard.onQueryActive(async () => {
    let n = -1;
    try {
      n = await countActiveWork();
    } catch {
      n = -1;
    }
    window.mediacliGuard.reportActive({ count: n, lang: state.lang });
  });
}

if (window.mediacliPreset) {
  window.mediacliPreset.onOpenPresetSettings(() => {
    showView('presets');
  });
}

/* ================= Размер слотов: Ctrl + колесо ================= */
function getSlotMin() {
  try {
    const v = parseInt(getComputedStyle(document.documentElement).getPropertyValue('--slotmin'), 10);
    return Number.isFinite(v) && v > 0 ? v : 190;
  } catch { return 190; }
}

/* ================= Init ================= */
async function init() {
  buildSwatches();
  cselectInit('preset');
  cselectInit('quality');
  cselectInit('dl-saved-preset', () => updateDlPresetDesc());
  cselectInit('conv-preset');
  cselectInit('set-language');
  cselectInit('set-user-goal');
  cselectInit('set-cookies-mode', () => syncConditionalRows());
  cselectInit('set-cookies-browser');
  cselectInit('set-video-preset');
  cselectInit('set-transcode-mode');
  cselectInit('set-audio-format');
  cselectInit('set-thumb-format');
  cselectInit('set-proxy-mode');
  cselectInit('set-fragments');
  cselectInit('set-queue-max');
  cselectInit('set-theme');
  cselectInit('set-progress-style');
  cselectInit('set-logo-mode', () => syncConditionalRows());
  cselectInit('set-logo-ascii');
  cselectInit('set-logo-protocol');
  cselectSet('quality', qualityOptions(), '');
  document.querySelectorAll('.settab').forEach((b) => {
    b.onclick = () => showSetPane(b.dataset.spane);
  });

  try {
    const slotmin = localStorage.getItem('mc_slotmin_v1');
    if (slotmin && parseInt(slotmin, 10) > 0) {
      document.documentElement.style.setProperty('--slotmin', `${parseInt(slotmin, 10)}px`);
    }
  } catch { /* ignore */ }
  restoreUI();

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
  await loadMeta();
  renderSlots();
  pumpQueue();

  // Главная: топбар.
  el('btn-dl-start').onclick = () => { void startDownload(); };
  el('url').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') void startDownload();
  });
  el('btn-dl-settings').onclick = () => {
    const p = el('dlpanel');
    const willShow = p.classList.contains('hidden');
    p.classList.toggle('hidden');
    if (willShow) void loadVideoPresets();
  };
  el('btn-dl-save-preset').onclick = () => { void saveCurrentDlAsPreset(); };

  // Drag & drop: всё окно главной ведёт в конвертацию.
  setupDrop(el('view-home'), (p) => dropToConvert(p), el('home-empty'));
  setupDrop(el('conv-drop'), (p) => setConvertInput(p));

  // Ctrl + колесо — размер слотов.
  el('view-home').addEventListener('wheel', (e) => {
    if (!e.ctrlKey) return;
    e.preventDefault();
    const next = Math.min(320, Math.max(140, getSlotMin() + (e.deltaY < 0 ? 15 : -15)));
    document.documentElement.style.setProperty('--slotmin', `${next}px`);
    try { localStorage.setItem('mc_slotmin_v1', String(next)); } catch { /* ignore */ }
  }, { passive: false });

  // Группировка и очистка.
  el('btn-group').onclick = () => {
    const keys = slots.filter((s) => selected.has(s.key))
      .sort((a, b) => a.createdAt - b.createdAt).map((s) => s.key);
    if (keys.length < 2) return;
    groupSeq += 1;
    const g = { id: `g${Date.now()}_${groupSeq}`, name: `${T('groupName')} ${groupSeq}`, createdAt: Date.now() };
    groups.push(g);
    slots.forEach((s) => { if (keys.includes(s.key)) s.groupId = g.id; });
    selected.clear();
    renderSlots();
    persistUI();
    pumpQueue();
  };
  el('btn-clear-done').onclick = () => {
    const drop = new Set(slots.filter(isTerminalSlot).map((s) => s.key));
    slots = slots.filter((s) => !drop.has(s.key));
    renderSlots();
    persistUI();
  };

  // Библиотека.
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
      subscribeConvert(task_id);
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
  // Логи на весь экран.
  el('btn-logviewer-close').onclick = closeLogViewer;
  el('lightbox').addEventListener('click', (e) => {
    if (e.target === el('lightbox')) closeLightbox();
  });

  // Система.
  el('btn-doctor-refresh').onclick = loadDoctor;

  // Пресеты (Ctrl+Shift+I).
  el('btn-presets-refresh').onclick = () => { void loadVideoPresets(); };
  el('btn-presets-create').onclick = () => { void saveCurrentDlAsPreset(); };

  // Настройки.
  el('set-accent').addEventListener('input', (e) => {
    const v = e.target.value.trim();
    if (hexToRgb(v)) applyAccent(v);
    else el('accent-prev').style.background = 'transparent';
  });
  el('btn-ffmpeg-check').onclick = () => { void checkFfmpeg(); };
  el('btn-settings-reset').onclick = async () => {
    if (!confirm(T('confirmReset'))) return;
    try {
      settingsCache = await backend.resetConfig();
      if (settingsCache.language && I18N[settingsCache.language]) state.lang = settingsCache.language;
      if (settingsCache.accent_color && hexToRgb(settingsCache.accent_color)) state.accent = settingsCache.accent_color;
      applyI18n();
      applyAccent(state.accent);
      await loadSettings();
      toast(T('tSaved'));
    } catch (e) { toast(`${T('tSaveFail')} ${e.message}`, true); }
  };
  el('btn-settings-save').onclick = async () => {
    if (!settingsCache) return;
    const accentRaw = el('set-accent').value.trim();
    const accent = accentRaw === '' ? state.accent : accentRaw;
    if (!hexToRgb(accent)) { toast(T('tBadAccent'), true); return; }
    applyAccent(accent);
    const cookiesMode = cselectGet('set-cookies-mode');
    const logoMode = cselectGet('set-logo-mode');
    const proxyModeRaw = cselectGet('set-proxy-mode');
    const proxyMode = (proxyModeRaw === 'system' || proxyModeRaw === 'custom' || proxyModeRaw === 'none')
      ? proxyModeRaw : (settingsCache.proxy_mode || 'system');
    const cfg = {
      ...settingsCache,
      download_dir: el('set-download-dir').value.trim(),
      language: cselectGet('set-language'),
      user_goal: cselectGet('set-user-goal') || 'editing',
      cookies_mode: cookiesMode || 'none',
      cookies_file: cookiesMode === 'file' ? el('set-cookies-file').value.trim() : (settingsCache.cookies_file || ''),
      cookies_browser: cookiesMode === 'browser' ? (cselectGet('set-cookies-browser') || 'chrome') : (settingsCache.cookies_browser || 'chrome'),
      proxy_mode: proxyMode,
      proxy_url: el('set-proxy-url').value.trim(),
      use_archive: el('set-archive').checked,
      archive_file: el('set-archive-file').value.trim(),
      video_preset: cselectGet('set-video-preset'),
      transcode_mode: cselectGet('set-transcode-mode') || 'embedded',
      audio_format: cselectGet('set-audio-format'),
      sub_langs: el('set-sub-langs').value.trim(),
      thumbnail_format: cselectGet('set-thumb-format') || 'png',
      use_ffmpeg_suffix: el('set-ffmpeg-suffix').checked,
      overwrite_original: el('set-overwrite').checked,
      concurrent_fragments: parseInt(cselectGet('set-fragments'), 10),
      bg_queue_max: parseInt(cselectGet('set-queue-max'), 10),
      no_mtime: el('set-no-mtime').checked,
      windows_filenames: el('set-win-names').checked,
      ffmpeg_path: el('set-ffmpeg-path').value.trim(),
      theme: cselectGet('set-theme') || 'cyan',
      progress_style: cselectGet('set-progress-style') || 'blocks',
      use_terminal_bg: el('set-terminal-bg').checked,
      notify_bell: el('set-notify-bell').checked,
      auto_check_deps: el('set-auto-check-deps').checked,
      logo_mode: logoMode || 'ascii',
      logo_ascii_preset: cselectGet('set-logo-ascii') || 'standard',
      logo_protocol: cselectGet('set-logo-protocol') || 'kitty',
      logo_image_path: logoMode === 'image' ? el('set-logo-image').value.trim() : (settingsCache.logo_image_path || ''),
      default_editor: el('set-default-editor').value.trim(),
      accent_color: state.accent,
    };
    try {
      settingsCache = await backend.saveConfig(cfg);
      if (settingsCache.language && I18N[settingsCache.language] && settingsCache.language !== state.lang) {
        state.lang = settingsCache.language;
      }
      applyI18n();
      void checkFfmpeg();
      toast(T('tSaved'));
    } catch (e) { toast(`${T('tSaveFail')} ${e.message}`, true); }
  };
}

function subscribeConvert(id) {
  const onSnap = (snap) => paintConvertTask(id, snap);
  if (MOCK) {
    mockApi.subscribe(id, onSnap);
    return;
  }
  const es = new EventSource(backend.eventsUrl(id));
  es.onmessage = (ev) => {
    try {
      const snap = JSON.parse(ev.data);
      onSnap(snap);
      if (TERMINAL.includes(snap.status)) es.close();
    } catch { /* keep-alive */ }
  };
  es.onerror = () => {
    es.close();
    pollTaskUntilTerminal(id, onSnap);
  };
}

init();
