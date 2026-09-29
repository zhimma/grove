import { describe, expect, it } from 'vitest';

import html from '../index.html?raw';

// Upstream Vben's index.html injects its own Baidu analytics beacon into
// production builds, which reported every deployment's page views to a third
// party. Nothing in the shell page may load code from another origin.
describe('index.html', () => {
  it('loads no third-party script', () => {
    expect(html).not.toMatch(/hm\.baidu\.com|googletagmanager|_hmt/);
    for (const [, src = ''] of html.matchAll(/<script[^>]*\ssrc="([^"]+)"/g)) {
      expect(src, src).toMatch(/^\//);
    }
  });
});
