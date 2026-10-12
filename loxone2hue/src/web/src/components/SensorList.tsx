import { Sensor } from '../types';
import type { Mapping } from '../services/api';
import {
  Activity,
  Thermometer,
  Sun,
  MousePointerClick,
  DoorOpen,
  RotateCw,
  Battery,
  BatteryLow,
  BatteryWarning,
  Link2,
  RefreshCw,
} from 'lucide-react';

interface SensorListProps {
  sensors: Sensor[];
  loading: boolean;
  mappings?: Mapping[];
  udpSendAll?: boolean;
}

const sensorTypeLabels: Record<string, string> = {
  motion: 'Bewegung',
  temperature: 'Temperatur',
  light_level: 'Helligkeit',
  button: 'Taster',
  contact: 'Kontakt',
  relative_rotary: 'Drehregler',
  device_power: 'Batterie',
};

const sensorTypeIcons: Record<string, typeof Activity> = {
  motion: Activity,
  temperature: Thermometer,
  light_level: Sun,
  button: MousePointerClick,
  contact: DoorOpen,
  relative_rotary: RotateCw,
  device_power: Battery,
};

function formatDateTime(isoString: string | undefined): string | null {
  if (!isoString) return null;
  const d = new Date(isoString);
  if (isNaN(d.getTime())) return null;
  const pad = (n: number) => String(n).padStart(2, '0');
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  const date = `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
  return `${date} ${time}`;
}

function formatSensorValue(sensor: Sensor): string {
  const s = sensor.state;
  switch (sensor.type) {
    case 'motion':
      return s.motion ? 'Bewegung erkannt' : 'Keine Bewegung';
    case 'temperature':
      return s.temperature != null ? `${s.temperature.toFixed(1)} \u00B0C` : '--';
    case 'light_level': {
      if (s.light_level == null) return '--';
      // HUE light level is in 10000 * log10(lux) + 1
      const lux = Math.pow(10, (s.light_level - 1) / 10000);
      return `${Math.round(lux)} Lux`;
    }
    case 'button':
      return s.button_event || '--';
    case 'contact':
      if (!s.contact_state) return '--';
      return s.contact_state === 'contact' ? 'Geschlossen' : 'Offen';
    case 'relative_rotary':
      if (s.rotary_action) return `${s.rotary_action} (${s.rotary_steps || 0} Schritte)`;
      return '--';
    case 'device_power':
      if (s.battery_level == null) return '--';
      return `${s.battery_level}%`;
    default:
      return '--';
  }
}

function SensorStateIndicator({ sensor }: { sensor: Sensor }) {
  switch (sensor.type) {
    case 'motion':
      return (
        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${
          sensor.state.motion
            ? 'bg-red-500/20 text-red-400'
            : 'bg-gray-700 text-gray-400'
        }`}>
          <span className={`w-2 h-2 rounded-full ${sensor.state.motion ? 'bg-red-400 animate-pulse' : 'bg-gray-500'}`} />
          {sensor.state.motion ? 'Aktiv' : 'Inaktiv'}
        </span>
      );
    case 'contact':
      return (
        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${
          sensor.state.contact_state === 'no_contact'
            ? 'bg-yellow-500/20 text-yellow-400'
            : 'bg-green-500/20 text-green-400'
        }`}>
          {sensor.state.contact_state === 'no_contact' ? 'Offen' : 'Geschlossen'}
        </span>
      );
    case 'device_power': {
      const level = sensor.state.battery_level ?? 0;
      const state = sensor.state.battery_state;
      let color = 'text-green-400';
      let BatIcon = Battery;
      if (state === 'critical' || level < 10) {
        color = 'text-red-400';
        BatIcon = BatteryWarning;
      } else if (state === 'low' || level < 30) {
        color = 'text-yellow-400';
        BatIcon = BatteryLow;
      }
      return (
        <span className={`inline-flex items-center gap-1 ${color}`}>
          <BatIcon size={16} />
          <span className="text-xs font-medium">{level}%</span>
        </span>
      );
    }
    default:
      return null;
  }
}

export function SensorList({ sensors, loading, mappings = [], udpSendAll = false }: SensorListProps) {
  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <RefreshCw className="animate-spin text-hue-orange" size={32} />
      </div>
    );
  }

  if (sensors.length === 0) {
    return (
      <div className="text-center py-12">
        <Activity className="mx-auto text-gray-600 mb-4" size={48} />
        <p className="text-gray-400">Keine Sensoren gefunden</p>
        <p className="text-gray-500 text-sm mt-2">
          Die HUE Bridge hat keine Sensoren, Taster oder andere Eingabegeraete gemeldet.
        </p>
      </div>
    );
  }

  // Group sensors by owner device
  const byDevice = new Map<string, Sensor[]>();
  const noDevice: Sensor[] = [];

  sensors.forEach((sensor) => {
    const key = sensor.owner || sensor.device_id;
    if (key) {
      if (!byDevice.has(key)) byDevice.set(key, []);
      byDevice.get(key)!.push(sensor);
    } else {
      noDevice.push(sensor);
    }
  });

  // Check if a sensor has a mapping
  const hasMappingOrSendAll = (sensorId: string) => {
    if (udpSendAll) return true;
    return mappings.some((m) => m.hue_id === sensorId && m.enabled);
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white flex items-center gap-2">
          <Activity size={22} className="text-hue-orange" />
          Sensoren & Taster
          <span className="text-sm text-gray-400 font-normal">({sensors.length})</span>
        </h2>
      </div>

      {/* Info box about sensor UDP feedback */}
      <div className="bg-gray-800/50 border border-gray-700 rounded-lg p-4 text-sm text-gray-400">
        <p>
          Sensor-Events werden automatisch per UDP an Loxone gesendet, wenn ein Mapping vorhanden ist oder <strong>Send All</strong> aktiviert ist.
        </p>
        <p className="mt-1 font-mono text-xs text-gray-500">
          Format: &lt;loxone_id&gt;/&lt;property&gt;:&lt;value&gt; &mdash; z.B. <code className="text-hue-orange">flur_motion/motion:1</code>
        </p>
      </div>

      {Array.from(byDevice.entries()).map(([deviceName, deviceSensors]) => (
        <div key={deviceName} className="bg-gray-800 rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-700">
            <h3 className="text-white font-medium">{deviceName}</h3>
          </div>
          <div className="divide-y divide-gray-700/50">
            {deviceSensors.map((sensor) => {
              const Icon = sensorTypeIcons[sensor.type] || Activity;
              const mapped = hasMappingOrSendAll(sensor.id);
              return (
                <div key={sensor.id} className="px-4 py-3 flex items-center gap-3">
                  <div className={`p-2 rounded-lg ${sensor.state.enabled ? 'bg-hue-orange/10 text-hue-orange' : 'bg-gray-700 text-gray-500'}`}>
                    <Icon size={20} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-white text-sm font-medium truncate">
                        {sensorTypeLabels[sensor.type] || sensor.type}
                      </span>
                      {mapped && (
                        <span title="Mapping vorhanden"><Link2 size={12} className="text-hue-orange flex-shrink-0" /></span>
                      )}
                    </div>
                    <div className="text-xs text-gray-500 font-mono truncate">{sensor.id}</div>
                  </div>
                  <div className="text-right flex flex-col items-end gap-1">
                    <span className="text-sm text-gray-300">{formatSensorValue(sensor)}</span>
                    <SensorStateIndicator sensor={sensor} />
                    {sensor.state.last_updated && (
                      <span className="text-xs text-gray-500">{formatDateTime(sensor.state.last_updated)}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ))}

      {noDevice.length > 0 && (
        <div className="bg-gray-800 rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-gray-700">
            <h3 className="text-white font-medium">Weitere Sensoren</h3>
          </div>
          <div className="divide-y divide-gray-700/50">
            {noDevice.map((sensor) => {
              const Icon = sensorTypeIcons[sensor.type] || Activity;
              return (
                <div key={sensor.id} className="px-4 py-3 flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-gray-700 text-gray-400">
                    <Icon size={20} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <span className="text-white text-sm">{sensor.name}</span>
                    <div className="text-xs text-gray-500 font-mono">{sensor.id}</div>
                  </div>
                  <div className="text-right flex flex-col items-end gap-1">
                    <span className="text-sm text-gray-300">{formatSensorValue(sensor)}</span>
                    {sensor.state.last_updated && (
                      <span className="text-xs text-gray-500">{formatDateTime(sensor.state.last_updated)}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
