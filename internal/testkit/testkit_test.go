package testkit

import "testing"

type widget struct {
	ID   uint
	Name string
}

func TestOpenDBMigratesAndIsolatesEachTest(t *testing.T) {
	first := OpenDB(t, &widget{})
	if err := first.Create(&widget{Name: "a"}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	second := OpenDB(t, &widget{})
	var count int64
	if err := second.Model(&widget{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("second database sees %d rows from the first", count)
	}
}
