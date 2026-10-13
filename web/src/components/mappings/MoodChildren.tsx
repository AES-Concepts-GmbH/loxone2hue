import { useState } from 'react';
import { ChevronDown, ChevronUp, Play, Plus, Power, Trash2 } from 'lucide-react';
import type { Mapping } from '../../types';
import { getHueResourceName } from '../../utils/hueResources';
import { getMoodNumber, sortByName } from '../../utils/moods';
import type { MappingEnv } from './env';

interface MoodChildrenProps {
  parent: Mapping;
  moods: Mapping[];
  env: MappingEnv;
  onNavigateToHueElement?: (hueId: string, hueType: string) => void;
  onSwap: (moodA: Mapping, moodB: Mapping) => void;
  onDelete: (mood: Mapping) => void;
  onAdd: (parentLoxoneId: string, sceneId: string) => Promise<void>;
}

/** The "_mood_<n>" scene mappings listed under their group mapping. */
export function MoodChildren({ parent, moods, env, onNavigateToHueElement, onSwap, onDelete, onAdd }: MoodChildrenProps) {
  const [adding, setAdding] = useState(false);

  const sceneMoods = moods.filter((m) => getMoodNumber(m.loxone_id) > 0);
  const availableScenes = sortByName(
    env.scenes
      .filter((s) => s.group_id === parent.hue_id)
      .filter((s) => !moods.some((mc) => mc.hue_id === s.id)),
  );

  const handleAdd = async (sceneId: string) => {
    await onAdd(parent.loxone_id, sceneId);
    setAdding(false);
  };

  return (
    <div className="bg-gray-800/60 rounded-b-xl border-t border-gray-700 px-4 py-3 space-y-1">
      <div className="flex items-center gap-2 mb-2">
        <Play size={14} className="text-hue-orange" />
        <span className="text-xs font-medium text-gray-400">Mood-Szenen ({moods.length})</span>
        <button
          type="button"
          aria-label="Mood-Szene hinzufügen"
          onClick={() => setAdding((a) => !a)}
          className="p-0.5 text-gray-400 hover:text-green-400"
          title="Szene hinzufügen"
        >
          <Plus size={14} />
        </button>
      </div>

      {adding && (
        <div className="bg-gray-700/30 rounded-lg p-3 mb-2 space-y-2">
          <p className="text-xs text-gray-400">Szene hinzufügen:</p>
          <div className="space-y-1 max-h-48 overflow-y-auto">
            {availableScenes.map((scene) => (
              <button
                key={scene.id}
                type="button"
                onClick={() => handleAdd(scene.id)}
                className="w-full text-left flex items-center gap-2 px-3 py-2 bg-gray-700/50 rounded-lg hover:bg-gray-600 text-sm text-gray-300 hover:text-white transition-colors"
              >
                <Play size={12} className="text-hue-orange flex-shrink-0" />
                <span className="truncate">{scene.name}</span>
              </button>
            ))}
            {availableScenes.length === 0 && (
              <p className="text-xs text-gray-500 py-1">Alle Szenen dieser Gruppe sind bereits zugeordnet.</p>
            )}
          </div>
          <button type="button" onClick={() => setAdding(false)} className="text-xs text-gray-500 hover:text-gray-300">
            Abbrechen
          </button>
        </div>
      )}

      {moods.map((mood) => {
        const moodNum = getMoodNumber(mood.loxone_id);
        const isOff = moodNum === 0;
        const sceneIdx = sceneMoods.indexOf(mood);
        const canUp = sceneIdx > 0;
        const canDown = sceneIdx >= 0 && sceneIdx < sceneMoods.length - 1;
        const name = getHueResourceName(env, mood.hue_id, mood.hue_type);
        return (
          <div key={mood.id} className="flex items-center gap-2 text-sm bg-gray-700/40 rounded-lg px-3 py-2">
            <span
              className={`w-7 text-center text-xs font-mono font-bold rounded px-1 py-0.5 flex-shrink-0 ${
                isOff ? 'bg-red-900/50 text-red-400' : 'bg-hue-orange/20 text-hue-orange'
              }`}
            >
              {moodNum}
            </span>
            {isOff ? (
              <Power size={14} className="text-red-400 flex-shrink-0" />
            ) : (
              <Play size={14} className="text-hue-orange flex-shrink-0" />
            )}
            <span className="text-gray-300 truncate">
              {isOff ? (
                'Alle Lichter aus'
              ) : onNavigateToHueElement ? (
                <button
                  type="button"
                  onClick={() => onNavigateToHueElement(mood.hue_id, mood.hue_type)}
                  className="text-hue-orange hover:underline"
                >
                  {name}
                </button>
              ) : (
                name
              )}
            </span>
            <code className="text-[10px] text-gray-500 font-mono ml-auto flex-shrink-0">{mood.loxone_id}</code>
            {!isOff && (
              <div className="flex flex-shrink-0">
                <button
                  type="button"
                  onClick={() => canUp && onSwap(mood, sceneMoods[sceneIdx - 1])}
                  disabled={!canUp}
                  className={`p-0.5 ${canUp ? 'text-gray-400 hover:text-white' : 'text-gray-600'}`}
                  title="Nach oben"
                >
                  <ChevronUp size={14} />
                </button>
                <button
                  type="button"
                  onClick={() => canDown && onSwap(mood, sceneMoods[sceneIdx + 1])}
                  disabled={!canDown}
                  className={`p-0.5 ${canDown ? 'text-gray-400 hover:text-white' : 'text-gray-600'}`}
                  title="Nach unten"
                >
                  <ChevronDown size={14} />
                </button>
              </div>
            )}
            <button
              type="button"
              aria-label="Mood-Szene löschen"
              onClick={() => onDelete(mood)}
              className="p-0.5 text-gray-500 hover:text-red-400 flex-shrink-0"
              title="Mood-Szene löschen"
            >
              <Trash2 size={14} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
