package database

import (
	"testing"

	"gorm.io/gorm"
)

func TestRepoSupportsNamedResources(t *testing.T) {
	defaultDB := &gorm.DB{}
	ordersDB := &gorm.DB{}

	repo := NewRepoWithConnections(defaultDB, map[string]*gorm.DB{
		"orders": ordersDB,
	})

	if got := repo.Default(); got != defaultDB {
		t.Fatal("expected default database to match")
	}

	got, err := repo.Get("orders")
	if err != nil {
		t.Fatalf("get orders db: %v", err)
	}
	if got != ordersDB {
		t.Fatal("expected orders database to match")
	}

	if !repo.Has("default") || !repo.Has("orders") {
		t.Fatal("expected repo to report configured resources")
	}
}
