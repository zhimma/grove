package handler

import "github.com/zhimma/grove/pkg/pagination"

type SortQuery struct {
	OrderBy []string `form:"order_by" label:"排序字段"`
}

type TimeRangeQuery struct {
	CreatedFrom string `form:"created_from" label:"创建开始时间"`
	CreatedTo   string `form:"created_to" label:"创建结束时间"`
}

type ListQuery struct {
	pagination.Request
	SortQuery
	TimeRangeQuery
	Keyword string `form:"keyword" label:"关键词"`
}
