package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/secretbox"
)

func TestAboutCommandPrintsFrameworkSummary(t *testing.T) {
	cmd := newAboutCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("about command failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "Grove 基础框架")
	assertContains(t, content, "api / console / worker")
	assertContains(t, content, "make verify")
}

func TestRootCommandIncludesGroveHelp(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "grove 是当前仓库唯一保留的 CLI 入口")
	assertContains(t, content, "about")
	assertContains(t, content, "doctor")
	assertContains(t, content, "rbac")
	assertContains(t, content, "make:module")
}

func TestRepositoryCommandsUseGroveCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	makefile := mustRead(t, filepath.Join(root, "Makefile"))
	readme := mustRead(t, filepath.Join(root, "README.md"))

	assertContains(t, makefile, "GROVE := $(GO) run ./cmd/grove")
	assertNotContains(t, makefile, "cmd/artisan")
	assertContains(t, readme, "go run ./cmd/grove")
	assertNotContains(t, readme, "cmd/artisan")
}

func TestMakefileUsesCurrentConsoleScripts(t *testing.T) {
	makefile := mustRead(t, filepath.Join("..", "..", "Makefile"))

	assertContains(t, makefile, "ADMIN_DIR := web/admin-vben")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) install --frozen-lockfile")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) dev:console")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) build:console")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) --filter @grove/console typecheck")
	assertNotContains(t, makefile, "install:admin-vben")
	assertNotContains(t, makefile, "dev:admin")
	assertNotContains(t, makefile, "build:admin")
	assertNotContains(t, makefile, "typecheck:admin")
}

func TestSeedCommandListsExplicitSafetyModes(t *testing.T) {
	cmd := newSeedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("seed help failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "bootstrap")
	assertContains(t, content, "demo")
}

func TestMigrateCommandSupportsCustomSourcePath(t *testing.T) {
	cmd := newMigrateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate help failed: %v", err)
	}
	assertContains(t, out.String(), "--path")
}

// migrate create used to write only the configured dialect, and the previous
// version of this test asserted exactly that — while TestDialectMigrationVersionsMatch
// failed on every such pair. Both trees must gain the same version at once.
func TestMigrateCreateWritesBothDialectsAtOneVersion(t *testing.T) {
	base := filepath.Join(t.TempDir(), "migrations")

	cmd := newMigrateCmd()
	cmd.SetArgs([]string{"--path", base, "create", "create_articles"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate create: %v", err)
	}

	versions := map[string]struct{}{}
	for _, dialect := range []string{"postgres", "mysql"} {
		files, err := filepath.Glob(filepath.Join(base, dialect, "*.sql"))
		if err != nil || len(files) != 2 {
			t.Fatalf("%s migrations = %v, err = %v; want an up and a down", dialect, files, err)
		}
		for _, file := range files {
			versions[strings.SplitN(filepath.Base(file), "_", 2)[0]] = struct{}{}
		}
	}
	if len(versions) != 1 {
		t.Fatalf("dialects got different versions: %v", versions)
	}
}

func TestKeyGenerateProducesKeysTheConfigAccepts(t *testing.T) {
	generate := func() string {
		cmd := newKeyGenerateCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("key:generate: %v", err)
		}
		return out.String()
	}
	first := generate()

	var keys struct {
		JWT struct {
			Secret string `yaml:"secret"`
		} `yaml:"jwt"`
		Security struct {
			ConfigEncryptionKey string `yaml:"config_encryption_key"`
		} `yaml:"security"`
	}
	if err := yaml.Unmarshal([]byte(first), &keys); err != nil {
		t.Fatalf("output is not YAML: %v\n%s", err, first)
	}
	if len(keys.JWT.Secret) < 32 {
		t.Fatalf("jwt.secret has %d characters, production requires 32", len(keys.JWT.Secret))
	}
	if _, err := secretbox.New(keys.Security.ConfigEncryptionKey); err != nil {
		t.Fatalf("config_encryption_key rejected by secretbox: %v", err)
	}
	if generate() == first {
		t.Fatal("two runs printed the same keys")
	}
}

