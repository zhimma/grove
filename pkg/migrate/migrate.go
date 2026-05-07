package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Manager struct {
	db  *gorm.DB
	dir string
}

type Status struct {
	Name    string
	Applied bool
}

type appliedMigration struct {
	Name string `gorm:"column:name"`
}

func NewManager(db *gorm.DB, dir string) *Manager {
	return &Manager{
		db:  db,
		dir: dir,
	}
}

func (m *Manager) Init() error {
	return m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error
}

func (m *Manager) Up() (int, error) {
	if err := m.Init(); err != nil {
		return 0, err
	}

	applied, err := m.appliedSet()
	if err != nil {
		return 0, err
	}

	files, err := listFiles(m.dir, ".up.sql")
	if err != nil {
		return 0, err
	}

	appliedCount := 0
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".up.sql")
		if applied[name] {
			continue
		}

		body, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return appliedCount, err
		}

		tx := m.db.Begin()
		if err := tx.Exec(string(body)).Error; err != nil {
			tx.Rollback()
			return appliedCount, err
		}
		if err := tx.Exec(
			`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`,
			name,
			time.Now(),
		).Error; err != nil {
			tx.Rollback()
			return appliedCount, err
		}
		if err := tx.Commit().Error; err != nil {
			return appliedCount, err
		}
		appliedCount++
	}

	return appliedCount, nil
}

func (m *Manager) Down() (string, error) {
	if err := m.Init(); err != nil {
		return "", err
	}

	var last struct {
		Name string `gorm:"column:name"`
	}
	if err := m.db.Raw(`
		SELECT name
		FROM schema_migrations
		ORDER BY applied_at DESC, name DESC
		LIMIT 1
	`).Scan(&last).Error; err != nil {
		return "", err
	}
	if last.Name == "" {
		return "", nil
	}

	downFile := filepath.Join(m.dir, last.Name+".down.sql")
	body, err := os.ReadFile(filepath.Clean(downFile))
	if err != nil {
		return "", err
	}

	tx := m.db.Begin()
	if err := tx.Exec(string(body)).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	if err := tx.Exec(`DELETE FROM schema_migrations WHERE name = ?`, last.Name).Error; err != nil {
		tx.Rollback()
		return "", err
	}
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return last.Name, nil
}

func (m *Manager) Status() ([]Status, error) {
	if err := m.Init(); err != nil {
		return nil, err
	}

	applied, err := m.appliedSet()
	if err != nil {
		return nil, err
	}

	files, err := listFiles(m.dir, ".up.sql")
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(files))
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".up.sql")
		statuses = append(statuses, Status{
			Name:    name,
			Applied: applied[name],
		})
	}
	return statuses, nil
}

func CreateFiles(dir, name string) (string, string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", err
	}

	name = sanitizeName(name)
	if name == "" {
		return "", "", fmt.Errorf("migration name is required")
	}

	prefix := time.Now().Format("20060102150405")
	upPath := filepath.Join(dir, prefix+"_"+name+".up.sql")
	downPath := filepath.Join(dir, prefix+"_"+name+".down.sql")

	if err := os.WriteFile(filepath.Clean(upPath), []byte("-- Write your UP migration here.\n"), 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(filepath.Clean(downPath), []byte("-- Write your DOWN migration here.\n"), 0o600); err != nil {
		return "", "", err
	}

	return upPath, downPath, nil
}

func RunSQLDir(db *gorm.DB, dir string) (int, error) {
	files, err := listFiles(dir, ".sql")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, file := range files {
		body, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return count, err
		}
		if err := db.Exec(string(body)).Error; err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (m *Manager) appliedSet() (map[string]bool, error) {
	var rows []appliedMigration
	if err := m.db.Raw(`SELECT name FROM schema_migrations ORDER BY name ASC`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(rows))
	for _, row := range rows {
		result[row.Name] = true
	}
	return result, nil
}

func listFiles(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var out strings.Builder
	lastUnderscore := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && out.Len() > 0 {
			out.WriteRune('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(out.String(), "_")
}
