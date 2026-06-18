interface Bridge {
  start(settings: any): Promise<{ ok: boolean; error?: string }>;
  stop(): Promise<{ ok: boolean }>;
  onLog(cb: (text: string) => void): void;
  onStatus(cb: (status: string) => void): void;
  onRunning(cb: (running: boolean) => void): void;
}
declare const bridge: Bridge;

const $ = (id: string) => document.getElementById(id) as HTMLElement;
const input = (id: string) => document.getElementById(id) as HTMLInputElement;
const select = (id: string) => document.getElementById(id) as HTMLSelectElement;

const logEl = $('log') as HTMLPreElement;
const statusEl = $('status');
const startBtn = $('start') as HTMLButtonElement;
const stopBtn = $('stop') as HTMLButtonElement;
const downloadLogsBtn = $('downloadLogs') as HTMLImageElement;
const linkInput = input('link');

stopBtn.disabled = true;

// Load settings from localStorage
try {
  const savedSettings = localStorage.getItem('joiner_settings');
  if (savedSettings) {
    const s = JSON.parse(savedSettings);
    if (s.link) linkInput.value = s.link;
    if (s.displayName) input('name').value = s.displayName;
    if (s.socksPort) input('socksPort').value = String(s.socksPort);
    if (s.socksUser) input('socksUser').value = s.socksUser;
    if (s.socksPass) input('socksPass').value = s.socksPass;
    if (s.tunnelMode) select('tunnelMode').value = s.tunnelMode;
    if (s.vp8Fps) input('vp8Fps').value = String(s.vp8Fps);
    if (s.vp8Batch) input('vp8Batch').value = String(s.vp8Batch);
    if (s.resources) select('resources').value = s.resources;
    if (s.dns) input('dns').value = s.dns;
    if (s.noTun !== undefined) input('noTun').checked = s.noTun;
  }
} catch (e) {
  console.error('Failed to load settings from localStorage', e);
}

downloadLogsBtn.addEventListener('click', () => {
  const blob = new Blob([logEl.textContent || ''], { type: 'text/plain' });
  const anchor = document.createElement('a');
  anchor.href = URL.createObjectURL(blob);
  anchor.download = 'joiner-logs-' + new Date().toISOString().replace(/[:.]/g, '-') + '.txt';
  anchor.click();
  URL.revokeObjectURL(anchor.href);
});

function appendLog(text: string) {
  logEl.textContent += text;
  logEl.scrollTop = logEl.scrollHeight;
}

bridge.onLog((text) => appendLog(text));
bridge.onStatus((s) => {
  statusEl.textContent = s;
  statusEl.dataset.state = s;
});
bridge.onRunning((running) => {
  startBtn.disabled = running;
  stopBtn.disabled = !running;
});

startBtn.addEventListener('click', async () => {
  appendLog('\n[ui] starting joiner...\n');
  const link = linkInput.value.trim();
  if (!link) {
    appendLog('[ui] link is required\n');
    return;
  }
  const settings = {
    link,
    displayName: input('name').value.trim() || 'Joiner',
    socksPort: parseInt(input('socksPort').value, 10) || 1080,
    socksUser: input('socksUser').value,
    socksPass: input('socksPass').value,
    tunnelMode: select('tunnelMode').value,
    vp8Fps: parseInt(input('vp8Fps').value, 10) || 24,
    vp8Batch: parseInt(input('vp8Batch').value, 10) || 30,
    resources: select('resources').value,
    dns: input('dns').value.trim() || '1.1.1.1,8.8.8.8',
    noTun: input('noTun').checked,
  };

  // Save settings to localStorage
  try {
    localStorage.setItem('joiner_settings', JSON.stringify(settings));
  } catch (e) {
    console.error('Failed to save settings to localStorage', e);
  }

  const r = await bridge.start(settings);
  if (!r.ok) appendLog(`[ui] start failed: ${r.error}\n`);
});

stopBtn.addEventListener('click', async () => {
  appendLog('\n[ui] stopping joiner...\n');
  await bridge.stop();
});
