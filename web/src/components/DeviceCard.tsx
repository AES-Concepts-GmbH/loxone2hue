import { useEffect, useState } from 'react';
import { Lightbulb, Power, Radio } from 'lucide-react';
import { Light } from '../types';
import type { Mapping } from '../services/api';
import { UdpInfoSection } from './UdpInfoSection';
import { getUdpProperties, sanitizeName } from '../utils/udp';

// Keys that move a range input; the value is sent once the key is released.
const SLIDER_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown']);

interface DeviceCardProps {
  device: Light;
  onToggle: (id: string, on: boolean) => void;
  onBrightness: (id: string, brightness: number) => void;
  mapping?: Mapping;
  udpSendAll?: boolean;
}

export function DeviceCard({ device, onToggle, onBrightness, mapping, udpSendAll = false }: DeviceCardProps) {
  const [localBrightness, setLocalBrightness] = useState(device.state.brightness);
  const [showUdp, setShowUdp] = useState(false);

  // The slider holds its own value while dragging, but has to follow changes
  // made elsewhere (Hue app, Loxone, auto-refresh). Without this it kept the
  // value from mount and the next commit sent that stale value back.
  useEffect(() => {
    setLocalBrightness(device.state.brightness);
  }, [device.state.brightness]);

  const handleBrightnessChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = parseFloat(e.target.value);
    setLocalBrightness(value);
  };

  const handleBrightnessCommit = () => {
    onBrightness(device.id, localBrightness);
  };

  const getColorStyle = () => {
    if (!device.state.on) {
      return { backgroundColor: '#374151' };
    }

    if (device.state.color?.hex_rgb) {
      return { backgroundColor: device.state.color.hex_rgb };
    }

    // Default warm white
    const warmth = device.state.color_temp
      ? Math.min(1, (device.state.color_temp - 153) / 347)
      : 0.5;
    const r = 255;
    const g = Math.round(180 + (75 * (1 - warmth)));
    const b = Math.round(100 + (155 * (1 - warmth)));
    return { backgroundColor: `rgb(${r}, ${g}, ${b})` };
  };

  return (
    <div className={`
      bg-gray-800 rounded-xl p-4 transition-all duration-200
      ${device.state.on ? 'ring-2 ring-hue-orange' : ''}
      ${!device.state.reachable ? 'opacity-50' : ''}
    `}>
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <div
            className="w-10 h-10 rounded-full flex items-center justify-center transition-colors"
            style={getColorStyle()}
          >
            <Lightbulb
              size={20}
              className={device.state.on ? 'text-gray-900' : 'text-gray-400'}
            />
          </div>
          <div>
            <h3 className="font-medium text-white">{device.name}</h3>
            <p className="text-xs text-gray-400">{device.product_name || device.type}</p>
            <p className="text-xs text-gray-500 font-mono"><span className="text-gray-600">HUE ID:</span> {device.id}</p>
          </div>
        </div>

        <div className="flex items-center gap-1">
          {(mapping || udpSendAll) && (
            <button
              onClick={() => setShowUdp(!showUdp)}
              title="UDP Status-Feedback"
              className={`
                p-2 rounded-lg transition-colors
                ${showUdp
                  ? 'bg-green-700 text-white'
                  : 'bg-gray-700 text-gray-400 hover:bg-gray-600 hover:text-white'
                }
              `}
            >
              <Radio size={16} />
            </button>
          )}
          <button
            onClick={() => onToggle(device.id, !device.state.on)}
            className={`
              p-2 rounded-lg transition-colors
              ${device.state.on
                ? 'bg-hue-orange text-gray-900'
                : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
              }
            `}
            disabled={!device.state.reachable}
          >
            <Power size={20} />
          </button>
        </div>
      </div>

      {device.capabilities.supports_dimming && (
        <div className="mt-3">
          <div className="flex justify-between text-xs text-gray-400 mb-1">
            <span>Helligkeit</span>
            <span>{Math.round(localBrightness)}%</span>
          </div>
          <input
            type="range"
            min="1"
            max="100"
            value={localBrightness}
            onChange={handleBrightnessChange}
            onMouseUp={handleBrightnessCommit}
            onTouchEnd={handleBrightnessCommit}
            onKeyUp={(e) => { if (SLIDER_KEYS.has(e.key)) handleBrightnessCommit(); }}
            className="w-full"
            disabled={!device.state.reachable || !device.state.on}
          />
        </div>
      )}

      {!device.state.reachable && (
        <p className="text-xs text-red-400 mt-2">Nicht erreichbar</p>
      )}

      {showUdp && (mapping || udpSendAll) && (() => {
        const loxoneId = mapping?.loxone_id ?? sanitizeName(device.name);
        return (
          <UdpInfoSection
            loxoneId={loxoneId}
            properties={getUdpProperties(loxoneId, device.capabilities)}
            miniserverId={mapping?.miniserver_id}
          />
        );
      })()}
    </div>
  );
}
