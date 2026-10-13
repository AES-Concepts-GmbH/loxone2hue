import { describe, expect, it } from 'vitest';
import { parseChangelog } from './changelog';

describe('parseChangelog', () => {
  it('splits releases and entries, folding indented lines into the previous entry', () => {
    const md = [
      '# Changelog',
      '',
      '## 1.5.0',
      '',
      '- Erster Punkt',
      '- Zweiter Punkt',
      '  - Detail dazu',
      '',
      '## 1.4.4',
      '- Alter Punkt   ',
      'Text ohne Strich wird ignoriert',
    ].join('\n');

    expect(parseChangelog(md)).toEqual([
      { version: '1.5.0', items: ['Erster Punkt', 'Zweiter Punkt (Detail dazu)'] },
      { version: '1.4.4', items: ['Alter Punkt'] },
    ]);
  });

  it('ignores entries before the first release heading and headings that are not versions', () => {
    const md = '- verloren\n## Unreleased\n- auch verloren\n## 2.0.0\n- da';
    expect(parseChangelog(md)).toEqual([{ version: '2.0.0', items: ['da'] }]);
  });

  it('returns an empty list for an empty file', () => {
    expect(parseChangelog('')).toEqual([]);
  });
});
