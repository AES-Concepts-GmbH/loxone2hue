import { useState, useEffect } from 'react';
import { Dashboard } from './components/Dashboard';
import { BridgeSetup } from './components/BridgeSetup';
import * as api from './services/api';
import { RefreshCw, AlertCircle } from 'lucide-react';

function App() {
  const [configured, setConfigured] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkBridge();
  }, []);

  const checkBridge = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.getBridge();
      setConfigured(response.configured);
    } catch (err) {
      // A network error, a 500 or a cancelled login is not "bridge not
      // configured"; showing the setup dialog here invited users to re-pair
      // a working bridge.
      console.error('Failed to check bridge:', err);
      setError(err instanceof Error ? err.message : 'Verbindung zum Gateway fehlgeschlagen');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
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
            onClick={checkBridge}
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

  return <Dashboard />;
}

export default App;
