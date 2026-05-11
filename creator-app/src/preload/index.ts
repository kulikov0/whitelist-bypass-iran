import { ipcRenderer } from 'electron';
import { IPC } from '../constants';

(window as any).bridge = {
  onRelayLog(cb: (tabId: string, msg: string) => void) {
    ipcRenderer.on(IPC.RELAY_LOG, (_e, data) => cb(data.tabId, data.msg));
  },
  closeTab(tabId: string) {
    return ipcRenderer.invoke(IPC.CLOSE_TAB, tabId);
  },
  getCookies(domain: string) {
    return ipcRenderer.invoke(IPC.GET_COOKIES, domain);
  },
  startHeadless(tabId: string) {
    return ipcRenderer.invoke(IPC.START_HEADLESS, tabId);
  },
  onBaleLoginRequired(cb: (tabId: string, url: string) => void) {
    ipcRenderer.on(IPC.BALE_LOGIN_REQUIRED, (_e, data) => cb(data.tabId, data.url));
  },
  onBaleLoginDone(cb: (tabId: string) => void) {
    ipcRenderer.on(IPC.BALE_LOGIN_DONE, (_e, data) => cb(data.tabId));
  },
};
