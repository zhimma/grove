package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var moduleNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*([_ -][A-Za-z0-9]+)*$`)

// Initialisms keep Go identifiers consistent without changing SQL/JSON spelling.
var initialisms = map[string]bool{
	"api": true, "ascii": true, "cpu": true, "css": true, "dns": true,
	"eof": true, "guid": true, "html": true, "http": true, "https": true,
	"id": true, "ip": true, "json": true, "qps": true, "ram": true,
	"rpc": true, "sla": true, "smtp": true, "sql": true, "ssh": true,
	"tcp": true, "tls": true, "ttl": true, "udp": true, "ui": true,
	"uid": true, "uuid": true, "uri": true, "url": true, "utf8": true,
	"vm": true, "xml": true,
}

// Go interprets these final filename segments as build constraints. Reject
// them instead of emitting a module that only builds on one platform or in tests.
func validateModuleFilename(snake string) error {
	_, suffix, found := strings.Cut(snake, "_")
	if !found {
		return nil
	}
	parts := strings.Split(suffix, "_")
	suffix = parts[len(parts)-1]
	reserved := " test aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos 386 amd64 amd64p32 arm arm64 arm64be armbe loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm "
	if strings.Contains(reserved, " "+suffix+" ") {
		return fmt.Errorf("模块名生成的文件 %s.go 包含 Go 保留后缀 %q，请使用其他名称", snake, suffix)
	}
	return nil
}

func toSnake(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	var out []rune
	runes := []rune(input)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			boundary := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
				(unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])))
			if boundary && len(out) > 0 && out[len(out)-1] != '_' {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			continue
		}
		if r == '-' || r == ' ' {
			if len(out) > 0 && out[len(out)-1] != '_' {
				out = append(out, '_')
			}
			continue
		}
		out = append(out, unicode.ToLower(r))
	}
	return strings.Trim(string(out), "_")
}

func toKebabPlural(input string) string {
	base := strings.ReplaceAll(toSnake(input), "_", "-")
	return pluralize(base)
}

func toSnakePlural(input string) string {
	return pluralize(toSnake(input))
}

func pluralize(base string) string {
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "y") && len(base) > 1 {
		prev := base[len(base)-2]
		if !strings.ContainsRune("aeiou", rune(prev)) {
			return strings.TrimSuffix(base, "y") + "ies"
		}
	}
	if strings.HasSuffix(base, "s") {
		return base
	}
	return base + "s"
}

func toPascal(input string) string {
	parts := strings.Split(toSnake(input), "_")
	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if initialisms[part] {
			out.WriteString(strings.ToUpper(part))
			continue
		}
		runes := []rune(strings.ToLower(part))
		runes[0] = unicode.ToUpper(runes[0])
		out.WriteString(string(runes))
	}
	return out.String()
}

func isValidGoIdentifier(value string) bool {
	for index, r := range value {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}
