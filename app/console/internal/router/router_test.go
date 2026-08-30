package router

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/datatype"
	appmiddleware "github.com/zhimma/grove/internal/middleware"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/rbac"
)

func TestConsoleRouterManagementFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App:         config.AppConfig{Name: "grove", Env: "test"},
		ConsolePort: "8081",
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Security: config.SecurityConfig{
			ConfigEncryptionKey: "0123456789abcdef0123456789abcdef",
		},
		Docs: config.DocsConfig{
			Enabled: false,
		},
		Storage: config.StorageConfig{
			Default:             "local",
			DefaultUploadPolicy: "document",
			UploadPolicies: map[string]config.UploadPolicyConfig{
				"document": {
					Directory:  "documents",
					MaxBytes:   64,
					Extensions: []string{".txt"},
					MIMETypes:  []string{"text/plain"},
				},
			},
			Disks: map[string]config.StorageDiskConfig{
				"local": {
					Driver:  "local",
					Root:    t.TempDir(),
					BaseURL: "/storage",
					Prefix:  "console-test",
				},
			},
		},
	}

	db := openConsoleTestDB(t)
	enforcer := openConsoleTestEnforcer(t, db)
	seedConsoleTestData(t, db, enforcer)

	p, err := provider.New(cfg, "console", provider.WithAuth(), provider.WithConfigSecrets(), provider.WithStorage())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	p.DB = database.NewConnectionsFromDBs(db, nil)
	p.Enforcers = map[string]*rbac.Enforcer{
		"console": enforcer,
	}

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.BodyLimit(1024), appmiddleware.RequestMeta("console"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	loginResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/login", map[string]any{
		"account":  "admin",
		"password": "password",
	}, "")
	if got := int(loginResp["code"].(float64)); got != 0 {
		t.Fatalf("login failed: %#v", loginResp)
	}

	loginData, _ := loginResp["data"].(map[string]any)
	loginToken, _ := loginData["token"].(map[string]any)
	token, _ := loginToken["access_token"].(string)
	refreshToken, _ := loginToken["refresh_token"].(string)
	if token == "" {
		t.Fatalf("missing access token: %#v", loginResp)
	}
	if refreshToken == "" {
		t.Fatalf("missing refresh token: %#v", loginResp)
	}
	var persistedSession model.ConsoleSession
	if err := db.First(&persistedSession, "admin_id = ?", "console-admin-demo").Error; err != nil {
		t.Fatalf("load login session: %v", err)
	}
	if persistedSession.RefreshTokenHash == refreshToken {
		t.Fatal("refresh token plaintext must not be persisted")
	}

	refreshResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, "")
	refreshedToken := refreshResp["data"].(map[string]any)["token"].(map[string]any)
	token = refreshedToken["access_token"].(string)
	newRefreshToken := refreshedToken["refresh_token"].(string)
	performJSONWithStatus(t, engine, http.MethodPost, "/console/v1/auth/refresh", map[string]any{
		"refresh_token": refreshToken,
	}, "", http.StatusUnauthorized)
	refreshToken = newRefreshToken

	sessionsResp := performJSON(t, engine, http.MethodGet, "/console/v1/sessions?status=active", nil, token)
	if got := int(sessionsResp["code"].(float64)); got != 0 {
		t.Fatalf("list sessions failed: %#v", sessionsResp)
	}
	if len(sessionsResp["data"].(map[string]any)["list"].([]any)) == 0 {
		t.Fatalf("expected active session: %#v", sessionsResp)
	}

	rolesResp := performJSON(t, engine, http.MethodGet, "/console/v1/roles", nil, token)
	if got := int(rolesResp["code"].(float64)); got != 0 {
		t.Fatalf("list roles failed: %#v", rolesResp)
	}

	createRoleResp := performJSON(t, engine, http.MethodPost, "/console/v1/roles", map[string]any{
		"name":         "Operator",
		"code":         "operator",
		"display_name": "Operations",
		"description":  "Operations role",
		"sort":         20,
	}, token)
	if got := int(createRoleResp["code"].(float64)); got != 0 {
		t.Fatalf("create role failed: %#v", createRoleResp)
	}

	roleData := createRoleResp["data"].(map[string]any)
	roleID, _ := roleData["id"].(string)
	if roleID == "" {
		t.Fatalf("missing role id: %#v", createRoleResp)
	}

	assignPermResp := performJSON(t, engine, http.MethodPost, "/console/v1/roles/"+roleID+"/permissions", map[string]any{
		"api_permissions": []string{
			"GET /console/v1/dashboard/summary",
			"GET /console/v1/admins",
		},
	}, token)
	if got := int(assignPermResp["code"].(float64)); got != 0 {
		t.Fatalf("assign permissions failed: %#v", assignPermResp)
	}

	rolePermsResp := performJSON(t, engine, http.MethodGet, "/console/v1/roles/"+roleID+"/permissions", nil, token)
	if got := int(rolePermsResp["code"].(float64)); got != 0 {
		t.Fatalf("get role permissions failed: %#v", rolePermsResp)
	}

	assignMenusResp := performJSON(t, engine, http.MethodPost, "/console/v1/roles/"+roleID+"/menus", map[string]any{
		"menu_keys": []string{"ConsoleDashboard", "ConsoleSystem", "ConsoleRoles"},
	}, token)
	if got := int(assignMenusResp["code"].(float64)); got != 0 {
		t.Fatalf("assign menus failed: %#v", assignMenusResp)
	}

	apiPermissionOptionsResp := performJSON(t, engine, http.MethodGet, "/console/v1/permissions/apis", nil, token)
	if got := int(apiPermissionOptionsResp["code"].(float64)); got != 0 {
		t.Fatalf("get api permission options failed: %#v", apiPermissionOptionsResp)
	}

	authPermissionsResp := performJSON(t, engine, http.MethodGet, "/console/v1/auth/permissions", nil, token)
	if got := int(authPermissionsResp["code"].(float64)); got != 0 {
		t.Fatalf("get auth permissions failed: %#v", authPermissionsResp)
	}

	createAdminResp := performJSON(t, engine, http.MethodPost, "/console/v1/admins", map[string]any{
		"account":      "operator1",
		"username":     "Operator One",
		"password":     "secret123",
		"role_id":      roleID,
		"display_name": "Operator 1",
	}, token)
	if got := int(createAdminResp["code"].(float64)); got != 0 {
		t.Fatalf("create admin failed: %#v", createAdminResp)
	}

	adminData := createAdminResp["data"].(map[string]any)
	adminID, _ := adminData["id"].(string)
	if adminID == "" {
		t.Fatalf("missing admin id: %#v", createAdminResp)
	}

	operatorSession, operatorTokenPair, err := consoleservice.NewSessionService(p.DB, p.TokenManager).Create(context.Background(), consoleservice.CreateSessionInput{
		AdminID:    adminID,
		DeviceName: "router-test",
	})
	if err != nil {
		t.Fatalf("issue operator token: %v", err)
	}
	operatorToken := operatorTokenPair.AccessToken

	operatorSummaryResp := performJSON(t, engine, http.MethodGet, "/console/v1/dashboard/summary", nil, operatorToken)
	if got := int(operatorSummaryResp["code"].(float64)); got != 0 {
		t.Fatalf("operator summary failed: %#v", operatorSummaryResp)
	}
	forceLogoutResp := performJSON(t, engine, http.MethodDelete, "/console/v1/sessions/"+operatorSession.ID, nil, token)
	if got := int(forceLogoutResp["code"].(float64)); got != 0 {
		t.Fatalf("force logout failed: %#v", forceLogoutResp)
	}
	performJSONWithStatus(t, engine, http.MethodGet, "/console/v1/dashboard/summary", nil, operatorToken, http.StatusUnauthorized)

	updateStatusResp := performJSON(t, engine, http.MethodPut, "/console/v1/admins/"+adminID+"/status", map[string]any{
		"status": 0,
	}, token)
	if got := int(updateStatusResp["code"].(float64)); got != 0 {
		t.Fatalf("update admin status failed: %#v", updateStatusResp)
	}

	resetPasswordResp := performJSON(t, engine, http.MethodPut, "/console/v1/admins/"+adminID+"/reset-password", map[string]any{
		"password": "changed123",
	}, token)
	if got := int(resetPasswordResp["code"].(float64)); got != 0 {
		t.Fatalf("reset password failed: %#v", resetPasswordResp)
	}

	adminDetailResp := performJSON(t, engine, http.MethodGet, "/console/v1/admins/"+adminID, nil, token)
	if got := int(adminDetailResp["code"].(float64)); got != 0 {
		t.Fatalf("get admin detail failed: %#v", adminDetailResp)
	}
	if got := int(adminDetailResp["data"].(map[string]any)["status"].(float64)); got != 0 {
		t.Fatalf("expected disabled admin status, got %#v", adminDetailResp)
	}

	systemConfigsResp := performJSON(t, engine, http.MethodGet, "/console/v1/system-configs?list_all=true", nil, token)
	if got := int(systemConfigsResp["code"].(float64)); got != 0 {
		t.Fatalf("list system configs failed: %#v", systemConfigsResp)
	}

	createSecretResp := performJSON(t, engine, http.MethodPost, "/console/v1/system-configs?value=query-secret", map[string]any{
		"config_group":  "integration",
		"config_key":    "webhook_token",
		"name":          "Webhook Token",
		"value_type":    "string",
		"value":         "router-secret-current",
		"default_value": "router-secret-default",
		"is_editable":   true,
		"is_secret":     true,
	}, token)
	secretData := createSecretResp["data"].(map[string]any)
	secretID := secretData["id"].(string)
	if secretData["value"] != consoleservice.SecretMask || secretData["default_value"] != consoleservice.SecretMask || secretData["is_secret"] != true {
		t.Fatalf("secret config response leaked value: %#v", createSecretResp)
	}
	var persistedSecret model.SystemConfig
	if err := db.First(&persistedSecret, "id = ?", secretID).Error; err != nil {
		t.Fatalf("load persisted secret config: %v", err)
	}
	if !strings.HasPrefix(persistedSecret.Value, "v1:") || strings.Contains(persistedSecret.Value, "router-secret") {
		t.Fatalf("secret config was not encrypted: %#v", persistedSecret)
	}
	originalSecretCiphertext := persistedSecret.Value

	keepSecretResp := performJSON(t, engine, http.MethodPut, "/console/v1/system-configs/"+secretID, map[string]any{
		"value":       "must-not-replace-secret",
		"keep_secret": true,
	}, token)
	if keepSecretResp["data"].(map[string]any)["value"] != consoleservice.SecretMask {
		t.Fatalf("kept secret response must remain masked: %#v", keepSecretResp)
	}
	if err := db.First(&persistedSecret, "id = ?", secretID).Error; err != nil {
		t.Fatalf("reload persisted secret: %v", err)
	}
	if persistedSecret.Value != originalSecretCiphertext {
		t.Fatal("keep_secret changed persisted ciphertext")
	}

	var secretAudit model.ConsoleOperationLog
	if err := db.Where("target_type = ? AND target_id = ?", "system_config", secretID).
		Order("created_at DESC").First(&secretAudit).Error; err != nil {
		t.Fatalf("load secret config audit: %v", err)
	}
	for _, forbidden := range []string{"router-secret", "must-not-replace-secret", consoleservice.SecretMask, originalSecretCiphertext, `"value"`, `"default_value"`} {
		if strings.Contains(secretAudit.DetailJSON, forbidden) {
			t.Fatalf("secret audit leaked %q: %s", forbidden, secretAudit.DetailJSON)
		}
	}
	if !strings.Contains(secretAudit.DetailJSON, `"config_key":"webhook_token"`) || !strings.Contains(secretAudit.DetailJSON, `"changed":true`) {
		t.Fatalf("secret audit lacks safe change metadata: %s", secretAudit.DetailJSON)
	}
	if secretAudit.RequestQuery != "" {
		t.Fatalf("system config audit must not retain query data: %q", secretAudit.RequestQuery)
	}

	storageConfigsResp := performJSON(t, engine, http.MethodGet, "/console/v1/storage/all-configs", nil, token)
	if got := int(storageConfigsResp["code"].(float64)); got != 0 {
		t.Fatalf("list storage configs failed: %#v", storageConfigsResp)
	}

	storageConfigResp := performJSON(t, engine, http.MethodGet, "/console/v1/storage/config?disk=local", nil, token)
	if got := int(storageConfigResp["code"].(float64)); got != 0 {
		t.Fatalf("get storage config failed: %#v", storageConfigResp)
	}

	uploadResp := performMultipart(t, engine, "/console/v1/storage/upload", token, map[string]string{
		"disk":    "local",
		"purpose": "document",
	}, "file", "avatar.txt", []byte("hello upload"))
	if got := int(uploadResp["code"].(float64)); got != 0 {
		t.Fatalf("upload storage file failed: %#v", uploadResp)
	}
	uploadData := uploadResp["data"].(map[string]any)
	if path, _ := uploadData["path"].(string); !strings.HasPrefix(path, "console-test/documents/") {
		t.Fatalf("upload policy directory was not applied: %#v", uploadResp)
	}

	activeContentResp := performMultipartWithStatus(t, engine, "/console/v1/storage/upload", token, map[string]string{
		"disk":    "local",
		"purpose": "document",
	}, "file", "page.txt", []byte("<!doctype html><script>alert(1)</script>"), http.StatusUnprocessableEntity)
	if activeContentResp["data"].(map[string]any)["error_code"] != "invalid_upload" {
		t.Fatalf("active content should be rejected: %#v", activeContentResp)
	}

	policyLimitResp := performMultipartWithStatus(t, engine, "/console/v1/storage/upload", token, map[string]string{
		"disk":    "local",
		"purpose": "document",
	}, "file", "large.txt", bytes.Repeat([]byte("a"), 65), http.StatusRequestEntityTooLarge)
	if policyLimitResp["data"].(map[string]any)["error_code"] != "upload_too_large" {
		t.Fatalf("policy size limit should be rejected: %#v", policyLimitResp)
	}

	bodyLimitResp := performMultipartWithStatus(t, engine, "/console/v1/storage/upload", token, map[string]string{
		"disk":    "local",
		"purpose": "document",
	}, "file", "body.txt", bytes.Repeat([]byte("a"), 2048), http.StatusRequestEntityTooLarge, true)
	if bodyLimitResp["data"].(map[string]any)["error_code"] != "request_body_too_large" {
		t.Fatalf("request body size limit should be rejected: %#v", bodyLimitResp)
	}

	operationLogsResp := performJSON(t, engine, http.MethodGet, "/console/v1/logs/operations?list_all=true", nil, token)
	if got := int(operationLogsResp["code"].(float64)); got != 0 {
		t.Fatalf("list operation logs failed: %#v", operationLogsResp)
	}
	operationList := operationLogsResp["data"].(map[string]any)["list"].([]any)
	if len(operationList) == 0 {
		t.Fatalf("expected operation logs to be recorded: %#v", operationLogsResp)
	}
	firstOperationID, _ := operationList[0].(map[string]any)["id"].(string)
	if firstOperationID == "" {
		t.Fatalf("expected operation log id: %#v", operationLogsResp)
	}

	operationDetailResp := performJSON(t, engine, http.MethodGet, "/console/v1/logs/operations/"+firstOperationID, nil, token)
	if got := int(operationDetailResp["code"].(float64)); got != 0 {
		t.Fatalf("get operation log detail failed: %#v", operationDetailResp)
	}

	loginLogsResp := performJSON(t, engine, http.MethodGet, "/console/v1/logs/logins?list_all=true", nil, token)
	if got := int(loginLogsResp["code"].(float64)); got != 0 {
		t.Fatalf("list login logs failed: %#v", loginLogsResp)
	}
	if len(loginLogsResp["data"].(map[string]any)["list"].([]any)) == 0 {
		t.Fatalf("expected login logs to be recorded: %#v", loginLogsResp)
	}

	createUserResp := performJSON(t, engine, http.MethodPost, "/console/v1/users", map[string]any{
		"name":   "终端用户",
		"email":  "member@example.com",
		"status": model.UserStatusDisabled,
	}, token)
	if got := int(createUserResp["code"].(float64)); got != 0 {
		t.Fatalf("create user failed: %#v", createUserResp)
	}
	userData := createUserResp["data"].(map[string]any)
	userID, _ := userData["id"].(string)
	if userID == "" || int(userData["status"].(float64)) != model.UserStatusDisabled {
		t.Fatalf("created user status was not preserved: %#v", createUserResp)
	}
	userListResp := performJSON(t, engine, http.MethodGet, "/console/v1/users?status=0", nil, token)
	if got := int(userListResp["code"].(float64)); got != 0 || len(userListResp["data"].(map[string]any)["list"].([]any)) != 1 {
		t.Fatalf("list users failed: %#v", userListResp)
	}
	performJSON(t, engine, http.MethodPut, "/console/v1/users/"+userID+"/status", map[string]any{"status": model.UserStatusActive}, token)

	createArticleResp := performJSON(t, engine, http.MethodPost, "/console/v1/articles", map[string]any{
		"title":    "平台公告",
		"slug":     "platform-notice",
		"summary":  "公告摘要",
		"content":  "公告正文",
		"category": "公告",
		"status":   model.ArticleStatusDraft,
	}, token)
	if got := int(createArticleResp["code"].(float64)); got != 0 {
		t.Fatalf("create article failed: %#v", createArticleResp)
	}
	articleID := createArticleResp["data"].(map[string]any)["id"].(string)
	articleStatusResp := performJSON(t, engine, http.MethodPut, "/console/v1/articles/"+articleID+"/status", map[string]any{"status": model.ArticleStatusPublished}, token)
	if got := int(articleStatusResp["data"].(map[string]any)["status"].(float64)); got != model.ArticleStatusPublished {
		t.Fatalf("publish article failed: %#v", articleStatusResp)
	}
	articleListResp := performJSON(t, engine, http.MethodGet, "/console/v1/articles?status=1", nil, token)
	if got := int(articleListResp["code"].(float64)); got != 0 || len(articleListResp["data"].(map[string]any)["list"].([]any)) != 1 {
		t.Fatalf("list articles failed: %#v", articleListResp)
	}

	logoutResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/logout", map[string]any{
		"refresh_token": refreshToken,
	}, token)
	if got := int(logoutResp["code"].(float64)); got != 0 {
		t.Fatalf("logout failed: %#v", logoutResp)
	}
	performJSONWithStatus(t, engine, http.MethodGet, "/console/v1/dashboard/summary", nil, token, http.StatusUnauthorized)
}

func openConsoleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/console.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Article{},
		&model.ConsoleRole{},
		&model.ConsoleAdmin{},
		&model.SystemConfig{},
		&model.ConsoleOperationLog{},
		&model.ConsoleLoginLog{},
		&model.ConsoleSession{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS console_casbin_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ptype TEXT,
    v0 TEXT,
    v1 TEXT,
    v2 TEXT,
    v3 TEXT,
    v4 TEXT,
    v5 TEXT
);`).Error; err != nil {
		t.Fatalf("create casbin table: %v", err)
	}
	return db
}

func openConsoleTestEnforcer(t *testing.T, db *gorm.DB) *rbac.Enforcer {
	t.Helper()

	enforcer, err := rbac.New(db, &rbac.Config{
		TableName: "console_casbin_rules",
	})
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return enforcer
}

func seedConsoleTestData(t *testing.T, db *gorm.DB, enforcer *rbac.Enforcer) {
	t.Helper()

	password, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	role := model.ConsoleRole{
		Base:        model.Base{ID: "console-role-admin"},
		Name:        "Administrator",
		Code:        "admin",
		DisplayName: "System Administrator",
		Description: "seed role",
		MenuKeys:    datatype.NewStringArray([]string{"ConsoleDashboard", "ConsoleOverview", "ConsoleConfigs", "ConsoleSystemConfigs", "ConsoleSiteConfigs", "ConsoleContent", "ConsoleArticles", "ConsoleSystem", "ConsoleAdmins", "ConsoleUsers", "ConsoleRoles", "ConsoleSessions"}),
		Status:      model.ConsoleRoleStatusActive,
		Sort:        10,
	}
	if err := db.WithContext(context.Background()).Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	admin := model.ConsoleAdmin{
		Base:        model.Base{ID: "console-admin-demo"},
		Account:     "admin",
		Username:    "Admin",
		Email:       "admin@example.com",
		Password:    string(password),
		RealName:    "Demo Admin",
		DisplayName: "Console Admin",
		RoleID:      role.ID,
		Status:      model.ConsoleAdminStatusActive,
	}
	if err := db.WithContext(context.Background()).Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	systemConfigs := []model.SystemConfig{
		{
			Base:         model.Base{ID: "syscfg-platform-name"},
			ConfigGroup:  "platform",
			ConfigKey:    "site_name",
			Name:         "平台名称",
			Description:  "管理后台展示名称",
			ValueType:    "string",
			Value:        "Grove Console",
			DefaultValue: "Grove Console",
			IsEditable:   true,
			IsSystem:     true,
			SortOrder:    10,
		},
	}
	if err := db.Create(&systemConfigs).Error; err != nil {
		t.Fatalf("create system configs: %v", err)
	}

	policies := [][]string{
		{role.ID, "GET /console/v1/dashboard/summary"},
		{role.ID, "GET /console/v1/permissions/apis"},
		{role.ID, "GET /console/v1/roles"},
		{role.ID, "GET /console/v1/roles/:id"},
		{role.ID, "POST /console/v1/roles"},
		{role.ID, "PUT /console/v1/roles/:id"},
		{role.ID, "DELETE /console/v1/roles/:id"},
		{role.ID, "GET /console/v1/roles/:id/permissions"},
		{role.ID, "POST /console/v1/roles/:id/permissions"},
		{role.ID, "GET /console/v1/roles/:id/menus"},
		{role.ID, "POST /console/v1/roles/:id/menus"},
		{role.ID, "GET /console/v1/admins"},
		{role.ID, "GET /console/v1/sessions"},
		{role.ID, "DELETE /console/v1/sessions/:id"},
		{role.ID, "GET /console/v1/admins/:id"},
		{role.ID, "POST /console/v1/admins"},
		{role.ID, "PUT /console/v1/admins/:id"},
		{role.ID, "PUT /console/v1/admins/:id/status"},
		{role.ID, "PUT /console/v1/admins/:id/reset-password"},
		{role.ID, "DELETE /console/v1/admins/:id"},
		{role.ID, "GET /console/v1/users"},
		{role.ID, "GET /console/v1/users/:id"},
		{role.ID, "POST /console/v1/users"},
		{role.ID, "PUT /console/v1/users/:id"},
		{role.ID, "PUT /console/v1/users/:id/status"},
		{role.ID, "DELETE /console/v1/users/:id"},
		{role.ID, "GET /console/v1/articles"},
		{role.ID, "GET /console/v1/articles/:id"},
		{role.ID, "POST /console/v1/articles"},
		{role.ID, "PUT /console/v1/articles/:id"},
		{role.ID, "PUT /console/v1/articles/:id/status"},
		{role.ID, "DELETE /console/v1/articles/:id"},
		{role.ID, "GET /console/v1/system-configs"},
		{role.ID, "GET /console/v1/system-configs/groups/:group"},
		{role.ID, "POST /console/v1/system-configs"},
		{role.ID, "PUT /console/v1/system-configs/:id"},
		{role.ID, "DELETE /console/v1/system-configs/:id"},
		{role.ID, "GET /console/v1/storage/config"},
		{role.ID, "GET /console/v1/storage/all-configs"},
		{role.ID, "POST /console/v1/storage/upload"},
		{role.ID, "GET /console/v1/logs/operations"},
		{role.ID, "GET /console/v1/logs/operations/:id"},
		{role.ID, "GET /console/v1/logs/logins"},
	}
	if err := enforcer.AddConsolePolicies(policies); err != nil {
		t.Fatalf("add policies: %v", err)
	}
	if err := enforcer.AddConsoleRoleForUser(admin.ID, role.ID); err != nil {
		t.Fatalf("add grouping policy: %v", err)
	}
}

func performMultipart(t *testing.T, engine *gin.Engine, requestPath, token string, fields map[string]string, fileField, fileName string, fileContent []byte) map[string]any {
	return performMultipartWithStatus(t, engine, requestPath, token, fields, fileField, fileName, fileContent, http.StatusOK)
}

func performMultipartWithStatus(t *testing.T, engine *gin.Engine, requestPath, token string, fields map[string]string, fileField, fileName string, fileContent []byte, expectedStatus int, unknownLength ...bool) map[string]any {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, requestPath, &body)
	if len(unknownLength) > 0 && unknownLength[0] {
		req.ContentLength = -1
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != expectedStatus {
		t.Fatalf("unexpected status %d for multipart %s: %s", resp.Code, requestPath, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode multipart response: %v body=%s", err, resp.Body.String())
	}

	return payload
}

func performJSON(t *testing.T, engine *gin.Engine, method, path string, body any, token string) map[string]any {
	return performJSONWithStatus(t, engine, method, path, body, token, http.StatusOK)
}

func performJSONWithStatus(t *testing.T, engine *gin.Engine, method, path string, body any, token string, expectedStatus int) map[string]any {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Code != expectedStatus {
		t.Fatalf("unexpected status %d for %s %s: %s", resp.Code, method, path, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
	}
	return payload
}

func TestConsoleRouterCreatedAdminCanReadPermissionsAndMenus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openConsoleTestDB(t)
	enforcer := openConsoleTestEnforcer(t, db)
	seedConsoleTestData(t, db, enforcer)

	adminSvc := consoleservice.NewAdminService(database.NewConnectionsFromDBs(db, nil), enforcer)
	roleSvc := consoleservice.NewRoleService(database.NewConnectionsFromDBs(db, nil), enforcer)

	role, err := roleSvc.CreateRole(context.Background(), consoleservice.CreateRoleInput{
		Name:        "Support",
		Code:        "support",
		DisplayName: "Support",
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := enforcer.AddConsolePolicies([][]string{
		{role.ID, "GET /console/v1/dashboard/summary"},
	}); err != nil {
		t.Fatalf("set role permissions: %v", err)
	}
	if err := roleSvc.SetRoleMenus(context.Background(), consoleservice.SetRoleMenusInput{
		RoleID:   role.ID,
		MenuKeys: []string{"ConsoleDashboard"},
	}); err != nil {
		t.Fatalf("set role menus: %v", err)
	}

	admin, err := adminSvc.CreateAdmin(context.Background(), consoleservice.CreateAdminInput{
		Account:  "support1",
		Password: "secret123",
		RoleID:   role.ID,
	})
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	ok, err := enforcer.Can(admin.ID, "GET /console/v1/dashboard/summary")
	if err != nil {
		t.Fatalf("check casbin permission: %v", err)
	}
	if !ok {
		t.Fatal("expected created admin to inherit role permission through casbin grouping policy")
	}
}

func TestConsoleRouterRejectsDisabledAdminEvenWithOldToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Docs: config.DocsConfig{Enabled: false},
	}

	db := openConsoleTestDB(t)
	enforcer := openConsoleTestEnforcer(t, db)
	seedConsoleTestData(t, db, enforcer)

	p, err := provider.New(cfg, "console", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	p.DB = database.NewConnectionsFromDBs(db, nil)
	p.Enforcers = map[string]*rbac.Enforcer{"console": enforcer}

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("console"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	loginResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/login", map[string]any{
		"account":  "admin",
		"password": "password",
	}, "")
	loginData := loginResp["data"].(map[string]any)
	loginToken := loginData["token"].(map[string]any)
	token := loginToken["access_token"].(string)

	if err := db.Model(&model.ConsoleAdmin{}).
		Where("id = ?", "console-admin-demo").
		Update("status", model.ConsoleAdminStatusDisabled).Error; err != nil {
		t.Fatalf("disable admin: %v", err)
	}

	resp := performJSONWithStatus(t, engine, http.MethodGet, "/console/v1/dashboard/summary", nil, token, http.StatusForbidden)
	if got := int(resp["code"].(float64)); got == 0 {
		t.Fatalf("expected disabled admin to be rejected, got %#v", resp)
	}
}

func TestConsoleRouterCreateRoleValidationErrorUsesFieldMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Docs: config.DocsConfig{Enabled: false},
	}

	db := openConsoleTestDB(t)
	enforcer := openConsoleTestEnforcer(t, db)
	seedConsoleTestData(t, db, enforcer)

	p, err := provider.New(cfg, "console", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	p.DB = database.NewConnectionsFromDBs(db, nil)
	p.Enforcers = map[string]*rbac.Enforcer{"console": enforcer}

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("console"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	loginResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/login", map[string]any{
		"account":  "admin",
		"password": "password",
	}, "")
	loginData := loginResp["data"].(map[string]any)
	loginToken := loginData["token"].(map[string]any)
	token := loginToken["access_token"].(string)

	resp := performJSONWithStatus(t, engine, http.MethodPost, "/console/v1/roles", map[string]any{
		"name": "运营专员",
	}, token, http.StatusUnprocessableEntity)

	if got := int(resp["code"].(float64)); got != -1 {
		t.Fatalf("expected code -1, got %#v", resp)
	}

	data := resp["data"].(map[string]any)
	if got := data["error_code"]; got != "invalid_params" {
		t.Fatalf("unexpected error_code: %#v", got)
	}
	errorsMap := data["errors"].(map[string]any)
	codeErrors := errorsMap["code"].([]any)
	if len(codeErrors) != 1 || codeErrors[0] != "角色编码不能为空" {
		t.Fatalf("unexpected code errors: %#v", codeErrors)
	}
}

func TestConsoleRouterCreateRoleConflictReturnsStableErrorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Name: "grove", Env: "test"},
		Log: config.LogConfig{
			Level:   "error",
			Path:    t.TempDir(),
			Console: false,
			Service: "console-test",
		},
		JWT: config.JWTConfig{
			Secret:            "test-secret",
			Issuer:            "grove",
			AccessExpiryHours: 24,
		},
		Docs: config.DocsConfig{Enabled: false},
	}

	db := openConsoleTestDB(t)
	enforcer := openConsoleTestEnforcer(t, db)
	seedConsoleTestData(t, db, enforcer)

	p, err := provider.New(cfg, "console", provider.WithAuth())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	p.DB = database.NewConnectionsFromDBs(db, nil)
	p.Enforcers = map[string]*rbac.Enforcer{"console": enforcer}

	engine := gin.New()
	engine.Use(appmiddleware.RequestID(), appmiddleware.RequestMeta("console"), appmiddleware.Recovery())
	New(cfg, p).InstallToEngine(engine)

	loginResp := performJSON(t, engine, http.MethodPost, "/console/v1/auth/login", map[string]any{
		"account":  "admin",
		"password": "password",
	}, "")
	loginData := loginResp["data"].(map[string]any)
	loginToken := loginData["token"].(map[string]any)
	token := loginToken["access_token"].(string)

	resp := performJSONWithStatus(t, engine, http.MethodPost, "/console/v1/roles", map[string]any{
		"name": "管理员副本",
		"code": "admin",
	}, token, http.StatusConflict)

	if got := int(resp["code"].(float64)); got != -1 {
		t.Fatalf("expected code -1, got %#v", resp)
	}

	data := resp["data"].(map[string]any)
	if got := data["error_code"]; got != "role_code_exists" {
		t.Fatalf("unexpected error_code: %#v", got)
	}
	if got := resp["message"]; got != "角色编码已存在" {
		t.Fatalf("unexpected message: %#v", got)
	}
}
