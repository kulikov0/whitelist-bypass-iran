import { app, BrowserWindow, session, Session } from 'electron';
import * as path from 'path';
import { TabManager } from './tab-manager';
import { SESSION_PARTITION, USER_AGENT, WINDOW_WIDTH, WINDOW_HEIGHT } from '../constants';
import { CallStatus } from '../types';

function stripCSP(ses: Session): void {
  ses.webRequest.onHeadersReceived((details, callback) => {
    const headers = { ...details.responseHeaders };
    delete headers['content-security-policy'];
    delete headers['Content-Security-Policy'];
    delete headers['content-security-policy-report-only'];
    delete headers['Content-Security-Policy-Report-Only'];
    callback({ responseHeaders: headers });
  });
}

function parseCallStatus(msg: string): { tabId: string; status: CallStatus } | null {
  const prefix = '[CALL_STATUS] ';
  const idx = msg.indexOf(prefix);
  if (idx === -1) return null;
  const parts = msg.substring(idx + prefix.length);
  const colonIdx = parts.indexOf(':');
  if (colonIdx === -1) return null;
  const status = parts.substring(colonIdx + 1);
  return {
    tabId: parts.substring(0, colonIdx),
    status: status === CallStatus.Active ? CallStatus.Active : CallStatus.Inactive,
  };
}

export function createWindow(tabManager: TabManager): BrowserWindow {
  const ses = session.fromPartition(SESSION_PARTITION);
  stripCSP(ses);
  ses.setPermissionRequestHandler((_wc, _perm, cb) => cb(true));
  ses.setPermissionCheckHandler(() => true);
  ses.setUserAgent(USER_AGENT);

  app.on('session-created', stripCSP);

  const win = new BrowserWindow({
    width: WINDOW_WIDTH,
    height: WINDOW_HEIGHT,
    icon: path.join(__dirname, '..', '..', 'resources', 'icon.png'),
    webPreferences: {
      preload: path.join(__dirname, '..', 'preload', 'index.js'),
      nodeIntegration: true,
      contextIsolation: false,
      webviewTag: true,
    },
  });

  win.loadFile('index.html');
  win.on('closed', () => {
    tabManager.mainWindow = null;
  });

  win.webContents.on('did-attach-webview', (_e, wvContents) => {
    wvContents.on('before-input-event', (_e, input) => {
      if (input.key === 'F12') wvContents.openDevTools();
    });

    wvContents.on('console-message', (_e, _level, msg) => {
      const callStatus = parseCallStatus(msg);
      if (callStatus) {
        console.log('[MAIN] Cached status for', callStatus.tabId, ':', callStatus.status);
        tabManager.setCallStatus(callStatus.tabId, callStatus.status);
      }
    });
  });

  return win;
}
