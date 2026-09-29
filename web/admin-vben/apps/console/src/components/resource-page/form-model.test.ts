import { describe, expect, it } from 'vitest';

import { loadFormModel, toSubmitPayload } from './form-model';

describe('resource-page form model', () => {
  it('does not carry a field over from the previously edited record', () => {
    const model: Record<string, any> = { due_at: undefined, title: undefined };
    loadFormModel(model, { due_at: '2026-10-01 10:00:00', title: 'a' });
    loadFormModel(model, { title: 'b' });

    expect(model).toEqual({ due_at: undefined, title: 'b' });
  });

  it('sends a cleared datetime as an empty string', () => {
    const payload = toSubmitPayload(
      [
        { key: 'due_at', label: 'due_at', type: 'datetime' },
        { key: 'amount', label: 'amount', type: 'number' },
      ],
      { amount: null, due_at: null },
    );

    expect(payload).toEqual({ amount: null, due_at: '' });
  });
});