func TestDemoSeedRefusesProduction(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	mustWrite(t, configPath, `app:
  env: production
jwt:
  secret: 0123456789abcdef0123456789abcdef
`)
	t.Setenv("APP_ENV", "")
	t.Setenv("JWT_SECRET", "")

	previousConfigFile := configFile
	configFile = configPath
	t.Cleanup(func() { configFile = previousConfigFile })

	cmd := newSeedCmd()
	cmd.SetArgs([]string{"demo"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected production demo seed to be rejected")
	}
	assertContains(t, err.Error(), "production")
}

func TestBootstrapSeedDoesNotOverwriteRootPassword(t *testing.T) {
	root := filepath.Join("..", "..")
	seed := mustRead(t, filepath.Join(root, "database", "seeds", "postgres", "bootstrap", "202604150002_console_root_super_admin.sql"))

	assertContains(t, seed, "{{GROVE_ROOT_PASSWORD_HASH}}")
	assertContains(t, seed, "ON CONFLICT DO NOTHING")
	assertNotContains(t, seed, "password = EXCLUDED.password")
}

func TestSeedFilesAreSplitBySafetyBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "database", "seeds")
	for _, dialect := range []string{"postgres", "mysql"} {
		for _, path := range []string{
			filepath.Join(root, dialect, "bootstrap", "202604150001_system_configs.sql"),
			filepath.Join(root, dialect, "bootstrap", "202604150002_console_root_super_admin.sql"),
			filepath.Join(root, dialect, "demo", "202604150001_api_user.sql"),
			filepath.Join(root, dialect, "demo", "202604150002_console_admin.sql"),
		} {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected seed file %s: %v", path, err)
			}
		}
	}
}

func TestResolveRootPasswordUsesEnvironment(t *testing.T) {
	t.Setenv(rootPasswordEnv, "configured-root-password")

	password, generated, err := resolveRootPassword(nil)
	if err != nil {
		t.Fatalf("resolve configured root password: %v", err)
	}
	if generated {
		t.Fatal("configured root password must not be marked as generated")
	}
	if password != "configured-root-password" {
		t.Fatalf("unexpected configured password: %q", password)
	}
}

func TestResolveRootPasswordPrefersConfig(t *testing.T) {
	t.Setenv(rootPasswordEnv, "environment-root-password")
	cfg := &config.Config{}
	cfg.Security.InitialRootPassword = "configured-root-password"

	password, generated, err := resolveRootPassword(cfg)
	if err != nil {
		t.Fatalf("resolve config root password: %v", err)
	}
	if generated {
		t.Fatal("config root password must not be marked as generated")
	}
	if password != "configured-root-password" {
		t.Fatalf("unexpected config password: %q", password)
	}
}

func TestResolveRootPasswordGeneratesRandomValue(t *testing.T) {
	t.Setenv(rootPasswordEnv, "")

	password, generated, err := resolveRootPassword(nil)
	if err != nil {
		t.Fatalf("generate root password: %v", err)
	}
	if !generated {
		t.Fatal("missing root password must generate a one-time value")
	}
	if len(password) < 24 {
		t.Fatalf("generated password is unexpectedly short: %d", len(password))
	}
}

func TestDoctorCommandPrintsConfigSummary(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "config.yaml"), `app:
  name: demo-app
  env: test
port: "18080"
console_port: "18081"
databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: "5432"
    user: grove
    dbname: grove
redis:
  enabled: true
job:
  enabled: false
casbin:
  enforcers:
    api:
      enabled: true
    console:
      enabled: false
storage:
  default: local
docs:
  enabled: true
`)

	cmd := newDoctorCmd()
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "配置文件路径")
	cmd.SetArgs([]string{"-c", filepath.Join(root, "config.yaml")})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "应用名称: demo-app")
	assertContains(t, content, "默认数据库: 已启用")
	assertContains(t, content, "Redis: 已启用")
	assertContains(t, content, "任务队列: 未启用")
}

