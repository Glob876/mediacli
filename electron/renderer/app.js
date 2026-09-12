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
  async presets() {
    return {
      video_presets: [
        { id: 'default', name_en: 'Original / Lossless Merge — Best Quality' },
        { id: 'standard_mp4', name_en: 'Standard MP4 (H.264 + AAC)' },
        { id: 'mkv_av1', name_en: 'Modern MKV (AV1 + Opus/AAC)' },
      ],
    };
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
};
if (MOCK) {
  mockApi.subscribe = (id, cb) => {
    (mockApi._listeners[id] = mockApi._listeners[id] || []).push(cb);
  };
}

const backend = MOCK ? mockApi : api;

/* ---------------- Tabs ---------------- */
document.querySelectorAll('.tab').forEach((btn) => {
  btn.onclick = () => {
    document.querySelectorAll('.tab').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('.tabpage').forEach((p) => p.classList.add('hidden'));
    btn.classList.add('active');
    el(`tab-${btn.dataset.tab}`).classList.remove('hidden');
    if (btn.dataset.tab === 'history') loadHistory();
    if (btn.dataset.tab === 'settings') loadSettings();
    if (btn.dataset.tab === 'doctor') loadDoctor();
  };
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
    <div class="row"><button class="btn small btn-cancel-task">Cancel</button></div>`;
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

function subscribe(id) {
  if (MOCK) {
    mockApi.subscribe(id, (snap) => paint(id, snap));
    return;
  }
  const es = new EventSource(backend.eventsUrl(id));
  es.onmessage = (ev) => {
    try {
      const snap = JSON.parse(ev.data);
      paint(id, snap);
      if (['done', 'failed', 'cancelled'].includes(snap.status)) es.close();
    } catch { /* keep-alive */ }
  };
  es.onerror = async () => {
    // SSE fallback: poll once per second.
    es.close();
    const timer = setInterval(async () => {
      try {
        const snap = await backend.req('GET', `/api/tasks/${id}`);
        paint(id, snap);
        if (['done', 'failed', 'cancelled'].includes(snap.status)) clearInterval(timer);
      } catch { clearInterval(timer); }
    }, 1000);
  };
}

/* ---------------- History ---------------- */
async function loadHistory() {
  const box = el('history');
  box.innerHTML = '<div class="muted">Loading…</div>';
  try {
    const { history } = await backend.history();
    if (!history.length) { box.innerHTML = '<div class="muted">No operations recorded yet.</div>'; return; }
    box.innerHTML = '';
    history.forEach((h) => {
      const row = document.createElement('div');
      row.className = 'hrow';
      row.innerHTML = `
        <div><b></b><div class="muted small"></div></div>
        <div class="row"><button class="btn small">Remove</button><button class="btn small danger">Delete file</button></div>`;
      row.querySelector('b').textContent = h.target || h.source;
      row.querySelector('.small').textContent = `${h.time} • ${h.type} • ${h.status}`;
      const [btnRm, btnDel] = row.querySelectorAll('button');
      btnRm.onclick = async () => { await backend.historyDelete(h, false); loadHistory(); };
      btnDel.onclick = async () => {
        if (!confirm(`Delete file from disk?\n${h.target || h.source}`)) return;
        await backend.historyDelete(h, true); loadHistory();
      };
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted">Failed: ${e.message}</div>`; }
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
  } catch (e) { alert(`Settings load failed: ${e.message}`); }
}

/* ---------------- Doctor ---------------- */
async function loadDoctor() {
  const box = el('doctor');
  box.innerHTML = '<div class="muted">Checking…</div>';
  try {
    const st = await backend.status();
    box.innerHTML = `<div class="muted">daemon ${st.version}</div>`;
    st.dependencies.forEach((d) => {
      const row = document.createElement('div');
      row.className = 'hrow';
      const mark = d.available ? 'FOUND' : 'MISSING';
      row.innerHTML = `<div><b>${d.name}</b> <span class="tag">${d.required ? 'required' : 'optional'}</span></div>
        <div class="muted small">${mark}${d.path ? ' (' + d.path + ')' : ''}</div>`;
      if (!d.available && d.required) row.classList.add('failed');
      box.appendChild(row);
    });
  } catch (e) { box.innerHTML = `<div class="muted">Failed: ${e.message}</div>`; }
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
    el('preset').innerHTML = '<option>daemon unreachable</option>';
  }

  try {
    const { tasks } = await backend.tasks();
    tasks.forEach((t) => {
      paint(t.id, t);
      if (t.status === 'running' || t.status === 'queued') subscribe(t.id);
    });
  } catch { /* fresh start, nothing to restore */ }

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
      paint(task_id, { id: task_id, title: url, source: url, status: 'running', stage: 'Queued…', progress: 0 });
      subscribe(task_id);
      el('url').value = '';
      el('confirm').classList.add('hidden');
    } catch (e) {
      alert(`Download failed to start:\n${e.message}`);
    } finally {
      el('btn-start').disabled = false;
    }
  };

  el('btn-history-refresh').onclick = loadHistory;
  el('btn-history-clear').onclick = async () => {
    if (!confirm('Clear entire operation history?')) return;
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
      alert('Preferences saved.');
    } catch (e) { alert(`Save failed: ${e.message}`); }
  };
}

init();
