// Package services 实现返佣业务逻辑。
package services

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
)

// AffService 返佣服务
type AffService struct {
	db           *gorm.DB
	newAPIClient *NewAPIClient
}

// NewAffService 创建返佣服务实例
func NewAffService(db *gorm.DB, newAPIClient *NewAPIClient) *AffService {
	return &AffService{
		db:           db,
		newAPIClient: newAPIClient,
	}
}

// GenerateAffCode 生成6位邀请码（大写字母+数字）
func (s *AffService) GenerateAffCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLength = 6
	maxRetries := 20

	for i := 0; i < maxRetries; i++ {
		code := make([]byte, codeLength)
		for j := 0; j < codeLength; j++ {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			code[j] = charset[n.Int64()]
		}

		codeStr := string(code)
		// 检查是否已存在
		var count int64
		if err := s.db.Model(&models.User{}).Where("aff_code = ?", codeStr).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return codeStr, nil
		}
	}
	return "", errors.New("failed to generate unique aff_code after retries")
}

// GetAffCode 获取用户的邀请码和URL
func (s *AffService) GetAffCode(userID int) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// 如果没有邀请码，生成一个
	if user.AffCode == "" {
		code, err := s.GenerateAffCode()
		if err != nil {
			return nil, err
		}
		if err := s.db.Model(&user).Update("aff_code", code).Error; err != nil {
			return nil, err
		}
		user.AffCode = code
	}

	baseURL := "https://leapnode-backend-production.up.railway.app" // TODO: 从配置读取
	return map[string]interface{}{
		"code": user.AffCode,
		"url":  fmt.Sprintf("%s/register?ref=%s", baseURL, user.AffCode),
	}, nil
}

// GetAffEarnings 查询返佣收益
func (s *AffService) GetAffEarnings(userID int) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// 查询收益历史
	var histories []models.AffHistory
	if err := s.db.Where("user_id = ? AND status = 1", userID).
		Order("created_at DESC").
		Limit(50).
		Find(&histories).Error; err != nil {
		return nil, err
	}

	// 计算总收益和已提现
	var totalEarnings int64
	var totalWithdrawn int64
	
	for _, h := range histories {
		if h.EventType != "withdraw" {
			totalEarnings += h.Quota
		} else {
			totalWithdrawn += h.Quota
		}
	}

	// 构建收益历史列表
	earningsHistory := make([]map[string]interface{}, 0)
	for _, h := range histories {
		// 查询被邀请人信息
		var invitee models.User
		inviteeName := ""
		if h.InviteeID > 0 {
			if err := s.db.First(&invitee, h.InviteeID).Error; err == nil {
				inviteeName = invitee.Username
			}
		}

		earningsHistory = append(earningsHistory, map[string]interface{}{
			"date":    h.CreatedAt.Format("2006-01-02 15:04:05"),
			"type":    h.EventType,
			"amount":  utils.QuotaToDollars(h.Quota),
			"invitee": inviteeName,
		})
	}

	return map[string]interface{}{
		"total_earnings":    utils.QuotaToDollars(totalEarnings),
		"available_balance": utils.QuotaToDollars(user.AffQuota),
		"total_withdrawn":   utils.QuotaToDollars(totalWithdrawn),
		"earnings_history":  earningsHistory,
	}, nil
}

// TransferAffQuota 收益转余额（aff_quota → quota）
func (s *AffService) TransferAffQuota(userID int, amountUSD float64) (map[string]interface{}, error) {
	if amountUSD <= 0 {
		return nil, errors.New("amount must be positive")
	}

	quotaAmount := utils.DollarsToQuota(amountUSD)

	// 步骤1: 事务内扣除 aff_quota（LeapNode 扩展列，New-API 不管）+ 写历史
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		if user.AffQuota < quotaAmount {
			return errors.New("insufficient aff_quota balance")
		}

		// 仅扣除 aff_quota（LeapNode 扩展列）
		if err := tx.Model(&user).Update("aff_quota", gorm.Expr("aff_quota - ?", quotaAmount)).Error; err != nil {
			return err
		}

		// 记录历史
		history := models.AffHistory{
			UserID:    userID,
			InviteeID: userID, // 自己转给自己
			Quota:     quotaAmount,
			EventType: "transfer",
			Status:    1, // completed
		}
		return tx.Create(&history).Error
	})

	if err != nil {
		return nil, err
	}

	// 步骤2: 通过 New-API 增加 quota（New-API 缓存的热点字段，必须走 API）
	// 注意: 若此步失败，aff_quota 已扣但 quota 未加，需补偿（记录到失败队列）。
	// TODO(补偿): 增加失败重试/对账机制。当前失败会返回错误，aff_quota 已扣需人工核对。
	if err := s.newAPIClient.IncreaseQuota(userID, int(quotaAmount)); err != nil {
		return nil, fmt.Errorf("aff_quota deducted but failed to increase quota via New-API (needs reconciliation): %w", err)
	}

	// 查询最新余额
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"new_aff_balance":   utils.QuotaToDollars(user.AffQuota),
		"new_quota_balance": utils.QuotaToDollars(user.Quota),
	}, nil
}

