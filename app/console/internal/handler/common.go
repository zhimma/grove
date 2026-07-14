package handler

type PaginationQuery struct {
	Page     int  `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int  `form:"page_size" binding:"omitempty,min=1,max=100" label:"每页条数"`
	Offset   int  `form:"offset" label:"偏移量"`
	Limit    int  `form:"limit" label:"限制条数"`
	ListAll  bool `form:"list_all" label:"是否返回全部"`
}

type SortQuery struct {
	OrderBy []string `form:"order_by" label:"排序字段"`
}

type TimeRangeQuery struct {
	CreatedFrom string `form:"created_from" label:"创建开始时间"`
	CreatedTo   string `form:"created_to" label:"创建结束时间"`
}

type ListQuery struct {
	PaginationQuery
	SortQuery
	TimeRangeQuery
	Keyword string `form:"keyword" label:"关键词"`
}

type ListMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}
