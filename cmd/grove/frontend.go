package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	adminWorkspaceDir = "web/admin-vben"
	consoleWebDir     = adminWorkspaceDir + "/apps/console/src"
	// The frontend is generated only when this file exists, so a fork that
	// dropped the admin app still gets the backend slice.
	consoleContractJSON = consoleWebDir + "/api/console-contract.json"
)

type consoleContract struct {
	BasePath   string              `json:"basePath"`
	Operations []contractOperation `json:"operations"`
}

type contractOperation struct {
	OperationID string `json:"operationId"`
	Method      string `json:"method"`
	Path        string `json:"path"`
}

// Operations are the five routes the handler template registers, under the
// operation IDs the docs template gives them. The generation test checks the
// three against each other through make contracts.
func (m moduleSpec) Operations() []contractOperation {
	item := m.RoutePath + "/{id}"
	return []contractOperation{
		{OperationID: "consoleList" + m.Plural, Method: http.MethodGet, Path: m.RoutePath},
		{OperationID: "consoleCreate" + m.Name, Method: http.MethodPost, Path: m.RoutePath},
		{OperationID: "consoleGet" + m.Name, Method: http.MethodGet, Path: item},
		{OperationID: "consoleUpdate" + m.Name, Method: http.MethodPut, Path: item},
		{OperationID: "consoleDelete" + m.Name, Method: http.MethodDelete, Path: item},
	}
}

func hasConsoleFrontend() bool {
	_, err := os.Stat(consoleContractJSON)
	return err == nil
}

func renderFrontendSources(spec moduleSpec) ([]generatedSource, error) {
	views := strings.TrimPrefix(spec.RoutePath, "/")
	files := []struct{ path, name, text string }{
		{filepath.Join(consoleWebDir, "api", spec.Kebab+".ts"), "frontend api", frontendAPITemplate},
		{filepath.Join(consoleWebDir, "views", views, "index.vue"), "frontend view", frontendViewTemplate},
		{filepath.Join(consoleWebDir, "router/routes/modules", views+".ts"), "frontend route", frontendRouteTemplate},
	}
	sources := make([]generatedSource, 0, len(files))
	for _, file := range files {
		content, err := render(file.name, file.text, spec)
		if err != nil {
			return nil, err
		}
		sources = append(sources, generatedSource{path: file.path, content: content})
	}
	return sources, nil
}

func prepareContractEdit(spec moduleSpec) (fileEdit, error) {
	body, err := os.ReadFile(consoleContractJSON)
	if err != nil {
		return fileEdit{}, err
	}
	updated, err := appendContractOperations(body, spec.Operations())
	if err != nil {
		return fileEdit{}, fmt.Errorf("%s: %w", consoleContractJSON, err)
	}
	return fileEdit{path: consoleContractJSON, original: body, updated: updated}, nil
}

// appendContractOperations re-encodes the registry in the layout prettier
// keeps it in, so the only diff is the appended entries. Unknown fields are an
// error rather than something a round trip silently drops.
func appendContractOperations(body []byte, operations []contractOperation) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var contract consoleContract
	if err := decoder.Decode(&contract); err != nil {
		return nil, err
	}
	registered := make(map[string]bool, len(contract.Operations))
	for _, operation := range contract.Operations {
		registered[operation.OperationID] = true
	}
	for _, operation := range operations {
		if !registered[operation.OperationID] {
			contract.Operations = append(contract.Operations, operation)
		}
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(contract); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// formatFrontend runs the admin workspace's own prettier over the generated
// frontend files: admin.lint rejects unformatted code and the templates cannot
// predict prettier's line wrapping. It reports false when prettier is not
// installed yet.
func formatFrontend(created []string) (bool, error) {
	// Prettier resolves its plugins from the working directory, so it runs
	// inside the workspace on workspace-relative paths.
	var files []string
	for _, path := range created {
		if relative, ok := strings.CutPrefix(path, adminWorkspaceDir+"/"); ok {
			files = append(files, relative)
		}
	}
	if len(files) == 0 {
		return true, nil
	}
	if _, err := os.Stat(filepath.Join(adminWorkspaceDir, "node_modules", ".bin", "prettier")); err != nil {
		return false, nil
	}
	registry := strings.TrimPrefix(consoleContractJSON, adminWorkspaceDir+"/")
	cmd := exec.Command(filepath.Join("node_modules", ".bin", "prettier"),
		append([]string{"--write", "--log-level", "warn", registry}, files...)...)
	cmd.Dir = adminWorkspaceDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("格式化前端文件: %w\n%s", err, output)
	}
	return true, nil
}

