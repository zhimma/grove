package service

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

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
