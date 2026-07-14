package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/zhimma/grove/pkg/errx"
)

const (
	maxConsoleMenuKeys      = 256
	maxConsoleMenuKeyLength = 128
)

var consoleMenuKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

func normalizeConsoleMenuKeys(keys []string) []string {
	if len(keys) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if key == "*" {
			return []string{"*"}
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func validateConsoleMenuKeys(keys []string) error {
	keys = normalizeConsoleMenuKeys(keys)
	if len(keys) == 0 || keys[0] == "*" {
		return nil
	}
	if len(keys) > maxConsoleMenuKeys {
		return errx.InvalidParams().WithHTTPStatus(422).WithMessage(
			fmt.Sprintf("菜单权限数量不能超过 %d 个", maxConsoleMenuKeys),
		)
	}

	for _, key := range keys {
		if len(key) > maxConsoleMenuKeyLength {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage(
				fmt.Sprintf("菜单权限标识长度不能超过 %d 个字符", maxConsoleMenuKeyLength),
			)
		}
		if !consoleMenuKeyPattern.MatchString(key) {
			return errx.InvalidParams().WithHTTPStatus(422).WithMessage(
				fmt.Sprintf("菜单权限标识 %q 格式不正确", key),
			)
		}
	}
	return nil
}
