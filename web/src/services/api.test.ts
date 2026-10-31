import { afterEach, expect, it, vi } from 'vitest';
import { AUTH_REQUIRED_EVENT, getConfig, login, updateConfig } from './api';
afterEach(() => vi.unstubAllGlobals());
it('requests a login when a protected API call loses its session', async () => {
 const dispatchEvent = vi.fn();
 vi.stubGlobal('window', { dispatchEvent });
 vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({ error: 'expired' }) }));
 await expect(getConfig()).rejects.toThrow('expired');
 expect(dispatchEvent.mock.calls[0][0].type).toBe(AUTH_REQUIRED_EVENT);
});
it('keeps failed login feedback in the login form', async () => {
 const dispatchEvent = vi.fn(); vi.stubGlobal('window', { dispatchEvent });
 vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({ error: 'wrong password' }) }));
 await expect(login('admin', 'wrong')).rejects.toThrow('wrong password');
 expect(dispatchEvent).not.toHaveBeenCalled();
});
it('propagates settings persistence failures to the caller', async () => {
 vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'settings not saved' }) }));
 await expect(updateConfig({})).rejects.toThrow('settings not saved');
});
