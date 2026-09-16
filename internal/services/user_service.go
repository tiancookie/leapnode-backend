// Package services 承载业务逻辑层。
package services

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/tiancookie/leapnode-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService 用户认证与账户服务。
type UserService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewUserService 创建 UserService。
func NewUserService(db *gorm.DB, newAPIClient *NewAPIClient) *UserService {
	return &UserService{
		db:           db,
		newAPIClient: newAPIClient,
	}
}

// 业务错误 (由 controller 映射为 message + 状态码)。
var (
	ErrUsernameTaken     = errors.New("username already exists")
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrUserDisabled      = errors.New("account disabled")
	ErrUserNotFound      = errors.New("user not found")
	ErrWrongPassword     = errors.New("current password is incorrect")
	ErrValidation        = errors.New("validation failed")
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// UserResponse 是返回给前端的用户对象 (严格对齐 SubRouter 消费字段)。
//
// 字段来源:
//   - AuthContext.jsx / Account.jsx / Dashboard.jsx / SubDistributor.jsx
//   - api-contract.md L130-157
//
// quota 采用 Q=500000 体系 (由 New-API 原生存储, 原样透传)。
type UserResponse struct {
	ID                   int     `json:"id"`
	Username             string  `json:"username"`
	DisplayName          string  `json:"display_name"`
	Email                string  `json:"email"`
	Role                 int     `json:"role"`
	Status               int     `json:"status"`
	Quota                int64   `json:"quota"`
	UsedQuota            int64   `json:"used_quota"`
	RequestCount         int     `json:"request_count"`
	AffCode              string  `json:"aff_code"`
	AffCount             int     `json:"aff_count"`
	AffQuota             int64   `json:"aff_quota"`
	AffHistoryQuota      int64   `json:"aff_history_quota"`
	CreatedTime          int64   `json:"created_time"`
	Group                string  `json:"group"`
	Language             string  `json:"language,omitempty"`
	UserLevel            int     `json:"user_level"`
	AnnouncementEmailEnabled bool `json:"announcement_email_enabled"`

	// TODO(parent): 以下字段前端会消费但 New-API 原生表无对应列,
	// 需分站/KOL 模块补全后回填 (当前返回零值占位, 不影响登录/账户页渲染):
	//   has_distributor / distributor_slug / distributor_name /
	//   commission_rate / default_commission_rate /
	//   package_used_quota / invite_milestone_enabled / invite_milestone_rules
}

// toResponse 把 models.User 转为对外响应对象。
func toResponse(u *models.User) *UserResponse {
	return &UserResponse{
		ID:              u.ID,
		Username:        u.Username,
		DisplayName:     u.DisplayName,
		Email:           u.Email,
		Role:            u.Role,
		Status:          u.Status,
		Quota:           u.Quota,
		UsedQuota:       u.UsedQuota,
		RequestCount:    u.RequestCount,
		AffCode:         u.AffCode,
		AffCount:        u.AffCount,
		AffQuota:        u.AffQuota,
		AffHistoryQuota: u.AffHistoryQuota,
		CreatedTime:     u.CreatedTime,
		Group:           u.Group,
		Language:        u.Language,
		UserLevel:       u.UserLevel,
		// New-API 无 announcement_email_enabled 列, 默认视为开启 (前端以 !== false 判断)
		AnnouncementEmailEnabled: true,
	}
}

// RegisterInput 注册参数 (对齐 Register.jsx 提交字段)。
// 邀请码兼容两个字段名: aff_code (前端表单) 与 ref (邀请 URL /register?ref=CODE)。
type RegisterInput struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	AffCode          string `json:"aff_code"`
	Ref              string `json:"ref"` // 批10：邀请 URL 的 ref 参数（aff_code 别名）
}

