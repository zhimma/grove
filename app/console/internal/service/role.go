package service

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/datatype"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/rbac"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/transaction"
)

type RoleService struct {
	dbRepo            database.Repo
	enforcer          *rbac.Enforcer
	runtimePermission *RuntimePermissionCatalog
	txManager         transaction.Manager
}

type ListRolesInput struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	Status      *int
	CreatedFrom string
	CreatedTo   string
}

type ListRolesOutput struct {
	List []Role
	Meta ListMeta
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

func NewRoleService(dbRepo database.Repo, enforcer *rbac.Enforcer, runtimePermission ...*RuntimePermissionCatalog) *RoleService {
	var catalog *RuntimePermissionCatalog
	if len(runtimePermission) > 0 {
		catalog = runtimePermission[0]
	}
	return &RoleService{
		dbRepo:            dbRepo,
		enforcer:          enforcer,
		runtimePermission: catalog,
	}
}

func (s *RoleService) WithTransaction(manager transaction.Manager) *RoleService {
	s.txManager = manager
	return s
}

func (s *RoleService) ListRoles(ctx context.Context, in ListRolesInput) (*ListRolesOutput, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	query := s.dbRepo.Default().WithContext(ctx).Model(&model.ConsoleRole{})
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

	page, pageSize := resolvePage(ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
		Offset:   in.Offset,
		Limit:    in.Limit,
		ListAll:  in.ListAll,
	})

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

	if !in.ListAll {
		offset := in.Offset
		if offset <= 0 {
			offset = (page - 1) * pageSize
		}
		query = query.Offset(offset).Limit(pageSize)
	}

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
		Meta: NewListMeta(total, page, pageSize),
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
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
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

	if err := s.dbRepo.Default().WithContext(ctx).Create(&role).Error; err != nil {
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
		if err := s.dbRepo.Default().WithContext(ctx).
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
	if err := s.dbRepo.Default().WithContext(ctx).
		Model(&model.ConsoleAdmin{}).
		Where("role_id = ?", in.RoleID).
		Count(&count).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	if count > 0 {
		return errx.Conflict().WithCode("role_in_use").WithMessage("该角色已被管理员使用，无法删除")
	}

	return s.execute(ctx, func(txCtx context.Context) error {
		db := transaction.GetDB(txCtx, s.dbRepo.Default())
		if db == nil {
			return errx.ServiceUnavailable().WithMessage("默认数据库未配置")
		}
		if err := db.Delete(&model.ConsoleRole{}, "id = ?", in.RoleID).Error; err != nil {
			return errx.Internal().WithCause(err)
		}
		if s.enforcer != nil {
			if err := s.enforcer.RemoveConsolePoliciesForRole(in.RoleID); err != nil {
				return errx.Internal().WithCause(err)
			}
			if _, err := s.enforcer.RemoveFilteredGroupingPolicy(1, in.RoleID); err != nil {
				return errx.Internal().WithCause(err)
			}
		}
		return nil
	})
}

func (s *RoleService) GetRolePermissions(ctx context.Context, in GetRolePermissionsInput) ([]string, error) {
	if _, err := s.loadRole(ctx, in.RoleID); err != nil {
		return nil, err
	}
	if s.enforcer == nil {
		return []string{}, nil
	}
	policies, err := s.enforcer.GetConsolePoliciesForRole(in.RoleID)
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
	if s.enforcer == nil {
		return nil
	}

	keys := uniqueNonEmptyStrings(in.APIPermissions)
	if err := s.validateAPIIdentifiers(keys); err != nil {
		return err
	}
	return s.execute(ctx, func(txCtx context.Context) error {
		if err := s.enforcer.RemoveConsolePoliciesForRole(in.RoleID); err != nil {
			return errx.Internal().WithCause(err)
		}
		if len(keys) == 0 {
			return nil
		}

		rules := make([][]string, 0, len(keys))
		for _, key := range keys {
			rules = append(rules, []string{in.RoleID, key})
		}
		if err := s.enforcer.AddConsolePolicies(rules); err != nil {
			return errx.Internal().WithCause(err)
		}
		return nil
	})
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
	return FilterConsoleMenuKeys(role.MenuKeys), nil
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
	if err := ValidateConsoleMenuKeys(keys); err != nil {
		return err
	}
	if err := s.dbRepo.Default().WithContext(ctx).
		Model(&model.ConsoleRole{}).
		Where("id = ?", in.RoleID).
		Update("menu_keys", datatype.NewStringArray(FilterConsoleMenuKeys(keys))).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *RoleService) loadRole(ctx context.Context, roleID string) (*model.ConsoleRole, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var role model.ConsoleRole
	if err := s.dbRepo.Default().WithContext(ctx).First(&role, "id = ?", strings.TrimSpace(roleID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("角色不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &role, nil
}

func (s *RoleService) ensureRoleCodeUnique(ctx context.Context, excludeID, code string) error {
	var count int64
	query := s.dbRepo.Default().WithContext(ctx).Model(&model.ConsoleRole{}).Where("code = ?", code)
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

func (s *RoleService) execute(ctx context.Context, fn func(context.Context) error) error {
	if s.txManager == nil {
		return fn(ctx)
	}
	return s.txManager.Execute(ctx, fn)
}
