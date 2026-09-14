// Package services 实现业务逻辑。
package services

import (
	"errors"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrRedeemCodeRequired 兑换码为空
	ErrRedeemCodeRequired = errors.New("redeem code required")
	// ErrRedeemCodeInvalid 兑换码无效（不存在）
	ErrRedeemCodeInvalid = errors.New("invalid redeem code")
	// ErrRedeemCodeUsed 兑换码已使用
	ErrRedeemCodeUsed = errors.New("redeem code already used")
	// ErrRedeemCodeDisabled 兑换码已禁用
	ErrRedeemCodeDisabled = errors.New("redeem code disabled")
	// ErrRedeemCodeExpired 兑换码已过期
	ErrRedeemCodeExpired = errors.New("redeem code expired")
)

// TopupService 充值相关业务逻辑。
type TopupService struct {
	db *gorm.DB
}

// NewTopupService 创建 TopupService。
func NewTopupService(db *gorm.DB) *TopupService {
	return &TopupService{db: db}
}

// RedeemCodeInput 兑换充值码入参。
type RedeemCodeInput struct {
	Key string `json:"key" binding:"required"`
}

// RedeemCodeResult 兑换充值码结果。
type RedeemCodeResult struct {
	Quota   int    `json:"quota"`   // 本次兑换增加的 quota
	Message string `json:"message"` // 成功提示消息
}

// RedeemCode 兑换充值码（事务安全）。
//
// 核心逻辑:
//  1. 查询充值码（加行锁 SELECT FOR UPDATE）
//  2. 校验状态（status=1 未使用、未过期）
//  3. 事务内更新充值码（status=3 已使用、记录使用时间和用户ID）
//  4. 事务内给用户增加 quota
//  5. 提交事务
//
// 并发安全: 使用 gorm Transaction + clause.Locking 防止重复兑换。
func (s *TopupService) RedeemCode(userID int, key string) (*RedeemCodeResult, error) {
	if key == "" {
		return nil, ErrRedeemCodeRequired
	}

	var result *RedeemCodeResult

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询充值码并加行锁（SELECT FOR UPDATE）
		var redemption models.Redemption
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key = ?", key).
			First(&redemption).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRedeemCodeInvalid
			}
			return err
		}

		// 2. 校验状态
		// Status: 1=未使用 2=已禁用 3=已使用
		switch redemption.Status {
		case 2:
			return ErrRedeemCodeDisabled
		case 3:
			return ErrRedeemCodeUsed
		case 1:
			// 继续
		default:
			return ErrRedeemCodeInvalid
		}

		// 3. 校验过期时间（0 表示不过期）
		now := time.Now().Unix()
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < now {
			return ErrRedeemCodeExpired
		}

		// 4. 更新充值码状态（防并发重复兑换：WHERE status=1）
		updateResult := tx.Model(&models.Redemption{}).
			Where("id = ? AND status = ?", redemption.Id, 1).
			Updates(map[string]interface{}{
				"status":        3,
				"redeemed_time": now,
				"used_user_id":  userID,
			})

		if updateResult.Error != nil {
			return updateResult.Error
		}

		// 并发情况下，另一个事务可能已经更新了状态，RowsAffected = 0
		if updateResult.RowsAffected == 0 {
			return ErrRedeemCodeUsed
		}

		// 5. 给用户增加 quota（redemption.Quota 直接加到 user.Quota）
		err = tx.Model(&models.User{}).
			Where("id = ?", userID).
			Update("quota", gorm.Expr("quota + ?", redemption.Quota)).
			Error

		if err != nil {
			return err
		}

		// 构造返回结果
		result = &RedeemCodeResult{
			Quota:   redemption.Quota,
			Message: "充值成功",
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetTopupHistory 获取当前用户的充值历史。
//
// 查询 topup_orders 表，WHERE user_id = 当前用户，按创建时间倒序。
func (s *TopupService) GetTopupHistory(userID int) ([]models.TopupOrder, error) {
	var orders []models.TopupOrder
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&orders).Error

	if err != nil {
		return nil, err
	}

	return orders, nil
}
