import { flushPromises, mount } from '@vue/test-utils';

import { describe, expect, it, vi } from 'vitest';

import * as api from '#/api/system-config';

import SiteConfigs from '../site/index.vue';
import SystemConfigs from '../system/index.vue';
import ConfigForm from './ConfigForm.vue';

vi.mock('#/api/system-config', () => ({
  createSystemConfig: vi.fn(),
  deleteSystemConfig: vi.fn(),
  getSystemConfigList: vi.fn(async () => ({
    list: [
      {
        id: 'config-1',
        config_group: 'site',
        config_key: 'credential',
        name: '凭据',
        value_type: 'string',
        value: '********',
        is_secret: true,
        is_editable: true,
      },
    ],
    meta: { total: 1 },
  })),
  updateSystemConfig: vi.fn(async () => ({})),
}));

describe('explicit configuration forms', () => {
  it.each([
    ['system', SystemConfigs],
    ['site', SiteConfigs],
  ] as const)(
    '%s page renders the form and preserves secret values on edit',
    async (_name, page) => {
      const wrapper = mount(page, { attachTo: document.body });
      try {
        await flushPromises();
        const edit = wrapper
          .findAll('tbody tr.ant-table-row button')
          .find((button) => button.text() === '编辑');
        expect(edit).toBeDefined();
        if (!edit) throw new Error('编辑按钮缺失');
        await edit.trigger('click');
        await flushPromises();
        expect(wrapper.findComponent(ConfigForm).exists()).toBe(true);
        const name = document.body.querySelector<HTMLInputElement>(
          'input[placeholder="显示名称"]',
        );
        expect(name).not.toBeNull();
        if (!name) throw new Error('配置名称输入框缺失');
        name.value = '更新名称';
        name.dispatchEvent(new Event('input', { bubbles: true }));
        await flushPromises();
        const save = document.body.querySelector<HTMLButtonElement>(
          '.ant-modal-footer .ant-btn-primary',
        );
        expect(save).not.toBeNull();
        if (!save) throw new Error('保存按钮缺失');
        save.click();
        await flushPromises();
        expect(api.updateSystemConfig).toHaveBeenLastCalledWith(
          'config-1',
          expect.objectContaining({
            name: '更新名称',
            config_group: 'site',
            keep_secret: true,
            value: '',
          }),
        );
      } finally {
        wrapper.unmount();
      }
    },
  );
});
