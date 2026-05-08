package service

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	pkgcasbin "github.com/zhimma/grove/pkg/casbin"
	"github.com/zhimma/grove/pkg/database"
	pkgerrors "github.com/zhimma/grove/pkg/errors"
	"github.com/zhimma/grove/pkg/transaction"
)

type AdminService struct {
	dbRepo    database.Repo
	enforcer  *pkgcasbin.Enforcer
	txManager transaction.Manager
}

type ListAdminsInput struct {
	Page        int
	PageSize    int
	Offset      int
	Limit       int
	ListAll     bool
	Keyword     string
	OrderBy     []string
	RoleID      string
	Status      *int
	CreatedFrom string
	CreatedTo   string
}

type ListAdminsResult struct {
	List []model.ConsoleAdmin
	Meta ListMeta
}

type GetAdminInput struct {
	AdminID string
}

type CreateAdminInput struct {
	Account     string
	Username    string
	Email       string
	Phone       string
	Password    string
	RealName    string
	DisplayName string
	Avatar      string
	RoleID      string
	Status      int
	Remark      string
}

type UpdateAdminInput struct {
	AdminID     string
	Account     *string
	Username    *string
	Email       *string
	Phone       *string
	Password    *string
	RealName    *string
	DisplayName *string
	Avatar      *string
	RoleID      *string
	Status      *int
	Remark      *string
}

type UpdateAdminStatusInput struct {
	AdminID string
	Status  int
}

type DeleteAdminInput struct {
	AdminID    string
	OperatorID string
}

type ResetAdminPasswordInput struct {
	AdminID  string
	Password string
}

func NewAdminService(dbRepo database.Repo, enforcer *pkgcasbin.Enforcer) *AdminService {
	return &AdminService{dbRepo: dbRepo, enforcer: enforcer}
}

func (s *AdminService) WithTransaction(manager transaction.Manager) *AdminService {
	s.txManager = manager
	return s
}

func (s *AdminService) ListAdmins(ctx context.Context, in ListAdminsInput) (*ListAdminsResult, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	page, pageSize := resolvePage(ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
		Offset:   in.Offset,
		Limit:    in.Limit,
		ListAll:  in.ListAll,
	})

	query := s.dbRepo.Default().WithContext(ctx).Model(&model.ConsoleAdmin{}).Preload("Role")
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"account LIKE ? OR username LIKE ? OR real_name LIKE ? OR display_name LIKE ? OR phone LIKE ? OR email LIKE ?",
			like, like, like, like, like, like,
		)
	}
	if roleID := strings.TrimSpace(in.RoleID); roleID != "" {
		query = query.Where("role_id = ?", roleID)
	}
	if in.Status != nil {
		query = query.Where("status = ?", *in.Status)
	}

	var err error
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, pkgerrors.InvalidParams().WithMessage("时间范围格式不正确")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, pkgerrors.Internal().WithCause(err)
	}

	if len(in.OrderBy) == 0 {
		query = query.Order("created_at DESC")
	} else {
		for _, item := range in.OrderBy {
			field, direction := parseOrderBy(item)
			switch field {
			case "created_at", "updated_at", "account", "status":
				query = query.Order(field + " " + direction)
			}
		}
	}

	if !in.ListAll {
		offset := in.Offset
		if offset <= 0 {
			offset = (page - 1) * pageSize
		}
		query = query.Offset(offset).Limit(pageSize)
	}

	var list []model.ConsoleAdmin
	if err := query.Find(&list).Error; err != nil {
		return nil, pkgerrors.Internal().WithCause(err)
	}
	for i := range list {
		list[i].Password = ""
	}

	return &ListAdminsResult{
		List: list,
		Meta: NewListMeta(total, page, pageSize),
	}, nil
}

func (s *AdminService) GetAdmin(ctx context.Context, in GetAdminInput) (*model.ConsoleAdmin, error) {
	return s.loadAdmin(ctx, in.AdminID)
}

func (s *AdminService) CreateAdmin(ctx context.Context, in CreateAdminInput) (*model.ConsoleAdmin, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	account := strings.TrimSpace(in.Account)
	username := strings.TrimSpace(in.Username)
	email := normalizeOptional(strings.TrimSpace(in.Email))
	phone := normalizeOptional(strings.TrimSpace(in.Phone))
	password := strings.TrimSpace(in.Password)
	roleID := strings.TrimSpace(in.RoleID)

	if account == "" || password == "" || roleID == "" {
		return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("账号、密码和角色不能为空")
	}
	if username == "" {
		username = account
	}
	if !isAdminStatusValid(in.Status) && in.Status != 0 {
		return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("管理员状态不合法")
	}

	if err := s.ensureRoleExists(ctx, roleID); err != nil {
		return nil, err
	}
	if err := s.ensureAdminUnique(ctx, "", account, email, phone); err != nil {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkgerrors.Internal().WithCause(err)
	}

	admin := model.ConsoleAdmin{
		Account:     account,
		Username:    username,
		Email:       email,
		Phone:       phone,
		Password:    string(hashedPassword),
		RealName:    strings.TrimSpace(in.RealName),
		DisplayName: strings.TrimSpace(in.DisplayName),
		Avatar:      strings.TrimSpace(in.Avatar),
		RoleID:      roleID,
		Status:      model.ConsoleAdminStatusActive,
		Remark:      strings.TrimSpace(in.Remark),
	}
	if in.Status == model.ConsoleAdminStatusDisabled || in.Status == model.ConsoleAdminStatusLocked {
		admin.Status = in.Status
	}

	if err := s.execute(ctx, func(txCtx context.Context) error {
		db := transaction.GetDB(txCtx, s.dbRepo.Default())
		if db == nil {
			return pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
		}
		if err := db.Create(&admin).Error; err != nil {
			return pkgerrors.Internal().WithCause(err)
		}
		return s.syncAdminRoleBinding(admin.ID, admin.RoleID)
	}); err != nil {
		return nil, err
	}
	return s.GetAdmin(ctx, GetAdminInput{AdminID: admin.ID})
}

