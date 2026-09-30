<script setup lang="ts">
import {
  createArticle,
  deleteArticle,
  getArticle,
  getArticleList,
  updateArticle,
  updateArticleStatus,
} from '#/api/article';
import ResourcePage from '#/components/resource-page/index.vue';

const statusOptions = [
  { label: '草稿', value: 0 },
  { label: '已发布', value: 1 },
  { label: '已归档', value: 2 },
];

const columns = [
  { title: '标题', dataIndex: 'title', key: 'title', width: 260 },
  { title: '标识', dataIndex: 'slug', key: 'slug', width: 220 },
  { title: '分类', dataIndex: 'category', key: 'category', width: 120 },
  { title: '状态', dataIndex: 'status_text', key: 'status', width: 100 },
  {
    title: '发布时间',
    dataIndex: 'published_at',
    key: 'published_at',
    width: 180,
  },
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
];
</script>

<template>
  <ResourcePage
    title="文章管理"
    :columns="columns"
    :search-fields="[
      { key: 'keyword', label: '关键词' },
      { key: 'category', label: '分类' },
      { key: 'status', label: '状态', type: 'select', options: statusOptions },
    ]"
    :form-fields="[
      { key: 'title', label: '标题', required: true },
      { key: 'slug', label: '标识' },
      { key: 'category', label: '分类' },
      {
        key: 'cover',
        label: '封面',
        type: 'uploadImg',
        purpose: 'article-cover',
      },
      { key: 'summary', label: '摘要', type: 'textarea' },
      {
        key: 'content',
        label: '内容（支持 Markdown）',
        type: 'textarea',
        required: true,
      },
      { key: 'status', label: '状态', type: 'radio', options: statusOptions },
    ]"
    :fetch-api="getArticleList"
    :get-detail-api="getArticle"
    :create-api="createArticle"
    :update-api="updateArticle"
    :delete-api="deleteArticle"
    :status-api="updateArticleStatus"
  />
</template>
