import type { MiniserverConfig } from '../../services/api';
import type { HueResources } from '../../utils/hueResources';

/** Bridge resources plus the Loxone side the mapping components need. */
export interface MappingEnv extends HueResources {
  miniservers: MiniserverConfig[];
  /** At least one Miniserver has UDP resp. HTTP feedback enabled. */
  hasUDP: boolean;
  hasHTTP: boolean;
}

export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}
