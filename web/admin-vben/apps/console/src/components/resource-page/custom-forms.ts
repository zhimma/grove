import type { Component } from 'vue';

// Custom edit forms live under views/console/custom/. The path used to be a
// relative template-literal import resolved from this component's own folder,
// so moving the component silently pointed it at a directory that does not
// exist and left the edit modal empty. A glob is resolved at build time and is
// something a test can enumerate.
const forms = import.meta.glob<{ default: Component }>(
  '../../views/console/custom/*.vue',
);

const prefix = '../../views/console/custom/';

export function resolveCustomForm(name: string) {
  return forms[`${prefix}${name}.vue`];
}

export function customFormNames(): string[] {
  return Object.keys(forms).map((path) =>
    path.slice(prefix.length, -'.vue'.length),
  );
}
