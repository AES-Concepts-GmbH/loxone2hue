# Konzept: Hue Bridge Pro – Raum-Bewegung und MotionAware

Stand: 14.10.2026 · Status: Phase 1 umgesetzt, Phase 2 wartet auf Ereignisdaten (siehe Abschnitt 8)

## 1. Ausgangslage

Loxone2HUE spricht ausschliesslich die CLIP API v2 der Hue Bridge. Die Hue Bridge Pro (Modell `BSB003`) verwendet dieselbe API, die bestehende Funktionalität (Lampen, Gruppen, Räume, Zonen, Szenen, Sensoren) läuft deshalb ohne Änderung.

Die Bridge Pro bringt jedoch zusätzliche Ressourcentypen mit, vor allem für **MotionAware** (Bewegungserkennung über die Funksignale der Lampen, ohne separaten Bewegungsmelder). Diese Typen wertet Loxone2HUE heute nicht aus.

Ziel dieses Konzepts: Bewegungsinformationen, die die Bridge auf Raum- bzw. Bereichsebene liefert, als Sensoren in Loxone2HUE verfügbar machen und per UDP/HTTP an den Miniserver weitergeben.

## 2. Analyse der Bridge Pro (10.1.19.45)

Abfrage vom 13.10.2026, nur lesend.

| Feld | Wert |
|---|---|
| Modell | `BSB003` (Hue Bridge Pro) |
| Firmware | 2071524040 |
| v1-API | 1.78.0 (weiterhin verfügbar) |
| Migration | Von alter Bridge `001788fffe41f854` übernommen am 04.03.2026 |

### 2.1 Unterstützte Ressourcentypen

Die Ressource `clip` listet 45 Typen. Relevant für dieses Konzept:

| Typ | Anzahl | Bedeutung | Heute in Loxone2HUE |
|---|---|---|---|
| `motion` | 1 | Klassischer Bewegungsmelder (SML003) | ja |
| `contact` | 0 | Secure-Kontaktsensor | ja |
| `grouped_motion` | 2 | Bewegung zusammengefasst pro Raum bzw. für das ganze Zuhause | **nein** |
| `grouped_light_level` | 2 | Helligkeit zusammengefasst pro Raum bzw. Zuhause | **nein** |
| `motion_area_candidate` | 12 | Lampen, die sich als MotionAware-Sensor eignen | nein (nur Info) |
| `motion_area_configuration` | 1 (Stand 14.10.) | Eingerichtete MotionAware-Bereiche | **nein** |
| `convenience_area_motion` | 1 (Stand 14.10.) | Bewegungsereignis eines Bereichs (Komfort) | **nein** |
| `security_area_motion` | 1 (Stand 14.10.) | Bewegungsereignis eines Bereichs (Sicherheit) | **nein** |
| `tamper` | 0 | Sabotagemeldung (Secure-Geräte) | nein |
| `camera_motion` | 0 | Bewegung von Secure-Kameras | nein |

Hinweis: Am 13.10. war noch kein MotionAware-Bereich eingerichtet (Anzahl 0). Am 14.10. wurde der Bereich „Büro Stefan“ angelegt und die Struktur abgefragt (Abschnitt 2.3). Das Bewegungsereignis selbst ist noch nicht erfasst (Abschnitt 8).

### 2.2 Beispiel `grouped_motion`

```json
{
  "id": "441a2437-80d8-4f6a-a270-6b680438355b",
  "type": "grouped_motion",
  "owner": { "rid": "79eeb5a2-25b3-45b0-9d06-7385038a5013", "rtype": "bridge_home" },
  "enabled": true,
  "motion": {
    "motion_report": { "changed": "2026-10-12T21:44:39.707Z", "motion": false }
  }
}
```

Der Owner ist entweder ein `room` oder `bridge_home`. Die Struktur des Feldes `motion` entspricht dem bekannten Typ `motion`, die bestehende Auswertung kann also wiederverwendet werden.

### 2.3 Echte Daten des MotionAware-Bereichs „Büro Stefan“

Abfrage vom 14.10.2026, nur lesend, Ruhezustand ohne Bewegung (gekürzt, IDs von Lampen weggelassen):

