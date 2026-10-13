# Loxone2HUE Gateway

Ein Gateway Service zur bidirektionalen Steuerung von Philips HUE Geräten über Loxone.

## Features

- **Loxone-Befehle per HTTP** — Virtuelle Ausgänge in Loxone Config rufen `GET /ws?cmd=…` auf
- **Status-Feedback an Loxone** per UDP (Virtueller UDP-Eingang) und/oder HTTP (Virtueller HTTP-Eingang), pro Miniserver und pro Mapping wählbar
- **Echtzeit-Status-Updates** via HUE SSE Event Stream, in der Web-UI live über WebSocket
- **Sensoren** — Bewegung, Temperatur, Helligkeit, Taster, Kontakt und Batterie, inklusive der von der Bridge pro Raum zusammengefassten Bewegung und Helligkeit (Hue Bridge Pro MotionAware)
- **Loxone Config Export** — XML-Vorlagen (Virtual UDP Input / Virtual HTTP Output) zum direkten Import
- **Web-Frontend** zur Visualisierung, Steuerung und Konfiguration
- **Mapping-Konfiguration** für Loxone ↔ HUE Zuordnungen, inklusive Mood-Mappings für den Lichtsteuerungs-Baustein
- **Backups** der gesamten Konfiguration, optional **Login-Schutz** für Web-UI und API
- **Home Assistant Add-on** (vorgebautes Image) oder **Docker**

## Architektur

```
┌─────────────┐  GET /ws?cmd=SET id ON   ┌─────────────────────┐   CLIP API v2 / SSE   ┌────────────┐
│   Loxone    │ ───────────────────────► │   Gateway (Go)      │ ◄──────────────────► │ HUE Bridge │
│ Miniserver  │ ◄─────────────────────── │   Port 8080         │                      └────────────┘
└─────────────┘  UDP / HTTP Feedback     └─────────────────────┘
                 id/on:1, id/bri:80                ▲
                                                   │ HTTP API + WebSocket (Live-Status)
┌─────────────┐                                    │
│  Web-UI     │ ◄──────────────────────────────────┘
│   Browser   │
└─────────────┘
```

## Installation

### Home Assistant Add-on (empfohlen)

Das einfachste Setup ist als Home Assistant Add-on. Das Add-on wird als fertiges Image von GitHub Container Registry geladen (`ghcr.io/aes-concepts-gmbh/loxone2hue-{arch}`), auf dem Home-Assistant-Host wird nichts gebaut.

1. **Repository hinzufügen:**
   - Gehe zu **Einstellungen** → **Add-ons** → **Add-on Store**
   - Klicke auf das **⋮** Menü (oben rechts) → **Repositories**
   - Füge hinzu: `https://github.com/AES-Concepts-GmbH/loxone2hue`

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
git clone https://github.com/AES-Concepts-GmbH/loxone2hue.git
cd loxone2hue
```

2. Konfiguration erstellen:
```bash
cp configs/config.example.yaml configs/config.yaml
```

3. Container starten:
```bash
docker compose up -d
```

4. Frontend öffnen: http://localhost:8080

Für Synology NAS siehe [synology/README.md](synology/README.md).

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
npm ci
npm run build
cd ..
```

3. Starten:
```bash
./gateway -config configs/config.yaml
```

## Konfiguration

Die Konfiguration liegt in `configs/config.yaml` (Vorlage: [configs/config.example.yaml](configs/config.example.yaml)). Miniserver und Mappings werden normalerweise über die Web-UI gepflegt.

```yaml
server:
  port: 8080
  host: "0.0.0.0"

auth:
  username: "admin"
  password: ""            # Leer = kein Login (Standard, nötig für HA-Ingress)

hue:
  bridge_ip: ""           # Leer für Auto-Discovery
  application_key: ""     # Wird beim Pairing gesetzt

loxone:
  miniservers:            # Ein Eintrag pro Miniserver
    - id: "generated-uuid"
      name: "Miniserver"
      ip: "192.168.1.10"
      port: 7777          # UDP-Zielport
      udp_enabled: true   # Status-Feedback per UDP
      http_enabled: false # Status-Feedback per HTTP (Virtueller HTTP-Eingang)
      http_url: ""        # Optional, Standard http://<ip>
      http_user: ""
      http_password: ""
      send_all: false     # Auch Geräte ohne Mapping melden

logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json oder console

mappings: []              # Über die Web-UI konfigurierbar
```

### Login-Schutz

Mit gesetztem `auth.password` verlangen Web-UI, API und WebSocket einen Login (HTTP Basic Auth, die Web-UI setzt danach ein Session-Cookie). Loxone-Befehle übergeben die Zugangsdaten in der Adresse des Virtuellen Ausgangs: `http://admin:passwort@gateway-ip:8080`. `GET /api/health` bleibt für den Container-Healthcheck offen. Im Home-Assistant-Add-on ist der Login standardmässig aus, weil Home Assistant den Zugang über Ingress bereits schützt.

