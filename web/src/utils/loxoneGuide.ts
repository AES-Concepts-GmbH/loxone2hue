// Text for the per-mapping "Loxone Config" panel: which Virtueller Ausgang
// Befehle to create, and which feedback a sensor mapping produces.
import type { Mapping } from '../types';
import { sensorKind } from './sensors';
import { isMoodChild } from './moods';

export interface GuideCommand {
  label: string;
  value: string;
  description: string;
}

export interface LoxoneGuideText {
  title: string;
  commands: GuideCommand[];
  note: string;
}

export interface SensorFeedbackInfo {
  property: string;
  range: string;
  example: string;
}

export interface TestUrl {
  label: string;
  url: string;
}

export interface GuideContext {
  /** CLIP type of the mapped sensor, for sensor mappings. */
  sensorType?: string;
  /** Whether any Miniserver has UDP resp. HTTP feedback enabled. */
  hasUDP: boolean;
  hasHTTP: boolean;
}

export function getSensorFeedbackInfo(sensorType: string): SensorFeedbackInfo[] {
  // Room/home aggregates (grouped_*) use the same properties as the device sensors.
  switch (sensorKind(sensorType)) {
    case 'motion':
      return [{ property: 'motion', range: '0 / 1', example: 'motion:1' }];
    case 'temperature':
      return [{ property: 'temperature', range: 'z.B. 21.5 (°C)', example: 'temperature:21.5' }];
    case 'light_level':
      return [
        { property: 'light_level', range: 'Hue-Rohwert: 10000·log10(Lux)+1', example: 'light_level:20001' },
        { property: 'lux', range: '0–100000 (Lux)', example: 'lux:100' },
      ];
    case 'button':
      return [{ property: 'button', range: '0=press, 2=short, 3=long_release, 4=long_press', example: 'button:2' }];
    case 'contact':
      return [{ property: 'contact', range: '0=geschlossen, 1=offen', example: 'contact:0' }];
    case 'relative_rotary':
      return [{ property: 'rotary', range: 'Schritte (positiv/negativ)', example: 'rotary:3' }];
    case 'device_power':
      return [{ property: 'battery', range: '0–100 (%)', example: 'battery:85' }];
    default:
      return [{ property: sensorType, range: 'variabel', example: `${sensorType}:<wert>` }];
  }
}

function sensorGuide(mapping: Mapping, ctx: GuideContext): LoxoneGuideText {
  const loxoneId = mapping.loxone_id;
  const sensorType = ctx.sensorType || 'unknown';
  const showUDP = ctx.hasUDP && mapping.feedback_udp !== false;
  const showHTTP = ctx.hasHTTP && mapping.feedback_http !== false;

  const commands: GuideCommand[] = [];
  for (const fp of getSensorFeedbackInfo(sensorType)) {
    const exampleValue = fp.example.split(':')[1];
    if (showUDP) {
      commands.push({
        label: `UDP: ${loxoneId}/${fp.property}:<wert>`,
        value: `${loxoneId}/${fp.property}:${exampleValue}`,
        description: `Wertebereich: ${fp.range}`,
      });
    }
    if (showHTTP) {
      commands.push({
        label: `HTTP: /dev/sps/io/${loxoneId}_${fp.property}/<wert>`,
        value: `/dev/sps/io/${loxoneId}_${fp.property}/${exampleValue}`,
        description: `Wertebereich: ${fp.range}`,
      });
    }
  }

  const protocols = [showUDP && 'UDP', showHTTP && 'HTTP'].filter(Boolean).join(' & ');
  const noteLines = [`Sensoren senden Werte automatisch per ${protocols || 'UDP/HTTP'} an Loxone.`];
  if (showUDP) noteLines.push('Erstelle einen Virtuellen UDP-Eingang mit passender Befehls-Erkennung.');
  if (showHTTP) noteLines.push('Erstelle einen Virtuellen HTTP-Eingang.');
  noteLines.push('Es werden keine Befehle an den Sensor gesendet.');

  return { title: `Sensor-Feedback (${sensorType})`, commands, note: noteLines.join(' ') };
}

