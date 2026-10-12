import { useState, useEffect, useRef, useCallback } from 'react';
import { Settings, Radio, Globe, Save, CheckCircle, AlertCircle, ScrollText, Search, RefreshCw, Plus, Trash2, Download, Upload, RotateCcw, Archive, Clock } from 'lucide-react';
import * as api from '../services/api';
import type { MiniserverConfig, LoxoneConfig, LogEntry, ConfigBackup, BackupSummary } from '../services/api';

const LEVEL_STYLES: Record<string, string> = {
  DEBUG: 'bg-gray-600 text-gray-200',
  INFO: 'bg-blue-900/80 text-blue-300',
  WARN: 'bg-yellow-900/80 text-yellow-300',
  ERROR: 'bg-red-900/80 text-red-300',
  FATAL: 'bg-red-800 text-red-200',
};

const LEVEL_FILTERS = ['Alle', 'DEBUG', 'INFO', 'WARN', 'ERROR', 'UDP'] as const;

function formatTimestamp(ts: string): string {
  const d = new Date(ts);
  const h = String(d.getHours()).padStart(2, '0');
  const m = String(d.getMinutes()).padStart(2, '0');
  const s = String(d.getSeconds()).padStart(2, '0');
  const ms = String(d.getMilliseconds()).padStart(3, '0');
  return `${h}:${m}:${s}.${ms}`;
}

function generateId(): string {
  return crypto.randomUUID?.() ?? Math.random().toString(36).slice(2) + Date.now().toString(36);
}

