import { describe, expect, it } from 'vitest';

import { customFormNames, resolveCustomForm } from './custom-forms';

const pageSources = import.meta.glob<string>('../../views/**/*.vue', {
  eager: true,
  import: 'default',
  query: '?raw',
});

function componentNamesUsedByPages(): string[] {
  return Object.values(pageSources).flatMap((source) =>
    [...source.matchAll(/component-name="([^"]+)"/g)].map(
      (match) => match[1] ?? '',
    ),
  );
}

// A custom form that cannot be resolved leaves the edit modal empty at runtime
// while typecheck, lint and build all pass — which is how moving resource-page
// broke the system and site config editors unnoticed.
describe('resource-page custom forms', () => {
  it('resolves every component-name a page passes', () => {
    const used = componentNamesUsedByPages();
    expect(used.length).toBeGreaterThan(0);

    for (const name of used) {
      expect(resolveCustomForm(name), name).toBeDefined();
    }
  });

  it('lists the forms it can resolve by bare name', () => {
    expect(customFormNames()).toContain('system-config-detail');
    expect(resolveCustomForm('does-not-exist')).toBeUndefined();
  });
});
