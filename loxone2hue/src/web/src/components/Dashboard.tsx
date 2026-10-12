import { useState, useEffect, useRef, useCallback } from 'react';
import { useHueDevices } from '../hooks/useHueDevices';
import { DeviceList } from './DeviceList';
import { GroupList } from './GroupList';
import { SceneList } from './SceneList';
import { SensorList } from './SensorList';
import { MappingConfig } from './MappingConfig';
import { LoxoneGuide } from './LoxoneGuide';
import { SettingsPanel } from './SettingsPanel';
import { ReleaseNotes } from './ReleaseNotes';
import * as api from '../services/api';
import type { Mapping } from '../services/api';
import {
  Lightbulb,
  Home,
  Palette,
  Link2,
  BookOpen,
  Code2,
  Settings,
  RefreshCw,
  Wifi,
  WifiOff,
  Menu,
  X,
  Layers,
  Router,
  Timer,
  ChevronDown,
  Activity,
} from 'lucide-react';

// Version info - injected at build time via Vite define
const VERSION = __APP_VERSION__;
const BUILD_DATE = __BUILD_DATE__;

type Tab = 'devices' | 'rooms' | 'zones' | 'scenes' | 'sensors' | 'mappings' | 'settings' | 'guide' | 'api';

export function Dashboard() {
  const [activeTab, setActiveTab] = useState<Tab>('guide');
  const [menuOpen, setMenuOpen] = useState(false);
  const [showReleaseNotes, setShowReleaseNotes] = useState(false);
  const [bridgeIP, setBridgeIP] = useState<string | null>(null);
  const [mappings, setMappings] = useState<Mapping[]>([]);
  const [udpSendAll, setUdpSendAll] = useState(false);
  const [autoRefreshInterval, setAutoRefreshInterval] = useState(10);
  const [showIntervalMenu, setShowIntervalMenu] = useState(false);
  const intervalMenuRef = useRef<HTMLDivElement>(null);

  const {
    lights,
    groups,
    scenes,
    sensors,
    loading,
    error,
    isConnected,
    refresh,
    setLightState,
    setGroupState,
    activateScene,
  } = useHueDevices();

  // Mappings feed the UDP buttons and link icons of the device, room and
  // sensor tabs. MappingConfig calls this after every change so those tabs
  // do not show the state from page load.
  const loadMappings = useCallback(async () => {
    try {
      const result = await api.getMappings();
      setMappings(result.mappings || []);
    } catch (err) {
      console.error('Failed to fetch mappings:', err);
    }
  }, []);

  // Fetch bridge IP and mappings on mount
  useEffect(() => {
    const fetchBridgeInfo = async () => {
      try {
        const config = await api.getConfig();
        if (config.hue?.bridge_ip) {
          setBridgeIP(config.hue.bridge_ip);
        }
        setUdpSendAll(
          (config.loxone?.miniservers || []).some(
            (ms) => ms.udp_enabled && ms.send_all
          )
        );
      } catch (err) {
        console.error('Failed to fetch bridge info:', err);
      }
    };
    fetchBridgeInfo();
    loadMappings();
  }, [loadMappings]);

  // Auto-refresh
  useEffect(() => {
    if (autoRefreshInterval <= 0) return;
    const id = setInterval(() => {
      refresh();
    }, autoRefreshInterval * 1000);
    return () => clearInterval(id);
  }, [autoRefreshInterval, refresh]);

  // Close interval menu on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (intervalMenuRef.current && !intervalMenuRef.current.contains(e.target as Node)) {
        setShowIntervalMenu(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const refreshIntervalOptions = [
    { label: 'Aus', value: 0 },
    { label: '5s', value: 5 },
    { label: '10s', value: 10 },
    { label: '15s', value: 15 },
    { label: '30s', value: 30 },
    { label: '60s', value: 60 },
  ];

  const handleLightToggle = (id: string, on: boolean) => {
    setLightState(id, on);
  };

  const handleLightBrightness = (id: string, brightness: number) => {
    setLightState(id, undefined, brightness);
  };

  const handleGroupToggle = (id: string, on: boolean) => {
    setGroupState(id, on);
  };

  const navigateToHueElement = (hueId: string, hueType: string) => {
    if (hueType === 'light') {
      setActiveTab('devices');
    } else if (hueType === 'group') {
      const group = groups.find((g) => g.id === hueId);
      setActiveTab(group?.type === 'zone' ? 'zones' : 'rooms');
    } else if (hueType === 'scene') {
      setActiveTab('scenes');
    } else if (hueType === 'sensor') {
      setActiveTab('sensors');
    }
  };

  // Separate rooms and zones
  const rooms = groups.filter((g) => g.type === 'room');
  const zones = groups.filter((g) => g.type === 'zone');

  const tabs = [
    { id: 'guide' as Tab, label: 'Anleitung', icon: BookOpen },
    { id: 'mappings' as Tab, label: 'Mappings', icon: Link2 },
    { id: 'devices' as Tab, label: 'Aktoren', icon: Lightbulb, count: lights.length },
    { id: 'sensors' as Tab, label: 'Sensoren', icon: Activity, count: sensors.length },
    { id: 'rooms' as Tab, label: 'Räume', icon: Home, count: rooms.length },
    { id: 'zones' as Tab, label: 'Zonen', icon: Layers, count: zones.length },
    { id: 'scenes' as Tab, label: 'Szenen', icon: Palette, count: scenes.length },
    { id: 'settings' as Tab, label: 'Einstellungen', icon: Settings },
    { id: 'api' as Tab, label: 'API', icon: Code2 },
  ];

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Header */}
      <header className="bg-gray-800 border-b border-gray-700 sticky top-0 z-10">
        <div className="max-w-6xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-hue-orange rounded-lg flex items-center justify-center">
                <Lightbulb size={24} className="text-gray-900" />
              </div>
              <div>
                <h1 className="text-xl font-bold text-white">Loxone2HUE</h1>
                <button
                  onClick={() => setShowReleaseNotes(true)}
                  className="text-xs text-gray-400 hover:text-hue-orange transition-colors cursor-pointer"
                  title="Release Notes anzeigen"
                >
                  Gateway Service v{VERSION}
                </button>
              </div>
            </div>

            <div className="flex items-center gap-4">
              {/* Bridge IP */}
              {bridgeIP && (
                <div className="hidden md:flex items-center gap-2 bg-gray-700/50 px-3 py-1.5 rounded-lg">
                  <Router size={16} className="text-hue-orange" />
                  <span className="text-sm text-gray-300 font-mono">{bridgeIP}</span>
                </div>
              )}

              {/* Connection Status */}
              <div className="hidden sm:flex items-center gap-2">
                {isConnected ? (
                  <Wifi size={18} className="text-green-500" />
                ) : (
                  <WifiOff size={18} className="text-red-500" />
                )}
                <span className="text-sm text-gray-400">
                  {isConnected ? 'Verbunden' : 'Getrennt'}
                </span>
              </div>

              {/* Auto-Refresh Control */}
              <div className="relative flex items-center" ref={intervalMenuRef}>
                <button
                  onClick={refresh}
                  disabled={loading}
                  className="p-2 text-gray-400 hover:text-white transition-colors"
                  title="Jetzt aktualisieren"
                >
                  <RefreshCw size={20} className={loading ? 'animate-spin' : ''} />
                </button>
                <button
                  onClick={() => setShowIntervalMenu(!showIntervalMenu)}
                  className={`
                    flex items-center gap-1 px-2 py-1 rounded-lg text-xs transition-colors
                    ${autoRefreshInterval > 0
                      ? 'text-hue-orange bg-hue-orange/10'
                      : 'text-gray-500 hover:text-gray-300'
                    }
                  `}
                  title="Auto-Refresh Intervall"
                >
                  <Timer size={14} />
                  <span>{autoRefreshInterval > 0 ? `${autoRefreshInterval}s` : 'Aus'}</span>
                  <ChevronDown size={12} />
                </button>

                {showIntervalMenu && (
                  <div className="absolute right-0 top-full mt-2 bg-gray-800 border border-gray-700 rounded-lg shadow-xl py-1 z-20 min-w-[120px]">
                    {refreshIntervalOptions.map((opt) => (
                      <button
                        key={opt.value}
                        onClick={() => {
                          setAutoRefreshInterval(opt.value);
                          setShowIntervalMenu(false);
                        }}
                        className={`
                          w-full text-left px-4 py-2 text-sm transition-colors
                          ${autoRefreshInterval === opt.value
                            ? 'text-hue-orange bg-hue-orange/10'
                            : 'text-gray-300 hover:bg-gray-700'
                          }
                        `}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {/* Mobile Menu Toggle */}
              <button
                onClick={() => setMenuOpen(!menuOpen)}
                className="sm:hidden p-2 text-gray-400"
              >
                {menuOpen ? <X size={24} /> : <Menu size={24} />}
              </button>
            </div>
          </div>

          {/* Desktop Navigation */}
          <nav className="hidden sm:flex gap-1 mt-4">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`
                  flex items-center gap-2 px-4 py-2 rounded-lg transition-colors
                  ${activeTab === tab.id
                    ? 'bg-hue-orange text-gray-900'
                    : 'text-gray-400 hover:text-white hover:bg-gray-700'
                  }
                `}
              >
                <tab.icon size={18} />
                {tab.label}
                {tab.count !== undefined && (
                  <span className={`
                    text-xs px-2 py-0.5 rounded-full
                    ${activeTab === tab.id ? 'bg-gray-900/20' : 'bg-gray-700'}
                  `}>
                    {tab.count}
                  </span>
                )}
              </button>
            ))}
          </nav>

          {/* Mobile Navigation */}
          {menuOpen && (
            <nav className="sm:hidden flex flex-col gap-1 mt-4 pb-2">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => {
                    setActiveTab(tab.id);
                    setMenuOpen(false);
                  }}
                  className={`
                    flex items-center gap-3 px-4 py-3 rounded-lg transition-colors
                    ${activeTab === tab.id
                      ? 'bg-hue-orange text-gray-900'
                      : 'text-gray-400 hover:bg-gray-700'
                    }
                  `}
                >
                  <tab.icon size={20} />
                  {tab.label}
                  {tab.count !== undefined && (
                    <span className="text-xs bg-gray-700 px-2 py-0.5 rounded-full ml-auto">
                      {tab.count}
                    </span>
                  )}
                </button>
              ))}
            </nav>
          )}
        </div>
      </header>

      {/* Error Banner */}
      {error && (
        <div className="bg-red-900/30 border-b border-red-500 px-4 py-3">
          <div className="max-w-6xl mx-auto">
            <p className="text-red-400 text-sm">{error}</p>
          </div>
        </div>
      )}

      {/* Main Content */}
      <main className="max-w-6xl mx-auto px-4 py-6">
        {activeTab === 'devices' && (
          <DeviceList
            devices={lights}
            groups={groups}
            loading={loading}
            onToggle={handleLightToggle}
            onBrightness={handleLightBrightness}
            onRefresh={refresh}
            mappings={mappings}
            udpSendAll={udpSendAll}
          />
        )}

        {activeTab === 'rooms' && (
          <GroupList
            groups={rooms}
            scenes={scenes}
            onToggle={handleGroupToggle}
            onActivateScene={activateScene}
            title="Räume"
            mappings={mappings}
            udpSendAll={udpSendAll}
          />
        )}

        {activeTab === 'zones' && (
          <GroupList
            groups={zones}
            scenes={scenes}
            onToggle={handleGroupToggle}
            onActivateScene={activateScene}
            title="Zonen"
            mappings={mappings}
            udpSendAll={udpSendAll}
          />
        )}

        {activeTab === 'scenes' && (
          <SceneList
            scenes={scenes}
            groups={groups}
            onActivateScene={activateScene}
          />
        )}

        {activeTab === 'sensors' && (
          <SensorList
            sensors={sensors}
            loading={loading}
            mappings={mappings}
            udpSendAll={udpSendAll}
          />
        )}

        {activeTab === 'mappings' && (
          <MappingConfig
            lights={lights}
            groups={groups}
            scenes={scenes}
            sensors={sensors}
            onNavigateToHueElement={navigateToHueElement}
            onMappingsChanged={loadMappings}
          />
        )}

        {activeTab === 'settings' && <SettingsPanel />}

        {activeTab === 'guide' && <LoxoneGuide />}

        {activeTab === 'api' && (
          <div className="bg-gray-800 rounded-xl overflow-hidden h-[calc(100vh-220px)]">
            <iframe
              src="./api/swagger"
              title="API Dokumentation"
              className="w-full h-full border-0 min-h-[600px]"
            />
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-gray-800 py-4 mt-8">
        <div className="max-w-6xl mx-auto px-4 text-center text-xs text-gray-500 space-y-1">
          <div>
            <button
              onClick={() => setShowReleaseNotes(true)}
              className="hover:text-hue-orange transition-colors cursor-pointer"
            >
              Loxone2HUE Gateway v{VERSION}
            </button>
            {' '}&bull; Build {BUILD_DATE}
          </div>
          <div className="text-gray-600">
            &copy; 2026 Stefan Beyeler
          </div>
        </div>
      </footer>

      {/* Release Notes Modal */}
      {showReleaseNotes && (
        <ReleaseNotes
          currentVersion={VERSION}
          onClose={() => setShowReleaseNotes(false)}
        />
      )}
    </div>
  );
}
