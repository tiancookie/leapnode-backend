// Package services 承载业务逻辑层。
package services

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
)

// TokenService 令牌 (API Key) 管理服务。
//
// 所有操作均以 userID 归属校验为前提, 防止越权操作他人 token。
// 复用 New-API 原生 tokens 表 (models.Token), 只做 CRUD, 不改列/不 AutoMigrate。
type TokenService struct {
	db            *gorm.DB
	newAPIClient  *NewAPIClient
}

// NewTokenService 创建 TokenService。
func NewTokenService(db *gorm.DB, newAPIClient *NewAPIClient) *TokenService {
	return &TokenService{
		db:           db,
		newAPIClient: newAPIClient,
	}
}

// 业务错误 (由 controller 映射为 message + 状态码)。
var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenNameReq  = errors.New("token name required")
	ErrKeyGenFailed  = errors.New("failed to generate token key")
)

// keyChars 与 New-API common.GenerateRandomCharsKey 一致 (62 进制字母表)。
const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// generateKey 生成 48 位随机 key (crypto/rand)。
//
// 注意: 存入 tokens.key 列的是纯 48 位随机串, 不含 "sk-" 前缀。
// 前端 Tokens.jsx 展示与复制时统一拼 "sk-" + token.key (L1248/1253),
// 故此处若再带前缀会渲染成 "sk-sk-..."。与 New-API 原生存储约定一致。
func generateKey() (string, error) {
	b := make([]byte, 48)
	maxI := big.NewInt(int64(len(keyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return "", err
		}
		b[i] = keyChars[n.Int64()]
	}
	return string(b), nil
}

// generateUniqueKey 生成一个在 tokens 表中唯一的 key (含软删除)。
func (s *TokenService) generateUniqueKey() (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		key, err := generateKey()
		if err != nil {
			return "", ErrKeyGenFailed
		}
		var count int64
		// Unscoped: 连软删除的 key 也算冲突 (key 列有 uniqueIndex)。
		if err := s.db.Unscoped().Model(&models.Token{}).Where("key = ?", key).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return key, nil
		}
	}
	return "", ErrKeyGenFailed
}

// CreateTokenInput 创建令牌请求 (对齐 Tokens.jsx buildTokenControlPayload + handleCreate)。
//
// 前端提交字段 (L142-151, L434):
//   name / expired_time / unlimited_quota / remain_quota / allow_ips / model_limits
// 其余前端字段 (type/key_group_id/subrouter_*/include_* 等) New-API tokens 表无对应列,
// 属分站路由/共享订阅模块, 本接口忽略 (见 TODO)。
type CreateTokenInput struct {
	Name               string  `json:"name"`
	ExpiredTime        *int64  `json:"expired_time"`         // -1 表永不过期; 前端总是传
	RemainQuota        *int    `json:"remain_quota"`         // unlimited_quota=true 时忽略
	UnlimitedQuota     *bool   `json:"unlimited_quota"`      // 默认 true (emptyControlForm)
	ModelLimits        *string `json:"model_limits"`         // 逗号分隔字符串
	ModelLimitsEnabled *bool   `json:"model_limits_enabled"` // 前端未直接传, 由 model_limits 非空推断
	Group              *string `json:"group"`
	AllowIps           *string `json:"allow_ips"`
}

// UpdateTokenInput 更新令牌请求 (对齐 Tokens.jsx handleToggle L506 + handleEditSave)。
//
// 所有字段均为指针, nil 表示不更新 (支持 handleToggle 只传 status 的场景)。
type UpdateTokenInput struct {
	Name               *string `json:"name"`
	Status             *int    `json:"status"`
	ExpiredTime        *int64  `json:"expired_time"`
	RemainQuota        *int    `json:"remain_quota"`
	UnlimitedQuota     *bool   `json:"unlimited_quota"`
	ModelLimits        *string `json:"model_limits"`
	ModelLimitsEnabled *bool   `json:"model_limits_enabled"`
	Group              *string `json:"group"`
	AllowIps           *string `json:"allow_ips"`
}

