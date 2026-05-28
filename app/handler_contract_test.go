package app_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHandlersUseUnifiedValidationBinding(t *testing.T) {
	for _, file := range handlerGoFiles(t) {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "ShouldBindJSON", "ShouldBindQuery", "ShouldBindUri", "ShouldBindURI":
				pos := fset.Position(selector.Pos())
				t.Errorf("%s: use validation.Bind* instead of c.%s", pos, selector.Sel.Name)
			}
			return true
		})
	}
}

func TestHandlerRequestFieldsHaveValidationLabels(t *testing.T) {
	for _, file := range handlerGoFiles(t) {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}

		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(typeSpec.Name.Name, "Request") {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				checkRequestStructFields(t, fset, file, typeSpec.Name.Name, structType)
			}
		}
	}
}

func checkRequestStructFields(t *testing.T, fset *token.FileSet, file, structName string, structType *ast.StructType) {
	t.Helper()

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 || field.Tag == nil {
			continue
		}
		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		if tag.Get("json") == "" && tag.Get("form") == "" && tag.Get("uri") == "" {
			continue
		}
		if isIgnoredRequestField(tag) {
			continue
		}

		pos := fset.Position(field.Pos())
		for _, name := range field.Names {
			if strings.TrimSpace(tag.Get("label")) == "" {
				t.Errorf("%s: %s.%s is request-bound and must define label", pos, structName, name.Name)
			}
			if tag.Get("uri") != "" && !bindingIncludes(tag.Get("binding"), "required") {
				t.Errorf("%s: %s.%s is URI-bound and must include binding:\"required\"", pos, structName, name.Name)
			}
			if tag.Get("form") == "page" && !bindingIncludes(tag.Get("binding"), "min=1") {
				t.Errorf("%s: %s.%s page must include min=1", pos, structName, name.Name)
			}
			if tag.Get("form") == "page_size" {
				binding := tag.Get("binding")
				if !bindingIncludes(binding, "min=1") || !bindingIncludes(binding, "max=100") {
					t.Errorf("%s: %s.%s page_size must include min=1,max=100", pos, structName, name.Name)
				}
			}
		}
	}
}

func handlerGoFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	for _, root := range []string{"api/handler", "console/internal/handler"} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || strings.HasSuffix(path, "_test.go") || !strings.HasSuffix(path, ".go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return files
}

func isIgnoredRequestField(tag reflect.StructTag) bool {
	for _, key := range []string{"json", "form", "uri"} {
		if firstTagPart(tag.Get(key)) == "-" {
			return true
		}
	}
	return false
}

func bindingIncludes(binding, expected string) bool {
	for _, part := range strings.Split(binding, ",") {
		if strings.TrimSpace(part) == expected {
			return true
		}
	}
	return false
}

func firstTagPart(tag string) string {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
