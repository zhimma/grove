import { flushPromises, mount } from '@vue/test-utils';

import { describe, expect, it, vi } from 'vitest';

import * as api from '#/api/scheduled-task';

import ScheduledTaskPage from './index.vue';

vi.mock('#/api/scheduled-task', () => ({
  getScheduledTaskList: vi.fn(async () => ({
    list: [
      {
        id: 'task-1',
        name: 'console.purge',
        display_name: '清理过期会话',
        schedule: '0 17 3 * * *',
        enabled: true,
        mutex: true,
        timeout_seconds: 0,
        run_requested_at: '',
        last_run_at: '2026-09-29 03:17:00',
        last_status: 'success',
        last_error: '',
        last_duration_ms: 12,
        updated_at: '2026-09-29 03:17:00',
      },
    ],
    meta: { total: 1 },
  })),
  runScheduledTask: vi.fn(async () => ({})),
  setScheduledTaskStatus: vi.fn(async () => ({})),
  updateScheduledTask: vi.fn(async () => ({})),
}));

describe('scheduled task page', () => {
  it('renders the task row and runs it through the shared list', async () => {
    const wrapper = mount(ScheduledTaskPage);
    await flushPromises();

    const row = wrapper.find('tbody tr.ant-table-row');
    expect(row.find('code').text()).toBe('0 17 3 * * *');
    expect(row.text()).toContain('已启用');
    expect(row.text()).toContain('成功');
    expect(row.text()).toContain('12ms');

    const buttons = row.findAll('button').map((button) => button.text());
    expect(buttons).toEqual(['执行一次', '停用', '编辑']);

    await row.findAll('button')[0]?.trigger('click');
    await flushPromises();
    expect(api.runScheduledTask).toHaveBeenCalledWith('task-1');
    expect(api.getScheduledTaskList).toHaveBeenCalledTimes(2);
  });

  it('submits only the timing fields when a task is edited', async () => {
    const wrapper = mount(ScheduledTaskPage, { attachTo: document.body });
    await flushPromises();

    const edit = wrapper
      .findAll('tbody tr.ant-table-row button')
      .find((button) => button.text() === '编辑');
    await edit?.trigger('click');
    await flushPromises();
    document.body
      .querySelector<HTMLButtonElement>('.ant-modal-footer .ant-btn-primary')
      ?.click();
    await flushPromises();

    expect(api.updateScheduledTask).toHaveBeenCalledWith('task-1', {
      mutex: true,
      schedule: '0 17 3 * * *',
      timeout_seconds: 0,
    });
    wrapper.unmount();
  });
});
