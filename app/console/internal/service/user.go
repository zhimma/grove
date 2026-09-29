package service

import (
	"context"
	"net/mail"
	"strings"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

type UserService struct {
	dbs   database.Connections
	pages pagination.Policy
}

type ListUsersInput struct {
	pagination.Request
	Keyword     string
	OrderBy     []string
	Status      *int
	CreatedFrom string
	CreatedTo   string
}

type ListUsersOutput struct {
	List []model.User
	Meta pagination.Meta
}

type GetUserInput struct {
	UserID string
}

type CreateUserInput struct {
	Name   string
	Email  string
	Phone  string
	Avatar string
	Remark string
	Status *int
}

type UpdateUserInput struct {
	UserID string
	Name   *string
	Email  *string
	Phone  *string
	Avatar *string
	Remark *string
	Status *int
}

type UpdateUserStatusInput struct {
	UserID string
	Status int
}

type DeleteUserInput struct {
	UserID string
}

func NewUserService(dbs database.Connections, pages pagination.Policy) *UserService {
	return &UserService{dbs: dbs, pages: pages}
}

func (s *UserService) ListUsers(ctx context.Context, in ListUsersInput) (*ListUsersOutput, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}

	page := s.pages.Resolve(in.Request)

	query := db.WithContext(ctx).Model(&model.User{})
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	if in.Status != nil {
		if !isUserStatusValid(*in.Status) {
			return nil, invalidUserParams("用户状态不合法")
		}
		query = query.Where("status = ?", *in.Status)
	}

	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithMessage("时间范围格式不正确")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}

	if len(in.OrderBy) == 0 {
		query = query.Order("created_at DESC")
	} else {
		for _, item := range in.OrderBy {
			field, direction := parseOrderBy(item)
			switch field {
			case "created_at", "updated_at", "name", "email", "status":
				query = query.Order(field + " " + direction)
			}
		}
	}

	query = page.Apply(query)

	list := make([]model.User, 0)
	if err := query.Find(&list).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &ListUsersOutput{List: list, Meta: pagination.NewMeta(total, page)}, nil
}

func (s *UserService) GetUser(ctx context.Context, in GetUserInput) (*model.User, error) {
	return s.loadUser(ctx, in.UserID)
}

func (s *UserService) CreateUser(ctx context.Context, in CreateUserInput) (*model.User, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}

	name, email, phone, avatar, remark, err := normalizeUserFields(in.Name, in.Email, in.Phone, in.Avatar, in.Remark)
	if err != nil {
		return nil, err
	}
	status := model.UserStatusActive
	if in.Status != nil {
		status = *in.Status
		if !isUserStatusValid(status) {
			return nil, invalidUserParams("用户状态不合法")
		}
	}
	if err := s.ensureEmailAvailable(ctx, "", email); err != nil {
		return nil, err
	}

	user := &model.User{
		Name:   name,
		Email:  email,
		Phone:  phone,
		Avatar: avatar,
		Status: status,
		Remark: remark,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.loadUser(ctx, user.ID)
}

func (s *UserService) UpdateUser(ctx context.Context, in UpdateUserInput) (*model.User, error) {
	user, err := s.loadUser(ctx, in.UserID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]any)
	newEmail := user.Email
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, invalidUserParams("用户名称不能为空")
		}
		if len(name) > 120 {
			return nil, invalidUserParams("用户名称过长")
		}
		updates["name"] = name
	}
	if in.Email != nil {
		newEmail, err = normalizeUserEmail(*in.Email)
		if err != nil {
			return nil, err
		}
		updates["email"] = newEmail
	}
	if in.Phone != nil {
		phone := strings.TrimSpace(*in.Phone)
		if len(phone) > 32 {
			return nil, invalidUserParams("手机号过长")
		}
		updates["phone"] = phone
	}
	if in.Avatar != nil {
		avatar := strings.TrimSpace(*in.Avatar)
		if len(avatar) > 255 {
			return nil, invalidUserParams("头像地址过长")
		}
		updates["avatar"] = avatar
	}
	if in.Remark != nil {
		remark := strings.TrimSpace(*in.Remark)
		if len(remark) > 500 {
			return nil, invalidUserParams("备注过长")
		}
		updates["remark"] = remark
	}
	if in.Status != nil {
		if !isUserStatusValid(*in.Status) {
			return nil, invalidUserParams("用户状态不合法")
		}
		updates["status"] = *in.Status
	}

	if err := s.ensureEmailAvailable(ctx, in.UserID, newEmail); err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.dbs.Default().WithContext(ctx).Model(&model.User{}).
			Where("id = ?", in.UserID).Updates(updates).Error; err != nil {
			return nil, errx.Internal().WithCause(err)
		}
	}
	return s.loadUser(ctx, in.UserID)
}