// ListTokens 返回当前用户的令牌列表 (排除软删除, 按 id 倒序)。
func (s *TokenService) ListTokens(userID int) ([]models.Token, error) {
	var tokens []models.Token
	err := s.db.Where("user_id = ?", userID).Order("id desc").Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		tokens = []models.Token{}
	}
	return tokens, nil
}

// GetToken 查询归属于 userID 的单个令牌 (排除软删除)。
// 不存在或不属于该用户时返回 ErrTokenNotFound。
func (s *TokenService) GetToken(userID, tokenID int) (*models.Token, error) {
	var token models.Token
	err := s.db.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}

// CreateToken 为 userID 创建一个新令牌。
func (s *TokenService) CreateToken(userID int, in CreateTokenInput) (*models.Token, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrTokenNameReq
	}

	key, err := s.generateUniqueKey()
	if err != nil {
		return nil, err
	}

	// unlimited_quota 默认 true (对齐 emptyControlForm)。
	unlimited := true
	if in.UnlimitedQuota != nil {
		unlimited = *in.UnlimitedQuota
	}

	// expired_time 默认 -1 (永不过期)。
	expired := int64(-1)
	if in.ExpiredTime != nil {
		expired = *in.ExpiredTime
	}

	// remain_quota: unlimited 时强制 0。
	remain := 0
	if !unlimited && in.RemainQuota != nil {
		remain = *in.RemainQuota
	}

	modelLimits := ""
	if in.ModelLimits != nil {
		modelLimits = *in.ModelLimits
	}
	// model_limits_enabled: 前端不直接传, 有 model_limits 内容即视为启用。
	modelLimitsEnabled := strings.TrimSpace(modelLimits) != ""
	if in.ModelLimitsEnabled != nil {
		modelLimitsEnabled = *in.ModelLimitsEnabled
	}

	group := ""
	if in.Group != nil {
		group = *in.Group
	}

	token := models.Token{
		UserId:             userID,
		Key:                key,
		Status:             1,
		Name:               name,
		CreatedTime:        time.Now().Unix(),
		AccessedTime:       time.Now().Unix(),
		ExpiredTime:        expired,
		RemainQuota:        remain,
		UnlimitedQuota:     unlimited,
		ModelLimits:        modelLimits,
		ModelLimitsEnabled: modelLimitsEnabled,
		AllowIps:           in.AllowIps,
		UsedQuota:          0,
		Group:              group,
	}
	if err := s.db.Create(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// UpdateToken 更新归属于 userID 的令牌 (先校验归属)。
// 只更新请求体中显式提供 (非 nil) 的字段。
//
// ✅ 数据一致性 (ADR-001): 调用 New-API PUT /api/token/ 更新，让 New-API 自己失效缓存。
func (s *TokenService) UpdateToken(userID, tokenID int, in UpdateTokenInput) (*models.Token, error) {
	// 先校验归属: 不存在或非本人的 token 返回 ErrTokenNotFound。
	if _, err := s.GetToken(userID, tokenID); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, ErrTokenNameReq
		}
		updates["name"] = name
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}
	if in.ExpiredTime != nil {
		updates["expired_time"] = *in.ExpiredTime
	}
	if in.UnlimitedQuota != nil {
		updates["unlimited_quota"] = *in.UnlimitedQuota
		// unlimited 打开时清零 remain_quota (对齐前端语义)。
		if *in.UnlimitedQuota {
			updates["remain_quota"] = 0
		}
	}
	if in.RemainQuota != nil {
		// 仅在没被 unlimited=true 覆盖时才写入。
		if _, cleared := updates["remain_quota"]; !cleared {
			updates["remain_quota"] = *in.RemainQuota
		}
	}
	if in.ModelLimits != nil {
		updates["model_limits"] = *in.ModelLimits
		// 未显式传 enabled 时, 按内容非空推断。
		if in.ModelLimitsEnabled == nil {
			updates["model_limits_enabled"] = strings.TrimSpace(*in.ModelLimits) != ""
		}
	}
	if in.ModelLimitsEnabled != nil {
		updates["model_limits_enabled"] = *in.ModelLimitsEnabled
	}
	if in.Group != nil {
		updates["group"] = *in.Group
	}
	if in.AllowIps != nil {
		updates["allow_ips"] = *in.AllowIps
	}

	if len(updates) > 0 {
		// 调用 New-API 更新（自动失效缓存）
		if s.newAPIClient != nil {
			if err := s.newAPIClient.UpdateToken(tokenID, updates); err != nil {
				return nil, err
			}
		} else {
			// 降级：直接写库（仅测试环境，生产必须走 New-API）
			if err := s.db.Model(&models.Token{}).
				Where("id = ? AND user_id = ?", tokenID, userID).
				Updates(updates).Error; err != nil {
				return nil, err
			}
		}
	}

	// 返回更新后的最新状态。
	return s.GetToken(userID, tokenID)
}

