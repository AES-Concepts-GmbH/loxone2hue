import { useState, useEffect } from 'react';
import { Radio, Send, Check, AlertCircle, Loader2 } from 'lucide-react';
import { testUDP, getConfig } from '../services/api';
import type { MiniserverConfig } from '../services/api';
import type { UdpProperty } from '../utils/udp';

function validateValue(property: string, value: string): string | null {
  if (value.trim() === '') return 'Wert eingeben';
  switch (property) {
    case 'on':
      if (value !== '0' && value !== '1') return '0 oder 1';
      break;
    case 'bri': {
      const n = Number(value);
      if (!Number.isInteger(n) || n < 0 || n > 100) return '0–100';
      break;
    }
    case 'ct': {
      const n = Number(value);
      if (!Number.isInteger(n) || n < 153 || n > 500) return '153–500';
      break;
    }
    case 'color_x':
    case 'color_y': {
      const n = Number(value);
      if (isNaN(n) || n < 0 || n > 1) return '0–1';
      break;
    }
  }
  return null;
}

type RowStatus = 'idle' | 'sending' | 'sent' | 'error';

interface RowState {
  value: string;
  status: RowStatus;
  message: string;
}

function TestCell({ loxoneId, prop, miniserverId }: { loxoneId: string; prop: UdpProperty; miniserverId: string }) {
  const [state, setState] = useState<RowState>({ value: '', status: 'idle', message: '' });

  const handleSend = async () => {
    const err = validateValue(prop.property, state.value);
    if (err) {
      setState(s => ({ ...s, status: 'error', message: err }));
      return;
    }

    setState(s => ({ ...s, status: 'sending', message: '' }));
    try {
      const result = await testUDP(loxoneId, prop.property, state.value, miniserverId);
      setState(s => ({ ...s, status: 'sent', message: result.message }));
      setTimeout(() => setState(s => s.status === 'sent' ? { ...s, status: 'idle', message: '' } : s), 3000);
    } catch (e) {
      setState(s => ({ ...s, status: 'error', message: e instanceof Error ? e.message : 'Fehler' }));
    }
  };

  return (
    <td className="px-3 py-1.5">
      <div className="flex items-center gap-1">
        <input
          type="text"
          value={state.value}
          onChange={e => setState({ value: e.target.value, status: 'idle', message: '' })}
          onKeyDown={e => e.key === 'Enter' && handleSend()}
          placeholder={prop.range}
          className="w-16 px-1.5 py-0.5 bg-gray-800 border border-gray-700 rounded text-xs text-white placeholder-gray-600 focus:border-blue-500 focus:outline-none"
        />
        <button
          onClick={handleSend}
          disabled={state.status === 'sending'}
          title="Test-UDP senden"
          className="p-0.5 rounded hover:bg-gray-700 text-gray-400 hover:text-blue-400 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {state.status === 'sending' ? (
            <Loader2 size={12} className="animate-spin" />
          ) : state.status === 'sent' ? (
            <Check size={12} className="text-green-400" />
          ) : state.status === 'error' ? (
            <AlertCircle size={12} className="text-red-400" />
          ) : (
            <Send size={12} />
          )}
        </button>
        {state.status === 'sent' && (
          <span className="text-[10px] text-green-400 truncate max-w-[120px]">{state.message}</span>
        )}
        {state.status === 'error' && (
          <span className="text-[10px] text-red-400 truncate max-w-[120px]">{state.message}</span>
        )}
      </div>
    </td>
  );
}

interface UdpInfoSectionProps {
  loxoneId: string;
  properties: UdpProperty[];
  miniserverId?: string;
}

export function UdpInfoSection({ loxoneId, properties, miniserverId }: UdpInfoSectionProps) {
  const [miniservers, setMiniservers] = useState<MiniserverConfig[]>([]);
  const [selectedMs, setSelectedMs] = useState<string>(miniserverId || '');

  useEffect(() => {
    getConfig().then((cfg) => {
      const ms = (cfg.loxone?.miniservers || []).filter((m) => m.udp_enabled);
      setMiniservers(ms);
      if (ms.length > 0) {
        // Keep an explicit choice, otherwise fall back to the mapping's
        // miniserver or the first one with UDP enabled.
        setSelectedMs((current) => current || miniserverId || ms[0].id);
      }
    }).catch(() => {});
  }, [miniserverId]);

  const activeMs = miniservers.find((m) => m.id === selectedMs);
  const hasTest = !!activeMs;

  return (
    <div className="border-t border-gray-700 pt-3 mt-3">
      <div className="flex items-center gap-2 mb-2">
        <Radio size={14} className="text-green-400" />
        <span className="text-sm font-medium text-green-400">UDP Status-Feedback</span>
      </div>
      <div className="text-xs text-gray-400 mb-2">
        Loxone ID: <span className="font-mono text-white">{loxoneId}</span>
      </div>

      {miniservers.length > 1 && (
        <div className="mb-2">
          <label className="text-xs text-gray-400 mr-2">Miniserver:</label>
          <select
            value={selectedMs}
            onChange={(e) => setSelectedMs(e.target.value)}
            className="bg-gray-800 text-white text-xs rounded px-2 py-1 border border-gray-700 focus:border-blue-500 focus:outline-none"
          >
            {miniservers.map((ms) => (
              <option key={ms.id} value={ms.id}>
                {ms.name || ms.ip} ({ms.ip}:{ms.port})
              </option>
            ))}
          </select>
        </div>
      )}

      {miniservers.length === 1 && (
        <div className="text-xs text-gray-500 mb-2">
          Miniserver: <span className="text-gray-300">{miniservers[0].name || miniservers[0].ip}</span> ({miniservers[0].ip}:{miniservers[0].port})
        </div>
      )}

      <div className="bg-gray-900 rounded-lg overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-gray-500 border-b border-gray-800">
              <th className="text-left px-3 py-1.5 font-medium">Check-Pattern</th>
              <th className="text-left px-3 py-1.5 font-medium">Eigenschaft</th>
              <th className="text-left px-3 py-1.5 font-medium">Werte</th>
              {hasTest && <th className="text-left px-3 py-1.5 font-medium">Test</th>}
            </tr>
          </thead>
          <tbody>
            {properties.map((prop) => (
              <tr key={prop.pattern} className="border-b border-gray-800/50 last:border-0">
                <td className="px-3 py-1.5 font-mono text-green-400">{prop.pattern}</td>
                <td className="px-3 py-1.5 text-gray-300">{prop.label}</td>
                <td className="px-3 py-1.5 text-gray-500">{prop.range}</td>
                {hasTest && <TestCell loxoneId={loxoneId} prop={prop} miniserverId={selectedMs} />}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-2 bg-blue-900/20 border border-blue-500/30 rounded-lg px-3 py-2">
        <p className="text-xs text-blue-300">
          <strong>Loxone Config:</strong> Erstelle einen <em>Virtuellen UDP Eingang</em> und
          trage das Check-Pattern als Befehlserkennung ein.
          Die Vorlage kann unter Einstellungen &gt; Loxone Config Export heruntergeladen werden.
        </p>
      </div>
    </div>
  );
}
