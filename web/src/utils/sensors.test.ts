import { describe, expect, it } from 'vitest';
import type { Sensor } from '../types';
import { sensorKind, sensorLux, sensorScopeLabels, sensorTypeLabels } from './sensors';

function sensor(state: Sensor['state']): Sensor {
  return { id: 's', name: 'S', type: 'light_level', state };
}

describe('sensorKind', () => {
  it('maps the room and home aggregates onto the device kinds', () => {
    expect(sensorKind('grouped_motion')).toBe('motion');
    expect(sensorKind('grouped_light_level')).toBe('light_level');
    expect(sensorKind('motion')).toBe('motion');
    expect(sensorKind('device_power')).toBe('device_power');
    expect(sensorKind('unknown_type')).toBeUndefined();
  });

  it('has a label for every known type and scope', () => {
    for (const type of ['motion', 'grouped_motion', 'temperature', 'light_level', 'grouped_light_level', 'button', 'contact', 'relative_rotary', 'device_power']) {
      expect(sensorTypeLabels[type]).toBeTruthy();
    }
    expect(sensorScopeLabels.room).toBe('Raum');
    expect(sensorScopeLabels.home).toBe('Zuhause');
  });
});

describe('sensorLux', () => {
  it('prefers the lux value from the backend', () => {
    expect(sensorLux(sensor({ enabled: true, light_level: 20001, lux: 123 }))).toBe(123);
  });

  it('falls back to the Hue formula 10^((level-1)/10000)', () => {
    expect(sensorLux(sensor({ enabled: true, light_level: 20001 }))).toBe(100);
    expect(sensorLux(sensor({ enabled: true, light_level: 1 }))).toBe(1);
    expect(sensorLux(sensor({ enabled: true, light_level: 30001 }))).toBe(1000);
  });

  it('returns null without a reading', () => {
    expect(sensorLux(sensor({ enabled: true }))).toBeNull();
  });
});