const moduleFieldsForTest = "title:string:required,amount:int,paid:bool,note:text,due_at:time"

// This is the contract the generator exists to keep: what it writes into a real
// copy of this repository must build, pass the route/OpenAPI contract, pass the
// dialect migration rules, and pass the CRUD test it generates alongside.
// It used to write a stub whose route failed make contracts on the spot.
func TestMakeModuleOutputPassesTheProjectGates(t *testing.T) {
	if testing.Short() {
		t.Skip("copies the repository and runs go test; skipped in -short")
	}
	root := prepareRepositoryCopy(t)
	chdir(t, root)

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"Invoice", "--label", "发票", "--fields", moduleFieldsForTest})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module failed: %v", err)
	}

	runGo(t, root, "vet", "./internal/model/...", "./app/console/...")
	// Contract includes TestConsoleFrontendContractMatchesOpenAPI, which holds
	// the entries appended to console-contract.json against the generated docs.
	runGo(t, root, "test",
		"./app/console/internal/docs/", "./app/console/internal/service/", "./pkg/migrate/",
		"-run", "Contract|InvoiceServiceCRUD|DialectMigrationVersionsMatch|EveryUpMigrationHasDown|MySQLMigrations",
	)

	api := mustRead(t, filepath.Join(root, consoleWebDir, "api/invoice.ts"))
	registry := mustRead(t, filepath.Join(root, consoleContractJSON))
	calls := regexp.MustCompile(`consoleEndpoint\(\s*'([^']+)'`).FindAllStringSubmatch(api, -1)
	if len(calls) != 5 {
		t.Fatalf("frontend api calls %d operations, want 5:\n%s", len(calls), api)
	}
	for _, call := range calls {
		assertContains(t, registry, `"operationId": "`+call[1]+`"`)
	}
	view := mustRead(t, filepath.Join(root, consoleWebDir, "views/invoices/index.vue"))
	assertContains(t, view, "from '#/api/invoice'")
	assertContains(t, view, "{ key: 'due_at', label: 'due_at', type: 'datetime' }")
	assertNotContains(t, view, "dataIndex: 'note'")
	route := mustRead(t, filepath.Join(root, consoleWebDir, "router/routes/modules/invoices.ts"))
	assertContains(t, route, "import('#/views/invoices/index.vue')")
}

// Single-field modules exercise the template branches the full field set hides:
// no string to trim or search, no time to import, nothing sortable but the
// timestamps. One repository copy holds all of them to keep the test cheap.
func TestMakeModuleCompilesForEachFieldShape(t *testing.T) {
	if testing.Short() {
		t.Skip("copies the repository and runs go test; skipped in -short")
	}
	root := prepareRepositoryCopy(t)
	chdir(t, root)

	shapes := map[string]string{
		"Counter":    "hits:int",
		"Flag":       "active:bool",
		"Moment":     "happened_at:time:required",
		"Memo":       "body:text:required",
		"HTTPClient": "user_id:string,api_url:string,api_at:time",
	}
	crudTests := make([]string, 0, len(shapes))
	for name, fields := range shapes {
		cmd := newMakeModuleCmd()
		cmd.SetArgs([]string{name, "--fields", fields})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("make:module %s --fields %s: %v", name, fields, err)
		}
		crudTests = append(crudTests, name+"ServiceCRUD")
	}

	runGo(t, root, "vet", "./internal/model/...", "./app/console/...")
	runGo(t, root, "test", "./app/console/internal/docs/", "./app/console/internal/service/",
		"-run", "Contract|"+strings.Join(crudTests, "|"))
}

