package main

import (
	"strings"
	"unicode"
)

func toSnake(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	var out []rune
	for i, r := range input {
		if unicode.IsUpper(r) {
			if i > 0 && out[len(out)-1] != '_' {
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
