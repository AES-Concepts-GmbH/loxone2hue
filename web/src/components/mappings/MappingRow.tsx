import { useState } from 'react';
import { Edit2, Save, Terminal, Trash2, X } from 'lucide-react';
import type { Mapping } from '../../types';
import { Tooltip } from '../Tooltip';
import { getHueResourceName, getTypeLabel } from '../../utils/hueResources';
import { errorMessage, type MappingEnv } from './env';
import { getHueResourceStatus, getTypeIcon } from './hueResourceUi';
import { HueResourceSelect } from './HueResourceSelect';
import { MappingFormFields } from './MappingFormFields';
import { MappingGuide } from './MappingGuide';
import { MoodChildren } from './MoodChildren';

interface MappingRowProps {
  mapping: Mapping;
  moods: Mapping[];
  env: MappingEnv;
  onNavigateToHueElement?: (hueId: string, hueType: string) => void;
  onUpdate: (mapping: Mapping) => Promise<void>;
  onDelete: (mapping: Mapping, moods: Mapping[]) => void;
  onSwapMoods: (moodA: Mapping, moodB: Mapping) => void;
  onDeleteMoodScene: (mood: Mapping) => void;
  onAddMoodScene: (parentLoxoneId: string, sceneId: string) => Promise<void>;
}

