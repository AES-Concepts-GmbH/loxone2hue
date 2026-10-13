import { useCallback, useEffect, useState } from 'react';
import { AlertCircle, Download, Link2, Plus, Upload } from 'lucide-react';
import type { Group, Light, Mapping, Scene, Sensor } from '../types';
import * as api from '../services/api';
import { Tooltip } from './Tooltip';
import { errorMessage, type MappingEnv } from './mappings/env';
import { ImportModal } from './mappings/ImportModal';
import { LoxoneExportPanel } from './mappings/LoxoneExportPanel';
import { MappingRow } from './mappings/MappingRow';
import { MappingWizard } from './mappings/MappingWizard';
import { getHueResourceName } from '../utils/hueResources';
import { getMoodChildren, getMoodNumber, getParentLoxoneId, isMoodChild, moodLoxoneId, sortByName } from '../utils/moods';

interface MappingConfigProps {
  lights: Light[];
  groups: Group[];
  scenes: Scene[];
  sensors?: Sensor[];
  onNavigateToHueElement?: (hueId: string, hueType: string) => void;
  /** Called after mappings were created, changed or deleted here. */
  onMappingsChanged?: () => void;
}

export function MappingConfig({ lights, groups, scenes, sensors = [], onNavigateToHueElement, onMappingsChanged }: MappingConfigProps) {
  const [mappings, setMappings] = useState<Mapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [miniservers, setMiniservers] = useState<api.MiniserverConfig[]>([]);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [importFile, setImportFile] = useState<api.MappingsBackup | null>(null);
  const [isExporting, setIsExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const env: MappingEnv = {
    lights,
    groups,
    scenes,
    sensors,
    miniservers,
    hasUDP: miniservers.some((ms) => ms.udp_enabled),
    hasHTTP: miniservers.some((ms) => ms.http_enabled),
  };

  const loadMappings = useCallback(async () => {
    try {
      const response = await api.getMappings();
      setMappings(response.mappings || []);
    } catch (err) {
      setError(errorMessage(err, 'Mappings konnten nicht geladen werden'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadMappings();
    api.getConfig()
      .then((cfg) => setMiniservers(cfg.loxone?.miniservers || []))
      .catch(() => {});
  }, [loadMappings]);

  // Tell the Dashboard the mappings changed so the other tabs refresh theirs.
  const notifyChanged = () => onMappingsChanged?.();

  /** Runs an API action; failures end up in the banner instead of only the console. */
  const run = async (what: string, action: () => Promise<void>) => {
    setError(null);
    try {
      await action();
    } catch (err) {
      console.error(`${what} failed:`, err);
      setError(`${what} fehlgeschlagen: ${errorMessage(err, 'unbekannter Fehler')}`);
    }
  };

  const handleCreated = (created: Mapping[]) => {
    if (created.length === 0) return;
    setMappings((prev) => [...prev, ...created]);
    notifyChanged();
  };

  const handleUpdate = async (mapping: Mapping) => {
    // Errors propagate to the row, which shows them next to the form.
    const updated = await api.updateMapping(mapping.id, mapping);
    setMappings((prev) => prev.map((m) => (m.id === mapping.id ? updated : m)));
    notifyChanged();
  };

  const handleDelete = (mapping: Mapping, moods: Mapping[]) => {
    const message = moods.length > 0
      ? `Mapping "${mapping.name}" und ${moods.length} zugehörige Mood-Mappings löschen?`
      : 'Mapping wirklich löschen?';
    if (!confirm(message)) return;

    run('Löschen', async () => {
      for (const child of moods) {
        await api.deleteMapping(child.id);
      }
      await api.deleteMapping(mapping.id);
      const deleted = new Set([mapping.id, ...moods.map((c) => c.id)]);
      setMappings((prev) => prev.filter((m) => !deleted.has(m.id)));
      notifyChanged();
    });
  };

  const parentNameFor = (parentLoxoneId: string) =>
    mappings.find((m) => m.loxone_id === parentLoxoneId)?.name || parentLoxoneId;

  const handleSwapMoods = (moodA: Mapping, moodB: Mapping) => {
    const parentLoxoneId = getParentLoxoneId(moodA.loxone_id);
    if (!parentLoxoneId) return;
    const numA = getMoodNumber(moodA.loxone_id);
    const numB = getMoodNumber(moodB.loxone_id);
    const parentName = parentNameFor(parentLoxoneId);

    run('Reihenfolge ändern', async () => {
      await api.updateMapping(moodA.id, {
        ...moodA,
        loxone_id: moodLoxoneId(parentLoxoneId, numB),
        description: `Mood ${numB} für ${parentName}`,
      });
      await api.updateMapping(moodB.id, {
        ...moodB,
        loxone_id: moodLoxoneId(parentLoxoneId, numA),
        description: `Mood ${numA} für ${parentName}`,
      });
      await loadMappings();
      notifyChanged();
    });
  };

  const handleDeleteMoodScene = (mood: Mapping) => {
    const moodNum = getMoodNumber(mood.loxone_id);
    const parentLoxoneId = getParentLoxoneId(mood.loxone_id);
    if (!parentLoxoneId) return;

    const sceneName = moodNum === 0 ? 'Alle Lichter aus' : getHueResourceName(env, mood.hue_id, mood.hue_type);
    if (!confirm(`Mood-Szene "${sceneName}" (Mood ${moodNum}) wirklich löschen?`)) return;

    run('Mood-Szene löschen', async () => {
      await api.deleteMapping(mood.id);

      // Close the gap so the Lichtsteuerungs-Baustein numbers stay contiguous.
      const parentName = parentNameFor(parentLoxoneId);
      const remaining = getMoodChildren(mappings, parentLoxoneId)
        .filter((m) => m.id !== mood.id && getMoodNumber(m.loxone_id) > 0);
      for (let i = 0; i < remaining.length; i++) {
        const expected = i + 1;
        if (getMoodNumber(remaining[i].loxone_id) !== expected) {
          await api.updateMapping(remaining[i].id, {
            ...remaining[i],
            loxone_id: moodLoxoneId(parentLoxoneId, expected),
            description: `Mood ${expected} für ${parentName}`,
          });
        }
      }

      await loadMappings();
      notifyChanged();
    });
  };

  const handleAddMoodScene = async (parentLoxoneId: string, sceneId: string) => {
    const parent = mappings.find((m) => m.loxone_id === parentLoxoneId);
    const scene = scenes.find((s) => s.id === sceneId);
    if (!parent || !scene) return;

    const sceneMoods = getMoodChildren(mappings, parentLoxoneId).filter((m) => getMoodNumber(m.loxone_id) > 0);
    const nextNum = sceneMoods.length > 0 ? Math.max(...sceneMoods.map((m) => getMoodNumber(m.loxone_id))) + 1 : 1;

    await run('Mood-Szene hinzufügen', async () => {
      await api.createMapping({
        name: `${parent.name} - ${scene.name}`,
        loxone_id: moodLoxoneId(parentLoxoneId, nextNum),
        hue_id: scene.id,
        hue_type: 'scene',
        enabled: true,
        description: `Mood ${nextNum} für ${parent.name}`,
        feedback_udp: parent.feedback_udp,
        feedback_http: parent.feedback_http,
        miniserver_id: parent.miniserver_id,
      });
      await loadMappings();
      notifyChanged();
    });
  };

  const handleExport = async () => {
    setIsExporting(true);
    await run('Export', async () => {
      const backup = await api.exportMappings();
      const blob = new Blob([JSON.stringify(backup, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `loxone-mappings-${new Date().toISOString().split('T')[0]}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    });
    setIsExporting(false);
  };

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;

    setError(null);
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const backup = JSON.parse(e.target?.result as string) as api.MappingsBackup;
        if (!backup.version || !Array.isArray(backup.mappings)) {
          throw new Error('Ungültiges Backup-Format');
        }
        setImportFile(backup);
      } catch {
        setError('Die Datei konnte nicht gelesen werden. Bitte wähle eine gültige Backup-Datei.');
      }
    };
    reader.readAsText(file);
  };

  const handleImported = async () => {
    await loadMappings();
    notifyChanged();
  };

  if (loading) {
    return <div className="text-center py-8 text-gray-400">Lade Mappings...</div>;
  }

  const topLevel = sortByName(mappings.filter((m) => !isMoodChild(m.loxone_id)));

  return (
    <div className="space-y-4">
      <LoxoneExportPanel />

      <div className="bg-gray-800 rounded-xl p-6 space-y-4">
        <div className="flex justify-between items-center">
          <h2 className="text-lg font-semibold text-white">Loxone Mappings</h2>
          <div className="flex gap-2">
            <Tooltip content="Backup exportieren" position="bottom">
              <button
                type="button"
                aria-label="Mappings exportieren"
                onClick={handleExport}
                disabled={isExporting}
                className="flex items-center gap-2 px-3 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Download size={18} />
                <span className="hidden sm:inline">Export</span>
              </button>
            </Tooltip>
            <Tooltip content="Backup importieren" position="bottom">
              <label className="flex items-center gap-2 px-3 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600 transition-colors cursor-pointer">
                <Upload size={18} />
                <span className="hidden sm:inline">Import</span>
                <input type="file" accept=".json" onChange={handleFileSelect} className="hidden" />
              </label>
            </Tooltip>
            <button
              type="button"
              onClick={() => setWizardOpen(true)}
              className="flex items-center gap-2 px-4 py-2 bg-hue-orange text-gray-900 rounded-lg hover:bg-orange-400 transition-colors"
            >
              <Plus size={18} />
              Neu
            </button>
          </div>
        </div>

        {error && (
          <div className="bg-red-900/30 border border-red-500/50 rounded-lg p-4 flex items-start gap-3">
            <AlertCircle className="text-red-400 flex-shrink-0 mt-0.5" size={20} />
            <div>
              <p className="text-red-300">{error}</p>
              <button type="button" onClick={() => setError(null)} className="text-red-400 text-sm hover:text-red-300 mt-1">
                Schliessen
              </button>
            </div>
          </div>
        )}

        {importFile && (
          <ImportModal backup={importFile} onClose={() => setImportFile(null)} onImported={handleImported} />
        )}

        {wizardOpen && (
          <MappingWizard env={env} onClose={() => setWizardOpen(false)} onCreated={handleCreated} />
        )}

        <div className="space-y-2">
          {topLevel.map((mapping) => (
            <MappingRow
              key={mapping.id}
              mapping={mapping}
              moods={getMoodChildren(mappings, mapping.loxone_id)}
              env={env}
              onNavigateToHueElement={onNavigateToHueElement}
              onUpdate={handleUpdate}
              onDelete={handleDelete}
              onSwapMoods={handleSwapMoods}
              onDeleteMoodScene={handleDeleteMoodScene}
              onAddMoodScene={handleAddMoodScene}
            />
          ))}

          {mappings.length === 0 && !wizardOpen && (
            <div className="text-center py-8">
              <Link2 className="mx-auto text-gray-600 mb-4" size={48} />
              <p className="text-gray-400">Keine Mappings konfiguriert</p>
              <p className="text-xs text-gray-500 mt-2">Mappings verbinden Loxone IDs mit HUE Geräten</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
