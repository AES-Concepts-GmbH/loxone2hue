import { useEffect, useState } from 'react';
import { Check, Copy, Eye, EyeOff, X } from 'lucide-react';
import { copyToClipboard } from '../../utils/clipboard';
import { buildExternalUrl, PASSWORD_PLACEHOLDER } from '../../utils/loxoneGuide';

interface FullUrlPanelProps {
  /** Relative command URL (./ws?cmd=...). */
  url: string;
  gatewayPort?: number;
  authUser: string;
  /** Password typed in this guide; lives only in component state, never stored. */
  password: string;
  onPasswordChange: (password: string) => void;
  onClose: () => void;
}

/**
 * Asks for the login password and builds the full command URL with it while
 * the user types. The password stays in memory only: it is not sent to the
 * gateway, not written to any storage and gone when the guide closes.
 */
export function FullUrlPanel({ url, gatewayPort, authUser, password, onPasswordChange, onClose }: FullUrlPanelProps) {
  const [showPassword, setShowPassword] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const fullUrl = buildExternalUrl(url, { gatewayPort, authUser, authPassword: password });
  // The preview masks the password unless the user asks to see it.
  const previewUrl = showPassword || !password
    ? fullUrl
    : fullUrl.replace(`:${encodeURIComponent(password)}@`, `:${'•'.repeat(Math.min(password.length, 12))}@`);

  const copy = async () => {
    try {
      await copyToClipboard(fullUrl);
      setCopied(true);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  return (
    <form
      className="mt-2 p-3 rounded-lg bg-gray-800 border border-gray-700 space-y-2"
      onSubmit={(e) => {
        e.preventDefault();
        copy();
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onClose();
      }}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs text-gray-400">
          Login-Schutz aktiv: Passwort für <strong className="text-gray-300">{authUser}</strong> eingeben (wird nicht gespeichert)
        </span>
        <button type="button" onClick={onClose} aria-label="Schliessen" className="text-gray-500 hover:text-white">
          <X size={14} />
        </button>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[180px]">
          <input
            type={showPassword ? 'text' : 'password'}
            autoComplete="off"
            autoFocus
            placeholder="Passwort"
            value={password}
            onChange={(e) => onPasswordChange(e.target.value)}
            className="w-full bg-gray-700 text-white rounded px-2 py-1 pr-8 border border-gray-600 focus:border-hue-orange focus:outline-none placeholder-gray-500 text-xs"
          />
          <button
            type="button"
            onClick={() => setShowPassword((v) => !v)}
            aria-label={showPassword ? 'Passwort verbergen' : 'Passwort anzeigen'}
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
          >
            {showPassword ? <EyeOff size={12} /> : <Eye size={12} />}
          </button>
        </div>
        <button
          type="submit"
          className={`px-2 py-1 rounded transition-all flex items-center gap-1 text-xs ${
            copied ? 'bg-green-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600 hover:text-white'
          }`}
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          <span>{copied ? 'Kopiert!' : 'URL kopieren'}</span>
        </button>
      </div>
      <code className="block text-xs text-gray-300 font-mono break-all">{previewUrl}</code>
      {!password && (
        <p className="text-xs text-gray-500">
          Ohne Passwort enthält die URL den Platzhalter <code>{PASSWORD_PLACEHOLDER}</code>.
        </p>
      )}
    </form>
  );
}
