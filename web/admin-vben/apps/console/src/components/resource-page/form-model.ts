import type { ConsoleFormField } from './types';

// Loading a record has to clear the previous one first: a field the response
// omits (Go's omitempty) would otherwise keep the last record's value and be
// saved onto this one.
export function loadFormModel(
  model: Record<string, any>,
  source: Record<string, any>,
) {
  for (const key of Object.keys(model)) {
    model[key] = undefined;
  }
  Object.assign(model, source);
}

// A cleared DatePicker yields null, which Go decodes the same as an absent
// field, so the stored time would silently stay. '' asks the backend to clear
// it.
export function toSubmitPayload(
  fields: ConsoleFormField[],
  model: Record<string, any>,
) {
  const payload = { ...model };
  for (const field of fields) {
    if (field.type === 'datetime' && payload[field.key] === null) {
      payload[field.key] = '';
    }
  }
  return payload;
}