func (s *AdminService) UpdateAdmin(ctx context.Context, in UpdateAdminInput) (*model.ConsoleAdmin, error) {
	admin, err := s.loadAdmin(ctx, in.AdminID)
	if err != nil {
		return nil, err
	}
	if admin.HasSuperAccess() {
		return nil, pkgerrors.Forbidden().WithMessage("超级管理员不允许修改")
	}

	newAccount := admin.Account
	newEmail := admin.Email
	newPhone := admin.Phone
	newRoleID := admin.RoleID
	updates := map[string]any{}

	if in.Account != nil {
		newAccount = strings.TrimSpace(*in.Account)
		if newAccount == "" {
			return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("账号不能为空")
		}
		updates["account"] = newAccount
	}
	if in.Username != nil {
		updates["username"] = strings.TrimSpace(*in.Username)
	}
	if in.Email != nil {
		newEmail = normalizeOptional(strings.TrimSpace(*in.Email))
		updates["email"] = newEmail
	}
	if in.Phone != nil {
		newPhone = normalizeOptional(strings.TrimSpace(*in.Phone))
		updates["phone"] = newPhone
	}
	if in.RealName != nil {
		updates["real_name"] = strings.TrimSpace(*in.RealName)
	}
	if in.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*in.DisplayName)
	}
	if in.Avatar != nil {
		updates["avatar"] = strings.TrimSpace(*in.Avatar)
	}
	if in.RoleID != nil {
		newRoleID = strings.TrimSpace(*in.RoleID)
		if newRoleID == "" {
			return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("角色不能为空")
		}
		if err := s.ensureRoleExists(ctx, newRoleID); err != nil {
			return nil, err
		}
		updates["role_id"] = newRoleID
	}
	if in.Status != nil {
		if !isAdminStatusValid(*in.Status) {
			return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("管理员状态不合法")
		}
		updates["status"] = *in.Status
	}
	if in.Remark != nil {
		updates["remark"] = strings.TrimSpace(*in.Remark)
	}
	if in.Password != nil && strings.TrimSpace(*in.Password) != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*in.Password)), bcrypt.DefaultCost)
		if err != nil {
			return nil, pkgerrors.Internal().WithCause(err)
		}
		updates["password"] = string(hashedPassword)
	}

	if err := s.ensureAdminUnique(ctx, in.AdminID, newAccount, newEmail, newPhone); err != nil {
		return nil, err
	}
	if err := s.execute(ctx, func(txCtx context.Context) error {
		db := transaction.GetDB(txCtx, s.dbRepo.Default())
		if db == nil {
			return pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
		}
		if len(updates) > 0 {
			if err := db.
				Model(&model.ConsoleAdmin{}).
				Where("id = ?", in.AdminID).
				Updates(updates).Error; err != nil {
				return pkgerrors.Internal().WithCause(err)
			}
		}
		if newRoleID != admin.RoleID {
			return s.syncAdminRoleBinding(in.AdminID, newRoleID)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.GetAdmin(ctx, GetAdminInput{AdminID: in.AdminID})
}

func (s *AdminService) UpdateAdminStatus(ctx context.Context, in UpdateAdminStatusInput) (*model.ConsoleAdmin, error) {
	admin, err := s.loadAdmin(ctx, in.AdminID)
	if err != nil {
		return nil, err
	}
	if admin.HasSuperAccess() {
		return nil, pkgerrors.Forbidden().WithMessage("超级管理员状态不允许修改")
	}
	if !isAdminStatusValid(in.Status) {
		return nil, pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("管理员状态不合法")
	}

	if err := s.dbRepo.Default().WithContext(ctx).
		Model(&model.ConsoleAdmin{}).
		Where("id = ?", in.AdminID).
		Update("status", in.Status).Error; err != nil {
		return nil, pkgerrors.Internal().WithCause(err)
	}
	return s.GetAdmin(ctx, GetAdminInput{AdminID: in.AdminID})
}

func (s *AdminService) DeleteAdmin(ctx context.Context, in DeleteAdminInput) error {
	admin, err := s.loadAdmin(ctx, in.AdminID)
	if err != nil {
		return err
	}
	if admin.HasSuperAccess() {
		return pkgerrors.Forbidden().WithMessage("超级管理员不允许删除")
	}
	if strings.TrimSpace(in.OperatorID) != "" && strings.TrimSpace(in.OperatorID) == in.AdminID {
		return pkgerrors.Forbidden().WithMessage("当前管理员不能删除自己")
	}

	return s.execute(ctx, func(txCtx context.Context) error {
		db := transaction.GetDB(txCtx, s.dbRepo.Default())
		if db == nil {
			return pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
		}
		if err := db.Delete(&model.ConsoleAdmin{}, "id = ?", in.AdminID).Error; err != nil {
			return pkgerrors.Internal().WithCause(err)
		}
		if s.enforcer != nil {
			if _, err := s.enforcer.RemoveFilteredGroupingPolicy(0, in.AdminID); err != nil {
				return pkgerrors.Internal().WithCause(err)
			}
		}
		return nil
	})
}

func (s *AdminService) ResetPassword(ctx context.Context, in ResetAdminPasswordInput) error {
	admin, err := s.loadAdmin(ctx, in.AdminID)
	if err != nil {
		return err
	}
	if admin.HasSuperAccess() {
		return pkgerrors.Forbidden().WithMessage("超级管理员密码不允许重置")
	}
	password := strings.TrimSpace(in.Password)
	if password == "" {
		return pkgerrors.InvalidParams().WithHTTPStatus(422).WithMessage("密码不能为空")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return pkgerrors.Internal().WithCause(err)
	}
	if err := s.dbRepo.Default().WithContext(ctx).
		Model(&model.ConsoleAdmin{}).
		Where("id = ?", in.AdminID).
		Update("password", string(hashedPassword)).Error; err != nil {
		return pkgerrors.Internal().WithCause(err)
	}
	return nil
}

func (s *AdminService) loadAdmin(ctx context.Context, adminID string) (*model.ConsoleAdmin, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, pkgerrors.ServiceUnavailable().WithMessage("默认数据库未配置")
	}

	var admin model.ConsoleAdmin
	if err := s.dbRepo.Default().WithContext(ctx).
		Preload("Role").
		First(&admin, "id = ?", strings.TrimSpace(adminID)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.NotFound().WithMessage("管理员不存在")
		}
		return nil, pkgerrors.Internal().WithCause(err)
	}
	admin.Password = ""
	return &admin, nil
}

