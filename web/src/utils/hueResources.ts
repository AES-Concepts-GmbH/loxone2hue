import type { Group, Light, Scene, Sensor } from '../types';

/** Everything the bridge knows, as the mapping UI needs it for lookups. */
export interface HueResources {
  lights: Light[];
  groups: Group[];
  scenes: Scene[];
  sensors: Sensor[];
}

export type HueType = 'light' | 'group' | 'scene' | 'sensor';

export const HUE_TYPES: readonly HueType[] = ['light', 'group', 'scene', 'sensor'];

export function getTypeLabel(hueType: string): string {
  switch (hueType) {
    case 'group':
      return 'Gruppe';
    case 'scene':
      return 'Szene';
    case 'sensor':
      return 'Sensor';
    default:
      return 'Licht';
  }
}

export function getGroupNameForScene(groups: Group[], scene: Scene): string {
  return groups.find((g) => g.id === scene.group_id)?.name || '';
}

/** "Wohnzimmer - Entspannen" so scenes of different rooms stay distinguishable. */
export function getSceneDisplayName(groups: Group[], scene: Scene): string {
  const groupName = getGroupNameForScene(groups, scene);
  return groupName ? `${groupName} - ${scene.name}` : scene.name;
}

export function getHueResourceName(res: HueResources, hueId: string, hueType: string): string {
  switch (hueType) {
    case 'group':
      return res.groups.find((g) => g.id === hueId)?.name || hueId;
    case 'scene': {
      const scene = res.scenes.find((s) => s.id === hueId);
      return scene ? getSceneDisplayName(res.groups, scene) : hueId;
    }
    case 'sensor':
      return res.sensors.find((s) => s.id === hueId)?.name || hueId;
    default:
      return res.lights.find((l) => l.id === hueId)?.name || hueId;
  }
}

/** Scenes sorted by group name, then scene name, with the group name attached. */
export function getScenesWithGroupName(res: HueResources): (Scene & { groupName: string })[] {
  return res.scenes
    .map((s) => ({ ...s, groupName: getGroupNameForScene(res.groups, s) }))
    .sort((a, b) => {
      const groupCompare = a.groupName.localeCompare(b.groupName, 'de');
      if (groupCompare !== 0) return groupCompare;
      return a.name.localeCompare(b.name, 'de');
    });
}
