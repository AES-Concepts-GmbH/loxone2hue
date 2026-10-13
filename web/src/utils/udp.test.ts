import { describe, expect, it } from 'vitest';
import { getGroupUdpProperties, getUdpProperties, sanitizeName } from './udp';

describe('sanitizeName', () => {
  // Mirrors sanitizeName in internal/api/export.go; the Go test pins the same cases.
  it('matches the Go implementation', () => {
    expect(sanitizeName('WZ Decke')).toBe('wz_decke');
    expect(sanitizeName('Küche Süd-Ost')).toBe('kueche_sued_ost');
    expect(sanitizeName('Büro (Stefan) 2.0')).toBe('buero_stefan_20');
    expect(sanitizeName('Straße')).toBe('strasse');
    expect(sanitizeName('ÄÖÜ')).toBe('aeoeue');
  });
});

describe('UDP property lists', () => {
  it('lists only the properties a light supports', () => {
    const onOff = getUdpProperties('plug', { supports_dimming: false, supports_color_temp: false, supports_color: false });
    expect(onOff.map((p) => p.property)).toEqual(['on']);
    expect(onOff[0].pattern).toBe('plug/on:\\v');

    const full = getUdpProperties('wz', { supports_dimming: true, supports_color_temp: true, supports_color: true });
    expect(full.map((p) => p.property)).toEqual(['on', 'bri', 'ct', 'color_x', 'color_y']);
  });

  it('always lists every property for a group', () => {
    expect(getGroupUdpProperties('wz').map((p) => p.property)).toEqual(['on', 'bri', 'ct', 'color_x', 'color_y']);
  });
});