export function SettingsPanel() {
  const [loxoneConfig, setLoxoneConfig] = useState<LoxoneConfig>({
    miniservers: [],
  });
  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<'success' | 'error' | null>(null);
  const [loading, setLoading] = useState(true);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // Backup state
  const [backups, setBackups] = useState<BackupSummary[]>([]);
  const [backupsLoading, setBackupsLoading] = useState(false);
  const [backupRemark, setBackupRemark] = useState('');
  const [creatingBackup, setCreatingBackup] = useState(false);
  const [restoreConfirm, setRestoreConfirm] = useState<string | null>(null);
  const [backupDeleteConfirm, setBackupDeleteConfirm] = useState<string | null>(null);
  const [backupMessage, setBackupMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Server logs state
  const [logEntries, setLogEntries] = useState<LogEntry[]>([]);
  const [logLevel, setLogLevel] = useState<string>('');
  const [logSearch, setLogSearch] = useState('');
  const [logsLoading, setLogsLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval>>();

  useEffect(() => {
    loadConfig();
    loadBackups();
  }, []);

  const loadLogs = useCallback(async () => {
    try {
      setLogsLoading(true);
      const isUDP = logLevel === 'UDP';
      const result = await api.getLogs({
        level: (!isUDP && logLevel) || undefined,
        search: isUDP ? 'UDP feedback' : logSearch || undefined,
        limit: 200,
      });
      setLogEntries(result.entries || []);
    } catch (err) {
      console.error('Failed to load logs:', err);
    } finally {
      setLogsLoading(false);
    }
  }, [logLevel, logSearch]);

  useEffect(() => {
    loadLogs();
  }, [loadLogs]);

  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(loadLogs, 3000);
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [autoRefresh, loadLogs]);

  const loadConfig = async () => {
    try {
      const config = await api.getConfig();
      setLoxoneConfig({
        miniservers: config.loxone?.miniservers || [],
      });
    } catch (err) {
      console.error('Failed to load config:', err);
    } finally {
      setLoading(false);
    }
  };

  const loadBackups = async () => {
    try {
      setBackupsLoading(true);
      const result = await api.listBackups();
      setBackups(result.backups || []);
    } catch (err) {
      console.error('Failed to load backups:', err);
    } finally {
      setBackupsLoading(false);
    }
  };

  const handleCreateBackup = async () => {
    setCreatingBackup(true);
    setBackupMessage(null);
    try {
      await api.createBackup(backupRemark);
      setBackupRemark('');
      await loadBackups();
      setBackupMessage({ type: 'success', text: 'Backup erstellt' });
      setTimeout(() => setBackupMessage(null), 3000);
    } catch (err) {
      setBackupMessage({ type: 'error', text: 'Fehler beim Erstellen des Backups' });
    } finally {
      setCreatingBackup(false);
    }
  };

  const handleDeleteBackup = async (id: string) => {
    try {
      await api.deleteBackup(id);
      setBackupDeleteConfirm(null);
      await loadBackups();
    } catch (err) {
      setBackupMessage({ type: 'error', text: 'Fehler beim Löschen' });
    }
  };

  const handleRestoreBackup = async (id: string) => {
    setBackupMessage(null);
    try {
      await api.restoreBackup(id);
      setRestoreConfirm(null);
      await loadConfig();
      setBackupMessage({ type: 'success', text: 'Konfiguration wiederhergestellt' });
      setTimeout(() => setBackupMessage(null), 3000);
    } catch (err) {
      setBackupMessage({ type: 'error', text: 'Fehler beim Wiederherstellen' });
    }
  };

  const handleUploadBackup = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setBackupMessage(null);
    try {
      const text = await file.text();
      const backup = JSON.parse(text) as ConfigBackup;
      if (!backup.config) {
        setBackupMessage({ type: 'error', text: 'Ungültiges Backup-Format' });
        return;
      }
      await api.uploadBackup(backup);
      await loadBackups();
      setBackupMessage({ type: 'success', text: 'Backup importiert' });
      setTimeout(() => setBackupMessage(null), 3000);
    } catch (err) {
      setBackupMessage({ type: 'error', text: 'Fehler beim Import der Backup-Datei' });
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setSaveResult(null);
    try {
      await api.updateConfig({ loxone: loxoneConfig });
      setSaveResult('success');
      setTimeout(() => setSaveResult(null), 3000);
    } catch (err) {
      setSaveResult('error');
      console.error('Failed to save config:', err);
    } finally {
      setSaving(false);
    }
  };

  const addMiniserver = () => {
    setLoxoneConfig((prev) => ({
      ...prev,
      miniservers: [
        ...prev.miniservers,
        {
          id: generateId(),
          name: '',
          ip: '',
          port: 7777,
          udp_enabled: false,
          http_enabled: false,
          http_url: '',
          http_user: '',
          http_password: '',
          send_all: false,
        },
      ],
    }));
  };

  const removeMiniserver = (id: string) => {
    setLoxoneConfig((prev) => ({
      ...prev,
      miniservers: prev.miniservers.filter((ms) => ms.id !== id),
    }));
    setDeleteConfirm(null);
  };

  const updateMiniserver = (id: string, update: Partial<MiniserverConfig>) => {
    setLoxoneConfig((prev) => ({
      ...prev,
      miniservers: prev.miniservers.map((ms) =>
        ms.id === id ? { ...ms, ...update } : ms
      ),
    }));
  };

  if (loading) {
    return <div className="text-gray-400 text-center py-12">Lade Konfiguration...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3 mb-6">
        <Settings size={24} className="text-hue-orange" />
        <h2 className="text-xl font-semibold text-white">Einstellungen</h2>
      </div>

      {/* Loxone Miniserver Section */}
      <div className="bg-gray-800 rounded-xl p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium text-white">Loxone Miniserver</h3>
          <button
            onClick={addMiniserver}
            className="flex items-center gap-1.5 bg-hue-orange hover:bg-hue-orange/90 text-gray-900 font-medium px-3 py-1.5 rounded-lg text-sm transition-colors"
          >
            <Plus size={16} />
            Hinzufügen
          </button>
        </div>

        <div className="bg-gray-700/50 rounded-lg p-3">
          <p className="text-xs text-gray-400 font-mono">
            Format: &lt;loxone_id&gt;/&lt;eigenschaft&gt;:&lt;wert&gt;
          </p>
          <p className="text-xs text-gray-500 mt-1">
            Beispiel: buero_stefan/on:1 &bull; buero_stefan/bri:80 &bull; buero_stefan/ct:250
          </p>
        </div>

        {loxoneConfig.miniservers.length === 0 && (
          <div className="text-center py-8 text-gray-500 text-sm">
            Kein Miniserver konfiguriert. Klicke auf "Hinzufügen" um einen Miniserver zu erfassen.
          </div>
        )}

        {loxoneConfig.miniservers.map((ms) => (
          <div
            key={ms.id}
            className="bg-gray-700/30 border border-gray-700 rounded-xl p-4 space-y-3"
          >
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1 grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Name</label>
                  <input
                    type="text"
                    value={ms.name}
                    onChange={(e) => updateMiniserver(ms.id, { name: e.target.value })}
                    placeholder="z.B. Haupthaus"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">IP-Adresse</label>
                  <input
                    type="text"
                    value={ms.ip}
                    onChange={(e) => updateMiniserver(ms.id, { ip: e.target.value })}
                    placeholder="192.168.1.10"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">UDP Port</label>
                  <input
                    type="number"
                    value={ms.port}
                    onChange={(e) => updateMiniserver(ms.id, { port: parseInt(e.target.value) || 7777 })}
                    placeholder="7777"
                    min={1}
                    max={65535}
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                </div>
              </div>

              {deleteConfirm === ms.id ? (
                <div className="flex items-center gap-1.5 shrink-0 mt-6">
                  <button
                    onClick={() => removeMiniserver(ms.id)}
                    className="px-2 py-1 bg-red-600 hover:bg-red-500 text-white text-xs rounded transition-colors"
                  >
                    Löschen
                  </button>
                  <button
                    onClick={() => setDeleteConfirm(null)}
                    className="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
                  >
                    Abbrechen
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => setDeleteConfirm(ms.id)}
                  title="Miniserver entfernen"
                  className="mt-6 p-1.5 text-gray-500 hover:text-red-400 hover:bg-gray-700 rounded transition-colors shrink-0"
                >
                  <Trash2 size={16} />
                </button>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-4">
              <label className="flex items-center gap-3 cursor-pointer">
                <div className="relative">
                  <input
                    type="checkbox"
                    checked={ms.udp_enabled}
                    onChange={(e) => updateMiniserver(ms.id, { udp_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-10 h-6 bg-gray-600 rounded-full peer-checked:bg-hue-orange transition-colors"></div>
                  <div className="absolute left-1 top-1 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-4"></div>
                </div>
                <div className="flex items-center gap-1.5">
                  <Radio size={14} className="text-hue-orange" />
                  <span className="text-gray-300 text-sm">UDP Feedback</span>
                </div>
              </label>

              <label className="flex items-center gap-3 cursor-pointer">
                <div className="relative">
                  <input
                    type="checkbox"
                    checked={ms.http_enabled}
                    onChange={(e) => updateMiniserver(ms.id, { http_enabled: e.target.checked })}
                    className="sr-only peer"
                  />
                  <div className="w-10 h-6 bg-gray-600 rounded-full peer-checked:bg-hue-orange transition-colors"></div>
                  <div className="absolute left-1 top-1 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-4"></div>
                </div>
                <div className="flex items-center gap-1.5">
                  <Globe size={14} className="text-hue-orange" />
                  <span className="text-gray-300 text-sm">HTTP Feedback</span>
                </div>
              </label>

              {(ms.udp_enabled || ms.http_enabled) && (
                <label className="flex items-center gap-3 cursor-pointer">
                  <div className="relative">
                    <input
                      type="checkbox"
                      checked={ms.send_all}
                      onChange={(e) => updateMiniserver(ms.id, { send_all: e.target.checked })}
                      className="sr-only peer"
                    />
                    <div className="w-10 h-6 bg-gray-600 rounded-full peer-checked:bg-hue-orange transition-colors"></div>
                    <div className="absolute left-1 top-1 w-4 h-4 bg-white rounded-full transition-transform peer-checked:translate-x-4"></div>
                  </div>
                  <div>
                    <span className="text-gray-300 text-sm">Alle Geräte senden</span>
                    <p className="text-xs text-gray-500">Auch ohne Mapping</p>
                  </div>
                </label>
              )}
            </div>

            {ms.http_enabled && (
              <div className="space-y-3">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Ziel-URL (Virtueller HTTP Eingang)</label>
                  <input
                    type="text"
                    value={ms.http_url}
                    onChange={(e) => updateMiniserver(ms.id, { http_url: e.target.value })}
                    placeholder="https://192.168.1.7:443/request.php"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    URL aus der Loxone-Config für virtuelle HTTP-Eingänge. Ohne Angabe wird http://&lt;IP&gt; verwendet.
                  </p>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">HTTP Benutzer</label>
                  <input
                    type="text"
                    value={ms.http_user}
                    onChange={(e) => updateMiniserver(ms.id, { http_user: e.target.value })}
                    placeholder="admin"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-1">HTTP Passwort</label>
                  <input
                    type="password"
                    value={ms.http_password}
                    onChange={(e) => updateMiniserver(ms.id, { http_password: e.target.value })}
                    placeholder="********"
                    className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
                  />
                </div>
              </div>
              </div>
            )}

            {(ms.udp_enabled || (ms.http_enabled && !ms.http_url)) && !ms.ip && (
              <div className="flex items-center gap-2 text-yellow-400 text-sm">
                <AlertCircle size={16} />
                <span>Bitte eine IP-Adresse eingeben</span>
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Save Button */}
      <div className="flex items-center gap-4">
        <button
          onClick={handleSave}
          disabled={saving}
          className="flex items-center gap-2 bg-hue-orange hover:bg-hue-orange/90 text-gray-900 font-medium px-6 py-2.5 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <Save size={18} />
          {saving ? 'Speichern...' : 'Einstellungen speichern'}
        </button>

        {saveResult === 'success' && (
          <div className="flex items-center gap-2 text-green-400 text-sm">
            <CheckCircle size={16} />
            <span>Gespeichert</span>
          </div>
        )}
        {saveResult === 'error' && (
          <div className="flex items-center gap-2 text-red-400 text-sm">
            <AlertCircle size={16} />
            <span>Fehler beim Speichern</span>
          </div>
        )}
      </div>

      {/* Backup & Restore */}
      <div className="bg-gray-800 rounded-xl p-6 space-y-4">
        <div className="flex items-center gap-2">
          <Archive size={20} className="text-hue-orange" />
          <h3 className="text-lg font-medium text-white">Backup & Restore</h3>
        </div>

        <p className="text-sm text-gray-400">
          Erstelle Sicherungen der gesamten Konfiguration (Server, HUE-Bridge, Miniserver, Mappings) und stelle sie bei Bedarf wieder her.
        </p>

        {/* Create Backup */}
        <div className="flex flex-wrap items-end gap-3">
          <div className="flex-1 min-w-[200px]">
            <label className="block text-sm text-gray-400 mb-1">Bemerkung</label>
            <input
              type="text"
              value={backupRemark}
              onChange={(e) => setBackupRemark(e.target.value)}
              placeholder="z.B. Vor Update auf v2.0"
              className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-sm"
            />
          </div>
          <button
            onClick={handleCreateBackup}
            disabled={creatingBackup}
            className="flex items-center gap-1.5 bg-hue-orange hover:bg-hue-orange/90 text-gray-900 font-medium px-4 py-2 rounded-lg text-sm transition-colors disabled:opacity-50"
          >
            <Archive size={16} />
            {creatingBackup ? 'Erstelle...' : 'Backup erstellen'}
          </button>
          <button
            onClick={() => fileInputRef.current?.click()}
            className="flex items-center gap-1.5 bg-gray-700 hover:bg-gray-600 text-gray-300 font-medium px-4 py-2 rounded-lg text-sm transition-colors"
          >
            <Upload size={16} />
            Datei importieren
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".json"
            onChange={handleUploadBackup}
            className="hidden"
          />
        </div>

        {backupMessage && (
          <div className={`flex items-center gap-2 text-sm ${backupMessage.type === 'success' ? 'text-green-400' : 'text-red-400'}`}>
            {backupMessage.type === 'success' ? <CheckCircle size={16} /> : <AlertCircle size={16} />}
            <span>{backupMessage.text}</span>
          </div>
        )}

        {/* Backup List */}
        {backupsLoading ? (
          <div className="text-gray-500 text-center py-4 text-sm">Lade Backups...</div>
        ) : backups.length === 0 ? (
          <div className="text-gray-500 text-center py-4 text-sm">Keine Backups vorhanden</div>
        ) : (
          <div className="space-y-2">
            {backups.map((backup) => (
              <div
                key={backup.id}
                className="bg-gray-700/30 border border-gray-700 rounded-lg px-4 py-3 flex flex-wrap items-center gap-3"
              >
                <div className="flex-1 min-w-[200px]">
                  <div className="flex items-center gap-2">
                    <Clock size={14} className="text-gray-500 shrink-0" />
                    <span className="text-sm text-gray-300">
                      {new Date(backup.created_at).toLocaleString('de-CH')}
                    </span>
                    <span className="text-xs bg-gray-600 text-gray-300 px-1.5 py-0.5 rounded">
                      v{backup.version}
                    </span>
                  </div>
                  {backup.remark && (
                    <p className="text-sm text-gray-400 mt-0.5 ml-5">{backup.remark}</p>
                  )}
                  <p className="text-xs text-gray-500 mt-0.5 ml-5">
                    {backup.mapping_count} Mappings &bull; {backup.miniserver_count} Miniserver
                  </p>
                </div>

                <div className="flex items-center gap-1.5 shrink-0">
                  {restoreConfirm === backup.id ? (
                    <>
                      <span className="text-xs text-yellow-400 mr-1">Wiederherstellen?</span>
                      <button
                        onClick={() => handleRestoreBackup(backup.id)}
                        className="px-2 py-1 bg-hue-orange hover:bg-hue-orange/90 text-gray-900 text-xs font-medium rounded transition-colors"
                      >
                        Ja
                      </button>
                      <button
                        onClick={() => setRestoreConfirm(null)}
                        className="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
                      >
                        Nein
                      </button>
                    </>
                  ) : (
                    <button
                      onClick={() => setRestoreConfirm(backup.id)}
                      title="Wiederherstellen"
                      className="p-1.5 text-gray-400 hover:text-hue-orange hover:bg-gray-700 rounded transition-colors"
                    >
                      <RotateCcw size={16} />
                    </button>
                  )}

                  <a
                    href={api.getBackupDownloadUrl(backup.id)}
                    download
                    title="Herunterladen"
                    className="p-1.5 text-gray-400 hover:text-blue-400 hover:bg-gray-700 rounded transition-colors"
                  >
                    <Download size={16} />
                  </a>

                  {backupDeleteConfirm === backup.id ? (
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleDeleteBackup(backup.id)}
                        className="px-2 py-1 bg-red-600 hover:bg-red-500 text-white text-xs rounded transition-colors"
                      >
                        Löschen
                      </button>
                      <button
                        onClick={() => setBackupDeleteConfirm(null)}
                        className="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
                      >
                        Abbrechen
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setBackupDeleteConfirm(backup.id)}
                      title="Löschen"
                      className="p-1.5 text-gray-500 hover:text-red-400 hover:bg-gray-700 rounded transition-colors"
                    >
                      <Trash2 size={16} />
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Server Logs */}
      <div className="bg-gray-800 rounded-xl p-6 space-y-4">
        <div className="flex items-center gap-2">
          <ScrollText size={20} className="text-hue-orange" />
          <h3 className="text-lg font-medium text-white">Server Logs</h3>
        </div>

        {/* Filter bar */}
        <div className="flex flex-wrap items-center gap-2">
          {LEVEL_FILTERS.map((lvl) => (
            <button
              key={lvl}
              onClick={() => setLogLevel(lvl === 'Alle' ? '' : lvl)}
              className={`px-3 py-1 rounded-md text-sm font-medium transition-colors ${
                (lvl === 'Alle' && logLevel === '') || logLevel === lvl
                  ? 'bg-hue-orange text-gray-900'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              }`}
            >
              {lvl}
            </button>
          ))}

          <div className="relative flex-1 min-w-[200px]">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
            <input
              type="text"
              value={logSearch}
              onChange={(e) => setLogSearch(e.target.value)}
              placeholder="Suche (z.B. 'EventStream')..."
              className="w-full bg-gray-700 text-white rounded-lg pl-9 pr-4 py-1.5 text-sm border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500"
            />
          </div>

          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            title={autoRefresh ? 'Auto-Refresh aktiv (3s)' : 'Auto-Refresh aus'}
            className={`p-1.5 rounded-md transition-colors ${
              autoRefresh
                ? 'bg-hue-orange/20 text-hue-orange'
                : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
            }`}
          >
            <RefreshCw size={16} className={autoRefresh ? 'animate-spin [animation-duration:3s]' : ''} />
          </button>
        </div>

        {/* Log entries */}
        <div
          ref={logContainerRef}
          className="bg-gray-900 rounded-lg border border-gray-700 overflow-auto max-h-[420px] font-mono text-[13px] leading-relaxed"
        >
          {logsLoading && logEntries.length === 0 ? (
            <div className="text-gray-500 text-center py-8 text-sm">Lade Logs...</div>
          ) : logEntries.length === 0 ? (
            <div className="text-gray-500 text-center py-8 text-sm">Keine Log-Einträge gefunden</div>
          ) : (
            <table className="w-full">
              <tbody>
                {logEntries.map((entry, i) => (
                  <tr
                    key={i}
                    className="border-b border-gray-800/50 last:border-0 hover:bg-gray-800/40"
                  >
                    <td className="px-3 py-1 text-gray-500 whitespace-nowrap align-top">
                      {formatTimestamp(entry.timestamp)}
                    </td>
                    <td className="px-1 py-1 whitespace-nowrap align-top">
                      <span className="inline-block px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider bg-gray-700 text-gray-400">
                        {entry.source}
                      </span>
                    </td>
                    <td className="px-1 py-1 whitespace-nowrap align-top">
                      <span
                        className={`inline-block px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wider ${
                          LEVEL_STYLES[entry.level] || 'bg-gray-600 text-gray-300'
                        }`}
                      >
                        {entry.level}
                      </span>
                    </td>
                    <td className="px-3 py-1 text-gray-300 break-all">
                      {entry.message}
                      {entry.fields && Object.keys(entry.fields).length > 0 && (
                        <span className="text-gray-500 ml-2">
                          {Object.entries(entry.fields)
                            .map(([k, v]) => `${k}=${typeof v === 'object' ? JSON.stringify(v) : v}`)
                            .join(' ')}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
