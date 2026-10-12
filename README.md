# Loxone2HUE Gateway

Ein Gateway Service zur bidirektionalen Steuerung von Philips HUE Geräten über Loxone.

## Features

- **Bidirektionale WebSocket-Kommunikation** zwischen Loxone und HUE
- **Echtzeit-Status-Updates** via HUE SSE Event Stream
- **UDP Status-Feedback** — automatische Rückmeldung an Loxone Miniserver per UDP
- **Loxone Config Export** — XML-Vorlagen (Virtual UDP Input / Virtual HTTP Output) zum direkten Import
- **Web-Frontend** zur Visualisierung und Steuerung
- **Mapping-Konfiguration** für Loxone ↔ HUE Zuordnungen
- **Docker-Deployment** für einfache Installation

## Architektur

```
┌─────────────┐     WebSocket      ┌─────────────────────┐     HUE API v2
│   Loxone    │◄──────────────────►│   Gateway (Go)      │◄───────────────►│ HUE Bridge │
│ Miniserver  │                    │   Port 8080         │
└─────────────┘                    └─────────────────────┘
                                            ▲
┌─────────────┐     HTTP/WebSocket          │
│  Frontend   │◄────────────────────────────┘
│   Browser   │
└─────────────┘
```

## Installation

### Home Assistant Add-on (empfohlen)

Das einfachste Setup ist als Home Assistant Add-on:

1. **Repository hinzufügen:**
   - Gehe zu **Einstellungen** → **Add-ons** → **Add-on Store**
   - Klicke auf das **⋮** Menü (oben rechts) → **Repositories**
   - Füge hinzu: `https://github.com/stefanbeyeler/Loxone2HUE`

2. **Add-on installieren:**
   - Suche nach "Loxone2HUE Gateway"
   - Klicke auf **Installieren**
   - Warte bis die Installation abgeschlossen ist

3. **Add-on starten:**
   - Aktiviere **Beim Start ausführen** (optional)
   - Klicke auf **Starten**
   - Öffne das **Web UI** über den Seitenbereich

4. **HUE Bridge verbinden:**
   - Die Bridge wird automatisch erkannt
   - Drücke den **Link-Button** auf der HUE Bridge
   - Klicke auf **Verbinden** im Web UI

Weitere Details: [Add-on Dokumentation](loxone2hue/DOCS.md)

### Mit Docker

1. Repository klonen:
```bash
git clone https://github.com/sbeyeler/loxone2hue.git
cd loxone2hue
```

2. Konfiguration erstellen:
```bash
cp configs/config.example.yaml configs/config.yaml
```

3. Container starten:
```bash
docker-compose up -d
```

4. Frontend öffnen: http://localhost:8080

### Manuell

**Voraussetzungen:**
- Go 1.27+
- Node.js 22+

1. Backend bauen:
```bash
go build -o gateway ./cmd/gateway
```

2. Frontend bauen:
```bash
cd web
npm install
npm run build
cd ..
```

3. Starten:
```bash
./gateway -config config.yaml
```

## Konfiguration

### config.yaml

```yaml
server:
  port: 8080
  host: "0.0.0.0"

hue:
  bridge_ip: ""           # Leer für Auto-Discovery
  application_key: ""     # Wird beim Pairing gesetzt

loxone:
  enabled: true
  miniserver_ip: ""       # Optional
  udp_feedback:
    enabled: false        # UDP Status-Feedback aktivieren (nutzt miniserver_ip als Ziel)
    port: 7777            # UDP Port

logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json oder console

mappings: []              # Über Frontend konfigurierbar
```

## Loxone Integration

### WebSocket-Verbindung

Loxone verbindet sich per WebSocket mit dem Gateway:

```
ws://gateway-ip:8080/ws?type=loxone&id=miniserver1
```

### Command-Format (JSON)

```json
{
  "type": "command",
  "target": "wohnzimmer_decke",
  "action": "set",
  "params": {
    "on": true,
    "brightness": 80
  }
}
```

### Unterstützte Actions

| Action | Parameter | Beschreibung |
|--------|-----------|--------------|
| `set` | `on` (bool) | Ein/Aus |
| `set` | `brightness` (0-100) | Helligkeit |
| `set` | `color` (hex) | Farbe z.B. "#FF5500" |
| `set` | `color_temp` (2000-6500) | Farbtemperatur in Kelvin |
| `scene` | `scene_id` (string) | Szene aktivieren |

