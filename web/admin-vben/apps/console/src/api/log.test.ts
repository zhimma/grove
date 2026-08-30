import { beforeEach, describe, expect, it, vi } from 'vitest';

import { consoleEndpoint } from './console-contract';
import {
  getLoginLogList,
  getOperationLogDetail,
  getOperationLogList,
} from './log';

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock('#/api/request', () => ({
  requestClient: {
    get: mocks.get,
  },
}));

describe('console log API contract', () => {
  beforeEach(() => {
    mocks.get.mockReset();
  });

  it('uses the list + meta response and backend filter names', () => {
    const params = {
      page: 2,
      page_size: 25,
      success: false,
      created_from: '2026-08-01 00:00:00',
      created_to: '2026-08-31 23:59:59',
    };

    getOperationLogList(params);

    expect(mocks.get).toHaveBeenCalledWith(
      consoleEndpoint('consoleListOperationLogs'),
      { params },
    );
  });

  it('keeps operation detail wrapped as log + detail', () => {
    getOperationLogDetail('operation-1');

    expect(mocks.get).toHaveBeenCalledWith(
      consoleEndpoint('consoleGetOperationLog', { id: 'operation-1' }),
    );
  });

  it('uses the same list + meta contract for login logs', () => {
    const params = { success: true };
    getLoginLogList(params);

    expect(mocks.get).toHaveBeenCalledWith(
      consoleEndpoint('consoleListLoginLogs'),
      { params },
    );
  });
});