## Loxone Integration

### Befehle: Virtueller Ausgang in Loxone Config

Loxone sendet Befehle als HTTP-GET an den Gateway. In Loxone Config:

1. **Virtueller Ausgang** anlegen, Adresse: `http://gateway-ip:8080`
2. **Virtuelle Ausgang-Befehle** anlegen, zum Beispiel:

| Zweck | Befehl bei EIN | Befehl bei AUS | Analog |
|---|---|---|---|
| Schalten | `/ws?cmd=SET <loxone_id> ON` | `/ws?cmd=SET <loxone_id> OFF` | nein |
| Dimmen | `/ws?cmd=SET <loxone_id> BRI <v>` | leer | ja |
| Lichtsteuerung (Mood) | `/ws?cmd=MOOD <loxone_id> <v>` | leer | ja |

Beim Dimmen schaltet `BRI 0` aus und jeder Wert über 0 ein. Die fertige Vorlage mit allen Mappings liefert der Export (siehe unten).

### Befehlsreferenz

| Befehl | Beschreibung | Beispiel |
|--------|--------------|----------|
| `SET <id> ON` | Licht/Gruppe einschalten | `SET wz_decke ON` |
| `SET <id> OFF` | Licht/Gruppe ausschalten | `SET wz_decke OFF` |
| `SET <id> BRI <0-100>` | Helligkeit, 0 = aus | `SET wz_decke BRI 75` |
| `SET <id> CT <2000-6500>` | Farbtemperatur in Kelvin (oder Mirek) | `SET wz_decke CT 4000` |
| `SET <id> COLOR <#hex>` | Farbe | `SET wz_decke COLOR #FF5500` |
| `SCENE <id>` | Szene über Szenen-Mapping aktivieren | `SCENE abend` |
| `MOOD <id> <n>` | Lichtsteuerung: 0 = aus, n = n-te Szene | `MOOD wohnzimmer 2` |
| `GET <id> STATUS` | Lichtzustand abfragen | `GET wz_decke STATUS` |

`<id>` ist die Loxone-ID eines Mappings. Unbekannte IDs werden mit 404 beantwortet; eine rohe HUE-Ressourcen-ID (UUID) ist weiterhin erlaubt.

Dieselben Befehle funktionieren über den WebSocket `/ws`, als Text oder als JSON `{"type":"command","target":"<id>","action":"set","params":{"on":true,"brightness":80}}`.

### Status-Feedback an Loxone

Der Gateway meldet HUE-Statusänderungen pro Eigenschaft an die konfigurierten Miniserver, per UDP und/oder HTTP (pro Miniserver in den Einstellungen, pro Mapping einstellbar).

| Kanal | Format | Loxone-Seite |
|---|---|---|
| UDP | `<loxone_id>/<eigenschaft>:<wert>` | Virtueller UDP-Eingang, Port aus den Miniserver-Einstellungen |
| HTTP | `GET /dev/sps/io/<loxone_id>_<eigenschaft>/<wert>` | Virtueller HTTP-Eingang `<loxone_id>_<eigenschaft>` |

Eigenschaften für Lichter und Gruppen: `on` (0/1), `bri` (0–100), `ct` (Mirek), `color_x`, `color_y`. Gruppen melden den von der Bridge zusammengefassten Raumzustand.

Eigenschaften für Sensoren: `motion` (0/1), `temperature`, `light_level` (Hue-Rohwert), `lux`, `button` (0–4), `contact` (0/1), `rotary` (Schritte), `battery` (0–100).

Ein nicht erreichbarer Miniserver bremst die Verarbeitung nicht: Feedback läuft in einem eigenen Worker, Ausfälle werden gezählt und unter `/api/health` ausgewiesen.

### Loxone Config Export

XML-Vorlagen für den Import in Loxone Config:

- **Virtual UDP Input** (`GET /api/export/inputs`): Empfängt Status-Feedback
- **Virtual HTTP Output** (`GET /api/export/outputs`): Steuert HUE-Geräte (BRI-, Ein/Aus- und Mood-Befehle)

Download über die Web UI unter **Einstellungen → Loxone Config Export**.

**Import in Loxone Config (2 Schritte):**

1. **Vorlage importieren:** In Loxone Config unter **Gerätevorlagen → Vorlage importieren** die XML-Datei laden.
2. **Vorlage ausführen:** Den entsprechenden Bereich im Peripheriebaum markieren, dann unter **Gerätevorlagen → Meine Vorlagen** die importierte Vorlage auswählen und ausführen.
   - **Virtual UDP Input:** *Virtuelle Eingänge* markieren und sicherstellen, dass *Vordefinierte UDP-Geräte* ausgewählt ist.
   - **Virtual HTTP Output:** *Virtuelle Ausgänge* markieren.