func (s *AdminService) ensureRoleExists(ctx context.Context, roleID string) error {
	var count int64
	if err := s.dbRepo.Default().WithContext(ctx).
		Model(&model.ConsoleRole{}).
		Where("id = ?", roleID).
		Count(&count).Error; err != nil {
		return pkgerrors.Internal().WithCause(err)
	}
	if count == 0 {
		return pkgerrors.NotFound().WithMessage("角色不存在")
	}
	return nil
}

func (s *AdminService) ensureAdminUnique(ctx context.Context, excludeID, account, email, phone string) error {
	checks := []struct {
		column  string
		value   string
		code    string
		message string
	}{
		{column: "account", value: account, code: "account_exists", message: "账号已存在"},
		{column: "email", value: email, code: "email_exists", message: "邮箱已存在"},
		{column: "phone", value: phone, code: "phone_exists", message: "手机号已存在"},
	}

	for _, check := range checks {
		if check.value == "" {
			continue
		}
		query := s.dbRepo.Default().WithContext(ctx).Model(&model.ConsoleAdmin{}).Where(check.column+" = ?", check.value)
		if strings.TrimSpace(excludeID) != "" {
			query = query.Where("id <> ?", excludeID)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return pkgerrors.Internal().WithCause(err)
		}
		if count > 0 {
			return pkgerrors.Conflict().WithCode(check.code).WithMessage(check.message)
		}
	}
	return nil
}

func normalizeOptional(value string) string {
	if value == "" {
		return ""
	}
	return value
}

func isAdminStatusValid(status int) bool {
	switch status {
	case model.ConsoleAdminStatusActive, model.ConsoleAdminStatusDisabled, model.ConsoleAdminStatusLocked:
		return true
	default:
		return false
	}
}

func (s *AdminService) syncAdminRoleBinding(adminID, roleID string) error {
	if s.enforcer == nil || strings.TrimSpace(adminID) == "" {
		return nil
	}
	if _, err := s.enforcer.RemoveFilteredGroupingPolicy(0, adminID); err != nil {
		return pkgerrors.Internal().WithCause(err)
	}
	if strings.TrimSpace(roleID) == "" {
		return nil
	}
	if _, err := s.enforcer.AddGroupingPolicy(adminID, roleID); err != nil {
		return pkgerrors.Internal().WithCause(err)
	}
	return nil
}

func (s *AdminService) execute(ctx context.Context, fn func(context.Context) error) error {
	if s.txManager == nil {
		return fn(ctx)
	}
	return s.txManager.Execute(ctx, fn)
}
