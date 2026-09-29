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
	migratedatabase "github.com/golang-migrate/migrate/v4/database"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

const (
	metadataTable              = "grove_migrations"
	migrationCreateLockName    = ".grove-migration-create.lock"
	migrationCreateLockTimeout = 5 * time.Second
)

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
	dir, err := m.migrationDir()
	if err != nil {
		return 0, err
	}
	files, err := listMigrations(dir)
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
	dir, err := m.migrationDir()
	if err != nil {
		return "", err
	}
	files, err := listMigrations(dir)
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
	if err := validateDownMigration(m.db, name); err != nil {
		return "", err
	}
	if err := engine.Steps(-1); err != nil && !errors.Is(err, golangmigrate.ErrNoChange) {
		return "", err
	}
	return name, nil
}

func validateDownMigration(db *gorm.DB, name string) error {
	if name != "202604150011_add_system_config_secrets" {
		return nil
	}
	if db == nil {
		return fmt.Errorf("migration database is required")
	}
	var hasSecrets bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM system_configs WHERE is_secret = TRUE)`).Scan(&hasSecrets).Error; err != nil {
		return fmt.Errorf("check encrypted system configs before down migration: %w", err)
	}
	if hasSecrets {
		return fmt.Errorf("cannot remove is_secret while encrypted system configs exist")
	}
	return nil
}

func (m *Manager) Status() ([]Status, error) {
	dir, err := m.migrationDir()
	if err != nil {
		return nil, err
	}
	files, err := listMigrations(dir)
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
	driverName := normalizeDriver(m.db.Dialector.Name())
	var migrationDriver migratedatabase.Driver
	switch driverName {
	case "postgres":
		migrationDriver, err = postgres.WithConnection(context.Background(), conn, &postgres.Config{
			MigrationsTable: metadataTable,
		})
	case "mysql":
		migrationDriver, err = migratemysql.WithConnection(context.Background(), conn, &migratemysql.Config{
			MigrationsTable: metadataTable,
		})
	default:
		_ = conn.Close()
		return nil, nil, fmt.Errorf("unsupported migration database driver: %s", m.db.Dialector.Name())
	}
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	dir, err := m.migrationDir()
	if err != nil {
		_ = migrationDriver.Close()
		return nil, nil, err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		_ = migrationDriver.Close()
		return nil, nil, err
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absDir)}).String()
	engine, err := golangmigrate.NewWithDatabaseInstance(sourceURL, driverName, migrationDriver)
	if err != nil {
		_ = migrationDriver.Close()
		return nil, nil, err
	}
	return engine, func() {
		_, _ = engine.Close()
	}, nil
}

func (m *Manager) migrationDir() (string, error) {
	if m == nil || m.db == nil {
		return "", fmt.Errorf("migration database is required")
	}
	return ResolveDialectDir(m.dir, m.db.Dialector.Name())
}

// ResolveDialectDir resolves a dialect-specific directory while keeping explicit
// concrete directories and the legacy PostgreSQL root directory compatible.
func ResolveDialectDir(baseDir, driver string) (string, error) {
	return ResolveDialectDirWithSuffix(baseDir, driver, ".up.sql")
}

// ResolveDialectDirWithSuffix resolves a dialect directory using the given
// file suffix. It is used by both migrations and SQL seed directories.
func ResolveDialectDirWithSuffix(baseDir, driver, suffix string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return "", fmt.Errorf("migration or seed directory is required")
	}
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return "", fmt.Errorf("migration or seed file suffix is required")
	}
	driver = normalizeDriver(driver)
	if driver != "postgres" && driver != "mysql" {
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}

	if files, err := listFiles(baseDir, suffix); err == nil && len(files) > 0 {
		return baseDir, nil
	}
	dialectDir := filepath.Join(baseDir, driver)
	if info, err := os.Stat(dialectDir); err == nil && info.IsDir() {
		return dialectDir, nil
	}
	return "", fmt.Errorf("database %s directory not found under %s", driver, baseDir)
}

func normalizeDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "postgresql" {
		return "postgres"
	}
	return driver
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

// Dialects are the migration trees every schema change must cover. The
// dialect-parity test holds them to identical version sets.
var Dialects = []string{"postgres", "mysql"}

// SQL is the up and down body of one migration in one dialect.
type SQL struct {
	Up   string
	Down string
}

// CreateFiles writes one migration pair per dialect under baseDir, all sharing
// one version. bodies maps a dialect to its SQL; a dialect without an entry
// gets placeholder comments. It returns every created path, postgres first.
//
// Creating the pair only in the configured dialect used to leave the other
// tree without that version, which the dialect-parity test rejects on the spot.
func CreateFiles(baseDir, name string, bodies map[string]SQL) ([]string, error) {
	name = sanitizeName(name)
	if name == "" {
		return nil, fmt.Errorf("migration name is required")
	}
	for _, dialect := range Dialects {
		if err := os.MkdirAll(filepath.Join(baseDir, dialect), 0o750); err != nil {
			return nil, err
		}
	}

	var created []string
	err := withMigrationCreateLock(baseDir, func() error {
		paths, err := createDialectMigrations(baseDir, name, bodies)
		created = paths
		return err
	})
	return created, err
}

func createDialectMigrations(baseDir, name string, bodies map[string]SQL) ([]string, error) {
	baseVersion := time.Now().Unix()
	for attempt := int64(0); attempt < 1000; attempt++ {
		version := time.Unix(baseVersion+attempt, 0).Format("20060102150405")
		created, err := createVersion(baseDir, version, name, bodies)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not allocate a unique migration version")
}

// createVersion publishes every dialect's pair at one version, or none of
// them: a version already taken in any dialect rolls back the ones written.
func createVersion(baseDir, version, name string, bodies map[string]SQL) ([]string, error) {
	created := make([]string, 0, len(Dialects)*2)
	for _, dialect := range Dialects {
		body, ok := bodies[dialect]
		if !ok {
			body = SQL{Up: "-- Write your UP migration here.\n", Down: "-- Write your DOWN migration here.\n"}
		}
		dir := filepath.Join(baseDir, dialect)
		upPath := filepath.Join(dir, version+"_"+name+".up.sql")
		downPath := filepath.Join(dir, version+"_"+name+".down.sql")
		if err := createMigrationPair(upPath, downPath, body.Up, body.Down); err != nil {
			for _, path := range created {
				_ = os.Remove(path)
			}
			return nil, err
		}
		created = append(created, upPath, downPath)
	}
	return created, nil
}

// createMigrationPair prepares both files before publishing either final
// filename. os.Link gives us O_EXCL-style publication without allowing a
// concurrent process to overwrite an existing migration.
func createMigrationPair(upPath, downPath, upContent, downContent string) error {
	upTemp, err := writeMigrationTemp(filepath.Dir(upPath), upContent)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(upTemp) }()

	downTemp, err := writeMigrationTemp(filepath.Dir(downPath), downContent)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(downTemp) }()

	if err := os.Link(upTemp, upPath); err != nil {
		return err
	}
	if err := os.Link(downTemp, downPath); err != nil {
		_ = os.Remove(upPath)
		return err
	}
	return nil
}

func writeMigrationTemp(dir, content string) (string, error) {
	file, err := os.CreateTemp(dir, ".grove-migration-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func withMigrationCreateLock(dir string, action func() error) error {
	lockPath := filepath.Join(dir, migrationCreateLockName)
	deadline := time.Now().Add(migrationCreateLockTimeout)
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			defer func() { _ = os.Remove(lockPath) }()
			return action()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create migration lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for migration creation lock %s", lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
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
