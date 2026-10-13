import { useEffect, useState } from 'react';
import { Activity, ChevronDown, ChevronUp, Home, Layers, Lightbulb, Play, Power, Save, X } from 'lucide-react';
import type { Mapping } from '../../types';
import * as api from '../../services/api';
import { getHueResourceName, getTypeLabel, HUE_TYPES } from '../../utils/hueResources';
import { moodLoxoneId, sortByName } from '../../utils/moods';
import { sanitizeName } from '../../utils/udp';
import { errorMessage, type MappingEnv } from './env';
import { getTypeIcon } from './hueResourceUi';
import { HueResourceSelect } from './HueResourceSelect';
import { MappingFormFields } from './MappingFormFields';

type WizardMode = 'single' | 'mood';

interface MappingWizardProps {
  env: MappingEnv;
  onClose: () => void;
  /** Called with every mapping that was created, also when some mood scenes failed. */
  onCreated: (created: Mapping[]) => void;
}

const STEP_LABELS: Record<WizardMode, string[]> = {
  single: ['', 'Ressource wählen', 'Details eingeben', 'Zusammenfassung'],
  mood: ['', 'Gruppe wählen', 'Details eingeben', 'Szenen-Reihenfolge', 'Zusammenfassung'],
};

function typeIcon(type: string) {
  const cls = 'mx-auto mb-1';
  switch (type) {
    case 'group':
      return <Home size={20} className={cls} />;
    case 'scene':
      return <Play size={20} className={cls} />;
    case 'sensor':
      return <Activity size={20} className={cls} />;
    default:
      return <Lightbulb size={20} className={cls} />;
  }
}

