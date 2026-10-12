// UDP feedback helpers shared by the device and group cards. They live apart
// from UdpInfoSection so that file only exports a component (Fast Refresh).
import type { Capabilities } from '../types';

/** Converts a HUE device name to a Loxone-safe identifier (mirrors Go sanitizeName in export.go) */
export function sanitizeName(name: string): string {
  let s = name.toLowerCase();
  s = s.replace(/[\s-]/g, '_');
  s = s.replace(/ä/g, 'ae').replace(/ö/g, 'oe').replace(/ü/g, 'ue').replace(/ß/g, 'ss');
  return s.replace(/[^a-z0-9_]/g, '');
}

export interface UdpProperty {
  pattern: string;
  label: string;
  range: string;
  property: string;
}

export function getUdpProperties(loxoneId: string, capabilities: Capabilities): UdpProperty[] {
  const props: UdpProperty[] = [
    { pattern: `${loxoneId}/on:\\v`, label: 'Ein/Aus', range: '0 | 1', property: 'on' },
  ];
  if (capabilities.supports_dimming) {
    props.push({ pattern: `${loxoneId}/bri:\\v`, label: 'Helligkeit', range: '0–100', property: 'bri' });
  }
  if (capabilities.supports_color_temp) {
    props.push({ pattern: `${loxoneId}/ct:\\v`, label: 'Farbtemperatur (Mirek)', range: '153–500', property: 'ct' });
  }
  if (capabilities.supports_color) {
    props.push({ pattern: `${loxoneId}/color_x:\\v`, label: 'Farbe X', range: '0–1', property: 'color_x' });
    props.push({ pattern: `${loxoneId}/color_y:\\v`, label: 'Farbe Y', range: '0–1', property: 'color_y' });
  }
  return props;
}

export function getGroupUdpProperties(loxoneId: string): UdpProperty[] {
  return [
    { pattern: `${loxoneId}/on:\\v`, label: 'Ein/Aus', range: '0 | 1', property: 'on' },
    { pattern: `${loxoneId}/bri:\\v`, label: 'Helligkeit', range: '0–100', property: 'bri' },
    { pattern: `${loxoneId}/ct:\\v`, label: 'Farbtemperatur (Mirek)', range: '153–500', property: 'ct' },
    { pattern: `${loxoneId}/color_x:\\v`, label: 'Farbe X', range: '0–1', property: 'color_x' },
    { pattern: `${loxoneId}/color_y:\\v`, label: 'Farbe Y', range: '0–1', property: 'color_y' },
  ];
}
