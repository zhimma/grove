package datatype

import (
	"database/sql/driver"
	"encoding/json"
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

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, s)
}

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal([]string(s))
}
