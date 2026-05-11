import { RendererTabManager } from './tab-manager';
import {
  renderTabs,
  renderContent,
  loadURL,
  startHookLogPoller,
  closeError,
  clearLog,
  saveLogs,
  exportCookies,
  copyToClipboard,
  renameTab,
  attachLoginWebview,
  detachLoginWebview,
} from './dom';
import { BALE_URL } from '../constants';
import { Platform, Bridge, LogPanel } from '../types';

declare const window: Window & { bridge: Bridge };

const tm = new RendererTabManager(() => {
  renderTabs(tm);
  renderContent(tm);
});

function bindTabBarEvents(): void {
  document.getElementById('tabBar')!.addEventListener('click', (event) => {
    const target = event.target as HTMLElement;
    const action = target.dataset.action;
    const tabEl = target.closest('[data-tab-id]') as HTMLElement | null;
    const tabId = tabEl?.dataset.tabId;

    if (action === 'add-tab') {
      tm.createTab();
      return;
    }
    if (action === 'close' && tabId) {
      event.stopPropagation();
      tm.closeTab(tabId);
      return;
    }
    if (action === 'rename' && tabId) {
      event.stopPropagation();
      renameTab(tm, tabId, tabEl!);
      return;
    }
    if (tabId) {
      tm.selectTab(tabId);
    }
  });
}

function bindToolbarEvents(): void {
  document.getElementById('btnBale')!.addEventListener('click', () => tm.switchToHeadless(Platform.Bale));
  document.getElementById('btnSaveLogs')!.addEventListener('click', saveLogs);
}

function bindActionBarEvents(): void {
  document.getElementById('btnBaleCookies')!.addEventListener('click', () => {
    exportCookies('bale', 'bale-cookies.json', 'No Bale cookies found.\nPlease log into Bale first.');
  });
}

function bindErrorPopup(): void {
  document.getElementById('errorPopup')!.addEventListener('click', closeError);
  document.querySelector('#errorPopup .popup')!.addEventListener('click', (event) => event.stopPropagation());
  document.getElementById('btnErrorClose')!.addEventListener('click', closeError);
}

function bindLogEvents(): void {
  document.getElementById('btnClearRelay')!.addEventListener('click', () => clearLog(LogPanel.Relay));
  document.getElementById('btnClearHook')!.addEventListener('click', () => clearLog(LogPanel.Hook));
  document.getElementById('btnSaveLogsHeadless')!.addEventListener('click', saveLogs);
}

function bindHeadlessEvents(): void {
  document.getElementById('headlessJoinLink')!.addEventListener('click', (event) => {
    copyToClipboard((event.target as HTMLElement).textContent || '');
  });
  document.getElementById('headlessInfo')!.addEventListener('click', (event) => {
    const target = event.target as HTMLElement;
    const copyTarget = target.dataset.copy;
    if (copyTarget) {
      const sourceEl = document.getElementById(copyTarget);
      if (sourceEl) copyToClipboard(sourceEl.textContent || '');
    }
  });
}

function init(): void {
  bindTabBarEvents();
  bindToolbarEvents();
  bindActionBarEvents();
  bindErrorPopup();
  bindLogEvents();
  bindHeadlessEvents();

  window.bridge.onRelayLog((tabId: string, msg: string) => {
    tm.appendRelayLog(tabId, msg);
  });

  window.bridge.onBaleLoginRequired((tabId: string, url: string) => {
    tm.showLoginWebview(tabId, url);
    attachLoginWebview(tm, tabId, url);
    if (tabId === tm.activeTabId) {
      renderTabs(tm);
      renderContent(tm);
    }
  });

  window.bridge.onBaleLoginDone((tabId: string) => {
    detachLoginWebview(tm, tabId);
    tm.hideLoginWebview(tabId);
    if (tabId === tm.activeTabId) {
      renderContent(tm);
    }
  });

  startHookLogPoller(tm);
}

init();
