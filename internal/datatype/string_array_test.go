package datatype

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestStringArrayScanSupportsBytesAndString(t *testing.T) {
	for name, input := range map[string]any{
		"bytes":  []byte(`["a","b"]`),
		"string": `["a","b"]`,
	} {
		t.Run(name, func(t *testing.T) {
			var values StringArray
			if err := values.Scan(input); err != nil {
				t.Fatalf("scan string array: %v", err)
			}
			if len(values) != 2 || values[0] != "a" || values[1] != "b" {
				t.Fatalf("unexpected values: %#v", values)
			}
		})
	}
}

func TestStringArrayGormDBDataTypeFollowsDialect(t *testing.T) {
	tests := []struct {
		name      string
		dialector gorm.Dialector
		want      string
	}{
		{name: "postgres", dialector: postgres.Open(""), want: "jsonb"},
		{name: "mysql", dialector: mysql.Open(""), want: "json"},
		{name: "sqlite", dialector: sqlite.Open(":memory:"), want: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &gorm.DB{Config: &gorm.Config{Dialector: tt.dialector}}
			if got := (StringArray{}).GormDBDataType(db, nil); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
