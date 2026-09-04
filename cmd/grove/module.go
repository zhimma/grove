package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// modulePath reads the module directive from the go.mod of the repository the
// generator is writing into.
//
// Generated code imports this path, and it is deliberately read from the
// working directory rather than from Grove's own build info: this repository is
// meant to be forked, and a fork renames its module. Baking Grove's path into
// the templates would make every generated file fail to compile there.
func modulePath() (string, error) {
	file, err := os.Open("go.mod")
	if err != nil {
		return "", fmt.Errorf("读取 go.mod（请在仓库根目录执行）: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path, ok := parseModuleDirective(scanner.Text())
		if ok {
			return path, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 go.mod: %w", err)
	}
	return "", fmt.Errorf("go.mod 中没有 module 声明")
}

func parseModuleDirective(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if index := strings.Index(line, "//"); index >= 0 {
		line = strings.TrimSpace(line[:index])
	}
	rest, ok := strings.CutPrefix(line, "module")
	if !ok {
		return "", false
	}
	// Require whitespace after the directive so "modulepath" is not a match.
	if rest == "" || !isSpace(rest[0]) {
		return "", false
	}
	path := strings.TrimSpace(rest)
	if unquoted, err := strconv.Unquote(path); err == nil {
		path = unquoted
	}
	if path == "" {
		return "", false
	}
	return path, true
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}
