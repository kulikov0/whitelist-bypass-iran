export const IPC = {
  START: 'joiner:start',
  STOP: 'joiner:stop',
  LOG: 'joiner:log',
  STATUS: 'joiner:status',
  RUNNING: 'joiner:running',
} as const;

export interface JoinerSettings {
  link: string;
  displayName: string;
  socksPort: number;
  socksUser: string;
  socksPass: string;
  tunnelMode: 'vp8' | 'dc';
  vp8Fps: number;
  vp8Batch: number;
  resources: 'moderate' | 'default' | 'unlimited';
  dns: string;
  noTun: boolean;
}
