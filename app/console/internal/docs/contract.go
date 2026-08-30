package docs

import (
	"net/http"

	"github.com/zhimma/grove/app/console/internal/handler"
	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/docsui"
	"github.com/zhimma/grove/pkg/storage"
)

func spec(cfg *config.Config) docsui.Document {
	doc := docsui.NewDocument(
		"Console - "+cfg.Docs.Title,
		"Console admin endpoints",
		cfg.Docs.Version,
		"/console/v1",
	)

	loginRequest := doc.AddSchema("ConsoleLoginRequest", handler.LoginRequest{})
	loginResponse := doc.AddSchema("ConsoleLoginResponse", handler.LoginResponse{})
	refreshRequest := doc.AddSchema("ConsoleRefreshTokenRequest", handler.RefreshTokenRequest{})
	refreshResponse := doc.AddSchema("ConsoleRefreshTokenResponse", handler.RefreshTokenResponse{})
	logoutRequest := doc.AddSchema("ConsoleLogoutRequest", handler.LogoutRequest{})
	updateProfileRequest := doc.AddSchema("ConsoleUpdateProfileRequest", handler.UpdateProfileRequest{})
	changePasswordRequest := doc.AddSchema("ConsoleChangePasswordRequest", handler.ChangePasswordRequest{})
	adminResponse := doc.AddSchema("ConsoleAdminResponse", handler.AdminResponse{})
	authorizationResponse := doc.AddSchema("ConsoleAuthorizationOverviewResponse", handler.AuthorizationOverviewResponse{})
	permissionTreeResponse := doc.AddSchema("ConsoleAPIPermissionTreeItem", handler.APIPermissionTreeItem{})
	dashboardResponse := doc.AddSchema("ConsoleDashboardSummary", consoleservice.SummaryOutput{})

	listSessionsResponse := doc.AddSchema("ConsoleListSessionsResponse", handler.ListSessionsResponse{})
	listRolesResponse := doc.AddSchema("ConsoleListRolesResponse", handler.ListRolesResponse{})
	roleResponse := doc.AddSchema("ConsoleRoleResponse", handler.RoleResponse{})
	createRoleRequest := doc.AddSchema("ConsoleCreateRoleRequest", handler.CreateRoleRequest{})
	updateRoleRequest := doc.AddSchema("ConsoleUpdateRoleRequest", handler.UpdateRoleRequest{})
	assignPermissionsRequest := doc.AddSchema("ConsoleAssignPermissionsRequest", handler.AssignPermissionsRequest{})
	assignMenusRequest := doc.AddSchema("ConsoleAssignMenusRequest", handler.AssignMenusRequest{})

	listAdminsResponse := doc.AddSchema("ConsoleListAdminsResponse", handler.ListAdminsResponse{})
	createAdminRequest := doc.AddSchema("ConsoleCreateAdminRequest", handler.CreateAdminRequest{})
	updateAdminRequest := doc.AddSchema("ConsoleUpdateAdminRequest", handler.UpdateAdminRequest{})
	updateAdminStatusRequest := doc.AddSchema("ConsoleUpdateAdminStatusRequest", handler.UpdateAdminStatusRequest{})
	resetAdminPasswordRequest := doc.AddSchema("ConsoleResetAdminPasswordRequest", handler.ResetAdminPasswordRequest{})
	messageResponse := doc.AddSchema("ConsoleMessageResponse", handler.MessageResponse{})
	userResponse := doc.AddSchema("ConsoleUserResponse", handler.UserResponse{})
	listUsersResponse := doc.AddSchema("ConsoleListUsersResponse", handler.ListUsersResponse{})
	createUserRequest := doc.AddSchema("ConsoleCreateUserRequest", handler.CreateUserRequest{})
	updateUserRequest := doc.AddSchema("ConsoleUpdateUserRequest", handler.UpdateUserRequest{})
	updateUserStatusRequest := doc.AddSchema("ConsoleUpdateUserStatusRequest", handler.UpdateUserStatusRequest{})
	articleResponse := doc.AddSchema("ConsoleArticleResponse", handler.ArticleResponse{})
	listArticlesResponse := doc.AddSchema("ConsoleListArticlesResponse", handler.ListArticlesResponse{})
	createArticleRequest := doc.AddSchema("ConsoleCreateArticleRequest", handler.CreateArticleRequest{})
	updateArticleRequest := doc.AddSchema("ConsoleUpdateArticleRequest", handler.UpdateArticleRequest{})
	updateArticleStatusRequest := doc.AddSchema("ConsoleUpdateArticleStatusRequest", handler.UpdateArticleStatusRequest{})

	listSystemConfigsResponse := doc.AddSchema("ConsoleListSystemConfigsResponse", handler.ListSystemConfigsResponse{})
	systemConfigItem := doc.AddSchema("ConsoleSystemConfigItem", handler.SystemConfigItem{})
	createSystemConfigRequest := doc.AddSchema("ConsoleCreateSystemConfigRequest", handler.CreateSystemConfigRequest{})
	updateSystemConfigRequest := doc.AddSchema("ConsoleUpdateSystemConfigRequest", handler.UpdateSystemConfigRequest{})

	storageConfigResponse := doc.AddSchema("ConsoleStorageClientConfig", storage.ClientConfig{})
	allStorageConfigsResponse := doc.AddSchema("ConsoleAllStorageConfigsResponse", consoleservice.GetAllStorageConfigsOutput{})
	uploadResponse := doc.AddSchema("ConsoleStoredFile", storage.StoredFile{})

	listOperationLogsResponse := doc.AddSchema("ConsoleListOperationLogsResponse", handler.ListOperationLogsResponse{})
	operationLogDetailResponse := doc.AddSchema("ConsoleOperationLogDetailResponse", handler.OperationLogDetailResponse{})
	listLoginLogsResponse := doc.AddSchema("ConsoleListLoginLogsResponse", handler.ListLoginLogsResponse{})

	addConsoleOperation(&doc, "认证", "/auth/login", http.MethodPost, "consoleLogin", "管理员登录", false,
		nil, docsui.JSONBody("登录凭据", loginRequest, true), loginResponse)
	addConsoleOperation(&doc, "认证", "/auth/refresh", http.MethodPost, "consoleRefreshToken", "刷新访问令牌", false,
		nil, docsui.JSONBody("刷新令牌", refreshRequest, true), refreshResponse)
	addConsoleOperation(&doc, "认证", "/auth/logout", http.MethodPost, "consoleLogout", "退出当前会话", true,
		nil, docsui.JSONBody("可选刷新令牌", logoutRequest, false), docsui.Schema{})
	addConsoleOperation(&doc, "认证", "/auth/me", http.MethodGet, "consoleGetCurrentAdmin", "获取当前管理员", true,
		nil, nil, adminResponse)
	addConsoleOperation(&doc, "认证", "/auth/me", http.MethodPut, "consoleUpdateCurrentAdmin", "更新当前管理员", true,
		nil, docsui.JSONBody("个人资料", updateProfileRequest, true), adminResponse)
	addConsoleOperation(&doc, "认证", "/auth/password", http.MethodPut, "consoleChangePassword", "修改当前管理员密码", true,
		nil, docsui.JSONBody("密码", changePasswordRequest, true), docsui.Schema{})
	addConsoleOperation(&doc, "认证", "/auth/permissions", http.MethodGet, "consoleGetAuthorizationOverview", "获取当前管理员权限", true,
		nil, nil, authorizationResponse)

	addConsoleOperation(&doc, "权限", "/permissions/apis", http.MethodGet, "consoleListAPIPermissions", "获取接口权限树", true,
		nil, nil, arrayOf(permissionTreeResponse))
	addConsoleOperation(&doc, "工作台", "/dashboard/summary", http.MethodGet, "consoleGetDashboardSummary", "获取工作台概览", true,
		nil, nil, dashboardResponse)

	addConsoleOperation(&doc, "会话", "/sessions", http.MethodGet, "consoleListSessions", "获取管理员会话列表", true,
		docsui.ParametersFor(handler.ListSessionsRequest{}, "form", "query"), nil, listSessionsResponse)
	addConsoleOperation(&doc, "会话", "/sessions/{id}", http.MethodDelete, "consoleRevokeSession", "强制下线管理员会话", true,
		idParameter("id", "会话ID"), nil, docsui.Schema{})

	rolePath := docsui.ParametersFor(handler.RolePathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "角色", "/roles", http.MethodGet, "consoleListRoles", "获取角色列表", true,
		docsui.ParametersFor(handler.ListRolesRequest{}, "form", "query"), nil, listRolesResponse)
	addConsoleOperation(&doc, "角色", "/roles", http.MethodPost, "consoleCreateRole", "创建角色", true,
		nil, docsui.JSONBody("角色", createRoleRequest, true), roleResponse)
	addConsoleOperation(&doc, "角色", "/roles/{id}", http.MethodGet, "consoleGetRole", "获取角色详情", true,
		rolePath, nil, roleResponse)
	addConsoleOperation(&doc, "角色", "/roles/{id}", http.MethodPut, "consoleUpdateRole", "更新角色", true,
		rolePath, docsui.JSONBody("角色", updateRoleRequest, true), roleResponse)
	addConsoleOperation(&doc, "角色", "/roles/{id}", http.MethodDelete, "consoleDeleteRole", "删除角色", true,
		rolePath, nil, docsui.Schema{})
	addConsoleOperation(&doc, "角色", "/roles/{id}/permissions", http.MethodGet, "consoleGetRolePermissions", "获取角色接口权限", true,
		rolePath, nil, arrayOf(docsui.Schema{Type: "string"}))
	addConsoleOperation(&doc, "角色", "/roles/{id}/permissions", http.MethodPost, "consoleAssignRolePermissions", "配置角色接口权限", true,
		rolePath, docsui.JSONBody("接口权限", assignPermissionsRequest, true), docsui.Schema{})
	addConsoleOperation(&doc, "角色", "/roles/{id}/menus", http.MethodGet, "consoleGetRoleMenus", "获取角色菜单权限", true,
		rolePath, nil, arrayOf(docsui.Schema{Type: "string"}))
	addConsoleOperation(&doc, "角色", "/roles/{id}/menus", http.MethodPost, "consoleAssignRoleMenus", "配置角色菜单权限", true,
		rolePath, docsui.JSONBody("菜单权限", assignMenusRequest, true), docsui.Schema{})

	adminPath := docsui.ParametersFor(handler.AdminPathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "管理员", "/admins", http.MethodGet, "consoleListAdmins", "获取管理员列表", true,
		docsui.ParametersFor(handler.ListAdminsRequest{}, "form", "query"), nil, listAdminsResponse)
	addConsoleOperation(&doc, "管理员", "/admins", http.MethodPost, "consoleCreateAdmin", "创建管理员", true,
		nil, docsui.JSONBody("管理员", createAdminRequest, true), adminResponse)
	addConsoleOperation(&doc, "管理员", "/admins/{id}", http.MethodGet, "consoleGetAdmin", "获取管理员详情", true,
		adminPath, nil, adminResponse)
	addConsoleOperation(&doc, "管理员", "/admins/{id}", http.MethodPut, "consoleUpdateAdmin", "更新管理员", true,
		adminPath, docsui.JSONBody("管理员", updateAdminRequest, true), adminResponse)
	addConsoleOperation(&doc, "管理员", "/admins/{id}/status", http.MethodPut, "consoleUpdateAdminStatus", "更新管理员状态", true,
		adminPath, docsui.JSONBody("状态", updateAdminStatusRequest, true), adminResponse)
	addConsoleOperation(&doc, "管理员", "/admins/{id}/reset-password", http.MethodPut, "consoleResetAdminPassword", "重置管理员密码", true,
		adminPath, docsui.JSONBody("新密码", resetAdminPasswordRequest, true), messageResponse)
	addConsoleOperation(&doc, "管理员", "/admins/{id}", http.MethodDelete, "consoleDeleteAdmin", "删除管理员", true,
		adminPath, nil, docsui.Schema{})

	userPath := docsui.ParametersFor(handler.UserPathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "用户管理", "/users", http.MethodGet, "consoleListUsers", "获取用户列表", true,
		docsui.ParametersFor(handler.ListUsersRequest{}, "form", "query"), nil, listUsersResponse)
	addConsoleOperation(&doc, "用户管理", "/users", http.MethodPost, "consoleCreateUser", "创建用户", true,
		nil, docsui.JSONBody("用户", createUserRequest, true), userResponse)
	addConsoleOperation(&doc, "用户管理", "/users/{id}", http.MethodGet, "consoleGetUser", "获取用户详情", true,
		userPath, nil, userResponse)
	addConsoleOperation(&doc, "用户管理", "/users/{id}", http.MethodPut, "consoleUpdateUser", "更新用户", true,
		userPath, docsui.JSONBody("用户", updateUserRequest, true), userResponse)
	addConsoleOperation(&doc, "用户管理", "/users/{id}/status", http.MethodPut, "consoleUpdateUserStatus", "更新用户状态", true,
		userPath, docsui.JSONBody("状态", updateUserStatusRequest, true), userResponse)
	addConsoleOperation(&doc, "用户管理", "/users/{id}", http.MethodDelete, "consoleDeleteUser", "删除用户", true,
		userPath, nil, docsui.Schema{})

	articlePath := docsui.ParametersFor(handler.ArticlePathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "内容管理", "/articles", http.MethodGet, "consoleListArticles", "获取文章列表", true,
		docsui.ParametersFor(handler.ListArticlesRequest{}, "form", "query"), nil, listArticlesResponse)
	addConsoleOperation(&doc, "内容管理", "/articles", http.MethodPost, "consoleCreateArticle", "创建文章", true,
		nil, docsui.JSONBody("文章", createArticleRequest, true), articleResponse)
	addConsoleOperation(&doc, "内容管理", "/articles/{id}", http.MethodGet, "consoleGetArticle", "获取文章详情", true,
		articlePath, nil, articleResponse)
	addConsoleOperation(&doc, "内容管理", "/articles/{id}", http.MethodPut, "consoleUpdateArticle", "更新文章", true,
		articlePath, docsui.JSONBody("文章", updateArticleRequest, true), articleResponse)
	addConsoleOperation(&doc, "内容管理", "/articles/{id}/status", http.MethodPut, "consoleUpdateArticleStatus", "更新文章状态", true,
		articlePath, docsui.JSONBody("状态", updateArticleStatusRequest, true), articleResponse)
	addConsoleOperation(&doc, "内容管理", "/articles/{id}", http.MethodDelete, "consoleDeleteArticle", "删除文章", true,
		articlePath, nil, docsui.Schema{})

	systemConfigPath := docsui.ParametersFor(handler.SystemConfigPathRequest{}, "uri", "path")
	systemConfigGroupPath := docsui.ParametersFor(handler.SystemConfigGroupPathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "系统配置", "/system-configs", http.MethodGet, "consoleListSystemConfigs", "获取系统配置列表", true,
		docsui.ParametersFor(handler.ListSystemConfigsRequest{}, "form", "query"), nil, listSystemConfigsResponse)
	addConsoleOperation(&doc, "系统配置", "/system-configs/groups/{group}", http.MethodGet, "consoleListSystemConfigGroup", "获取系统配置分组", true,
		systemConfigGroupPath, nil, arrayOf(systemConfigItem))
	addConsoleOperation(&doc, "系统配置", "/system-configs", http.MethodPost, "consoleCreateSystemConfig", "创建系统配置", true,
		nil, docsui.JSONBody("系统配置", createSystemConfigRequest, true), systemConfigItem)
	addConsoleOperation(&doc, "系统配置", "/system-configs/{id}", http.MethodPut, "consoleUpdateSystemConfig", "更新系统配置", true,
		systemConfigPath, docsui.JSONBody("配置值", updateSystemConfigRequest, true), systemConfigItem)
	addConsoleOperation(&doc, "系统配置", "/system-configs/{id}", http.MethodDelete, "consoleDeleteSystemConfig", "删除系统配置", true,
		systemConfigPath, nil, docsui.Schema{})

	addConsoleOperation(&doc, "文件存储", "/storage/config", http.MethodGet, "consoleGetStorageConfig", "获取存储配置", true,
		docsui.ParametersFor(handler.StorageConfigRequest{}, "form", "query"), nil, storageConfigResponse)
	addConsoleOperation(&doc, "文件存储", "/storage/all-configs", http.MethodGet, "consoleListStorageConfigs", "获取全部存储配置", true,
		nil, nil, allStorageConfigsResponse)
	addConsoleOperation(&doc, "文件存储", "/storage/upload", http.MethodPost, "consoleUploadFile", "上传文件", true,
		nil, docsui.MultipartBody("上传文件", uploadRequestSchema(), true), uploadResponse)
	addConsoleOperation(&doc, "文件存储", "/storage/download", http.MethodGet, "consoleDownloadFile", "下载文件", true,
		docsui.ParametersFor(handler.StorageDownloadRequest{}, "form", "query"), nil,
		docsui.Schema{Type: "string", Format: "binary"})

	operationLogPath := docsui.ParametersFor(handler.OperationLogPathRequest{}, "uri", "path")
	addConsoleOperation(&doc, "系统日志", "/logs/operations", http.MethodGet, "consoleListOperationLogs", "获取操作日志列表", true,
		docsui.ParametersFor(handler.ListOperationLogsRequest{}, "form", "query"), nil, listOperationLogsResponse)
	addConsoleOperation(&doc, "系统日志", "/logs/operations/{id}", http.MethodGet, "consoleGetOperationLog", "获取操作日志详情", true,
		operationLogPath, nil, operationLogDetailResponse)
	addConsoleOperation(&doc, "系统日志", "/logs/logins", http.MethodGet, "consoleListLoginLogs", "获取登录日志列表", true,
		docsui.ParametersFor(handler.ListLoginLogsRequest{}, "form", "query"), nil, listLoginLogsResponse)

	return doc
}

