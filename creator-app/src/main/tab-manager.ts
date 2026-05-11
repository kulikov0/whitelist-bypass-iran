import { app } from 'electron';
import { spawn, ChildProcess } from 'child_process';
import { BrowserWindow, session } from 'electron';
import * as net from 'net';
import * as path from 'path';
import * as fs from 'fs/promises';
import { TabState, PortPair, TabListEntry, Platform, TunnelMode, RelayMode, CallStatus } from '../types';
import {
  INITIAL_PORT_BASE,
  IPC,
  RELAY_RESTART_DELAY_MS,
  SESSION_PARTITION,
  BALE_COOKIE_DOMAINS,
  BALE_URL,
  LOG_CAPTURE_SNIPPET,
} from '../constants';

function resolveResourcePath(devRelative: string, packedName: string): string {
  if (app.isPackaged) {
    return path.join(process.resourcesPath!, packedName);
  }
  return path.join(__dirname, '..', '..', '..', devRelative);
}

function binaryName(base: string): string {
  return process.platform === 'win32' ? base + '.exe' : base;
}

export class TabManager {
  private tabs = new Map<string, TabState>();
  private callStatusCache = new Map<string, CallStatus>();
  private nextPortBase = INITIAL_PORT_BASE;
  private _mainWindow: BrowserWindow | null = null;
  private relayPath: string;
  private headlessBalePath: string;
  private hooksDir: string;

  constructor() {
    this.relayPath = resolveResourcePath(
      path.join('relay', binaryName('relay')),
      binaryName('relay'),
    );
    this.headlessBalePath = resolveResourcePath(
      path.join('headless', 'bale', binaryName('headless-bale-creator')),
      binaryName('headless-bale-creator'),
    );
    this.hooksDir = app.isPackaged
      ? path.join(process.resourcesPath!, 'hooks')
      : path.join(__dirname, '..', '..', '..', 'hooks');
  }

  get mainWindow(): BrowserWindow | null {
    return this._mainWindow;
  }

  set mainWindow(w: BrowserWindow | null) {
    this._mainWindow = w;
  }

  private isPortFree(port: number): Promise<boolean> {
    return new Promise((resolve) => {
      const server = net.createServer();
      server.once('error', () => resolve(false));
      server.once('listening', () => {
        server.close(() => resolve(true));
      });
      server.listen(port, '127.0.0.1');
    });
  }

  async allocPorts(): Promise<PortPair> {
    while (true) {
      const pion = this.nextPortBase;
      this.nextPortBase += 1;
      if (await this.isPortFree(pion)) {
        return { pion };
      }
    }
  }

  async getOrCreateTab(tabId: string): Promise<TabState> {
    if (!this.tabs.has(tabId)) {
      const ports = await this.allocPorts();
      this.tabs.set(tabId, {
        relay: null,
        tunnelMode: TunnelMode.PionVideo,
        platform: Platform.Bale,
        pionPort: ports.pion,
      });
    }
    return this.tabs.get(tabId)!;
  }

  getTab(tabId: string): TabState | undefined {
    return this.tabs.get(tabId);
  }

  deleteTab(tabId: string): void {
    const tab = this.tabs.get(tabId);
    if (tab) {
      this.killRelay(tabId, tab);
      this.tabs.delete(tabId);
    }
    this.callStatusCache.delete(tabId);
  }

  setCallStatus(tabId: string, status: CallStatus): void {
    this.callStatusCache.set(tabId, status);
  }

  getCallStatus(tabId: string): CallStatus {
    return this.callStatusCache.get(tabId) || CallStatus.Inactive;
  }

  getTabList(): TabListEntry[] {
    const result: TabListEntry[] = [];
    this.tabs.forEach((tab, tabId) => {
      result.push({
        id: tabId,
        platform: tab.platform,
        mode: tab.tunnelMode,
        callStatus: this.getCallStatus(tabId),
      });
    });
    return result;
  }

  private sendLog(tabId: string, msg: string): void {
    if (this._mainWindow && !this._mainWindow.isDestroyed()) {
      this._mainWindow.webContents.send(IPC.RELAY_LOG, { tabId, msg });
    }
  }

  private attachProcessOutput(
    proc: ChildProcess,
    tabId: string,
    inspect?: (msg: string) => void,
  ): void {
    const onData = (data: Buffer) => {
      data
        .toString()
        .trim()
        .split('\n')
        .forEach((msg) => {
          if (!msg) return;
          console.log(`[relay:${tabId}]`, msg);
          this.sendLog(tabId, msg);
          if (inspect) inspect(msg);
        });
    };
    proc.stdout?.on('data', onData);
    proc.stderr?.on('data', onData);
  }

