export interface PageParams {
  page?: number;
  page_size?: number;
}

export interface ListMeta {
  total: number;
  page: number;
  page_size: number;
  total_pages?: number;
}

export interface PageData<T> {
  list: T[];
  meta: ListMeta;
}

export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
  request_id: string;
}

export type FieldErrors = Record<string, string[]>;

export interface ApiErrorData {
  error_code?: string;
  errors?: FieldErrors;
}

export interface ApiErrorResponse {
  code: number;
  data?: ApiErrorData;
  message: string;
  request_id?: string;
}

// 通用的列表请求参数
export interface ListParams extends PageParams {
  keyword?: string;
}
