'use strict';
// Minimal preload: keeps contextIsolation on. The renderer talks to the
// loopback daemon via fetch/EventSource directly (CORS is open on 127.0.0.1
// + bearer token auth), so no privileged bridge is needed yet.
// mediacliGuard: main-процесс спрашивает renderer об активных задачах,
// чтобы при закрытии окна показать «Вы уверены?».
import { contextBridge, ipcRenderer } from 'electron';
import type { ActiveReport } from './types';

contextBridge.exposeInMainWorld('mediacliShell', {
  version: '1.0.0',
});

contextBridge.exposeInMainWorld('mediacliGuard', {
  onQueryActive: (cb: () => void) => ipcRenderer.on('mediacli:query-active', () => cb()),
  reportActive: (payload: ActiveReport) => ipcRenderer.send('mediacli:active-report', payload),
});

contextBridge.exposeInMainWorld('mediacliPreset', {
  onOpenPresetSettings: (cb: () => void) => ipcRenderer.on('mediacli:open-preset-settings', () => cb()),
});
