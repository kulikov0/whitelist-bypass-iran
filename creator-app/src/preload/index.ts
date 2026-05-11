import { ipcRenderer } from 'electron';
import { IPC } from '../constants';

(window as any).bridge = {
  onRelayLog(cb: (tabId: string, msg: string) => void) {
    ipcRenderer.on(IPC.RELAY_LOG, (_e, data) => cb(data.tabId, data.msg));
  },
  getHookCode(tabId: string, url: string) {
    return ipcRenderer.invoke(IPC.GET_HOOK_CODE, tabId, url);
  },
  setTunnelMode(tabId: string, mode: string, platform?: string) {
    return ipcRenderer.invoke(IPC.SET_TUNNEL_MODE, tabId, mode, platform);
  },
  startRelay(tabId: string) {
    return ipcRenderer.invoke(IPC.START_RELAY, tabId);
  },
  closeTab(tabId: string) {
    return ipcRenderer.invoke(IPC.CLOSE_TAB, tabId);
  },
  getCallCreatorCode(scriptFile: string) {
    return ipcRenderer.invoke(IPC.GET_CALL_CREATOR_CODE, scriptFile);
  },
  getCookies(domain: string) {
    return ipcRenderer.invoke(IPC.GET_COOKIES, domain);
  },
  startHeadless(tabId: string, platform: string) {
    return ipcRenderer.invoke(IPC.START_HEADLESS, tabId, platform);
  },
  onBaleLoginRequired(cb: (tabId: string, url: string) => void) {
    ipcRenderer.on(IPC.BALE_LOGIN_REQUIRED, (_e, data) => cb(data.tabId, data.url));
  },
  onBaleLoginDone(cb: (tabId: string) => void) {
    ipcRenderer.on(IPC.BALE_LOGIN_DONE, (_e, data) => cb(data.tabId));
  },
};
