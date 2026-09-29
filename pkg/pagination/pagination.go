// Package pagination resolves list query parameters into a bounded page and
// builds the meta block list responses carry.
package pagination

import "gorm.io/gorm"

const (
	defaultSize = 20
	maxSize     = 100
)

// Policy bounds the page size a caller may ask for. The zero value allows 20
// rows per page and at most 100.
type Policy struct {
	Default int
	Max     int
}

// Request is the pagination part of a list query: page and page_size, offset
// and limit, or list_all for every row. Sizes above the policy's Max are
// capped, not rejected, so the limit follows api.max_per_page.
type Request struct {
	Page     int  `form:"page" binding:"omitempty,min=1" label:"页码"`
	PageSize int  `form:"page_size" binding:"omitempty,min=1" label:"每页条数"`
	Offset   int  `form:"offset" label:"偏移量"`
	Limit    int  `form:"limit" label:"限制条数"`
	ListAll  bool `form:"list_all" label:"是否返回全部"`
}

// Page is a resolved Request. Size 0 means every row.
type Page struct {
	Number int
	Size   int
	Offset int
}

// Resolve bounds r by the policy. limit takes precedence over page_size and
// an explicit offset over the one page implies.
func (p Policy) Resolve(r Request) Page {
	limit := p.Max
	if limit <= 0 {
		limit = maxSize
	}
	fallback := p.Default
	if fallback <= 0 {
		fallback = defaultSize
	}

	number := max(r.Page, 1)
	if r.ListAll {
		return Page{Number: number}
	}
	size := r.PageSize
	if r.Limit > 0 {
		size = r.Limit
	}
	if size <= 0 {
		size = fallback
	}
	size = min(size, limit)
	offset := r.Offset
	if offset <= 0 {
		offset = (number - 1) * size
	}
	return Page{Number: number, Size: size, Offset: offset}
}

// Apply limits query to the page and leaves a list_all page unbounded.
func (p Page) Apply(query *gorm.DB) *gorm.DB {
	if p.Size == 0 {
		return query
	}
	return query.Offset(p.Offset).Limit(p.Size)
}

// Meta is the pagination block of a list response.
type Meta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// NewMeta describes page within total matching rows.
func NewMeta(total int64, page Page) Meta {
	totalPages := 0
	if page.Size > 0 {
		totalPages = int((total + int64(page.Size) - 1) / int64(page.Size))
	}
	return Meta{Total: total, Page: page.Number, PageSize: page.Size, TotalPages: totalPages}
}
