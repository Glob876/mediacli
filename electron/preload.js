'use strict';
// Minimal preload: keeps contextIsolation on. The renderer talks to the
// loopback daemon via fetch/EventSource directly (CORS is open on 127.0.0.1
// + bearer token auth), so no privileged bridge is needed yet.
const { contextBridge } = require('electron');

contextBridge.exposeInMainWorld('mediacliShell', {
  version: '0.1.0',
});
