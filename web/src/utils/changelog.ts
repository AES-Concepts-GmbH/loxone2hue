export interface Release {
  version: string;
  items: string[];
}

/**
 * Parses the CHANGELOG: "## x.y.z" starts a release, "- text" is an entry,
 * indented "  - text" lines are folded into the previous entry.
 */
export function parseChangelog(markdown: string): Release[] {
  const releases: Release[] = [];
  let current: Release | null = null;

  for (const rawLine of markdown.split('\n')) {
    const line = rawLine.replace(/\s+$/, '');
    const heading = /^##\s+(\d+\.\d+\.\d+)\s*$/.exec(line);
    if (heading) {
      current = { version: heading[1], items: [] };
      releases.push(current);
      continue;
    }
    if (!current) continue;

    const nested = /^\s+-\s+(.*)$/.exec(line);
    if (nested && current.items.length > 0) {
      current.items[current.items.length - 1] += ` (${nested[1]})`;
      continue;
    }
    const item = /^-\s+(.*)$/.exec(line);
    if (item) {
      current.items.push(item[1]);
    }
  }

  return releases;
}