```json
// motion_area_configuration
{ "id": "df0e9044-…", "name": "Büro Stefan", "type": "motion_area_configuration",
  "group": { "rid": "03091793-…", "rtype": "room" },
  "participants": [ { "resource": { "rid": "…", "rtype": "motion_area_candidate" },
                     "status": { "health": "healthy" } } ],   // 4 Lampen
  "services": [ { "rid": "66e1dbe3-…", "rtype": "convenience_area_motion" },
                { "rid": "b6c5b664-…", "rtype": "security_area_motion" } ],
  "health": "healthy", "enabled": true }

// convenience_area_motion  (kein Feld "motion" im Ruhezustand)
{ "id": "66e1dbe3-…", "type": "convenience_area_motion",
  "owner": { "rid": "df0e9044-…", "rtype": "motion_area_configuration" },
  "enabled": true, "sensitivity": { "sensitivity": 2, "sensitivity_max": 4 } }

// security_area_motion  (leeres "motion", enabled=false)
{ "id": "b6c5b664-…", "type": "security_area_motion",
  "owner": { "rid": "df0e9044-…", "rtype": "motion_area_configuration" },
  "enabled": false, "motion": {}, "sensitivity": { "sensitivity": 2, "sensitivity_max": 4 } }
```

Erkenntnisse:

- Der Owner der beiden Bewegungstypen ist `motion_area_configuration`, der Name steht dort im Feld `name`. Die Namensauflösung aus Abschnitt 4.2 stimmt.
- `motion_area_configuration.services` verweist auf beide Bewegungsdienste, `group` auf den Raum.
- Im Ruhezustand fehlt bei `convenience_area_motion` das Feld `motion` ganz, bei `security_area_motion` ist es `{}`. Der Konverter muss ein fehlendes oder leeres `motion` als „unbekannt“ behandeln (nicht als Absturz). Ob der Wert bei der ersten Bewegung nachgeliefert wird, ist offen.
- `security_area_motion` hat `enabled: false`. Mit `enabled = false` ist der Sensor im Konzept laut Abschnitt 7 zu behandeln.

## 3. Umfang

### Phase 1 – Raum-Bewegung und Raum-Helligkeit (auch für normale Bridge)

- `grouped_motion` als Sensortyp, Wert `0/1` an Loxone
- `grouped_light_level` als Sensortyp, Wert in Lux an Loxone

Diese Typen gibt es auch auf neueren normalen Bridges. Phase 1 ist deshalb unabhängig von der Bridge Pro nützlich und lässt sich bereits heute testen.

### Phase 2 – MotionAware (nur Bridge Pro)

- `convenience_area_motion` als Sensortyp, Wert `0/1` an Loxone
- `security_area_motion` als Sensortyp, Wert `0/1` an Loxone
- Namen aus `motion_area_configuration` übernehmen

### Nicht im Umfang

- Einrichten oder Ändern von MotionAware-Bereichen (geschieht in der Hue-App)
- `tamper`, `camera_motion`, `bell_button`, `speaker` (Secure-Produkte, aktuell keine Geräte vorhanden)
- Steuern von Automationen (`behavior_instance`)

## 4. Technisches Design

Die neuen Typen werden wie die bestehenden Sensoren behandelt. Es entsteht kein neues Konzept im Mapping, in der UI oder bei der Rückmeldung an Loxone.

### 4.1 Datenmodell – [internal/models/sensor.go](internal/models/sensor.go)

- Kommentar zu `Sensor.Type` um die neuen Typen ergänzen.
- `SensorState` bleibt unverändert: `Motion *bool` und `LightLevel *int` decken alle neuen Typen ab.
- Optional: Feld `Scope string` (`"device"`, `"room"`, `"home"`, `"area"`) für Filterung und Icons in der UI.

### 4.2 Abfrage – [internal/hue/client.go](internal/hue/client.go)

**Endpunkte** in `GetSensors()` ergänzen:

```go
{"/clip/v2/resource/grouped_motion", "grouped_motion"},
{"/clip/v2/resource/grouped_light_level", "grouped_light_level"},
{"/clip/v2/resource/convenience_area_motion", "convenience_area_motion"},
{"/clip/v2/resource/security_area_motion", "security_area_motion"},
```