// Register 创建新用户。
func (s *UserService) Register(in RegisterInput) (*models.User, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(strings.ToLower(in.Email))

	// 邀请码兼容: 优先 aff_code，其次 ref（邀请链接参数）。
	if in.AffCode == "" && in.Ref != "" {
		in.AffCode = in.Ref
	}

	// --- 输入校验 (注册接口对外无认证, 必须严格校验) ---
	if !usernameRe.MatchString(username) {
		return nil, ErrValidation
	}
	// 密码强度: 与 Register.jsx 前端校验一致 (8-20)
	if len(in.Password) < 8 || len(in.Password) > 20 {
		return nil, ErrValidation
	}
	if email != "" && !emailRe.MatchString(email) {
		return nil, ErrValidation
	}

	// TODO(parent): 邮箱验证码校验。当 site.registration_email_verification_enabled
	// 开启时, 前端会强制要求 verification_code。发信/校验依赖外部邮件服务
	// (SMTP 密钥等), 尚未接入。此处不校验 in.VerificationCode, 留待邮件服务模块补齐。

	// --- 防重复注册 ---
	var count int64
	if err := s.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrUsernameTaken
	}
	if email != "" {
		if err := s.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, ErrEmailTaken
		}
	}

	// --- aff_code 归因（批7补完 + 批10补充注册返佣）---
	// 1. 查找邀请人
	var inviter *models.User
	var distributorID int = 0 // 默认总站用户
	if in.AffCode != "" {
		var inv models.User
		if err := s.db.Where("aff_code = ?", in.AffCode).First(&inv).Error; err == nil {
			inviter = &inv
			// 如果邀请人是分站主（user_level=3），新用户归属该分站。
			// distributor_id 存 distributor_sites.id（站点主键），与分站后台
			// getDistributorID 返回的口径一致，绝不能存 inviter.ID（user.ID）。
			if inv.UserLevel == 3 {
				var site models.DistributorSite
				if err := s.db.Where("owner_id = ?", inv.ID).First(&site).Error; err == nil {
					distributorID = int(site.ID)
				}
			}
		}
		// 推广码无效不报错（允许用户随便填），只是不归因
	}

	// aff_code 列有 uniqueIndex, 每个用户必须有唯一邀请码 (New-API 语义)。
	// 空串会在第 2 个用户注册时撞唯一约束, 故此处生成唯一随机码。
	affCode, err := s.generateUniqueAffCode()
	if err != nil {
		return nil, err
	}

	// --- 通过 New-API 创建用户 (写操作) ---
	userID, err := s.newAPIClient.CreateUser(username, in.Password, email)
	if err != nil {
		return nil, err
	}

	// --- 读回本地库（New-API AutoMigrate 已写入）---
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	// --- 补写 LeapNode 扩展字段（aff_code/user_level/distributor_id）---
	user.AffCode = affCode
	user.UserLevel = 0
	if inviter != nil {
		user.InviterID = inviter.ID
	}
	if distributorID > 0 {
		user.DistributorID = &distributorID
	}
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	// --- 批10补充：触发注册返佣 ---
	// user 已 Save（数据已落库），此处同步发放注册奖励。
	// 防重复由 GrantRegisterReward 内部保证；失败仅记日志，不回滚注册。
	if inviter != nil {
		affService := NewAffService(s.db, s.newAPIClient)
		if err := affService.GrantRegisterReward(inviter.ID, user.ID); err != nil {
			fmt.Printf("Failed to grant register reward: inviter=%d, invitee=%d, err=%v\n", inviter.ID, user.ID, err)
		}
	}

	return &user, nil
}

// Login 校验凭证并返回用户。
func (s *UserService) Login(username, password string) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredential
	}
	// 密码校验委托给 New-API（密码由 New-API 管理，哈希算法一致性由其保证）。
	userID, err := s.newAPIClient.VerifyLogin(username, password)
	if err != nil {
		return nil, ErrInvalidCredential
	}

	// 校验通过后从本地库读取完整用户信息（含 LeapNode 扩展字段 user_level/aff_code 等）。
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredential
		}
		return nil, err
	}
	if user.Status != 1 {
		return nil, ErrUserDisabled
	}
	return &user, nil
}

// GetByID 按 ID 查询用户。
func (s *UserService) GetByID(id int) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// ChangePassword 修改密码 (需校验原密码)。
func (s *UserService) ChangePassword(userID int, originalPassword, newPassword string) error {
	if len(newPassword) < 8 || len(newPassword) > 20 {
		return ErrValidation
	}
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(originalPassword)); err != nil {
		return ErrWrongPassword
	}
	// 通过 New-API 修改密码
	return s.newAPIClient.UpdateUserPassword(userID, newPassword)
}

// UpdateLanguage 更新用户界面语言。
func (s *UserService) UpdateLanguage(userID int, language string) error {
	language = strings.TrimSpace(language)
	if language == "" || len(language) > 16 {
		return ErrValidation
	}
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("language", language).Error
}

// ToResponse 暴露给 controller 的转换函数。
func (s *UserService) ToResponse(u *models.User) *UserResponse {
	return toResponse(u)
}

// affCodeChars 邀请码字符集 (数字+大小写字母)。
const affCodeChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// generateUniqueAffCode 生成一个在 users 表中唯一的 8 位邀请码。
// aff_code 列有 uniqueIndex, 必须保证不冲突。
func (s *UserService) generateUniqueAffCode() (string, error) {
	maxI := big.NewInt(int64(len(affCodeChars)))
	for attempt := 0; attempt < 8; attempt++ {
		b := make([]byte, 8)
		for i := range b {
			n, err := rand.Int(rand.Reader, maxI)
			if err != nil {
				return "", err
			}
			b[i] = affCodeChars[n.Int64()]
		}
		code := string(b)
		var count int64
		if err := s.db.Model(&models.User{}).Where("aff_code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", errors.New("failed to generate unique aff_code")
}