export function getLoxoneGuide(mapping: Mapping, ctx: GuideContext): LoxoneGuideText {
  const loxoneId = mapping.loxone_id;

  if (mapping.hue_type === 'sensor') {
    return sensorGuide(mapping, ctx);
  }

  if (mapping.hue_type === 'scene') {
    if (isMoodChild(loxoneId)) {
      return {
        title: 'Mood-Szene (für Lichtsteuerungs-Baustein)',
        commands: [
          {
            label: 'Direkt aktivieren',
            value: `/ws?cmd=SCENE ${loxoneId}`,
            description: 'Szene direkt aktivieren (ohne Lichtsteuerungs-Baustein)',
          },
        ],
        note: 'Diese Szene ist Teil eines MOOD-Mappings. Sie wird automatisch aktiviert, wenn der Lichtsteuerungs-Baustein die entsprechende Stimmungsnummer sendet. Verwende den MOOD-Befehl am Gruppen-Mapping.',
      };
    }
    return {
      title: 'Szene aktivieren (Impuls-Taster)',
      commands: [
        {
          label: 'Befehl bei EIN',
          value: `/ws?cmd=SCENE ${loxoneId}`,
          description: 'Virtueller Ausgang Befehl - wird bei Impuls ausgeführt',
        },
      ],
      note: 'Für Szenen empfiehlt sich ein Taster/Impuls in Loxone. Die Szene wird bei jedem Impuls aktiviert.',
    };
  }

  if (mapping.hue_type === 'group') {
    return {
      title: 'Gruppe/Raum steuern',
      commands: [
        { label: 'Befehl bei EIN', value: `/ws?cmd=SET ${loxoneId} ON`, description: 'Virtueller Ausgang Befehl - Gruppe einschalten' },
        { label: 'Befehl bei AUS', value: `/ws?cmd=SET ${loxoneId} OFF`, description: 'Virtueller Ausgang Befehl - Gruppe ausschalten' },
        { label: 'Helligkeit (0-100%)', value: `/ws?cmd=SET ${loxoneId} BRI <v>`, description: 'Virtueller Ausgang Befehl (Analog) - <v> wird durch Wert ersetzt' },
        { label: 'Lichtsteuerungs-Baustein (MOOD)', value: `/ws?cmd=MOOD ${loxoneId} <v>`, description: 'AQ-Ausgang → Virtueller Ausgang (Analog) - <v> = Stimmungsnummer' },
      ],
      note: `Für den Lichtsteuerungs-Baustein: Erstelle zusätzlich Szenen-Mappings mit dem Muster "${loxoneId}_mood_1", "${loxoneId}_mood_2", etc. MOOD 0 schaltet diese Gruppe aus.`,
    };
  }

  return {
    title: 'Einzelnes Licht steuern',
    commands: [
      { label: 'Befehl bei EIN', value: `/ws?cmd=SET ${loxoneId} ON`, description: 'Virtueller Ausgang Befehl - Licht einschalten' },
      { label: 'Befehl bei AUS', value: `/ws?cmd=SET ${loxoneId} OFF`, description: 'Virtueller Ausgang Befehl - Licht ausschalten' },
      { label: 'Helligkeit (0-100%)', value: `/ws?cmd=SET ${loxoneId} BRI <v>`, description: 'Virtueller Ausgang Befehl (Analog) - <v> wird durch Wert ersetzt' },
      { label: 'Farbtemperatur (2000-6500K)', value: `/ws?cmd=SET ${loxoneId} CT <v>`, description: 'Virtueller Ausgang Befehl (Analog) - optional' },
      { label: 'Farbe (Hex)', value: `/ws?cmd=SET ${loxoneId} COLOR #FF5500`, description: 'Virtueller Ausgang Befehl - Farbe als Hex-Wert' },
    ],
    note: 'Erstelle einen Virtuellen Ausgang mit der Adresse des Gateways (http://<gateway-ip>:<port>) und füge die Befehle als Virtuelle Ausgang Befehle hinzu.',
  };
}

function buildTestUrl(cmd: string): string {
  return `./ws?cmd=${encodeURIComponent(cmd)}`;
}

/** Browser-testable command URLs. Sensors only send data, so they have none. */
export function getTestUrls(mapping: Mapping): TestUrl[] {
  const loxoneId = mapping.loxone_id;

  if (mapping.hue_type === 'sensor') {
    return [];
  }

  if (mapping.hue_type === 'scene') {
    return [{ label: 'Szene aktivieren', url: buildTestUrl(`SCENE ${loxoneId}`) }];
  }

  if (isMoodChild(loxoneId)) {
    return [{ label: 'Mood aktivieren (Szene)', url: buildTestUrl(`SCENE ${loxoneId}`) }];
  }

  const common: TestUrl[] = [
    { label: 'Einschalten', url: buildTestUrl(`SET ${loxoneId} ON`) },
    { label: 'Ausschalten', url: buildTestUrl(`SET ${loxoneId} OFF`) },
    { label: 'Helligkeit 50%', url: buildTestUrl(`SET ${loxoneId} BRI 50`) },
  ];

  if (mapping.hue_type === 'group') {
    return [
      ...common,
      { label: 'MOOD 0 (Aus)', url: buildTestUrl(`MOOD ${loxoneId} 0`) },
      { label: 'MOOD 1', url: buildTestUrl(`MOOD ${loxoneId} 1`) },
      { label: 'MOOD 2', url: buildTestUrl(`MOOD ${loxoneId} 2`) },
    ];
  }

  return [...common, { label: 'Farbtemperatur 4000K', url: buildTestUrl(`SET ${loxoneId} CT 4000`) }];
}