## API

Die vollständige, ausführbare Beschreibung liegt unter `GET /api/swagger` (Swagger UI) und `GET /api/swagger.json`. Alle Endpunkte ausser `/api/health` verlangen bei aktiviertem Login HTTP Basic Auth oder das Session-Cookie der Web-UI.

| Methode | Endpoint | Beschreibung |
|---------|----------|--------------|
| GET | `/api/health` | Health Check, inkl. Zähler verworfener Events |
| GET | `/api/bridge` | Bridge-Info |
| GET | `/api/bridge/discover` | Bridges im Netz suchen |
| POST | `/api/bridge/pair` | Bridge pairen |
| POST | `/api/bridge/test` | Erreichbarkeit einer Bridge prüfen |
| GET | `/api/devices` | Alle Lichter |
| GET, PUT | `/api/devices/{id}` | Licht lesen / steuern |
| GET | `/api/groups` | Alle Räume und Zonen |
| GET, PUT | `/api/groups/{id}` | Gruppe lesen / steuern |
| GET | `/api/scenes` | Alle Szenen |
| POST | `/api/scenes/{id}/activate` | Szene aktivieren |
| GET | `/api/sensors` | Alle Sensoren (Gerät, Raum, Zuhause) |
| GET, POST | `/api/mappings` | Mappings lesen / erstellen |
| PUT, DELETE | `/api/mappings/{id}` | Mapping aktualisieren / löschen |
| GET | `/api/mappings/export` | Mappings als JSON exportieren |
| POST | `/api/mappings/import` | Mappings importieren (replace/merge) |
| GET, PUT | `/api/config` | Konfiguration lesen / aktualisieren (Passwörter maskiert) |
| GET, POST | `/api/backups` | Backups auflisten (Metadaten) / erstellen |
| POST | `/api/backups/upload` | Backup-Datei hochladen |
| DELETE | `/api/backups/{id}` | Backup löschen |
| GET | `/api/backups/{id}/download` | Backup herunterladen (vollständig, enthält Secrets) |
| POST | `/api/backups/{id}/restore` | Backup wiederherstellen |
| POST | `/api/udp/test` | Test-UDP-Meldung an einen Miniserver senden |
| GET | `/api/export/inputs` | Loxone Virtual UDP Input XML |
| GET | `/api/export/outputs` | Loxone Virtual HTTP Output XML |
| GET | `/api/logs` | Server-Log (Ringpuffer), Filter `level`, `search`, `limit` |
| GET | `/ws?cmd=…` | Loxone-Befehl ausführen |
| WS | `/ws` | WebSocket: Befehle und Live-Status (`status`, `group_update`, `sensor_update`) |

## Entwicklung

### Backend

```bash
go run ./cmd/gateway -config configs/config.yaml
go test -race ./...
```

### Frontend (mit Hot-Reload)

```bash
cd web
npm run dev
```

Das Frontend läuft auf Port 3000 und proxied API-Requests zu Port 8080.

### CI

Jeder Push und Pull Request läuft durch `gofmt`, `go vet`, Tests mit Race-Detector, `govulncheck`, Lint/Typecheck/Build der Web-UI, einen Versionsabgleich und einen Build des Add-on-Images. Ein Push auf `main` veröffentlicht die Add-on-Images nach GHCR.

## Troubleshooting

### Bridge wird nicht gefunden

- Prüfe, ob die Bridge im gleichen Netzwerk ist
- Für Docker: Verwende `network_mode: host` für mDNS-Discovery
- Oder gib die Bridge-IP manuell ein

### Pairing funktioniert nicht

- Drücke den Link-Button auf der HUE Bridge
- Versuche das Pairing innerhalb von 30 Sekunden

### Loxone erreicht den Gateway nicht

- Prüfe Firewall-Einstellungen und ob Port 8080 erreichbar ist
- Teste den Befehl im Browser: `http://gateway-ip:8080/ws?cmd=SET <loxone_id> ON` liefert JSON mit `status: ok` oder einer Fehlermeldung
- Bei aktiviertem Login die Zugangsdaten in die Adresse des Virtuellen Ausgangs aufnehmen

### Kein Feedback in Loxone

- Unter **Einstellungen** prüfen, ob UDP bzw. HTTP für den Miniserver aktiv ist
- Server-Log in der Web-UI (Filter UDP) zeigt jede gesendete Meldung
- `GET /api/health` zeigt `dropped_feedback`, falls Meldungen verworfen wurden

## Lizenz

[MIT](LICENSE)
