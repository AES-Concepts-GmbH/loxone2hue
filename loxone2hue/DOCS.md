# Loxone2HUE Gateway

Gateway Service für die bidirektionale Kommunikation zwischen Loxone Miniserver und Philips HUE Bridge.

## Funktionen

- **HUE Bridge Discovery**: Automatische Erkennung der HUE Bridge im Netzwerk
- **Gerätesteuerung**: Ein/Aus, Helligkeit, Farbe, Farbtemperatur
- **Gruppen & Zonen**: Räume und Zonen gemeinsam steuern
- **Szenen**: HUE Szenen über Loxone aktivieren
- **Echtzeit-Updates**: Statusänderungen der Bridge live in der Web UI und als Feedback an Loxone
- **UDP Status-Feedback**: Automatische Status-Rückmeldung an Loxone per UDP
- **Sensoren**: Bewegung, Temperatur, Helligkeit, Taster, Kontakt, Batterie sowie Raum-Bewegung und Raum-Helligkeit der Bridge (Hue Bridge Pro MotionAware)
- **Loxone Config Export**: XML-Vorlagen für Virtual UDP Input und Virtual HTTP Output
- **Web UI**: Übersichtliches Dashboard zur Konfiguration

## Erste Schritte

### 1. Add-on starten

Starte das Add-on und warte, bis es bereit ist.

Das Add-on lädt ein fertiges Image von `ghcr.io/aes-concepts-gmbh/loxone2hue-{arch}`. Bricht die Installation mit einem Pull-Fehler (403/denied) ab, sind die Pakete in der GitHub-Organisation noch nicht auf Public gestellt.

### 2. HUE Bridge verbinden

1. Öffne die Web UI
2. Die Bridge wird automatisch erkannt
3. Drücke den **Link-Button** auf deiner HUE Bridge
4. Klicke auf "Bridge verbinden"
5. Das Pairing ist abgeschlossen

### 3. Mappings erstellen

Mappings verbinden Loxone IDs mit HUE Geräten:

1. Gehe zum Tab "Mappings"
2. Klicke auf "Neu"
3. Wähle:
   - **Name**: Beschreibender Name
   - **Loxone ID**: ID aus Loxone Config
   - **HUE Typ**: light, group oder scene
   - **HUE Gerät**: Das zu steuernde Gerät

### 4. Login-Schutz setzen (empfohlen)

Das Add-on läuft mit `host_network`, die Web-UI und die API sind damit auf Port 8080 im ganzen LAN erreichbar, nicht nur über Ingress. Setze unter **Einstellungen → Login-Schutz** ein Passwort; die Web-UI zeigt bis dahin einen Warnhinweis. Loxone übergibt die Zugangsdaten danach in der Adresse des Virtuellen Ausgangs: `http://admin:<passwort>@<home-assistant-ip>:8080`.

### 5. Loxone konfigurieren

Erstelle in Loxone Config:

1. **Virtuellen Ausgang**:
   - Adresse: `http://<home-assistant-ip>:8080`

2. **Virtuelle Ausgang-Befehle**:
   ```
   Ein: /ws?cmd=SET <mapping-id> ON
   Aus: /ws?cmd=SET <mapping-id> OFF
   Helligkeit: /ws?cmd=SET <mapping-id> BRI <v>
   ```

## Befehlsreferenz

| Befehl | Beschreibung | Beispiel |
|--------|--------------|----------|
| `SET id ON` | Gerät einschalten | `SET licht1 ON` |
| `SET id OFF` | Gerät ausschalten | `SET licht1 OFF` |
| `SET id BRI x` | Helligkeit (0-100) | `SET licht1 BRI 75` |
| `SET id COLOR #hex` | RGB Farbe | `SET licht1 COLOR #FF5500` |
| `SET id CT x` | Farbtemperatur (2000-6500K) | `SET licht1 CT 4000` |
| `SET id SCENE x` | Szene aktivieren | `SET wohnzimmer SCENE 1` |
| `MOOD id x` | Stimmung aktivieren (Lichtsteuerung) | `MOOD wohnzimmer 1` |
| `GET id STATUS` | Status abfragen | `GET wz_decke STATUS` |

## UDP Status-Feedback

Der Gateway kann Status-Änderungen von HUE-Geräten automatisch per UDP an den Loxone Miniserver senden. So erhält Loxone Echtzeit-Rückmeldungen über den Zustand der Lampen.

### Aktivierung

1. Gehe zum Tab **Einstellungen** in der Web UI
2. Aktiviere **UDP Feedback**
3. Gib die **Miniserver-IP** und den **UDP-Port** ein (Standard: 7777)
4. Klicke auf **Speichern**

### UDP-Nachrichtenformat

