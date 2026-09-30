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
