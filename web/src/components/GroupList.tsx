import { useState } from 'react';
import { Group, Scene } from '../types';
import type { Mapping } from '../services/api';
import { Home, Layers, Power, Play, Radio } from 'lucide-react';
import { UdpInfoSection } from './UdpInfoSection';
import { getGroupUdpProperties, sanitizeName } from '../utils/udp';

interface GroupListProps {
  groups: Group[];
  scenes: Scene[];
  onToggle: (id: string, on: boolean) => void;
  onActivateScene: (id: string) => void;
  /** Rooms and zones share the component; the kind only changes the empty state. */
  kind?: 'room' | 'zone';
  mappings?: Mapping[];
  udpSendAll?: boolean;
}

export function GroupList({ groups, scenes, onToggle, onActivateScene, kind = 'room', mappings = [], udpSendAll = false }: GroupListProps) {
  const [expandedGroup, setExpandedGroup] = useState<string | null>(null);
  const [expandedUdp, setExpandedUdp] = useState<string | null>(null);

  const getGroupScenes = (groupId: string) => {
    return scenes
      .filter((scene) => scene.group_id === groupId)
      .sort((a, b) => a.name.localeCompare(b.name, 'de'));
  };

  const sortedGroups = [...groups].sort((a, b) =>
    a.name.localeCompare(b.name, 'de')
  );

  return (
    <div className="space-y-4">
      {sortedGroups.map((group) => {
        const groupScenes = getGroupScenes(group.id);
        const isExpanded = expandedGroup === group.id;
        const mapping = mappings.find(m => m.hue_id === group.id && m.enabled);
        const isUdpExpanded = expandedUdp === group.id;

        return (
          <div
            key={group.id}
            className={`
              bg-gray-800 rounded-xl overflow-hidden transition-all
              ${group.state.any_on ? 'ring-2 ring-hue-orange' : ''}
            `}
          >
            <div
              className="p-4 flex items-center justify-between cursor-pointer"
              onClick={() => setExpandedGroup(isExpanded ? null : group.id)}
            >
              <div className="flex items-center gap-3">
                <div
                  className={`
                    w-10 h-10 rounded-full flex items-center justify-center
                    ${group.state.any_on ? 'bg-hue-orange' : 'bg-gray-700'}
                  `}
                >
                  {group.type === 'room' ? (
                    <Home size={20} className={group.state.any_on ? 'text-gray-900' : 'text-gray-400'} />
                  ) : (
                    <Layers size={20} className={group.state.any_on ? 'text-gray-900' : 'text-gray-400'} />
                  )}
                </div>
                <div>
                  <h3 className="font-medium text-white">{group.name}</h3>
                  <p className="text-xs text-gray-400">
                    {group.lights.length} Geräte • {groupScenes.length} Szenen
                  </p>
                  <p className="text-xs text-gray-500 font-mono"><span className="text-gray-600">HUE ID:</span> {group.id}</p>
                </div>
              </div>

              <div className="flex items-center gap-1">
                {(mapping || udpSendAll) && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setExpandedUdp(isUdpExpanded ? null : group.id);
                    }}
                    title="UDP Status-Feedback"
                    className={`
                      p-2 rounded-lg transition-colors
                      ${isUdpExpanded
                        ? 'bg-green-700 text-white'
                        : 'bg-gray-700 text-gray-400 hover:bg-gray-600 hover:text-white'
                      }
                    `}
                  >
                    <Radio size={16} />
                  </button>
                )}
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onToggle(group.id, !group.state.any_on);
                  }}
                  className={`
                    p-2 rounded-lg transition-colors
                    ${group.state.any_on
                      ? 'bg-hue-orange text-gray-900'
                      : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
                    }
                  `}
                >
                  <Power size={20} />
                </button>
              </div>
            </div>

            {isUdpExpanded && (mapping || udpSendAll) && (() => {
              const loxoneId = mapping?.loxone_id ?? sanitizeName(group.name);
              return (
                <div className="px-4 pb-4">
                  <UdpInfoSection
                    loxoneId={loxoneId}
                    properties={getGroupUdpProperties(loxoneId)}
                    miniserverId={mapping?.miniserver_id}
                  />
                </div>
              );
            })()}

            {isExpanded && groupScenes.length > 0 && (
              <div className="border-t border-gray-700 p-4">
                <h4 className="text-sm text-gray-400 mb-3">Szenen</h4>
                <div className="grid grid-cols-2 gap-2">
                  {groupScenes.map((scene) => (
                    <button
                      key={scene.id}
                      onClick={() => onActivateScene(scene.id)}
                      className="
                        flex items-center gap-2 p-3 bg-gray-700 rounded-lg
                        hover:bg-gray-600 transition-colors text-left
                      "
                    >
                      <Play size={16} className="text-hue-orange flex-shrink-0" />
                      <div className="min-w-0">
                        <span className="text-sm text-white truncate block">{scene.name}</span>
                        <span className="text-xs text-gray-500 font-mono truncate block"><span className="text-gray-600">HUE ID:</span> {scene.id}</span>
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        );
      })}

      {groups.length === 0 && (
        <div className="text-center py-12">
          {kind === 'zone' ? (
            <Layers className="mx-auto text-gray-600 mb-4" size={48} />
          ) : (
            <Home className="mx-auto text-gray-600 mb-4" size={48} />
          )}
          <p className="text-gray-400">Keine {kind === 'zone' ? 'Zonen' : 'Räume'} gefunden</p>
        </div>
      )}
    </div>
  );
}