Auf einer Bridge ohne diese Typen schlägt die Abfrage fehl und wird wie bisher mit einem Debug-Log übersprungen. Es braucht keine Erkennung des Bridge-Modells.

**Namensauflösung:** Heute werden Namen nur über `owner.rid → device` aufgelöst. Die neuen Typen haben andere Owner:

| Typ | Owner-Typ | Name |
|---|---|---|
| `grouped_motion`, `grouped_light_level` | `room` | Raumname + " Bewegung" bzw. " Helligkeit" |
| `grouped_motion`, `grouped_light_level` | `bridge_home` | "Zuhause Bewegung" bzw. "Zuhause Helligkeit" |
| `convenience_area_motion`, `security_area_motion` | voraussichtlich `motion_area_configuration` | Bereichsname + " (Komfort)" bzw. " (Sicherheit)" |

Umsetzung: Die Map `deviceNames` wird zu einer allgemeinen Map `ownerNames`, die aus `device`, `room` und – falls vorhanden – `motion_area_configuration` befüllt wird. Raumnamen liegen bereits im Gruppen-Cache (`GetGroupName`). Die zusätzlichen Abfragen sind deshalb nur für `motion_area_configuration` nötig.

**Konvertierung** in `convertHueSensor()`:

- `grouped_motion`, `convenience_area_motion`, `security_area_motion` verwenden denselben Zweig wie `motion`.
- `grouped_light_level` verwendet denselben Zweig wie `light_level`.
- Nur der Namenszusatz unterscheidet sich.

### 4.3 Events – [internal/hue/events.go](internal/hue/events.go)

- Die neuen Typen in `sensorTypes` aufnehmen.
- In `updateSensorFromEvent()` die neuen Typen auf die bestehenden Fälle `motion` bzw. `light_level` abbilden.

Die Event-Payload der Bereichstypen ist zu verifizieren. Erwartet wird dieselbe Struktur `motion.motion_report` wie bei `grouped_motion`.

### 4.4 Rückmeldung an Loxone – [internal/api/websocket.go](internal/api/websocket.go)

Die neuen Typen werden auf die bestehenden Properties abgebildet:

| Hue-Typ | Property an Loxone | Wert |
|---|---|---|
| `grouped_motion` | `motion` | `0` / `1` |
| `convenience_area_motion` | `motion` | `0` / `1` |
| `security_area_motion` | `motion` | `0` / `1` |
| `grouped_light_level` | `light_level` | Lux (Ganzzahl) |

Damit funktionieren bestehende virtuelle Eingänge im Miniserver ohne Anpassung. Der Loxone-Befehl bleibt `<loxoneID>/motion` bzw. `<loxoneID>/light_level`.

Achtung: Die Hue-API liefert `light_level` als `10000 * log10(lux) + 1`. Wie heute bei `light_level` wird der Rohwert weitergegeben. Eine Umrechnung in Lux wäre für beide Typen gemeinsam zu entscheiden.

### 4.5 Web-UI – [web/src/](web/src/)

- Typen in [web/src/types](web/src/types) ergänzen.
- Sensorliste: Icon und Bezeichnung je Typ (z. B. "Raum-Bewegung", "MotionAware Komfort", "MotionAware Sicherheit").
- Optional: Filter nach `Scope`.
- Das Mapping funktioniert unverändert, weil es nur `hueType = "sensor"` und die ID kennt.

### 4.6 API-Dokumentation – [internal/api/swagger.go](internal/api/swagger.go)

Das Enum für `Sensor.type` um die neuen Werte erweitern.

### 4.7 Code-Kopien

Stand 14.10.2026 gibt es nur noch den Code im Root (`internal/`, `web/`). Die früheren Kopien unter `loxone2hue/src/` und `homeassistant/loxone2hue/src/` sowie `sync-addons.sh` existieren nicht mehr. Für das Add-on ist nur [loxone2hue/CHANGELOG.md](loxone2hue/CHANGELOG.md) zu pflegen.

## 5. Tests

