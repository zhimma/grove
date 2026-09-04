package datatype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type StringArray []string

func NewStringArray(values []string) StringArray {
	if values == nil {
		return StringArray{}
	}
	result := make(StringArray, len(values))
	copy(result, values)
	return result
}

func (s *StringArray) Scan(value any) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}

	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return fmt.Errorf("unsupported string array scan type %T", value)
	}

	return json.Unmarshal(raw, s)
}

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal([]string(s))
}

func (StringArray) GormDataType() string {
	return "json"
}

func (StringArray) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db == nil || db.Dialector == nil {
		return "json"
	}
	switch db.Dialector.Name() {
	case "postgres":
		return "jsonb"
	case "mysql":
		return "json"
	default:
		return "json"
	}
}
