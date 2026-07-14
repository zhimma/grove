import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { loadScript } from '../resources';

const testJsPath =
  'https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.0/jquery.min.js';
let appendedScripts: HTMLScriptElement[] = [];

describe('loadScript', () => {
  beforeEach(() => {
    appendedScripts = [];
    vi.spyOn(document.head, 'append').mockImplementation((...nodes) => {
      appendedScripts.push(
        ...nodes.filter(
          (node): node is HTMLScriptElement =>
            node instanceof HTMLScriptElement,
        ),
      );
    });
    vi.spyOn(document, 'querySelector').mockImplementation((selector) => {
      const matched = /^script\[src="(.+)"\]$/.exec(selector);
      if (!matched) {
        return null;
      }
      return (
        appendedScripts.find(
          (script) => script.getAttribute('src') === matched[1],
        ) || null
      );
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should resolve when the script loads successfully', async () => {
    const promise = loadScript(testJsPath);

    // 此时脚本元素已被创建并插入
    const script = document.querySelector(
      `script[src="${testJsPath}"]`,
    ) as HTMLScriptElement;
    expect(script).toBeTruthy();

    // 模拟加载成功
    script.dispatchEvent(new Event('load'));

    // 等待 promise resolve
    await expect(promise).resolves.toBeUndefined();
  });

  it('should not insert duplicate script and resolve immediately if already loaded', async () => {
    // 先手动插入一个相同 src 的 script
    const existing = document.createElement('script');
    existing.src = 'bar.js';
    document.head.append(existing);

    // 再次调用
    const promise = loadScript('bar.js');

    // 立即 resolve
    await expect(promise).resolves.toBeUndefined();

    expect(
      appendedScripts.filter(
        (script) => script.getAttribute('src') === 'bar.js',
      ),
    ).toHaveLength(1);
  });

  it('should reject when the script fails to load', async () => {
    const promise = loadScript('error.js');

    const script = document.querySelector(
      'script[src="error.js"]',
    ) as HTMLScriptElement;
    expect(script).toBeTruthy();

    // 模拟加载失败
    script.dispatchEvent(new Event('error'));

    await expect(promise).rejects.toThrow('Failed to load script: error.js');
  });

  it('should handle multiple concurrent calls and only insert one script tag', async () => {
    const p1 = loadScript(testJsPath);
    const p2 = loadScript(testJsPath);

    const script = document.querySelector(
      `script[src="${testJsPath}"]`,
    ) as HTMLScriptElement;
    expect(script).toBeTruthy();

    // 触发一次 load，两个 promise 都应该 resolve
    script.dispatchEvent(new Event('load'));

    await expect(p1).resolves.toBeUndefined();
    await expect(p2).resolves.toBeUndefined();

    expect(
      appendedScripts.filter(
        (script) => script.getAttribute('src') === testJsPath,
      ),
    ).toHaveLength(1);
  });
});