// The generator re-encodes console-contract.json to append to it, so encoding
// the untouched registry has to reproduce it exactly or every generation would
// reformat entries it did not add.
func TestContractRegistryRoundTripsByteForByte(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", consoleContractJSON))
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	same, err := appendContractOperations(body, nil)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if !bytes.Equal(same, body) {
		t.Fatal("encoding the registry changed its layout")
	}

	spec, err := newModuleSpec("example.com/app", "Invoice", "", nil)
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	once, err := appendContractOperations(body, spec.Operations())
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	twice, err := appendContractOperations(once, spec.Operations())
	if err != nil {
		t.Fatalf("append again: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Fatal("appending the same operations twice duplicated them")
	}
	assertContains(t, string(once), `"path": "/invoices/{id}"`)

	if _, err := appendContractOperations([]byte(`{"basePath":"/x","operations":[],"extra":1}`), nil); err == nil {
		t.Fatal("unknown registry fields must fail instead of being dropped")
	}
}

func TestMakeModuleWiresRoutesOperationsAndMigrations(t *testing.T) {
	root := prepareModuleWorkspace(t, stubRouterWithMarker)
	chdir(t, root)

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory", "--label", "商品分类", "--fields", "name:string:required,sort:int"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module failed: %v", err)
	}

	router := mustRead(t, filepath.Join(root, routerFile))
	assertContains(t, router, "\thandler.RegisterProductCategoryRoutes(protected, r.p.DB, pages, catalog)\n")
	contract := mustRead(t, filepath.Join(root, contractFile))
	assertContains(t, contract, "\taddProductCategoryOperations(&doc)\n")

	handler := mustRead(t, filepath.Join(root, "app/console/internal/handler/product_category.go"))
	assertContains(t, handler, `wrapRoute(protected.Group("/product-categories"), catalog)`)
	// Permission names group by the part before the dot, so the label is what
	// operators see in the role editor.
	for _, name := range []string{"商品分类.列表", "商品分类.详情", "商品分类.创建", "商品分类.更新", "商品分类.删除"} {
		assertContains(t, handler, `.Name("`+name+`")`)
	}
	assertContains(t, handler, `binding:"required,max=255"`)

	model := mustRead(t, filepath.Join(root, "internal/model/product_category.go"))
	assertContains(t, model, `return "product_categories"`)
	assertNotContains(t, model, "default:")

	versions := map[string]struct{}{}
	for _, dialect := range []string{"postgres", "mysql"} {
		ups, _ := filepath.Glob(filepath.Join(root, migrationsDir, dialect, "*_create_product_categories.up.sql"))
		downs, _ := filepath.Glob(filepath.Join(root, migrationsDir, dialect, "*_create_product_categories.down.sql"))
		if len(ups) != 1 || len(downs) != 1 {
			t.Fatalf("%s migrations: up=%v down=%v", dialect, ups, downs)
		}
		versions[strings.SplitN(filepath.Base(ups[0]), "_", 2)[0]] = struct{}{}
	}
	if len(versions) != 1 {
		t.Fatalf("dialects got different migration versions: %v", versions)
	}
}

func TestMakeModuleDefaultsToANameField(t *testing.T) {
	root := prepareModuleWorkspace(t, stubRouterWithMarker)
	chdir(t, root)

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"Tag"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module without --fields failed: %v", err)
	}
	model := mustRead(t, filepath.Join(root, "internal/model/tag.go"))
	assertContains(t, model, `Name string`)
}

func TestMakeModulePreflightsBeforeWriting(t *testing.T) {
	root := prepareModuleWorkspace(t, `package router

func register() {
	// marker missing on purpose
}
`)
	chdir(t, root)

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing router marker error")
	}
	assertNothingGenerated(t, root, "product_category", "product_categories")
}

func TestMakeModuleRejectsInvalidInputBeforeWriting(t *testing.T) {
	cases := map[string][]string{
		"invalid name":        {"123-product"},
		"unknown field type":  {"Product", "--fields", "price:money"},
		"reserved field name": {"Product", "--fields", "id:string"},
		"sql keyword field":   {"Product", "--fields", "order:int"},
		"required bool":       {"Product", "--fields", "active:bool:required"},
		"label with a dot":    {"Product", "--label", "商品.管理"},
		"label with a quote":  {"Product", "--label", "商品'管理"},
		"test suffix":         {"InvoiceTest"},
		"os suffix":           {"InvoiceLinux"},
		"arch suffix":         {"InvoiceAMD64"},
		"field collision":     {"Product", "--fields", "api_url:string,a_p_i_u_r_l:string"},
		"base collision":      {"Product", "--fields", "base:string"},
		"method collision":    {"Product", "--fields", "table_name:string"},
		"input collision":     {"Product", "--fields", "product_id:string"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			root := prepareModuleWorkspace(t, stubRouterWithMarker)
			chdir(t, root)

			cmd := newMakeModuleCmd()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("expected %s to be rejected", name)
			}
			assertNothingGenerated(t, root, "product", "products")
		})
	}
}

