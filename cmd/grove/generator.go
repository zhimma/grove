package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhimma/grove/pkg/migrate"
)

const (
	routeMarker           = "\t// grove:register-routes\n"
	operationMarker       = "\t// grove:register-operations\n"
	generatorLockFilename = ".grove-generate.lock"
	migrationsDir         = "database/migrations"
	routerFile            = "app/console/internal/router/router.go"
	contractFile          = "app/console/internal/docs/contract.go"
)

type generatedSource struct {
	path    string
	content []byte
}

// fileEdit is a line inserted into an existing file at a grove marker. The
// original is kept so a failed generation can put the file back.
type fileEdit struct {
	path     string
	original []byte
	updated  []byte
}

// generateConsoleModule writes a complete vertical slice: model, CRUD service
// with its test, handler, response, OpenAPI operations and a migration in every
// dialect, then registers the routes and operations. Either all of it lands or
// none of it does, and the result has to pass make contracts as generated.
func generateConsoleModule(input, fieldSpec, label string) ([]string, error) {
	module, err := modulePath()
	if err != nil {
		return nil, err
	}
	fields, err := parseFields(fieldSpec)
	if err != nil {
		return nil, err
	}
	spec, err := newModuleSpec(module, input, label, fields)
	if err != nil {
		return nil, err
	}
	sources, err := renderModuleSources(spec)
	if err != nil {
		return nil, err
	}
	migrations, err := renderMigrations(spec)
	if err != nil {
		return nil, err
	}

	var created []string
	err = withGeneratorLock(routerFile, func() error {
		for _, source := range sources {
			if err := ensureFileAbsent(source.path); err != nil {
				return err
			}
		}
		edits, err := prepareEdits(spec)
		if err != nil {
			return err
		}
		created, err = commitGeneratedModule(sources, migrations, spec.Table, edits)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func renderModuleSources(spec moduleSpec) ([]generatedSource, error) {
	files := []struct{ path, name, text string }{
		{filepath.Join("internal/model", spec.Snake+".go"), "model", modelTemplate},
		{filepath.Join("app/console/internal/service", spec.Snake+".go"), "service", serviceTemplate},
		{filepath.Join("app/console/internal/service", spec.Snake+"_test.go"), "service test", serviceTestTemplate},
		{filepath.Join("app/console/internal/handler", spec.Snake+".go"), "handler", handlerTemplate},
		{filepath.Join("app/console/internal/handler", spec.Snake+"_response.go"), "response", responseTemplate},
		{filepath.Join("app/console/internal/docs", spec.Snake+".go"), "docs", docsTemplate},
	}
	sources := make([]generatedSource, 0, len(files))
	for _, file := range files {
		content, err := renderGo(file.name, file.text, spec)
		if err != nil {
			return nil, err
		}
		sources = append(sources, generatedSource{path: file.path, content: content})
	}
	return sources, nil
}

func renderMigrations(spec moduleSpec) (map[string]migrate.SQL, error) {
	postgresUp, err := render("postgres migration", postgresUpTemplate, spec)
	if err != nil {
		return nil, err
	}
	mysqlUp, err := render("mysql migration", mysqlUpTemplate, spec)
	if err != nil {
		return nil, err
	}
	down, err := render("down migration", dropTableTemplate, spec)
	if err != nil {
		return nil, err
	}
	bodies := map[string]migrate.SQL{
		"postgres": {Up: string(postgresUp), Down: string(down)},
		"mysql":    {Up: string(mysqlUp), Down: string(down)},
	}
	// A dialect added to migrate.Dialects without a template here would get
	// placeholder SQL and a table that never exists; fail instead.
	for _, dialect := range migrate.Dialects {
		if _, ok := bodies[dialect]; !ok {
			return nil, fmt.Errorf("缺少 %s 方言的迁移模板", dialect)
		}
	}
	return bodies, nil
}

func prepareEdits(spec moduleSpec) ([]fileEdit, error) {
	router, err := insertAtMarker(routerFile, routeMarker,
		fmt.Sprintf("\thandler.Register%sRoutes(protected, r.p.DB, pagePolicies, catalog)\n", spec.Name))
	if err != nil {
		return nil, err
	}
	contract, err := insertAtMarker(contractFile, operationMarker,
		fmt.Sprintf("\tadd%sOperations(&doc)\n", spec.Name))
	if err != nil {
		return nil, err
	}
	return []fileEdit{router, contract}, nil
}

func insertAtMarker(path, marker, line string) (fileEdit, error) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fileEdit{}, err
	}
	edit := fileEdit{path: path, original: body, updated: body}
	if bytes.Contains(body, []byte(line)) {
		return edit, nil
	}
	if !bytes.Contains(body, []byte(marker)) {
		return fileEdit{}, fmt.Errorf("未在 %s 中找到标记 %q", path, strings.TrimSpace(marker))
	}
	edit.updated = bytes.Replace(body, []byte(marker), []byte(line+marker), 1)
	return edit, nil
}

// withGeneratorLock serializes the read-modify-write of router.go across CLI
// processes. Without it, two different module generations can both read the
// same marker and the later atomic rename silently drops the earlier route.
func withGeneratorLock(routerPath string, action func() error) error {
	lockPath := filepath.Join(filepath.Dir(routerPath), generatorLockFilename)
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			defer func() { _ = os.Remove(lockPath) }()
			return action()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create generator lock: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for generator lock %s", lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ensureFileAbsent(path string) error {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return fmt.Errorf("文件已存在: %s", path)
	case os.IsNotExist(err):
		return nil
	default:
		return err
	}
}

func commitGeneratedModule(sources []generatedSource, migrations map[string]migrate.SQL, table string, edits []fileEdit) ([]string, error) {
	created := make([]string, 0, len(sources)+len(migrate.Dialects)*2)
	var applied []fileEdit
	rollback := func() {
		for _, edit := range applied {
			_ = replaceFileAtomic(edit.path, edit.original)
		}
		for _, path := range created {
			_ = os.Remove(path)
		}
	}

	for _, source := range sources {
		if err := writeNewFileAtomic(source.path, source.content, 0o600); err != nil {
			rollback()
			return nil, err
		}
		created = append(created, source.path)
	}
	paths, err := migrate.CreateFiles(migrationsDir, "create_"+table, migrations)
	if err != nil {
		rollback()
		return nil, err
	}
	created = append(created, paths...)
	for _, edit := range edits {
		if err := replaceFileAtomic(edit.path, edit.updated); err != nil {
			rollback()
			return nil, err
		}
		applied = append(applied, edit)
	}
	return created, nil
}

func writeNewFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tempPath, err := writeTempFile(dir, content, mode)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempPath) }()
	if err := os.Link(tempPath, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("文件已存在: %s", path)
		}
		return err
	}
	return nil
}

func replaceFileAtomic(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tempPath, err := writeTempFile(filepath.Dir(path), content, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempPath) }()
	return os.Rename(tempPath, path)
}

func writeTempFile(dir string, content []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, ".grove-generate-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(mode); err != nil {
		cleanup()
		return "", err
	}
	if _, err := file.Write(content); err != nil {
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
