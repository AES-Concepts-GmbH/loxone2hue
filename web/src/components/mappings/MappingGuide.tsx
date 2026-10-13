import { ExternalLink, Terminal } from 'lucide-react';
import type { Mapping } from '../../types';
import { CopyButton } from '../CopyButton';
import { Tooltip } from '../Tooltip';
import { getLoxoneGuide, getTestUrls } from '../../utils/loxoneGuide';
import type { MappingEnv } from './env';

interface MappingGuideProps {
  mapping: Mapping;
  env: MappingEnv;
}

/** The expandable "Loxone Config" panel of a mapping row: commands to configure and URLs to test. */
export function MappingGuide({ mapping, env }: MappingGuideProps) {
  const sensorType = mapping.hue_type === 'sensor'
    ? env.sensors.find((s) => s.id === mapping.hue_id)?.type
    : undefined;
  const guide = getLoxoneGuide(mapping, { sensorType, hasUDP: env.hasUDP, hasHTTP: env.hasHTTP });
  const testUrls = getTestUrls(mapping);

  return (
    <div className="border-t border-gray-700 pt-3 mt-3">
      <div className="flex items-center gap-2 mb-3">
        <Terminal size={16} className="text-hue-orange" />
        <h4 className="text-sm font-medium text-white">Loxone Config - {guide.title}</h4>
      </div>

      <div className="space-y-2">
        {guide.commands.map((cmd) => (
          <div key={cmd.value} className="bg-gray-900 rounded-lg p-3">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-gray-400">{cmd.label}</span>
              <CopyButton text={cmd.value} />
            </div>
            <code className="block text-sm text-hue-orange font-mono break-all">{cmd.value}</code>
            <p className="text-xs text-gray-500 mt-1">{cmd.description}</p>
          </div>
        ))}
      </div>

      <div className="mt-3 p-3 bg-blue-900/20 border border-blue-500/30 rounded-lg">
        <p className="text-xs text-blue-300">
          <strong>Hinweis:</strong> {guide.note}
        </p>
      </div>

      {testUrls.length > 0 && (
        <div className="mt-4 border-t border-gray-700 pt-3">
          <div className="flex items-center gap-2 mb-3">
            <ExternalLink size={16} className="text-green-500" />
            <h4 className="text-sm font-medium text-white">Befehle</h4>
            <span className="text-xs text-gray-500">(Loxone Virtueller Ausgang Befehl)</span>
          </div>
          <div className="space-y-2">
            {testUrls.map((test) => (
              <div key={test.url} className="bg-gray-900 rounded-lg p-3">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-gray-400">{test.label}</span>
                  <div className="flex gap-2">
                    <CopyButton text={test.url} />
                    <Tooltip content="Im Browser testen" position="top">
                      <a
                        href={test.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="px-2 py-1 rounded bg-green-700 text-white hover:bg-green-600 transition-colors flex items-center gap-1 text-xs"
                      >
                        <ExternalLink size={12} />
                        <span>Testen</span>
                      </a>
                    </Tooltip>
                  </div>
                </div>
                <code className="block text-sm text-green-400 font-mono break-all">{test.url}</code>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