| Bereich | Test |
|---|---|
| `convertHueSensor` | Tabellentest je neuem Typ mit Owner `room`, `bridge_home` und Bereich; Namen und State prüfen |
| `GetSensors` | Testserver liefert 404 für die Bereichstypen (normale Bridge), die übrigen Sensoren müssen trotzdem geladen werden |
| Events | `grouped_motion`- und `convenience_area_motion`-Event aktualisiert den Sensor-Cache |
| Rückmeldung | `websocket_feedback_test.go`: neue Typen senden `motion` bzw. `light_level` |
| Manuell | Bridge Pro 10.1.19.45: Raum betreten, Wert im Miniserver prüfen |

## 6. Aufwand und Reihenfolge

1. Phase 1 Backend (Modell, Abfrage, Namen, Events, Rückmeldung) inkl. Tests
2. Phase 1 UI und Swagger
3. MotionAware-Bereich in der Hue-App einrichten, echte JSON-Daten erfassen
4. Phase 2 Backend und UI anhand der echten Daten
5. Sync in die Add-on-Kopien, CHANGELOG, Release

## 7. Offene Punkte

- **Bewegungsereignis der Bereichstypen:** Struktur von `motion` bei `convenience_area_motion` und `security_area_motion` im Event-Stream erfassen (Abschnitt 8).
- **Sicherheitsbewegung nur bei aktivem Alarm?** Der Sicherheitsbereich ist auf der Test-Bridge deaktiviert (`enabled: false`). Prüfen, ob er nach dem Aktivieren meldet und ob er Hue Secure voraussetzt.
- **Helligkeitswert:** Rohwert oder Lux an Loxone senden (betrifft auch den bestehenden Typ `light_level`).
- **`enabled = false`:** Ob deaktivierte Raum- oder Bereichssensoren angezeigt oder ausgeblendet werden.
- **Application-Key:** Der Key vom 13.10. wurde im Chat übermittelt, der Key vom 14.10. lag kurz in `%USERPROFILE%\.hue_key`. Beide in der Hue-App entfernen und die Datei löschen, sobald sie nicht mehr gebraucht werden.

## 8. Vorgehen für später: Bewegungsereignis erfassen und Phase 2 umsetzen

Voraussetzung: Man ist vor Ort (die Bridge `10.1.19.45` ist nur im Heimnetz erreichbar) und der Bereich „Büro Stefan“ existiert.

1. **Application Key bereitstellen.** Neuen Key in der Hue-App erzeugen oder den vorhandenen verwenden. In eine Datei ausserhalb des Repos schreiben, damit er nicht im Chat landet:
   ```powershell
   Set-Content -Path $env:USERPROFILE\.hue_key -Value "KEY" -NoNewline
   ```
2. **Event-Stream aufzeichnen** (ca. 60 Sekunden, nur lesend). Während der Aufzeichnung durch das Büro laufen und danach den Raum verlassen und ruhig bleiben, damit „Bewegung“ und „keine Bewegung“ beide erfasst werden:
   ```powershell
   $k = (Get-Content $env:USERPROFILE\.hue_key -Raw).Trim()
   curl.exe -sk -N --max-time 60 -H "hue-application-key: $k" -H "Accept: text/event-stream" `
     "https://10.1.19.45/eventstream/clip/v2" | Tee-Object $env:TEMP\hue-events.txt
   ```
3. **Aufzeichnung auswerten:** Zeilen mit `convenience_area_motion` und `security_area_motion` heraussuchen. Zu klären: Pfad des Werts (erwartet `motion.motion_report.motion`), Zeitstempel, ob nach der Bewegung wieder `false` kommt.
4. **Zweiter Lauf mit aktiviertem Sicherheitsbereich** (in der Hue-App einschalten), um das Verhalten von `security_area_motion` zu prüfen.
5. **Phase 2 umsetzen** nach Abschnitt 4: Endpunkte in `GetSensors()`, Namensauflösung über `motion_area_configuration`, Events in `events.go`, Typen in `sensorkind.go`, Rückmeldung in `websocket.go`, Web-UI, Swagger, Tests (u. a. fehlendes bzw. leeres `motion`), CHANGELOG und README-Abschnitt „MotionAware-Daten liefern“ anpassen.
6. **Aufräumen:** `.hue_key` und `hue-events.txt` löschen, Key in der Hue-App entfernen.
