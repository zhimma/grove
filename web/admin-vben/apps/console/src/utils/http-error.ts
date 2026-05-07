import type { ApiErrorResponse, FieldErrors } from '#/types/common';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function normalizeFieldErrors(value: unknown): FieldErrors {
  if (!isRecord(value)) {
    return {};
  }

  const result: FieldErrors = {};
  Object.entries(value).forEach(([field, errors]) => {
    if (Array.isArray(errors)) {
      const messages = errors.filter(
        (item): item is string => typeof item === 'string' && item.length > 0,
      );
      if (messages.length > 0) {
        result[field] = messages;
      }
    }
  });

  return result;
}

export interface ParsedApiError {
  errorCode: string;
  fieldErrors: FieldErrors;
  message: string;
  requestId: string;
  status: number;
}

export function parseApiError(error: any): ParsedApiError {
  const response = error?.response;
  const responseData = (response?.data ?? {}) as ApiErrorResponse;
  const payloadData = isRecord(responseData.data) ? responseData.data : {};

  return {
    errorCode:
      typeof payloadData.error_code === 'string' ? payloadData.error_code : '',
    fieldErrors: normalizeFieldErrors(payloadData.errors),
    message:
      typeof responseData.message === 'string' ? responseData.message : '',
    requestId:
      typeof responseData.request_id === 'string' ? responseData.request_id : '',
    status: typeof response?.status === 'number' ? response.status : 0,
  };
}
