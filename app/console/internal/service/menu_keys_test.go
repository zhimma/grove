package service

import (
	"fmt"
	"strings"
	"testing"
)

func TestNormalizeConsoleMenuKeysPreservesUnknownKeys(t *testing.T) {
	keys := normalizeConsoleMenuKeys([]string{
		"ConsoleDashboard",
		" legacy.invalid ",
		"ConsoleFuture",
		"ConsoleDashboard",
		"",
	})
	want := []string{"ConsoleDashboard", "legacy.invalid", "ConsoleFuture"}
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		t.Fatalf("normalized keys = %v, want %v", keys, want)
	}
}

func TestNormalizeConsoleMenuKeysCollapsesWildcard(t *testing.T) {
	keys := normalizeConsoleMenuKeys([]string{"ConsoleDashboard", "*", "ConsoleRoles"})
	if len(keys) != 1 || keys[0] != "*" {
		t.Fatalf("unexpected wildcard normalization: %v", keys)
	}
}

func TestValidateConsoleMenuKeysAcceptsNewFrontendRouteName(t *testing.T) {
	if err := validateConsoleMenuKeys([]string{"ConsoleFuture", "reports.monthly:v2"}); err != nil {
		t.Fatalf("valid frontend route names should pass: %v", err)
	}
}

func TestValidateConsoleMenuKeysRejectsInvalidFormatAndLimits(t *testing.T) {
	if err := validateConsoleMenuKeys([]string{"bad key"}); err == nil {
		t.Fatal("expected invalid format error")
	}
	if err := validateConsoleMenuKeys([]string{strings.Repeat("a", maxConsoleMenuKeyLength+1)}); err == nil {
		t.Fatal("expected key length error")
	}

	keys := make([]string, 0, maxConsoleMenuKeys+1)
	for i := 0; i <= maxConsoleMenuKeys; i++ {
		keys = append(keys, fmt.Sprintf("ConsoleMenu%d", i))
	}
	if err := validateConsoleMenuKeys(keys); err == nil {
		t.Fatal("expected key count error")
	}
}
