package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
)

const routeMarker = "\t// grove:register-routes\n"

type generatedSource struct {
	path    string
	content []byte
}

func generateConsoleModule(input string) ([]string, error) {
	name := toPascal(input)
	snake := toSnake(input)
	if !isValidGoIdentifier(name) || snake == "" {
		return nil, fmt.Errorf("模块名称 %q 不能转换为合法 Go 标识符", input)
	}

	sources := []generatedSource{
		{path: filepath.Join("internal/model", snake+".go"), content: []byte(modelTemplate(name, snake))},
		{path: filepath.Join("app/console/internal/service", snake+".go"), content: []byte(consoleServiceTemplate(name, snake))},
		{path: filepath.Join("app/console/internal/handler", snake+".go"), content: []byte(consoleHandlerTemplate(name, snake))},
	}
	for _, source := range sources {
		if err := ensureFileAbsent(source.path); err != nil {
			return nil, err
		}
	}

	routerPath := filepath.Join("app/console/internal/router", "router.go")
	line := fmt.Sprintf("\thandler.Register%sRoutes(protected, r.p)\n", name)
	routerContent, err := prepareRouteRegistration(routerPath, line)
	if err != nil {
		return nil, err
	}

	for i := range sources {
		formatted, err := format.Source(sources[i].content)
		if err != nil {
			return nil, fmt.Errorf("格式化生成文件 %s: %w", sources[i].path, err)
		}
		sources[i].content = formatted
	}

	if err := commitGeneratedModule(sources, routerPath, routerContent); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		paths = append(paths, source.path)
	}
	return paths, nil
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
	defer os.Remove(tempPath)
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
	defer os.Remove(tempPath)
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
