import { Bridge, Webview } from '../types';
import { SESSION_PARTITION } from '../constants';
import { RendererTabManager } from './tab-manager';

declare const window: Window & { bridge: Bridge };

export function renderTabs(tm: RendererTabManager): void {
  const bar = document.getElementById('tabBar')!;
  let html = '';
  Object.keys(tm.tabs).forEach((id) => {
    const tab = tm.tabs[id];
    const label = tm.getTabLabel(tab);
    const cls = id === tm.activeTabId ? 'tab active' : 'tab';
    html +=
      `<div class="${cls}" data-tab-id="${id}">` +
      `<span class="tab-label">${escapeHtml(label)}</span>` +
      `<img class="edit" src="resources/icons8-pencil-50.png" data-action="rename">` +
      ` <span class="close" data-action="close">&#x2715;</span></div>`;
  });
  html += '<div class="tab-add" data-action="add-tab" title="New tab">+</div>';
  bar.innerHTML = html;
}

export function renderContent(tm: RendererTabManager): void {
  const welcome = document.getElementById('welcome')!;
  const toolbar = document.getElementById('toolbar')!;
  const logsPanel = document.getElementById('logsPanel')!;
  const headlessInfo = document.getElementById('headlessInfo')!;

  if (!tm.activeTabId || !tm.tabs[tm.activeTabId]) {
    welcome.style.display = 'flex';
    toolbar.style.display = 'none';
    logsPanel.style.display = 'none';
    return;
  }

  welcome.style.display = 'none';
  logsPanel.style.display = 'flex';

  const activeTab = tm.tabs[tm.activeTabId];
  if (activeTab.headless) {
    toolbar.style.display = 'none';
    headlessInfo.style.display = 'block';
    document.getElementById('headlessTitle')!.textContent = 'Headless Bale';
    document.getElementById('headlessStatus')!.textContent = activeTab.headlessStatus || 'Starting...';
    const callInfo = activeTab.callInfo;
    const callInfoEl = document.getElementById('headlessCallInfo')!;
    if (callInfo) {
      callInfoEl.style.display = 'block';
      document.getElementById('headlessJoinLink')!.textContent = callInfo.joinLink || '';
      document.getElementById('headlessProtocol')!.textContent = callInfo.protocol || '';
    } else {
      callInfoEl.style.display = 'none';
    }
  } else {
    toolbar.style.display = 'flex';
    headlessInfo.style.display = 'none';
  }

  if (activeTab.loginWebview) {
    headlessInfo.style.display = 'none';
    activeTab.loginWebview.classList.remove('hidden');
  }

  document.getElementById('relayLog')!.textContent = activeTab.relayLogs || '';
  scrollLogs();
}

export function scrollLogs(): void {
  const relayEl = document.getElementById('relayLog');
  if (relayEl) relayEl.scrollTop = relayEl.scrollHeight;
}

export function attachLoginWebview(tm: RendererTabManager, tabId: string, url: string): void {
  const tab = tm.tabs[tabId];
  if (!tab) return;
  if (tab.loginWebview) tab.loginWebview.remove();
  const webview = document.createElement('webview') as unknown as Webview;
  webview.setAttribute('src', url);
  webview.setAttribute('partition', SESSION_PARTITION);
  webview.setAttribute('useragent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36');
  webview.classList.add('webview-full');
  webview.dataset.tabId = tabId;
  document.getElementById('content')!.appendChild(webview);
  tab.loginWebview = webview;
}

export function detachLoginWebview(tm: RendererTabManager, tabId: string): void {
  const tab = tm.tabs[tabId];
  if (!tab) return;
  if (tab.loginWebview) {
    tab.loginWebview.remove();
    tab.loginWebview = undefined;
  }
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

export function showError(msg: string): void {
  document.getElementById('errorText')!.textContent = msg;
  document.getElementById('errorPopup')!.classList.add('visible');
}

export function closeError(): void {
  document.getElementById('errorPopup')!.classList.remove('visible');
}

export function clearLog(): void {
  document.getElementById('relayLog')!.textContent = '';
}

export function saveLogs(): void {
  const relay = document.getElementById('relayLog')!.textContent || '';
  const blob = new Blob([relay], { type: 'text/plain' });
  const anchor = document.createElement('a');
  anchor.href = URL.createObjectURL(blob);
  anchor.download = 'tunnel-logs-' + new Date().toISOString().replace(/[:.]/g, '-') + '.txt';
  anchor.click();
  URL.revokeObjectURL(anchor.href);
}

export function exportCookies(domain: string, filename: string, errorMsg: string): void {
  window.bridge.getCookies(domain).then((cookies) => {
    if (!cookies.length) {
      showError(errorMsg);
      return;
    }
    const simple = cookies.map((cookie) => ({ name: cookie.name, value: cookie.value }));
    const blob = new Blob([JSON.stringify(simple, null, 2)], { type: 'application/json' });
    const anchor = document.createElement('a');
    anchor.href = URL.createObjectURL(blob);
    anchor.download = filename;
    anchor.click();
    URL.revokeObjectURL(anchor.href);
  });
}

export function copyToClipboard(text: string): void {
  navigator.clipboard.writeText(text);
}

export function renameTab(tm: RendererTabManager, tabId: string, tabEl: HTMLElement): void {
  const span = tabEl.querySelector('.tab-label') as HTMLElement;
  if (!span) return;
  const input = document.createElement('input');
  input.type = 'text';
  input.value = tm.tabs[tabId]?.name || '';
  input.placeholder = tm.getTabLabel(tm.tabs[tabId]);
  input.className = 'tab-rename-input';
  span.replaceWith(input);
  input.focus();
  input.select();
  const done = () => {
    if (tm.tabs[tabId]) {
      tm.tabs[tabId].name = input.value.trim();
    }
    renderTabs(tm);
  };
  input.addEventListener('blur', done);
  input.addEventListener('keydown', (ev) => {
    if (ev.key === 'Enter') input.blur();
    if (ev.key === 'Escape') {
      input.value = '';
      input.blur();
    }
  });
}
