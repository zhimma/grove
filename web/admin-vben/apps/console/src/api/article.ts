import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface ConsoleArticle {
  id: string;
  title: string;
  slug: string;
  summary?: string;
  content?: string;
  cover?: string;
  category?: string;
  status: number;
  status_text: string;
  published_at?: string;
  author_id?: string;
  created_at: string;
  updated_at: string;
}

export type ConsoleArticleListItem = Omit<ConsoleArticle, 'content'>;

export function getArticleList(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<ConsoleArticleListItem>>(
    consoleEndpoint('consoleListArticles'),
    { params },
  );
}

export function getArticle(id: string) {
  return requestClient.get<ConsoleArticle>(
    consoleEndpoint('consoleGetArticle', { id }),
  );
}

export function createArticle(data: Record<string, any>) {
  return requestClient.post<ConsoleArticle>(
    consoleEndpoint('consoleCreateArticle'),
    data,
  );
}

export function updateArticle(id: string, data: Record<string, any>) {
  return requestClient.put<ConsoleArticle>(
    consoleEndpoint('consoleUpdateArticle', { id }),
    data,
  );
}

export function updateArticleStatus(id: string, status: number) {
  return requestClient.put<ConsoleArticle>(
    consoleEndpoint('consoleUpdateArticleStatus', { id }),
    { status },
  );
}

export function deleteArticle(id: string) {
  return requestClient.delete(consoleEndpoint('consoleDeleteArticle', { id }));
}
