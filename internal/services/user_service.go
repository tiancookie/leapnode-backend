// Package services 承载业务逻辑层。
package services

import (
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService 用户认证与账户服务。
type UserService struct {
	db *gorm.DB
}

// NewUserService 创建 UserService。
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
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
type RegisterInput struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	AffCode          string `json:"aff_code"`
}

// Register 创建新用户。
func (s *UserService) Register(in RegisterInput) (*models.User, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(strings.ToLower(in.Email))

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

	// --- bcrypt 哈希 ---
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// TODO(parent): aff_code 归因。in.AffCode 为邀请人的 aff_code,
	// 需查邀请人并写 inviter_id + 发放注册奖励 (referral_config)。
	// 涉及资金池结算, 留待返佣模块统一实现。当前仅忽略。

	// aff_code 列有 uniqueIndex, 每个用户必须有唯一邀请码 (New-API 语义)。
	// 空串会在第 2 个用户注册时撞唯一约束, 故此处生成唯一随机码。
	affCode, err := s.generateUniqueAffCode()
	if err != nil {
		return nil, err
	}

	user := models.User{
		Username:     username,
		Password:     string(hash),
		DisplayName:  username,
		Email:        email,
		Role:         1, // New-API: 1=普通用户
		Status:       1, // 1=正常
		Quota:        0,
		AffCode:      affCode,
		Group:        "default",
		UserLevel:    0,
		CreatedTime:  time.Now().Unix(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Login 校验凭证并返回用户。
func (s *UserService) Login(username, password string) (*models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, ErrInvalidCredential
	}
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredential
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredential
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
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("password", string(hash)).Error
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