func (s *UserService) UpdateUserStatus(ctx context.Context, in UpdateUserStatusInput) (*model.User, error) {
	if !isUserStatusValid(in.Status) {
		return nil, invalidUserParams("用户状态不合法")
	}
	if _, err := s.loadUser(ctx, in.UserID); err != nil {
		return nil, err
	}
	if err := s.dbs.Default().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", strings.TrimSpace(in.UserID)).Update("status", in.Status).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.loadUser(ctx, in.UserID)
}

func (s *UserService) DeleteUser(ctx context.Context, in DeleteUserInput) error {
	if _, err := s.loadUser(ctx, in.UserID); err != nil {
		return err
	}
	if err := s.dbs.Default().WithContext(ctx).Delete(&model.User{}, "id = ?", strings.TrimSpace(in.UserID)).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *UserService) loadUser(ctx context.Context, userID string) (*model.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, invalidUserParams("用户ID不能为空")
	}
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	var user model.User
	if err := db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.NotFound().WithMessage("用户不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &user, nil
}

func (s *UserService) ensureEmailAvailable(ctx context.Context, userID, email string) error {
	db, err := s.defaultDB()
	if err != nil {
		return err
	}
	var count int64
	query := db.WithContext(ctx).Model(&model.User{}).Where("email = ?", email)
	if userID = strings.TrimSpace(userID); userID != "" {
		query = query.Where("id <> ?", userID)
	}
	if err := query.Count(&count).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	if count > 0 {
		return errx.Conflict().WithCode("user_email_exists").WithMessage("邮箱已被使用")
	}
	return nil
}

func (s *UserService) defaultDB() (*gorm.DB, error) {
	if s == nil || s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default(), nil
}

func normalizeUserFields(name, email, phone, avatar, remark string) (string, string, string, string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", "", "", invalidUserParams("用户名称不能为空")
	}
	if len(name) > 120 {
		return "", "", "", "", "", invalidUserParams("用户名称过长")
	}
	normalizedEmail, err := normalizeUserEmail(email)
	if err != nil {
		return "", "", "", "", "", err
	}
	phone = strings.TrimSpace(phone)
	if len(phone) > 32 {
		return "", "", "", "", "", invalidUserParams("手机号过长")
	}
	avatar = strings.TrimSpace(avatar)
	if len(avatar) > 255 {
		return "", "", "", "", "", invalidUserParams("头像地址过长")
	}
	remark = strings.TrimSpace(remark)
	if len(remark) > 500 {
		return "", "", "", "", "", invalidUserParams("备注过长")
	}
	return name, normalizedEmail, phone, avatar, remark, nil
}

func normalizeUserEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if len(email) == 0 || len(email) > 160 {
		return "", invalidUserParams("邮箱格式不正确")
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || !strings.Contains(email, "@") {
		return "", invalidUserParams("邮箱格式不正确")
	}
	return email, nil
}

func invalidUserParams(message string) error {
	return errx.InvalidParams().WithHTTPStatus(422).WithMessage(message)
}

func isUserStatusValid(status int) bool {
	return status == model.UserStatusDisabled || status == model.UserStatusActive
}
