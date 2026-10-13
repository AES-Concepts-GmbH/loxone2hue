import type { HueResources } from '../../utils/hueResources';
import { getScenesWithGroupName, getSceneDisplayName } from '../../utils/hueResources';
import { sortByName } from '../../utils/moods';

interface HueResourceSelectProps {
  hueType: string;
  value: string;
  /** Receives the chosen id and its display name (for pre-filling the mapping name). */
  onChange: (hueId: string, name: string) => void;
  resources: HueResources;
  className?: string;
  placeholder?: string;
}

/** Dropdown of lights, groups, scenes or sensors depending on `hueType`. */
export function HueResourceSelect({
  hueType,
  value,
  onChange,
  resources,
  className = 'w-full bg-gray-700 text-white rounded-lg px-3 py-2.5',
  placeholder = 'Ressource wählen...',
}: HueResourceSelectProps) {
  const options = (() => {
    switch (hueType) {
      case 'group':
        return sortByName(resources.groups).map((g) => ({ id: g.id, label: `${g.name} (${g.type})`, name: g.name }));
      case 'scene':
        return getScenesWithGroupName(resources).map((s) => {
          const label = getSceneDisplayName(resources.groups, s);
          return { id: s.id, label, name: label };
        });
      case 'sensor':
        return sortByName(resources.sensors).map((s) => ({ id: s.id, label: `${s.name} (${s.type})`, name: s.name }));
      default:
        return sortByName(resources.lights).map((l) => ({ id: l.id, label: l.name, name: l.name }));
    }
  })();

  return (
    <select
      title="HUE Ressource"
      value={value}
      onChange={(e) => {
        const id = e.target.value;
        onChange(id, options.find((o) => o.id === id)?.name || '');
      }}
      className={className}
    >
      <option value="">{placeholder}</option>
      {options.map((o) => (
        <option key={o.id} value={o.id}>{o.label}</option>
      ))}
    </select>
  );
}
