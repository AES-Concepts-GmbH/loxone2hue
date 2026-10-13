import { describe, expect, it } from 'vitest';
import type { Mapping } from '../types';
import { getMoodChildren, getMoodNumber, getParentLoxoneId, isMoodChild, moodLoxoneId, sortByName } from './moods';

function mapping(loxoneId: string, name = loxoneId): Mapping {
  return { id: loxoneId, name, loxone_id: loxoneId, hue_id: 'h', hue_type: 'scene', enabled: true };
}

describe('mood naming', () => {
  it('recognises the _mood_<n> suffix and extracts parent and number', () => {
    expect(isMoodChild('wz_mood_0')).toBe(true);
    expect(isMoodChild('wz_mood_12')).toBe(true);
    expect(isMoodChild('wz')).toBe(false);
    expect(isMoodChild('wz_mood_')).toBe(false);
    expect(isMoodChild('wz_mood_x')).toBe(false);

    expect(getParentLoxoneId('wz_mood_3')).toBe('wz');
    expect(getParentLoxoneId('a_mood_1_mood_2')).toBe('a_mood_1');
    expect(getParentLoxoneId('wz')).toBeNull();

    expect(getMoodNumber('wz_mood_3')).toBe(3);
    expect(getMoodNumber('wz')).toBe(-1);
    expect(moodLoxoneId('wz', 4)).toBe('wz_mood_4');
  });

  it('lists the children of a parent sorted by mood number', () => {
    const all = [mapping('wz_mood_2'), mapping('wz'), mapping('wz_mood_0'), mapping('kueche_mood_1'), mapping('wz_mood_1')];
    expect(getMoodChildren(all, 'wz').map((m) => m.loxone_id)).toEqual(['wz_mood_0', 'wz_mood_1', 'wz_mood_2']);
    expect(getMoodChildren(all, 'bad')).toEqual([]);
  });

  it('sorts by name with German collation without mutating the input', () => {
    const input = [mapping('b', 'Zimmer'), mapping('a', 'Ärger'), mapping('c', 'anfang')];
    const sorted = sortByName(input);
    expect(sorted.map((m) => m.name)).toEqual(['anfang', 'Ärger', 'Zimmer']);
    expect(input[0].name).toBe('Zimmer');
  });
});
