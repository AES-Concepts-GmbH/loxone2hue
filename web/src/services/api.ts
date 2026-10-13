import { Light, Group, Scene, Sensor, Mapping, BridgeInfo, DeviceCommand } from '../types';
export type { Mapping };

const API_BASE = './api';

/**
 * Fired on window when the API answers 401, so the app can show the login
 * form instead of every component handling it.
 */
export const AUTH_REQUIRED_EVENT = 'loxone2hue:auth-required';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      // Tells the server this is the UI: a 401 then comes without a Basic
      // Auth challenge, so the browser does not open its own dialog.
      'X-Requested-With': 'loxone2hue-ui',
      ...options?.headers,
    },
  });

  if (!response.ok) {
    if (response.status === 401 && !url.endsWith('/auth/login')) {
      window.dispatchEvent(new Event(AUTH_REQUIRED_EVENT));
    }
    const error = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(error.error || 'API request failed');
  }

  return response.json();
}

// Login
export interface AuthStatus {
  enabled: boolean;
  username: string;
  authenticated: boolean;
}

export async function getAuthStatus(): Promise<AuthStatus> {
  return fetchJSON(`${API_BASE}/auth/status`);
}

export async function login(username: string, password: string): Promise<{ status: string; enabled: boolean }> {
  return fetchJSON(`${API_BASE}/auth/login`, {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

export async function logout(): Promise<void> {
  await fetchJSON(`${API_BASE}/auth/logout`, { method: 'POST' });
}

export interface AuthUpdate {
  username?: string;
  current_password?: string;
  new_password?: string;
  disable?: boolean;
}

/** Sets, changes or disables the login; the server issues a fresh session on success. */
export async function updateAuth(update: AuthUpdate): Promise<{ status: string; enabled: boolean; username?: string }> {
  return fetchJSON(`${API_BASE}/auth`, {
    method: 'PUT',
    body: JSON.stringify(update),
  });
}

// Bridge endpoints
export async function getBridge(): Promise<{ configured: boolean; info?: unknown }> {
  return fetchJSON(`${API_BASE}/bridge`);
}

export async function discoverBridges(): Promise<{ bridges: BridgeInfo[] }> {
  return fetchJSON(`${API_BASE}/bridge/discover`);
}

export async function pairBridge(bridgeIP: string): Promise<{ success: boolean; application_key: string }> {
  return fetchJSON(`${API_BASE}/bridge/pair`, {
    method: 'POST',
    body: JSON.stringify({ bridge_ip: bridgeIP }),
  });
}

// Device endpoints
export async function getDevices(): Promise<{ devices: Light[] }> {
  return fetchJSON(`${API_BASE}/devices`);
}

export async function getDevice(id: string): Promise<Light> {
  return fetchJSON(`${API_BASE}/devices/${id}`);
}

export async function setDevice(id: string, command: DeviceCommand): Promise<void> {
  await fetchJSON(`${API_BASE}/devices/${id}`, {
    method: 'PUT',
    body: JSON.stringify(command),
  });
}

// Group endpoints
export async function getGroups(): Promise<{ groups: Group[] }> {
  return fetchJSON(`${API_BASE}/groups`);
}

export async function setGroup(id: string, command: DeviceCommand): Promise<void> {
  await fetchJSON(`${API_BASE}/groups/${id}`, {
    method: 'PUT',
    body: JSON.stringify(command),
  });
}

// Scene endpoints
export async function getScenes(): Promise<{ scenes: Scene[] }> {
  return fetchJSON(`${API_BASE}/scenes`);
}

export async function activateScene(id: string): Promise<void> {
  await fetchJSON(`${API_BASE}/scenes/${id}/activate`, {
    method: 'POST',
  });
}

// Sensor endpoints
export async function getSensors(): Promise<{ sensors: Sensor[] }> {
  return fetchJSON(`${API_BASE}/sensors`);
}

// Mapping endpoints
export async function getMappings(): Promise<{ mappings: Mapping[] }> {
  return fetchJSON(`${API_BASE}/mappings`);
}

export async function createMapping(mapping: Omit<Mapping, 'id'>): Promise<Mapping> {
  return fetchJSON(`${API_BASE}/mappings`, {
    method: 'POST',
    body: JSON.stringify(mapping),
  });
}

// PUT replaces the whole mapping, so the caller has to send every field.
export async function updateMapping(id: string, mapping: Mapping): Promise<Mapping> {
  return fetchJSON(`${API_BASE}/mappings/${id}`, {
    method: 'PUT',
    body: JSON.stringify(mapping),
  });
}

export async function deleteMapping(id: string): Promise<void> {
  await fetchJSON(`${API_BASE}/mappings/${id}`, {
    method: 'DELETE',
  });
}

// Backup types
export interface MappingsBackup {
  version: string;
  created_at: string;
  mappings: Mapping[];
}

export interface ImportResult {
  status: string;
  imported: number;
  updated: number;
  skipped: number;
  total: number;
}

// Export mappings - triggers file download
export async function exportMappings(): Promise<MappingsBackup> {
  return fetchJSON(`${API_BASE}/mappings/export`);
}

// Import mappings
export async function importMappings(
  backup: MappingsBackup,
  mode: 'replace' | 'merge'
): Promise<ImportResult> {
  return fetchJSON(`${API_BASE}/mappings/import`, {
    method: 'POST',
    body: JSON.stringify({ mode, backup }),
  });
}

// Config endpoint
export interface MiniserverConfig {
  id: string;
  name: string;
  ip: string;
  port: number;
  udp_enabled: boolean;
  http_enabled: boolean;
  http_url: string;
  http_user: string;
  http_password: string;
  /** Verify the Miniserver certificate when http_url is https (default off: self-signed). */
  http_verify_tls: boolean;
  send_all: boolean;
}

export interface LoxoneConfig {
  miniservers: MiniserverConfig[];
}

export interface ConfigResponse {
  server: {
    host: string;
    port: number;
  };
  auth: {
    enabled: boolean;
    username: string;
  };
  hue: {
    bridge_ip: string;
    configured: boolean;
  };
  loxone: LoxoneConfig;
  logging: {
    level: string;
  };
}

export async function getConfig(): Promise<ConfigResponse> {
  return fetchJSON(`${API_BASE}/config`);
}

export async function updateConfig(update: { loxone?: LoxoneConfig }): Promise<{ status: string }> {
  return fetchJSON(`${API_BASE}/config`, {
    method: 'PUT',
    body: JSON.stringify(update),
  });
}

// Logs
export interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  source: string;
  fields?: Record<string, unknown>;
}

export async function getLogs(params?: {
  level?: string;
  search?: string;
  limit?: number;
}): Promise<{ entries: LogEntry[] }> {
  const query = new URLSearchParams();
  if (params?.level) query.set('level', params.level);
  if (params?.search) query.set('search', params.search);
  if (params?.limit) query.set('limit', String(params.limit));
  const qs = query.toString();
  return fetchJSON(`${API_BASE}/logs${qs ? `?${qs}` : ''}`);
}

// UDP test
export interface TestUDPResult {
  status: string;
  message: string;
}

export async function testUDP(loxoneId: string, property: string, value: string, miniserverId: string): Promise<TestUDPResult> {
  return fetchJSON(`${API_BASE}/udp/test`, {
    method: 'POST',
    body: JSON.stringify({ loxone_id: loxoneId, property, value, miniserver_id: miniserverId }),
  });
}

// Backup types
export interface ConfigBackup {
  id: string;
  version: string;
  created_at: string;
  remark: string;
  config: {
    server: { host: string; port: number };
    hue: { bridge_ip: string; application_key: string };
    loxone: LoxoneConfig;
    logging: { level: string };
    mappings: Mapping[];
  };
}

// What the API returns for a backup in listings and after create/upload.
// The full file (with HUE key and Miniserver passwords) is only available
// through the download endpoint.
export interface BackupSummary {
  id: string;
  version: string;
  created_at: string;
  remark: string;
  mapping_count: number;
  miniserver_count: number;
}

// Backup endpoints
export async function listBackups(): Promise<{ backups: BackupSummary[] }> {
  return fetchJSON(`${API_BASE}/backups`);
}

export async function createBackup(remark: string): Promise<BackupSummary> {
  return fetchJSON(`${API_BASE}/backups`, {
    method: 'POST',
    body: JSON.stringify({ remark }),
  });
}

export async function deleteBackup(id: string): Promise<void> {
  await fetchJSON(`${API_BASE}/backups/${id}`, { method: 'DELETE' });
}

export async function uploadBackup(backup: ConfigBackup): Promise<BackupSummary> {
  return fetchJSON(`${API_BASE}/backups/upload`, {
    method: 'POST',
    body: JSON.stringify(backup),
  });
}

export async function restoreBackup(id: string): Promise<{ status: string; backup: string; remark: string }> {
  return fetchJSON(`${API_BASE}/backups/${id}/restore`, { method: 'POST' });
}

export function getBackupDownloadUrl(id: string): string {
  return `${API_BASE}/backups/${id}/download`;
}

// Health check
export async function getHealth(): Promise<{ status: string; hue_configured: boolean }> {
  return fetchJSON(`${API_BASE}/health`);
}

// Bridge connection test
export interface BridgeTestResult {
  bridge_ip: string;
  dns_lookup: {
    success: boolean;
    addresses?: string[];
    error?: string;
  };
  tcp_443: {
    success: boolean;
    error?: string;
  };
  tcp_80: {
    success: boolean;
    error?: string;
  };
  https_request: {
    success: boolean;
    status_code?: number;
    error?: string;
  };
}

export async function testBridgeConnection(bridgeIP: string): Promise<BridgeTestResult> {
  return fetchJSON(`${API_BASE}/bridge/test`, {
    method: 'POST',
    body: JSON.stringify({ bridge_ip: bridgeIP }),
  });
}
