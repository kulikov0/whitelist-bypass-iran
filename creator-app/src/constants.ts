export const INITIAL_PORT_BASE = 10000;

export const SCAN_INTERVAL_MS = 2000;
export const KICK_DELAY_MS = 500;

export const BALE_URL = 'https://web.bale.ai/';

export const SESSION_PARTITION = 'persist:creator';
export const WINDOW_WIDTH = 1200;
export const WINDOW_HEIGHT = 800;

export const USER_AGENT =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) ' +
  'AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';

export const BALE_COOKIE_DOMAINS = ['bale.ai'];

export enum Selector {
  BALE_LEAVE_CALL = '[data-testid="leave-call-button"]',
}

export enum IPC {
  START_HEADLESS = 'start-headless',
  CLOSE_TAB = 'close-tab',
  GET_COOKIES = 'get-cookies',
  RELAY_LOG = 'relay-log',
  BALE_LOGIN_REQUIRED = 'bale-login-required',
  BALE_LOGIN_DONE = 'bale-login-done',
}

export enum HeadlessLogMarker {
  CALL_CREATED = 'CALL CREATED',
  JOIN_LINK = 'join_link:',
  PROTOCOL = 'protocol:',
  TUNNEL_CONNECTED = 'TUNNEL CONNECTED',
}