// RequestWithdraw 提现申请
func (s *AffService) RequestWithdraw(userID int, amountUSD float64, method, accountInfo string) (map[string]interface{}, error) {
	if amountUSD <= 0 {
		return nil, errors.New("amount must be positive")
	}

	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	quotaAmount := utils.DollarsToQuota(amountUSD)
	if user.AffQuota < quotaAmount {
		return nil, errors.New("insufficient aff_quota balance")
	}

	// 创建提现申请（不立即扣款，审核通过后再扣）
	request := models.WithdrawalRequest{
		UserID:        userID,
		Amount:        amountUSD,
		PaymentMethod: method,
		AccountInfo:   accountInfo,
		Status:        0, // pending
	}

	if err := s.db.Create(&request).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"request_id":     request.ID,
		"status":         "pending",
		"estimated_days": 3,
	}, nil
}

// GetInvitees 查询邀请列表（一级+二级）
func (s *AffService) GetInvitees(userID int) (map[string]interface{}, error) {
	// 一级邀请人（直接邀请）
	var level1Users []models.User
	if err := s.db.Where("inviter_id = ?", userID).Find(&level1Users).Error; err != nil {
		return nil, err
	}

	level1List := make([]map[string]interface{}, 0)
	level1IDs := make([]int, 0)
	
	for _, u := range level1Users {
		// 查询首充时间和总消费
		var firstTopupAt string
		var topup models.TopupOrder
		if err := s.db.Where("user_id = ?", u.ID).Order("created_at ASC").First(&topup).Error; err == nil {
			firstTopupAt = time.Unix(0, topup.CreatedAt*int64(time.Millisecond)).Format("2006-01-02 15:04:05")
		}

		level1List = append(level1List, map[string]interface{}{
			"username":          u.Username,
			"registered_at":     time.Unix(u.CreatedTime, 0).Format("2006-01-02 15:04:05"),
			"first_topup_at":    firstTopupAt,
			"total_consumption": utils.QuotaToDollars(u.UsedQuota),
		})
		level1IDs = append(level1IDs, u.ID)
	}

	// 二级邀请人（一级邀请人的邀请人）
	level2List := make([]map[string]interface{}, 0)
	if len(level1IDs) > 0 {
		var level2Users []models.User
		if err := s.db.Where("inviter_id IN ?", level1IDs).Find(&level2Users).Error; err == nil {
			for _, u := range level2Users {
				var firstTopupAt string
				var topup models.TopupOrder
				if err := s.db.Where("user_id = ?", u.ID).Order("created_at ASC").First(&topup).Error; err == nil {
					firstTopupAt = time.Unix(0, topup.CreatedAt*int64(time.Millisecond)).Format("2006-01-02 15:04:05")
				}

				level2List = append(level2List, map[string]interface{}{
					"username":          u.Username,
					"registered_at":     time.Unix(u.CreatedTime, 0).Format("2006-01-02 15:04:05"),
					"first_topup_at":    firstTopupAt,
					"total_consumption": utils.QuotaToDollars(u.UsedQuota),
				})
			}
		}
	}

	return map[string]interface{}{
		"level1": level1List,
		"level2": level2List,
	}, nil
}

