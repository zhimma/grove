package migrate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateFilesSanitizesUnsafeMigrationName(t *testing.T) {
	dir := t.TempDir()

	upPath, downPath, err := CreateFiles(dir, "../Create Demo-Table!")
	if err != nil {
		t.Fatalf("CreateFiles returned error: %v", err)
	}

	for _, path := range []string{upPath, downPath} {
		if filepath.Dir(path) != dir {
			t.Fatalf("migration path escaped target dir: %s", path)
		}
		base := filepath.Base(path)
		if strings.Contains(base, "..") || strings.Contains(base, "-") || strings.Contains(base, "!") {
			t.Fatalf("migration filename was not sanitized: %s", base)
		}
	}

	if !strings.Contains(filepath.Base(upPath), "_create_demo_table.up.sql") {
		t.Fatalf("unexpected up migration name: %s", upPath)
	}
	if !strings.Contains(filepath.Base(downPath), "_create_demo_table.down.sql") {
		t.Fatalf("unexpected down migration name: %s", downPath)
	}
}
