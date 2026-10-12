import type { Sensor } from '../types';

/**
 * Groups HUE sensor resource types by the payload they carry. The bridge
 * reports motion and light level per device and aggregated per room or home
 * ("grouped_*"); the UI treats both alike and only labels them differently.
 */
export type SensorKind =
  | 'motion'
  | 'temperature'
  | 'light_level'
  | 'button'
  | 'contact'
  | 'relative_rotary'
  | 'device_power';

const kinds: Record<string, SensorKind> = {
  motion: 'motion',
  grouped_motion: 'motion',
  temperature: 'temperature',
  light_level: 'light_level',
  grouped_light_level: 'light_level',
  button: 'button',
  contact: 'contact',
  relative_rotary: 'relative_rotary',
  device_power: 'device_power',
};

export function sensorKind(type: string): SensorKind | undefined {
  return kinds[type];
}

export const sensorTypeLabels: Record<string, string> = {
  motion: 'Bewegung',
  grouped_motion: 'Raum-Bewegung',
  temperature: 'Temperatur',
  light_level: 'Helligkeit',
  grouped_light_level: 'Raum-Helligkeit',
  button: 'Taster',
  contact: 'Kontakt',
  relative_rotary: 'Drehregler',
  device_power: 'Batterie',
};

export const sensorScopeLabels: Record<string, string> = {
  room: 'Raum',
  home: 'Zuhause',
};

/** Lux for display: prefers the backend's value, falls back to the formula. */
export function sensorLux(sensor: Sensor): number | null {
  if (sensor.state.lux != null) return sensor.state.lux;
  if (sensor.state.light_level == null) return null;
  return Math.round(Math.pow(10, (sensor.state.light_level - 1) / 10000));
}
