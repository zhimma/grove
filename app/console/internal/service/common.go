package service

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type ListRequest struct {
	Page     int
	PageSize int
	Offset   int
	Limit    int
	ListAll  bool
}

// PagePolicy is the executable pagination contract shared by Console list
// services. Values are copied into each service so different applications do
// not rely on mutable process-global defaults.
type PagePolicy struct {
	Default int
	Max     int
}

func NewPagePolicy(defaultPerPage, maxPerPage int) PagePolicy {
	if defaultPerPage <= 0 {
		defaultPerPage = 20
	}
	if maxPerPage <= 0 {
		maxPerPage = 100
	}
	if defaultPerPage > maxPerPage {
		defaultPerPage = maxPerPage
	}
	return PagePolicy{Default: defaultPerPage, Max: maxPerPage}
}

func pagePolicyFromArgs(policies []PagePolicy) PagePolicy {
	if len(policies) == 0 {
		return NewPagePolicy(20, 100)
	}
	return NewPagePolicy(policies[0].Default, policies[0].Max)
}

type ListMeta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

func NewListMeta(total int64, page, pageSize int) ListMeta {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return ListMeta{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

func resolvePage(input ListRequest) (int, int) {
	return resolvePageWithPolicy(input, NewPagePolicy(20, 100))
}

func resolvePageWithPolicy(input ListRequest, policy PagePolicy) (int, int) {
	policy = NewPagePolicy(policy.Default, policy.Max)
	page := input.Page
	if page <= 0 {
		page = 1
	}

	pageSize := input.PageSize
	if input.Limit > 0 {
		pageSize = input.Limit
	}
	if pageSize <= 0 {
		pageSize = policy.Default
	}
	if policy.Max > 0 && pageSize > policy.Max {
		pageSize = policy.Max
	}
	if input.ListAll {
		pageSize = 0
	}

	return page, pageSize
}

func parseOrderBy(input string) (string, string) {
	item := strings.TrimSpace(input)
	if item == "" {
		return "", ""
	}

	direction := "ASC"
	if strings.HasPrefix(item, "-") {
		item = strings.TrimPrefix(item, "-")
		direction = "DESC"
	} else if strings.HasPrefix(item, "+") {
		item = strings.TrimPrefix(item, "+")
	}

	field := strings.ToLower(strings.TrimSpace(item))
	if field == "" {
		return "", ""
	}
	return field, direction
}

func applyTimeRange(query *gorm.DB, column, from, to string) (*gorm.DB, error) {
	if query == nil {
		return query, nil
	}
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	if from != "" {
		parsed, err := parseTimeValue(from, false)
		if err != nil {
			return nil, err
		}
		query = query.Where(column+" >= ?", parsed)
	}
	if to != "" {
		parsed, err := parseTimeValue(to, true)
		if err != nil {
			return nil, err
		}
		query = query.Where(column+" <= ?", parsed)
	}
	return query, nil
}

func parseTimeValue(value string, endOfDay bool) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" && endOfDay {
			return parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second), nil
		}
		return parsed, nil
	}
	return time.Time{}, gorm.ErrInvalidData
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
