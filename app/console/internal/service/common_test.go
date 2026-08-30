package service

import "testing"

func TestResolvePageWithPolicyUsesConfiguredDefaultAndMaximum(t *testing.T) {
	if page, size := resolvePage(ListRequest{}); page != 1 || size != 20 {
		t.Fatalf("legacy resolvePage defaults changed: page=%d size=%d", page, size)
	}
	policy := NewPagePolicy(25, 60)
	if page, size := resolvePageWithPolicy(ListRequest{}, policy); page != 1 || size != 25 {
		t.Fatalf("unexpected defaults: page=%d size=%d", page, size)
	}
	if page, size := resolvePageWithPolicy(ListRequest{Page: 2, PageSize: 100}, policy); page != 2 || size != 60 {
		t.Fatalf("page size was not capped: page=%d size=%d", page, size)
	}
	if page, size := resolvePageWithPolicy(ListRequest{Page: 2, Limit: 80}, policy); page != 2 || size != 60 {
		t.Fatalf("limit was not capped: page=%d size=%d", page, size)
	}
	if page, size := resolvePageWithPolicy(ListRequest{ListAll: true}, policy); page != 1 || size != 0 {
		t.Fatalf("list_all was not preserved: page=%d size=%d", page, size)
	}
}
