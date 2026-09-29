import { flushPromises, mount } from '@vue/test-utils';
import { h } from 'vue';

import { describe, expect, it, vi } from 'vitest';

import ResourcePage from './index.vue';

describe('resource-page slots', () => {
  it('lets a page take over cells and add row actions without losing the defaults', async () => {
    const fetchApi = vi.fn(async () => ({
      list: [{ id: 'task-1', name: 'alpha', enabled: true, mutex: false }],
      meta: { total: 1 },
    }));
    const wrapper = mount(ResourcePage, {
      props: {
        title: '任务',
        columns: [
          { title: '名称', dataIndex: 'name', key: 'name' },
          { title: '状态', dataIndex: 'enabled', key: 'enabled' },
          { title: '互斥', dataIndex: 'mutex', key: 'mutex' },
        ],
        fetchApi,
      },
      slots: {
        cell: ({ column }: { column: { key?: unknown } }) =>
          column.key === 'enabled' ? h('b', { class: 'state' }, 'ON') : null,
        actions: ({ record }: { record: Record<string, any> }) =>
          h('i', { class: 'run' }, `run ${record.id}`),
      },
    });
    await flushPromises();

    const row = wrapper.find('tbody tr.ant-table-row');
    expect(row.text()).toContain('alpha'); // untouched column: raw value
    expect(row.find('.state').text()).toBe('ON'); // taken over by the page
    expect(row.text()).toContain('否'); // boolean column the page left alone
    expect(row.find('.run').text()).toBe('run task-1');

    await (wrapper.vm as unknown as { reload: () => Promise<void> }).reload();
    expect(fetchApi).toHaveBeenCalledTimes(2);
  });
});
