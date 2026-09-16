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
	"gorm.io/gorm/clause"
)

// 返佣规则常量（硬编码，后续通过 admin 配置接口动态调整）
// TODO: 实现 admin 配置接口 (aff_settings 表 + GET/PUT /api/admin/aff/settings)
const (
	LEVEL1_RATE     = 0.10 // 一级返佣比例：首充金额的 10%
	LEVEL2_RATE     = 0.05 // 二级返佣比例：首充金额的 5%
	REGISTER_REWARD = 0.5  // 注册奖励：$0.5 给直接邀请人
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

// GrantRegisterReward 注册奖励（邀请人获得 $0.5）
// 防重复：检查 aff_history 是否已有相同 (user_id, invitee_id, event_type='register') 记录
func (s *AffService) GrantRegisterReward(inviterID, inviteeID int) error {
	// 防重复检查
	var count int64
	if err := s.db.Model(&models.AffHistory{}).
		Where("user_id = ? AND invitee_id = ? AND event_type = 'register'", inviterID, inviteeID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		// 已发放过注册奖励，跳过
		return nil
	}

	quotaAmount := utils.DollarsToQuota(REGISTER_REWARD)

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

// GrantFirstTopupReward 首充奖励（一级返佣 10%，二级返佣 5%）
// 防重复：检查 aff_history 是否已有 (invitee_id, event_type='first_topup') 记录
func (s *AffService) GrantFirstTopupReward(inviteeID int, topupAmount float64) error {
	// 检查是否已发放过首充奖励（按 invitee_id 去重，不关心邀请人）
	var count int64
	if err := s.db.Model(&models.AffHistory{}).
		Where("invitee_id = ? AND event_type = 'first_topup'", inviteeID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		// 已发放，跳过
		return nil
	}

	// 查询被邀请人信息
	var invitee models.User
	if err := s.db.First(&invitee, inviteeID).Error; err != nil {
		return err
	}

	if invitee.InviterID == 0 {
		// 无邀请人，跳过
		return nil
	}

	// 计算一级返佣
	level1Reward := topupAmount * LEVEL1_RATE
	level1Quota := utils.DollarsToQuota(level1Reward)

	// 查询一级邀请人
	var level1Inviter models.User
	if err := s.db.First(&level1Inviter, invitee.InviterID).Error; err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 给一级邀请人发放返佣
		if err := tx.Model(&models.User{}).Where("id = ?", level1Inviter.ID).
			Update("aff_quota", gorm.Expr("aff_quota + ?", level1Quota)).Error; err != nil {
			return err
		}

		// 记录一级返佣历史
		history1 := models.AffHistory{
			UserID:         level1Inviter.ID,
			InviteeID:      inviteeID,
			Quota:          level1Quota,
			CommissionRate: LEVEL1_RATE,
			EventType:      "first_topup",
			Status:         1, // completed
		}
		if err := tx.Create(&history1).Error; err != nil {
			return err
		}

		// 二级返佣（如果一级邀请人也有邀请人）
		if level1Inviter.InviterID > 0 {
			level2Reward := topupAmount * LEVEL2_RATE
			level2Quota := utils.DollarsToQuota(level2Reward)

			// 给二级邀请人发放返佣
			if err := tx.Model(&models.User{}).Where("id = ?", level1Inviter.InviterID).
				Update("aff_quota", gorm.Expr("aff_quota + ?", level2Quota)).Error; err != nil {
				return err
			}

			// 记录二级返佣历史
			history2 := models.AffHistory{
				UserID:         level1Inviter.InviterID,
				InviteeID:      inviteeID,
				Quota:          level2Quota,
				CommissionRate: LEVEL2_RATE,
				EventType:      "first_topup",
				Status:         1, // completed
			}
			if err := tx.Create(&history2).Error; err != nil {
				return err
			}
		}

		return nil
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

// =============================================================================
// 批10：admin 提现审批 + 返佣统计看板
// =============================================================================

// 提现审批相关错误
var (
	ErrWithdrawalNotFound   = errors.New("withdrawal request not found")
	ErrWithdrawalNotPending = errors.New("withdrawal request is not pending")
	ErrInsufficientAffQuota = errors.New("insufficient aff_quota for withdrawal")
	ErrRemarkRequired       = errors.New("admin_remark is required for rejection")
)

// ListWithdrawals 查询提现申请列表（admin 用，支持 status 过滤 + 分页）
// statusFilter: nil 表示不过滤; 否则按 status 过滤 (0=pending 1=approved 2=rejected)
func (s *AffService) ListWithdrawals(statusFilter *int, limit, offset int) (map[string]interface{}, error) {
	query := s.db.Model(&models.WithdrawalRequest{})
	if statusFilter != nil {
		query = query.Where("status = ?", *statusFilter)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	var requests []models.WithdrawalRequest
	if err := query.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&requests).Error; err != nil {
		return nil, err
	}

	// 收集 user_id 批量查 username
	userIDs := make([]int, 0, len(requests))
	for _, r := range requests {
		userIDs = append(userIDs, r.UserID)
	}
	usernameMap := make(map[int]string)
	if len(userIDs) > 0 {
		var users []models.User
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				usernameMap[u.ID] = u.Username
			}
		}
	}

	data := make([]map[string]interface{}, 0, len(requests))
	for _, r := range requests {
		processedAt := ""
		if r.ProcessedAt != nil {
			processedAt = r.ProcessedAt.Format("2006-01-02 15:04:05")
		}
		data = append(data, map[string]interface{}{
			"id":             r.ID,
			"user_id":        r.UserID,
			"username":       usernameMap[r.UserID],
			"amount":         r.Amount,
			"payment_method": r.PaymentMethod,
			"account_info":   r.AccountInfo,
			"status":         r.Status,
			"applied_at":     r.CreatedAt.Format("2006-01-02 15:04:05"),
			"processed_at":   processedAt,
			"admin_remark":   r.AdminRemark,
		})
	}

	return map[string]interface{}{
		"data":  data,
		"total": total,
	}, nil
}

// ApproveWithdrawal 审批通过提现申请（事务安全）
// 流程:
//  1. 行锁查 withdrawal_requests, 校验 status=0 (pending)
//  2. 校验 user.aff_quota >= amount (避免重复审批导致负数)
//  3. 扣 user.aff_quota
//  4. 写 aff_history (event_type='withdraw', quota=负数, status=1)
//  5. 更新 withdrawal_requests: status=1, processed_at=NOW(), admin_remark=?
func (s *AffService) ApproveWithdrawal(requestID int64, adminRemark string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查提现申请（防并发）
		var request models.WithdrawalRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", requestID).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawalNotFound
			}
			return err
		}

		// 校验 pending 状态
		if request.Status != 0 {
			return ErrWithdrawalNotPending
		}

		// 2. 校验用户 aff_quota 充足
		var user models.User
		if err := tx.First(&user, request.UserID).Error; err != nil {
			return err
		}
		quotaAmount := utils.DollarsToQuota(request.Amount)
		if user.AffQuota < quotaAmount {
			return ErrInsufficientAffQuota
		}

		// 3. 扣除 aff_quota
		if err := tx.Model(&models.User{}).Where("id = ?", request.UserID).
			Update("aff_quota", gorm.Expr("aff_quota - ?", quotaAmount)).Error; err != nil {
			return err
		}

		// 4. 写 aff_history (event_type='withdraw', quota=负数)
		history := models.AffHistory{
			UserID:    request.UserID,
			InviteeID: request.UserID, // 提现是用户自己的行为
			Quota:     -quotaAmount,   // 负数表示扣款
			EventType: "withdraw",
			Status:    1, // completed
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}

		// 5. 更新提现申请状态
		now := time.Now()
		if err := tx.Model(&models.WithdrawalRequest{}).
			Where("id = ?", requestID).
			Updates(map[string]interface{}{
				"status":       1, // approved
				"processed_at": now,
				"admin_remark": adminRemark,
			}).Error; err != nil {
			return err
		}

		return nil
	})
}