/** Step-by-step creation of a single mapping or of a group with its mood scenes. */
export function MappingWizard({ env, onClose, onCreated }: MappingWizardProps) {
  const [mode, setMode] = useState<WizardMode | null>(null);
  const [step, setStep] = useState(0);
  const [form, setForm] = useState<Partial<Mapping>>({});
  const [loxoneIdManual, setLoxoneIdManual] = useState(false);
  const [moodOrder, setMoodOrder] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [partialErrors, setPartialErrors] = useState<string[]>([]);

  const { scenes } = env;
  const totalSteps = mode === 'mood' ? 4 : 3;
  const isLastStep = step === totalSteps;
  const done = partialErrors.length > 0;

  // Initialise the mood order when the group changes. `scenes` is deliberately
  // not a dependency: an auto-refresh must not reset the order the user made.
  useEffect(() => {
    if (mode !== 'mood' || !form.hue_id) {
      setMoodOrder([]);
      return;
    }
    setMoodOrder(sortByName(scenes.filter((s) => s.group_id === form.hue_id)).map((s) => s.id));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, form.hue_id]);

  const patch = (p: Partial<Mapping>) => setForm((prev) => ({ ...prev, ...p }));

  const selectMode = (m: WizardMode) => {
    setMode(m);
    if (m === 'mood') {
      patch({ hue_type: 'group' });
    } else if (!form.hue_type) {
      patch({ hue_type: 'light' });
    }
    // A resource prefilled from elsewhere skips straight to the details.
    setStep(form.hue_id ? 2 : 1);
  };

  const selectResource = (hueId: string, resourceName: string) => {
    setForm((prev) => {
      const name = prev.name || resourceName;
      return {
        ...prev,
        hue_id: hueId,
        name,
        ...(!loxoneIdManual && name ? { loxone_id: sanitizeName(name) } : {}),
      };
    });
  };

  const canProceed = (): boolean => {
    if (step === 1) return !!form.hue_type && !!form.hue_id;
    if (step === 2) return !!form.name && !!form.loxone_id;
    return true;
  };

  const back = () => {
    setError(null);
    if (step === 1) {
      setStep(0);
      setMode(null);
    } else {
      setStep((s) => s - 1);
    }
  };

  const moveMoodScene = (index: number, direction: 'up' | 'down') => {
    const target = direction === 'up' ? index - 1 : index + 1;
    if (target < 0 || target >= moodOrder.length) return;
    const next = [...moodOrder];
    [next[index], next[target]] = [next[target], next[index]];
    setMoodOrder(next);
  };

  const submit = async () => {
    if (!form.name || !form.loxone_id || !form.hue_id || submitting) return;
    setSubmitting(true);
    setError(null);

    const base = {
      enabled: true,
      feedback_udp: form.feedback_udp,
      feedback_http: form.feedback_http,
      miniserver_id: form.miniserver_id,
    };
    const created: Mapping[] = [];
    const failed: string[] = [];

    try {
      created.push(await api.createMapping({
        ...base,
        name: form.name,
        loxone_id: form.loxone_id,
        hue_id: form.hue_id,
        hue_type: form.hue_type || 'light',
        description: form.description,
      }));
    } catch (err) {
      setError(errorMessage(err, 'Mapping konnte nicht erstellt werden'));
      setSubmitting(false);
      return;
    }

    if (mode === 'mood') {
      const moodTargets = [
        {
          name: `${form.name} - Alle Lichter aus`,
          loxone_id: moodLoxoneId(form.loxone_id, 0),
          hue_id: form.hue_id,
          hue_type: 'group',
          description: `Mood 0 für ${form.name} (Gruppe ausschalten)`,
        },
        ...moodOrder.flatMap((sceneId, i) => {
          const scene = scenes.find((s) => s.id === sceneId);
          if (!scene) return [];
          return [{
            name: `${form.name} - ${scene.name}`,
            loxone_id: moodLoxoneId(form.loxone_id!, i + 1),
            hue_id: scene.id,
            hue_type: 'scene',
            description: `Mood ${i + 1} für ${form.name}`,
          }];
        }),
      ];
      for (const target of moodTargets) {
        try {
          created.push(await api.createMapping({ ...base, ...target }));
        } catch (err) {
          failed.push(`${target.loxone_id}: ${errorMessage(err, 'unbekannter Fehler')}`);
        }
      }
    }

    onCreated(created);
    setSubmitting(false);
    if (failed.length > 0) {
      setPartialErrors(failed);
    } else {
      onClose();
    }
  };

  return (
    <div className="bg-gray-800 rounded-xl overflow-hidden">
      <div className="bg-gray-700/50 px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h3 className="font-medium text-white">Neues Mapping</h3>
          {mode && (
            <span className="text-xs text-gray-500">
              Schritt {step} von {totalSteps} &mdash; {STEP_LABELS[mode][step] || ''}
            </span>
          )}
        </div>
        <button type="button" aria-label="Schliessen" onClick={onClose} className="text-gray-400 hover:text-white">
          <X size={18} />
        </button>
      </div>

      <div className="p-4 space-y-4">
        {step === 0 && (
          <div>
            <p className="text-sm text-gray-400 mb-3">Welche Art von Mapping möchtest du erstellen?</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => selectMode('single')}
                className="p-5 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors text-left space-y-2"
              >
                <Lightbulb size={28} className="text-hue-orange" />
                <span className="font-medium text-white block">Einzelsteuerung</span>
                <span className="text-xs text-gray-400 block">Ein einzelnes Licht, eine Gruppe oder Szene steuern</span>
              </button>
              <button
                type="button"
                onClick={() => selectMode('mood')}
                className="p-5 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors text-left space-y-2"
              >
                <Layers size={28} className="text-hue-orange" />
                <span className="font-medium text-white block">Mood-Steuerung</span>
                <span className="text-xs text-gray-400 block">Lichtsteuerungs-Baustein mit Szenen-Stimmungen</span>
              </button>
            </div>
          </div>
        )}

        {mode === 'single' && step === 1 && (
          <div className="space-y-3">
            <p className="text-sm text-gray-400">Wähle den Typ und die HUE-Ressource:</p>
            <div className="grid grid-cols-4 gap-2">
              {HUE_TYPES.map((type) => (
                <button
                  key={type}
                  type="button"
                  onClick={() => patch({ hue_type: type, hue_id: '', name: '' })}
                  className={`p-3 rounded-lg text-center transition-colors ${
                    form.hue_type === type ? 'bg-hue-orange text-gray-900' : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                  }`}
                >
                  {typeIcon(type)}
                  <span className="text-sm block">{getTypeLabel(type)}</span>
                </button>
              ))}
            </div>
            <HueResourceSelect hueType={form.hue_type || 'light'} value={form.hue_id || ''} onChange={selectResource} resources={env} />
          </div>
        )}

        {mode === 'mood' && step === 1 && (
          <div className="space-y-3">
            <p className="text-sm text-gray-400">Wähle die Gruppe / den Raum für die Mood-Steuerung:</p>
            <HueResourceSelect hueType="group" value={form.hue_id || ''} onChange={selectResource} resources={env} placeholder="Gruppe wählen..." />
            {form.hue_id && (
              <div className="bg-gray-700/30 rounded-lg p-3 flex items-center gap-2">
                <Play size={14} className="text-hue-orange" />
                <span className="text-xs text-gray-400">
                  {scenes.filter((s) => s.group_id === form.hue_id).length} Szenen verfügbar
                </span>
              </div>
            )}
          </div>
        )}

        {step === 2 && (
          <div className="space-y-3">
            <p className="text-sm text-gray-400">Gib Name und Loxone ID ein:</p>
            <MappingFormFields
              value={form}
              onChange={patch}
              env={env}
              autoLoxoneId={!loxoneIdManual}
              onLoxoneIdEdited={() => setLoxoneIdManual(true)}
              loxoneIdPlaceholder={mode === 'mood' ? 'Loxone ID (Basis)' : 'Loxone ID'}
              showDescription={mode === 'single'}
            >
              {mode === 'mood' && (
                <p className="text-xs text-gray-500">
                  Die Loxone ID wird als Basis verwendet. Mood-Szenen erhalten automatisch Suffixe: _mood_0, _mood_1, etc.
                </p>
              )}
            </MappingFormFields>
          </div>
        )}

        {mode === 'single' && step === 3 && (
          <div className="space-y-3">
            <p className="text-sm text-gray-400">Überprüfe die Angaben:</p>
            <div className="bg-gray-700/50 rounded-lg p-4 space-y-3">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 bg-gray-600 rounded-full flex items-center justify-center">{getTypeIcon(form.hue_type || 'light')}</div>
                <div>
                  <span className="text-white font-medium block">{form.name}</span>
                  <span className="text-xs text-gray-400">{getTypeLabel(form.hue_type || 'light')}</span>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <span className="text-gray-500 text-xs block">HUE Ressource</span>
                  <span className="text-gray-300">{getHueResourceName(env, form.hue_id || '', form.hue_type || 'light')}</span>
                </div>
                <div>
                  <span className="text-gray-500 text-xs block">Loxone ID</span>
                  <code className="text-hue-orange font-mono">{form.loxone_id}</code>
                </div>
              </div>
              {form.description && (
                <div className="text-sm">
                  <span className="text-gray-500 text-xs block">Beschreibung</span>
                  <span className="text-gray-300">{form.description}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {mode === 'mood' && step === 3 && (
          <div className="space-y-2">
            <p className="text-sm text-gray-400">Ordne die Szenen-Reihenfolge an:</p>
            <div className="flex items-center gap-2 text-sm bg-gray-700/40 rounded-lg px-3 py-2">
              <span className="w-7 text-center text-xs font-mono font-bold rounded px-1 py-0.5 bg-red-900/50 text-red-400 flex-shrink-0">0</span>
              <Power size={14} className="text-red-400 flex-shrink-0" />
              <span className="text-gray-300">Alle Lichter aus</span>
              <code className="text-xs text-gray-500 font-mono ml-auto">{moodLoxoneId(form.loxone_id || '', 0)}</code>
            </div>
            {moodOrder.map((sceneId, idx) => {
              const scene = scenes.find((s) => s.id === sceneId);
              if (!scene) return null;
              const first = idx === 0;
              const last = idx === moodOrder.length - 1;
              return (
                <div key={sceneId} className="flex items-center gap-2 text-sm bg-gray-700/40 rounded-lg px-3 py-2">
                  <span className="w-7 text-center text-xs font-mono font-bold rounded px-1 py-0.5 bg-hue-orange/20 text-hue-orange flex-shrink-0">{idx + 1}</span>
                  <Play size={14} className="text-hue-orange flex-shrink-0" />
                  <span className="text-gray-300 truncate">{scene.name}</span>
                  <code className="text-xs text-gray-500 font-mono ml-auto flex-shrink-0">{moodLoxoneId(form.loxone_id || '', idx + 1)}</code>
                  <div className="flex flex-shrink-0">
                    <button
                      type="button"
                      onClick={() => moveMoodScene(idx, 'up')}
                      disabled={first}
                      className={`p-0.5 ${first ? 'text-gray-600' : 'text-gray-400 hover:text-white'}`}
                      title="Nach oben"
                    >
                      <ChevronUp size={14} />
                    </button>
                    <button
                      type="button"
                      onClick={() => moveMoodScene(idx, 'down')}
                      disabled={last}
                      className={`p-0.5 ${last ? 'text-gray-600' : 'text-gray-400 hover:text-white'}`}
                      title="Nach unten"
                    >
                      <ChevronDown size={14} />
                    </button>
                  </div>
                </div>
              );
            })}
            {moodOrder.length === 0 && <p className="text-xs text-gray-500">Keine Szenen für diese Gruppe gefunden</p>}
          </div>
        )}

        {mode === 'mood' && step === 4 && (
          <div className="space-y-2">
            <p className="text-sm text-gray-400">Folgende Mappings werden erstellt:</p>
            <div className="bg-gray-700/50 rounded-lg p-3 flex items-center gap-3">
              <div className="w-8 h-8 bg-gray-600 rounded-full flex items-center justify-center flex-shrink-0">
                <Home size={16} className="text-hue-orange" />
              </div>
              <div className="min-w-0">
                <span className="text-white font-medium block truncate">{form.name}</span>
                <span className="text-xs text-gray-400">{getHueResourceName(env, form.hue_id || '', 'group')}</span>
              </div>
              <code className="text-hue-orange text-xs font-mono ml-auto flex-shrink-0">{form.loxone_id}</code>
            </div>
            <div className="bg-gray-700/30 rounded-lg p-2 ml-6 flex items-center gap-2 text-sm">
              <span className="w-6 text-center text-xs font-mono font-bold bg-red-900/50 text-red-400 rounded px-1">0</span>
              <Power size={12} className="text-red-400" />
              <span className="text-gray-400">Alle Lichter aus</span>
              <code className="text-[10px] text-gray-500 font-mono ml-auto">{moodLoxoneId(form.loxone_id || '', 0)}</code>
            </div>
            {moodOrder.map((sceneId, idx) => {
              const scene = scenes.find((s) => s.id === sceneId);
              if (!scene) return null;
              return (
                <div key={sceneId} className="bg-gray-700/30 rounded-lg p-2 ml-6 flex items-center gap-2 text-sm">
                  <span className="w-6 text-center text-xs font-mono font-bold bg-hue-orange/20 text-hue-orange rounded px-1">{idx + 1}</span>
                  <Play size={12} className="text-hue-orange" />
                  <span className="text-gray-400 truncate">{scene.name}</span>
                  <code className="text-[10px] text-gray-500 font-mono ml-auto flex-shrink-0">{moodLoxoneId(form.loxone_id || '', idx + 1)}</code>
                </div>
              );
            })}
            <p className="text-xs text-gray-500 pt-1">
              Gesamt: {moodOrder.length + 2} Mappings (1 Gruppe + 1 Aus + {moodOrder.length} Szenen)
            </p>
          </div>
        )}

        {error && (
          <p className="text-sm text-red-300 bg-red-900/30 border border-red-500/50 rounded-lg px-3 py-2">{error}</p>
        )}

        {done && (
          <div className="text-sm bg-yellow-900/30 border border-yellow-500/50 rounded-lg px-3 py-2 space-y-1">
            <p className="text-yellow-200">
              Das Gruppen-Mapping wurde erstellt, {partialErrors.length} Mood-Szene(n) jedoch nicht:
            </p>
            <ul className="text-xs text-yellow-100/80 list-disc ml-4">
              {partialErrors.map((e) => <li key={e}>{e}</li>)}
            </ul>
            <p className="text-xs text-yellow-100/80">Fehlende Szenen lassen sich unter dem Gruppen-Mapping einzeln nachtragen.</p>
          </div>
        )}

        {step > 0 && (
          <div className="flex gap-2 pt-2 border-t border-gray-700">
            {done ? (
              <>
                <div className="flex-1" />
                <button type="button" onClick={onClose} className="px-6 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600 transition-colors">
                  Schliessen
                </button>
              </>
            ) : (
              <>
                <button type="button" onClick={back} className="flex items-center gap-1 px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600 transition-colors">
                  Zurück
                </button>
                <div className="flex-1" />
                {isLastStep ? (
                  <button
                    type="button"
                    onClick={submit}
                    disabled={submitting}
                    className="flex items-center gap-2 px-6 py-2 bg-green-600 text-white rounded-lg hover:bg-green-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <Save size={18} />
                    {submitting ? 'Wird erstellt...' : 'Erstellen'}
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => setStep((s) => s + 1)}
                    disabled={!canProceed()}
                    className="flex items-center gap-1 px-6 py-2 bg-hue-orange text-gray-900 rounded-lg hover:bg-orange-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Weiter
                  </button>
                )}
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