const frontendAPITemplate = `import type { PageData, PageParams } from '#/types/pagination';

import { consoleEndpoint } from '#/api/console-contract';
import { requestClient } from '#/api/request';

export interface Console{{.Name}} {
  id: string;
{{- range .Fields}}
  {{.Name}}{{if .IsTime}}?{{end}}: {{.TSType}};
{{- end}}
  created_at: string;
  updated_at: string;
}

export function get{{.Name}}List(params: PageParams & Record<string, any>) {
  return requestClient.get<PageData<Console{{.Name}}>>(
    consoleEndpoint('consoleList{{.Plural}}'),
    { params },
  );
}

export function get{{.Name}}(id: string) {
  return requestClient.get<Console{{.Name}}>(
    consoleEndpoint('consoleGet{{.Name}}', { id }),
  );
}

export function create{{.Name}}(data: Record<string, any>) {
  return requestClient.post<Console{{.Name}}>(
    consoleEndpoint('consoleCreate{{.Name}}'),
    data,
  );
}

export function update{{.Name}}(id: string, data: Record<string, any>) {
  return requestClient.put<Console{{.Name}}>(
    consoleEndpoint('consoleUpdate{{.Name}}', { id }),
    data,
  );
}

export function delete{{.Name}}(id: string) {
  return requestClient.delete(consoleEndpoint('consoleDelete{{.Name}}', { id }));
}
`

const frontendViewTemplate = `<script setup lang="ts">
import {
  create{{.Name}},
  delete{{.Name}},
  get{{.Name}},
  get{{.Name}}List,
  update{{.Name}},
} from '#/api/{{.Kebab}}';
import ResourcePage from '#/components/resource-page/index.vue';

const columns = [
{{- range .ColumnFields}}
  { title: '{{.Name}}', dataIndex: '{{.Name}}', key: '{{.Name}}' },
{{- end}}
  { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180 },
];
</script>

<template>
  <ResourcePage
    title="{{.Label}}"
    :columns="columns"
{{- if .SearchFields}}
    :search-fields="[{ key: 'keyword', label: '关键词' }]"
{{- end}}
    :form-fields="[
{{- range .Fields}}
      { key: '{{.Name}}', label: '{{.Name}}'{{with .FormType}}, type: '{{.}}'{{end}}{{if .Required}}, required: true{{end}} },
{{- end}}
    ]"
    :fetch-api="get{{.Name}}List"
    :get-detail-api="get{{.Name}}"
    :create-api="create{{.Name}}"
    :update-api="update{{.Name}}"
    :delete-api="delete{{.Name}}"
  />
</template>
`

const frontendRouteTemplate = `import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    meta: { icon: 'lucide:folder', order: 1000, title: '{{.Label}}' },
    name: 'Console{{.Plural}}',
    path: '{{.RoutePath}}',
    redirect: '{{.RoutePath}}/list',
    children: [
      {
        name: 'Console{{.Plural}}List',
        path: '{{.RoutePath}}/list',
        component: () => import('#/views{{.RoutePath}}/index.vue'),
        meta: { title: '{{.Label}}列表' },
      },
    ],
  },
];

export default routes;
`
