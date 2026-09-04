package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"time"
)

const (
	routeMarker           = "\t// grove:register-routes\n"
	generatorLockFilename = ".grove-generate.lock"
)

type generatedSource struct {
	path    string
	content []byte
}

func generateConsoleModule(input string) ([]string, error) {
	module, err := modulePath()
	if err != nil {
		return nil, err
	}
	name := toPascal(input)
	snake := toSnake(input)
	if !isValidGoIdentifier(name) || snake == "" {
		return nil, fmt.Errorf("模块名称 %q 不能转换为合法 Go 标识符", input)
	}

	sources := []generatedSource{
		{path: filepath.Join("internal/model", snake+".go"), content: []byte(modelTemplate(name, snake))},
		{path: filepath.Join("app/console/internal/service", snake+".go"), content: []byte(consoleServiceTemplate(module, name, snake))},
		{path: filepath.Join("app/console/internal/handler", snake+".go"), content: []byte(consoleHandlerTemplate(module, name, snake))},
	}
	routerPath := filepath.Join("app/console/internal/router", "router.go")
	line := fmt.Sprintf("\thandler.Register%sRoutes(protected, r.p.DB, r.p.RouteCatalog)\n", name)

	for i := range sources {
		formatted, err := format.Source(sources[i].content)
		if err != nil {
			return nil, fmt.Errorf("格式化生成文件 %s: %w", sources[i].path, err)
		}
		sources[i].content = formatted
	}

	if err := withGeneratorLock(routerPath, func() error {
		for _, source := range sources {
			if err := ensureFileAbsent(source.path); err != nil {
				return err
			}
		}
		routerContent, err := prepareRouteRegistration(routerPath, line)
		if err != nil {
			return err
		}
		return commitGeneratedModule(sources, routerPath, routerContent)
	}); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, source.path)
	}
	return paths, nil
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

func prepareRouteRegistration(path, line string) ([]byte, error) {
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if bytes.Contains(body, []byte(line)) {
		return body, nil
	}
	if !bytes.Contains(body, []byte(routeMarker)) {
		return nil, fmt.Errorf("未在 %s 中找到 grove 路由标记", path)
	}
	return bytes.Replace(body, []byte(routeMarker), []byte(line+routeMarker), 1), nil
}

func commitGeneratedModule(sources []generatedSource, routerPath string, routerContent []byte) error {
	created := make([]string, 0, len(sources))
	rollback := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}

	for _, source := range sources {
		if err := writeNewFileAtomic(source.path, source.content, 0o600); err != nil {
			rollback()
			return err
		}
		created = append(created, source.path)
	}
	if err := replaceFileAtomic(routerPath, routerContent); err != nil {
		rollback()
		return err
	}
	return nil
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