// RejectWithdrawal 拒绝提现申请
// 流程:
//  1. 查 withdrawal_requests, 校验 status=0 (pending)
//  2. 更新: status=2, processed_at=NOW(), admin_remark=?
func (s *AffService) RejectWithdrawal(requestID int64, adminRemark string) error {
	if strings.TrimSpace(adminRemark) == "" {
		return ErrRemarkRequired
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var request models.WithdrawalRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", requestID).First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWithdrawalNotFound
			}
			return err
		}

		if request.Status != 0 {
			return ErrWithdrawalNotPending
		}

		now := time.Now()
		return tx.Model(&models.WithdrawalRequest{}).
			Where("id = ?", requestID).
			Updates(map[string]interface{}{
				"status":       2, // rejected
				"processed_at": now,
				"admin_remark": adminRemark,
			}).Error
	})
}

// GetAffStats 用户/分站自己的返佣统计（GET /api/dist/aff/stats）
func (s *AffService) GetAffStats(userID int) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// 历史总收益（aff_history 非 withdraw 且非 transfer 的 sum）
	var totalEarnings int64
	s.db.Model(&models.AffHistory{}).
		Where("user_id = ? AND status = 1 AND event_type NOT IN ('withdraw', 'transfer')", userID).
		Select("COALESCE(SUM(quota), 0)").Scan(&totalEarnings)

	// 已提现（aff_history.withdraw 的 sum，取绝对值）
	var totalWithdrawnNeg int64
	s.db.Model(&models.AffHistory{}).
		Where("user_id = ? AND status = 1 AND event_type = 'withdraw'", userID).
		Select("COALESCE(SUM(quota), 0)").Scan(&totalWithdrawnNeg)
	totalWithdrawn := -totalWithdrawnNeg // withdraw 记录为负数

	// 一级邀请人数
	var level1Count int64
	s.db.Model(&models.User{}).Where("inviter_id = ?", userID).Count(&level1Count)

	// 二级邀请人数
	var level2Count int64
	var level1IDs []int
	s.db.Model(&models.User{}).Where("inviter_id = ?", userID).Pluck("id", &level1IDs)
	if len(level1IDs) > 0 {
		s.db.Model(&models.User{}).Where("inviter_id IN ?", level1IDs).Count(&level2Count)
	}

	// 本月新增邀请（一级，created_at >= 本月1号）
	var thisMonthNew int64
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	// users.created_at 是 Unix 秒
	s.db.Model(&models.User{}).
		Where("inviter_id = ? AND created_at >= ?", userID, monthStart.Unix()).
		Count(&thisMonthNew)

	return map[string]interface{}{
		"total_earnings":         utils.QuotaToDollars(totalEarnings),
		"available_balance":      utils.QuotaToDollars(user.AffQuota),
		"total_withdrawn":        utils.QuotaToDollars(totalWithdrawn),
		"level1_invitees":        level1Count,
		"level2_invitees":        level2Count,
		"this_month_new_invitees": thisMonthNew,
	}, nil
}