func addConsoleOperation(
	doc *docsui.Document,
	tag string,
	path string,
	method string,
	id string,
	summary string,
	bearer bool,
	parameters []docsui.Parameter,
	body *docsui.RequestBody,
	response docsui.Schema,
) {
	doc.Add(path, docsui.Operation{
		Method:      method,
		ID:          id,
		Summary:     summary,
		Tags:        []string{tag},
		BearerAuth:  bearer,
		Parameters:  parameters,
		RequestBody: body,
		Responses:   docsui.StandardResponses("请求成功", response),
	})
}

func arrayOf(item docsui.Schema) docsui.Schema {
	return docsui.Schema{Type: "array", Items: &item}
}

func idParameter(name, description string) []docsui.Parameter {
	return []docsui.Parameter{{
		Name:        name,
		In:          "path",
		Description: description,
		Required:    true,
		Schema:      docsui.Schema{Type: "string"},
	}}
}

func uploadRequestSchema() docsui.Schema {
	return docsui.Schema{
		Type: "object",
		Properties: map[string]docsui.Schema{
			"file":    {Type: "string", Format: "binary", Description: "文件"},
			"disk":    {Type: "string", Description: "存储磁盘"},
			"purpose": {Type: "string", Description: "上传用途"},
		},
		Required: []string{"file"},
	}
}
