package database

import (
	"testing"

	"gorm.io/gorm"
)

func TestConnectionsSupportNamedResources(t *testing.T) {
	defaultDB := &gorm.DB{}
	ordersDB := &gorm.DB{}

	dbs := NewConnectionsFromDBs(defaultDB, map[string]*gorm.DB{
		"orders": ordersDB,
	})

	if got := dbs.Default(); got != defaultDB {
		t.Fatal("expected default database to match")
	}

	got, err := dbs.Get("orders")
	if err != nil {
		t.Fatalf("get orders db: %v", err)
	}
	if got != ordersDB {
		t.Fatal("expected orders database to match")
	}

	if !dbs.Has("default") || !dbs.Has("orders") {
		t.Fatal("expected dbs to report configured resources")
	}
}
