// Package services 提供套餐管理业务逻辑。
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/tiancookie/leapnode-backend/internal/models"
	"github.com/tiancookie/leapnode-backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PackageService 套餐服务
type PackageService struct {
	db *gorm.DB
}

// NewPackageService 创建套餐服务实例
func NewPackageService(db *gorm.DB) *PackageService {
	return &PackageService{db: db}
}

// PackageCreateInput 创建套餐请求
type PackageCreateInput struct {
	Name          string  `json:"name" binding:"required"`
	Description   string  `json:"description"`
	Price         float64 `json:"price" binding:"required,gt=0"`
	OriginalPrice float64 `json:"original_price"`
	QuotaAmount   int64   `json:"quota_amount" binding:"required,gt=0"`
	DurationDays  int     `json:"duration_days" binding:"required,gt=0"`
	ResetPeriod   string  `json:"reset_period" binding:"required,oneof=none daily weekly monthly"`
	Enabled       *bool   `json:"enabled"`
}

// PackageUpdateInput 更新套餐请求
type PackageUpdateInput struct {
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	QuotaAmount   *int64   `json:"quota_amount"`
	DurationDays  *int     `json:"duration_days"`
	ResetPeriod   *string  `json:"reset_period"`
	Enabled       *bool    `json:"enabled"`
}

