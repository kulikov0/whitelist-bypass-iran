import { ipcMain, session } from 'electron';
import { TabManager } from './tab-manager';
import {
  IPC,
  SESSION_PARTITION,
  BALE_COOKIE_DOMAINS,
} from '../constants';

export function registerIpcHandlers(tabManager: TabManager): void {
  ipcMain.handle(IPC.START_HEADLESS, async (_e, tabId: string) => {
    await tabManager.startHeadless(tabId);
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
