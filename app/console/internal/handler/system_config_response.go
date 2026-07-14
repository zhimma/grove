package handler

import "github.com/zhimma/grove/internal/model"

type SystemConfigItem struct {
	ID           string `json:"id"`
	ConfigGroup  string `json:"config_group"`
	ConfigKey    string `json:"config_key"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ValueType    string `json:"value_type"`
	Value        string `json:"value"`
	DefaultValue string `json:"default_value"`
	IsEditable   bool   `json:"is_editable"`
	IsSystem     bool   `json:"is_system"`
	IsSecret     bool   `json:"is_secret"`
	SortOrder    int    `json:"sort_order"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func newSystemConfigItem(item model.SystemConfig) SystemConfigItem {
	return SystemConfigItem{
		ID:           item.ID,
		ConfigGroup:  item.ConfigGroup,
		ConfigKey:    item.ConfigKey,
		Name:         item.Name,
		Description:  item.Description,
		ValueType:    item.ValueType,
		Value:        item.Value,
		DefaultValue: item.DefaultValue,
		IsEditable:   item.IsEditable,
		IsSystem:     item.IsSystem,
		IsSecret:     item.IsSecret,
		SortOrder:    item.SortOrder,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