func TestMakeModuleChecksAllTargetsBeforeWriting(t *testing.T) {
	root := prepareModuleWorkspace(t, stubRouterWithMarker)
	mustWrite(t, filepath.Join(root, "app/console/internal/service/product_category.go"), "package service\n")
	chdir(t, root)

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected existing service error")
	}
	for _, path := range []string{
		"internal/model/product_category.go",
		"app/console/internal/handler/product_category.go",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("target preflight left generated file %s", path)
		}
	}
	if migrations, _ := filepath.Glob(filepath.Join(root, migrationsDir, "*", "*_create_product_categories.*")); len(migrations) != 0 {
		t.Fatalf("target preflight left migrations: %v", migrations)
	}
}

func TestMakeModuleConcurrentGenerationsPreserveBothRegistrations(t *testing.T) {
	root := prepareModuleWorkspace(t, stubRouterWithMarker)
	chdir(t, root)

	inputs := []string{"ProductCategory", "OrderItem"}
	errs := make(chan error, len(inputs))
	var wg sync.WaitGroup
	for _, input := range inputs {
		wg.Add(1)
		go func(input string) {
			defer wg.Done()
			_, err := generateConsoleModule(input, "", "")
			errs <- err
		}(input)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent module generation failed: %v", err)
		}
	}

	router := mustRead(t, filepath.Join(root, routerFile))
	contract := mustRead(t, filepath.Join(root, contractFile))
	for _, name := range []string{"ProductCategory", "OrderItem"} {
		assertContains(t, router, "handler.Register"+name+"Routes(protected, r.p.DB, pages, catalog)")
		assertContains(t, contract, "add"+name+"Operations(&doc)")
	}
}

// A failure after some files are written must leave the repository as it was:
// new files removed, edited files restored.
func TestCommitGeneratedModuleRollsBackEverythingOnFailure(t *testing.T) {
	root := t.TempDir()
	chdir(t, root)

	edited := filepath.Join(root, "router.go")
	mustWrite(t, edited, "package router\n")
	unwritable := filepath.Join(root, "contract")
	mustMkdir(t, unwritable)

	sources := []generatedSource{
		{path: filepath.Join(root, "model.go"), content: []byte("package model\n")},
		{path: filepath.Join(root, "service.go"), content: []byte("package service\n")},
	}
	edits := []fileEdit{
		{path: edited, original: []byte("package router\n"), updated: []byte("package router // changed\n")},
		// Replacing a directory with a file fails, after the first edit landed.
		{path: unwritable, original: nil, updated: []byte("package docs\n")},
	}

	if _, err := commitGeneratedModule(sources, nil, "widgets", edits); err == nil {
		t.Fatal("expected the second edit to fail")
	}
	for _, source := range sources {
		if _, err := os.Stat(source.path); !os.IsNotExist(err) {
			t.Fatalf("failure left generated file %s", source.path)
		}
	}
	if migrations, _ := filepath.Glob(filepath.Join(root, migrationsDir, "*", "*_create_widgets.*")); len(migrations) != 0 {
		t.Fatalf("failure left migrations: %v", migrations)
	}
	if got := mustRead(t, edited); got != "package router\n" {
		t.Fatalf("failure did not restore the edited file: %q", got)
	}
}

const stubRouterWithMarker = `package router

func register() {
	// grove:register-routes
}
`

