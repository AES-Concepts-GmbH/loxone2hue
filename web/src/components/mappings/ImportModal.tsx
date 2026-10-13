import { useState } from 'react';
import { X } from 'lucide-react';
import * as api from '../../services/api';

interface ImportModalProps {
  backup: api.MappingsBackup;
  onClose: () => void;
  /** Called after a successful import so the caller can reload its mappings. */
  onImported: () => Promise<void> | void;
}

export function ImportModal({ backup, onClose, onImported }: ImportModalProps) {
  const [mode, setMode] = useState<'replace' | 'merge'>('merge');
  const [result, setResult] = useState<api.ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isImporting, setIsImporting] = useState(false);

  const handleImport = async () => {
    setIsImporting(true);
    setError(null);
    try {
      const res = await api.importMappings(backup, mode);
      setResult(res);
      await onImported();
    } catch (err) {
      console.error('Failed to import mappings:', err);
      setError(`Import fehlgeschlagen: ${err instanceof Error ? err.message : 'unbekannter Fehler'}`);
    } finally {
      setIsImporting(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-xl p-6 max-w-md w-full space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-white">Mappings importieren</h3>
          <button type="button" aria-label="Schliessen" onClick={onClose} className="text-gray-400 hover:text-white">
            <X size={20} />
          </button>
        </div>

        {!result ? (
          <>
            <div className="bg-gray-700 rounded-lg p-4">
              <p className="text-sm text-gray-300">
                <strong>Version:</strong> {backup.version}
              </p>
              <p className="text-sm text-gray-300">
                <strong>Erstellt:</strong> {backup.created_at ? new Date(backup.created_at).toLocaleString('de-CH') : '-'}
              </p>
              <p className="text-sm text-gray-300">
                <strong>Mappings:</strong> {backup.mappings.length}
              </p>
            </div>

            <div className="space-y-2">
              <p className="text-sm text-gray-400">Import-Modus:</p>
              <label className="flex items-start gap-3 p-3 bg-gray-700 rounded-lg cursor-pointer hover:bg-gray-600">
                <input type="radio" name="importMode" value="merge" checked={mode === 'merge'} onChange={() => setMode('merge')} className="mt-1" />
                <div>
                  <p className="text-white font-medium">Zusammenführen</p>
                  <p className="text-xs text-gray-400">
                    Bestehende Mappings bleiben erhalten. Nur neue werden hinzugefügt, vorhandene (gleiche Loxone ID) werden aktualisiert.
                  </p>
                </div>
              </label>
              <label className="flex items-start gap-3 p-3 bg-gray-700 rounded-lg cursor-pointer hover:bg-gray-600">
                <input type="radio" name="importMode" value="replace" checked={mode === 'replace'} onChange={() => setMode('replace')} className="mt-1" />
                <div>
                  <p className="text-white font-medium">Ersetzen</p>
                  <p className="text-xs text-gray-400">
                    Alle bestehenden Mappings werden gelöscht und durch die importierten ersetzt.
                  </p>
                </div>
              </label>
            </div>

            {error && (
              <div className="bg-red-900/30 border border-red-500/50 rounded-lg p-3">
                <p className="text-red-300 text-sm">{error}</p>
              </div>
            )}

            <div className="flex gap-2">
              <button
                type="button"
                onClick={handleImport}
                disabled={isImporting}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-hue-orange text-gray-900 rounded-lg hover:bg-orange-400 disabled:opacity-50"
              >
                {isImporting ? 'Importiere...' : 'Importieren'}
              </button>
              <button type="button" onClick={onClose} className="px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600">
                Abbrechen
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="bg-green-900/30 border border-green-500/50 rounded-lg p-4">
              <p className="text-green-300 font-medium mb-2">Import erfolgreich!</p>
              <div className="space-y-1 text-sm text-gray-300">
                <p><strong>Importiert:</strong> {result.imported} neue Mappings</p>
                <p><strong>Aktualisiert:</strong> {result.updated} bestehende Mappings</p>
                {result.skipped > 0 && <p><strong>Übersprungen:</strong> {result.skipped}</p>}
                <p className="text-gray-400 mt-2">Gesamt: {result.total} Mappings verarbeitet</p>
              </div>
            </div>

            <button type="button" onClick={onClose} className="w-full px-4 py-2 bg-hue-orange text-gray-900 rounded-lg hover:bg-orange-400">
              Schliessen
            </button>
          </>
        )}
      </div>
    </div>
  );
}
