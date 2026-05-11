import { RendererTab, Bridge } from '../types';
import { HeadlessLogMarker } from '../constants';

declare const window: Window & { bridge: Bridge };

export class RendererTabManager {
  tabs: Record<string, RendererTab> = {};
  activeTabId: string | null = null;
  private nextId = 1;
  private onRender: () => void;

  constructor(onRender: () => void) {
    this.onRender = onRender;
  }

  createTab(): string {
    const tabId = 'tab-' + this.nextId++;
    this.tabs[tabId] = {
      relayLogs: '',
      name: '',
    };
    this.selectTab(tabId);
    return tabId;
  }

  switchToHeadless(): void {
    if (!this.activeTabId) return;
    const tab = this.tabs[this.activeTabId];
    tab.name = 'Headless Bale';
    tab.headless = true;
    tab.callInfo = undefined;
    tab.headlessStatus = 'Starting...';
    tab.tunnelConnected = false;
    tab.relayLogs = '';
    window.bridge.startHeadless(this.activeTabId);
    this.onRender();
  }

  showLoginWebview(tabId: string, _url: string): void {
    const tab = this.tabs[tabId];
    if (!tab) return;
    tab.headlessStatus = 'Waiting for Bale login...';
    this.onRender();
  }

  hideLoginWebview(tabId: string): void {
    const tab = this.tabs[tabId];
    if (!tab) return;
    if (tab.loginWebview) {
      tab.loginWebview.remove();
      tab.loginWebview = undefined;
    }
    tab.headlessStatus = 'Starting...';
    this.onRender();
  }

  closeTab(tabId: string): void {
    window.bridge.closeTab(tabId);
    delete this.tabs[tabId];
    if (this.activeTabId === tabId) {
      const ids = Object.keys(this.tabs);
      this.activeTabId = ids.length > 0 ? ids[ids.length - 1] : null;
    }
    this.onRender();
  }

  selectTab(tabId: string): void {
    this.saveCurrentTabLogs();
    this.activeTabId = tabId;
    this.onRender();
  }

  saveCurrentTabLogs(): void {
    if (this.activeTabId && this.tabs[this.activeTabId]) {
      const relayEl = document.getElementById('relayLog');
      if (relayEl) this.tabs[this.activeTabId].relayLogs = relayEl.textContent || '';
    }
  }

  getActiveTab(): RendererTab | null {
    if (!this.activeTabId) return null;
    return this.tabs[this.activeTabId] || null;
  }

  getTabLabel(tab: RendererTab): string {
    if (tab.name) return tab.name;
    return 'New';
  }

  appendRelayLog(tabId: string, msg: string): void {
    const tab = this.tabs[tabId];
    if (!tab) return;
    tab.relayLogs += (tab.relayLogs ? '\n' : '') + msg;
    let rendered = false;
    if (tab.headless) rendered = this.parseHeadlessLog(tabId, msg);
    if (tabId === this.activeTabId && !rendered) {
      const el = document.getElementById('relayLog');
      if (el) {
        if (el.textContent!.length > 0) el.textContent += '\n';
        el.textContent += msg;
        el.scrollTop = el.scrollHeight;
      }
    }
  }

  private parseHeadlessLog(tabId: string, msg: string): boolean {
    const tab = this.tabs[tabId];
    if (!tab) return false;
    const trimmed = msg.trim();
    let changed = false;

    if (trimmed === HeadlessLogMarker.CALL_CREATED) {
      tab.callInfo = {};
      tab.headlessStatus = 'Call created';
      changed = true;
    }
    if (trimmed.includes(HeadlessLogMarker.JOIN_LINK) && tab.callInfo) {
      tab.callInfo.joinLink = trimmed.split(HeadlessLogMarker.JOIN_LINK)[1].trim();
      changed = true;
    }
    if (trimmed.includes(HeadlessLogMarker.PROTOCOL) && tab.callInfo) {
      tab.callInfo.protocol = trimmed.split(HeadlessLogMarker.PROTOCOL)[1].trim();
      changed = true;
    }
    if (trimmed.includes('[FATAL]')) {
      const fatalMessage = trimmed.split('[FATAL]')[1]?.trim() || 'fatal error';
      tab.headlessStatus = 'Disconnected: ' + fatalMessage;
      tab.tunnelConnected = false;
      changed = true;
    }
    if (changed && tabId === this.activeTabId) this.onRender();
    return changed;
  }
}
