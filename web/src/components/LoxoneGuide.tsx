import { BookOpen, Settings, Code, Link2, Terminal, Lightbulb, Home, Play, CheckCircle2, AlertTriangle, Zap, HelpCircle, Layers, Radio, Download } from 'lucide-react';

interface LoxoneGuideProps {
  /** Port the gateway listens on (from the server config); falls back to the page's port. */
  gatewayPort?: number;
}

export function LoxoneGuide({ gatewayPort }: LoxoneGuideProps) {
  const gatewayHost = window.location.hostname;
  const port = gatewayPort ?? (window.location.port ? Number(window.location.port) : 8080);
  const gatewayUrl = `${gatewayHost}:${port}`;

  return (
    <div className="space-y-8 max-w-4xl">
      {/* Übersicht */}
      <section className="bg-gray-800 rounded-xl p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 bg-hue-orange rounded-lg flex items-center justify-center">
            <BookOpen size={20} className="text-gray-900" />
          </div>
          <h2 className="text-xl font-bold text-white">Loxone Integration</h2>
        </div>
        <p className="text-gray-300 leading-relaxed">
          Der Loxone2HUE Gateway ermöglicht die bidirektionale Kommunikation zwischen deinem
          Loxone Miniserver und der Philips HUE Bridge. Du kannst HUE Lampen, Gruppen,
          Szenen und Sensoren direkt aus Loxone steuern und überwachen.
        </p>

        <div className="mt-4 grid gap-3 sm:grid-cols-5">
          <div className="bg-gray-900 rounded-lg p-3 text-center">
            <Lightbulb size={24} className="mx-auto text-hue-orange mb-2" />
            <span className="text-sm text-gray-300">Geräte</span>
            <p className="text-xs text-gray-500 mt-1">Nach Räumen gruppiert</p>
          </div>
          <div className="bg-gray-900 rounded-lg p-3 text-center">
            <Home size={24} className="mx-auto text-hue-orange mb-2" />
            <span className="text-sm text-gray-300">Räume</span>
            <p className="text-xs text-gray-500 mt-1">Physische Bereiche</p>
          </div>
          <div className="bg-gray-900 rounded-lg p-3 text-center">
            <Layers size={24} className="mx-auto text-hue-orange mb-2" />
            <span className="text-sm text-gray-300">Zonen</span>
            <p className="text-xs text-gray-500 mt-1">Virtuelle Gruppen</p>
          </div>
          <div className="bg-gray-900 rounded-lg p-3 text-center">
            <Play size={24} className="mx-auto text-hue-orange mb-2" />
            <span className="text-sm text-gray-300">Szenen</span>
            <p className="text-xs text-gray-500 mt-1">Lichtstimmungen</p>
          </div>
          <div className="bg-gray-900 rounded-lg p-3 text-center">
            <Radio size={24} className="mx-auto text-hue-orange mb-2" />
            <span className="text-sm text-gray-300">Sensoren</span>
            <p className="text-xs text-gray-500 mt-1">Taster, Bewegung, etc.</p>
          </div>
        </div>
      </section>

      {/* Dashboard Übersicht */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Settings size={20} className="text-hue-orange" />
          Dashboard Übersicht
        </h3>
        <div className="space-y-4 text-gray-300">
          <p className="text-sm">
            Das Dashboard bietet eine übersichtliche Steuerung aller HUE-Ressourcen:
          </p>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Lightbulb size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Geräte</span>
              </div>
              <p className="text-sm text-gray-400">
                Alle Lampen werden automatisch nach ihren zugewiesenen Räumen gruppiert dargestellt.
                Du siehst sofort, welche Geräte zu welchem Raum gehören.
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Home size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Räume</span>
              </div>
              <p className="text-sm text-gray-400">
                Physische Räume wie "Wohnzimmer" oder "Küche". Jeder Raum kann mehrere Lampen enthalten
                und verfügt über eigene Szenen.
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Layers size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Zonen</span>
              </div>
              <p className="text-sm text-gray-400">
                Virtuelle Gruppen, die Lampen aus verschiedenen Räumen zusammenfassen können,
                z.B. "Erdgeschoss" oder "Alle Flure".
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Play size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Szenen</span>
              </div>
              <p className="text-sm text-gray-400">
                Vordefinierte Lichtstimmungen. Im Dropdown werden Szenen mit ihrem zugehörigen
                Raum angezeigt: "Raum - Szenenname".
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Radio size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Sensoren</span>
              </div>
              <p className="text-sm text-gray-400">
                HUE Sensoren, Taster und Zubehör. Bewegungsmelder, Temperatursensoren,
                Helligkeitssensoren, Taster (Dimmer Switch, Tap Dial) und Kontaktsensoren.
                Sensorwerte werden automatisch per UDP/HTTP an Loxone gesendet.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Architektur */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Code size={20} className="text-hue-orange" />
          Funktionsweise
        </h3>
        <div className="bg-gray-900 rounded-lg p-4 font-mono text-sm text-gray-300 overflow-x-auto">
          <pre>{`┌─────────────────┐  HTTP   ┌─────────────────────────┐  CLIP  ┌───────────────┐
│     Loxone      │────────►│    Loxone2HUE Gateway   │◄──────►│   HUE Bridge  │
│   Miniserver    │◄────────│        (Port ${port})      │  SSE   │               │
└─────────────────┘UDP/HTTP └─────────────────────────┘        └───────────────┘
                                       │
                  HTTP: Befehle senden  │ WebSocket /ws
                  UDP/HTTP: Feedback    │
                                       ▼
                            ┌─────────────────────────┐
                            │   Frontend (Browser)    │
                            └─────────────────────────┘`}</pre>
        </div>
        <p className="text-sm text-gray-400 mt-3">
          Der Gateway unterstützt mehrere Loxone Miniserver gleichzeitig. Jeder Miniserver kann
          eigene IP, UDP-Port und Feedback-Einstellungen haben. Mappings werden einem bestimmten
          Miniserver zugeordnet.
        </p>
      </section>

      {/* Mapping erstellen */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Link2 size={20} className="text-hue-orange" />
          Schritt 1: Mappings erstellen
        </h3>
        <div className="space-y-4 text-gray-300">
          <p>
            Ein Mapping verbindet eine <strong>Loxone ID</strong> mit einer
            <strong> HUE Ressource</strong> (Licht, Gruppe oder Szene). Die Loxone ID wird automatisch
            aus dem Namen generiert (Kleinbuchstaben, Umlaute konvertiert, Leerzeichen als Unterstriche)
            und kann bei Bedarf manuell angepasst werden.
          </p>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Lightbulb size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Licht</span>
              </div>
              <p className="text-sm text-gray-400">
                Einzelne Lampe steuern (Ein/Aus, Helligkeit, Farbe)
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Home size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Gruppe/Raum</span>
              </div>
              <p className="text-sm text-gray-400">
                Alle Lampen eines Raums gleichzeitig steuern
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Play size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Szene</span>
              </div>
              <p className="text-sm text-gray-400">
                Vordefinierte Lichtstimmung aktivieren
              </p>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Radio size={18} className="text-hue-orange" />
                <span className="font-medium text-white">Sensor</span>
              </div>
              <p className="text-sm text-gray-400">
                Sensorwerte empfangen (Bewegung, Temperatur, Taster)
              </p>
            </div>
          </div>

          <div className="bg-blue-900/30 border border-blue-500/30 rounded-lg p-4">
            <h4 className="font-medium text-blue-400 mb-2">Beispiel-Mappings:</h4>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-400">
                  <th className="pb-2">Name</th>
                  <th className="pb-2">Loxone ID (auto)</th>
                  <th className="pb-2">HUE Ressource</th>
                </tr>
              </thead>
              <tbody className="text-gray-300">
                <tr>
                  <td className="py-1">Wohnzimmer Decke</td>
                  <td className="py-1 font-mono text-xs">wohnzimmer_decke</td>
                  <td className="py-1">Deckenlampe (Licht)</td>
                </tr>
                <tr>
                  <td className="py-1">Küche komplett</td>
                  <td className="py-1 font-mono text-xs">kueche_komplett</td>
                  <td className="py-1">Küche (Gruppe)</td>
                </tr>
                <tr>
                  <td className="py-1">Büro Stefan</td>
                  <td className="py-1 font-mono text-xs">buero_stefan</td>
                  <td className="py-1">Büro Stefan (Gruppe)</td>
                </tr>
              </tbody>
            </table>
            <p className="text-xs text-gray-500 mt-2">
              Die Loxone ID wird beim Erstellen automatisch aus dem Namen abgeleitet: Kleinbuchstaben, ä→ae, ö→oe, ü→ue, ß→ss, Leerzeichen→Unterstrich.
            </p>
          </div>
        </div>
      </section>

      {/* Loxone Konfiguration */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Terminal size={20} className="text-hue-orange" />
          Schritt 2: Loxone Miniserver konfigurieren
        </h3>
        <div className="space-y-6 text-gray-300">
          <div>
            <h4 className="font-medium text-white mb-3">2.1 Virtuellen Ausgang erstellen</h4>
            <ol className="list-decimal list-inside space-y-2 text-sm ml-4">
              <li>Öffne <strong>Loxone Config</strong></li>
              <li>Erstelle einen neuen <strong>Virtuellen Ausgang</strong></li>
              <li>Konfiguriere die Adresse: <code className="bg-gray-900 px-2 py-0.5 rounded">http://{gatewayUrl}</code></li>
              <li>Speichere den Virtuellen Ausgang</li>
            </ol>
          </div>

          <div>
            <h4 className="font-medium text-white mb-3">2.2 Virtuelle Ausgang-Befehle erstellen</h4>
            <p className="text-sm mb-3">
              Erstelle für jedes Mapping einen <strong>Virtuellen Ausgang Befehl</strong> unter dem
              Virtuellen Ausgang. Rechtsklick → Virtuellen Ausgang Befehl hinzufügen.
            </p>

            <div className="space-y-4">
              {/* Ein/Aus Steuerung */}
              <div className="bg-gray-900 rounded-lg p-4">
                <h5 className="font-medium text-hue-orange mb-3">Ein/Aus Steuerung (Digital)</h5>
                <p className="text-sm text-gray-400 mb-3">Für Licht oder Gruppe ein- und ausschalten.</p>
                <div className="space-y-2 text-sm">
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Bezeichnung:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">Büro Licht</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=SET buero_stefan ON</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei AUS:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=SET buero_stefan OFF</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei AUS:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Als Digitalausgang:</span>
                    <span className="text-green-400">✓ aktiviert</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  Alle anderen Felder (HTTP header, HTTP body, HTTP-Antwort speichern, Wiederholung) leer lassen.
                </p>
              </div>

              {/* Helligkeit */}
              <div className="bg-gray-900 rounded-lg p-4">
                <h5 className="font-medium text-hue-orange mb-3">Helligkeit (Analog 0-100%)</h5>
                <p className="text-sm text-gray-400 mb-3">Für stufenlose Helligkeitssteuerung über einen Analogwert.</p>
                <div className="space-y-2 text-sm">
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Bezeichnung:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">Büro Helligkeit</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=SET buero_stefan BRI &lt;v&gt;</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Als Digitalausgang:</span>
                    <span className="text-red-400">✗ deaktiviert</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  <code className="bg-gray-800 px-1 rounded">&lt;v&gt;</code> wird von Loxone automatisch durch den aktuellen Wert (0-100) ersetzt.
                  Befehl bei AUS kann leer bleiben.
                </p>
              </div>

              {/* Szene aktivieren */}
              <div className="bg-gray-900 rounded-lg p-4">
                <h5 className="font-medium text-hue-orange mb-3">Szene aktivieren (Impuls)</h5>
                <p className="text-sm text-gray-400 mb-3">Aktiviert eine HUE-Szene bei Tastendruck.</p>
                <div className="space-y-2 text-sm">
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Bezeichnung:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">Szene Entspannen</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=SCENE sz_relax</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Als Digitalausgang:</span>
                    <span className="text-green-400">✓ aktiviert</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  Befehl bei AUS leer lassen. Als Taster/Impuls konfigurieren.
                </p>
              </div>

              {/* Lichtsteuerungs-Baustein */}
              <div className="bg-gray-900 rounded-lg p-4 border-2 border-hue-orange/50">
                <h5 className="font-medium text-hue-orange mb-3">Lichtsteuerungs-Baustein (MOOD)</h5>
                <p className="text-sm text-gray-400 mb-3">
                  Für den Loxone Lichtsteuerungs-Baustein. Verbinde den <strong>AQ</strong>-Ausgang
                  (Stimmungsnummer) mit diesem Virtuellen Ausgang Befehl.
                </p>
                <div className="space-y-2 text-sm">
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Bezeichnung:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">Wohnzimmer HUE</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=MOOD wohnzimmer &lt;v&gt;</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Als Digitalausgang:</span>
                    <span className="text-red-400">✗ deaktiviert</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  <code className="bg-gray-800 px-1 rounded">&lt;v&gt;</code> = Stimmungsnummer vom AQ-Ausgang (0=Aus, 1=Mood 1, 2=Mood 2, ...).
                  Befehl bei AUS leer lassen.
                </p>
                <div className="mt-3 p-3 bg-blue-900/30 border border-blue-500/30 rounded">
                  <h6 className="text-xs font-medium text-blue-400 mb-2">Benötigte Mappings:</h6>
                  <ul className="text-xs text-blue-300 space-y-1">
                    <li><code className="bg-blue-900/50 px-1 rounded">wohnzimmer</code> → Gruppe (für Mood 0 = Aus)</li>
                    <li><code className="bg-blue-900/50 px-1 rounded">wohnzimmer_mood_1</code> → Szene (Mood 1)</li>
                    <li><code className="bg-blue-900/50 px-1 rounded">wohnzimmer_mood_2</code> → Szene (Mood 2)</li>
                    <li>... weitere Moods nach Bedarf</li>
                  </ul>
                </div>
              </div>

              {/* Farbtemperatur */}
              <div className="bg-gray-900 rounded-lg p-4">
                <h5 className="font-medium text-hue-orange mb-3">Farbtemperatur (Analog 2000-6500K)</h5>
                <p className="text-sm text-gray-400 mb-3">Für stufenlose Farbtemperatur-Steuerung.</p>
                <div className="space-y-2 text-sm">
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Bezeichnung:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">Büro Farbtemperatur</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Befehl bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">/ws?cmd=SET buero_stefan CT &lt;v&gt;</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">HTTP Methode bei EIN:</span>
                    <code className="bg-gray-800 px-3 py-1.5 rounded font-mono">GET</code>
                  </div>
                  <div className="grid grid-cols-[180px_1fr] gap-2 items-center">
                    <span className="text-gray-400">Als Digitalausgang:</span>
                    <span className="text-red-400">✗ deaktiviert</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  <code className="bg-gray-800 px-1 rounded">&lt;v&gt;</code> wird durch den Kelvin-Wert (2000-6500) ersetzt.
                  Befehl bei AUS kann leer bleiben.
                </p>
              </div>
            </div>

            {/* Allgemeine Hinweise */}
            <div className="mt-4 p-4 bg-blue-900/30 border border-blue-500/30 rounded-lg">
              <h5 className="text-sm font-medium text-blue-400 mb-2">Wichtige Hinweise zu den Feldern</h5>
              <ul className="text-xs text-blue-300 space-y-1.5">
                <li>• <strong>HTTP Methode</strong>: Immer <code className="bg-blue-900/50 px-1 rounded">GET</code> verwenden (bei EIN und AUS)</li>
                <li>• <strong>HTTP header / HTTP body</strong>: Leer lassen</li>
                <li>• <strong>HTTP-Antwort speichern</strong>: Leer lassen</li>
                <li>• <strong>Wiederholung</strong>: Auf 0 lassen</li>
                <li>• <strong>Als Digitalausgang</strong>: Aktiviert für Ein/Aus und Szene, deaktiviert für Analog-Werte (Helligkeit, MOOD, Farbtemperatur)</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      {/* Kommando-Referenz */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Code size={20} className="text-hue-orange" />
          Kommando-Referenz
        </h3>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-gray-400 border-b border-gray-700">
                <th className="pb-3 pr-4">Kommando</th>
                <th className="pb-3 pr-4">Beschreibung</th>
                <th className="pb-3">Beispiel</th>
              </tr>
            </thead>
            <tbody className="text-gray-300">
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SET &lt;id&gt; ON</td>
                <td className="py-3 pr-4">Licht/Gruppe einschalten</td>
                <td className="py-3 font-mono text-xs">SET wz_decke ON</td>
              </tr>
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SET &lt;id&gt; OFF</td>
                <td className="py-3 pr-4">Licht/Gruppe ausschalten</td>
                <td className="py-3 font-mono text-xs">SET wz_decke OFF</td>
              </tr>
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SET &lt;id&gt; BRI &lt;0-100&gt;</td>
                <td className="py-3 pr-4">Helligkeit setzen (%). 0 schaltet aus, jeder Wert über 0 schaltet ein</td>
                <td className="py-3 font-mono text-xs">SET wz_decke BRI 75</td>
              </tr>
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SET &lt;id&gt; CT &lt;2000-6500&gt;</td>
                <td className="py-3 pr-4">Farbtemperatur (Kelvin)</td>
                <td className="py-3 font-mono text-xs">SET wz_decke CT 4000</td>
              </tr>
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SET &lt;id&gt; COLOR &lt;hex&gt;</td>
                <td className="py-3 pr-4">Farbe setzen (RGB Hex)</td>
                <td className="py-3 font-mono text-xs">SET wz_decke COLOR #FF5500</td>
              </tr>
              <tr className="border-b border-gray-700/50">
                <td className="py-3 pr-4 font-mono text-hue-orange">SCENE &lt;id&gt;</td>
                <td className="py-3 pr-4">Szene aktivieren</td>
                <td className="py-3 font-mono text-xs">SCENE sz_relax</td>
              </tr>
              <tr className="border-b border-gray-700/50 bg-hue-orange/10">
                <td className="py-3 pr-4 font-mono text-hue-orange">MOOD &lt;id&gt; &lt;nr&gt;</td>
                <td className="py-3 pr-4">Stimmung aktivieren (Lichtsteuerung)</td>
                <td className="py-3 font-mono text-xs">MOOD wohnzimmer 1</td>
              </tr>
              <tr>
                <td className="py-3 pr-4 font-mono text-hue-orange">GET &lt;id&gt; STATUS</td>
                <td className="py-3 pr-4">Status abfragen</td>
                <td className="py-3 font-mono text-xs">GET wz_decke STATUS</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      {/* Praxis-Beispiel: Lichtsteuerungs-Baustein */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Zap size={20} className="text-hue-orange" />
          Praxis-Beispiel: Lichtsteuerungs-Baustein
        </h3>
        <div className="space-y-4 text-gray-300">
          <p className="text-sm">
            Komplettes Beispiel für einen Raum "Wohnzimmer" mit dem Loxone Lichtsteuerungs-Baustein
            und 3 HUE-Szenen.
          </p>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">1. Benötigte Mappings erstellen</h4>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-400 border-b border-gray-700">
                  <th className="pb-2 pr-4">Loxone ID</th>
                  <th className="pb-2 pr-4">HUE Typ</th>
                  <th className="pb-2">HUE Ressource</th>
                </tr>
              </thead>
              <tbody>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">wohnzimmer</td>
                  <td className="py-2 pr-4">Gruppe</td>
                  <td className="py-2">Wohnzimmer (Raum)</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">wohnzimmer_mood_1</td>
                  <td className="py-2 pr-4">Szene</td>
                  <td className="py-2">Entspannen</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">wohnzimmer_mood_2</td>
                  <td className="py-2 pr-4">Szene</td>
                  <td className="py-2">Konzentrieren</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">wohnzimmer_mood_3</td>
                  <td className="py-2 pr-4">Szene</td>
                  <td className="py-2">Energie tanken</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">2. Loxone Config: Virtueller Ausgang</h4>
            <div className="space-y-2 text-sm">
              <div>
                <span className="text-gray-400">Adresse:</span>
                <code className="block bg-gray-800 px-3 py-2 rounded mt-1 font-mono">
                  http://{gatewayUrl}
                </code>
              </div>
            </div>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">3. Loxone Config: Virtueller Ausgang Befehl</h4>
            <div className="space-y-2 text-sm">
              <div>
                <span className="text-gray-400">Bezeichnung:</span>
                <span className="ml-2">Wohnzimmer HUE</span>
              </div>
              <div>
                <span className="text-gray-400">Befehl (Analog vom AQ-Ausgang):</span>
                <code className="block bg-gray-800 px-3 py-2 rounded mt-1 font-mono">
                  /ws?cmd=MOOD wohnzimmer &lt;v&gt;
                </code>
              </div>
            </div>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">4. Loxone Config: Lichtsteuerungs-Baustein</h4>
            <div className="space-y-2 text-sm">
              <p className="text-gray-400">
                Verbinde den <strong>AQ</strong>-Ausgang (Stimmungsnummer) mit dem Virtuellen Ausgang Befehl.
              </p>
              <div className="mt-3 p-3 bg-blue-900/30 border border-blue-500/30 rounded">
                <p className="text-xs text-blue-300">
                  Der AQ-Ausgang gibt die aktuelle Stimmungsnummer aus:
                  <strong> 0</strong> = Aus,
                  <strong> 1</strong> = Mood 1,
                  <strong> 2</strong> = Mood 2, etc.
                </p>
              </div>
            </div>
          </div>

          <div className="bg-hue-orange/10 border border-hue-orange/30 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-2">Ergebnis</h4>
            <ul className="text-sm text-orange-300 space-y-1">
              <li>• Mood 0 → Wohnzimmer-Gruppe wird ausgeschaltet</li>
              <li>• Mood 1 → Szene "Entspannen" wird aktiviert</li>
              <li>• Mood 2 → Szene "Konzentrieren" wird aktiviert</li>
              <li>• Mood 3 → Szene "Energie tanken" wird aktiviert</li>
            </ul>
          </div>
        </div>
      </section>

      {/* Status-Feedback */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Radio size={20} className="text-hue-orange" />
          Status-Feedback (UDP &amp; HTTP)
        </h3>
        <div className="space-y-4 text-gray-300">
          <p className="text-sm">
            Der Gateway sendet Status-Änderungen von HUE-Geräten automatisch per <strong>UDP</strong> und/oder <strong>HTTP</strong> an den Loxone Miniserver.
            So erhält Loxone Echtzeit-Rückmeldungen über den Zustand der Lampen und Sensoren.
          </p>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">Aktivierung (Einstellungen)</h4>
            <ol className="list-decimal list-inside space-y-2 text-sm ml-4">
              <li>Gehe zum Tab <strong>Einstellungen</strong></li>
              <li>Aktiviere <strong>UDP Feedback</strong> und/oder <strong>HTTP Feedback</strong> pro Miniserver</li>
              <li>Für UDP: Gib die <strong>Miniserver-IP</strong> und den <strong>UDP-Port</strong> ein (Standard: 7777)</li>
              <li>Für HTTP: Gib die <strong>Ziel-URL</strong> ein (z.B. <code>https://192.168.1.7:443/request.php</code>) sowie Benutzer/Passwort</li>
              <li>Klicke auf <strong>Speichern</strong></li>
            </ol>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">Pro-Mapping Protokollwahl</h4>
            <p className="text-sm text-gray-400 mb-2">
              Beim Erstellen oder Bearbeiten eines Mappings kann das Feedback-Protokoll pro Mapping gewählt werden:
            </p>
            <ul className="list-disc list-inside space-y-1 text-sm ml-4 text-gray-400">
              <li><strong>UDP</strong> — Klassisches UDP-Paket an den Miniserver</li>
              <li><strong>HTTP</strong> — HTTP GET-Request an den virtuellen HTTP-Eingang</li>
              <li><strong>Beide</strong> — Standard, wenn keine explizite Auswahl getroffen wird</li>
            </ul>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">UDP-Format</h4>
            <p className="text-sm text-gray-400 mb-3">
              Pro Eigenschaftsänderung wird ein UDP-Paket gesendet:
            </p>
            <div className="bg-gray-800 rounded-lg p-3 font-mono text-sm mb-3">
              <code>&lt;loxone_id&gt;/&lt;eigenschaft&gt;:&lt;wert&gt;</code>
            </div>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">HTTP-Format</h4>
            <p className="text-sm text-gray-400 mb-3">
              Pro Eigenschaftsänderung wird ein HTTP GET-Request gesendet:
            </p>
            <div className="bg-gray-800 rounded-lg p-3 font-mono text-sm mb-3">
              <code>&lt;ziel_url&gt;/dev/sps/io/&lt;loxone_id&gt;_&lt;eigenschaft&gt;/&lt;wert&gt;</code>
            </div>
            <p className="text-sm text-gray-400">
              Beispiel: <code className="text-xs">https://192.168.1.7/dev/sps/io/buero_stefan_on/1</code>
            </p>
            <p className="text-sm text-yellow-400/80 mt-3">
              Benutzer und Passwort gehen als HTTP Basic Auth mit. Über <code className="text-xs">http://</code> ist
              das Klartext im lokalen Netz; eine <code className="text-xs">https://</code>-URL verschlüsselt die
              Übertragung. Das Zertifikat des Miniservers ist selbstsigniert und wird standardmässig nicht geprüft;
              die Prüfung lässt sich in den Einstellungen einschalten.
            </p>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">Aktoren — Feedback-Eigenschaften</h4>
            <p className="text-sm text-gray-400 mb-3">
              Lampen und Gruppen senden folgende Eigenschaften bei Statusänderungen:
            </p>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-400 border-b border-gray-700">
                  <th className="pb-2 pr-4">Eigenschaft</th>
                  <th className="pb-2 pr-4">Wertebereich</th>
                  <th className="pb-2 pr-4">Beispiel (UDP)</th>
                  <th className="pb-2">Beispiel (HTTP)</th>
                </tr>
              </thead>
              <tbody>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">on</td>
                  <td className="py-2 pr-4">0 / 1</td>
                  <td className="py-2 pr-4 font-mono text-xs">buero_stefan/on:1</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/buero_stefan_on/1</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">bri</td>
                  <td className="py-2 pr-4">0–100</td>
                  <td className="py-2 pr-4 font-mono text-xs">buero_stefan/bri:80</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/buero_stefan_bri/80</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">ct</td>
                  <td className="py-2 pr-4">153–500 (Mirek)</td>
                  <td className="py-2 pr-4 font-mono text-xs">buero_stefan/ct:250</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/buero_stefan_ct/250</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">color_x</td>
                  <td className="py-2 pr-4">0–1 (Float)</td>
                  <td className="py-2 pr-4 font-mono text-xs">buero_stefan/color_x:0.4573</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/buero_stefan_color_x/0.4573</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">color_y</td>
                  <td className="py-2 pr-4">0–1 (Float)</td>
                  <td className="py-2 pr-4 font-mono text-xs">buero_stefan/color_y:0.4100</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/buero_stefan_color_y/0.41</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-3">Sensoren — Feedback-Eigenschaften</h4>
            <p className="text-sm text-gray-400 mb-3">
              Sensoren senden Werte automatisch an Loxone. Es werden keine Befehle an Sensoren gesendet.
            </p>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-gray-400 border-b border-gray-700">
                  <th className="pb-2 pr-4">Sensortyp</th>
                  <th className="pb-2 pr-4">Eigenschaft</th>
                  <th className="pb-2 pr-4">Wertebereich</th>
                  <th className="pb-2 pr-4">Beispiel (UDP)</th>
                  <th className="pb-2">Beispiel (HTTP)</th>
                </tr>
              </thead>
              <tbody>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Bewegung</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">motion</td>
                  <td className="py-2 pr-4">0 / 1</td>
                  <td className="py-2 pr-4 font-mono text-xs">sensor_flur/motion:1</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/sensor_flur_motion/1</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Temperatur</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">temperature</td>
                  <td className="py-2 pr-4">z.B. 21.5 (°C)</td>
                  <td className="py-2 pr-4 font-mono text-xs">sensor_bad/temperature:21.5</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/sensor_bad_temperature/21.5</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Raum-Bewegung (grouped_motion)</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">motion</td>
                  <td className="py-2 pr-4">0 / 1, von der Bridge pro Raum zusammengefasst</td>
                  <td className="py-2 pr-4 font-mono text-xs">wohnzimmer_bewegung/motion:1</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/wohnzimmer_bewegung_motion/1</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Helligkeit (Rohwert)</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">light_level</td>
                  <td className="py-2 pr-4">Hue-Rohwert: 10000·log10(Lux)+1</td>
                  <td className="py-2 pr-4 font-mono text-xs">sensor_flur/light_level:20001</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/sensor_flur_light_level/20001</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Helligkeit (Lux)</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">lux</td>
                  <td className="py-2 pr-4">0–100000 (Lux), auch für Raum-Helligkeit</td>
                  <td className="py-2 pr-4 font-mono text-xs">sensor_flur/lux:100</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/sensor_flur_lux/100</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Taster</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">button</td>
                  <td className="py-2 pr-4">0=press, 2=short, 3=long_release, 4=long_press</td>
                  <td className="py-2 pr-4 font-mono text-xs">smart_button_1/button:2</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/smart_button_1_button/2</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Kontakt</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">contact</td>
                  <td className="py-2 pr-4">0=geschlossen, 1=offen</td>
                  <td className="py-2 pr-4 font-mono text-xs">tuer_sensor/contact:0</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/tuer_sensor_contact/0</td>
                </tr>
                <tr className="border-b border-gray-700/50">
                  <td className="py-2 pr-4 text-gray-300">Drehregler</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">rotary</td>
                  <td className="py-2 pr-4">Schritte (+/-)</td>
                  <td className="py-2 pr-4 font-mono text-xs">tap_dial/rotary:3</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/tap_dial_rotary/3</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 text-gray-300">Batterie</td>
                  <td className="py-2 pr-4 font-mono text-xs text-hue-orange">battery</td>
                  <td className="py-2 pr-4">0–100 (%)</td>
                  <td className="py-2 pr-4 font-mono text-xs">smart_button_1/battery:85</td>
                  <td className="py-2 font-mono text-xs">/dev/sps/io/smart_button_1_battery/85</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div className="bg-blue-900/30 border border-blue-500/30 rounded-lg p-4">
            <p className="text-sm text-blue-300">
              <strong>Hinweis:</strong> Standardmässig werden nur Geräte übertragen, für die ein aktives Mapping existiert.
              Die Loxone ID aus dem Mapping wird als Prefix verwendet.
            </p>
            <p className="text-sm text-blue-300 mt-2">
              Mit der Option <strong>Alle Geräte senden</strong> in den Einstellungen werden
              auch Geräte ohne Mapping übertragen. In diesem Fall wird der Gerätename als Loxone-ID verwendet.
            </p>
          </div>
        </div>
      </section>

      {/* Loxone Config Export */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Download size={20} className="text-hue-orange" />
          Loxone Config Export
        </h3>
        <div className="space-y-4 text-gray-300">
          <p className="text-sm">
            XML-Vorlagen für den Import in Loxone Config. Die Dateien können unter
            <strong> Einstellungen → Loxone Config Export</strong> heruntergeladen werden.
          </p>

          <div className="bg-yellow-900/30 border border-yellow-500/30 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">Import in Loxone Config (2 Schritte)</h4>
            <ol className="text-sm text-yellow-300 space-y-2 list-decimal list-inside">
              <li>
                <strong>Vorlage importieren:</strong> In Loxone Config unter <strong>Gerätevorlagen → Vorlage importieren</strong> die
                heruntergeladene XML-Datei laden.
              </li>
              <li>
                <strong>Vorlage ausführen:</strong> Den entsprechenden Bereich im Peripheriebaum markieren, dann
                unter <strong>Gerätevorlagen → Meine Vorlagen</strong> die importierte Vorlage auswählen und ausführen.
              </li>
            </ol>
            <div className="mt-3 space-y-1.5 text-xs text-yellow-200/80">
              <p>
                <strong>Virtual UDP Input:</strong> Im Peripheriebaum <strong>Virtuelle Eingänge</strong> markieren
                und sicherstellen, dass <strong>Vordefinierte UDP-Geräte</strong> ausgewählt ist.
              </p>
              <p>
                <strong>Virtual HTTP Output:</strong> Im Peripheriebaum <strong>Virtuelle Ausgänge</strong> markieren.
              </p>
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="bg-gray-900 rounded-lg p-4">
              <h4 className="font-medium text-hue-orange mb-2">Virtual UDP Input</h4>
              <p className="text-sm text-gray-400">
                Empfängt Status-Feedback (On/Off, Helligkeit, Farbtemperatur, Farbe) für gemappte Geräte.
                Erstellt automatisch passende Befehls-Erkennungen zum UDP-Feedback-Format.
              </p>
              <ul className="text-xs text-gray-500 mt-2 space-y-1">
                <li>• <strong>Gemappte Geräte</strong>: Nur Geräte mit aktivem Mapping</li>
                <li>• <strong>Alle HUE-Geräte</strong>: Alle erkannten Geräte</li>
              </ul>
            </div>
            <div className="bg-gray-900 rounded-lg p-4">
              <h4 className="font-medium text-hue-orange mb-2">Virtual HTTP Output</h4>
              <p className="text-sm text-gray-400">
                Erstellt Befehle zur Steuerung von HUE-Geräten aus Loxone.
                Mood-Mappings werden automatisch zu einem MOOD-Befehl pro Zielgruppe zusammengefasst.
              </p>
              <ul className="text-xs text-gray-500 mt-2 space-y-1">
                <li>• <strong>MOOD-Befehle</strong>: Für Lichtsteuerungs-Bausteine</li>
                <li>• <strong>SET BRI-Befehle</strong>: Für direkte Lichter/Gruppen</li>
              </ul>
            </div>
          </div>

          <div className="bg-hue-orange/10 border border-hue-orange/30 rounded-lg p-4">
            <h4 className="font-medium text-hue-orange mb-2">Empfohlener Workflow</h4>
            <ol className="text-sm text-orange-300 space-y-1 list-decimal list-inside">
              <li>Mappings im Tab "Mappings" erstellen</li>
              <li>Feedback (UDP/HTTP) in Einstellungen aktivieren</li>
              <li>XML-Vorlagen herunterladen (Einstellungen → Loxone Config Export)</li>
              <li>In Loxone Config importieren (Gerätevorlagen → Vorlage importieren)</li>
              <li>Bereich markieren (Virtuelle Eingänge bzw. Virtuelle Ausgänge), dann Vorlage ausführen (Gerätevorlagen → Meine Vorlagen)</li>
              <li>Loxone Config speichern und auf Miniserver übertragen</li>
            </ol>
          </div>
        </div>
      </section>

      {/* Troubleshooting */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <AlertTriangle size={20} className="text-yellow-500" />
          Troubleshooting
        </h3>
        <div className="space-y-4">
          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">Befehl wird nicht ausgeführt</h4>
            <ul className="text-sm text-gray-300 space-y-2">
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe die Gateway-IP-Adresse im Virtuellen Ausgang</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Stelle sicher, dass der Gateway-Container läuft (Port {port} erreichbar)</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Teste die URL direkt im Browser: <code className="bg-gray-800 px-1 rounded">http://{gatewayUrl}/ws?cmd=SET test ON</code></span>
              </li>
            </ul>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">"no mapping found" Fehler</h4>
            <ul className="text-sm text-gray-300 space-y-2">
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Die Loxone ID im Befehl stimmt nicht mit einem Mapping überein</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Bei MOOD: Prüfe, ob sowohl Gruppen-Mapping als auch Szenen-Mappings (<code className="bg-gray-800 px-1 rounded">*_mood_1</code>, <code className="bg-gray-800 px-1 rounded">*_mood_2</code>, ...) existieren</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe, ob das Mapping aktiviert ist (Enabled = true)</span>
              </li>
            </ul>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">HUE reagiert nicht</h4>
            <ul className="text-sm text-gray-300 space-y-2">
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe, ob die HUE Bridge verbunden ist (Dashboard: "Verbunden")</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Teste die HUE-Steuerung direkt über das Dashboard (Geräte/Räume)</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe die HUE Bridge IP in der Konfiguration</span>
              </li>
            </ul>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">Feedback (UDP/HTTP) kommt nicht an</h4>
            <ul className="text-sm text-gray-300 space-y-2">
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe, ob UDP- und/oder HTTP-Feedback in den Einstellungen aktiviert ist</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Für UDP: Prüfe die Ziel-IP und den Port (Standard: 7777)</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Für HTTP: Prüfe die Ziel-URL und die Zugangsdaten</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe, ob das gewünschte Protokoll im Mapping aktiviert ist (UDP/HTTP Checkboxen)</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Stelle sicher, dass ein Mapping für das Gerät existiert oder <strong>Alle Geräte senden</strong> aktiviert ist</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Für UDP: Prüfe die Firewall — UDP-Port muss am Miniserver offen sein</span>
              </li>
            </ul>
          </div>

          <div className="bg-gray-900 rounded-lg p-4">
            <h4 className="font-medium text-yellow-400 mb-2">Helligkeit/MOOD-Wert wird nicht übertragen</h4>
            <ul className="text-sm text-gray-300 space-y-2">
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Stelle sicher, dass <code className="bg-gray-800 px-1 rounded">&lt;v&gt;</code> im Befehl enthalten ist (wird durch den Wert ersetzt)</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Der Virtuelle Ausgang Befehl muss als "Analog" konfiguriert sein</span>
              </li>
              <li className="flex items-start gap-2">
                <HelpCircle size={16} className="text-gray-500 mt-0.5 flex-shrink-0" />
                <span>Prüfe den Wertebereich: Helligkeit 0-100, MOOD 0-9</span>
              </li>
            </ul>
          </div>
        </div>
      </section>

      {/* Checkliste */}
      <section className="bg-gray-800 rounded-xl p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <CheckCircle2 size={20} className="text-hue-orange" />
          Checkliste
        </h3>
        <ul className="space-y-3 text-gray-300">
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>HUE Bridge im Gateway gepairt (grüner Status "Verbunden")</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Mappings im Tab "Mappings" erstellt</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Gateway-IP notiert (Docker Host oder Container IP)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Feedback aktiviert (Einstellungen: UDP und/oder HTTP konfiguriert)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>XML-Vorlagen exportiert (Einstellungen → Loxone Config Export)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>XML in Loxone Config importiert (Gerätevorlagen → Vorlage importieren)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Vorlage ausgeführt (Bereich markieren → Gerätevorlagen → Meine Vorlagen)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Loxone Config gespeichert und auf Miniserver übertragen</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Test: URL im Browser testen (Mappings → Test URLs)</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Test: Licht über Loxone schalten</span>
          </li>
          <li className="flex items-start gap-3">
            <div className="w-5 h-5 rounded border-2 border-gray-600 flex-shrink-0 mt-0.5"></div>
            <span>Test: Sensor-Feedback prüfen (Logs → UDP/HTTP Filter)</span>
          </li>
        </ul>
      </section>

      {/* Quick Reference Card */}
      <section className="bg-gradient-to-br from-hue-orange/20 to-orange-600/10 rounded-xl p-6 border border-hue-orange/30">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Zap size={20} className="text-hue-orange" />
          Schnellreferenz
        </h3>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Licht Ein/Aus</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=SET id ON|OFF</code>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Helligkeit</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=SET id BRI &lt;v&gt;</code>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Szene</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=SCENE id</code>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Lichtsteuerung (MOOD)</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=MOOD id &lt;v&gt;</code>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Farbtemperatur</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=SET id CT &lt;v&gt;</code>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-3">
            <h4 className="text-sm font-medium text-hue-orange mb-2">Farbe</h4>
            <code className="text-xs text-gray-300 font-mono">/ws?cmd=SET id COLOR #hex</code>
          </div>
        </div>
        <div className="mt-4">
          <h4 className="text-sm font-medium text-white mb-3">Sensor-Feedback (automatisch)</h4>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="bg-gray-900/50 rounded-lg p-3">
              <h4 className="text-sm font-medium text-hue-orange mb-1">UDP-Format</h4>
              <code className="text-xs text-gray-300 font-mono">id/eigenschaft:wert</code>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3">
              <h4 className="text-sm font-medium text-hue-orange mb-1">HTTP-Format</h4>
              <code className="text-xs text-gray-300 font-mono">/dev/sps/io/id_eigenschaft/wert</code>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
