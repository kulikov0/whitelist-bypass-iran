import type { ChildProcess } from 'child_process';

export interface PortPair {
  pion: number;
}

export interface TabState {
  relay: ChildProcess | null;
  pionPort: number;
}

export interface CallInfo {
  joinLink?: string;
  protocol?: string;
}

export interface Webview extends Electron.WebviewTag {
  getURL(): string;
  setAudioMuted(muted: boolean): void;
  executeJavaScript(code: string): Promise<any>;
  reload(): void;
}

export interface RendererTab {
  relayLogs: string;
  name: string;
  headless?: boolean;
  headlessStatus?: string;
  callInfo?: CallInfo;
  tunnelConnected?: boolean;
  loginWebview?: Webview;
}

export interface RelayLogData {
  tabId: string;
  msg: string;
}

export interface Bridge {
  onRelayLog(cb: (tabId: string, msg: string) => void): void;
  closeTab(tabId: string): Promise<void>;
  getCookies(domain: string): Promise<Electron.Cookie[]>;
  startHeadless(tabId: string): Promise<void>;
  onBaleLoginRequired(cb: (tabId: string, url: string) => void): void;
  onBaleLoginDone(cb: (tabId: string) => void): void;
}
