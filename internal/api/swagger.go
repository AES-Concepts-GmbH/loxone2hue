package api

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed swagger-ui.html
var swaggerUIHTML embed.FS

// SwaggerUI serves the Swagger UI page
func (s *Server) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFS(swaggerUIHTML, "swagger-ui.html"))
	tmpl.Execute(w, nil)
}

// SwaggerSpec serves the OpenAPI specification
func (s *Server) SwaggerSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(openAPISpec))
}

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Loxone2HUE Gateway API",
    "description": "Gateway Service zur bidirektionalen Kommunikation zwischen Loxone und Philips HUE.",
    "version": "1.3.3",
    "contact": {
      "name": "Loxone2HUE Gateway"
    }
  },
  "servers": [
    {
      "url": ".",
      "description": "API Endpunkt (relativ)"
    }
  ],
  "tags": [
    {
      "name": "Health",
      "description": "Service Health Check"
    },
    {
      "name": "Loxone",
      "description": "Loxone Befehle (HTTP/WebSocket Endpoint)"
    },
    {
      "name": "Bridge",
      "description": "HUE Bridge Discovery und Pairing"
    },
    {
      "name": "Devices",
      "description": "HUE Lampen (Lights) verwalten"
    },
    {
      "name": "Groups",
      "description": "HUE Räume und Zonen verwalten. Räume sind physische Bereiche (z.B. Wohnzimmer), Zonen sind virtuelle Gruppen die Geräte aus mehreren Räumen zusammenfassen können."
    },
    {
      "name": "Scenes",
      "description": "HUE Szenen verwalten"
    },
    {
      "name": "Sensors",
      "description": "HUE Sensoren, Taster und Zubehör (Bewegungsmelder, Temperatur, Helligkeit, Taster, Kontakt, Drehregler, Batterie)"
    },
    {
      "name": "Mappings",
      "description": "Loxone zu HUE Mappings verwalten"
    },
    {
      "name": "Config",
      "description": "Gateway Konfiguration"
    },
    {
      "name": "Export",
      "description": "Loxone Config XML-Vorlagen exportieren (Virtual UDP Input, Virtual HTTP Output)"
    }
  ],
  "paths": {
    "/health": {
      "get": {
        "tags": ["Health"],
        "summary": "Health Check",
        "description": "Gibt den aktuellen Status des Gateway Services zurück.",
        "responses": {
          "200": {
            "description": "Service ist gesund",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/HealthResponse"
                }
              }
            }
          }
        }
      }
    },
    "/../ws": {
      "get": {
        "tags": ["Loxone"],
        "summary": "Loxone Befehl ausführen (WebSocket)",
        "description": "Führt einen Loxone-Befehl aus. Dieser Endpoint ist für Loxone Virtual Outputs gedacht.\n\n**Hinweis:** Der WebSocket-Endpoint liegt auf /ws (Root-Ebene, nicht unter /api). Der Pfad /../ws ist relativ zum API-Basispfad /api.\n\n## Verfügbare Befehle\n\n| Befehl | Beschreibung | Beispiel |\n|--------|--------------|----------|\n| SET id ON | Licht/Gruppe einschalten | SET wz_decke ON |\n| SET id OFF | Licht/Gruppe ausschalten | SET wz_decke OFF |\n| SET id BRI n | Helligkeit setzen (0-100%) | SET wz_decke BRI 75 |\n| SET id CT n | Farbtemperatur (2000-6500K) | SET wz_decke CT 4000 |\n| SET id COLOR hex | Farbe setzen | SET wz_decke COLOR #FF5500 |\n| SCENE id | Szene aktivieren | SCENE sz_relax |\n| MOOD id n | Stimmung aktivieren (Lichtsteuerung) | MOOD wohnzimmer 1 |\n| GET id STATUS | Status abfragen | GET wz_decke STATUS |\n\n## MOOD-Befehl für Lichtsteuerungs-Baustein\n\nDer MOOD-Befehl ist für den Loxone Lichtsteuerungs-Baustein konzipiert:\n- MOOD 0: Schaltet die zugehörige Gruppe/Licht aus\n- MOOD 1-9: Aktiviert die entsprechende Szene\n\nBenötigte Mappings für MOOD:\n- target -> Gruppe (für Mood 0 = Aus)\n- target_mood_1 -> Szene (für Mood 1)\n- target_mood_2 -> Szene (für Mood 2)\n- etc.",
        "parameters": [
          {
            "name": "cmd",
            "in": "query",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "Der auszuführende Befehl",
            "examples": {
              "on": {
                "summary": "Licht einschalten",
                "value": "SET wz_decke ON"
              },
              "off": {
                "summary": "Licht ausschalten",
                "value": "SET wz_decke OFF"
              },
              "brightness": {
                "summary": "Helligkeit setzen",
                "value": "SET wz_decke BRI 75"
              },
              "scene": {
                "summary": "Szene aktivieren",
                "value": "SCENE sz_relax"
              },
              "mood": {
                "summary": "Stimmung aktivieren",
                "value": "MOOD wohnzimmer 1"
              }
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Befehl erfolgreich ausgeführt",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/LoxoneCommandResponse"
                }
              }
            }
          },
          "400": {
            "description": "Ungültiger Befehl oder fehlende Parameter",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          },
          "404": {
            "description": "Kein Mapping für Target gefunden",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/LoxoneMoodError"
                }
              }
            }
          }
        }
      }
    },
    "/bridge": {
      "get": {
        "tags": ["Bridge"],
        "summary": "Bridge Info",
        "description": "Gibt Informationen über die verbundene HUE Bridge zurück.",
        "responses": {
          "200": {
            "description": "Bridge Informationen",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/BridgeResponse"
                }
              }
            }
          }
        }
      }
    },
    "/bridge/discover": {
      "get": {
        "tags": ["Bridge"],
        "summary": "Bridge Discovery",
        "description": "Sucht nach HUE Bridges im lokalen Netzwerk via mDNS.",
        "responses": {
          "200": {
            "description": "Gefundene Bridges",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/DiscoverResponse"
                }
              }
            }
          }
        }
      }
    },
    "/bridge/pair": {
      "post": {
        "tags": ["Bridge"],
        "summary": "Bridge Pairing",
        "description": "Koppelt den Gateway mit einer HUE Bridge. Der Link-Button auf der Bridge muss vorher gedrückt werden.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/PairRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Pairing erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/PairResponse"
                }
              }
            }
          },
          "400": {
            "description": "Pairing fehlgeschlagen (Link-Button nicht gedrückt)",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      }
    },
    "/bridge/test": {
      "post": {
        "tags": ["Bridge"],
        "summary": "Bridge Verbindungstest",
        "description": "Testet die Netzwerkverbindung zu einer HUE Bridge. Führt DNS-Lookup, TCP-Verbindungstests (Port 80 und 443) und einen HTTPS-Request an die Bridge-API durch.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/PairRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Testergebnisse (auch bei fehlgeschlagenen Tests)",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/BridgeTestResult"
                }
              }
            }
          },
          "400": {
            "description": "Ungültige Anfrage (bridge_ip fehlt)",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      }
    },
    "/devices": {
      "get": {
        "tags": ["Devices"],
        "summary": "Alle Lampen abrufen",
        "description": "Gibt eine Liste aller HUE Lampen zurück.",
        "responses": {
          "200": {
            "description": "Liste der Lampen",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/DevicesResponse"
                }
              }
            }
          }
        }
      }
    },
    "/devices/{id}": {
      "get": {
        "tags": ["Devices"],
        "summary": "Einzelne Lampe abrufen",
        "description": "Gibt Details zu einer bestimmten Lampe zurück.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Lampe"
          }
        ],
        "responses": {
          "200": {
            "description": "Lampendetails",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Light"
                }
              }
            }
          },
          "404": {
            "description": "Lampe nicht gefunden",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      },
      "put": {
        "tags": ["Devices"],
        "summary": "Lampe steuern",
        "description": "Setzt den Zustand einer Lampe (Ein/Aus, Helligkeit, Farbe, etc.).",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Lampe"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeviceCommand"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["Devices"],
        "summary": "Lampe steuern (POST)",
        "description": "Setzt den Zustand einer Lampe. Identisch mit PUT, wird für Loxone-Kompatibilität unterstützt.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Lampe"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeviceCommand"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      }
    },
    "/groups": {
      "get": {
        "tags": ["Groups"],
        "summary": "Alle Räume und Zonen abrufen",
        "description": "Gibt eine Liste aller HUE Räume (type: room) und Zonen (type: zone) zurück. Räume sind physische Bereiche, Zonen sind virtuelle Gruppen. Jede Gruppe enthält eine Liste der zugehörigen Lampen-IDs.",
        "responses": {
          "200": {
            "description": "Liste der Gruppen",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/GroupsResponse"
                }
              }
            }
          }
        }
      }
    },
    "/groups/{id}": {
      "get": {
        "tags": ["Groups"],
        "summary": "Einzelne Gruppe abrufen",
        "description": "Gibt Details zu einer bestimmten Gruppe zurück.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Gruppe"
          }
        ],
        "responses": {
          "200": {
            "description": "Gruppendetails",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Group"
                }
              }
            }
          },
          "404": {
            "description": "Gruppe nicht gefunden",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      },
      "put": {
        "tags": ["Groups"],
        "summary": "Gruppe steuern",
        "description": "Setzt den Zustand aller Lampen in einer Gruppe.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Gruppe"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeviceCommand"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["Groups"],
        "summary": "Gruppe steuern (POST)",
        "description": "Setzt den Zustand aller Lampen in einer Gruppe. Identisch mit PUT, wird für Loxone-Kompatibilität unterstützt.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Gruppe"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/DeviceCommand"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      }
    },
    "/scenes": {
      "get": {
        "tags": ["Scenes"],
        "summary": "Alle Szenen abrufen",
        "description": "Gibt eine Liste aller HUE Szenen zurück. Jede Szene ist einem Raum oder einer Zone zugeordnet (group_id). Die Szenen werden im Frontend mit dem Format 'Raum - Szenenname' angezeigt.",
        "responses": {
          "200": {
            "description": "Liste der Szenen",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ScenesResponse"
                }
              }
            }
          }
        }
      }
    },
    "/scenes/{id}/activate": {
      "post": {
        "tags": ["Scenes"],
        "summary": "Szene aktivieren",
        "description": "Aktiviert eine HUE Szene.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID der Szene"
          }
        ],
        "responses": {
          "200": {
            "description": "Szene aktiviert",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      }
    },
    "/sensors": {
      "get": {
        "tags": ["Sensors"],
        "summary": "Alle Sensoren abrufen",
        "description": "Gibt eine Liste aller HUE Sensoren zurück, gruppiert nach Gerät. Unterstützte Sensortypen: motion (Bewegung), temperature (Temperatur), light_level (Helligkeit), button (Taster), contact (Kontakt), relative_rotary (Drehregler), device_power (Batterie).",
        "responses": {
          "200": {
            "description": "Liste der Sensoren",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/SensorsResponse"
                }
              }
            }
          }
        }
      }
    },
    "/mappings": {
      "get": {
        "tags": ["Mappings"],
        "summary": "Alle Mappings abrufen",
        "description": "Gibt eine Liste aller Loxone zu HUE Mappings zurück.",
        "responses": {
          "200": {
            "description": "Liste der Mappings",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/MappingsResponse"
                }
              }
            }
          }
        }
      },
      "post": {
        "tags": ["Mappings"],
        "summary": "Mapping erstellen",
        "description": "Erstellt ein neues Loxone zu HUE Mapping.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/MappingCreate"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Mapping erstellt",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Mapping"
                }
              }
            }
          }
        }
      }
    },
    "/mappings/{id}": {
      "put": {
        "tags": ["Mappings"],
        "summary": "Mapping aktualisieren",
        "description": "Aktualisiert ein bestehendes Mapping.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID des Mappings"
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/Mapping"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Mapping aktualisiert",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Mapping"
                }
              }
            }
          },
          "404": {
            "description": "Mapping nicht gefunden",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      },
      "delete": {
        "tags": ["Mappings"],
        "summary": "Mapping löschen",
        "description": "Löscht ein Mapping.",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            },
            "description": "ID des Mappings"
          }
        ],
        "responses": {
          "200": {
            "description": "Mapping gelöscht",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          },
          "404": {
            "description": "Mapping nicht gefunden",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      }
    },
    "/mappings/export": {
      "get": {
        "tags": ["Mappings"],
        "summary": "Mappings exportieren",
        "description": "Exportiert alle Mappings als JSON-Backup. Die Datei enthält Versionsinformationen, einen Zeitstempel und alle Mappings.",
        "responses": {
          "200": {
            "description": "Backup der Mappings",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/MappingsBackup"
                }
              }
            }
          }
        }
      }
    },
    "/mappings/import": {
      "post": {
        "tags": ["Mappings"],
        "summary": "Mappings importieren",
        "description": "Importiert Mappings aus einem zuvor exportierten Backup. Unterstützt zwei Modi:\n\n- **replace**: Alle bestehenden Mappings werden gelöscht und durch die importierten ersetzt.\n- **merge**: Bestehende Mappings bleiben erhalten. Mappings mit gleicher Loxone ID werden aktualisiert, neue werden hinzugefügt.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ImportRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Import erfolgreich",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ImportResult"
                }
              }
            }
          },
          "400": {
            "description": "Ungültiges Backup-Format",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ErrorResponse"
                }
              }
            }
          }
        }
      }
    },
    "/export/inputs": {
      "get": {
        "tags": ["Export"],
        "summary": "Virtual UDP Input XML exportieren",
        "description": "Generiert eine Loxone Virtual UDP Input XML-Vorlage zum Import in Loxone Config. Erstellt Befehls-Erkennungen für On/Off, Helligkeit, Farbtemperatur und Farbe (XY) passend zum UDP-Feedback-Format.\n\nDie Datei kann in Loxone Config unter **Gerätevorlagen → Vorlage importieren** geladen werden.",
        "parameters": [
          {
            "name": "all",
            "in": "query",
            "required": false,
            "schema": {
              "type": "string",
              "enum": ["true"]
            },
            "description": "Wenn 'true', werden alle HUE-Geräte exportiert (nicht nur gemappte)"
          }
        ],
        "responses": {
          "200": {
            "description": "XML-Datei (VirtualInUdp)",
            "content": {
              "text/xml": {
                "schema": {
                  "type": "string"
                },
                "example": "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<VirtualInUdp Title=\"Loxone2HUE Status\" Port=\"7777\">\n  <VirtualInUdpCmd Title=\"Büro On\" Check=\"buero/on:\\\\v\" Analog=\"true\"/>\n</VirtualInUdp>"
              }
            }
          }
        }
      }
    },
    "/export/outputs": {
      "get": {
        "tags": ["Export"],
        "summary": "Virtual HTTP Output XML exportieren",
        "description": "Generiert eine Loxone Virtual HTTP Output XML-Vorlage zum Import in Loxone Config. Erstellt Befehle zur Steuerung von HUE-Geräten über den Gateway.\n\nMood-Mappings (Loxone ID mit Pattern *_mood_N) werden automatisch zu einem einzigen MOOD-Befehl pro Zielgruppe zusammengefasst. Direkte Light/Group-Mappings erhalten SET BRI Befehle.\n\nDie Datei kann in Loxone Config unter Gerätevorlagen > Vorlage importieren geladen werden.",
        "responses": {
          "200": {
            "description": "XML-Datei (VirtualOut)",
            "content": {
              "text/xml": {
                "schema": {
                  "type": "string"
                },
                "example": "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<VirtualOut Title=\"Loxone2HUE\" Address=\"http://gateway:8080\">\n  <VirtualOutCmd Title=\"Wohnzimmer (Mood)\" CmdOn=\"/ws?cmd=MOOD wohnzimmer <v>\" Analog=\"true\"/>\n</VirtualOut>"
              }
            }
          }
        }
      }
    },
    "/config": {
      "get": {
        "tags": ["Config"],
        "summary": "Konfiguration abrufen",
        "description": "Gibt die aktuelle Gateway-Konfiguration zurück (ohne sensible Daten).",
        "responses": {
          "200": {
            "description": "Konfiguration",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ConfigResponse"
                }
              }
            }
          }
        }
      },
      "put": {
        "tags": ["Config"],
        "summary": "Konfiguration aktualisieren",
        "description": "Aktualisiert die Gateway-Konfiguration.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ConfigUpdate"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Konfiguration aktualisiert",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StatusResponse"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "HealthResponse": {
        "type": "object",
        "properties": {
          "status": {
            "type": "string",
            "example": "healthy"
          },
          "timestamp": {
            "type": "string",
            "format": "date-time"
          },
          "hue_configured": {
            "type": "boolean"
          }
        }
      },
      "BridgeResponse": {
        "type": "object",
        "properties": {
          "configured": {
            "type": "boolean"
          },
          "info": {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "name": {
                "type": "string"
              },
              "model": {
                "type": "string"
              },
              "software_version": {
                "type": "string"
              }
            }
          }
        }
      },
      "DiscoverResponse": {
        "type": "object",
        "properties": {
          "bridges": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": {
                  "type": "string"
                },
                "ip": {
                  "type": "string"
                }
              }
            }
          }
        }
      },
      "BridgeTestResult": {
        "type": "object",
        "description": "Ergebnis des Bridge-Verbindungstests mit detaillierten Diagnoseinformationen",
        "properties": {
          "bridge_ip": {
            "type": "string",
            "description": "Getestete IP-Adresse",
            "example": "192.168.1.100"
          },
          "dns_lookup": {
            "type": "object",
            "properties": {
              "success": {
                "type": "boolean"
              },
              "addresses": {
                "type": "array",
                "items": {
                  "type": "string"
                },
                "description": "Aufgelöste IP-Adressen"
              },
              "error": {
                "type": "string"
              }
            }
          },
          "tcp_443": {
            "type": "object",
            "description": "TCP-Verbindungstest auf Port 443 (HTTPS)",
            "properties": {
              "success": {
                "type": "boolean"
              },
              "error": {
                "type": "string"
              }
            }
          },
          "tcp_80": {
            "type": "object",
            "description": "TCP-Verbindungstest auf Port 80 (HTTP)",
            "properties": {
              "success": {
                "type": "boolean"
              },
              "error": {
                "type": "string"
              }
            }
          },
          "https_request": {
            "type": "object",
            "description": "HTTPS-Request an die Bridge-API (/api/config)",
            "properties": {
              "success": {
                "type": "boolean"
              },
              "status_code": {
                "type": "integer",
                "description": "HTTP Status Code der Antwort"
              },
              "error": {
                "type": "string"
              }
            }
          }
        }
      },
      "PairRequest": {
        "type": "object",
        "required": ["bridge_ip"],
        "properties": {
          "bridge_ip": {
            "type": "string",
            "example": "192.168.1.100",
            "description": "IP-Adresse der HUE Bridge"
          }
        }
      },
      "PairResponse": {
        "type": "object",
        "properties": {
          "success": {
            "type": "boolean"
          },
          "application_key": {
            "type": "string",
            "description": "Der generierte Application Key für die API"
          }
        }
      },
      "Light": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "type": {
            "type": "string"
          },
          "model": {
            "type": "string"
          },
          "manufacturer": {
            "type": "string"
          },
          "state": {
            "$ref": "#/components/schemas/LightState"
          }
        }
      },
      "LightState": {
        "type": "object",
        "properties": {
          "on": {
            "type": "boolean"
          },
          "brightness": {
            "type": "integer",
            "minimum": 0,
            "maximum": 100
          },
          "color_temp": {
            "type": "integer",
            "description": "Farbtemperatur in Kelvin (2000-6500)"
          },
          "reachable": {
            "type": "boolean"
          }
        }
      },
      "DevicesResponse": {
        "type": "object",
        "properties": {
          "devices": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Light"
            }
          }
        }
      },
      "DeviceCommand": {
        "type": "object",
        "properties": {
          "on": {
            "type": "boolean",
            "description": "Ein/Aus"
          },
          "brightness": {
            "type": "integer",
            "minimum": 0,
            "maximum": 100,
            "description": "Helligkeit in Prozent"
          },
          "color_temp": {
            "type": "integer",
            "description": "Farbtemperatur in Kelvin (2000-6500)"
          },
          "color": {
            "type": "string",
            "description": "Farbe als Hex-Wert (z.B. #FF5500)"
          }
        }
      },
      "Group": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "description": "Eindeutige ID der Gruppe (UUID)"
          },
          "name": {
            "type": "string",
            "description": "Name des Raums oder der Zone"
          },
          "type": {
            "type": "string",
            "enum": ["room", "zone"],
            "description": "room = physischer Raum, zone = virtuelle Gruppe"
          },
          "lights": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "description": "Liste der Light-IDs (nicht Device-IDs!) die zu dieser Gruppe gehören"
          },
          "state": {
            "$ref": "#/components/schemas/GroupState"
          }
        }
      },
      "GroupState": {
        "type": "object",
        "properties": {
          "any_on": {
            "type": "boolean",
            "description": "Mindestens eine Lampe ist an"
          },
          "all_on": {
            "type": "boolean",
            "description": "Alle Lampen sind an"
          }
        }
      },
      "GroupsResponse": {
        "type": "object",
        "properties": {
          "groups": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Group"
            }
          }
        }
      },
      "Scene": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "description": "Eindeutige ID der Szene (UUID)"
          },
          "name": {
            "type": "string",
            "description": "Name der Szene (z.B. 'Entspannen', 'Konzentrieren')"
          },
          "group_id": {
            "type": "string",
            "description": "ID des zugehörigen Raums oder der Zone. Im Frontend als 'Raum - Szene' dargestellt."
          }
        }
      },
      "ScenesResponse": {
        "type": "object",
        "properties": {
          "scenes": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Scene"
            }
          }
        }
      },
      "Mapping": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string"
          },
          "name": {
            "type": "string"
          },
          "loxone_id": {
            "type": "string",
            "description": "Loxone Identifikator für dieses Mapping. Wird automatisch aus dem Namen generiert (Kleinbuchstaben, Umlaute konvertiert, Leerzeichen als Unterstriche)."
          },
          "hue_id": {
            "type": "string",
            "description": "ID der HUE Ressource"
          },
          "hue_type": {
            "type": "string",
            "enum": ["light", "group", "scene", "sensor"],
            "description": "Typ der HUE Ressource"
          },
          "enabled": {
            "type": "boolean"
          },
          "feedback_udp": {
            "type": "boolean",
            "description": "UDP-Feedback für dieses Mapping aktivieren. Standard: true (wenn nicht gesetzt).",
            "nullable": true
          },
          "feedback_http": {
            "type": "boolean",
            "description": "HTTP-Feedback für dieses Mapping aktivieren. Standard: true (wenn nicht gesetzt).",
            "nullable": true
          },
          "description": {
            "type": "string"
          },
          "miniserver_id": {
            "type": "string",
            "description": "ID des zugeordneten Loxone Miniservers. Bei nur einem Miniserver automatisch zugewiesen."
          }
        }
      },
      "MappingCreate": {
        "type": "object",
        "required": ["name", "loxone_id", "hue_id", "hue_type"],
        "properties": {
          "name": {
            "type": "string"
          },
          "loxone_id": {
            "type": "string",
            "description": "Wird automatisch aus dem Namen generiert, kann aber manuell überschrieben werden."
          },
          "hue_id": {
            "type": "string"
          },
          "hue_type": {
            "type": "string",
            "enum": ["light", "group", "scene", "sensor"]
          },
          "feedback_udp": {
            "type": "boolean",
            "description": "UDP-Feedback aktivieren. Standard: true.",
            "nullable": true
          },
          "feedback_http": {
            "type": "boolean",
            "description": "HTTP-Feedback aktivieren. Standard: true.",
            "nullable": true
          },
          "description": {
            "type": "string"
          },
          "miniserver_id": {
            "type": "string",
            "description": "ID des Miniservers. Optional, wird automatisch zugewiesen wenn nur ein Miniserver konfiguriert ist."
          }
        }
      },
      "MappingsResponse": {
        "type": "object",
        "properties": {
          "mappings": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Mapping"
            }
          }
        }
      },
      "MappingsBackup": {
        "type": "object",
        "description": "Backup-Datei für Mappings mit Versionsinformationen",
        "properties": {
          "version": {
            "type": "string",
            "description": "Version des Backup-Formats",
            "example": "1.0"
          },
          "created_at": {
            "type": "string",
            "format": "date-time",
            "description": "Zeitpunkt der Backup-Erstellung"
          },
          "mappings": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Mapping"
            },
            "description": "Liste aller exportierten Mappings"
          }
        }
      },
      "ImportRequest": {
        "type": "object",
        "required": ["mode", "backup"],
        "properties": {
          "mode": {
            "type": "string",
            "enum": ["replace", "merge"],
            "description": "Import-Modus: 'replace' = alle ersetzen, 'merge' = zusammenführen"
          },
          "backup": {
            "$ref": "#/components/schemas/MappingsBackup",
            "description": "Das zu importierende Backup"
          }
        }
      },
      "ImportResult": {
        "type": "object",
        "description": "Ergebnis des Import-Vorgangs",
        "properties": {
          "status": {
            "type": "string",
            "example": "ok"
          },
          "imported": {
            "type": "integer",
            "description": "Anzahl neu importierter Mappings"
          },
          "updated": {
            "type": "integer",
            "description": "Anzahl aktualisierter bestehender Mappings"
          },
          "skipped": {
            "type": "integer",
            "description": "Anzahl übersprungener Mappings"
          },
          "total": {
            "type": "integer",
            "description": "Gesamtanzahl verarbeiteter Mappings"
          }
        }
      },
      "ConfigResponse": {
        "type": "object",
        "properties": {
          "server": {
            "type": "object",
            "properties": {
              "host": {
                "type": "string"
              },
              "port": {
                "type": "integer"
              }
            }
          },
          "hue": {
            "type": "object",
            "properties": {
              "bridge_ip": {
                "type": "string"
              },
              "configured": {
                "type": "boolean"
              }
            }
          },
          "loxone": {
            "type": "object",
            "properties": {
              "miniservers": {
                "type": "array",
                "description": "Liste der konfigurierten Loxone Miniserver",
                "items": {
                  "$ref": "#/components/schemas/MiniserverConfig"
                }
              }
            }
          }
        }
      },
      "ConfigUpdate": {
        "type": "object",
        "properties": {
          "loxone": {
            "type": "object",
            "properties": {
              "miniservers": {
                "type": "array",
                "items": {
                  "$ref": "#/components/schemas/MiniserverConfig"
                }
              }
            }
          }
        }
      },
      "MiniserverConfig": {
        "type": "object",
        "description": "Konfiguration eines einzelnen Loxone Miniservers. Mehrere Miniserver können gleichzeitig konfiguriert werden, jeweils mit eigenen UDP- und HTTP-Feedback-Einstellungen.",
        "properties": {
          "id": {
            "type": "string",
            "description": "Eindeutige ID des Miniservers (UUID)"
          },
          "name": {
            "type": "string",
            "description": "Anzeigename des Miniservers",
            "example": "Haupthaus"
          },
          "ip": {
            "type": "string",
            "description": "IP-Adresse des Miniservers (für UDP-Feedback)",
            "example": "10.1.19.40"
          },
          "port": {
            "type": "integer",
            "description": "UDP-Port für Status-Feedback (Standard: 7777)",
            "example": 7777,
            "minimum": 1,
            "maximum": 65535
          },
          "udp_enabled": {
            "type": "boolean",
            "description": "UDP Status-Feedback für diesen Miniserver aktivieren"
          },
          "http_enabled": {
            "type": "boolean",
            "description": "HTTP Status-Feedback für diesen Miniserver aktivieren"
          },
          "http_url": {
            "type": "string",
            "description": "Ziel-URL für HTTP-Feedback (Virtueller HTTP-Eingang). Falls leer, wird http://<ip> verwendet.",
            "example": "https://192.168.1.7:443/request.php"
          },
          "http_user": {
            "type": "string",
            "description": "Benutzername für HTTP Basic Auth"
          },
          "http_password": {
            "type": "string",
            "description": "Passwort für HTTP Basic Auth"
          },
          "send_all": {
            "type": "boolean",
            "description": "Feedback für alle Geräte senden (auch ohne Mapping). Der Gerätename wird als Loxone-ID verwendet."
          }
        }
      },
      "StatusResponse": {
        "type": "object",
        "properties": {
          "status": {
            "type": "string",
            "example": "ok"
          }
        }
      },
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": {
            "type": "string"
          }
        }
      },
      "LoxoneCommandResponse": {
        "type": "object",
        "properties": {
          "status": {
            "type": "string",
            "example": "ok"
          },
          "target": {
            "type": "string",
            "description": "Die Loxone ID aus dem Befehl",
            "example": "wz_decke"
          },
          "action": {
            "type": "string",
            "description": "Die ausgeführte Aktion",
            "enum": ["set", "scene", "mood"],
            "example": "set"
          },
          "hue_id": {
            "type": "string",
            "description": "Die aufgelöste HUE Ressourcen-ID",
            "example": "abc123-def456"
          },
          "hue_type": {
            "type": "string",
            "description": "Der Typ der HUE Ressource",
            "enum": ["light", "group", "scene", "sensor"],
            "example": "light"
          }
        }
      },
      "SensorsResponse": {
        "type": "object",
        "properties": {
          "sensors": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/Sensor"
            }
          }
        }
      },
      "Sensor": {
        "type": "object",
        "description": "HUE Sensor (Bewegung, Temperatur, Helligkeit, Taster, Kontakt, Drehregler, Batterie)",
        "properties": {
          "id": {
            "type": "string",
            "description": "Eindeutige Sensor-ID"
          },
          "name": {
            "type": "string",
            "description": "Name des Sensors"
          },
          "type": {
            "type": "string",
            "enum": ["motion", "temperature", "light_level", "button", "contact", "relative_rotary", "device_power"],
            "description": "Sensortyp"
          },
          "device_id": {
            "type": "string",
            "description": "ID des übergeordneten HUE-Geräts"
          },
          "owner": {
            "type": "string",
            "description": "Name des übergeordneten Geräts"
          },
          "state": {
            "$ref": "#/components/schemas/SensorState"
          }
        }
      },
      "SensorState": {
        "type": "object",
        "description": "Aktueller Zustand eines Sensors. Je nach Sensortyp sind unterschiedliche Felder gefüllt.",
        "properties": {
          "motion": {
            "type": "boolean",
            "description": "Bewegung erkannt (motion sensor)",
            "nullable": true
          },
          "temperature": {
            "type": "number",
            "description": "Temperatur in Celsius (temperature sensor)",
            "nullable": true
          },
          "light_level": {
            "type": "integer",
            "description": "Helligkeit in Lux (light_level sensor)",
            "nullable": true
          },
          "button_event": {
            "type": "string",
            "enum": ["initial_press", "repeat", "short_release", "long_release", "long_press"],
            "description": "Letztes Taster-Event (button sensor)",
            "nullable": true
          },
          "control_id": {
            "type": "integer",
            "description": "Taster-Index, 1-basiert (button sensor)",
            "nullable": true
          },
          "contact_state": {
            "type": "string",
            "enum": ["contact", "no_contact"],
            "description": "Kontaktzustand (contact sensor)",
            "nullable": true
          },
          "rotary_action": {
            "type": "string",
            "enum": ["start", "repeat"],
            "description": "Dreh-Aktion (relative_rotary sensor)",
            "nullable": true
          },
          "rotary_steps": {
            "type": "integer",
            "description": "Schritte: positiv=Uhrzeigersinn, negativ=Gegenuhrzeiger (relative_rotary sensor)",
            "nullable": true
          },
          "battery_level": {
            "type": "integer",
            "minimum": 0,
            "maximum": 100,
            "description": "Batteriestand in Prozent (device_power sensor)",
            "nullable": true
          },
          "battery_state": {
            "type": "string",
            "enum": ["normal", "low", "critical"],
            "description": "Batteriestatus (device_power sensor)",
            "nullable": true
          },
          "enabled": {
            "type": "boolean",
            "description": "Sensor ist aktiviert"
          },
          "last_updated": {
            "type": "string",
            "description": "Zeitpunkt der letzten Aktualisierung"
          }
        }
      },
      "LoxoneMoodError": {
        "type": "object",
        "properties": {
          "error": {
            "type": "string",
            "example": "no mapping found for mood"
          },
          "target": {
            "type": "string",
            "description": "Die Loxone ID aus dem Befehl",
            "example": "wohnzimmer"
          },
          "mood_number": {
            "type": "string",
            "description": "Die angeforderte Stimmungsnummer",
            "example": "1"
          }
        }
      }
    }
  }
}`