// PackageDTO 套餐 DTO（前端契约字段）
type PackageDTO struct {
	ID               uint    `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Price            float64 `json:"price"`
	OriginalPrice    float64 `json:"original_price"`
	QuotaAmount      int64   `json:"quota_amount"`
	Duration         int     `json:"duration"`              // 前端用 duration，不是 duration_days
	QuotaResetPeriod string  `json:"quota_reset_period"`    // 前端用这个字段名
	Enabled          bool    `json:"enabled"`
}

// SubscriptionDTO 订阅 DTO（前端契约字段）
type SubscriptionDTO struct {
	ID            uint  `json:"id"`
	PackageID     uint  `json:"package_id"`
	AmountTotal   int64 `json:"amount_total"`    // 总额度
	AmountUsed    int64 `json:"amount_used"`     // 已用额度
	EndTime       int64 `json:"end_time"`        // Unix 时间戳（秒）
	NextResetTime int64 `json:"next_reset_time"` // 下次重置时间（秒，0=不重置）
}

// CreatePackage 创建套餐（分站站长）
func (s *PackageService) CreatePackage(distributorID int, input *PackageCreateInput) (*models.Package, error) {
	pkg := &models.Package{
		DistributorID: distributorID,
		Name:          input.Name,
		Description:   input.Description,
		Price:         input.Price,
		OriginalPrice: input.OriginalPrice,
		QuotaAmount:   input.QuotaAmount,
		DurationDays:  input.DurationDays,
		ResetPeriod:   input.ResetPeriod,
		Enabled:       true,
	}

	if input.Enabled != nil {
		pkg.Enabled = *input.Enabled
	}

	if err := s.db.Create(pkg).Error; err != nil {
		return nil, fmt.Errorf("failed to create package: %w", err)
	}

	return pkg, nil
}

// UpdatePackage 更新套餐
func (s *PackageService) UpdatePackage(id int, distributorID int, input *PackageUpdateInput) (*models.Package, error) {
	var pkg models.Package
	
	// 先查询确保套餐存在且归属正确
	if err := s.db.Where("id = ? AND distributor_id = ?", id, distributorID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("package not found or access denied")
		}
		return nil, fmt.Errorf("failed to find package: %w", err)
	}

	// 更新字段
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Price != nil {
		updates["price"] = *input.Price
	}
	if input.OriginalPrice != nil {
		updates["original_price"] = *input.OriginalPrice
	}
	if input.QuotaAmount != nil {
		updates["quota_amount"] = *input.QuotaAmount
	}
	if input.DurationDays != nil {
		updates["duration_days"] = *input.DurationDays
	}
	if input.ResetPeriod != nil {
		updates["reset_period"] = *input.ResetPeriod
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}

	if err := s.db.Model(&pkg).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update package: %w", err)
	}

	return &pkg, nil
}

// DeletePackage 删除套餐（硬删除，因为有外键约束需谨慎）
func (s *PackageService) DeletePackage(id int, distributorID int) error {
	// 先检查归属
	var pkg models.Package
	if err := s.db.Where("id = ? AND distributor_id = ?", id, distributorID).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("package not found or access denied")
		}
		return fmt.Errorf("failed to find package: %w", err)
	}

	// 检查是否有活跃订阅
	var count int64
	if err := s.db.Model(&models.Subscription{}).
		Where("package_id = ? AND status = 1", id).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check subscriptions: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("cannot delete package with %d active subscriptions", count)
	}

	// 硬删除
	if err := s.db.Delete(&pkg).Error; err != nil {
		return fmt.Errorf("failed to delete package: %w", err)
	}

	return nil
}

// GetDistributorPackages 查询分站的套餐列表
func (s *PackageService) GetDistributorPackages(distributorID int) ([]*PackageDTO, error) {
	var packages []models.Package
	if err := s.db.Where("distributor_id = ?", distributorID).
		Order("created_at DESC").
		Find(&packages).Error; err != nil {
		return nil, fmt.Errorf("failed to get packages: %w", err)
	}

	dtos := make([]*PackageDTO, len(packages))
	for i, pkg := range packages {
		dtos[i] = toPackageDTO(&pkg)
	}

	return dtos, nil
}

// GetPublicPackages 查询公开套餐列表（enabled=true）
func (s *PackageService) GetPublicPackages() ([]*PackageDTO, error) {
	var packages []models.Package
	if err := s.db.Where("enabled = ?", true).
		Order("distributor_id ASC, price ASC"). // 总站套餐优先，价格升序
		Find(&packages).Error; err != nil {
		return nil, fmt.Errorf("failed to get public packages: %w", err)
	}

	dtos := make([]*PackageDTO, len(packages))
	for i, pkg := range packages {
		dtos[i] = toPackageDTO(&pkg)
	}

	return dtos, nil
}

// SubscribePackage 订阅套餐
func (s *PackageService) SubscribePackage(userID int, packageID int) (map[string]interface{}, error) {
	// 汇率常量：1 USD = 7.14 CNY（与前端 Packages.jsx 396 行一致）
	const usdRate = 7.14

	var result map[string]interface{}
	
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询套餐（enabled=true）
		var pkg models.Package
		if err := tx.Where("id = ? AND enabled = ?", packageID, true).First(&pkg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("package not found or disabled")
			}
			return fmt.Errorf("failed to find package: %w", err)
		}

		// 2. 锁定用户行（FOR UPDATE）
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userID).First(&user).Error; err != nil {
			return fmt.Errorf("failed to lock user: %w", err)
		}

		// 3. 计算套餐价格对应的 quota（price_cny / usdRate * 500000）
		quotaCost := int64(pkg.Price / usdRate * float64(utils.QuotaPerDollar))

		// 4. 检查余额
		if user.Quota < quotaCost {
			return fmt.Errorf("insufficient quota: need %d, have %d", quotaCost, user.Quota)
		}

		// 5. 扣款
		if err := tx.Model(&user).Update("quota", gorm.Expr("quota - ?", quotaCost)).Error; err != nil {
			return fmt.Errorf("failed to deduct quota: %w", err)
		}

		// 6. 计算订阅时间
		now := time.Now()
		startAt := now
		expireAt := now.AddDate(0, 0, pkg.DurationDays)
		nextResetAt := calculateNextReset(pkg.ResetPeriod, now)

		// 7. 创建订阅记录
		subscription := &models.Subscription{
			UserID:      uint(userID),
			PackageID:   pkg.ID,
			Status:      1,
			StartAt:     startAt,
			ExpireAt:    expireAt,
			RemainQuota: pkg.QuotaAmount,
			NextResetAt: nextResetAt,
		}

		if err := tx.Create(subscription).Error; err != nil {
			return fmt.Errorf("failed to create subscription: %w", err)
		}

		result = map[string]interface{}{
			"subscription_id": subscription.ID,
			"remain_quota":    subscription.RemainQuota,
		}
		
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetActiveSubscriptions 查询用户的活跃订阅
func (s *PackageService) GetActiveSubscriptions(userID int) ([]*SubscriptionDTO, error) {
	var subscriptions []models.Subscription
	if err := s.db.Where("user_id = ? AND status = 1", userID).
		Order("created_at DESC").
		Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}

	// 需要关联 package 获取总额度
	var packageIDs []uint
	for _, sub := range subscriptions {
		packageIDs = append(packageIDs, sub.PackageID)
	}

	var packages []models.Package
	if len(packageIDs) > 0 {
		if err := s.db.Where("id IN ?", packageIDs).Find(&packages).Error; err != nil {
			return nil, fmt.Errorf("failed to get packages: %w", err)
		}
	}

	// 建立 package_id -> quota_amount 映射
	pkgMap := make(map[uint]int64)
	for _, pkg := range packages {
		pkgMap[pkg.ID] = pkg.QuotaAmount
	}

	dtos := make([]*SubscriptionDTO, len(subscriptions))
	for i, sub := range subscriptions {
		dtos[i] = toSubscriptionDTO(&sub, pkgMap[sub.PackageID])
	}

	return dtos, nil
}

// calculateNextReset 计算下次重置时间
func calculateNextReset(resetPeriod string, now time.Time) time.Time {
	switch resetPeriod {
	case "daily":
		return now.AddDate(0, 0, 1)
	case "weekly":
		return now.AddDate(0, 0, 7)
	case "monthly":
		return now.AddDate(0, 1, 0)
	case "none":
		fallthrough
	default:
		return time.Time{} // zero time（前端会转成 0）
	}
}

// toPackageDTO 模型转 DTO（字段名映射）
func toPackageDTO(pkg *models.Package) *PackageDTO {
	return &PackageDTO{
		ID:               pkg.ID,
		Name:             pkg.Name,
		Description:      pkg.Description,
		Price:            pkg.Price,
		OriginalPrice:    pkg.OriginalPrice,
		QuotaAmount:      pkg.QuotaAmount,
		Duration:         pkg.DurationDays,     // 映射字段名
		QuotaResetPeriod: pkg.ResetPeriod,      // 映射字段名
		Enabled:          pkg.Enabled,
	}
}

// toSubscriptionDTO 订阅模型转 DTO
func toSubscriptionDTO(sub *models.Subscription, totalQuota int64) *SubscriptionDTO {
	amountUsed := totalQuota - sub.RemainQuota
	if amountUsed < 0 {
		amountUsed = 0
	}

	nextResetTime := int64(0)
	if !sub.NextResetAt.IsZero() {
		nextResetTime = sub.NextResetAt.Unix()
	}

	return &SubscriptionDTO{
		ID:            sub.ID,
		PackageID:     sub.PackageID,
		AmountTotal:   totalQuota,
		AmountUsed:    amountUsed,
		EndTime:       sub.ExpireAt.Unix(),
		NextResetTime: nextResetTime,
	}
}
