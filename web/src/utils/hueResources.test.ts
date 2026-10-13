import { describe, expect, it } from 'vitest';
import type { Group, Light, Scene, Sensor } from '../types';
import { getHueResourceName, getSceneDisplayName, getScenesWithGroupName, getTypeLabel, HUE_TYPES } from './hueResources';

const groups: Group[] = [
  { id: 'room-1', name: 'Wohnzimmer', type: 'room', lights: [], state: { all_on: false, any_on: false } },
  { id: 'room-2', name: 'Küche', type: 'room', lights: [], state: { all_on: false, any_on: false } },
];
const scenes: Scene[] = [
  { id: 'scene-1', name: 'Lesen', group_id: 'room-1', type: 'room' },
  { id: 'scene-2', name: 'Kochen', group_id: 'room-2', type: 'room' },
  { id: 'scene-3', name: 'Abend', group_id: 'room-1', type: 'room' },
  { id: 'scene-4', name: 'Verwaist', group_id: 'gone', type: 'room' },
];
const lights: Light[] = [
  { id: 'light-1', name: 'Decke', type: 'ceiling', model_id: 'm', product_name: 'p', state: { on: true, brightness: 50, reachable: true }, capabilities: { supports_color: false, supports_color_temp: false, supports_dimming: true } },
];
const sensors: Sensor[] = [{ id: 'sensor-1', name: 'Flur Motion', type: 'motion', state: { enabled: true } }];
const res = { lights, groups, scenes, sensors };

describe('hue resource lookups', () => {
  it('labels every type', () => {
    expect(HUE_TYPES.map(getTypeLabel)).toEqual(['Licht', 'Gruppe', 'Szene', 'Sensor']);
    expect(getTypeLabel('whatever')).toBe('Licht');
  });

  it('resolves names per type and falls back to the id', () => {
    expect(getHueResourceName(res, 'light-1', 'light')).toBe('Decke');
    expect(getHueResourceName(res, 'room-2', 'group')).toBe('Küche');
    expect(getHueResourceName(res, 'scene-1', 'scene')).toBe('Wohnzimmer - Lesen');
    expect(getHueResourceName(res, 'scene-4', 'scene')).toBe('Verwaist');
    expect(getHueResourceName(res, 'sensor-1', 'sensor')).toBe('Flur Motion');
    expect(getHueResourceName(res, 'missing', 'group')).toBe('missing');
  });

  it('prefixes scenes with their room and sorts by room, then scene', () => {
    expect(getSceneDisplayName(groups, scenes[0])).toBe('Wohnzimmer - Lesen');
    expect(getScenesWithGroupName(res).map((s) => `${s.groupName}|${s.name}`)).toEqual([
      '|Verwaist',
      'Küche|Kochen',
      'Wohnzimmer|Abend',
      'Wohnzimmer|Lesen',
    ]);
  });
});
