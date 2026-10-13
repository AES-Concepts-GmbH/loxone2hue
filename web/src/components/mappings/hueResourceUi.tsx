import type { ReactNode } from 'react';
import { Activity, Home, Lightbulb, Play, PowerOff, Sun, SunDim, WifiOff } from 'lucide-react';
import type { HueResources } from '../../utils/hueResources';

export function getTypeIcon(hueType: string, size = 20): ReactNode {
  const className = 'text-hue-orange';
  switch (hueType) {
    case 'group':
      return <Home size={size} className={className} />;
    case 'scene':
      return <Play size={size} className={className} />;
    case 'sensor':
      return <Activity size={size} className={className} />;
    default:
      return <Lightbulb size={size} className={className} />;
  }
}

export interface ResourceStatus {
  icon: ReactNode;
  label: string;
}

/** Live state of the mapped resource; scenes have none. */
export function getHueResourceStatus(res: HueResources, hueId: string, hueType: string): ResourceStatus | null {
  if (hueType === 'light') {
    const light = res.lights.find((l) => l.id === hueId);
    if (!light) return null;
    if (!light.state.reachable) return { icon: <WifiOff size={14} className="text-gray-500" />, label: 'Nicht erreichbar' };
    if (light.state.on) return { icon: <Sun size={14} className="text-green-400" />, label: `An (${light.state.brightness}%)` };
    return { icon: <PowerOff size={14} className="text-red-400/70" />, label: 'Aus' };
  }
  if (hueType === 'group') {
    const group = res.groups.find((g) => g.id === hueId);
    if (!group) return null;
    if (group.state.all_on) {
      const bri = group.state.brightness != null ? ` (${group.state.brightness}%)` : '';
      return { icon: <Sun size={14} className="text-green-400" />, label: `Alle an${bri}` };
    }
    if (group.state.any_on) return { icon: <SunDim size={14} className="text-yellow-400" />, label: 'Teilweise an' };
    return { icon: <PowerOff size={14} className="text-red-400/70" />, label: 'Aus' };
  }
  if (hueType === 'sensor') {
    const sensor = res.sensors.find((s) => s.id === hueId);
    if (!sensor) return null;
    if (sensor.state.enabled) return { icon: <Activity size={14} className="text-green-400" />, label: 'Aktiv' };
    return { icon: <PowerOff size={14} className="text-gray-500" />, label: 'Deaktiviert' };
  }
  return null;
}
