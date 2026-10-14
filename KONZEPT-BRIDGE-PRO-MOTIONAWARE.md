# Konzept: Hue Bridge Pro – Raum-Bewegung und MotionAware

Stand: 13.10.2026 · Status: Entwurf

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
| `motion_area_configuration` | 0 | Eingerichtete MotionAware-Bereiche | **nein** |
| `convenience_area_motion` | 0 | Bewegungsereignis eines Bereichs (Komfort) | **nein** |
| `security_area_motion` | 0 | Bewegungsereignis eines Bereichs (Sicherheit) | **nein** |
| `tamper` | 0 | Sabotagemeldung (Secure-Geräte) | nein |
| `camera_motion` | 0 | Bewegung von Secure-Kameras | nein |

Hinweis: Auf der Test-Bridge ist noch kein MotionAware-Bereich eingerichtet. Die JSON-Struktur von `motion_area_configuration`, `convenience_area_motion` und `security_area_motion` muss deshalb noch an echten Daten verifiziert werden (siehe Abschnitt 7).

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

Der Go- und Web-Code liegt dreifach im Repo:

- Root (`internal/`, `web/`)
- [loxone2hue/src/](loxone2hue/src/)
- [homeassistant/loxone2hue/src/](homeassistant/loxone2hue/src/)

Änderungen werden im Root gemacht und mit [sync-addons.sh](sync-addons.sh) verteilt.

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

- **Struktur der MotionAware-Typen:** Felder von `motion_area_configuration`, `convenience_area_motion` und `security_area_motion` an der Bridge Pro verifizieren, sobald ein Bereich eingerichtet ist (`GET /clip/v2/resource/motion_area_configuration`).
- **Sicherheitsbewegung nur bei aktivem Alarm?** Prüfen, ob `security_area_motion` nur bei scharf geschaltetem Hue Secure meldet.
- **Helligkeitswert:** Rohwert oder Lux an Loxone senden (betrifft auch den bestehenden Typ `light_level`).
- **`enabled = false`:** Ob deaktivierte Raum- oder Bereichssensoren angezeigt oder ausgeblendet werden.
- **Application-Key:** Der für die Analyse verwendete Key wurde im Chat übermittelt und sollte in der Hue-App entfernt werden, falls er nicht weiter gebraucht wird.