// GetAffOverview 平台级返佣大盘（GET /api/admin/aff/overview）
func (s *AffService) GetAffOverview() (map[string]interface{}, error) {
	// 平台总发放（所有 aff_history 非 withdraw 且非 transfer 的 sum）
	var totalDistributed int64
	s.db.Model(&models.AffHistory{}).
		Where("status = 1 AND event_type NOT IN ('withdraw', 'transfer')").
		Select("COALESCE(SUM(quota), 0)").Scan(&totalDistributed)

	// 待提现（所有 users.aff_quota 的 sum）
	var pendingWithdrawal int64
	s.db.Model(&models.User{}).
		Select("COALESCE(SUM(aff_quota), 0)").Scan(&pendingWithdrawal)

	// 待审核提现申请数（status=0）
	var pendingRequests int64
	s.db.Model(&models.WithdrawalRequest{}).Where("status = 0").Count(&pendingRequests)

	// 返佣收益 TOP10（按 user_id 聚合 aff_history 收益）
	type topInviterRow struct {
		UserID        int
		TotalEarnings int64
	}
	var topRows []topInviterRow
	s.db.Model(&models.AffHistory{}).
		Where("status = 1 AND event_type NOT IN ('withdraw', 'transfer')").
		Select("user_id, COALESCE(SUM(quota), 0) as total_earnings").
		Group("user_id").
		Order("total_earnings DESC").
		Limit(10).
		Scan(&topRows)

	topInviters := make([]map[string]interface{}, 0, len(topRows))
	for _, row := range topRows {
		var username string
		var u models.User
		if err := s.db.First(&u, row.UserID).Error; err == nil {
			username = u.Username
		}
		// 邀请人数（一级）
		var inviteesCount int64
		s.db.Model(&models.User{}).Where("inviter_id = ?", row.UserID).Count(&inviteesCount)

		topInviters = append(topInviters, map[string]interface{}{
			"username":       username,
			"total_earnings": utils.QuotaToDollars(row.TotalEarnings),
			"invitees_count": inviteesCount,
		})
	}

	return map[string]interface{}{
		"total_distributed":  utils.QuotaToDollars(totalDistributed),
		"pending_withdrawal": utils.QuotaToDollars(pendingWithdrawal),
		"pending_requests":   pendingRequests,
		"top_inviters":       topInviters,
	}, nil
}