// DeleteToken 软删除归属于 userID 的令牌 (先校验归属)。
//
// ✅ 数据一致性 (ADR-001): 调用 New-API DELETE /api/token/:id，让 New-API 自己失效缓存。
func (s *TokenService) DeleteToken(userID, tokenID int) error {
	token, err := s.GetToken(userID, tokenID)
	if err != nil {
		return err
	}
	
	// 调用 New-API 删除（自动失效缓存）
	if s.newAPIClient != nil {
		return s.newAPIClient.DeleteToken(token.Id)
	}
	
	// 降级：直接写库（仅测试环境，生产必须走 New-API）
	return s.db.Where("id = ? AND user_id = ?", token.Id, userID).Delete(&models.Token{}).Error
}

// TokenSupportedModelsResponse 令牌可用模型响应 (对齐 Tokens.jsx handleToggleSupportedModels L624-634)。
type TokenSupportedModelsResponse struct {
	Models                 []string `json:"models"`
	Count                  int      `json:"count"`
	ProviderNames          []string `json:"provider_names"`
	RestrictedByProviders  bool     `json:"restricted_by_providers"`
	RestrictedByModels     bool     `json:"restricted_by_models"`
}

// GetTokenSupportedModels 返回令牌可用的模型列表。
//
// 逻辑:
//   - 若 token 启用了 model_limits (model_limits_enabled 且 model_limits 非空),
//     则可用模型 = model_limits 解析结果, restricted_by_models=true。
//   - 否则可用模型 = 站点全部可用模型 (复用 SiteService.GetSiteModels), 不受限。
//
// TODO(parent): provider_names / restricted_by_providers 依赖 subrouter 路由偏好
// (subrouter_providers 等前端字段), New-API tokens 表无对应列, 分站路由模块补齐后回填。
// 当前 provider_names 恒空, restricted_by_providers 恒 false。
func (s *TokenService) GetTokenSupportedModels(userID, tokenID int, siteService *SiteService) (*TokenSupportedModelsResponse, error) {
	token, err := s.GetToken(userID, tokenID)
	if err != nil {
		return nil, err
	}

	resp := &TokenSupportedModelsResponse{
		Models:                []string{},
		Count:                 0,
		ProviderNames:         []string{},
		RestrictedByProviders: false,
		RestrictedByModels:    false,
	}

	limited := token.ModelLimitsEnabled && strings.TrimSpace(token.ModelLimits) != ""
	if limited {
		// model_limits 是逗号分隔的模型名。
		for _, m := range strings.Split(token.ModelLimits, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				resp.Models = append(resp.Models, m)
			}
		}
		resp.RestrictedByModels = true
		resp.Count = len(resp.Models)
		return resp, nil
	}

	// 不受限: 返回站点全部可用模型。
	siteModels, err := siteService.GetSiteModels()
	if err != nil {
		return resp, nil // 出错则返回空, 不阻断前端
	}
	for _, m := range siteModels {
		resp.Models = append(resp.Models, m.ModelName)
	}
	resp.Count = len(resp.Models)
	return resp, nil
}