Pro Eigenschaftsänderung wird ein UDP-Paket gesendet:

```text
<loxone_id>/<eigenschaft>:<wert>
```

| Eigenschaft | Wertebereich | Beispiel |
| ----------- | ------------ | -------- |
| `on` | 0/1 | `buero_stefan/on:1` |
| `bri` | 0-100 | `buero_stefan/bri:80` |
| `ct` | 153-500 (Mirek) | `buero_stefan/ct:250` |
| `color_x` | 0-1 (Float) | `buero_stefan/color_x:0.4573` |
| `color_y` | 0-1 (Float) | `buero_stefan/color_y:0.4100` |

### Loxone Config: Virtual UDP Input

Um die UDP-Pakete in Loxone zu empfangen, wird ein **Virtueller UDP Eingang** benötigt. Dieser kann automatisch exportiert werden:

1. Gehe zum Tab **Einstellungen** → **Loxone Config Export**
2. Klicke auf **Gemappte Geräte** (oder **Alle HUE-Geräte**) um die XML-Datei herunterzuladen
3. **Vorlage importieren:** In Loxone Config unter **Gerätevorlagen → Vorlage importieren** die XML-Datei laden
4. **Vorlage ausführen:** Im Peripheriebaum **Virtuelle Eingänge** markieren (sicherstellen, dass **Vordefinierte UDP-Geräte** ausgewählt ist), dann unter **Gerätevorlagen → Meine Vorlagen** die importierte Vorlage auswählen und ausführen

## Loxone Config Export

XML-Vorlagen können direkt zum Import in Loxone Config heruntergeladen werden:

### Virtual UDP Input

Empfängt Status-Feedback (On/Off, Helligkeit, Farbtemperatur, Farbe) für gemappte Geräte.

- **Gemappte Geräte**: Nur Geräte mit aktivem Mapping
- **Alle HUE-Geräte**: Alle erkannten HUE-Geräte

### Virtual HTTP Output

Erstellt Befehle zur Steuerung von HUE-Geräten aus Loxone:

- **MOOD-Befehle** für Lichtsteuerungs-Bausteine (automatische Gruppierung)
- **SET BRI-Befehle** für direkte Lichter/Gruppen

Die Dateien werden unter **Einstellungen → Loxone Config Export** heruntergeladen.

**Import in Loxone Config (2 Schritte):**

1. **Vorlage importieren:** In Loxone Config unter **Gerätevorlagen → Vorlage importieren** die XML-Datei laden.
2. **Vorlage ausführen:** Den entsprechenden Bereich im Peripheriebaum markieren, dann unter **Gerätevorlagen → Meine Vorlagen** die importierte Vorlage auswählen und ausführen.
   - **Virtual UDP Input:** *Virtuelle Eingänge* markieren und sicherstellen, dass *Vordefinierte UDP-Geräte* ausgewählt ist.
   - **Virtual HTTP Output:** *Virtuelle Ausgänge* markieren.

## Backup & Restore

Die Mappings können exportiert und importiert werden:

1. Gehe zum Tab "Mappings"
2. Klicke auf "Export" für ein JSON-Backup
3. Zum Wiederherstellen: "Import" → Datei wählen → Modus auswählen

## Fehlerbehebung

### Bridge wird nicht gefunden

- Stelle sicher, dass die HUE Bridge im selben Netzwerk ist
- Prüfe, ob mDNS funktioniert (Port 5353 UDP)
- Gib die IP-Adresse manuell in den Add-on-Optionen an

### Verbindung zu Loxone schlägt fehl

- Prüfe die Firewall-Einstellungen
- Stelle sicher, dass Port 8080 erreichbar ist
- Prüfe die Adresse des Virtuellen Ausgangs (`http://<gateway-ip>:8080`) und teste einen Befehl im Browser: `http://<gateway-ip>:8080/ws?cmd=SET <mapping-id> ON`
- Bei aktiviertem Login die Zugangsdaten in die Adresse aufnehmen: `http://admin:<passwort>@<gateway-ip>:8080`. Nach fünf Fehlversuchen ist die Adresse 30 Sekunden gesperrt (Antwort 429)

### Gerät reagiert nicht

- Prüfe, ob das Mapping korrekt erstellt wurde
- Stelle sicher, dass die Loxone ID übereinstimmt
- Prüfe die Gateway-Logs auf Fehlermeldungen

## Logs

Die Logs sind über die Home Assistant Add-on-Seite einsehbar oder via:

```bash
docker logs addon_local_loxone2hue
```

## Updates

Das Add-on wird automatisch aktualisiert, wenn eine neue Version im Repository verfügbar ist. Die Konfiguration und Mappings bleiben erhalten.