  startRelay(tabId: string, tab: TabState): void {
    this.killRelay(tabId, tab);
    const relayMode: RelayMode = RelayMode.BaleVideoCreator;
    const proc = spawn(this.relayPath, ['--mode', relayMode, '--ws-port', String(tab.pionPort)], {
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    tab.relay = proc;
    this.attachProcessOutput(proc, tabId);
    proc.on('close', (code) => {
      this.sendLog(tabId, `Relay exited with code ${code}`);
    });
  }

  async startHeadless(tabId: string, platform: Platform): Promise<void> {
    const tab = await this.getOrCreateTab(tabId);
    tab.tunnelMode = TunnelMode.HeadlessBale;
    tab.platform = platform;
    let cookieStr = await this.getBaleCookieString();
    if (!cookieStr) {
      this.sendLog(tabId, 'No Bale cookies found, opening login.');
      if (this._mainWindow && !this._mainWindow.isDestroyed()) {
        this._mainWindow.webContents.send(IPC.BALE_LOGIN_REQUIRED, { tabId, url: BALE_URL });
      }
      cookieStr = await this.waitForBaleLogin();
      if (this._mainWindow && !this._mainWindow.isDestroyed()) {
        this._mainWindow.webContents.send(IPC.BALE_LOGIN_DONE, { tabId });
      }
      this.sendLog(tabId, 'Bale login captured.');
    }
    this.killRelay(tabId, tab);
    const proc = spawn(this.headlessBalePath, ['--resources', 'default'], {
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    tab.relay = proc;
    let sawAuthFailure = false;
    this.attachProcessOutput(proc, tabId, (msg) => {
      if (msg.includes('status 401') || msg.includes('status 403') || msg.includes('Unauthorized')) {
        sawAuthFailure = true;
      }
    });
    proc.stdin?.write(cookieStr + '\n');
    proc.on('close', async (code) => {
      this.sendLog(tabId, `Headless exited with code ${code}`);
      if (sawAuthFailure) {
        this.sendLog(tabId, 'Bale session rejected, clearing and re-prompting login.');
        await this.clearBaleAuthCookies();
        if (this.tabs.get(tabId) === tab) {
          this.startHeadless(tabId, platform);
        }
      }
    });
  }

  private async clearBaleAuthCookies(): Promise<void> {
    const ses = session.fromPartition(SESSION_PARTITION);
    const matches = await ses.cookies.get({ name: 'access_token' });
    for (const cookie of matches) {
      if (!cookie.domain || !BALE_COOKIE_DOMAINS.some((d) => cookie.domain!.includes(d))) continue;
      const host = cookie.domain.startsWith('.') ? cookie.domain.slice(1) : cookie.domain;
      const url = `https://${host}${cookie.path || '/'}`;
      try {
        await ses.cookies.remove(url, cookie.name);
      } catch (err) {
        console.log(`[COOKIES] failed to remove ${cookie.name} on ${url}:`, err);
      }
    }
  }

  private waitForBaleLogin(): Promise<string> {
    return new Promise((resolve) => {
      const ses = session.fromPartition(SESSION_PARTITION);
      const check = async () => {
        const cookieStr = await this.getBaleCookieString();
        if (cookieStr.includes('access_token=')) {
          ses.cookies.removeListener('changed', onChanged);
          resolve(cookieStr);
        }
      };
      const onChanged = (
        _e: Electron.Event,
        cookie: Electron.Cookie,
        _cause: string,
        removed: boolean,
      ) => {
        if (removed) return;
        if (cookie.name !== 'access_token') return;
        if (!cookie.domain || !BALE_COOKIE_DOMAINS.some((d) => cookie.domain!.includes(d))) return;
        check();
      };
      ses.cookies.on('changed', onChanged);
      check();
    });
  }

  killRelay(tabId: string, tab: TabState): void {
    if (tab.relay) {
      console.log(`[${tabId}] killing process pid=${tab.relay.pid}`);
      tab.relay.kill();
      tab.relay = null;
    }
  }

  killAllRelays(): void {
    this.tabs.forEach((tab, tabId) => this.killRelay(tabId, tab));
  }

  async loadHook(tabId: string, _url: string, tab: TabState): Promise<string> {
    tab.platform = Platform.Bale;
    const hook = await fs.readFile(path.join(this.hooksDir, 'video-bale.js'), 'utf8');
    return LOG_CAPTURE_SNIPPET + `window.PION_PORT=${tab.pionPort};window.IS_CREATOR=true;` + hook;
  }

  async setTunnelMode(tabId: string, mode: TunnelMode, platform?: Platform): Promise<void> {
    const tab = await this.getOrCreateTab(tabId);
    tab.tunnelMode = mode;
    if (platform) tab.platform = platform;
    if (mode === TunnelMode.HeadlessBale) return;
    this.killRelay(tabId, tab);
    setTimeout(() => this.startRelay(tabId, tab), RELAY_RESTART_DELAY_MS);
  }

  async getBaleCookieString(): Promise<string> {
    const ses = session.fromPartition(SESSION_PARTITION);
    const all = await ses.cookies.get({});
    const baleCookies = all.filter((cookie) => {
      return cookie.domain != null && BALE_COOKIE_DOMAINS.some((d) => cookie.domain!.includes(d));
    });
    return baleCookies.map((cookie) => `${cookie.name}=${cookie.value}`).join('; ');
  }
}
