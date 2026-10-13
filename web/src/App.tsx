import { useState, useEffect, useCallback } from 'react';
import { Dashboard } from './components/Dashboard';
import { BridgeSetup } from './components/BridgeSetup';
import { LoginScreen } from './components/LoginScreen';
import * as api from './services/api';
import { RefreshCw, AlertCircle } from 'lucide-react';

type AuthState =
  | { kind: 'unknown' }
  | { kind: 'required'; username: string }
  | { kind: 'ok'; enabled: boolean };

function App() {
  const [auth, setAuth] = useState<AuthState>({ kind: 'unknown' });
  const [configured, setConfigured] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const start = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const status = await api.getAuthStatus();
      if (status.enabled && !status.authenticated) {
        setAuth({ kind: 'required', username: status.username });
        return;
      }
      setAuth({ kind: 'ok', enabled: status.enabled });
      const response = await api.getBridge();
      setConfigured(response.configured);
    } catch (err) {
      // A network error, a 500 or a cancelled login is not "bridge not
      // configured"; showing the setup dialog here invited users to re-pair
      // a working bridge.
      console.error('Failed to start:', err);
      setError(err instanceof Error ? err.message : 'Verbindung zum Gateway fehlgeschlagen');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    start();
  }, [start]);

  // Any 401 from the API (expired session, password changed elsewhere)
  // brings the login form back.
  useEffect(() => {
    const onAuthRequired = () => {
      setAuth((prev) => (prev.kind === 'required' ? prev : { kind: 'required', username: '' }));
    };
    window.addEventListener(api.AUTH_REQUIRED_EVENT, onAuthRequired);
    return () => window.removeEventListener(api.AUTH_REQUIRED_EVENT, onAuthRequired);
  }, []);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch (err) {
      console.error('Logout failed:', err);
    }
    setAuth({ kind: 'required', username: '' });
  };

  if (auth.kind === 'required') {
    return <LoginScreen username={auth.username} onLoggedIn={start} />;
  }

  if (loading || auth.kind === 'unknown') {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <RefreshCw className="animate-spin text-hue-orange" size={48} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center p-4">
        <div className="bg-gray-800 rounded-xl p-6 max-w-md w-full text-center">
          <AlertCircle className="mx-auto text-red-400 mb-3" size={40} />
          <h1 className="text-lg font-medium text-white mb-2">Gateway nicht erreichbar</h1>
          <p className="text-sm text-gray-400 mb-4 break-words">{error}</p>
          <button
            onClick={start}
            className="px-4 py-2 rounded-lg bg-hue-orange text-gray-900 font-medium hover:bg-orange-400 transition-colors"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  if (!configured) {
    return <BridgeSetup onComplete={() => setConfigured(true)} />;
  }

  return <Dashboard authEnabled={auth.enabled} onLogout={handleLogout} onAuthChanged={(enabled) => setAuth({ kind: 'ok', enabled })} />;
}

export default App;
