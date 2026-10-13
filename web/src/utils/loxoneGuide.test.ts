import { describe, expect, it } from 'vitest';
import type { Mapping } from '../types';
import { getLoxoneGuide, getSensorFeedbackInfo, getTestUrls } from './loxoneGuide';

function mapping(loxoneId: string, hueType: string, extra: Partial<Mapping> = {}): Mapping {
  return { id: loxoneId, name: loxoneId, loxone_id: loxoneId, hue_id: 'h', hue_type: hueType, enabled: true, ...extra };
}

describe('getSensorFeedbackInfo', () => {
  it('describes light level as raw value plus lux and treats room aggregates like devices', () => {
    expect(getSensorFeedbackInfo('grouped_light_level').map((f) => f.property)).toEqual(['light_level', 'lux']);
    expect(getSensorFeedbackInfo('grouped_motion')[0].property).toBe('motion');
    expect(getSensorFeedbackInfo('device_power')[0].property).toBe('battery');
    expect(getSensorFeedbackInfo('exotic')[0]).toEqual({ property: 'exotic', range: 'variabel', example: 'exotic:<wert>' });
  });
});

describe('getLoxoneGuide', () => {
  it('lists UDP and HTTP feedback for a sensor according to the enabled protocols and the mapping', () => {
    const both = getLoxoneGuide(mapping('wz_licht', 'sensor'), { sensorType: 'light_level', hasUDP: true, hasHTTP: true });
    expect(both.commands.map((c) => c.value)).toEqual([
      'wz_licht/light_level:20001',
      '/dev/sps/io/wz_licht_light_level/20001',
      'wz_licht/lux:100',
      '/dev/sps/io/wz_licht_lux/100',
    ]);
    expect(both.note).toContain('UDP & HTTP');

    const udpOff = getLoxoneGuide(mapping('wz_licht', 'sensor', { feedback_udp: false }), { sensorType: 'motion', hasUDP: true, hasHTTP: true });
    expect(udpOff.commands.map((c) => c.value)).toEqual(['/dev/sps/io/wz_licht_motion/1']);
  });

  it('gives groups the MOOD command and lights the colour commands', () => {
    const group = getLoxoneGuide(mapping('wz', 'group'), { hasUDP: false, hasHTTP: false });
    expect(group.commands.map((c) => c.value)).toContain('/ws?cmd=MOOD wz <v>');
    expect(group.note).toContain('wz_mood_1');

    const light = getLoxoneGuide(mapping('wz_decke', 'light'), { hasUDP: false, hasHTTP: false });
    expect(light.commands.map((c) => c.value)).toContain('/ws?cmd=SET wz_decke COLOR #FF5500');
  });

  it('tells mood scenes apart from plain scenes', () => {
    expect(getLoxoneGuide(mapping('wz_mood_1', 'scene'), { hasUDP: false, hasHTTP: false }).title).toContain('Mood-Szene');
    expect(getLoxoneGuide(mapping('abend', 'scene'), { hasUDP: false, hasHTTP: false }).commands[0].value).toBe('/ws?cmd=SCENE abend');
  });
});

describe('getTestUrls', () => {
  it('has no test URLs for sensors and URL-encodes the command', () => {
    expect(getTestUrls(mapping('s', 'sensor'))).toEqual([]);
    expect(getTestUrls(mapping('abend', 'scene'))).toEqual([{ label: 'Szene aktivieren', url: './ws?cmd=SCENE%20abend' }]);
    expect(getTestUrls(mapping('wz_mood_2', 'scene'))[0].url).toBe('./ws?cmd=SCENE%20wz_mood_2');
  });

  it('offers MOOD tests for groups and colour temperature for lights', () => {
    expect(getTestUrls(mapping('wz', 'group')).map((t) => t.label)).toEqual([
      'Einschalten', 'Ausschalten', 'Helligkeit 50%', 'MOOD 0 (Aus)', 'MOOD 1', 'MOOD 2',
    ]);
    const lightUrls = getTestUrls(mapping('wz_decke', 'light'));
    expect(lightUrls[lightUrls.length - 1]).toEqual({ label: 'Farbtemperatur 4000K', url: './ws?cmd=SET%20wz_decke%20CT%204000' });
  });
});
