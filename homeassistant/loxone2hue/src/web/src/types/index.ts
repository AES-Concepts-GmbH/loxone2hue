export interface Light {
  id: string;
  name: string;
  type: string;
  model_id: string;
  product_name: string;
  state: LightState;
  capabilities: Capabilities;
}

export interface LightState {
  on: boolean;
  brightness: number;
  color_temp?: number;
  color?: Color;
  reachable: boolean;
}

export interface Color {
  xy: [number, number];
  gamut?: string;
  hex_rgb?: string;
}

export interface Capabilities {
  supports_color: boolean;
  supports_color_temp: boolean;
  supports_dimming: boolean;
}

export interface Group {
  id: string;
  name: string;
  type: string;
  lights: string[];
  state: GroupState;
  scenes?: Scene[];
  grouped_light_id?: string;
}

export interface GroupState {
  all_on: boolean;
  any_on: boolean;
  brightness?: number;
}

export interface Scene {
  id: string;
  name: string;
  group_id: string;
  type: string;
}

export interface Sensor {
  id: string;
  name: string;
  type: string; // "motion", "temperature", "light_level", "button", "contact", "relative_rotary", "device_power"
  device_id?: string;
  owner?: string;
  state: SensorState;
}

export interface SensorState {
  motion?: boolean;
  temperature?: number;
  light_level?: number;
  button_event?: string;
  control_id?: number;
  contact_state?: string;
  rotary_action?: string;
  rotary_steps?: number;
  battery_level?: number;
  battery_state?: string;
  enabled: boolean;
  last_updated?: string;
}

export interface Mapping {
  id: string;
  name: string;
  loxone_id: string;
  hue_id: string;
  hue_type: string;
  enabled: boolean;
  feedback_udp?: boolean;
  feedback_http?: boolean;
  description?: string;
  miniserver_id?: string;
}

export interface BridgeInfo {
  id: string;
  ip: string;
  name: string;
}

export interface DeviceCommand {
  on?: boolean;
  brightness?: number;
  color_temp?: number;
  color?: Color;
}

export interface WebSocketMessage {
  type: string;
  payload: unknown;
}

export interface StatusMessage {
  type: 'status';
  device: string;
  state: LightState;
}

export interface SensorUpdateMessage {
  type: 'sensor_update';
  sensor: Sensor;
}

export interface GroupUpdateMessage {
  type: 'group_update';
  group: Group;
}

export type WSMessage = StatusMessage | SensorUpdateMessage | GroupUpdateMessage;
