package model

import (
	"encoding/json"
	"strconv"
)

type SystemConfig struct {
	Base
	ConfigGroup  string `gorm:"size:64;index:idx_system_configs_group_key,unique;not null" json:"config_group"`
	ConfigKey    string `gorm:"size:120;index:idx_system_configs_group_key,unique;not null" json:"config_key"`
	Name         string `gorm:"size:120;not null" json:"name"`
	Description  string `gorm:"size:255;not null;default:''" json:"description"`
	ValueType    string `gorm:"size:20;not null;default:'string'" json:"value_type"`
	Value        string `gorm:"type:text;not null;default:''" json:"value"`
	DefaultValue string `gorm:"type:text;not null;default:''" json:"default_value"`
	IsEditable   bool   `gorm:"not null;default:true" json:"is_editable"`
	IsSystem     bool   `gorm:"not null;default:false" json:"is_system"`
	SortOrder    int    `gorm:"not null;default:0" json:"sort_order"`
}

func (SystemConfig) TableName() string {
	return "system_configs"
}

func (c SystemConfig) EffectiveValue() string {
	if c.Value != "" {
		return c.Value
	}
	return c.DefaultValue
}

func (c SystemConfig) IntValue() (int, error) {
	if c.EffectiveValue() == "" {
		return 0, nil
	}
	return strconv.Atoi(c.EffectiveValue())
}

func (c SystemConfig) BoolValue() (bool, error) {
	if c.EffectiveValue() == "" {
		return false, nil
	}
	return strconv.ParseBool(c.EffectiveValue())
}

func (c SystemConfig) JSONValue(target any) error {
	if c.EffectiveValue() == "" {
		return nil
	}
	return json.Unmarshal([]byte(c.EffectiveValue()), target)
}
