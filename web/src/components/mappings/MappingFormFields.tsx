import type { ReactNode } from 'react';
import type { Mapping } from '../../types';
import { sanitizeName } from '../../utils/udp';
import type { MappingEnv } from './env';

interface MappingFormFieldsProps {
  value: Partial<Mapping>;
  onChange: (patch: Partial<Mapping>) => void;
  env: MappingEnv;
  /** Derive the Loxone ID from the name until the user edits the ID by hand. */
  autoLoxoneId?: boolean;
  onLoxoneIdEdited?: () => void;
  loxoneIdPlaceholder?: string;
  showDescription?: boolean;
  /** Rendered between the Loxone ID and the description (type selects, hints). */
  children?: ReactNode;
}

const inputClass = 'w-full bg-gray-700 text-white rounded-lg px-3 py-2';

/** Name, Loxone ID, description, Miniserver and feedback protocol – shared by wizard and inline edit. */
export function MappingFormFields({
  value,
  onChange,
  env,
  autoLoxoneId = false,
  onLoxoneIdEdited,
  loxoneIdPlaceholder = 'Loxone ID',
  showDescription = true,
  children,
}: MappingFormFieldsProps) {
  const { miniservers, hasUDP, hasHTTP } = env;

  return (
    <>
      <input
        type="text"
        placeholder="Name"
        value={value.name || ''}
        onChange={(e) => {
          const name = e.target.value;
          onChange(autoLoxoneId ? { name, loxone_id: sanitizeName(name) } : { name });
        }}
        className={inputClass}
      />
      <input
        type="text"
        placeholder={loxoneIdPlaceholder}
        value={value.loxone_id || ''}
        onChange={(e) => {
          onLoxoneIdEdited?.();
          onChange({ loxone_id: e.target.value });
        }}
        className={inputClass}
      />
      {children}
      {showDescription && (
        <textarea
          placeholder="Beschreibung (optional)"
          value={value.description || ''}
          onChange={(e) => onChange({ description: e.target.value })}
          className={inputClass}
          rows={2}
        />
      )}
      {miniservers.length > 1 && (
        <div>
          <label className="block text-sm text-gray-400 mb-1">Miniserver</label>
          <select
            title="Miniserver wählen"
            value={value.miniserver_id || ''}
            onChange={(e) => onChange({ miniserver_id: e.target.value })}
            className={inputClass}
          >
            <option value="">Miniserver wählen...</option>
            {miniservers.map((ms) => (
              <option key={ms.id} value={ms.id}>{ms.name || ms.ip} ({ms.ip}:{ms.port})</option>
            ))}
          </select>
        </div>
      )}
      {miniservers.length === 1 && (
        <div className="text-xs text-gray-500">
          Miniserver: <span className="text-gray-300">{miniservers[0].name || miniservers[0].ip}</span>
        </div>
      )}
      {(hasUDP || hasHTTP) && (
        <div>
          <label className="block text-sm text-gray-400 mb-2">Feedback-Protokoll</label>
          <div className="flex items-center gap-4">
            {hasUDP && (
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={value.feedback_udp !== false}
                  onChange={(e) => onChange({ feedback_udp: e.target.checked })}
                  className="w-4 h-4 rounded border-gray-600 text-hue-orange focus:ring-hue-orange bg-gray-700"
                />
                <span className="text-gray-300 text-sm">UDP</span>
              </label>
            )}
            {hasHTTP && (
              <label className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={value.feedback_http !== false}
                  onChange={(e) => onChange({ feedback_http: e.target.checked })}
                  className="w-4 h-4 rounded border-gray-600 text-hue-orange focus:ring-hue-orange bg-gray-700"
                />
                <span className="text-gray-300 text-sm">HTTP</span>
              </label>
            )}
          </div>
        </div>
      )}
    </>
  );
}
