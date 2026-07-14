package migrate

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	golangmigrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

const metadataTable = "grove_migrations"

type Manager struct {
	db  *gorm.DB
	dir string
}

type Status struct {
	Name    string
	Applied bool
}

type migrationFile struct {
	Version uint
	Name    string
}

func NewManager(db *gorm.DB, dir string) *Manager {
	return &Manager{
		db:  db,
		dir: dir,
	}
}

func (m *Manager) Up() (int, error) {
	files, err := listMigrations(m.dir)
	if err != nil {
		return 0, err
	}
	engine, closeEngine, err := m.openEngine()
	if err != nil {
		return 0, err
	}
	defer closeEngine()

	before, _, err := migrationVersion(engine)
	if err != nil {
		return 0, err
	}
	if err := engine.Up(); err != nil && !errors.Is(err, golangmigrate.ErrNoChange) {
		return 0, err
	}
	after, _, err := migrationVersion(engine)
	if err != nil {
		return 0, err
	}
	return countApplied(files, before, after), nil
}

func (m *Manager) Down() (string, error) {
	files, err := listMigrations(m.dir)
	if err != nil {
		return "", err
	}
	engine, closeEngine, err := m.openEngine()
	if err != nil {
		return "", err
	}
	defer closeEngine()

	current, hasVersion, err := migrationVersion(engine)
	if err != nil {
		return "", err
	}
	if !hasVersion {
		return "", nil
	}
	name := migrationName(files, current)
	if name == "" {
		return "", fmt.Errorf("migration version %d has no matching file", current)
	}
	if err := engine.Steps(-1); err != nil && !errors.Is(err, golangmigrate.ErrNoChange) {
		return "", err
	}
	return name, nil
}

func (m *Manager) Status() ([]Status, error) {
	files, err := listMigrations(m.dir)
	if err != nil {
		return nil, err
	}
	engine, closeEngine, err := m.openEngine()
	if err != nil {
		return nil, err
	}
	defer closeEngine()

	current, hasVersion, err := migrationVersion(engine)
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(files))
	for _, file := range files {
		statuses = append(statuses, Status{
			Name:    file.Name,
			Applied: hasVersion && file.Version <= current,
		})
	}
	return statuses, nil
}

func (m *Manager) openEngine() (*golangmigrate.Migrate, func(), error) {
	if m.db == nil {
		return nil, nil, fmt.Errorf("migration database is required")
	}
	sqlDB, err := m.db.DB()
	if err != nil {
		return nil, nil, err
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return nil, nil, err
	}
	driver, err := postgres.WithConnection(context.Background(), conn, &postgres.Config{
		MigrationsTable: metadataTable,
	})
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	absDir, err := filepath.Abs(m.dir)
	if err != nil {
		_ = driver.Close()
		return nil, nil, err
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absDir)}).String()
	engine, err := golangmigrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		_ = driver.Close()
		return nil, nil, err
	}
	return engine, func() {
		_, _ = engine.Close()
	}, nil
}

func migrationVersion(engine *golangmigrate.Migrate) (uint, bool, error) {
	version, dirty, err := engine.Version()
	if errors.Is(err, golangmigrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if dirty {
		return 0, false, fmt.Errorf("migration version %d is dirty", version)
	}
	return version, true, nil
}

func listMigrations(dir string) ([]migrationFile, error) {
	paths, err := listFiles(dir, ".up.sql")
	if err != nil {
		return nil, err
	}
	files := make([]migrationFile, 0, len(paths))
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".up.sql")
		versionText, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", filepath.Base(path))
		}
		version, err := strconv.ParseUint(versionText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid migration version in %q: %w", filepath.Base(path), err)
		}
		files = append(files, migrationFile{Version: uint(version), Name: name})
	}
	return files, nil
}

func countApplied(files []migrationFile, before uint, after uint) int {
	count := 0
	for _, file := range files {
		if file.Version > before && file.Version <= after {
			count++
		}
	}
	return count
}

func migrationName(files []migrationFile, version uint) string {
	for _, file := range files {
		if file.Version == version {
			return file.Name
		}
	}
	return ""
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
	return RunSQLDirWithReplacements(db, dir, nil)
}

// RunSQLDirWithReplacements executes SQL files after replacing trusted placeholders.
func RunSQLDirWithReplacements(db *gorm.DB, dir string, replacements map[string]string) (int, error) {
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
		content := string(body)
		for placeholder, value := range replacements {
			content = strings.ReplaceAll(content, placeholder, value)
		}
		if err := db.Exec(content).Error; err != nil {
			return count, err
		}
		count++
	}
	return count, nil
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