// prepareRepositoryCopy copies the Go tree with its tests, plus the frontend
// contract the OpenAPI contract test reads, so generated code is checked
// against the real router, docs and migration rules rather than stubs.
func prepareRepositoryCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join("..", "..")
	for _, dir := range []string{"app", "internal", "pkg", "database"} {
		copyTree(t, filepath.Join(repoRoot, dir), filepath.Join(root, dir), true)
	}
	for _, file := range []string{"go.mod", "go.sum", "web/admin-vben/apps/console/src/api/console-contract.json"} {
		if err := copyFile(t, filepath.Join(repoRoot, file), filepath.Join(root, file)); err != nil {
			t.Fatalf("copy %s: %v", file, err)
		}
	}
	return root
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("go", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func assertNothingGenerated(t *testing.T, root, snake, table string) {
	t.Helper()
	for _, path := range []string{
		"internal/model/" + snake + ".go",
		"app/console/internal/service/" + snake + ".go",
		"app/console/internal/handler/" + snake + ".go",
		"app/console/internal/docs/" + snake + ".go",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("rejected generation left %s", path)
		}
	}
	if migrations, _ := filepath.Glob(filepath.Join(root, migrationsDir, "*", "*_create_"+table+".*")); len(migrations) != 0 {
		t.Fatalf("rejected generation left migrations: %v", migrations)
	}
}

func prepareModuleWorkspace(t *testing.T, router string) string {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "pkg", "app/console"} {
		copyTree(t, filepath.Join(repoRoot, dir), filepath.Join(root, dir), false)
	}
	mustWrite(t, filepath.Join(root, routerFile), router)
	for _, file := range []string{"go.mod", "go.sum"} {
		if err := copyFile(t, filepath.Join(repoRoot, file), filepath.Join(root, file)); err != nil {
			t.Fatalf("copy %s: %v", file, err)
		}
	}
	return root
}

// copyTree copies a directory. Files are read and written whole rather than
// streamed through handles held until cleanup, which ran into the open-file
// limit once whole-repository copies came along.
func copyTree(t *testing.T, source, target string, includeTests bool) {
	t.Helper()
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !includeTests && strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o750)
		}
		return copyFile(t, path, destination)
	})
	if err != nil {
		t.Fatalf("copy tree %s: %v", source, err)
	}
}

func copyFile(t *testing.T, source, target string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Clean(source))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(target), content, 0o600)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected content to contain %q\ncontent:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected content not to contain %q\ncontent:\n%s", needle, haystack)
	}
}

// This repository is meant to be forked, and a fork renames its module. The
// generator must therefore take the import path from the target repository's
// go.mod — baking Grove's own path into the templates would emit code that
// does not compile anywhere but here.
func TestGeneratedCodeImportsTheTargetModulePath(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "app/console/internal/router"))
	mustMkdir(t, filepath.Join(root, "app/console/internal/service"))
	mustMkdir(t, filepath.Join(root, "app/console/internal/handler"))
	mustMkdir(t, filepath.Join(root, "internal/model"))
	mustWrite(t, filepath.Join(root, "app/console/internal/router/router.go"), `package router

func register() {
	// grove:register-routes
}
`)
	mustMkdir(t, filepath.Join(root, "app/console/internal/docs"))
	mustWrite(t, filepath.Join(root, contractFile), "package docs\n\nfunc spec() {\n\t// grove:register-operations\n}\n")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/renamed-fork\n\ngo 1.25.0\n")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"Invoice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module failed: %v", err)
	}

	for _, generated := range []string{
		"app/console/internal/service/invoice.go",
		"app/console/internal/handler/invoice.go",
	} {
		content := mustRead(t, filepath.Join(root, generated))
		assertContains(t, content, `"example.com/renamed-fork/pkg/database"`)
		if strings.Contains(content, "github.com/zhimma/grove") {
			t.Fatalf("%s still imports Grove's own module path:\n%s", generated, content)
		}
	}
}

func TestModulePathReportsAMissingGoMod(t *testing.T) {
	root := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	if _, err := modulePath(); err == nil {
		t.Fatal("modulePath must fail outside a module rather than emit uncompilable imports")
	}
}
