import { useState } from 'react';
import { Lightbulb, LogIn, AlertCircle } from 'lucide-react';
import * as api from '../services/api';

interface LoginScreenProps {
  username: string;
  onLoggedIn: () => void;
}

/** Shown instead of the dashboard while the gateway requires a login. */
export function LoginScreen({ username: initialUsername, onLoggedIn }: LoginScreenProps) {
  const [username, setUsername] = useState(initialUsername || 'admin');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.login(username, password);
      setPassword('');
      onLoggedIn();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Anmeldung fehlgeschlagen');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-900 flex items-center justify-center p-4">
      <form onSubmit={submit} className="bg-gray-800 rounded-xl p-6 max-w-sm w-full space-y-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-hue-orange rounded-lg flex items-center justify-center">
            <Lightbulb size={24} className="text-gray-900" />
          </div>
          <div>
            <h1 className="text-lg font-semibold text-white">Loxone2HUE</h1>
            <p className="text-xs text-gray-400">Anmeldung erforderlich</p>
          </div>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1" htmlFor="login-user">Benutzername</label>
          <input
            id="login-user"
            type="text"
            autoComplete="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none"
          />
        </div>
        <div>
          <label className="block text-sm text-gray-400 mb-1" htmlFor="login-pass">Passwort</label>
          <input
            id="login-pass"
            type="password"
            autoComplete="current-password"
            autoFocus
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full bg-gray-700 text-white rounded-lg px-3 py-2 border border-gray-600 focus:border-hue-orange focus:outline-none"
          />
        </div>

        {error && (
          <p className="flex items-start gap-2 text-sm text-red-300 bg-red-900/30 border border-red-500/50 rounded-lg px-3 py-2">
            <AlertCircle size={16} className="flex-shrink-0 mt-0.5" />
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={busy || !password}
          className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-hue-orange text-gray-900 font-medium rounded-lg hover:bg-orange-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <LogIn size={18} />
          {busy ? 'Anmelden...' : 'Anmelden'}
        </button>
      </form>
    </div>
  );
}
