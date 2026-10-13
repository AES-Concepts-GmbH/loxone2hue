// Helpers for the "_mood_<n>" naming convention that ties the scene mappings
// of a Lichtsteuerungs-Baustein to their parent group mapping.
import type { Mapping } from '../types';

export const MOOD_SUFFIX = '_mood_';

export function isMoodChild(loxoneId: string): boolean {
  return /_mood_\d+$/.test(loxoneId);
}

export function getParentLoxoneId(moodLoxoneId: string): string | null {
  const match = moodLoxoneId.match(/^(.+)_mood_\d+$/);
  return match ? match[1] : null;
}

export function getMoodNumber(loxoneId: string): number {
  const match = loxoneId.match(/_mood_(\d+)$/);
  return match ? parseInt(match[1], 10) : -1;
}

export function moodLoxoneId(parentLoxoneId: string, moodNumber: number): string {
  return `${parentLoxoneId}${MOOD_SUFFIX}${moodNumber}`;
}

/** Mood mappings of a parent, sorted by mood number (0 = off first). */
export function getMoodChildren(mappings: Mapping[], parentLoxoneId: string): Mapping[] {
  return mappings
    .filter((m) => m.loxone_id.startsWith(`${parentLoxoneId}${MOOD_SUFFIX}`))
    .sort((a, b) => getMoodNumber(a.loxone_id) - getMoodNumber(b.loxone_id));
}

export function sortByName<T extends { name: string }>(items: readonly T[]): T[] {
  return [...items].sort((a, b) => a.name.localeCompare(b.name, 'de'));
}
