import { beforeEach, describe, expect, it, vi } from 'vitest';

import { consoleEndpoint } from './console-contract';
import {
  getScheduledTaskList,
  runScheduledTask,
  setScheduledTaskStatus,
  updateScheduledTask,
} from './scheduled-task';

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}));

vi.mock('#/api/request', () => ({
  requestClient: {
    get: mocks.get,
    post: mocks.post,
    put: mocks.put,
  },
}));

describe('console scheduled task API contract', () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.put.mockReset();
  });

  it('sends list filters under the names the backend binds', () => {
    getScheduledTaskList({ page: 2, page_size: 25, enabled: false });

    expect(mocks.get).toHaveBeenCalledWith(
      consoleEndpoint('consoleListScheduledTasks'),
      { params: { page: 2, page_size: 25, enabled: false } },
    );
  });

  it('puts schedule edits on the task path', () => {
    updateScheduledTask('task-1', {
      schedule: '0 30 2 * * *',
      mutex: false,
      timeout_seconds: 90,
    });

    expect(mocks.put).toHaveBeenCalledWith(
      consoleEndpoint('consoleUpdateScheduledTask', { id: 'task-1' }),
      { schedule: '0 30 2 * * *', mutex: false, timeout_seconds: 90 },
    );
  });

  // Status is its own endpoint so a toggle cannot silently overwrite the
  // schedule fields an operator did not touch.
  it('toggles status through the dedicated status endpoint', () => {
    setScheduledTaskStatus('task-1', false);

    expect(mocks.put).toHaveBeenCalledWith(
      consoleEndpoint('consoleSetScheduledTaskStatus', { id: 'task-1' }),
      { enabled: false },
    );
  });

  it('requests a run with no body', () => {
    runScheduledTask('task-1');

    expect(mocks.post).toHaveBeenCalledWith(
      consoleEndpoint('consoleRunScheduledTask', { id: 'task-1' }),
    );
  });
});
