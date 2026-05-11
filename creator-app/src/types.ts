import type { ChildProcess } from 'child_process';

export enum TunnelMode {
  PionVideo = 'pion-video',
  HeadlessBale = 'headless-bale',
}

export enum Platform {
  Bale = 'bale',
}

export enum RelayMode {
  BaleVideoCreator = 'bale-video-creator',
}

export enum CallStatus {
  Active = 'active',
  Inactive = 'inactive',
}

export enum LogPanel {
  Relay = 'relay',
  Hook = 'hook',
}

export interface PortPair {
  pion: number;
}

export interface TabState {
  relay: ChildProcess | null;
  tunnelMode: TunnelMode;
  platform: Platform;
  pionPort: number;
}

export interface TabListEntry {
  id: string;
  platform: Platform;
  mode: TunnelMode;
  callStatus: CallStatus;
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
  wv: Webview | null;
  url: string;
  mode: TunnelMode;
  relayLogs: string;
  hookLogs: string;
  name: string;
  platform?: Platform;
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
  getHookCode(tabId: string, url: string): Promise<string>;
  setTunnelMode(tabId: string, mode: string, platform?: string): Promise<void>;
  startRelay(tabId: string): Promise<void>;
  closeTab(tabId: string): Promise<void>;
  getCallCreatorCode(scriptFile: string): Promise<string>;
  getCookies(domain: string): Promise<Electron.Cookie[]>;
  startHeadless(tabId: string, platform: string): Promise<void>;
  onBaleLoginRequired(cb: (tabId: string, url: string) => void): void;
  onBaleLoginDone(cb: (tabId: string) => void): void;
}
