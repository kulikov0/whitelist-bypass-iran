import { ipcMain, session } from 'electron';
import * as path from 'path';
import * as fs from 'fs/promises';
import { TabManager } from './tab-manager';
import {
  IPC,
  SESSION_PARTITION,
  BALE_COOKIE_DOMAINS,
} from '../constants';
import { TunnelMode, Platform } from '../types';

export function registerIpcHandlers(tabManager: TabManager): void {
  ipcMain.handle(IPC.GET_HOOK_CODE, async (_e, tabId: string, url: string) => {
    const tab = await tabManager.getOrCreateTab(tabId);
    return tabManager.loadHook(tabId, url, tab);
  });

  ipcMain.handle(IPC.GET_CALL_CREATOR_CODE, async (_e, scriptFile: string) => {
    const filePath = path.join(__dirname, '..', '..', 'scripts', scriptFile || 'call-checker.js');
    return fs.readFile(filePath, 'utf8');
  });

  ipcMain.handle(IPC.SET_TUNNEL_MODE, (_e, tabId: string, mode: string, platform?: string) => {
    if (!Object.values(TunnelMode).includes(mode as TunnelMode)) return;
    tabManager.setTunnelMode(tabId, mode as TunnelMode, platform as Platform | undefined);
  });

  ipcMain.handle(IPC.START_RELAY, async (_e, tabId: string) => {
    const tab = await tabManager.getOrCreateTab(tabId);
    tabManager.startRelay(tabId, tab);
  });

  ipcMain.handle(IPC.START_HEADLESS, async (_e, tabId: string, platform: string) => {
    await tabManager.startHeadless(tabId, platform as Platform);
  });

  ipcMain.handle(IPC.CLOSE_TAB, (_e, tabId: string) => {
    tabManager.deleteTab(tabId);
  });

  ipcMain.handle(IPC.GET_COOKIES, async (_e, _domain: string) => {
    const ses = session.fromPartition(SESSION_PARTITION);
    const all = await ses.cookies.get({});
    const filtered = all.filter((cookie) => {
      return cookie.domain != null && BALE_COOKIE_DOMAINS.some((d) => cookie.domain!.includes(d));
    });
    console.log(`[COOKIES] total: ${all.length} bale: ${filtered.length}`);
    return filtered;
  });
}