### UDP Status-Feedback

Der Gateway kann HUE-Statusänderungen automatisch per UDP an den Loxone Miniserver senden. Aktivierung über die Web UI unter **Einstellungen**.

Format pro Eigenschaftsänderung:

```text
<loxone_id>/<eigenschaft>:<wert>
```

Beispiele: `buero_stefan/on:1`, `buero_stefan/bri:80`, `buero_stefan/ct:250`

### Loxone Config Export

XML-Vorlagen für den Import in Loxone Config:

- **Virtual UDP Input** (`GET /api/export/inputs`): Empfängt Status-Feedback
- **Virtual HTTP Output** (`GET /api/export/outputs`): Steuert HUE-Geräte

Download über die Web UI unter **Einstellungen → Loxone Config Export**.

**Import in Loxone Config (2 Schritte):**

1. **Vorlage importieren:** In Loxone Config unter **Gerätevorlagen → Vorlage importieren** die XML-Datei laden.
2. **Vorlage ausführen:** Den entsprechenden Bereich im Peripheriebaum markieren, dann unter **Gerätevorlagen → Meine Vorlagen** die importierte Vorlage auswählen und ausführen.
   - **Virtual UDP Input:** *Virtuelle Eingänge* markieren und sicherstellen, dass *Vordefinierte UDP-Geräte* ausgewählt ist.
   - **Virtual HTTP Output:** *Virtuelle Ausgänge* markieren.

### Status-Updates

Der Gateway sendet automatisch Status-Updates an verbundene Clients:

```json
{
  "type": "status",
  "device": "wohnzimmer_decke",
  "state": {
    "on": true,
    "brightness": 80,
    "reachable": true
  }
}
```

### Loxone Virtual Output Beispiel

In Loxone Config:

1. **Virtual Output** erstellen
2. **Address**: `ws://gateway-ip:8080/ws?type=loxone`
3. **Command für Ein**: `{"type":"command","target":"<MAPPING_ID>","action":"set","params":{"on":true}}`
4. **Command für Aus**: `{"type":"command","target":"<MAPPING_ID>","action":"set","params":{"on":false}}`

Für Dimming kann ein Virtual Output mit analogem Wert verwendet werden.

## API Endpoints

| Methode | Endpoint | Beschreibung |
|---------|----------|--------------|
| GET | `/api/health` | Health Check |
| GET | `/api/bridge` | Bridge-Info |
| GET | `/api/bridge/discover` | Bridges suchen |
| POST | `/api/bridge/pair` | Bridge pairen |
| GET | `/api/devices` | Alle Lichter |
| PUT | `/api/devices/{id}` | Licht steuern |
| GET | `/api/groups` | Alle Gruppen |
| PUT | `/api/groups/{id}` | Gruppe steuern |
| GET | `/api/scenes` | Alle Szenen |
| POST | `/api/scenes/{id}/activate` | Szene aktivieren |
| GET | `/api/mappings` | Alle Mappings |
| POST | `/api/mappings` | Mapping erstellen |
| PUT | `/api/mappings/{id}` | Mapping aktualisieren |
| DELETE | `/api/mappings/{id}` | Mapping löschen |
| GET | `/api/config` | Konfiguration abrufen |
| PUT | `/api/config` | Konfiguration aktualisieren |
| GET | `/api/export/inputs` | Loxone Virtual UDP Input XML |
| GET | `/api/export/outputs` | Loxone Virtual HTTP Output XML |

## Entwicklung

### Backend

```bash
go run ./cmd/gateway -config configs/config.yaml
```

### Frontend (mit Hot-Reload)

```bash
cd web
npm run dev
```

Das Frontend läuft auf Port 3000 und proxied API-Requests zu Port 8080.

## Troubleshooting

### Bridge wird nicht gefunden

- Prüfe, ob die Bridge im gleichen Netzwerk ist
- Für Docker: Verwende `network_mode: host` für mDNS-Discovery
- Oder gib die Bridge-IP manuell ein

### Pairing funktioniert nicht

- Drücke den Link-Button auf der HUE Bridge
- Versuche das Pairing innerhalb von 30 Sekunden

### WebSocket-Verbindung bricht ab

- Prüfe Firewall-Einstellungen
- Stelle sicher, dass Port 8080 erreichbar ist

## Lizenz

MIT
