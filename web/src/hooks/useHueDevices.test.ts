import { act, create } from 'react-test-renderer';
import { createElement } from 'react';
import { beforeEach, expect, it, vi } from 'vitest';
import { useHueDevices } from './useHueDevices';
import * as api from '../services/api';
const connection = vi.hoisted(() => ({ connected: false }));
vi.mock('./useWebSocket', () => ({ useWebSocket: () => ({ isConnected: connection.connected }) }));
vi.mock('../services/api', () => ({ getDevices: vi.fn(), getGroups: vi.fn(), getScenes: vi.fn(), getSensors: vi.fn() }));
beforeEach(() => {
 connection.connected = false;
 vi.mocked(api.getDevices).mockResolvedValue({ devices: [] });
 vi.mocked(api.getGroups).mockResolvedValue({ groups: [] });
 vi.mocked(api.getScenes).mockResolvedValue({ scenes: [] });
 vi.mocked(api.getSensors).mockResolvedValue({ sensors: [] });
 vi.clearAllMocks();
});
it('resynchronizes resources after a disconnected socket reconnects', async () => {
 function Probe() { useHueDevices(); return null; }
 let root: ReturnType<typeof create>;
 await act(async () => { root = create(createElement(Probe)); });
 expect(api.getDevices).toHaveBeenCalledTimes(1);
 connection.connected = true;
 await act(async () => { root.update(createElement(Probe)); });
 expect(api.getDevices).toHaveBeenCalledTimes(2);
 connection.connected = false;
 await act(async () => { root.update(createElement(Probe)); });
 connection.connected = true;
 await act(async () => { root.update(createElement(Probe)); });
 expect(api.getDevices).toHaveBeenCalledTimes(3);
 act(() => root.unmount());
});
