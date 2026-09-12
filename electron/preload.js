'use strict';
// Minimal preload: keeps contextIsolation on. The renderer talks to the
// loopback daemon via fetch/EventSource directly (CORS is open on 127.0.0.1
// + bearer token auth), so no privileged bridge is needed yet.
// mediacliGuard: main-процесс спрашивает renderer об активных задачах,
// чтобы при закрытии окна показать «Вы уверены?».
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('mediacliShell', {
  version: '0.1.0',
});

contextBridge.exposeInMainWorld('mediacliGuard', {
  onQueryActive: (cb) => ipcRenderer.on('mediacli:query-active', () => cb()),
  reportActive: (payload) => ipcRenderer.send('mediacli:active-report', payload),
});
