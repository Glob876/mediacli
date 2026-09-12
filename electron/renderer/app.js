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
    if (!res.ok) throw new Error(`${method} ${path}: ${res.status}`);
    return res.json();
  },
  presets: () => api.req('GET', '/api/presets'),
  tasks: () => api.req('GET', '/api/tasks'),
  download: (payload) => api.req('POST', '/api/downloads', payload),
  eventsUrl: (id) => `${BASE}/api/tasks/${id}/events?token=${encodeURIComponent(TOKEN)}`,
};

/* Mock for `npm run dev`: fake tasks with timer-driven progress. */
const mockApi = {
  _id: 0,
  _tasks: [],
  _listeners: {},
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
  eventsUrl: (id) => `mock:${id}`,
};
if (MOCK) {
  mockApi.subscribe = (id, cb) => {
    (mockApi._listeners[id] = mockApi._listeners[id] || []).push(cb);
  };
}

const backend = MOCK ? mockApi : api;

/* ---------------- UI ---------------- */
function cardShell(task) {
  const div = document.createElement('div');
  div.className = 'dl-card active';
  div.id = `task-${task.id}`;
  div.innerHTML = `
    <div class="dl-title"></div>
    <div class="dl-stage"></div>
    <progress max="100" value="0"></progress>
    <div class="dl-meta"><span class="st"></span><span class="pc"></span></div>`;
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
  div.classList.toggle('active', snap.status === 'running' || snap.status === 'queued');
  div.classList.toggle('failed', String(snap.status).startsWith('fail'));
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
  } catch (e) {
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
}

init();
