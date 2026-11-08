import type { MiniserverConfig } from '../../services/api';
import type { HueResources } from '../../utils/hueResources';

/** Bridge resources plus the Loxone side the mapping components need. */
export interface MappingEnv extends HueResources {
  miniservers: MiniserverConfig[];
  /** At least one Miniserver has UDP resp. HTTP feedback enabled. */
  hasUDP: boolean;
  hasHTTP: boolean;
  /** Port the gateway listens on (server config), used to build URLs for external clients. */
  gatewayPort?: number;
  /** Username of the login protection while it is switched on, else undefined. */
  authUser?: string;
}

export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}