/** One top-level mapping: summary, inline edit, Loxone guide and its mood scenes. */
export function MappingRow({
  mapping,
  moods,
  env,
  onNavigateToHueElement,
  onUpdate,
  onDelete,
  onSwapMoods,
  onDeleteMoodScene,
  onAddMoodScene,
}: MappingRowProps) {
  const [draft, setDraft] = useState<Mapping | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [guideOpen, setGuideOpen] = useState(false);

  const hasMoods = moods.length > 0;
  const status = getHueResourceStatus(env, mapping.hue_id, mapping.hue_type);
  const resourceName = getHueResourceName(env, mapping.hue_id, mapping.hue_type);
  const miniserver = mapping.miniserver_id && env.miniservers.length > 1
    ? env.miniservers.find((m) => m.id === mapping.miniserver_id)
    : undefined;
  const feedbackRestricted = (env.hasUDP || env.hasHTTP) && (mapping.feedback_udp === false || mapping.feedback_http === false);
  const feedbackLabel = [
    mapping.feedback_udp !== false && env.hasUDP ? 'UDP' : '',
    mapping.feedback_http !== false && env.hasHTTP ? 'HTTP' : '',
  ].filter(Boolean).join(', ') || 'keins';

  const handleSave = async () => {
    if (!draft || saving) return;
    setSaving(true);
    setSaveError(null);
    try {
      await onUpdate(draft);
      setDraft(null);
    } catch (err) {
      setSaveError(errorMessage(err, 'Speichern fehlgeschlagen'));
    } finally {
      setSaving(false);
    }
  };

  const cancelEdit = () => {
    setDraft(null);
    setSaveError(null);
  };

  return (
    <div>
      <div className={`bg-gray-800 rounded-xl p-4 transition-all ${!mapping.enabled ? 'opacity-50' : ''} ${hasMoods ? 'rounded-b-none' : ''}`}>
        {draft ? (
          <div className="space-y-3">
            <MappingFormFields value={draft} onChange={(patch) => setDraft({ ...draft, ...patch })} env={env}>
              <div className="grid grid-cols-2 gap-3">
                <select
                  title="HUE Ressourcentyp"
                  value={draft.hue_type || 'light'}
                  onChange={(e) => setDraft({ ...draft, hue_type: e.target.value, hue_id: '' })}
                  className="bg-gray-700 text-white rounded-lg px-3 py-2"
                >
                  <option value="light">Licht</option>
                  <option value="group">Gruppe/Raum</option>
                  <option value="scene">Szene</option>
                  <option value="sensor">Sensor</option>
                </select>
                <HueResourceSelect
                  hueType={draft.hue_type}
                  value={draft.hue_id || ''}
                  onChange={(hueId) => setDraft({ ...draft, hue_id: hueId })}
                  resources={env}
                  className="bg-gray-700 text-white rounded-lg px-3 py-2"
                  placeholder="HUE Ressource wählen..."
                />
              </div>
            </MappingFormFields>
            {saveError && (
              <p className="text-sm text-red-300 bg-red-900/30 border border-red-500/50 rounded-lg px-3 py-2">{saveError}</p>
            )}
            <div className="flex gap-2">
              <button
                type="button"
                onClick={handleSave}
                disabled={saving || !draft.name || !draft.loxone_id || !draft.hue_id}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-500 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Save size={18} />
                {saving ? 'Speichert...' : 'Speichern'}
              </button>
              <Tooltip content="Abbrechen" position="top">
                <button type="button" aria-label="Abbrechen" onClick={cancelEdit} className="px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600">
                  <X size={18} />
                </button>
              </Tooltip>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="relative w-10 h-10 bg-gray-700 rounded-full flex items-center justify-center">
                  {getTypeIcon(mapping.hue_type)}
                  {status && (
                    <Tooltip content={status.label} position="top">
                      <span className="absolute -top-1 -right-1 bg-gray-800 rounded-full p-0.5 border border-gray-700">{status.icon}</span>
                    </Tooltip>
                  )}
                </div>
                <div>
                  <h3 className="font-medium text-white flex items-center gap-2">
                    {mapping.name}
                    {hasMoods && (
                      <span className="text-[10px] bg-hue-orange/20 text-hue-orange px-1.5 py-0.5 rounded-full font-normal">
                        {moods.length} Moods
                      </span>
                    )}
                    {status && <span className="text-[10px] text-gray-500 font-normal">{status.label}</span>}
                  </h3>
                  <p className="text-xs text-gray-400">
                    {mapping.loxone_id} →{' '}
                    {onNavigateToHueElement ? (
                      <button
                        type="button"
                        onClick={() => onNavigateToHueElement(mapping.hue_id, mapping.hue_type)}
                        className="text-hue-orange hover:underline"
                      >
                        {resourceName}
                      </button>
                    ) : (
                      resourceName
                    )}
                  </p>
                  <p className="text-xs text-gray-500">
                    Typ: {getTypeLabel(mapping.hue_type)}
                    {miniserver && <> &bull; Miniserver: {miniserver.name || miniserver.ip}</>}
                    {feedbackRestricted && <> &bull; Feedback: {feedbackLabel}</>}
                  </p>
                </div>
              </div>
              <div className="flex gap-2">
                <Tooltip content="Loxone Config" position="bottom">
                  <button
                    type="button"
                    aria-label="Loxone Konfiguration anzeigen"
                    onClick={() => setGuideOpen((o) => !o)}
                    className={`p-2 transition-colors ${guideOpen ? 'text-hue-orange' : 'text-gray-400 hover:text-white'}`}
                  >
                    <Terminal size={18} />
                  </button>
                </Tooltip>
                <Tooltip content="Bearbeiten" position="bottom">
                  <button type="button" aria-label="Bearbeiten" onClick={() => setDraft({ ...mapping })} className="p-2 text-gray-400 hover:text-white">
                    <Edit2 size={18} />
                  </button>
                </Tooltip>
                <Tooltip content={hasMoods ? 'Gruppe löschen' : 'Löschen'} position="bottom">
                  <button type="button" aria-label="Löschen" onClick={() => onDelete(mapping, moods)} className="p-2 text-gray-400 hover:text-red-400">
                    <Trash2 size={18} />
                  </button>
                </Tooltip>
              </div>
            </div>

            {guideOpen && <MappingGuide mapping={mapping} env={env} />}
          </div>
        )}
      </div>

      {hasMoods && (
        <MoodChildren
          parent={mapping}
          moods={moods}
          env={env}
          onNavigateToHueElement={onNavigateToHueElement}
          onSwap={onSwapMoods}
          onDelete={onDeleteMoodScene}
          onAdd={onAddMoodScene}
        />
      )}
    </div>
  );
}
