package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/datatype"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/rbac"
)

type RoleService struct {
	dbs               *database.Connections
	rolePolicies      rolePolicyStore
	runtimePermission *RuntimePermissionCatalog
	pages             pagination.Policy
}

type rolePolicyStore interface {
	GetConsolePoliciesForRole(roleID string) ([][]string, error)
	ReplaceConsolePoliciesForRole(roleID string, permissions []string) error
}

type ListRolesInput struct {
	pagination.Request
	Keyword     string
	OrderBy     []string
	Status      *int
	CreatedFrom string
	CreatedTo   string
}

type ListRolesOutput struct {
	List []Role
	Meta pagination.Meta
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Status      int    `json:"status"`
	IsSuper     bool   `json:"is_super"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type GetRoleInput struct {
	RoleID string
}

type CreateRoleInput struct {
	Name        string
	Code        string
	DisplayName string
	Description string
	Sort        int
	Status      int
}

type UpdateRoleInput struct {
	RoleID      string
	Name        *string
	Code        *string
	DisplayName *string
	Description *string
	Status      *int
	Sort        *int
}

type DeleteRoleInput struct {
	RoleID string
}

type GetRolePermissionsInput struct {
	RoleID string
}

type SetRolePermissionsInput struct {
	RoleID         string
	APIPermissions []string
}

type GetRoleMenusInput struct {
	RoleID string
}

type SetRoleMenusInput struct {
	RoleID   string
	MenuKeys []string
}

func NewRoleService(dbs *database.Connections, enforcer *rbac.Enforcer, catalog *RuntimePermissionCatalog, pages pagination.Policy) *RoleService {
	return &RoleService{
		dbs:               dbs,
		rolePolicies:      enforcer,
		runtimePermission: catalog,
		pages:             pages,
	}
}

func (s *RoleService) ListRoles(ctx context.Context, in ListRolesInput) (*ListRolesOutput, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	query := s.dbs.Default().WithContext(ctx).Model(&model.ConsoleRole{})
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"name LIKE ? OR code LIKE ? OR display_name LIKE ? OR description LIKE ?",
			like, like, like, like,
		)
	}
	if in.Status != nil {
		query = query.Where("status = ?", *in.Status)
	}

	var err error
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage("时间范围格式不正确")
	}

	page := s.pages.Resolve(in.Request)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	if len(in.OrderBy) == 0 {
		query = query.Order("sort ASC").Order("created_at DESC")
	} else {
		for _, item := range in.OrderBy {
			field, direction := parseOrderBy(item)
			column := ""
			switch field {
			case "sort", "created_at", "updated_at", "status", "name", "code":
				column = field
			}
			if column == "" {
				continue
			}
			query = query.Order(column + " " + direction)
		}
	}

	query = page.Apply(query)

	var roles []model.ConsoleRole
	if err := query.Find(&roles).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	list := make([]Role, 0, len(roles))
	for _, item := range roles {
		list = append(list, toRoleOutput(item))
	}

	return &ListRolesOutput{
		List: list,
		Meta: pagination.NewMeta(total, page),
	}, nil
}

func (s *RoleService) GetRole(ctx context.Context, in GetRoleInput) (*Role, error) {
	roleModel, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return nil, err
	}
	result := toRoleOutput(*roleModel)
	return &result, nil
}

func (s *RoleService) CreateRole(ctx context.Context, in CreateRoleInput) (*Role, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)
	if name == "" || code == "" {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("角色名称和角色编码不能为空")
	}
	if !isRoleStatusValid(in.Status) && in.Status != 0 {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("角色状态不合法")
	}
	if err := s.ensureRoleCodeUnique(ctx, "", code); err != nil {
		return nil, err
	}

	role := model.ConsoleRole{
		Name:        name,
		Code:        code,
		DisplayName: strings.TrimSpace(in.DisplayName),
		Description: strings.TrimSpace(in.Description),
		Sort:        in.Sort,
		Status:      model.ConsoleRoleStatusActive,
	}
	if in.Status == model.ConsoleRoleStatusDisabled {
		role.Status = in.Status
	}

	if err := s.dbs.Default().WithContext(ctx).Create(&role).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.GetRole(ctx, GetRoleInput{RoleID: role.ID})
}

func (s *RoleService) UpdateRole(ctx context.Context, in UpdateRoleInput) (*Role, error) {
	current, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return nil, err
	}
	if current.IsSuper {
		return nil, errx.Forbidden().WithMessage("系统角色不允许修改")
	}

	updates := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("角色名称不能为空")
		}
		updates["name"] = name
	}
	if in.Code != nil {
		code := strings.TrimSpace(*in.Code)
		if code == "" {
			return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("角色编码不能为空")
		}
		if err := s.ensureRoleCodeUnique(ctx, in.RoleID, code); err != nil {
			return nil, err
		}
		updates["code"] = code
	}
	if in.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*in.DisplayName)
	}
	if in.Description != nil {
		updates["description"] = strings.TrimSpace(*in.Description)
	}
	if in.Status != nil {
		if !isRoleStatusValid(*in.Status) {
			return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("角色状态不合法")
		}
		updates["status"] = *in.Status
	}
	if in.Sort != nil {
		updates["sort"] = *in.Sort
	}

	if len(updates) > 0 {
		if err := s.dbs.Default().WithContext(ctx).
			Model(&model.ConsoleRole{}).
			Where("id = ?", in.RoleID).
			Updates(updates).Error; err != nil {
			return nil, errx.Internal().WithCause(err)
		}
	}

	return s.GetRole(ctx, GetRoleInput{RoleID: in.RoleID})
}

func (s *RoleService) DeleteRole(ctx context.Context, in DeleteRoleInput) error {
	role, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return err
	}
	if role.IsSuper {
		return errx.Forbidden().WithMessage("系统角色不允许删除")
	}

	var count int64
	if err := s.dbs.Default().WithContext(ctx).
		Model(&model.ConsoleAdmin{}).
		Where("role_id = ?", in.RoleID).
		Count(&count).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	if count > 0 {
		return errx.Conflict().WithCode("role_in_use").WithMessage("该角色已被管理员使用，无法删除")
	}

	permissions, err := s.rolePermissionKeys(in.RoleID)
	if err != nil {
		return err
	}
	if s.rolePolicies != nil {
		if err := s.rolePolicies.ReplaceConsolePoliciesForRole(in.RoleID, nil); err != nil {
			return rbacSyncError("角色权限清理失败", err, nil)
		}
	}
	if err := s.dbs.Default().WithContext(ctx).Delete(&model.ConsoleRole{}, "id = ?", in.RoleID).Error; err != nil {
		var compensationErr error
		if s.rolePolicies != nil {
			compensationErr = s.rolePolicies.ReplaceConsolePoliciesForRole(in.RoleID, permissions)
		}
		return errx.Internal().WithCause(errors.Join(err, compensationErr))
	}
	return nil
}

func (s *RoleService) GetRolePermissions(ctx context.Context, in GetRolePermissionsInput) ([]string, error) {
	if _, err := s.loadRole(ctx, in.RoleID); err != nil {
		return nil, err
	}
	if s.rolePolicies == nil {
		return []string{}, nil
	}
	policies, err := s.rolePolicies.GetConsolePoliciesForRole(in.RoleID)
	if err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	keys := make([]string, 0, len(policies))
	for _, policy := range policies {
		if len(policy) >= 2 {
			keys = append(keys, policy[1])
		}
	}
	return uniqueNonEmptyStrings(keys), nil
}

func (s *RoleService) SetRolePermissions(ctx context.Context, in SetRolePermissionsInput) error {
	role, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return err
	}
	if role.IsSuper {
		return errx.Forbidden().WithMessage("系统角色不允许修改接口权限")
	}
	if s.rolePolicies == nil {
		return nil
	}

	keys := uniqueNonEmptyStrings(in.APIPermissions)
	if err := s.validateAPIIdentifiers(keys); err != nil {
		return err
	}
	if err := s.rolePolicies.ReplaceConsolePoliciesForRole(in.RoleID, keys); err != nil {
		return rbacSyncError("角色权限更新失败", err, nil)
	}
	return nil
}

func (s *RoleService) rolePermissionKeys(roleID string) ([]string, error) {
	if s.rolePolicies == nil {
		return nil, nil
	}
	policies, err := s.rolePolicies.GetConsolePoliciesForRole(roleID)
	if err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	keys := make([]string, 0, len(policies))
	for _, policy := range policies {
		if len(policy) >= 2 {
			keys = append(keys, policy[1])
		}
	}
	return uniqueNonEmptyStrings(keys), nil
}

func (s *RoleService) validateAPIIdentifiers(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if s.runtimePermission == nil {
		return errx.ServiceUnavailable().WithMessage("运行时接口权限清单未加载")
	}

	invalid := make([]string, 0)
	for _, key := range keys {
		if !s.runtimePermission.HasAPIIdentifier(key) {
			invalid = append(invalid, key)
		}
	}
	if len(invalid) > 0 {
		return errx.InvalidParams().WithHTTPStatus(422).WithMessage("存在未注册的接口权限标识: " + strings.Join(invalid, ", "))
	}
	return nil
}

func (s *RoleService) GetRoleMenus(ctx context.Context, in GetRoleMenusInput) ([]string, error) {
	role, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return nil, err
	}
	return normalizeConsoleMenuKeys(role.MenuKeys), nil
}

func (s *RoleService) SetRoleMenus(ctx context.Context, in SetRoleMenusInput) error {
	role, err := s.loadRole(ctx, in.RoleID)
	if err != nil {
		return err
	}
	if role.IsSuper {
		return errx.Forbidden().WithMessage("系统角色不允许修改菜单权限")
	}

	keys := uniqueNonEmptyStrings(in.MenuKeys)
	if err := validateConsoleMenuKeys(keys); err != nil {
		return err
	}
	if err := s.dbs.Default().WithContext(ctx).
		Model(&model.ConsoleRole{}).
		Where("id = ?", in.RoleID).
		Update("menu_keys", datatype.NewStringArray(normalizeConsoleMenuKeys(keys))).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *RoleService) loadRole(ctx context.Context, roleID string) (*model.ConsoleRole, error) {
	if s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var role model.ConsoleRole
	if err := s.dbs.Default().WithContext(ctx).First(&role, "id = ?", strings.TrimSpace(roleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("角色不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &role, nil
}

func (s *RoleService) ensureRoleCodeUnique(ctx context.Context, excludeID, code string) error {
	var count int64
	query := s.dbs.Default().WithContext(ctx).Model(&model.ConsoleRole{}).Where("code = ?", code)
	if strings.TrimSpace(excludeID) != "" {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	if count > 0 {
		return errx.Conflict().WithCode("role_code_exists").WithMessage("角色编码已存在")
	}
	return nil
}

func isRoleStatusValid(status int) bool {
	return status == model.ConsoleRoleStatusActive || status == model.ConsoleRoleStatusDisabled
}

func toRoleOutput(role model.ConsoleRole) Role {
	return Role{
		ID:          role.ID,
		Name:        role.Name,
		Code:        role.Code,
		DisplayName: role.DisplayName,
		Description: role.Description,
		Sort:        role.Sort,
		Status:      role.Status,
		IsSuper:     role.IsSuper,
		CreatedAt:   formatTime(role.CreatedAt),
		UpdatedAt:   formatTime(role.UpdatedAt),
	}
}