// ApplyKOL 商家/分站申请
func (s *AffService) ApplyKOL(userID int, applyType, reason, contactInfo string) (map[string]interface{}, error) {
	// 检查是否已有pending申请
	var count int64
	if err := s.db.Model(&models.KolApplication{}).
		Where("user_id = ? AND status = 0", userID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("you already have a pending application")
	}

	// 类型验证
	if applyType != "merchant" && applyType != "distributor" {
		return nil, errors.New("invalid apply_type, must be merchant or distributor")
	}

	// 创建申请
	app := models.KolApplication{
		UserID:      userID,
		ApplyType:   applyType,
		ContactInfo: contactInfo,
		Status:      0, // pending
	}

	// reason 和 company_name 放在 ContactInfo JSON 里或分离存储（这里简化处理）
	app.CompanyName = extractCompanyName(reason) // 简化：从 reason 提取公司名

	if err := s.db.Create(&app).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"application_id": app.ID,
		"status":         "pending",
	}, nil
}

// GetKOLStatus 查询申请状态
func (s *AffService) GetKOLStatus(userID int) (map[string]interface{}, error) {
	var apps []models.KolApplication
	if err := s.db.Where("user_id = ?", userID).
		Order("applied_at DESC").
		Find(&apps).Error; err != nil {
		return nil, err
	}

	appList := make([]map[string]interface{}, 0)
	for _, app := range apps {
		statusStr := "pending"
		if app.Status == 1 {
			statusStr = "approved"
		} else if app.Status == 2 {
			statusStr = "rejected"
		}

		reviewedAt := ""
		if app.ProcessedAt != nil {
			reviewedAt = app.ProcessedAt.Format("2006-01-02 15:04:05")
		}

		appList = append(appList, map[string]interface{}{
			"id":           app.ID,
			"type":         app.ApplyType,
			"status":       statusStr,
			"submitted_at": app.AppliedAt.Format("2006-01-02 15:04:05"),
			"reviewed_at":  reviewedAt,
			"admin_notes":  app.AdminRemark,
		})
	}

	return map[string]interface{}{
		"applications": appList,
	}, nil
}

// GrantRegisterReward 注册奖励（邀请人获得 $0.10）
func (s *AffService) GrantRegisterReward(inviterID, inviteeID int) error {
	// 查询配置
	var config models.ReferralConfig
	if err := s.db.Where("reward_type = ? AND enabled = true", "register").First(&config).Error; err != nil {
		// 配置不存在或未启用，跳过
		return nil
	}

	quotaAmount := utils.DollarsToQuota(config.RewardAmount)

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 增加邀请人 aff_quota
		if err := tx.Model(&models.User{}).Where("id = ?", inviterID).
			Update("aff_quota", gorm.Expr("aff_quota + ?", quotaAmount)).Error; err != nil {
			return err
		}

		// 记录历史
		history := models.AffHistory{
			UserID:    inviterID,
			InviteeID: inviteeID,
			Quota:     quotaAmount,
			EventType: "register",
			Status:    1, // completed
		}
		return tx.Create(&history).Error
	})
}

// GrantFirstTopupReward 首充奖励（邀请人获得 $1.00）
func (s *AffService) GrantFirstTopupReward(inviteeID int) error {
	// 查询被邀请人信息
	var invitee models.User
	if err := s.db.First(&invitee, inviteeID).Error; err != nil {
		return err
	}

	if invitee.InviterID == 0 {
		// 无邀请人，跳过
		return nil
	}

	// 检查是否已发放过首充奖励
	var count int64
	if err := s.db.Model(&models.AffHistory{}).
		Where("user_id = ? AND invitee_id = ? AND event_type = 'first_topup'", invitee.InviterID, inviteeID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		// 已发放，跳过
		return nil
	}

	// 查询配置
	var config models.ReferralConfig
	if err := s.db.Where("reward_type = ? AND enabled = true", "first_topup").First(&config).Error; err != nil {
		// 配置不存在或未启用，跳过
		return nil
	}

	quotaAmount := utils.DollarsToQuota(config.RewardAmount)

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 增加邀请人 aff_quota
		if err := tx.Model(&models.User{}).Where("id = ?", invitee.InviterID).
			Update("aff_quota", gorm.Expr("aff_quota + ?", quotaAmount)).Error; err != nil {
			return err
		}

		// 记录历史
		history := models.AffHistory{
			UserID:    invitee.InviterID,
			InviteeID: inviteeID,
			Quota:     quotaAmount,
			EventType: "first_topup",
			Status:    1, // completed
		}
		return tx.Create(&history).Error
	})
}

// extractCompanyName 从 reason 字符串中提取公司名（简化实现）
func extractCompanyName(reason string) string {
	// 简单取前50个字符作为公司名
	if len(reason) > 50 {
		return strings.TrimSpace(reason[:50])
	}
	return strings.TrimSpace(reason)
}
