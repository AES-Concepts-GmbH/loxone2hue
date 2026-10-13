# Loxone2HUE Gateway für Synology NAS

Standalone Docker-Deployment ohne Home Assistant Abhängigkeiten.

## Schnellstart

### Option A: Vorgebautes Image importieren

1. **Image importieren:**
   ```bash
   docker load -i loxone2hue.tar
   ```

2. **Container starten:**
   ```bash
   docker run -d \
     --name loxone2hue \
     --network host \
     -v loxone2hue-data:/data \
     --restart unless-stopped \
     loxone2hue:latest
   ```

### Option B: Aus Quellcode bauen

1. **Repository klonen:**
   ```bash
   git clone https://github.com/AES-Concepts-GmbH/loxone2hue.git
   cd loxone2hue/synology
   ```

2. **Container bauen und starten:**
   ```bash
   docker-compose up -d --build
   ```

## Zugriff

- **Web-UI:** `http://<NAS-IP>:8080`
- **API:** `http://<NAS-IP>:8080/api/`
- **Health Check:** `http://<NAS-IP>:8080/api/health`

## Konfiguration

Die Konfiguration wird persistent im Docker-Volume `loxone2hue-data` gespeichert.

Nach dem ersten Start:
1. Web-UI öffnen
2. HUE Bridge IP eingeben
3. Link-Button auf der Bridge drücken
4. "Verbinden" klicken

## Loxone Anbindung

### Virtueller Ausgang

In Loxone Config einen **Virtuellen Ausgang** erstellen:
- **Adresse:** `http://<NAS-IP>:8080`

### Befehle (Virtueller Ausgang Befehl)

Befehle werden als HTTP GET an den WebSocket-Endpoint gesendet:

| Aktion | Befehl |
|--------|--------|
| Licht/Gruppe EIN | `/ws?cmd=SET%20<LoxoneID>%20ON` |
| Licht/Gruppe AUS | `/ws?cmd=SET%20<LoxoneID>%20OFF` |
| Helligkeit | `/ws?cmd=SET%20<LoxoneID>%20BRI%20<v>` |
| Farbtemperatur | `/ws?cmd=SET%20<LoxoneID>%20CT%20<v>` |
| Farbe (Hex) | `/ws?cmd=SET%20<LoxoneID>%20COLOR%20%23FF5500` |
| Szene aktivieren | `/ws?cmd=SCENE%20<LoxoneID>` |
| Mood (Lichtsteuerung) | `/ws?cmd=MOOD%20<LoxoneID>%20<v>` |

> `<LoxoneID>` = Loxone ID aus dem Mapping, `<v>` = Analogwert vom Baustein

### Beispiel: Licht steuern

1. Mapping im Web-UI erstellen (z.B. `Buero_Stefan` → HUE Licht)
2. Virtueller Ausgang in Loxone Config:
   - Befehl bei EIN: `/ws?cmd=SET%20Buero_Stefan%20ON`
   - Befehl bei AUS: `/ws?cmd=SET%20Buero_Stefan%20OFF`
3. Für Dimmer (Analogwert): `/ws?cmd=SET%20Buero_Stefan%20BRI%20<v>`

### Beispiel: Lichtsteuerungs-Baustein (MOOD)

1. Gruppen-Mapping erstellen (z.B. `Wohnzimmer` → HUE Raum)
2. Szenen-Mappings erstellen: `Wohnzimmer_mood_1`, `Wohnzimmer_mood_2`, etc.
3. AQ-Ausgang des Lichtsteuerungs-Bausteins → Virtueller Ausgang:
   - Befehl: `/ws?cmd=MOOD%20Wohnzimmer%20<v>`
   - MOOD 0 = Aus, MOOD 1 = Szene 1, MOOD 2 = Szene 2, etc.

## UDP Status-Feedback

Der Gateway kann Status-Änderungen von HUE-Geräten automatisch per UDP an den Loxone Miniserver senden.

1. Web-UI öffnen → Tab **Einstellungen**
2. **UDP Feedback** aktivieren
3. Miniserver-IP und UDP-Port (Standard: 7777) eingeben
4. Speichern

Format: `<loxone_id>/<eigenschaft>:<wert>` (z.B. `Buero_Stefan/on:1`, `Buero_Stefan/bri:80`)

Um die UDP-Pakete in Loxone zu empfangen, unter **Einstellungen → Loxone Config Export** die XML-Vorlage für **Virtual UDP Input** herunterladen und in Loxone Config importieren (siehe Import-Anleitung unten).

## Loxone Config Export

XML-Vorlagen für den Import in Loxone Config:

| Export | URL | Beschreibung |
| ------ | --- | ------------ |
| Virtual UDP Input | `/api/export/inputs` | Status-Feedback empfangen |
| Virtual UDP Input (alle) | `/api/export/inputs?all=true` | Alle HUE-Geräte |
| Virtual HTTP Output | `/api/export/outputs` | HUE-Geräte steuern |

Download über die Web-UI unter **Einstellungen → Loxone Config Export**.

**Import in Loxone Config (2 Schritte):**

1. **Vorlage importieren:** In Loxone Config unter **Gerätevorlagen → Vorlage importieren** die XML-Datei laden.
2. **Vorlage ausführen:** Den entsprechenden Bereich im Peripheriebaum markieren, dann unter **Gerätevorlagen → Meine Vorlagen** die importierte Vorlage auswählen und ausführen.
   - **Virtual UDP Input:** *Virtuelle Eingänge* markieren und sicherstellen, dass *Vordefinierte UDP-Geräte* ausgewählt ist.
   - **Virtual HTTP Output:** *Virtuelle Ausgänge* markieren.

## Container Manager (DSM 7.2+)

1. **Container Manager** → **Projekt** → **Erstellen**
2. Projektname: `loxone2hue`
3. Pfad: Ordner mit diesen Dateien auswählen
4. Quelle: `docker-compose.yml` verwenden

## Logs anzeigen

```bash
docker logs -f loxone2hue
```

## Update

```bash
cd Loxone2HUE
git pull
cd synology
docker-compose up -d --build
```

## Ports

| Port | Beschreibung |
|------|--------------|
| 8080 | Web-UI, API & WebSocket |
